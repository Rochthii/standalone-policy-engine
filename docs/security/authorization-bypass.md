# Authorization Bypass Prevention Specification

Tài liệu này đặc tả các mối đe dọa vượt quyền/bỏ qua bước phân quyền (Authorization Bypass) và các giải pháp kỹ thuật phòng thủ cho **Standalone Policy Engine**.

---

## 1. Mối đe dọa Vượt qua Phân quyền (Bypass Threats)

Kẻ tấn công có thể cố gắng bypass hệ thống thông qua các con đường:
1.  **Direct PDP Attack (Tấn công trực tiếp PDP):** Vượt qua API Gateway (PEP), kết nối trực tiếp đến cổng gRPC của PDP để tự gửi request giả mạo tự trả về ALLOW.
2.  **Man-in-the-Middle (Tấn công xen giữa):** Đánh cắp gói tin gRPC trên đường truyền mạng nội bộ và chỉnh sửa kết quả CheckResponse từ DENY thành ALLOW.
3.  **State Mismatch (Lệch trạng thái):** Tận dụng sự trễ đồng bộ cache RAM để thực hiện hành vi ngay khi chính sách cấm vừa được ban hành nhưng chưa kịp update lên PDP.

---

## 2. Giải pháp Phòng thủ (Defense Controls)

### A. Triển khai gRPC mTLS với Client Certificate Binding
*   Để chặn đứng tấn công *Direct PDP Attack*, cổng gRPC `50051` của PDP cấu hình chế độ **`RequireAndVerifyClientCert`**.
*   Mỗi API Gateway (PEP) khi deploy lên K8s sẽ được cấp phát một client certificate độc bản thông qua Vault hoặc Cert-Manager. PDP sẽ so khớp trường Common Name (CN) hoặc Subject Alternative Name (SAN) của certificate để xác minh Gateway được quyền gọi dịch vụ. Mọi kết nối từ các máy khách lạ đều bị ngắt kết nối ngay từ bước bắt tay TCP/TLS (Fail-Closed).

### B. Ký mã hóa kết quả Phản hồi (Signed Decision Payload)
*   Để chống lại tấn công *Man-in-the-Middle* sửa đổi kết quả truyền tin, PDP có thể cấu hình ký số lên payload trả về:
    *   Trường `CheckResponse` ngoài `decision` sẽ chứa thêm một chữ ký điện tử ngắn (Signature) được sinh ra từ khóa riêng tư của PDP.
    *   API Gateway (PEP) nhận được response bắt buộc phải verify chữ ký này bằng khóa công khai của PDP trước khi cho phép client truy cập tài nguyên.

### C. Triết lý Fail-Closed tuyệt đối
*   Nếu có bất kỳ lỗi nào xảy ra trong quá trình gRPC giao tiếp (ví dụ: Timeout kết nối, lỗi parse Protobuf, lỗi bộ nhớ đệm), API Gateway (PEP) bắt buộc phải thực thi chính sách **Fail-Closed**: Từ chối quyền truy cập của người dùng ngay lập tức (trả về lỗi 403 / 500), tuyệt đối không được phép cho qua (Fail-Open).
*   Mã code tích hợp của PEP SDK (định nghĩa tại `/pkg/client`) bắt buộc phải tuân thủ nghiêm ngặt quy tắc này.
