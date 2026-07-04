# Policy Language Overview

Động cơ phân quyền **Standalone Policy Engine** sử dụng một ngôn ngữ khai báo chính sách riêng (DSL - Domain Specific Language) được thiết kế theo cảm hứng từ ngôn ngữ **Cedar** của AWS. 

Ngôn ngữ này tách biệt hoàn toàn logic phân quyền khỏi mã nguồn ứng dụng, giúp các chính sách trở nên rõ ràng, dễ hiểu đối với cả lập trình viên lẫn quản trị viên bảo mật.

---

## 1. Triết lý Thiết kế Ngôn ngữ

*   **Rõ ràng và Khai báo (Declarative):** Mô tả *ai* được làm *gì*, trên *tài nguyên nào*, dưới *điều kiện gì* thay vì viết các khối lệnh điều kiện `if/else` lập trình.
*   **An toàn và Hạn chế (Safe & Non-Turing Complete):** Ngôn ngữ không hỗ trợ vòng lặp (`for`, `while`) hoặc đệ quy tùy ý. Điều này đảm bảo thời gian đánh giá chính sách luôn luôn hữu hạn và có thể dự đoán được (tránh lỗi lặp vô hạn gây sập hệ thống).
*   **Hỗ trợ Tĩnh (Analysable):** Cấu trúc luật cho phép các công cụ phân tích tĩnh (Static Analysis Tools) kiểm duyệt logic, phát hiện mâu thuẫn luật trước khi chạy runtime.

---

## 2. Cấu trúc cơ bản của một câu luật

Một câu luật đầy đủ gồm 3 thành phần chính:

```text
[Hiệu lực] ( [Đối tượng], [Hành động], [Tài nguyên] )
when {
    [Điều kiện ABAC động]
};
```

### Ví dụ thực tế:
```cedar
permit(
    principal == user:alice,
    action == action:DELETE,
    resource == file:report.pdf
)
when {
    context.ip_address in "192.168.1.0/24" &&
    context.request_time < "17:00"
};
```

*   **Hiệu lực (Effect):** `permit` (cho phép) hoặc `forbid` (cấm).
*   **Phạm vi (Scope):** Định nghĩa `principal` (ai), `action` (hành động nào), `resource` (tài nguyên nào).
*   **Mệnh đề Điều kiện (Condition Clause):** Mệnh đề `when` (hoặc `unless`) chứa các điều kiện ABAC đánh giá thuộc tính động lấy từ `context`.
