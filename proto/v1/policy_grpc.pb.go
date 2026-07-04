// Code sinh tay giả lập cấu trúc sinh ra bởi protoc-gen-go-grpc để đảm bảo dự án có thể build được ngay
// mà không bắt buộc cài đặt protoc trên máy cục bộ.
package policyv1

import (
	"context"
	"google.golang.org/grpc"
)

// PolicyDecisionPointClient là client API cho dịch vụ PolicyDecisionPoint.
type PolicyDecisionPointClient interface {
	CheckAccess(ctx context.Context, in *CheckAccessRequest, opts ...grpc.CallOption) (*CheckAccessResponse, error)
	ExplainDecision(ctx context.Context, in *ExplainRequest, opts ...grpc.CallOption) (*ExplainResponse, error)
}

type policyDecisionPointClient struct {
	cc grpc.ClientConnInterface
}

func NewPolicyDecisionPointClient(cc grpc.ClientConnInterface) PolicyDecisionPointClient {
	return &policyDecisionPointClient{cc}
}

func (c *policyDecisionPointClient) CheckAccess(ctx context.Context, in *CheckAccessRequest, opts ...grpc.CallOption) (*CheckAccessResponse, error) {
	out := new(CheckAccessResponse)
	err := c.cc.Invoke(ctx, "/policy.v1.PolicyDecisionPoint/CheckAccess", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (c *policyDecisionPointClient) ExplainDecision(ctx context.Context, in *ExplainRequest, opts ...grpc.CallOption) (*ExplainResponse, error) {
	out := new(ExplainResponse)
	err := c.cc.Invoke(ctx, "/policy.v1.PolicyDecisionPoint/ExplainDecision", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// PolicyDecisionPointServer là server API cho dịch vụ PolicyDecisionPoint.
type PolicyDecisionPointServer interface {
	CheckAccess(context.Context, *CheckAccessRequest) (*CheckAccessResponse, error)
	ExplainDecision(context.Context, *ExplainRequest) (*ExplainResponse, error)
}

// UnimplementedPolicyDecisionPointServer có thể được nhúng để có forward compatibility.
type UnimplementedPolicyDecisionPointServer struct{}

func (UnimplementedPolicyDecisionPointServer) CheckAccess(context.Context, *CheckAccessRequest) (*CheckAccessResponse, error) {
	return nil, grpc.Errorf(grpc.Code(grpc.Codes(grpc.Codes(12))), "method CheckAccess not implemented") // codes.Unimplemented
}

func (UnimplementedPolicyDecisionPointServer) ExplainDecision(context.Context, *ExplainRequest) (*ExplainResponse, error) {
	return nil, grpc.Errorf(grpc.Code(grpc.Codes(grpc.Codes(12))), "method ExplainDecision not implemented")
}

func RegisterPolicyDecisionPointServer(s grpc.ServiceRegistrar, srv PolicyDecisionPointServer) {
	s.RegisterService(&PolicyDecisionPoint_ServiceDesc, srv)
}

var PolicyDecisionPoint_ServiceDesc = grpc.ServiceDesc{
	ServiceName: "policy.v1.PolicyDecisionPoint",
	HandlerType: (*PolicyDecisionPointServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "CheckAccess",
			Handler:    _PolicyDecisionPoint_CheckAccess_Handler,
		},
		{
			MethodName: "ExplainDecision",
			Handler:    _PolicyDecisionPoint_ExplainDecision_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "proto/v1/policy.proto",
}

func _PolicyDecisionPoint_CheckAccess_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(CheckAccessRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PolicyDecisionPointServer).CheckAccess(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/policy.v1.PolicyDecisionPoint/CheckAccess",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PolicyDecisionPointServer).CheckAccess(ctx, req.(*CheckAccessRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _PolicyDecisionPoint_ExplainDecision_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(ExplainRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(PolicyDecisionPointServer).ExplainDecision(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/policy.v1.PolicyDecisionPoint/ExplainDecision",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(PolicyDecisionPointServer).ExplainDecision(ctx, req.(*ExplainRequest))
	}
	return interceptor(ctx, in, info, handler)
}
