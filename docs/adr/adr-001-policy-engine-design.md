# ADR-001: Standalone Policy Engine Core Architectural Decisions

*   **Trạng thái:** APPROVED
*   **Tác giả:** Hội đồng Kỹ thuật cấp cao
*   **Ngày quyết định:** 2026-07-04

---

## 1. Bối cảnh (Context)

Hệ thống phân quyền SaaS đa khách thuê yêu cầu tốc độ phản hồi quyết định phân quyền siêu tốc (latency < 1.5ms) dưới tải lớn hàng chục ngàn RPS, đồng thời phải hỗ trợ các chính sách ABAC linh động.
Các giải pháp hiện có như OPA (dùng ngôn ngữ Rego) có bộ nhớ lớn và tốc độ đánh giá các cấu trúc dữ liệu động tương đối trễ. AWS Verified Permissions (dùng Cedar) thì bị khóa chặt vào cloud AWS.

---

## 2. Các Quyết định Kiến trúc & Phân tích (Architectural Decisions)

### Quyết định 1: Tự phát triển Ngôn ngữ DSL phân quyền (Cedar-like DSL) thay vì JSON tĩnh
*   **Lý do:** JSON tĩnh rất khó để biểu diễn các logic ABAC phức tạp (như kiểm tra dải IP, logic so sánh thời gian, quan hệ thừa kế vai trò) một cách rõ ràng. Việc tự phát triển Lexer/Parser riêng giúp câu luật trở nên trực quan, cô đọng, dễ đọc và dễ bảo trì dài hạn.
*   **Hệ quả:** Tăng chi phí phát triển ban đầu do phải tự viết và fuzz-test bộ Parser đệ quy bằng Go, nhưng mang lại trải nghiệm viết luật cực tốt và hiệu năng đánh giá AST tối ưu.

### Quyết định 2: Tổ chức chính sách dạng Radix Trie và AST bất biến trên RAM
*   **Lý do:** Tránh duyệt tuyến tính $O(N)$ khi kiểm tra quyền. Cây Trie giúp lọc nhanh các chính sách liên quan theo tiền tố `subject` và `resource` trong thời gian $O(\log N)$.
*   **Hệ quả:** RAM tiêu thụ tăng nhẹ (khoảng 2.5 GB cho 1,000,000 policies) nhưng CPU sử dụng cho tra cứu giảm 99%, triệt tiêu độ trễ thắt nút cổ chai.

### Quyết định 3: Sử dụng mô hình Concurrency Copy-On-Write (COW) và Atom Pointer Swap
*   **Lý do:** Tránh nghẽn thread đọc gRPC khi Admin cập nhật chính sách. Lệnh hoán đổi nguyên tử của Go giúp cập nhật nóng chính sách tức thì không downtime và không cần lock luồng đọc chính.
*   **Hệ quả:** Luồng đọc luôn chạy tự do không bị block, đảm bảo SLO độ trễ P99 ổn định.
*   **Tối ưu GC:** Sử dụng `sync.Pool` tái sử dụng bộ nhớ context để loại bỏ GC pause.

### Quyết định 4: Sử dụng giao thức gRPC (HTTP/2) để bắt tay PEP-PDP
*   **Lý do:** Giảm chi phí thiết lập kết nối TCP (keepalive HTTP/2 multiplexing) và giảm kích thước gói tin nhờ Protobuf v3.
*   **Hệ quả:** API Gateway (PEP) và Engine (PDP) duy trì kết nối persistent ổn định, độ trễ mạng nội bộ được đè xuống mức tối thiểu.
