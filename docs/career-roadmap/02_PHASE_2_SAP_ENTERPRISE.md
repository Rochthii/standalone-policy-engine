# GIAI ĐOẠN 2: LÀM CHỦ ODOO & TÍCH HỢP HỆ THỐNG
> **Thời gian:** Năm 3 Kỳ 2 | **Mục tiêu:** Chuyển từ người dùng sang người viết lõi hệ thống (Core Engine Developer)

---

## 🎯 1. KIẾN THỨC KỸ THUẬT BẮT BUỘC PHẢI NẮM CHẮC

1. **Odoo ORM Chuyên Sâu (Python):**
   - Cách định nghĩa trường quan hệ phức tạp: `fields.Many2one`, `fields.One2many`, `fields.Many2many`.
   - Các hàm ghi đè nghiệp vụ vòng đời: `create()`, `write()`, `unlink()`.
   - Decorators tối ưu hiệu năng: `@api.depends`, `@api.onchange`, `@api.constrains`.
2. **Hệ Thống Phân Quyền Trong Odoo (Security Layers):**
   - **`ir.model.access.csv`:** Phân quyền mức bảng (CRUD: Read, Write, Create, Unlink) theo Nhóm người dùng (Groups).
   - **Record Rules (`ir.rule`):** Cơ chế lọc dòng dữ liệu theo thuộc tính (Chi nhánh, Phòng ban, Hạn mức) bằng Python Domain.
   - *Điểm cốt lõi:* Liên hệ trực tiếp với tư duy **PostgreSQL Row-Level Security (RLS)** mà bạn đã xây dựng trong dự án `secure-multitenant-saas`.
3. **Thiết Kế API & Webhook Giao Tiếp Đa Hệ Thống:**
   - Viết Odoo Controller (`@http.route`) xử lý luồng dữ liệu REST / JSON-RPC.
   - Kết nối hai chiều với các hệ thống bên ngoài (POS, Web E-Commerce, Máy quét mã vạch, Cổng thanh toán).

---

## 🛠️ 2. DỰ ÁN THỰC HÀNH THEN CHỐT

* **Xây dựng Custom Module:** **`custom_approval_and_branch_isolation`**
  - **Phê duyệt đơn hàng theo hạn mức:** Tích hợp với **Standalone Go PDP Engine** để đánh giá quyền phê duyệt dưới 0.35ms.
  - **Cô lập chi nhánh:** Người dùng chi nhánh TP.HCM không thể nhìn thấy dữ liệu và báo cáo tài chính của chi nhánh Hà Nội.

---

## ⚠️ 3. ĐIỂM MÙ CẦN TRÁNH TUYỆT ĐỐI

* **Bỏ qua ràng buộc nghiệp vụ (Business Constraints):**
  - *Điểm mù:* Một đơn hàng đã xuất kho và thanh toán thì **KHÔNG ĐƯỢC PHÉP XÓA** khỏi CSDL.
  - Nếu code của bạn cho phép xóa hoặc sửa trực tiếp vào DB sẽ làm hỏng toàn bộ sổ sách kế toán kép $\sum \text{Debit} = \sum \text{Credit}$.
  - *Hành động chuẩn:* Luôn xử lý bằng cơ chế hủy chứng từ (Cancel), tạo đơn trả hàng (Return Picking) hoặc tạo bút toán đảo (Reversal Move).
