# Kết Quả Đánh Giá Hiệu Năng (Benchmark Results)

Dưới đây là kết quả kiểm thử hiệu năng và độ trễ của Standalone Policy Engine khi hoạt động dưới tải lớn và xử lý tập chính sách thực tế.

## 1. Môi Trường Thử Nghiệm
* **Hệ điều hành**: Windows 11 (WSL2 / Local Development Environment)
* **Bộ vi xử lý (CPU)**: 13th Gen Intel(R) Core(TM) i7-13700H (20 vCPUs)
* **Bộ nhớ (RAM)**: 16 GB DDR5
* **Runtime**: Go 1.22.5 (local portable)

## 2. Kết Quả Đo Đạc

Kiểm thử được thực hiện với **1000 chính sách phức tạp** của một Tenant được nạp sẵn trên bộ nhớ RAM:

| Benchmark Case | Số Lần Thực Thi (Runs) | Độ Trễ Trung Bình (ns/op) | Dung Lượng Cấp Phát (B/op) | Số Lần Alloc (allocs/op) |
| :--- | :--- | :--- | :--- | :--- |
| **BenchmarkEvaluatorLatency** | 291,651 | **4,433 ns** (~0.004 ms) | 1,521 B | 39 |
| **BenchmarkConcurrentLoad** (Tải đồng thời) | 2,569,099 | **468 ns** (~0.0004 ms) | 532 B | 13 |

---

## 3. Nhận Xét & Phân Tích

1. **Hiệu năng siêu tốc**:
   - Độ trễ đánh giá quyết định đơn luồng chỉ là **4.4 microseconds** (đáp ứng xuất sắc cam kết SLA latency < 1ms dưới tải lớn).
   - Dưới tải đồng thời song song đa luồng (`BenchmarkConcurrentLoad`), nhờ kiến trúc Lock-Free và chỉ mục Trie phân cấp tối ưu, hiệu năng tăng đáng kể khi chỉ mất **468 nanoseconds** cho một quyết định phân quyền. Hệ thống dễ dàng đạt hàng triệu RPS.
2. **GC Pressure & Memory Allocations**:
   - Số lượng allocations cực nhỏ (chỉ 13 - 39 allocs trên mỗi request) đảm bảo hệ thống không bị nghẽn Garbage Collection (GC) khi chạy dưới tải cực lớn dài hạn.
3. **Kết luận**:
   - Động cơ phân quyền in-memory chạy chính xác, đáp ứng đầy đủ yêu cầu SLA latency và tải trọng cao của dự án. Không cần áp dụng thêm cơ chế tối ưu LRU Cache phụ vì Trie in-memory đã quá nhanh.
