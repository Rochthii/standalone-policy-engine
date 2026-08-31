# GIAI ĐOẠN 1: XÂY MÓNG KỸ THUẬT & KHẮC PHỤC ĐIỂM MÙ BAN ĐẦU
> **Thời gian:** Năm 3 Kỳ 1 (Thời điểm hiện tại tại PTIT)  
> **Mục tiêu:** Nắm chắc môn cơ sở trường PTIT CLC, dựng Odoo 17 trên Docker + PostgreSQL và triệt tiêu điểm mù mã nguồn.

---

## 🎯 1. MÔN BẮT BUỘC PHẢI HỌC TỐT TẠI TRƯỜNG (PTIT CLC)

1. **Cơ Sở Dữ Liệu (Databases):**
   - Nắm vững thiết kế lược đồ quan hệ (ERD), khóa chính / khóa ngoại (Primary / Foreign Keys).
   - Chuẩn hóa dữ liệu (3NF) để tránh dư thừa và bất thường dữ liệu.
   - Cơ chế Giao dịch (**Transaction ACID**) và mức cô lập (Isolation Levels).
2. **Hệ Điều Hành & Mạng Máy Tính (OS & Computer Networks):**
   - Hiểu sâu về Tiến trình / Luồng (Process / Thread), Bộ nhớ ảo (Virtual Memory).
   - Cơ chế cân bằng tải và giao thức mạng cốt lõi (TCP/IP, HTTP/1.1, HTTP/2, REST).
3. **An Toàn Thông Tin Cơ Sở (Information Security):**
   - Bổ trợ tư duy bảo mật quyền hạn, phân quyền theo vai trò/thuộc tính và mã hóa dữ liệu.

---

## 🛠️ 2. KỸ NĂNG THỰC HÀNH SONG SONG (ODOO FOUNDATION)

* **Khởi chạy môi trường:** Cài đặt **Odoo Community (v17/v18)** chạy trên **Docker** kết hợp với **PostgreSQL 16**.
  *(Đã cấu hình sẵn tại `E:\Projects\ERP_Mastery_Hub\01_Odoo_Lab\docker-compose.yml`)*.
* **Trải nghiệm nghiệp vụ cốt lõi:** Dùng thử giao diện người dùng (UI) của 3 ứng dụng chính:
  1. **Sales (Bán hàng):** Tạo Báo giá $\rightarrow$ Đơn bán hàng (SO).
  2. **Inventory (Kho):** Phiếu xuất kho / Nhập kho $\rightarrow$ Xác nhận giao hàng.
  3. **Invoicing / Accounting (Hóa đơn / Kế toán):** Xuất hóa đơn $\rightarrow$ Ghi nhận thanh toán.
* **Cấu trúc thư mục của một Odoo Module chuẩn:**
  ```text
  custom_addons/my_module/
  ├── __init__.py
  ├── __manifest__.py        # Khai báo metadata, dependencies & views XML
  ├── models/                # Lớp nghiệp vụ ORM Python (kế thừa models.Model)
  ├── views/                 # Giao diện XML (Form, Tree, Kanban, Action)
  └── security/              # Phân quyền ir.model.access.csv & ir.rule
  ```

---

## ⚠️ 3. ĐIỂM MÙ CẦN TRIỆT TIÊU NGAY LẬP TỨC

1. **Ảo tưởng về Code do AI sinh ra (AI Code Illusion):**
   - *Điểm mù:* AI sinh code Odoo rất nhanh, nhưng nếu không biết code đó tác động đến bảng nào trong PostgreSQL thì hệ thống sẽ bị lỗi tiềm ẩn nghiêm trọng.
   - *Hành động khắc phục:* Mở phần mềm quản trị CSDL (**DBeaver** hoặc **pgAdmin**), soi trực tiếp các bảng `sale_order`, `stock_picking`, `res_partner`, `account_move` xem khi tạo 1 đơn hàng trên web thì dòng nào, cột nào được thêm mới.
2. **Trì hoãn Ngoại ngữ:**
   - *Điểm mù:* Đợi năm 4 mới học tiếng Anh là quá muộn cho thị trường quốc tế.
   - *Hành động khắc phục:* Chuyển toàn bộ tài liệu đọc, prompt AI và giao diện Odoo/SAP sang **tiếng Anh 100%** (Tiếng Anh chuyên ngành IT & Kinh tế).
