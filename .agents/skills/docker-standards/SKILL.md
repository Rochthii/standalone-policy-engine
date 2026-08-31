---
name: docker-standards
description: Expert rules, debugging heuristics, and operational standards for Docker, Multi-Stage Builds, Database Migrations, and E2E Containerized Testing.
---

# Docker & Containerized DevOps Engineering Standards Skill

## 🎯 Mission
Prevent build failures, eliminate platform-specific encoding traps (UTF-8 BOM), guarantee seamless multi-stage Go image builds, and ensure reliable E2E integration testing on Docker.

---

## 🔑 Critical Hardened Rules & Lessons Learned

### 1. 🐳 Go Toolchain & Base Image Alignment in Dockerfile
- **Cạm bẫy**: Khai báo go 1.25.0+ trong go.mod nhưng dùng image cũ FROM golang:1.22-alpine dẫn đến lỗi go: go.mod requires go >= ....
- **Quy tắc chuẩn**:
  - Luôn sử dụng FROM golang:alpine AS builder (để tự động đồng bộ phiên bản Go mới nhất) hoặc khớp chính xác phiên bản với go.mod.
  - Biên dịch tĩnh hoàn toàn: CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s".
  - Stage 2 tối giản: FROM alpine:3.19 (hoặc latest) và luôn cài đặt ca-certificates.

### 2. 🚫 Vô Hiệu Hóa Tuyệt Đối UTF-8 BOM Trên Windows
- **Cạm bẫy**: Lệnh PowerShell mặc định Set-Content -Encoding UTF8 tự động chèn ký tự vô hình **Byte Order Mark (\ufeff)** vào byte 0. PostgreSQL parser sẽ báo lỗi syntax error at or near "\ufeff" làm "Dirty database" và sập migration.
- **Quy tắc chuẩn**:
  - Khi tạo/sửa file .sql, .json, .yaml, .env, **bắt buộc ghi file không có BOM**:
    `powershell
    $utf8NoBom = New-Object System.Text.UTF8Encoding($false)
    [System.IO.File]::WriteAllText($filePath, $content, $utf8NoBom)
    `

### 3. ⏱️ Docker Compose Healthcheck & Readiness Handshake
- **Cạm bẫy**: Khởi chạy container PDP/Control-Plane trước khi PostgreSQL kịp sẵn sàng nhận kết nối $\to$ CrashLoopBackOff.
- **Quy tắc chuẩn**:
  - Container PostgreSQL 15 phải có `healthcheck` (`pg_isready -U postgres`).
  - Container ứng dụng phải có `depends_on: { db: { condition: service_healthy } }`.
  - Sidecar Vector (`timberio/vector:0.35.0-alpine`) chạy song song, chia sẻ volume UDS socket với PDP server.
  - Hàm `waitForServices(t *testing.T)` trong E2E test phải có vòng lặp kiểm tra cả gRPC `:50051` lẫn HTTP `/metrics` với timeout tối thiểu 30-45 giây.
  - Đồng bộ E2E: Kiểm chứng qua PostgreSQL Monotonic Sequence (`LISTEN/NOTIFY`) và Instant Gap Catch-Up (< 50ms) thay vì kiểm thử timeout Redis cũ.

### 4. 🔄 Khả Năng Tương Thích Ngược Của API Response
- **Cạm bẫy**: Thay đổi tên trường JSON (ví dụ id thành policy_id) làm vỡ các client hoặc bộ test E2E.
- **Quy tắc chuẩn**:
  - Các handler REST API luôn trả về cả 2 trường {"id": "...", "policy_id": "..."} để đảm bảo tương thích ngược 100%.

### 5. 🧹 Quy Trình Dọn Dẹp E2E Container An Toàn
- **Quy tắc chuẩn**:
  - Trước khi chạy test: docker compose down -v và xóa image cũ để tránh cache lỗi BuildKit.
  - Sau khi test xong (defer): docker compose down --rmi local -v để giải phóng tài nguyên hệ thống.