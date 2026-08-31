# GIAI ĐOẠN 4: THỰC TẬP THỰC TẾ & CHUẨN BỊ XUẤT NGOẠI (CAMPUCHIA & GLOBAL)
> **Thời gian:** Học kỳ cuối & Sau khi tốt nghiệp  
> **Mục tiêu:** Làm việc trực tiếp với Mentor, chuẩn bị toàn diện hành trang sang thị trường tài chính Campuchia (Phnom Penh) và vươn tầm Global Solutions Architect.

---

## 🤝 1. LÀM VIỆC TRỰC TIẾP VỚI MENTOR

### A. Chủ Động Khai Thác Yêu Cầu Dự Án Thực Tế:
* **Hệ sinh thái công nghệ tại Campuchia:**
  - Doanh nghiệp khách hàng thuộc ngành gì (Ngân hàng số, Bán lẻ, Khách sạn - Nghỉ dưỡng, hay Bất động sản)?
  - Hệ thống đang vận hành là **Odoo Enterprise**, **SAP Business One**, hay **SAP S/4HANA**?
  - Bài toán cụ thể cần giải quyết: Viết Module nghiệp vụ mới, tích hợp Cổng thanh toán quốc gia (**NBC Bakong**), hay tối ưu hóa dữ liệu báo cáo tài chính?
* **Tham gia tài liệu thực chiến:**
  - Xin mentor các tài liệu đặc tả yêu cầu nghiệp vụ (**BRD - Business Requirement Document**) hoặc tài liệu thiết kế kiến trúc kỹ thuật (**Technical Architecture Design**) từ các dự án trước để làm quen với ngữ cảnh thực tế.

---

## ✈️ 2. CHUẨN BỊ HÀNH TRANG CHO THỊ TRƯỜNG CAMPUCHIA

1. **Năng Lực Làm Việc Độc Lập:**
   - Khả năng tự tra cứu tài liệu kỹ thuật quốc tế, tự debug lỗi hệ thống phân tán, tự trace log cơ sở dữ liệu và chủ động đề xuất giải pháp.
2. **Kỹ Năng Giao Tiếp Đa Văn Hóa & Tiếng Anh Chuyên Môn:**
   - Sử dụng thành thạo tiếng Anh thương mại trong email, báo cáo kỹ thuật và trình bày giải pháp trước các Giám đốc / Trưởng bộ phận nghiệp vụ.
3. **Bộ Hồ Sơ Năng Lực (Portfolio Đỉnh Cao):**
   - Một CV ấn tượng nêu rõ: *Tốt nghiệp Kỹ sư Kỹ thuật Phần mềm (SE) Chất lượng cao từ PTIT, có năng lực can thiệp sâu kernel Odoo/PostgreSQL, hiểu chuẩn tích hợp SAP OData, và sở hữu nền tảng bảo mật Zero Trust cấp ngân hàng (FAPI 2.0, OpenZiti, Sub-0.35ms PDP).*

---

## 📊 3. BẢNG ĐỐI CHIẾU KIẾN THỨC BẮT BUỘC & KẾ THỪA NỀN TẢNG

| Trụ cột | Nền tảng trường PTIT & Đã có | Kiến thức ERP Bắt buộc phải học | Điểm mù thường gặp |
|---|---|---|---|
| **Lập trình & Ngôn ngữ** | Go, TypeScript, C++, Python cơ bản | Python chuyên sâu cho Odoo (ORM, Decorators, Inheritance) | Chỉ biết prompt AI sinh code mà không tự trace được stack trace khi gặp lỗi runtime. |
| **Cơ sở dữ liệu** | PostgreSQL RLS, B-Tree, NoSQL | PostgreSQL Transaction, Locks, Query Optimization, Foreign Keys | Viết code không kiểm soát được deadlock hoặc làm sai lệch số liệu tồn kho. |
| **Bảo mật & Phân quyền** | Zero Trust, mTLS, DPoP, PBAC/ABAC | Odoo ACL (`ir.model.access`), Odoo Record Rules (`ir.rule`), SAP SoD | Nhầm lẫn giữa quyền truy cập giao diện (UI) và quyền truy cập dữ liệu tầng CSDL. |
| **Tư duy Nghiệp vụ** | Tư duy kỹ thuật, xử lý dữ liệu phân tán | Các luồng kinh doanh chuẩn: Order-to-Cash (O2C), Procure-to-Pay (P2P) | Nghĩ rằng ERP chỉ là bảng nhập liệu CRUD đơn giản mà quên mất tính toàn vẹn tài chính. |
