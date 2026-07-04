# Data Flow Specification

Tài liệu này đặc tả chi tiết dòng chảy dữ liệu (Data Flow) trong các kịch bản vận hành chính của **Standalone Policy Engine**.

---

## 1. Luồng nạp và Cập nhật Chính sách (Policy Load & Sync Flow)

```mermaid
graph TD
    Admin[Admin Manager] -->|1. Save Policy DSL| Admin_API[Control Plane API]
    Admin_API -->|2. Validate Syntax & AST| Validator[AST Validator]
    Validator -->|3. Persist to DB| DB[(Persistent Database)]
    DB -->|4. Emit Event| PubSub[Redis PubSub / gRPC Stream]
    PubSub -->|5. Broadcast Update| Engine_1[PDP Pod 1]
    PubSub -->|5. Broadcast Update| Engine_2[PDP Pod 2]
    
    Engine_1 -->|6. Compile AST & Insert Trie| RAM_1[In-Memory RAM Trie 1]
    Engine_2 -->|6. Compile AST & Insert Trie| RAM_2[In-Memory RAM Trie 2]
```

1.  **Lưu trữ:** Quản trị viên gửi luật DSL mới sang Control Plane API.
2.  **Kiểm duyệt:** Cú pháp DSL được biên dịch thử thành AST. Nếu có lỗi, trả về lỗi ngay lập tức cho Admin, chặn đứng việc lưu trữ chính sách hỏng.
3.  **Lưu bền vững:** Chính sách hợp lệ được lưu vào Database bền vững.
4.  **Phát sự kiện:** Hệ thống phát sự kiện cập nhật chính sách sang kênh PubSub.
5.  **Cập nhật nóng:** Các node PDP nhận sự kiện, tự động tải chính sách mới từ DB, compile AST và hoán đổi con trỏ root của RAM Trie một cách an toàn mà không gián đoạn dịch vụ.

---

## 2. Luồng đánh giá Quyết định (Decision Flow)

```mermaid
graph TD
    PEP[gRPC Client / PEP] -->|1. Call CheckPermission| gRPC[gRPC Server]
    gRPC -->|2. Parse Request Context| Parser[Context Resolver]
    
    subgraph Data_Plane_Engine ["In-Memory Data Plane Engine"]
        Parser -->|3. Lookup Trie| Trie[Radix Trie Indexer]
        Trie -->|4. Get Matched ASTs| Evaluator[AST Evaluator]
        Evaluator -->|5. Evaluate Conditions| Engine[Decision Core]
    end

    Engine -->|6. Deny-by-Default Logic| Result{Decision Result}
    Result -->|7. ALLOW / DENY| gRPC
    gRPC -->|8. Return Response| PEP
    
    Engine -->|9. Audit Event| Buffer[Ring Buffer]
```

1.  **Tiếp nhận:** gRPC Server tiếp nhận request chứa: `Subject`, `Action`, `Resource`, `Context Map`.
2.  **Phân tách:** Hệ thống trích xuất các thuộc tính động từ Context.
3.  **Tra cứu nhanh:** Radix Trie Indexer dựa vào `Subject` và `Resource` để lọc ra danh sách các chính sách có khả năng áp dụng (loại bỏ 99% các chính sách không liên quan).
4.  **Đánh giá:** AST Evaluator duyệt qua cây cú pháp của các chính sách được lọc, kiểm tra các điều kiện ABAC động.
5.  **Ra quyết định:** Áp dụng logic **Deny-by-Default**. Chỉ trả về `ALLOW` nếu có luật ALLOW khớp và không có bất kỳ luật DENY nào cản trở.
6.  **Ghi log nền:** Đẩy bản ghi log quyết định sang Ring Buffer để ghi log bất đồng bộ.

---

## 3. Luồng ghi Nhật ký Kiểm toán Bất đồng bộ (Async Audit Flow)

```mermaid
graph TD
    PDP[Decision Core] -->|1. Push Log Event| Buffer[RAM Ring Buffer / Go Channel]
    
    subgraph Async_Log_Worker ["Async Log Worker Thread"]
        Buffer -->|2. Pull Batch Events| Worker[Batch Aggregator]
        Worker -->|3. Flush to Disk / Stream| Storage[(Audit Log WORM / Kafka)]
    end
```

*   **Non-Blocking:** Luồng xử lý gRPC chính chỉ làm nhiệm vụ đẩy log event vào Go channel/Ring Buffer và trả kết quả ngay lập tức cho PEP.
*   **Batch Aggregator:** Worker chạy nền gom các sự kiện log lại theo nhóm (ví dụ: mỗi 100 logs hoặc sau mỗi 50ms) và thực hiện ghi hàng loạt (Bulk Write) xuống đĩa cứng hoặc đẩy sang Kafka, giảm thiểu tối đa chi phí I/O vật lý.
