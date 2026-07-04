# Syntax Reference

Tài liệu này hướng dẫn chi tiết cách viết cú pháp chính sách (Syntax) và các kiểu dữ liệu được hỗ trợ trong ngôn ngữ DSL của **Standalone Policy Engine**.

---

## 1. Các Kiểu Dữ Liệu Được Hỗ Trợ

Ngôn ngữ hỗ trợ 5 kiểu dữ liệu nguyên bản phục vụ đánh giá thuộc tính động (ABAC):

| Kiểu dữ liệu | Định dạng DSL | Ví dụ thực tế |
| :--- | :--- | :--- |
| **String (Chuỗi)** | Bao quanh bởi dấu nháy kép `""`. | `"file:doc.pdf"`, `"finance"` |
| **Integer (Số nguyên)** | Số nguyên có hoặc không có dấu âm. | `100`, `-5`, `0` |
| **Boolean (Logic)** | Giá trị logic tĩnh. | `true`, `false` |
| **IP Address (Địa chỉ IP)** | Địa chỉ IPv4 đơn lẻ hoặc dải mạng CIDR. | `"192.168.1.1"`, `"10.0.0.0/8"` |
| **DateTime (Thời gian)** | Định dạng chuẩn ISO 8601 UTC. | `"2026-07-04T10:20:00Z"` |

---

## 2. Các Toán Tử Hệ Thống (System Operators)

### A. Toán tử So sánh & Quan hệ (Relational Operators)
*   `==` (Bằng): So khớp tuyệt đối giá trị.
    *   *Ví dụ:* `principal.role == "admin"`
*   `!=` (Không bằng):
    *   *Ví dụ:* `resource.owner != "user:anonymous"`
*   `>`, `<`, `>=`, `<=` (So sánh số học): Áp dụng cho kiểu dữ liệu `Integer` và `DateTime`.
    *   *Ví dụ:* `context.request_time < "17:00:00Z"`

### B. Toán tử Logic (Logical Operators)
*   `&&` (AND): Trả về true nếu cả hai biểu thức đều true.
*   `||` (OR): Trả về true nếu một trong hai biểu thức true.
*   `!` (NOT): Đảo ngược giá trị logic của biểu thức.
    *   *Ví dụ:* `!(context.device_status == "compromised")`

### C. Toán tử Tập hợp & Đặc thù (Collection Operators)
*   `in` (Thuộc về): Kiểm tra xem một đối tượng có thuộc về một nhóm/dải IP/phân cấp vai trò nào đó không.
    *   *Ví dụ 1 (IP Range check):* `context.ip_address in "192.168.1.0/24"`
    *   *Ví dụ 2 (Role Hierarchy check):* `principal in role:admin`
*   `contains` (Chứa): Kiểm tra xem một tập hợp (mảng) có chứa một phần tử cụ thể không.
    *   *Ví dụ:* `principal.groups contains "finance"`
