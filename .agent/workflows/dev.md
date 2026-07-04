# Môi trường Phát triển (Development Workflow)

Tài liệu này hướng dẫn cách khởi động, phát triển và chạy thử nghiệm động cơ phân quyền độc lập **Standalone Policy Engine**.

---

## 🚀 Khởi chạy Môi trường Local

### 1. Cài đặt các thư viện cần thiết
Dự án sử dụng gRPC và Protobuf. Để sinh code Go từ file `.proto`, hãy cài đặt:
```bash
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
```

### 2. Biên dịch Protobuf
Mỗi khi có thay đổi trong file `proto/policy.proto`, chạy lệnh sau để sinh code mới:
```bash
protoc --go_out=. --go-grpc_out=. proto/policy.proto
```

### 3. Chạy Server ở chế độ Development
Chạy trực tiếp từ thư mục gốc:
```bash
go run main.go
```
Mặc định server sẽ lắng nghe gRPC tại cổng `50051`.

---

## ⚡ Kiểm thử Hiệu năng & Đo đạc Latency

Để đo đạc độ trễ và khả năng đáp ứng RPS cực lớn, chúng ta sử dụng công cụ benchmark chuyên dụng:
1. **Kiểm tra đơn lẻ qua gRPC CLI:**
   Sử dụng công cụ `grpcurl` để gọi test trực tiếp:
   ```bash
   grpcurl -plaintext -d '{"subject": "user:alice", "action": "DELETE", "resource": "file:doc.pdf"}' localhost:50051 policy.PolicyDecisionPoint/CheckPermission
   ```

2. **Chạy Stress Test bằng ghz:**
   Sử dụng công cụ `ghz` (gRPC benchmarking tool) để tải giả lập 10,000 requests song song:
   ```bash
   ghz --insecure \
     --proto=proto/policy.proto \
     --call=policy.PolicyDecisionPoint.CheckPermission \
     -d '{"subject":"user:alice", "action":"DELETE", "resource":"file:doc.pdf"}' \
     -c 100 -n 100000 localhost:50051
   ```
   **Chỉ tiêu chấp nhận (Acceptance Criteria):**
   - Latency P99 < 1.0 ms
   - Success rate = 100%
