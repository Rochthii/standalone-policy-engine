# Giai Đoạn 1: Làm Chủ Ruột Gan ERP Với Odoo & Lập Trình Phân Tán
> **Thời gian:** Năm 3 (Tháng 1 — Tháng 3) | **Mục tiêu:** Hiểu thấu 100% cách ERP tổ chức dữ liệu & dòng tiền

---

## 🎯 1. Mục Tiêu Cốt Lõi (Learning Objectives)

1. Nắm vững 4 luồng chu chuyển nghiệp vụ và dòng tiền trong doanh nghiệp.
2. Hiểu cấu trúc Database thực tế của một hệ thống ERP (Bảng chứng từ, bảng chi tiết, khóa ngoại, trạng thái).
3. Tự tay lập trình 1 Custom Module Odoo bằng Python kết nối trực tiếp với **Standalone PDP Engine (Go)** qua gRPC/REST.

---

## 📚 2. Cần Học Những Gì? (Kiến Thức & Tài Liệu Chuẩn)

### A. 4 Chu Trình Kinh Tế Cốt Lõi (Business Flow Mastery)
- **Procure-to-Pay (P2P - Mua sắm & Thanh toán):**
  $$\text{Purchase Requisition (PR)} \xrightarrow{\text{Approve}} \text{Purchase Order (PO)} \xrightarrow{\text{Receive}} \text{Stock Picking (GRN)} \xrightarrow{\text{Bill}} \text{Vendor Bill} \xrightarrow{\text{Pay}} \text{Payment}$$
- **Order-to-Cash (O2C - Bán hàng & Thu tiền):**
  $$\text{Quotation} \xrightarrow{\text{Confirm}} \text{Sales Order (SO)} \xrightarrow{\text{Deliver}} \text{Stock Delivery} \xrightarrow{\text{Invoice}} \text{Customer Invoice} \xrightarrow{\text{Reconcile}} \text{Payment}$$
- **Kế toán Kép (Double-Entry Bookkeeping):**
  - Nguyên tắc vàng: $\sum \text{Debit (Nợ)} = \sum \text{Credit (Có)}$ trên mọi bút toán (`account.move.line`).
  - Phân loại tài khoản: Tài sản (1xx, 2xx), Nợ phải trả (3xx), Vốn chủ sở hữu (4xx), Doanh thu (5xx), Chi phí (6xx, 8xx).
- **Kho vận (Double-Entry Inventory):**
  - Hàng hóa không tự nhiên sinh ra hay mất đi: Luôn chuyển từ vị trí nguồn (`location_id`) sang vị trí đích (`location_dest_id`).

### B. Kỹ Thuật Lập Trình Odoo & PostgreSQL
- **Odoo ORM:** Models (`models.Model`), Trường dữ liệu (`fields.Char`, `fields.Float`, `fields.Many2one`, `fields.One2many`), Decorators (`@api.depends`, `@api.onchange`, `@api.constrains`).
- **Odoo Security:**
  - `ir.model.access.csv`: Phân quyền CRUD trên Model (tương đương RBAC).
  - `ir.rule`: Phân quyền trên từng dòng bản ghi (Record Rules) bằng Python Domain (chính là ABAC).
- **Tài liệu học tập:**
  - [Odoo Developer Documentation 17.0](https://www.odoo.com/documentation/17.0/developer.html).
  - Sách: *"Odoo 15/16/17 Development Essentials"* — Daniel Reis.

---

## 🛠️ 3. Cần Làm Những Gì? (Checklist Thực Chiến Từng Tuần)

### Tuần 1–2: Dựng Hạ Tầng & Khám Phá Quy Trình Chuẩn
- [ ] Viết file `docker-compose.yml` khởi chạy Odoo 17 Community + PostgreSQL 16.
```yaml
version: '3.8'
services:
  web:
    image: odoo:17.0
    depends_on:
      - db
    ports:
      - "8069:8069"
    environment:
      - HOST=db
      - USER=odoo
      - PASSWORD=odoo
    volumes:
      - odoo-web-data:/var/lib/odoo
      - ./custom-addons:/mnt/extra-addons
  db:
    image: postgres:16
    environment:
      - POSTGRES_DB=postgres
      - POSTGRES_PASSWORD=odoo
      - POSTGRES_USER=odoo
    volumes:
      - odoo-db-data:/var/lib/postgresql/data
volumes:
  odoo-web-data:
  odoo-db-data:
```
- [ ] Tạo cơ sở dữ liệu mẫu có sẵn demo data. Thực hành trọn vẹn 1 chu trình P2P và 1 chu trình O2C trên giao diện Web.

### Tuần 3–6: Viết Module Custom Odoo Kết Nối Go PDP Engine
- [ ] Tạo module Odoo: `custom-addons/pdp_authorization_connector/`.
- [ ] Định nghĩa logic kiểm tra quyền động khi duyệt Đơn mua hàng (Purchase Order):
```python
# custom-addons/pdp_authorization_connector/models/purchase_order.py
from odoo import models, api, exceptions
import requests

class PurchaseOrder(models.Model):
    _inherit = 'purchase.order'

    def button_approve(self, force=False):
        for order in self:
            # 1. Thu thập ngữ cảnh ABAC từ Odoo
            payload = {
                "tenant_id": f"company_{order.company_id.id}",
                "subject": f"user:{self.env.user.login}",
                "action": "APPROVE_PO",
                "resource": "resource:purchase_order",
                "context": {
                    "principal.department": self.env.user.department_id.name or "UNKNOWN",
                    "resource.department": order.department_id.name or "UNKNOWN",
                    "resource.amount": str(order.amount_total),
                    "resource.creator_id": f"user:{order.create_uid.login}"
                }
            }
            
            # 2. Gửi yêu cầu phân quyền sang Standalone PDP Engine (Port 8080/50051)
            try:
                resp = requests.post("http://host.docker.internal:8080/api/v1/decisions", json=payload, timeout=2.0)
                decision = resp.json().get("decision")
                if decision != 1: # 1 == ALLOW, 0 == DENY
                    reason = resp.json().get("reason", "Bị từ chối bởi chính sách an toàn")
                    raise exceptions.UserError(f"PDP Authorization Rejected: {reason}")
            except Exception as e:
                raise exceptions.UserError(f"Lỗi kiểm tra quyền phân tán: {str(e)}")

        return super(PurchaseOrder, self).button_approve(force=force)
```
- [ ] Kiểm thử 3 trường hợp:
  - Trưởng phòng duyệt PO đúng phòng ban và số tiền $\le$ hạn mức $\rightarrow$ Duyệt thành công (`ALLOW`).
  - Người tạo PO cố tình tự duyệt PO của mình $\rightarrow$ Bị chặn bởi luật cấm SoD (`DENY`).
  - Duyệt PO vượt hạn mức $\rightarrow$ Bị chặn (`DENY`).

---

## 🎁 4. Thu Được Những Gì? (Deliverables & CV Points)

1. **Sản phẩm bàn giao:** Repository GitHub chứa module tích hợp Odoo $\leftrightarrow$ Go PDP Engine hoàn chỉnh.
2. **Kỹ năng đạt được:** Thành thạo kiến trúc module Odoo, mô hình hóa dữ liệu quan hệ trong PostgreSQL, tích hợp microservices gRPC/REST.
3. **Bullet Point đưa vào CV:**
   > *"Engineered custom Odoo 17 enterprise modules with distributed gRPC authorization hooks, offloading dynamic ABAC decision logic to an in-memory Go Policy Decision Point with sub-millisecond evaluation latency."*
