# Execution Engine Specification

Tài liệu này đặc tả chi tiết thuật toán hoạt động của bộ thực thi quyết định phân quyền (Execution Engine) trên bộ nhớ RAM của **Standalone Policy Engine**, đã được tối ưu hóa sau kiểm duyệt kiến trúc.

---

## 1. Kiến trúc Chỉ mục bộ nhớ (In-Memory Radix/Trie Indexer)

Để tránh việc duyệt tuyến tính qua hàng chục nghìn quy tắc phân quyền cho mỗi request, Engine tổ chức các chính sách thành một cấu trúc cây chỉ mục nhiều cấp (Multi-level Index Trie) trên RAM:

```mermaid
graph TD
    Root[Trie Root] --> Tenant_A[Tenant: 1111-1111]
    Root --> Tenant_B[Tenant: 2222-2222]

    Tenant_A --> Sub_User[Subject: user:*]
    Tenant_A --> Sub_Role[Subject: role:admin]
    Tenant_A --> Sub_Wildcard[Subject: any - Global Rules Partition]

    Sub_User --> Res_File[Resource: file:*]
    Sub_User --> Res_Any[Resource: any]

    Res_File --> Act_Read[Action: READ -> [P-001, P-002]]
    Res_File --> Act_Delete[Action: DELETE -> [P-003]]
```

### Thuật toán tối ưu hóa Wildcard (Global Rules Partitioning):
Để ngăn ngừa hiện tượng ô nhiễm Trie (Trie Pollution) làm suy giảm hiệu năng từ $O(\log N)$ về $O(N)$ khi có quá nhiều luật wildcard:
1.  **Phân vùng Luật Toàn cục (Global Rules):** Các chính sách có `principal == any` và `resource == any` sẽ được bóc tách và lưu trữ riêng tại một phân vùng đặc biệt gọi là **`Global Rules Partition`**.
2.  **Hai bước đánh giá:** Khi request đi vào:
    *   *Bước 1:* PDP tra cứu nhanh trên Radix Trie để lấy các chính sách cụ thể của Tenant/Subject/Resource $\rightarrow$ Thu được mảng `TriePolicies`.
    *   *Bước 2:* PDP gộp mảng `TriePolicies` với mảng `GlobalPolicies` từ phân vùng toàn cục $\rightarrow$ Tiến hành evaluate.
3.  Cơ chế này giúp giữ cho cấu trúc Trie luôn sạch sẽ, gọn nhẹ và tối ưu hóa thời gian tra cứu.

---

## 2. Thuật toán đánh giá AST (AST Evaluation Algorithm)

Evaluator thực hiện duyệt đệ quy cây AST và áp dụng tối ưu hóa đoản mạch logic:

```go
func Evaluate(node Node, ctx *EvalContext) (Value, error) {
    switch n := node.(type) {
    case *ValueNode:
        return n.ToValue(), nil
        
    case *VariableNode:
        val, found := ctx.GetAttribute(n.Scope, n.Field)
        if !found {
            // Xử lý an toàn khi thiếu thuộc tính: Trả về lỗi có kiểm soát
            return nil, ErrMissingAttribute
        }
        return val, nil
        
    case *BinaryExprNode:
        leftVal, err := Evaluate(n.Left, ctx)
        if err != nil {
            return nil, err
        }
        
        // Đoản mạch logic (Short-Circuit Evaluation)
        if n.Op == "&&" && !leftVal.ToBool() {
            return NewBoolValue(false), nil
        }
        if n.Op == "||" && leftVal.ToBool() {
            return NewBoolValue(true), nil
        }
        
        rightVal, err := Evaluate(n.Right, ctx)
        if err != nil {
            return nil, err
        }
        
        return ExecuteBinaryOp(n.Op, leftVal, rightVal)
    }
    return nil, ErrUnknownNodeType
}
```

---

## 3. Quản lý Concurrency Thread-Safe không khóa nghẽn

Hệ thống sử dụng cơ chế **Hoán đổi con trỏ Copy-On-Write (COW Root Pointer Swap)** để đảm bảo an toàn đa luồng:

1.  **Luồng Đọc (gRPC CheckAccess):**
    *   Đọc dữ liệu thông qua con trỏ gốc hiện tại: `root := engine.GetRootPointer()`.
    *   Thực hiện tra cứu Trie và đánh giá AST hoàn toàn không cần lock hay tranh chấp tài nguyên.
2.  **Luồng Ghi (Admin Update Policy):**
    *   Tạo bản sao tạm thời của cây Trie.
    *   Thêm/Sửa/Xóa node chính sách trên cây Trie tạm thời này.
    *   Sử dụng lệnh hoán đổi nguyên tử (Atomic Pointer Swap) của Go để trỏ con trỏ gốc sang cây Trie mới:
        ```go
        atomic.StorePointer(&engine.rootPointer, unsafe.Pointer(newTrieRoot))
        ```
    *   Cơ chế này giúp loại bỏ hoàn toàn việc sử dụng khóa ghi (`Lock()`) dài hạn trên luồng đọc chính, đảm bảo độ trễ đọc luôn ổn định dưới 1ms ngay cả khi có cập nhật chính sách liên tục.
