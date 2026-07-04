# Operations, Monitoring & Runbooks Specification

Tài liệu này đặc tả quy trình vận hành (Operations), hệ thống giám sát (Monitoring & SLOs) và cẩm nang xử lý sự cố (Runbooks) cho **Standalone Policy Engine**.

---

## 1. Hệ thống Giám sát & Chỉ số SLO (Monitoring & SLOs)

Chúng ta đo lường độ tin cậy của Policy Engine thông qua 3 chỉ số SLI/SLO cốt lõi:

| Chỉ số SLI | Phương pháp đo | Mục tiêu SLO | Hành động nếu vi phạm SLO |
| :--- | :--- | :--- | :--- |
| **Availability** | Tỷ lệ gRPC requests trả về mã lỗi khác `INTERNAL (13)`. | **`>= 99.99%`** hàng tháng. | Alert cảnh báo On-call Engineer kiểm tra log của cluster ngay lập tức. |
| **Latency** | Thời gian xử lý gRPC CheckPermission toàn trình. | **`< 1.5ms`** ở phân vị P99. | Kiểm tra mức độ nghẽn CPU hoặc hiện tượng nghẽn khóa (Lock Contention) trên RAM. |
| **Cache Sync Delay**| Thời gian lệch phiên bản chính sách giữa DB và RAM của PDP Pods. | **`< 1.0s`** cho 100% Pods. | Cảnh báo nghẽn mạng nội bộ hoặc nghẽn kênh Pub/Sub. |

---

## 2. Cẩm nang Xử lý Sự cố (Runbooks)

### Sự cố 1: Độ trễ gRPC tăng đột biến (> 10ms)
*   **Triệu chứng:** Prometheus giám sát báo hiệu latency P99 vượt quá 10ms.
*   **Nguyên nhân khả dĩ:**
    1.  Hiện tượng nghẽn CPU trên các Pod (Pod bị throttled do giới hạn CPU limit quá thấp).
    2.  Garbage Collector (GC) của Go phải dọn dẹp quá nhiều đối tượng rác sinh ra từ AST (thiếu tối ưu `sync.Pool`).
*   **Quy trình xử lý:**
    1.  Kiểm tra đồ thị CPU trên Grafana. Nếu CPU utilization > 80%, thực hiện Scale-out tăng số lượng Pods:
        ```bash
        kubectl scale deployment pdp-server --replicas=15
        ```
    2.  Kiểm tra tần suất GC pause. Nếu số lần GC pause tăng đột biến, kiểm tra lại code xem có hàm nào đang cấp phát bộ nhớ liên tục trong luồng xử lý chính.

### Sự cố 2: Lệch phiên bản Cache (Cache Incoherence)
*   **Triệu chứng:** Admin cập nhật luật nhưng API Gateway vẫn nhận kết quả phân quyền theo luật cũ.
*   **Nguyên nhân khả dĩ:** Kênh Pub/Sub (Redis) bị ngắt kết nối, PDP Pods không nhận được sự kiện cập nhật để reload cache.
*   **Quy trình xử lý:**
    1.  Kiểm tra kết nối mạng giữa PDP Cluster và Redis.
    2.  Kích hoạt API dọn dẹp và nạp lại thủ công (Hard Reload Cache API) trên Control Plane để cưỡng ép toàn bộ PDP Pods reload cache trực tiếp từ PostgreSQL:
        ```bash
        curl -X POST -H "Authorization: Bearer <token>" http://control-plane.internal/api/v1/cache/reload
        ```

### Sự cố 3: Cơ sở dữ liệu PostgreSQL trung tâm gặp sự cố ngoại tuyến (Database Offline)
*   **Triệu chứng:** Control Plane không thể tạo mới/sửa đổi chính sách. Database connection pool báo lỗi Timeout.
*   **Quy trình xử lý:**
    1.  *PDP (Data Plane) không bị ảnh hưởng:* Hệ thống PDP tiếp tục phục vụ requests bình thường từ RAM cache. Tuyệt đối không khởi động lại (restart) các PDP Pods trong lúc này vì sẽ làm mất dữ liệu RAM cache và không thể nạp lại được.
    2.  Bật chế độ Read-Only trên toàn hệ thống.
    3.  Thực hiện kiểm tra trạng thái PostgreSQL, kích hoạt Node dự phòng (Failover to Replica Pod) để khôi phục Control Plane.
