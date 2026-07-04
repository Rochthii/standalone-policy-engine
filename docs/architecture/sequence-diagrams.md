# Sequence Diagrams Specification

Tài liệu này cung cấp các sơ đồ tuần tự (Sequence Diagrams) mô tả tương tác động giữa các thành phần trong **Standalone Policy Engine**.

---

## 1. Sơ đồ Tuần tự: Yêu cầu Kiểm tra Quyền (gRPC Evaluation Sequence)

Sơ đồ này mô tả chi tiết từ lúc client yêu cầu truy cập tài nguyên cho đến khi PDP đưa ra quyết định phân quyền:

```mermaid
sequenceDiagram
    autonumber
    participant Client as Client Application
    participant PEP as API Gateway (PEP)
    participant PDP as Policy Engine (PDP)
    participant Index as In-Memory Trie Index
    participant Eval as AST Evaluator
    participant Log as Async Ring Buffer

    Client->>PEP: Gửi yêu cầu (Ví dụ: DELETE /file/123)
    PEP->>PEP: Trích xuất token và ngữ cảnh (IP, Time)
    PEP->>PDP: Gọi gRPC CheckPermission(Subject, Action, Resource, Context)
    
    activate PDP
    PDP->>Index: Tra cứu danh sách Policy khớp (Subject, Resource)
    activate Index
    Index-->>PDP: Trả về matched Policies (ASTs)
    deactivate Index
    
    PDP->>Eval: Đánh giá thuộc tính động (ABAC Expressions)
    activate Eval
    Eval-->>PDP: Trả về kết quả đánh giá (Match/No Match)
    deactivate Eval
    
    PDP->>PDP: Tổng hợp kết quả (Deny-by-Default)
    PDP->>Log: Đẩy thông tin log (Async Push)
    
    PDP-->>PEP: Trả về CheckResponse (ALLOW / DENY + Reason)
    deactivate PDP
    
    alt Quyết định là ALLOW
        PEP->>Client: Cho phép truy cập (200 OK + Resource Data)
    else Quyết định là DENY
        PEP->>Client: Từ chối truy cập (403 Forbidden)
    end
```

---

## 2. Sơ đồ Tuần tự: Đồng bộ hóa Chính sách (Policy Sync Sequence)

Sơ đồ này mô tả quá trình cập nhật nóng chính sách lên RAM cache của các node PDP khi Admin thay đổi:

```mermaid
sequenceDiagram
    autonumber
    actor Admin as System Administrator
    participant Control as Control Plane (API)
    participant DB as Persistent DB
    participant PubSub as Central Pub/Sub (Redis)
    participant PDP as PDP Engine Pod (Data Plane)
    participant Trie as In-Memory RAM Trie

    Admin->>Control: PUT /api/v1/policies/{id} (Gửi DSL mới)
    activate Control
    Control->>Control: Phân tích cú pháp & Biên dịch AST thử
    
    alt Biên dịch AST lỗi
        Control-->>Admin: Trả về 400 Bad Request (Syntax Error)
    else Biên dịch AST thành công
        Control->>DB: Ghi đè chính sách & AST mới
        Control->>PubSub: Phát sự kiện "PolicyUpdated" (policy_id, tenant_id)
        Control-->>Admin: Trả về 200 OK (Chính sách đã cập nhật)
        deactivate Control
        
        PubSub->>PDP: Kênh Pub/Sub gửi sự kiện đến PDP Pod
        activate PDP
        PDP->>DB: Truy vấn lấy AST đã compile của policy_id
        DB-->>PDP: Trả về compiled AST
        PDP->>Trie: Ghi đè node cũ trên RAM (Copy-On-Write Root Swap)
        PDP-->>PDP: Flush cache logic
        deactivate PDP
    end
```
