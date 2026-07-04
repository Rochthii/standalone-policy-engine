# Standalone PBAC/ABAC Policy Engine - AI Coding Rules

## 🎯 Bối Cảnh Dự Án (Project Context)
Dự án này là Động cơ Quyết định Phân quyền Độc lập (Standalone Policy Decision Point - PDP) hiệu năng cao viết bằng Golang.
- **Mục tiêu:** Đánh giá quyền truy cập động dựa trên thuộc tính (ABAC) và chính sách (PBAC) với độ trễ < 1ms dưới tải cực lớn (> 10,000 RPS).
- **Kiến trúc:** Nhận yêu cầu gRPC từ các Policy Enforcement Points (PEP) -> Tra cứu và đánh giá AST (Abstract Syntax Tree) trên RAM (In-memory decision tree) -> Trả về ALLOW/DENY.

---

## 🛠️ Quy Tắc Lập Trình Cho AI (AI Coding Rules)

### 1. Nguyên Tắc Thiết Kế Hiệu Năng Cao (In-Memory Performance)
- **Không duyệt tuyến tính (Linear Scan):** Tuyệt đối không dùng vòng lặp `for` duyệt qua toàn bộ danh sách policy của hệ thống khi có request. Bắt buộc phải tổ chức dữ liệu dạng trie, map lồng nhau, hoặc Radix Tree dựa trên `subject` và `resource` để đạt độ phức tạp tìm kiếm `O(1)` hoặc `O(log N)`.
- **Hạn chế cấp phát bộ nhớ (Allocation Minimization):** Tránh cấp phát lại bộ nhớ liên tục trong luồng xử lý chính. Tận dụng `sync.Pool` cho các struct ngữ cảnh, bộ đệm parser hoặc AST evaluate để giảm tải cho Garbage Collector (GC).
- **Concurrency & Lock Optimization:** Sử dụng `sync.RWMutex` tối ưu cho các luồng đọc ghi bất đối xứng (Read-Heavy / Write-Rare). Sử dụng các cơ chế lock-free hoặc double-checking lock khi cập nhật RAM store để tránh nghẽn thread.

### 2. Ngôn Ngữ Luật (Policy Language & AST Parser)
- **No Hardcoded Logic:** Luật phân quyền không được code cứng trong Go mà phải được phân tích từ các file chính sách (dạng JSON hoặc ngôn ngữ khai báo tương tự Cedar).
- **AST Safety:** Mọi biểu thức đánh giá thuộc tính động (ABAC) phải được parse thành cây cú pháp trừu tượng (AST) và đánh giá an toàn, ngăn chặn việc thực thi code tùy ý (Arbitrary Code Execution).

### 3. Chuẩn gRPC & Protobuf
- Sử dụng Protobuf được định nghĩa chuẩn xác tại thư mục `/proto`.
- Đảm bảo gRPC Connection Multiplexing hoạt động mượt mà, tối ưu hóa keepalive parameters để duy trì kết nối persistent siêu tốc giữa API Gateway (PEP) và Policy Engine (PDP).

### 4. Quy Tắc Vibe Coding & Code Sạch (Clean Code & No Mock)
- Không viết code demo, các hàm trả về kết quả giả lập cố định (hardcoded ALLOW/DENY). Mọi quyết định phải được tính toán dựa trên các chính sách lưu trên bộ nhớ thực tế.
