package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"standalone-policy-engine/internal/config"
	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/server"
	"standalone-policy-engine/internal/storage"
	"syscall"
	"time"
)

func main() {
	log.Println("[Control-Plane] Đang khởi chạy Policy Management API (Control Plane)...")

	// Nạp cấu hình tập trung và xác thực fail-fast
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[Control-Plane] Lỗi cấu hình hệ thống: %v", err)
	}

	// 1. Khởi tạo Database Storage
	store, err := storage.NewStorage(cfg.Database.URL)
	if err != nil {
		log.Fatalf("[Control-Plane] Khởi tạo DB Storage thất bại: %v", err)
	}
	defer store.Close()
	log.Println("[Control-Plane] Kết nối PostgreSQL thành công.")

	// 2. Khởi tạo Engine có GC để phục vụ cho API REST Fallback /decisions
	eng := engine.NewEngineWithGC(engine.GCConfig{
		Enabled:     !cfg.Engine.DisableGC,
		Interval:    cfg.Engine.GCInterval,
		IdleTimeout: cfg.Engine.GCIdle,
	})

	// 3. Khởi chạy HTTP Server
	httpServer, err := server.StartHTTPServer(cfg.Server.HTTPPort, store, eng)
	if err != nil {
		log.Fatalf("[Control-Plane] Không thể chạy HTTP server: %v", err)
	}
	log.Printf("[Control-Plane] HTTP Management API đang lắng nghe tại cổng :%d...", cfg.Server.HTTPPort)

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
