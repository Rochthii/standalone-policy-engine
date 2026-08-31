# GIAI ĐOẠN 3: PHÂN CHUYÊN NGÀNH SE, NẠP TƯ DUY SAP & ĐỒ ÁN TỐT NGHIỆP
> **Thời gian:** Năm 4 (Học kỳ 1 & 2) | **Mục tiêu:** Tăng tốc, định hình tư duy chuẩn mực Enterprise và bảo vệ Khóa luận xuất sắc

---

## 🎯 1. ĐĂNG KÝ CHUYÊN NGÀNH TẠI PTIT

* **Chọn chuyên ngành:** **SE (Kỹ thuật Phần mềm — Software Engineering)**.
* **Các môn học cần tập trung cao độ:**
  1. **Kiến trúc & Thiết kế Phần mềm (Software Architecture & Design):** Hiểu sâu cách phân rã module lớn, giảm khớp nối (Loose Coupling), tăng gắn kết (High Cohesion).
  2. **Phát triển Phần mềm An toàn (Secure Software Development):** Áp dụng mô hình Threat Modeling, DevSecOps, phòng thủ chống leo thang quyền hạn.
  3. **Hệ thống Phân tán (Distributed Systems):** Hiểu sự đồng thuận (Consensus), phân vùng mạng (CAP theorem), Message Queue và Cache Invalidation.

---

## 🏛️ 2. HỌC CHUẨN TƯ DUY SAP (ENTERPRISE STANDARD)

Không cần cài đặt SAP thật, hãy tập trung làm chủ **Chuẩn tích hợp và Cấu trúc quy trình**:

### A. 3 Phân Hệ Cốt Lõi Của SAP:
1. **SAP MM (Materials Management):** Chuỗi cung ứng, quản lý hàng tồn kho, chu trình mua sắm **Procure-to-Pay (P2P)**.
2. **SAP SD (Sales & Distribution):** Quản lý khách hàng, đơn bán hàng, bảng giá, chu trình bán hàng **Order-to-Cash (O2C)**.
3. **SAP FI (Financial Accounting):** Sổ cái tổng hợp (General Ledger), hạch toán Nợ/Có, hóa đơn phải thu / phải trả (AR/AP).

### B. Chuẩn Giao Tiếp Tích Hợp SAP Hiện Đại:
* Nắm vững cách gọi và cấu trúc dữ liệu **SAP OData Services (v2/v4 RESTful JSON)** và **BAPI**.
* Triết lý **SAP Clean Core**: Không sửa core, viết mở rộng Side-by-Side trên **SAP BTP**.

---

## 🎓 3. ĐỊNH HƯỚNG ĐỒ ÁN TỐT NGHIỆP ĐỈNH CAO (4 RQs)

* **Tên đề tài:** Nghiên cứu và Phát triển Động cơ Policy Decision Point (PDP) Hiệu Năng Cao Phục Vụ Phân Quyền Hệ Thống Doanh Nghiệp Và Chốt Chặn An Toàn Cho Tác Tử AI Tự Hành.
* **Mô hình thực nghiệm:** Xây dựng cổng trung gian Middleware đồng bộ dữ liệu Zero Trust giữa **Odoo 17 (Chi nhánh)** và hệ thống ERP trung tâm (**Mock SAP S/4HANA OData**), áp dụng phân quyền thời gian thực bằng **Standalone Go PDP Engine**.

---

## ⚠️ 4. ĐIỂM MÙ CẦN TRÁNH TUYỆT ĐỐI

* **Học vẹt mã giao dịch (T-Code) của SAP:**
  - *Điểm mù:* Chăm chăm học thuộc lòng mã T-Code (như `VA01`, `ME21N`, `FB01`).
  - *Thực tế:* Nhà tuyển dụng và đối tác quốc tế cần bạn **hiểu bản chất luồng dữ liệu đi qua các trạng thái nào trong Database**, chứ không phải trí nhớ giao diện.
