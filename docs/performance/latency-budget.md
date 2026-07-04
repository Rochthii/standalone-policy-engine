# Latency Budget Specification

Tài liệu này đặc tả chi tiết phân bổ hạn mức thời gian xử lý (Latency Budget) cho từng lớp trong **Standalone Policy Engine** để đảm bảo thời gian phản hồi toàn trình gRPC luôn nhỏ hơn **`1.5ms`** (P99).

---

## 1. Phân bổ Hạn mức Thời gian (Latency Budget Allocation)

Để đạt mục tiêu phản hồi siêu tốc, tổng thời gian xử lý toàn trình từ lúc PEP gửi yêu cầu đến khi nhận kết quả được cấu trúc cụ thể:

| Phân lớp xử lý | Thời gian mục tiêu (P50) | Giới hạn tối đa (P99) | Nội dung xử lý chi tiết |
| :--- | :---: | :---: | :--- |
| **Network (mTLS Handshake & HTTP/2)** | `0.15 ms` | `0.50 ms` | Thời gian truyền tải gói tin Protobuf trên mạng nội bộ (mạng ảo VPC). |
| **Context Resolver & Binding** | `0.05 ms` | `0.15 ms` | PEP/PDP trích xuất biến động từ Map và gán vào Context Evaluator. |
| **Radix Trie Index Lookup** | `0.02 ms` | `0.08 ms` | So khớp Tenant/Subject/Resource trên cây chỉ mục RAM để lấy danh sách AST. |
| **AST Evaluation (Condition Check)**| `0.08 ms` | `0.22 ms` | Duyệt cây AST, đánh giá các biểu thức logic ABAC (đoản mạch logic). |
| **Audit Log Push** | `0.01 ms` | `0.05 ms` | Đẩy log decision sang RAM Ring Buffer (không block luồng chính). |
| **Cộng gộp toàn trình (End-to-End)** | **`0.31 ms`** | **`1.00 ms`** | Tổng thời gian đáp ứng (thấp hơn nhiều mục tiêu SLO 1.5ms). |

---

## 2. Các Quy tắc tối ưu hóa mã nguồn để duy trì Latency Budget

Để giữ thời gian xử lý nội bộ của Engine `< 0.35ms`, Go code phải tuân thủ:
*   **Tránh ghép chuỗi động (No Dynamic String Concatenation):** Không sử dụng phép cộng chuỗi `+` hoặc `fmt.Sprintf` trong các luồng Hot Path của Engine. Bắt buộc sử dụng `strings.Builder` hoặc so sánh con trỏ trực tiếp thông qua String Interning pool.
*   **Tránh sử dụng Reflection:** Không sử dụng thư viện `reflect` của Go để kiểm tra kiểu dữ liệu hoặc đọc thuộc tính động trong Evaluator. Mọi so khớp kiểu và binding context phải được ép kiểu tĩnh thủ công (Type Assertion) để đạt tốc độ của phần cứng.
*   **Tối ưu hóa các thao tác IP mạng:** Không sử dụng thư viện `net.ParseIP` ở runtime. Toàn bộ IP đã được Compiler biên dịch thành số nguyên `uint32` ở Control Plane, việc kiểm tra dải mạng chỉ là các phép toán số học CPU.
