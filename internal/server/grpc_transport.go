package server

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"standalone-policy-engine/internal/audit"
	"standalone-policy-engine/internal/config"
	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/security"
	policyv1 "standalone-policy-engine/proto/v1"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

func StartGRPCServer(lis net.Listener, eng *engine.EngineWithGC, logger *audit.AuditLogger, securityConfig config.SecurityConfig, serverConfig config.ServerConfig) (*grpc.Server, error) {
	grpcServer, _, err := StartGRPCServerWithRevocations(context.Background(), lis, eng, logger, securityConfig, serverConfig, nil)
	return grpcServer, err
}

func StartGRPCServerWithRevocations(ctx context.Context, lis net.Listener, eng *engine.EngineWithGC, logger *audit.AuditLogger, securityConfig config.SecurityConfig, serverConfig config.ServerConfig, revocationStore security.RevocationStore) (*grpc.Server, *security.RevocationSyncer, error) {
	options := []grpc.ServerOption{
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     15 * time.Second,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 5 * time.Second,
			Time:                  5 * time.Second,
			Timeout:               time.Second,
		}),
		grpc.UnaryInterceptor(traceInterceptor),
		grpc.MaxRecvMsgSize(serverConfig.GRPCMaxReceiveBytes),
		grpc.MaxSendMsgSize(serverConfig.GRPCMaxSendBytes),
	}

	if securityConfig.TLSCertFile != "" && securityConfig.TLSKeyFile != "" && securityConfig.TLSCAFile != "" {
		credentials, err := loadMTLSServerCredentials(
			securityConfig.TLSCertFile,
			securityConfig.TLSKeyFile,
			securityConfig.TLSCAFile,
		)
		if err != nil {
			return nil, nil, fmt.Errorf("cấu hình mTLS thất bại: %w", err)
		}
		options = append(options, grpc.Creds(credentials))
		log.Printf("[PDP-Server] mTLS enabled with cert=%s ca=%s", securityConfig.TLSCertFile, securityConfig.TLSCAFile)
	} else {
		log.Println("[PDP-Server] WARNING: insecure transport is allowed only outside production")
	}

	service, err := newGRPCServerWithSecurity(eng, logger, securityConfig, serverConfig, revocationStore)
	if err != nil {
		return nil, nil, fmt.Errorf("cấu hình delegation key ring thất bại: %w", err)
	}
	var revocationSyncer *security.RevocationSyncer
	if revocationStore != nil {
		revocationSyncer = security.NewRevocationSyncer(service.delegationMgr, revocationStore)
		if err := revocationSyncer.Start(ctx); err != nil {
			return nil, nil, fmt.Errorf("khởi tạo đồng bộ revocation thất bại: %w", err)
		}
	}
	grpcServer := grpc.NewServer(options...)
	policyv1.RegisterPolicyDecisionPointServer(
		grpcServer,
		service,
	)
	go func() { _ = grpcServer.Serve(lis) }()
	return grpcServer, revocationSyncer, nil
}

func traceInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	metadataValues, ok := metadata.FromIncomingContext(ctx)
	traceID := "none"
	if ok {
		if value := metadataValues.Get("x-trace-id"); len(value) > 0 {
			traceID = value[0]
		} else if value := metadataValues.Get("traceparent"); len(value) > 0 {
			parts := strings.Split(value[0], "-")
			if len(parts) >= 2 {
				traceID = parts[1]
			}
		}
	}
	if traceID != "none" {
		log.Printf("[Trace-Context] request=%s trace_id=%s", info.FullMethod, traceID)
	}
	return handler(ctx, req)
}

func loadMTLSServerCredentials(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
	serverCert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("không thể tải server cert/key (%s, %s): %w", certFile, keyFile, err)
	}
	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("không thể đọc CA cert file %s: %w", caFile, err)
	}
	certPool := x509.NewCertPool()
	if !certPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("CA cert từ %s không hợp lệ hoặc không phải PEM format", caFile)
	}
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientCAs:    certPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS12,
	}), nil
}
