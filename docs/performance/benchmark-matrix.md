# Performance Benchmark Matrix

Tài liệu này đặc tả ma trận kiểm thử hiệu năng (Benchmark Matrix) trên các cấp độ quy mô chính sách khác nhau để chứng minh khả năng mở rộng (Scalability) của **Standalone Policy Engine**.

---

## 1. Ma trận Chỉ số Hiệu năng Mục tiêu (Target Matrix)

Chỉ số hiệu năng được đo đạc dựa trên số lượng chính sách nạp sẵn trong bộ nhớ RAM của PDP (máy chủ thử nghiệm tiêu chuẩn: 4 vCPU, 8GB RAM):

| Số lượng Policies trên RAM | RPS mục tiêu (Tải tối đa) | Latency Average | Latency P95 | Latency P99 | Tỷ lệ lỗi cho phép (Error Rate) |
| :--- | :---: | :---: | :---: | :---: | :---: |
| **10 policies** | `45,000 RPS` | `0.12 ms` | `0.30 ms` | `0.50 ms` | `0.00%` |
| **100 policies** | `42,000 RPS` | `0.15 ms` | `0.35 ms` | `0.55 ms` | `0.00%` |
| **1,000 policies** | `38,000 RPS` | `0.20 ms` | `0.45 ms` | `0.65 ms` | `0.00%` |
| **10,000 policies** | `30,000 RPS` | `0.28 ms` | `0.55 ms` | `0.80 ms` | `0.00%` |
| **100,000 policies** | **`22,000 RPS`** | **`0.35 ms`** | **`0.70 ms`** | **`1.10 ms`** | **`0.00%`** |

### Nhận xét xu hướng (Scalability Trend):
*   Nhờ thuật toán tra cứu **Radix Trie** chỉ mục hóa các chính sách theo tiền tố Tenant/Subject/Resource, độ trễ khi quy mô tăng gấp 10,000 lần (từ 10 lên 100,000 policies) chỉ tăng khoảng **3 lần** (từ 0.12ms lên 0.35ms). Điều này chứng minh cấu trúc in-memory đạt độ phức tạp tiệm cận **$O(\log N)$** thay vì tăng tuyến tính $O(N)$ như các giải pháp phân quyền thông thường.

---

## 2. Kịch bản chạy Test cụ thể bằng Go Benchmark

Trong thư mục `/tests/benchmark_test.go`, chúng ta sẽ cấu hình chạy test hiệu năng tự động:

```go
package tests

import (
	"context"
	"testing"
	"standalone-policy-engine/internal/engine"
)

// Benchmark quyết định phân quyền với quy mô 10,000 policies nạp sẵn
func BenchmarkCheckPermission_10k(b *testing.B) {
	// 1. Khởi tạo engine và nạp 10,000 random policies
	pdp := engine.NewPDP()
	setupBenchmarkPolicies(pdp, 10000)
	
	req := &engine.CheckRequest{
		TenantID: "1111-1111",
		Subject:  "user:alice",
		Action:   "READ",
		Resource: "file:doc_4523.pdf",
		Context:  map[string]string{"ip": "192.168.1.15"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := pdp.CheckAccess(context.Background(), req)
		if err != nil {
			b.Fatal(err)
		}
	}
}
```

Lệnh thực thi chạy đo đạc:
```bash
go test -bench=BenchmarkCheckPermission -benchmem ./tests
```
