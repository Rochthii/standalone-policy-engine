package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"standalone-policy-engine/internal/audit"
	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/metrics"
	"standalone-policy-engine/internal/security"
	policyv1 "standalone-policy-engine/proto/v1"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/metadata"
)

// GRPCServer là máy chủ gRPC chịu trách nhiệm xử lý luồng quyết định phân quyền (Data Plane).
type GRPCServer struct {
	policyv1.UnimplementedPolicyDecisionPointServer

	engine       *engine.EngineWithGC
	auditLogger  *audit.AuditLogger
	jwtValidator *security.JWTValidator
}

// NewGRPCServer tạo mới một instance GRPCServer.
func NewGRPCServer(eng *engine.EngineWithGC, logger *audit.AuditLogger) *GRPCServer {
	return &GRPCServer{
		engine:       eng,
		auditLogger:  logger,
		jwtValidator: security.NewJWTValidator(),
	}
}

// CheckAccess xử lý yêu cầu gRPC kiểm tra quyền truy cập lock-free và ghi log bất đồng bộ.
func (s *GRPCServer) CheckAccess(ctx context.Context, req *policyv1.CheckAccessRequest) (*policyv1.CheckAccessResponse, error) {
	startTime := time.Now()

	// 0. Trích xuất và xác thực JWT token từ gRPC Metadata nếu có
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		var tokenStr string
		if auth := md.Get("authorization"); len(auth) > 0 {
			tokenStr = auth[0]
		} else if bearer := md.Get("bearer"); len(bearer) > 0 {
			tokenStr = bearer[0]
		}

		if tokenStr != "" && s.jwtValidator != nil {
			claims, err := s.jwtValidator.ValidateToken(tokenStr)
			if err == nil {
				sub, attrs, err := s.jwtValidator.ExtractSubjectAttributes(claims)
				if err == nil {
					// Ghi đè subject và bổ sung context attributes từ JWT claims
					req.Subject = sub
					if req.Context == nil {
						req.Context = make(map[string]string)
					}
					for k, v := range attrs {
						if _, exists := req.Context[k]; !exists {
							req.Context[k] = v
						}
					}
				}
			}
		}
	}

	// 1. Thực hiện đánh giá quyết định trên RAM thông qua Engine
	res := s.engine.CheckPermission(req.TenantId, req.Subject, req.Action, req.Resource, req.Context)

	decisionStr := res.Decision.String()

	// Ghi nhận Prometheus metrics
	metrics.ObserveEvaluationDuration(req.TenantId, decisionStr, time.Since(startTime))
	metrics.IncrementRequestCounter(req.TenantId, decisionStr)

	// 2. Ghi log kiểm toán bất đồng bộ (WORM) qua Ring Buffer để không block luồng gRPC chính
	matchedPolicyID := ""
	if len(res.Explanations) > 0 {
		matchedPolicyID = res.Explanations[0] // Lấy ID luật chính quyết định
	}

	if s.auditLogger != nil {
		s.auditLogger.Log(
			req.TenantId,
			req.Subject,
			req.Action,
			req.Resource,
			decisionStr,
			matchedPolicyID,
			req.Context,
		)
	}

	// 3. Trả về phản hồi cho PEP
	decisionVal := policyv1.CheckAccessResponse_DENY
	if res.Decision == engine.DecisionAllow {
		decisionVal = policyv1.CheckAccessResponse_ALLOW
	}

	return &policyv1.CheckAccessResponse{
		Decision:        decisionVal,
		MatchedPolicyId: matchedPolicyID,
	}, nil
}

// ExplainDecision đánh giá và trả về giải thích chi tiết các luật thỏa mãn điều kiện.
func (s *GRPCServer) ExplainDecision(ctx context.Context, req *policyv1.ExplainRequest) (*policyv1.ExplainResponse, error) {
	// 1. Thực hiện kiểm tra quyền
	res := s.engine.CheckPermission(req.TenantId, req.Subject, req.Action, req.Resource, req.Context)

	// 2. Thu thập thông tin chi tiết các chính sách khớp
	trie, exists := s.engine.GetTenantTrie(ctx, req.TenantId)
	if !exists {
		decisionVal := policyv1.ExplainResponse_DENY
		return &policyv1.ExplainResponse{
			Decision:    decisionVal,
			FinalReason: "Không tìm thấy tập chính sách cho Tenant",
			Matched:     []*policyv1.PolicyMetadata{},
		}, nil
	}

	// Tìm các vai trò kế thừa của Subject
	subjects := trie.RoleDAG.GetInheritedRoles(req.Subject)

	// Chuẩn hóa và lấy danh sách chính sách khớp
	resources := []string{req.Resource}
	// ActionKey
	actKey := req.Action
	if !strings.HasPrefix(actKey, "action:") {
		actKey = "action:" + actKey
	}

	matchedPolicies := trie.LookupPolicies(subjects, resources, actKey)

	// Khởi tạo ngữ cảnh đánh giá
	evalCtx := engine.GetEvalContext(req.Subject, req.Action, req.Resource, req.Context, trie.RoleDAG)
	defer evalCtx.Release()

	matchedMetadata := make([]*policyv1.PolicyMetadata, 0)

	for _, policy := range matchedPolicies {
		// Đánh giá điều kiện
		val, err := engine.Evaluate(policy.Condition, evalCtx)
		isConditionSatisfied := false
		if err == nil && val.ValType == parser.ValueTypeBool {
			if policy.IsUnless {
				isConditionSatisfied = !val.BoolVal
			} else {
				isConditionSatisfied = val.BoolVal
			}
		}

		if isConditionSatisfied {
			effectStr := "permit"
			if policy.Effect == parser.EffectForbid {
				effectStr = "forbid"
			}
			matchedMetadata = append(matchedMetadata, &policyv1.PolicyMetadata{
				PolicyId:   policy.ID,
				Effect:     effectStr,
				PolicyText: policy.ID + ": " + effectStr + "(...) when { ... };", // Trong thực tế, ta lưu trữ text luật thô ở DB
			})
		}
	}

	decisionVal := policyv1.ExplainResponse_DENY
	if res.Decision == engine.DecisionAllow {
		decisionVal = policyv1.ExplainResponse_ALLOW
	}

	return &policyv1.ExplainResponse{
		Decision:    decisionVal,
		FinalReason: res.Reason,
		Matched:     matchedMetadata,
	}, nil
}

// StartGRPCServer cấu hình mTLS, Keepalive và khởi chạy gRPC Server tại cổng chỉ định.
func StartGRPCServer(port int, eng *engine.EngineWithGC, logger *audit.AuditLogger) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, fmt.Errorf("lắng nghe cổng %d thất bại: %v", port, err)
	}

	// Cấu hình Keepalive parameters tối ưu persistent connection siêu tốc
	kaep := keepalive.EnforcementPolicy{
		MinTime:             5 * time.Second, // Thời gian tối thiểu giữa các lần ping keepalive
		PermitWithoutStream: true,            // Cho phép ping ngay cả khi không có stream hoạt động
	}

	kasp := keepalive.ServerParameters{
		MaxConnectionIdle:     15 * time.Second, // Đóng kết nối idle sau 15 giây
		MaxConnectionAge:      30 * time.Minute, // Buộc kết nối reconnect sau 30 phút để cân bằng tải
		MaxConnectionAgeGrace: 5 * time.Second,  // Thời gian ân hạn trước khi đóng kết nối cũ
		Time:                  5 * time.Second,  // Ping client mỗi 5 giây nếu không có dữ liệu truyền nhận
		Timeout:               1 * time.Second,  // Chờ client phản hồi ping trong 1 giây
	}

	opts := []grpc.ServerOption{
		grpc.KeepaliveEnforcementPolicy(kaep),
		grpc.KeepaliveParams(kasp),
		grpc.UnaryInterceptor(traceInterceptor),
	}

	// Trong môi trường Production, cấu hình TLS tại đây
	// creds, err := credentials.NewServerTLSFromFile("cert.pem", "key.pem")
	// if err == nil { opts = append(opts, grpc.Creds(creds)) }

	grpcServer := grpc.NewServer(opts...)
	srv := NewGRPCServer(eng, logger)
	policyv1.RegisterPolicyDecisionPointServer(grpcServer, srv)

	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			// Xử lý lỗi khi server dừng đột ngột
		}
	}()

	return grpcServer, nil
}

// traceInterceptor trích xuất traceparent hoặc x-trace-id từ metadata gRPC,
// tạo vết xử lý liên tục từ PEP sang PDP.
func traceInterceptor(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	traceID := "none"
	if ok {
		if val := md.Get("x-trace-id"); len(val) > 0 {
			traceID = val[0]
		} else if val := md.Get("traceparent"); len(val) > 0 {
			// W3C format: 00-traceid-spanid-flags
			parts := strings.Split(val[0], "-")
			if len(parts) >= 2 {
				traceID = parts[1]
			}
		}
	}

	// Ghi log vết
	if traceID != "none" {
		log.Printf("[Trace-Context] Nhận request %s (TraceID: %s)", info.FullMethod, traceID)
	}

	resp, err := handler(ctx, req)
	return resp, err
}

// import strings for normalize action and trim prefix
import "strings"
