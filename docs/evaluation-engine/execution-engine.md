# Execution Engine Specification

Tài liệu này đặc tả chi tiết thuật toán hoạt động của bộ thực thi quyết định phân quyền (Execution Engine) trên bộ nhớ RAM của **Standalone Policy Engine**. Đây là tài liệu kỹ thuật cốt lõi quyết định chỉ số độ trễ xử lý `< 0.35ms`.

---

## 1. Kiến trúc Chỉ mục bộ nhớ (In-Memory Radix/Trie Indexer)

Để tránh việc duyệt tuyến tính qua hàng chục nghìn quy tắc phân quyền cho mỗi request, Engine tổ chức các chính sách thành một cấu trúc cây chỉ mục nhiều cấp (Multi-level Index Trie) trên RAM:

```mermaid
graph TD
    Root[Trie Root] --> Tenant_A[Tenant: 1111-1111]
    Root --> Tenant_B[Tenant: 2222-2222]

    Tenant_A --> Sub_User[Subject: user:*]
    Tenant_A --> Sub_Role[Subject: role:admin]

    Sub_User --> Res_File[Resource: file:*]
    Sub_User --> Res_Any[Resource: any]

    Res_File --> Act_Read[Action: READ -> [P-001, P-002]]
    Res_File --> Act_Delete[Action: DELETE -> [P-003]]
```

### Thuật toán tra cứu (Lookup Algorithm):
1.  **Bước 1:** Trích xuất `tenant_id` từ request, tìm nhánh Trie tương ứng với Tenant. Độ phức tạp: $O(1)$ qua Hash Map lookup.
2.  **Bước 2:** So khớp `Subject` (ví dụ: `user:alice`). Indexer sẽ tìm nhánh chính xác `user:alice` và nhánh wildcard `user:*`.
3.  **Bước 3:** Tiếp tục so khớp xuống cấp `Resource` và `Action` theo cơ chế tương tự.
4.  **Kết quả:** Trả về một mảng chứa rất ít chính sách (thường chỉ từ 1 đến 5 chính sách) cần phải chạy Evaluator AST, thay vì quét qua toàn bộ 100,000 chính sách của hệ thống. Độ phức tạp trung bình của luồng tra cứu này là **$O(\log N)$** với $N$ là số lượng đối tượng trong một nhánh Tenant.

---

## 2. Thuật toán đánh giá AST (AST Evaluation Algorithm)

Sau khi có danh sách các chính sách khớp phạm vi, bộ đánh giá (Evaluator) sẽ duyệt qua cây AST của từng chính sách để kiểm tra biểu thức điều kiện:

```go
func Evaluate(node Node, ctx *EvalContext) (Value, error) {
    switch n := node.(type) {
    case *ValueNode:
        return n.ToValue(), nil
        
    case *VariableNode:
        // Lấy thuộc tính động từ Context
        val, found := ctx.GetAttribute(n.Scope, n.Field)
        if !found {
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
        
    case *UnaryExprNode:
        childVal, err := Evaluate(n.Child, ctx)
        if err != nil {
            return nil, err
        }
        return ExecuteUnaryOp(n.Op, childVal)
    }
    return nil, ErrUnknownNodeType
}
```

### Các tối ưu thuật toán:
*   **Đoản mạch logic (Short-Circuit Evaluation):** Nếu biểu thức trái của phép `&&` là `false`, hoặc biểu thức trái của phép `||` là `true`, Evaluator bỏ qua không duyệt tiếp nhánh AST bên phải, giúp tiết kiệm chu kỳ CPU.
*   **Hạn chế đệ quy sâu (Call Stack Overflow mitigation):** Giới hạn độ sâu tối đa của AST là 15 ở bước Control Plane đảm bảo call stack của Go runtime khi duyệt đệ quy luôn nằm trong ngưỡng an toàn tuyệt đối, tránh lỗi Stack Overflow.

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

---

## 4. Tối ưu hóa Bộ dọn rác (Garbage Collector Mitigation)

Để tránh các khoảng dừng đột ngột (GC Pause) của Go runtime làm tăng trễ P99, Engine thực hiện:
*   **`sync.Pool` tái sử dụng bộ nhớ:**
    *   Các struct lưu trữ kết quả trung gian (`EvalContext`, `ValueNode` tạm thời) được duy trì trong một pool dùng chung.
    *   Sau khi phản hồi gRPC hoàn tất, các struct này được reset và trả lại pool qua lệnh `defer ctxPool.Put(ctx)`.
*   **Sử dụng Flat Structs:** Tránh sử dụng con trỏ pointer lồng nhau trong các node AST tĩnh nếu có thể. Điều này giúp Go compiler ưu tiên phân bổ vùng nhớ trên Stack thay vì Heap (Escape Analysis Optimization).
