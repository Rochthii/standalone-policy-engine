# Compiler Design

Tài liệu này đặc tả thiết kế của bộ biên dịch chính sách (Policy Compiler), chuyển đổi cây AST thô thành cấu trúc dữ liệu tối ưu trên bộ nhớ RAM để phục vụ việc tra cứu và đánh giá siêu tốc ở Data Plane.

---

## 1. Vai trò của Compiler

Bộ biên dịch (Compiler) là cầu nối trung gian giữa **Control Plane** (lưu trữ DB) và **Data Plane** (đánh giá trên RAM).

```text
[Policy DSL Text] 
       │ 
       ▼ (Parser Layer)
  [Raw AST Nodes] 
       │ 
       ▼ (Compiler Layer)
  [In-Memory Radix/Trie Indexer & Compiled Evaluators]
```

*   **Tải tĩnh (Pre-computation):** Thực hiện tất cả các tác vụ nặng (như phân tích cú pháp dải IP Subnet, chuyển đổi chuỗi thời gian ISO 8601 sang Unix timestamp nguyên bản) ngay ở bước compile. 
*   **Tránh tính toán ở luồng chính (Zero Runtime Parsing):** Khi gRPC request CheckAccess đi vào Data Plane, Engine tuyệt đối không phải thực hiện bất kỳ thao tác so khớp chuỗi regex hay parse IP nào. Mọi so khớp đều là so khớp số nguyên hoặc so khớp bitmap cực nhanh.

---

## 2. Quy trình Biên dịch & Tối ưu hóa (Compilation Optimization)

1.  **IP Address Pre-parsing:** 
    *   Mỗi chuỗi IP dạng dải mạng (ví dụ `"192.168.1.0/24"`) sẽ được compiler chuyển thành một số nguyên 32-bit (IPv4) đại diện cho địa chỉ mạng và một mặt nạ bit (Bitmask) 32-bit.
    *   Tại runtime, phép kiểm tra `ip in CIDR` chỉ tốn đúng **2 phép toán logic bitwise CPU**: `(request_ip & netmask) == network_address`, đạt tốc độ tối đa của phần cứng.
2.  **Constant Folding (Thu gọn hằng số):**
    *   Nếu trong biểu thức logic có chứa các phép toán tĩnh giữa các hằng số (ví dụ: `100 > 50`), compiler sẽ tự động tính toán trước và thay thế toàn bộ cụm biểu thức đó bằng giá trị boolean tĩnh `true` trên cây AST.
3.  **String Interning (Bản đồ chuỗi duy nhất):**
    *   Để tránh việc cấp phát ô nhớ cho các chuỗi trùng lặp (như tên các Action `READ`, `DELETE` hay Tenant ID), compiler sử dụng một bản đồ chuỗi tập trung (String Pool). Các node AST sẽ chỉ lưu trữ con trỏ trỏ tới String Pool này.
