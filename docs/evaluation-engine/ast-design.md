# AST (Abstract Syntax Tree) Design

Tài liệu này đặc tả thiết kế cây cú pháp trừu tượng AST (Abstract Syntax Tree) cho **Standalone Policy Engine**, cấu trúc các node và các kiểu biểu thức logic trong bộ nhớ RAM.

---

## 1. Biểu diễn cấu trúc Node trong Go

Cây AST được biểu diễn thông qua một interface gốc `Node` và các struct cụ thể đại diện cho từng loại toán tử:

```go
package ast

// Node đại diện cho một nút bất kỳ trên cây AST
type Node interface {
	Type() NodeType
}

// NodeType xác định kiểu của Node
type NodeType string

const (
	NodePolicy      NodeType = "Policy"
	NodeScope       NodeType = "Scope"
	NodeBinaryExpr  NodeType = "BinaryExpr"
	NodeUnaryExpr   NodeType = "UnaryExpr"
	NodeVariable    NodeType = "Variable"
	NodeValue       NodeType = "Value"
)

// PolicyNode là Root Node của mỗi tệp luật
type PolicyNode struct {
	Effect    string      // "permit" hoặc "forbid"
	Principal ScopeNode   // So khớp principal
	Action    ScopeNode   // So khớp action
	Resource  ScopeNode   // So khớp resource
	Condition Node        // Biểu thức điều kiện when/unless (nil nếu không có)
}

// ScopeNode mô tả phạm vi so khớp
type ScopeNode struct {
	Operator string // "==" hoặc "in" hoặc "any"
	Value    string // ví dụ "user:alice", "role:admin"
}

// BinaryExprNode đại diện cho toán tử hai ngôi (&&, ||, ==, in...)
type BinaryExprNode struct {
	Op    string // "&&", "||", "==", "in", "contains", ...
	Left  Node   // Node con bên trái
	Right Node   // Node con bên phải
}

// UnaryExprNode đại diện cho toán tử một ngôi (!, -)
type UnaryExprNode struct {
	Op    string // "!", "-"
	Child Node   
}

// VariableNode đại diện cho biến tham chiếu ngữ cảnh (principal.role, context.ip)
type VariableNode struct {
	Scope string // "principal", "resource", "context"
	Field string // ví dụ "role", "ip_address"
}

// ValueNode đại diện cho các giá trị hằng (Literal Values)
type ValueNode struct {
	ValType string      // "string", "int", "bool", "ip", "datetime"
	Value   interface{} // Giá trị thực tế đã parse
}
```

---

## 2. Thiết kế Bộ nhớ tối ưu (Memory Optimization)

Để giảm tải cho bộ dọn rác (GC) của Go khi chạy hàng triệu luồng đánh giá song song, cấu trúc AST áp dụng các quy tắc sau:
1.  **Read-Only AST:** Sau khi cây AST được tạo ở Control Plane, nó là **bất biến (Immutable)**. Các luồng xử lý Data Plane tuyệt đối không được phép chỉnh sửa dữ liệu trên các node AST này.
2.  **No Pointers where possible:** Sử dụng các struct phẳng thay vì lạm dụng pointer để giảm sự phân tán ô nhớ trên Heap, tối ưu CPU cache line.
3.  **sync.Pool cho Context Evaluation:** Khi đánh giá AST, ta chỉ cần sinh các context map tạm thời. Context này được quản lý qua `sync.Pool` để tái sử dụng ô nhớ.
