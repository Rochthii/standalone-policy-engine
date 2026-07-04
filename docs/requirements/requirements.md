# Functional & Non-Functional Requirements Specification

Tài liệu này đặc tả chi tiết các yêu cầu chức năng (FR) và phi chức năng (NFR) chuẩn hóa cho **Standalone Policy Engine**, đã tích hợp các bản vá bảo mật và hiệu năng từ đợt kiểm duyệt kiến trúc (Architecture Review).

---

## 1. Yêu cầu Chức năng (Functional Requirements)

### FR-001: Đánh giá Chính sách (Policy Evaluation)
*   Hệ thống phải tiếp nhận yêu cầu phân quyền gRPC chứa `Subject`, `Action`, `Resource`, `Context` và trả về quyết định `ALLOW` hoặc `DENY` ngay lập tức dựa trên tập chính sách tương ứng của Tenant.
*   Chính sách được đánh giá theo nguyên lý **Deny-by-Default** và ưu tiên luật cấm: Nếu có bất kỳ luật cấm (DENY) nào khớp, hoặc không có luật cho phép (ALLOW) nào khớp, kết quả trả về bắt buộc phải là `DENY`.

### FR-002: Hỗ trợ RBAC (Role-Based Access Control)
*   Hệ thống phải hỗ trợ phân quyền theo vai trò và cấu trúc phân cấp vai trò (Role Hierarchies).
*   Ví dụ chính sách: Cho phép bất kỳ người dùng nào có vai trò `admin` hoặc kế thừa từ `admin` thực hiện hành động.

### FR-003: Hỗ trợ ABAC (Attribute-Based Access Control)
*   Hệ thống phải hỗ trợ đánh giá các thuộc tính động được gửi kèm trong request context (ví dụ: Địa chỉ IP, thời gian yêu cầu, trạng thái thiết bị).
*   Hỗ trợ so khớp thuộc tính phức tạp bằng các phép toán logic (`&&`, `||`, `!`), toán tử quan hệ (`==`, `!=`, `>`, `<`, `>=`, `<=`) và các toán tử tập hợp (`in`, `contains`).

### FR-004: Hỗ trợ PBAC (Policy-Based Access Control)
*   Hệ thống phải hỗ trợ cấu trúc phân quyền dựa trên các tệp chính sách viết bằng ngôn ngữ khai báo độc lập (Policy-as-Code).
*   Tập luật chính sách có thể được cập nhật, thêm mới hoặc thu hồi một cách linh hoạt mà không cần biên dịch lại mã nguồn hệ thống.

### FR-005: Giải thích Quyết định (Policy Explain)
*   PDP phải có khả năng trả về danh sách các ID chính sách cụ thể dẫn đến quyết định phân quyền cuối cùng (ví dụ: Quyết định DENY do chính sách có ID `P-098` cấm, hoặc ALLOW nhờ chính sách `P-001` cho phép).

### FR-006: Quản lý Phiên bản Chính sách (Policy Versioning)
*   Mỗi khi một chính sách bị sửa đổi, hệ thống phải tự động tăng chỉ số phiên bản (`version`) và lưu giữ bản ghi lịch sử của phiên bản cũ trong Database để phục vụ việc rollback khi cần thiết.

### FR-007: Cách ly Đa khách thuê (Multi-Tenant Isolation)
*   Hệ thống phải cô lập tuyệt đối dữ liệu và luật của từng Tenant.
*   Yêu cầu CheckAccess của Tenant A không được phép tham chiếu hoặc quét qua các chính sách thuộc sở hữu của Tenant B.

### FR-008: Nạp nóng Không gián đoạn (Hot Reload)
*   PDP phải hỗ trợ nạp nóng các chính sách mới vào RAM cache (Trie Index) ngay khi có sự thay đổi từ Control Plane mà không được gây downtime hay gián đoạn dịch vụ gRPC đang chạy.

### FR-009: Nhật ký Kiểm toán Bất biến (Audit Logging)
*   PDP phải ghi lại chi tiết mọi quyết định phân quyền một cách bất đồng bộ xuống cơ sở dữ liệu kiểm toán bất biến (WORM).
*   **Cơ chế Spill-to-Disk:** Khi hàng đợi ghi log bị đầy do Database quá tải, hệ thống phải tự động chuyển hướng ghi log thô tạm thời xuống đĩa cứng vật lý (SSD) thay vì block luồng gRPC chính, đảm bảo không mất mát log kiểm toán.

### FR-010: Giả lập Chính sách (Policy Simulation)
*   Control Plane phải cung cấp API giả lập, cho phép quản trị viên gửi thử request giả định cùng một tập chính sách chưa kích hoạt (Draft) để kiểm tra xem quyết định phân quyền sẽ ra sao trước khi xuất bản (Publish) chính sách đó lên môi trường production.

---

## 2. Yêu cầu Phi Chức năng (Non-Functional Requirements)

### NFR-001: Độ trễ xử lý (Latency)
*   Độ trễ xử lý đánh giá trong bộ nhớ (In-memory evaluation latency): `< 0.35ms` (P99).
*   Độ trễ toàn trình gRPC CheckAccess: `< 1.2ms` (P99) trong mạng nội bộ.

### NFR-002: Khả năng sẵn sàng (Availability)
*   Hệ thống phải đạt chỉ số HA `>= 99.99%`.
*   Data Plane chạy phi trạng thái (Stateless) hoàn toàn trên RAM để đảm bảo nếu DB sập, PDP vẫn trả lời quyền bình thường.

### NFR-003: Khả năng mở rộng (Scalability)
*   Hệ thống phải đạt hiệu năng tối thiểu `15,000 RPS` trên mỗi core vCPU.
*   Hỗ trợ mở rộng theo chiều ngang tự động qua Kubernetes HPA dựa trên tải CPU.

### NFR-004: Bảo mật an ninh (Security)
*   Bắt buộc mã hóa đường truyền gRPC bằng TLS 1.3 và xác thực mTLS X.509.
*   Kiểm tra nghiêm ngặt độ sâu tối đa của AST (giới hạn 15 cấp) để ngăn chặn tấn công DoS bằng biểu thức lồng đệ quy.

### NFR-005: Khả năng quan sát (Observability)
*   Cung cấp Prometheus metrics exporter (RPS, Latency histogram, CPU/RAM utilization, RAM cache policy count).
*   Tích hợp OpenTelemetry tracing.

---

## 3. Ràng buộc An toàn & Ngăn ngừa Quá tải (Safety Constraints)

*   **Ràng buộc Wildcard:** Control Plane bắt buộc phải từ chối kích hoạt chính sách nếu một Tenant thiết lập số lượng chính sách wildcard kép (`principal == any` và `resource == any` đồng thời) vượt quá **`5%`** tổng số chính sách đang hoạt động của Tenant đó.
*   **Giới hạn AST Depth:** Giới hạn tối đa độ sâu của cây biểu thức logic AST là **`15`** cấp để bảo vệ call stack của Go runtime khỏi lỗi tràn bộ nhớ (Stack Overflow).
*   **Version Epoch Matching:** Mỗi request CheckAccess từ PEP bắt buộc phải gửi kèm số phiên bản epoch của tập chính sách hiện tại để phát hiện lệch cache và đồng bộ tức thì.
