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
	"strings"
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

	// 2. Khởi tạo Redis Universal Client (hỗ trợ Single/Sentinel/Cluster)
	rdb := initRedis()

	// 3. Khởi tạo Engine có GC để phục vụ cho API REST Fallback /decisions
	eng := engine.NewEngineWithGC(1*time.Hour, 24*time.Hour)

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

// initRedis khởi tạo UniversalClient hỗ trợ chế độ Cluster, Sentinel hoặc Single.
func initRedis() redis.UniversalClient {
	mode := os.Getenv("REDIS_MODE")
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	var rdb redis.UniversalClient
	switch mode {
	case "cluster":
		addrs := strings.Split(redisAddr, ",")
		rdb = redis.NewClusterClient(&redis.ClusterOptions{Addrs: addrs})
		log.Printf("[Control-Plane] Khởi tạo Redis CLUSTER tại %v", addrs)
	case "sentinel":
		sentinelAddrs := strings.Split(os.Getenv("REDIS_SENTINEL_ADDRS"), ",")
		masterName := os.Getenv("REDIS_MASTER_NAME")
		if masterName == "" {
			masterName = "mymaster"
		}
		rdb = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    masterName,
			SentinelAddrs: sentinelAddrs,
		})
		log.Printf("[Control-Plane] Khởi tạo Redis SENTINEL (Master: %s)", masterName)
	default:
		rdb = redis.NewClient(&redis.Options{Addr: redisAddr})
		log.Printf("[Control-Plane] Khởi tạo Redis SINGLE tại %s", redisAddr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("[Control-Plane] Cảnh báo: Không thể ping Redis (%v). Tính năng Pub/Sub bị vô hiệu hóa.", err)
		return nil
	}
	log.Println("[Control-Plane] Ping Redis thành công.")
	return rdb
}
