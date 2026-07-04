# Threat Model & Attack Surface Analysis

Tài liệu này phân tích mô hình đe dọa (Threat Modeling) theo tiêu chuẩn **STRIDE**, đánh giá bề mặt tấn công (Attack Surface) và các kịch bản lạm dụng (Abuse Cases) đối với **Standalone Policy Engine**.

---

## 1. Mô hình Đe dọa STRIDE

Dưới đây là các mối đe dọa tiềm tàng được phân tích theo 6 danh mục của STRIDE:

| Danh mục STRIDE | Mối đe dọa chi tiết (Threat) | Tác động (Impact) | Biện pháp giảm thiểu (Mitigation) |
| :--- | :--- | :--- | :--- |
| **S**poofing (Mạo danh) | Kẻ tấn công giả mạo làm API Gateway (PEP) hợp lệ để gửi yêu cầu phân quyền rác hoặc dò tìm cấu trúc quyền. | Trung bình | Yêu cầu bắt buộc **mTLS X.509** ở tầng gRPC. Chỉ chấp nhận các kết nối có client certificate do CA nội bộ của hệ thống ký. |
| **T**ampering (Sửa đổi) | Hacker chiếm quyền Admin hoặc DB, cố tình sửa đổi chính sách để tự cấp quyền ALLOW cho tài khoản của mình. | Nghiêm trọng | Ký số chính sách (Policy Signing): Mỗi chính sách khi lưu ở DB sẽ đi kèm chữ ký số cryptographic. PDP kiểm tra chữ ký này trước khi load lên RAM. |
| **R**epudiation (Chối bỏ) | Kẻ tấn công thực hiện hành vi trái phép nhưng cố tình xóa log quyết định của PDP để chối bỏ trách nhiệm. | Cao | Thiết lập hệ thống **Decision Logging bất biến (WORM)** truyền thẳng log sang SIEM trung tâm qua kết nối mã hóa không thể đảo ngược. |
| **I**nformation Disclosure | Rò rỉ dữ liệu chính sách hoặc thông tin khách thuê (Tenant Leak) sang cho đối thủ thông qua các lỗi truy vấn RAM. | Cao | Cách ly bộ nhớ ở mức logic bằng cách index dữ liệu theo Root Key là `tenant_id`. Không bao giờ gộp chung index của các tenant khác nhau. |
| **D**enial of Service (DoS) | Kẻ tấn công gửi hàng triệu request phức tạp hoặc đẩy các chính sách đệ quy sâu để làm sập Engine trên RAM. | Nghiêm trọng | Giới hạn độ sâu AST, giới hạn kích thước request payload gRPC, áp dụng cơ chế Rate Limiting ở tầng Envoy Proxy trước PDP. |
| **E**levation of Privilege | Người dùng thông thường gửi request sửa đổi context (ví dụ: tự khai IP nội bộ) để lọt qua vòng kiểm tra ABAC. | Nghiêm trọng | **Không tin cậy Client:** Mọi thông tin ngữ cảnh nhạy cảm (như IP thực, vai trò) bắt buộc phải do Gateway tự trích xuất từ HTTPS session/JWT, không cho phép client truyền trực tiếp. |

---

## 2. Phân tích Bề mặt Tấn công (Attack Surface Analysis)

Bề mặt tấn công của hệ thống được thu hẹp tối đa bằng cách đóng kín các cổng giao tiếp:

*   **gRPC Server Port (50051):** Chỉ mở trong mạng nội bộ bảo mật (Intranet). Không bao giờ NAT ra Internet. Chỉ chấp nhận traffic mã hóa mTLS từ các PEP Pod đã được định danh.
*   **Admin HTTP Port (8080):** Chỉ mở cho Admin Control Plane, yêu cầu xác thực bằng OAuth2 Bearer Token với quyền hành hạn chế (RBAC cấp cao).
*   **AST Engine Parser:** Điểm yếu tiềm tàng nằm ở bộ Parser DSL. Nếu parser viết lỗi, kẻ tấn công có thể gửi các chuỗi DSL có cấu trúc dị biệt để kích hoạt lỗi Panic trong Go runtime (gây sập ứng dụng). 
    *   *Biện pháp:* Viết Fuzzing tests cho bộ Parser để phát hiện sớm các trường hợp crash trước khi deploy.

---

## 3. Các Kịch bản Lạm dụng (Abuse Cases)

### Kịch bản 1: Tấn công "AST Explosion" (Từ chối dịch vụ cấp ứng dụng)
*   **Hành vi:** Kẻ tấn công có quyền viết luật cố tình tạo ra một quy tắc phân quyền chứa biểu thức logic lồng nhau 100 lần (ví dụ: `A OR (B OR (C OR ...)))`).
*   **Mục tiêu:** Khi PDP nạp luật này và chạy evaluator, nó sẽ phải thực hiện duyệt đệ quy sâu trên RAM, chiếm dụng 100% tài nguyên CPU và làm chậm toàn bộ luồng gRPC khác.
*   **Phòng thủ:** Thiết lập giới hạn tối đa độ sâu của cây AST là **15**. Mọi chính sách vượt quá giới hạn này sẽ bị chặn ngay lập tức ở bước validate của Control Plane.

### Kịch bản 2: Tấn công "Context IP Spoofing"
*   **Hành vi:** Alice cố tình truyền tham số `context: { "ip": "10.0.0.1" }` lên client-side để lừa hệ thống cho phép truy cập tài nguyên bảo mật nội bộ.
*   **Mục tiêu:** Vượt qua bộ lọc ABAC yêu cầu IP nội bộ.
*   **Phòng thủ:** API Gateway (PEP) khi chuyển tiếp request phân quyền bắt buộc phải ghi đè (override) các thuộc tính hệ thống như `ip` bằng IP thực tế lấy từ TCP connection header, không sử dụng trực tiếp dữ liệu do Client truyền lên.
