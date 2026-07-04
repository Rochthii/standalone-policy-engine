// Code sinh tay giả lập cấu trúc sinh ra bởi protoc-gen-go để đảm bảo dự án có thể build được ngay
// mà không bắt buộc cài đặt protoc trên máy cục bộ.
package policyv1

import (
	"encoding/json"
	"google.golang.org/grpc/encoding"
)

type CheckAccessRequest struct {
	TenantId string            `json:"tenant_id,omitempty"`
	Subject  string            `json:"subject,omitempty"`
	Action   string            `json:"action,omitempty"`
	Resource string            `json:"resource,omitempty"`
	Context  map[string]string `json:"context,omitempty"`
}

type CheckAccessResponse_Decision int32

const (
	CheckAccessResponse_DENY  CheckAccessResponse_Decision = 0
	CheckAccessResponse_ALLOW CheckAccessResponse_Decision = 1
)

type CheckAccessResponse struct {
	Decision        CheckAccessResponse_Decision `json:"decision,omitempty"`
	MatchedPolicyId string                       `json:"matched_policy_id,omitempty"`
}

type ExplainRequest struct {
	TenantId string            `json:"tenant_id,omitempty"`
	Subject  string            `json:"subject,omitempty"`
	Action   string            `json:"action,omitempty"`
	Resource string            `json:"resource,omitempty"`
	Context  map[string]string `json:"context,omitempty"`
}

type ExplainResponse_Decision int32

const (
	ExplainResponse_DENY  ExplainResponse_Decision = 0
	ExplainResponse_ALLOW ExplainResponse_Decision = 1
)

type ExplainResponse struct {
	Decision    ExplainResponse_Decision `json:"decision,omitempty"`
	FinalReason string                   `json:"final_reason,omitempty"`
	Matched     []*PolicyMetadata        `json:"matched,omitempty"`
}

type PolicyMetadata struct {
	PolicyId   string `json:"policy_id,omitempty"`
	Effect     string `json:"effect,omitempty"`
	PolicyText string `json:"policy_text,omitempty"`
}

// Các phương thức tương thích protobuf interface cơ bản để có thể compile
func (x *CheckAccessRequest) Reset()         { *x = CheckAccessRequest{} }
func (x *CheckAccessRequest) String() string { return "" }
func (*CheckAccessRequest) ProtoMessage()    {}

func (x *CheckAccessResponse) Reset()         { *x = CheckAccessResponse{} }
func (x *CheckAccessResponse) String() string { return "" }
func (*CheckAccessResponse) ProtoMessage()    {}

func (x *ExplainRequest) Reset()         { *x = ExplainRequest{} }
func (x *ExplainRequest) String() string { return "" }
func (*ExplainRequest) ProtoMessage()    {}

func (x *ExplainResponse) Reset()         { *x = ExplainResponse{} }
func (x *ExplainResponse) String() string { return "" }
func (*ExplainResponse) ProtoMessage()    {}

func (x *PolicyMetadata) Reset()         { *x = PolicyMetadata{} }
func (x *PolicyMetadata) String() string { return "" }
func (*PolicyMetadata) ProtoMessage()    {}

type jsonCodec struct{}

func (jsonCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func (jsonCodec) Name() string {
	return "json"
}

func init() {
	encoding.RegisterCodec(jsonCodec{})
}
