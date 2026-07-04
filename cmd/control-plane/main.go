package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/server"
	"standalone-policy-engine/internal/storage"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

func main() {
	log.Println("[Control-Plane] Đang khởi chạy Policy Management API (Control Plane)...")

	// Đọc cấu hình từ biến môi trường
	dbConnStr := os.Getenv("DATABASE_URL")
	if dbConnStr == "" {
		dbConnStr = "postgres://postgres:postgres@localhost:5432/policy_engine?sslmode=disable"
	}

	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	// 1. Khởi tạo Database Storage
	store, err := storage.NewStorage(dbConnStr)
	if err != nil {
		log.Fatalf("[Control-Plane] Khởi tạo DB Storage thất bại: %v", err)
	}
	defer store.Close()
	log.Println("[Control-Plane] Kết nối PostgreSQL thành công.")

	// 2. Khởi tạo Redis Client
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("[Control-Plane] Cảnh báo: Không kết nối được Redis (%v). Tính năng hot reload qua Pub/Sub sẽ bị vô hiệu hóa.", err)
		rdb = nil
	} else {
		log.Println("[Control-Plane] Kết nối Redis thành công.")
	}
	cancel()

	// 3. Khởi tạo Engine trống để phục vụ cho API REST Fallback /decisions
	eng := engine.NewEngine()

	// 4. Khởi chạy HTTP Server (cổng 8080)
	httpPort := 8080
	httpServer, err := server.StartHTTPServer(httpPort, store, eng, rdb)
	if err != nil {
		log.Fatalf("[Control-Plane] Không thể chạy HTTP server: %v", err)
	}
	log.Printf("[Control-Plane] HTTP Management API đang lắng nghe tại cổng :%d...", httpPort)

	// Lắng nghe tín hiệu dừng chương trình
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[Control-Plane] Đang tắt an toàn dịch vụ...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[Control-Plane] HTTP Shutdown lỗi: %v", err)
	}
	log.Println("[Control-Plane] Dừng dịch vụ hoàn tất. Tạm biệt!")
}
