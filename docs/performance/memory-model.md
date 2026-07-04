# Memory Model Specification

Tài liệu này đặc tả mô hình tổ chức bộ nhớ (Memory Layout), chiến lược phân bổ Heap/Stack và cơ chế dọn dẹp vùng nhớ của **Standalone Policy Engine** chạy trên Go runtime.

---

## 1. Stack vs Heap Allocation Strategy

Trong ngôn ngữ Go, việc cấp phát bộ nhớ lên Heap bắt buộc phải qua Garbage Collector (GC) dọn dẹp, gây ra độ trễ trồi sụt bất thường (Latency Spikes). Mục tiêu của Engine là đạt **`Zero Heap Allocation`** trong luồng Hot Path (CheckPermission):

```text
[CheckPermission Request] 
       │
       ▼ (Go Escape Analysis)
  ┌────────────────────────────────────────────────────────┐
  │  Stack Memory (Tự dọn dẹp khi hàm return - 0ns cost)    │
  │  - local primitive variables (uint32 IP, timestamps)   │
  │  - fixed-size arrays / slices                          │
  └────────────────────────────────────────────────────────┘
       │
       ▼ (Nếu thoát lên Heap)
  ┌────────────────────────────────────────────────────────┐
  │  sync.Pool Reuse (Tái sử dụng - Không kích hoạt GC)    │
  │  - Request Context Maps                                │
  │  - AST evaluation value nodes                          │
  └────────────────────────────────────────────────────────┘
```

### Các quy tắc thiết kế chống Escape lên Heap:
1.  **Sử dụng Slice có kích thước cố định:** Trong các hàm so khớp và duyệt, nếu cần mảng tạm thời, ta sử dụng mảng tĩnh có kích thước xác định (ví dụ: `[8]string` thay vì `[]string`) để Go compiler phân bổ trực tiếp trên Stack frame của goroutine.
2.  **Tránh truyền Pointer làm tham số trả về (Return Pointers):** Các hàm đánh giá con không được trả về con trỏ tới struct tạm thời. Việc trả về struct phẳng (Value Receiver) cho phép compiler copy trực tiếp giá trị trên Stack mà không cần đưa struct lên Heap.
3.  **Sử dụng `sync.Pool` cho các đối tượng lớn:** Đối với các đối tượng phức tạp như Context Map chứa thuộc tính của request, ta dùng `sync.Pool` để quản lý. Trước khi trả về pool, đối tượng được gọi hàm `Reset()` để dọn dẹp dữ liệu cũ nhưng giữ nguyên dung lượng mảng đã cấp phát (cap).

---

## 2. Cấu trúc RAM của Radix Trie Node

Mỗi node trên cây Trie được thiết kế phẳng để tối ưu cache CPU:

```go
type TrieNode struct {
    // Sử dụng map lồng nhau cho các nhánh con (nhưng được khởi tạo lười - lazy init)
    Children map[string]*TrieNode
    
    // Slice chứa con trỏ tới các AST PolicyNode bất biến
    Policies []*ast.PolicyNode
    
    // Bitmask hỗ trợ lọc nhanh theo Action hoặc Tenant
    FilterMask uint64
}
```

*   **Lazy Initialization:** Chỉ khởi tạo map `Children` khi thực sự có nhánh con để tiết kiệm RAM.
*   **Cache Line Alignment:** Cấu trúc struct được sắp xếp theo thứ tự giảm dần kích thước kiểu dữ liệu (8-byte pointers lên trước, 1-byte bools xuống dưới) để căn chỉnh ô nhớ hoàn hảo (Alignment), giúp CPU đọc cấu trúc dữ liệu chỉ trong một chu kỳ truy xuất bus RAM.
