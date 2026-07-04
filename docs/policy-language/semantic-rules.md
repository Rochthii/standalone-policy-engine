# Semantic Rules & Type Safety

Tài liệu này đặc tả các quy tắc ngữ nghĩa (Semantic Rules), cơ chế kiểm tra kiểu dữ liệu (Type Safety) và cách xử lý lỗi của bộ đánh giá chính sách (Evaluator) tại runtime.

---

## 1. Kiểm tra Kiểu dữ liệu Tĩnh & Động (Type Checking)

Để đảm bảo an toàn tuyệt đối và tránh các kết quả phân quyền không lường trước (Undefined Behavior), Engine thực thi quy tắc kiểm tra kiểu nghiêm ngặt:

*   **Không tự động ép kiểu (No Implicit Type Casting):** 
    *   Phép so sánh giữa hai kiểu dữ liệu khác nhau (ví dụ: `Integer == String` hay `Boolean == String`) sẽ bị coi là lỗi ngữ nghĩa lập tức.
    *   *Ví dụ:* `context.age == "18"` sẽ bị parser/validator từ chối ngay lập tức ở bước Control Plane (Compile-time error).
*   **Toán tử quan hệ ràng buộc kiểu:**
    *   Toán tử so sánh lớn hơn/nhỏ hơn (`>`, `<`, `>=`, `<=`) chỉ được áp dụng trên kiểu `Integer` và `DateTime`. Áp dụng trên `String` hay `Boolean` sẽ gây lỗi compile.
    *   Toán tử `in` đối với địa chỉ IP yêu cầu vế trái phải là biến kiểu `IPAddress` và vế phải phải là một chuỗi mô tả dải mạng CIDR hợp lệ (ví dụ: `"10.0.0.0/8"`).

---

## 2. Giải quyết Thừa kế Vai trò (Role Hierarchy Resolution)

Toán tử `in` được sử dụng để kiểm tra phân cấp vai trò (Role Hierarchy):

*   **Định nghĩa:** Nếu vai trò `super_admin` kế thừa `admin`, và `admin` kế thừa `operator`, thì khi kiểm tra biểu thức:
    *   `principal in role:operator` -> Sẽ trả về `true` cho người dùng có vai trò `super_admin` hoặc `admin`.
*   **Giải quyết trên RAM:** Engine duy trì một đồ thị phân cấp vai trò dạng DAG (Directed Acyclic Graph) để giải quyết quan hệ thừa kế này với độ trễ tối thiểu (dùng thuật toán DFS/BFS tìm kiếm đường đi trên đồ thị, kết quả được cache lại).
*   **Phòng tránh vòng lặp đệ quy:** Validator sẽ quét đồ thị vai trò để đảm bảo không có chuỗi thừa kế vòng lặp (ví dụ: A kế thừa B, B kế thừa C, C lại kế thừa A). Nếu phát hiện vòng lặp, chính sách sẽ bị từ chối lưu.

---

## 3. Quy tắc xử lý thuộc tính bị thiếu (Missing Attributes Handling)

Tại runtime, có khả năng request gRPC từ PEP truyền thiếu thuộc tính mà chính sách yêu cầu đánh giá (ví dụ: Chính sách yêu cầu kiểm tra `context.location` nhưng client không gửi lên).

### Quy tắc xử lý:
1.  **Không gây sập hệ thống (No Panic):** Go runtime tuyệt đối không được phép xảy ra panic hay crash.
2.  **Đánh giá lỗi (Error Evaluation):**
    *   Biểu thức con chứa thuộc tính bị thiếu sẽ ngay lập tức được đánh giá là `ERROR`.
    *   Một biểu thức so sánh có chứa `ERROR` sẽ tự động trả về giá trị logic **`false`**.
    *   *Ví dụ:* Biểu thức `context.location == "VN" && context.device_secure == true`. 
        *   Nếu thiếu `context.location`, biểu thức con `context.location == "VN"` trả về `false`.
        *   Kết quả toàn bộ biểu thức logic sẽ là `false && true = false`, dẫn đến việc không thỏa mãn luật ALLOW (kết quả DENY).
3.  **Audit Logging:** Log kiểm toán phải ghi nhận lại cảnh báo việc thiếu thuộc tính để quản trị viên SOC phát hiện và sửa cấu hình PEP.
