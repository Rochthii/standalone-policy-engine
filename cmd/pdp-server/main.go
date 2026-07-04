package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"standalone-policy-engine/internal/audit"
	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/server"
	"standalone-policy-engine/internal/storage"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

func main() {
	log.Println("[PDP-Server] Đang khởi chạy Standalone Policy Decision Point (Data Plane)...")

	// Đọc cấu hình từ biến môi trường
	dbConnStr := os.Getenv("DATABASE_URL")
	if dbConnStr == "" {
		// Mặc định cho môi trường dev cục bộ
		dbConnStr = "postgres://postgres:postgres@localhost:5432/policy_engine?sslmode=disable"
	}

	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// 1. Khởi tạo Database Storage
	store, err := storage.NewStorage(dbConnStr)
	if err != nil {
		log.Fatalf("[PDP-Server] Khởi tạo DB Storage thất bại: %v", err)
	}
	defer store.Close()
	log.Println("[PDP-Server] Kết nối PostgreSQL thành công.")

	// 2. Khởi tạo Redis Client
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	// Kiểm tra kết nối Redis
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("[PDP-Server] Cảnh báo: Không thể kết nối Redis (%v). Sẽ chuyển sang chế độ Polling dự phòng.", err)
		rdb = nil
	} else {
		log.Println("[PDP-Server] Kết nối Redis thành công.")
	}
	cancel()

	// 3. Khởi tạo Core Engine
	eng := engine.NewEngine()

	// 4. Khởi tạo Audit Logger bất đồng bộ
	// Thư mục spill logs cục bộ trên SSD
	spillDir := "./spill-logs"
	auditLogger := audit.NewAuditLogger(store, spillDir, 5000)
	
	ctxServer, stopServer := context.WithCancel(context.Background())
	defer stopServer()

	auditLogger.Start(ctxServer)
	log.Println("[PDP-Server] Khởi chạy Audit Logger bất đồng bộ (Spill-to-Disk) thành công.")

	// 5. Khởi tạo Syncer đồng bộ cache nóng
	syncer := engine.NewSyncer(eng, store, rdb)
	syncer.Start(ctxServer)
	log.Println("[PDP-Server] Khởi chạy Syncer đồng bộ cache nóng thành công.")

	// 6. Khởi chạy gRPC Server (cổng 50051)
	grpcPort := 50051
	grpcServer, err := server.StartGRPCServer(grpcPort, eng, auditLogger)
	if err != nil {
		log.Fatalf("[PDP-Server] Không thể chạy gRPC server: %v", err)
	}
	log.Printf("[PDP-Server] gRPC Server đang lắng nghe tại cổng :%d...", grpcPort)

	// Lắng nghe tín hiệu dừng chương trình (Graceful Shutdown)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[PDP-Server] Đang tắt an toàn dịch vụ...")
	grpcServer.GracefulStop()
	syncer.Stop()
	auditLogger.Stop()
	log.Println("[PDP-Server] Dừng dịch vụ hoàn tất. Tạm biệt!")
}
