# Giai Đoạn 3: Hoàn Thiện Đồ Án Tốt Nghiệp Xuất Sắc & Săn Thực Tập
> **Thời gian:** Năm 4 (Tháng 7 — Tháng 12) | **Mục tiêu:** Đạt điểm Đồ án Tốt nghiệp 9.5–10 & Nhận Offer Thực tập/Fresher có lương

---

## 🎯 1. Mục Tiêu Cốt Lõi (Learning Objectives)

1. Chọn chuyên ngành **Kỹ thuật Phần mềm (Software Engineering)** và bảo vệ đề tài Đồ án Tốt nghiệp xuất sắc.
2. Hoàn thiện toàn bộ hệ thống thuyết minh đồ án, bảng đo lường benchmark và kịch bản **Live Demo ấn tượng**.
3. Rải CV và vượt qua các vòng phỏng vấn kỹ thuật để giành vị trí Thực tập sinh / Kỹ sư chính thức tại các doanh nghiệp hàng đầu.

---

## 📚 2. Cần Chuẩn Bị Những Gì? (Hồ Sơ Đồ Án & Phỏng Vấn)

### A. Cấu Trúc Báo Cáo Thuyết Minh Đồ Án Chuẩn Mực (70–100 Trang)
Sử dụng trực tiếp kho tài liệu 35+ file trong thư mục `docs/` của dự án:
- **Chương 1: Tổng quan & Đặt vấn đề:** Thực trạng phân quyền trong ERP, giới hạn của RBAC, nhu cầu chuyển dịch sang ABAC/PBAC.
- **Chương 2: Cơ sở lý thuyết & Mô hình phân quyền:** Tiêu chuẩn NIST SP 800-162, kiến trúc PEP-PDP-PIP-PAP, triết lý Policy-as-Code.
- **Chương 3: Thiết kế kiến trúc hệ thống Standalone PDP:** Cấu trúc Trie Radix $O(\log N)$, Role DAG Transitive Closure $O(1)$, AST Compiler, Copy-On-Write, Async Ring Buffer WORM Audit Log, Zero Trust mTLS / OpenZiti.
- **Chương 4: Hiện thực hóa & Thử nghiệm trên các phân hệ ERP:** 7 kịch bản kiểm thử thực tế (PO Approval, SoD, Multi-Branch, Payroll).
- **Chương 5: Đánh giá thực nghiệm & Đo lường hiệu năng:** Bảng số liệu Benchmark Latency ($< 1$ µs) và Throughput ($> 1$M RPS).
- **Chương 6: Kết luận & Hướng phát triển.**

### B. Danh Sách Doanh Nghiệp Mục Tiêu Nộp CV:
1. **Khối Doanh nghiệp Công nghệ Triển khai ERP lớn:**
   - **FPT Software:** Các đơn vị chuyên về SAP (FJP, FHN, FHM) hoặc Odoo/Enterprise Solutions.
   - **Viettel Solutions / VNPT-IT:** Mảng chuyển đổi số doanh nghiệp lớn.
   - **CMC Global / BAP / SmartOSC / A1 Consulting:** Các đối tác lớn của Odoo & SAP tại Việt Nam.
2. **Khối Tư vấn Quốc tế (Technology Consulting):**
   - **Big 4:** Deloitte, PwC, EY, KPMG (Chuyên môn Technology Risk, SAP Implementation, Enterprise IT Advisory).

---

## 🛠️ 3. Cần Làm Những Gì? (Kịch Bản Live Demo Bảo Vệ)

```text
┌─────────────────────────────────────────────────────────────────────────────┐
│                    KỊCH BẢN LIVE DEMO 5 PHÚT TRƯỚC HỘI ĐỒNG                 │
├─────────────────────────────────────────────────────────────────────────────┤
│  PHÚT 1: Giới thiệu kiến trúc tổng thể (Envoy PEP ↔ PDP Go ↔ PostgreSQL)   │
│  PHÚT 2: Demo tính năng phân quyền trên Odoo thật (PO Approval & SoD)       │
│  PHÚT 3: Demo tính năng giải thích quyết định (ExplainDecision gRPC)        │
│  PHÚT 4: Demo Hot-Reload < 300ms qua Redis Pub/Sub và Spill-to-Disk khi DB sập│
│  PHÚT 5: Chạy Benchmark trực tiếp: Show màn hình > 1.000.000 RPS, < 1µs RAM │
└─────────────────────────────────────────────────────────────────────────────┘
```

- [ ] **Lệnh chạy Benchmark trực tiếp trước Hội đồng:**
  ```bash
  go test -bench=BenchmarkERP -benchmem ./tests/erp_scenarios_test.go
  ```
- [ ] **Lệnh kiểm thử 7 kịch bản ERP thực tế:**
  ```bash
  go test -v ./tests/erp_scenarios_test.go
  ```

---

## 🎤 4. Bộ Câu Hỏi Phỏng Vấn Kinh Điển & Cách Trả Lời "Ăn Điểm"

### Câu 1: *"Tại sao em không dùng luôn phân quyền có sẵn của Odoo/SAP mà lại viết riêng Standalone PDP?"*
* **Trả lời chuẩn:** *"Phân quyền có sẵn của Odoo hoạt động bằng cách chèn Domain Filter vào câu SQL `WHERE`, khiến Database bị nghẽn khi có hàng triệu bản ghi. Phân quyền SAP truyền thống phụ thuộc vào mã T-Code tĩnh. Standalone PDP của em tách rời hoàn toàn logic kiểm tra ra khỏi ứng dụng, đánh giá trong RAM với độ trễ chỉ $0.9$ µs và thông lượng $1.000.000$ RPS, đồng thời thống nhất một chính sách chung cho cả Odoo và SAP."*

### Câu 2: *"Cơ chế nào giúp hệ thống của em đạt được độ trễ dưới 1 microsecond?"*
* **Trả lời chuẩn:** *"Hệ thống kết hợp 3 kỹ thuật: (1) Chỉ mục Trie 4 tầng dùng mã băm FNV-1a 64-bit loại bỏ so sánh chuỗi, (2) Đồ thị Role DAG tính sẵn Bao đóng bắc cầu (Transitive Closure) để kiểm tra kế thừa vai trò chỉ mất $O(1)$, và (3) Tối ưu hóa bộ nhớ Zero-Allocation qua `sync.Pool` và con trỏ nguyên tử Copy-On-Write."*

---

## 🎁 5. Thu Được Những Gì? (Deliverables & Kết Quả)

1. **Điểm khóa luận:** Đạt loại Xuất sắc (9.5 — 10.0 điểm).
2. **Việc làm:** Offer chính thức (Fresher / Junior SAP/Odoo/Golang Engineer) ngay trước khi nhận bằng tốt nghiệp.
3. **Mức lương khởi điểm kỳ vọng:** $500 - $1,000/tháng (vượt trội so với mặt bằng chung sinh viên mới ra trường).
