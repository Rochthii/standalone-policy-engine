// Package parser cung cấp bộ phân tích cú pháp cho ngôn ngữ DSL phân quyền.
// Ngôn ngữ DSL có cú pháp tương tự Cedar của AWS:
//
//	permit(principal == user:"alice", action == action:READ, resource == any)
//	when { context.ip_address in "192.168.1.0/24" && context.request_time >= "08:00:00Z" };
package parser

import "net"

// ---------------------------------------------------------------------------
// NodeType — định danh kiểu của mỗi Node trên cây AST
// ---------------------------------------------------------------------------

// NodeType là kiểu định danh cho từng loại node trên cây AST.
type NodeType uint8

const (
	NodePolicy     NodeType = iota + 1 // Gốc của chính sách (PolicyNode)
	NodeScope                          // Phạm vi so khớp tĩnh (ScopeNode)
	NodeBinaryExpr                     // Toán tử hai ngôi (BinaryExprNode)
	NodeUnaryExpr                      // Toán tử một ngôi (UnaryExprNode)
	NodeVariable                       // Biến tham chiếu ngữ cảnh (VariableNode)
	NodeValue                          // Hằng số literal (ValueNode)
)

// String trả về tên dạng chuỗi của NodeType (hỗ trợ debug).
func (t NodeType) String() string {
	switch t {
	case NodePolicy:
		return "Policy"
	case NodeScope:
		return "Scope"
	case NodeBinaryExpr:
		return "BinaryExpr"
	case NodeUnaryExpr:
		return "UnaryExpr"
	case NodeVariable:
		return "Variable"
	case NodeValue:
		return "Value"
	default:
		return "Unknown"
	}
}

// ---------------------------------------------------------------------------
// Node — interface gốc của tất cả các node trên cây AST
// ---------------------------------------------------------------------------

// Node đại diện cho một nút bất kỳ trên cây AST.
// Mọi struct cụ thể đều phải implement interface này.
// Cây AST là bất biến (Immutable) sau khi Parser & Compiler hoàn tất.
// Các luồng Data Plane (gRPC CheckAccess) tuyệt đối không được sửa đổi node này.
type Node interface {
	// Type trả về kiểu định danh của node.
	Type() NodeType
	// Pos trả về vị trí token đầu tiên của node trong chuỗi nguồn (hỗ trợ báo lỗi).
	Pos() Position
}

// ---------------------------------------------------------------------------
// Effect — hiệu lực quyết định của chính sách
// ---------------------------------------------------------------------------

// Effect biểu diễn hiệu lực của quyết định phân quyền.
type Effect uint8

const (
	EffectPermit Effect = iota + 1 // permit — cho phép truy cập
	EffectForbid                   // forbid — cấm truy cập tường minh
)

// String trả về chuỗi đại diện của Effect.
func (e Effect) String() string {
	if e == EffectPermit {
		return "permit"
	}
	return "forbid"
}

// ---------------------------------------------------------------------------
// ScopeOp — toán tử so khớp trong phần Scope tĩnh
// ---------------------------------------------------------------------------

// ScopeOp là toán tử so khớp tĩnh dùng trong phần Scope của câu luật.
type ScopeOp uint8

const (
	ScopeOpEq  ScopeOp = iota + 1 // == (so khớp tuyệt đối)
	ScopeOpIn                      // in (so khớp phân cấp vai trò)
	ScopeOpAny                     // any (áp dụng cho tất cả)
)

// ---------------------------------------------------------------------------
// ScopeNode — phạm vi so khớp tĩnh (principal, action, resource)
// ---------------------------------------------------------------------------

// ScopeNode mô tả một điều khoản phạm vi tĩnh của câu luật.
// Ví dụ: principal == user:"alice"  →  {Operator: ScopeOpEq, EntityType: "user", EntityID: "alice"}
// Ví dụ: action in role:admin       →  {Operator: ScopeOpIn, EntityType: "role", EntityID: "admin"}
// Ví dụ: resource == any            →  {Operator: ScopeOpAny}
type ScopeNode struct {
	Operator   ScopeOp  // Toán tử so khớp
	EntityType string   // Loại thực thể, ví dụ: "user", "role", "action", "file"
	EntityID   string   // ID thực thể, ví dụ: "alice", "admin", "READ"
	pos        Position // Vị trí trong chuỗi nguồn
}

func (s *ScopeNode) Type() NodeType { return NodeScope }
func (s *ScopeNode) Pos() Position  { return s.pos }

// ---------------------------------------------------------------------------
// PolicyNode — root node của mỗi câu chính sách
// ---------------------------------------------------------------------------

// PolicyNode là nút gốc (Root Node) đại diện cho một câu chính sách hoàn chỉnh.
// Mỗi file chính sách có thể chứa nhiều PolicyNode (một mảng các PolicyNode).
type PolicyNode struct {
	ID        string     // Mã định danh duy nhất của chính sách (tự sinh hoặc do Control Plane cấp)
	TenantID  string     // ID của Tenant sở hữu chính sách này (đảm bảo cô lập đa khách thuê)
	Effect    Effect     // Hiệu lực: EffectPermit hoặc EffectForbid
	Principal *ScopeNode // Điều khoản phạm vi principal
	Action    *ScopeNode // Điều khoản phạm vi action
	Resource  *ScopeNode // Điều khoản phạm vi resource
	Condition Node       // Biểu thức điều kiện when/unless (nil nếu không có mệnh đề điều kiện)
	IsUnless  bool       // true nếu dùng từ khóa "unless" (đảo logic), false nếu "when"
	pos       Position
}

func (p *PolicyNode) Type() NodeType { return NodePolicy }
func (p *PolicyNode) Pos() Position  { return p.pos }

// ---------------------------------------------------------------------------
// BinaryExprNode — toán tử hai ngôi
// ---------------------------------------------------------------------------

// BinaryOp định nghĩa các toán tử hai ngôi được hỗ trợ trong mệnh đề điều kiện.
type BinaryOp uint8

const (
	BinOpAnd      BinaryOp = iota + 1 // &&
	BinOpOr                           // ||
	BinOpEq                           // ==
	BinOpNe                           // !=
	BinOpGt                           // >
	BinOpLt                           // <
	BinOpGe                           // >=
	BinOpLe                           // <=
	BinOpIn                           // in (kiểm tra tập hợp / dải IP / phân cấp vai trò)
	BinOpContains                     // contains (kiểm tra mảng)
)

// BinaryExprNode đại diện cho một biểu thức với toán tử hai ngôi.
// Ví dụ: context.ip_address in "192.168.1.0/24"
// Ví dụ: context.age >= 18 && context.verified == true
type BinaryExprNode struct {
	Op    BinaryOp // Toán tử
	Left  Node     // Biểu thức con bên trái
	Right Node     // Biểu thức con bên phải
	pos   Position
}

func (b *BinaryExprNode) Type() NodeType { return NodeBinaryExpr }
func (b *BinaryExprNode) Pos() Position  { return b.pos }

// ---------------------------------------------------------------------------
// UnaryExprNode — toán tử một ngôi
// ---------------------------------------------------------------------------

// UnaryOp định nghĩa các toán tử một ngôi được hỗ trợ.
type UnaryOp uint8

const (
	UnaryOpNot UnaryOp = iota + 1 // ! (phủ định logic)
	UnaryOpNeg                     // - (phủ định số học, dùng cho Integer âm)
)

// UnaryExprNode đại diện cho biểu thức phủ định.
// Ví dụ: !(context.device_status == "compromised")
type UnaryExprNode struct {
	Op    UnaryOp // Toán tử
	Child Node    // Biểu thức con
	pos   Position
}

func (u *UnaryExprNode) Type() NodeType { return NodeUnaryExpr }
func (u *UnaryExprNode) Pos() Position  { return u.pos }

// ---------------------------------------------------------------------------
// VariableNode — biến tham chiếu thuộc tính động
// ---------------------------------------------------------------------------

// VarScope xác định không gian tên của biến.
type VarScope uint8

const (
	VarScopePrincipal VarScope = iota + 1 // principal.attr
	VarScopeResource                       // resource.attr
	VarScopeContext                        // context.attr
)

// VariableNode đại diện cho một biến tham chiếu thuộc tính động từ context.
// Ví dụ: context.ip_address  →  {Scope: VarScopeContext, Field: "ip_address"}
// Ví dụ: principal.role      →  {Scope: VarScopePrincipal, Field: "role"}
// Ví dụ: resource.owner_id   →  {Scope: VarScopeResource, Field: "owner_id"}
type VariableNode struct {
	Scope VarScope // Không gian tên của biến
	Field string   // Tên thuộc tính
	pos   Position
}

func (v *VariableNode) Type() NodeType { return NodeVariable }
func (v *VariableNode) Pos() Position  { return v.pos }

// ---------------------------------------------------------------------------
// ValueType — kiểu dữ liệu của hằng số
// ---------------------------------------------------------------------------

// ValueType phân loại kiểu dữ liệu của một hằng số (literal).
type ValueType uint8

const (
	ValueTypeString   ValueType = iota + 1 // Chuỗi ký tự (String)
	ValueTypeInt                           // Số nguyên (Integer)
	ValueTypeBool                          // Logic (Boolean)
	ValueTypeIP                            // Địa chỉ IP đơn lẻ (IPv4)
	ValueTypeIPNet                         // Dải mạng CIDR (IPv4 Network) — đã pre-parsed
	ValueTypeDateTime                      // Thời gian ISO 8601 — đã chuyển sang Unix nanosecond
)

// ValueNode đại diện cho một giá trị hằng số (Literal) trong biểu thức điều kiện.
//
// Sau khi Compiler xử lý:
//   - IPNet được lưu dưới dạng net.IPNet đã phân tích, so sánh runtime chỉ dùng bitwise.
//   - DateTime được lưu dưới dạng int64 Unix nanosecond, so sánh runtime chỉ dùng số nguyên.
//   - String, Int, Bool được lưu trực tiếp.
//
// Thiết kế: Dùng struct cụ thể để tránh interface boxing và giảm GC pressure.
type ValueNode struct {
	ValType ValueType

	// Chỉ một trong các trường bên dưới có giá trị tùy theo ValType.
	// Không dùng interface{} để tránh heap allocation không cần thiết.
	StrVal      string   // ValType == ValueTypeString
	IntVal      int64    // ValType == ValueTypeInt hoặc ValueTypeDateTime (Unix ns)
	BoolVal     bool     // ValType == ValueTypeBool
	IPVal       net.IP   // ValType == ValueTypeIP (đã parse)
	IPNetVal    net.IPNet // ValType == ValueTypeIPNet (đã pre-parse, dùng cho Contains/In check)

	pos Position
}

func (v *ValueNode) Type() NodeType { return NodeValue }
func (v *ValueNode) Pos() Position  { return v.pos }

// ---------------------------------------------------------------------------
// Position — vị trí trong chuỗi nguồn (hỗ trợ báo lỗi chính xác)
// ---------------------------------------------------------------------------

// Position lưu trữ vị trí của một token trong chuỗi nguồn gốc.
// Được sử dụng để tạo thông điệp lỗi có thông tin dòng/cột giúp người viết luật
// xác định chính xác vị trí lỗi cú pháp.
type Position struct {
	Line   int // Số thứ tự dòng (bắt đầu từ 1)
	Column int // Số thứ tự cột trên dòng đó (bắt đầu từ 1)
	Offset int // Byte offset tuyệt đối tính từ đầu chuỗi nguồn
}
