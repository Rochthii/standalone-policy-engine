package tests

import (
	"context"
	"net"
	"os"
	"runtime"
	"sort"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"standalone-policy-engine/internal/audit"
	"standalone-policy-engine/internal/config"
	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/parser"
	"standalone-policy-engine/internal/security"
	"standalone-policy-engine/internal/server"
	policyv1 "standalone-policy-engine/proto/v1"

	"github.com/golang-jwt/jwt/v5"
	"github.com/shirou/gopsutil/v4/process"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

const fullPathSampleCount = 10_000

type fullPathAuditWriter struct {
	written atomic.Uint64
}

func (w *fullPathAuditWriter) InsertAuditLogsBatch(_ context.Context, entries []*audit.LogEntry) error {
	w.written.Add(uint64(len(entries)))
	return nil
}

func TestFullPathEvidence(t *testing.T) {
	if os.Getenv("RUN_PERF_FULL") != "1" {
		t.Skip("set RUN_PERF_FULL=1 to run the 10,000-request full-path measurement")
	}

	connection, grpcServer, logger, writer, client, requestContext, request := newFullPathFixture(t)
	t.Cleanup(func() { _ = connection.Close() })
	t.Cleanup(grpcServer.Stop)
	t.Cleanup(logger.Stop)

	if response, err := client.CheckAccess(requestContext, request); err != nil || response.Decision != policyv1.CheckAccessResponse_ALLOW {
		t.Fatalf("warm-up full-path request failed: response=%v err=%v", response, err)
	}

	proc, err := process.NewProcess(int32(os.Getpid()))
	if err != nil {
		t.Fatalf("open current process for CPU/RSS evidence: %v", err)
	}
	runtime.GC()
	beforeMemory, err := proc.MemoryInfo()
	if err != nil {
		t.Fatalf("read starting RSS: %v", err)
	}
	beforeCPU, err := proc.Times()
	if err != nil {
		t.Fatalf("read starting CPU time: %v", err)
	}
	var beforeGC runtime.MemStats
	runtime.ReadMemStats(&beforeGC)

	latencies := make([]time.Duration, fullPathSampleCount)
	clock := newPerformanceClock(t)
	started := time.Now()
	for i := range latencies {
		requestStarted := clock.now()
		response, err := client.CheckAccess(requestContext, request)
		latencies[i] = clock.elapsed(requestStarted)
		if err != nil || response.Decision != policyv1.CheckAccessResponse_ALLOW {
			t.Fatalf("request %d failed: response=%v err=%v", i, response, err)
		}
	}
	elapsed := time.Since(started)

	logger.Stop()
	stats := logger.Stats()
	if stats.Written < fullPathSampleCount+1 || writer.written.Load() < fullPathSampleCount+1 {
		t.Fatalf("audit queue did not drain benchmark entries: stats=%+v", stats)
	}

	afterMemory, err := proc.MemoryInfo()
	if err != nil {
		t.Fatalf("read ending RSS: %v", err)
	}
	afterCPU, err := proc.Times()
	if err != nil {
		t.Fatalf("read ending CPU time: %v", err)
	}
	var afterGC runtime.MemStats
	runtime.ReadMemStats(&afterGC)

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	if latencies[0] <= 0 {
		t.Fatalf("invalid zero latency sample: %d of %d requests completed before timing started", countZeroDurations(latencies), len(latencies))
	}
	cpuSeconds := (afterCPU.User + afterCPU.System) - (beforeCPU.User + beforeCPU.System)
	t.Logf(
		"PERF_FULL samples=%d errors=0 elapsed=%s throughput=%.0f rps p50=%s p95=%s p99=%s p99.9=%s cpu=%.1f%% rss_start=%d rss_end=%d gc_cycles=%d gc_pause=%s",
		fullPathSampleCount,
		elapsed.Round(time.Microsecond),
		float64(fullPathSampleCount)/elapsed.Seconds(),
		percentile(latencies, 0.50),
		percentile(latencies, 0.95),
		percentile(latencies, 0.99),
		percentile(latencies, 0.999),
		100*cpuSeconds/elapsed.Seconds(),
		beforeMemory.RSS,
		afterMemory.RSS,
		afterGC.NumGC-beforeGC.NumGC,
		time.Duration(afterGC.PauseTotalNs-beforeGC.PauseTotalNs),
	)
}

func newFullPathFixture(t *testing.T) (*grpc.ClientConn, *grpc.Server, *audit.AuditLogger, *fullPathAuditWriter, policyv1.PolicyDecisionPointClient, context.Context, *policyv1.CheckAccessRequest) {
	t.Helper()
	const (
		tenantID         = "tenant-perf-full"
		agent            = "agent:perf_agent"
		action           = "action:APPROVE_PURCHASE_ORDER"
		resource         = "purchase_order:PO001"
		jwtSecret        = "perf-full-jwt-secret-at-least-thirty-two-bytes"
		delegationKeyID  = "perf-key-2026"
		delegationSecret = "perf-full-delegation-secret-at-least-thirty-two-bytes"
	)

	eng := engine.NewEngineWithGC(engine.GCConfig{Enabled: false})
	policy := compileHelper(t, parser.NewCompiler(), "P-PERF-FULL", `permit(principal == agent:perf_agent, action == action:APPROVE_PURCHASE_ORDER, resource == purchase_order:PO001)
when { context.amount <= 2000 && context.execution_mode == "autonomous_run" };`)
	if err := eng.UpdateTenantPolicies(tenantID, []*parser.PolicyNode{policy}, nil); err != nil {
		t.Fatalf("seed full-path policy: %v", err)
	}

	writer := &fullPathAuditWriter{}
	logger, err := audit.NewBatchAuditLogger(writer, audit.BatchConfig{
		QueueCapacity: 20_000,
		BatchSize:     128,
		FlushInterval: 10 * time.Millisecond,
		WriteTimeout:  time.Second,
	})
	if err != nil {
		t.Fatalf("create full-path audit logger: %v", err)
	}
	logger.Start(context.Background())

	securityConfig := config.SecurityConfig{
		JWTSecret:             jwtSecret,
		JWTIssuer:             "perf-full",
		JWTAudience:           "perf-full-client",
		DelegationActiveKeyID: delegationKeyID,
		DelegationKeys:        map[string]string{delegationKeyID: delegationSecret},
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen full-path gRPC: %v", err)
	}
	grpcServer, err := server.StartGRPCServer(listener, eng, logger, securityConfig, config.ServerConfig{
		EvaluationTimeout:   100 * time.Millisecond,
		GRPCMaxReceiveBytes: 1 << 20,
		GRPCMaxSendBytes:    1 << 20,
	})
	if err != nil {
		_ = listener.Close()
		t.Fatalf("start full-path gRPC: %v", err)
	}

	dialContext, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	connection, err := grpc.DialContext(dialContext, listener.Addr().String(), grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock())
	if err != nil {
		grpcServer.Stop()
		t.Fatalf("dial full-path gRPC: %v", err)
	}

	signer, err := security.NewDelegationManagerWithKeyring(delegationKeyID, map[string]string{delegationKeyID: delegationSecret})
	if err != nil {
		_ = connection.Close()
		grpcServer.Stop()
		t.Fatalf("create delegation signer: %v", err)
	}
	validUntil := strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)
	delegationContext := signedDelegationContext(t, signer, tenantID, "grant-perf-full", "user:manager", agent, action, resource, "1500", validUntil, "user:creator")

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":       agent,
		"tenant_id": tenantID,
		"iss":       securityConfig.JWTIssuer,
		"aud":       securityConfig.JWTAudience,
		"exp":       time.Now().Add(time.Hour).Unix(),
	})
	tokenString, err := token.SignedString([]byte(jwtSecret))
	if err != nil {
		_ = connection.Close()
		grpcServer.Stop()
		t.Fatalf("sign full-path JWT: %v", err)
	}

	return connection,
		grpcServer,
		logger,
		writer,
		policyv1.NewPolicyDecisionPointClient(connection),
		metadata.NewOutgoingContext(context.Background(), metadata.Pairs("authorization", "Bearer "+tokenString)),
		&policyv1.CheckAccessRequest{TenantId: tenantID, Subject: agent, Action: action, Resource: resource, Context: delegationContext}
}

func percentile(samples []time.Duration, fraction float64) time.Duration {
	index := int(float64(len(samples))*fraction+0.999999999) - 1
	if index < 0 {
		index = 0
	}
	return samples[index]
}

func countZeroDurations(samples []time.Duration) int {
	count := 0
	for _, sample := range samples {
		if sample == 0 {
			count++
		}
	}
	return count
}
