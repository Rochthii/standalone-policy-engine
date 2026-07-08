package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"standalone-policy-engine/internal/audit"
	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/server"
	"standalone-policy-engine/internal/storage"
	"strings"
	"syscall"
	"time"

	"github.com/openziti/sdk-golang/ziti"
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

	ctxServer, stopServer := context.WithCancel(context.Background())
	defer stopServer()

	// 2. Khởi tạo Redis Universal Client (hỗ trợ Single/Sentinel/Cluster)
	rdb := initRedis()

	// 3. Khởi tạo Core Engine có GC dọn dẹp RAM dựa trên biến môi trường
	disableGC := os.Getenv("DISABLE_GC") == "true"
	gcIntervalStr := os.Getenv("GC_INTERVAL")
	gcIdleStr := os.Getenv("GC_IDLE_TIMEOUT")

	gcInterval := 1 * time.Hour
	if gcIntervalStr != "" {
		if d, err := time.ParseDuration(gcIntervalStr); err == nil {
			gcInterval = d
		}
	}
	gcIdle := 24 * time.Hour
	if gcIdleStr != "" {
		if d, err := time.ParseDuration(gcIdleStr); err == nil {
			gcIdle = d
		}
	}

	eng := engine.NewEngineWithGC(engine.GCConfig{
		Enabled:     !disableGC,
		Interval:    gcInterval,
		IdleTimeout: gcIdle,
	})
	eng.StartGC(ctxServer)

	// 4. Khởi tạo Audit Logger bất đồng bộ
	spillDir := "./spill-logs"
	auditLogger := audit.NewAuditLogger(store, spillDir, 5000)
	auditLogger.Start(ctxServer)
	log.Println("[PDP-Server] Khởi chạy Audit Logger bất đồng bộ (Spill-to-Disk) thành công.")

	// 5. Khởi tạo Syncer đồng bộ cache nóng
	syncer := engine.NewSyncer(eng, store, rdb)
	
	// Đăng ký lazyLoader callback để tự động tải lại Tenant từ Postgres khi bị GC unload
	eng.SetLazyLoader(func(ctx context.Context, tenantID string) error {
		syncer.SyncTenant(ctx, tenantID)
		return nil
	})
	
	syncer.Start(ctxServer)
	log.Println("[PDP-Server] Khởi chạy Syncer đồng bộ cache nóng thành công.")

	// 6. Khởi tạo net.Listener (TCP truyền thống hoặc Ziti Dark Service)
	var listener net.Listener
	useZiti := strings.EqualFold(os.Getenv("USE_ZITI"), "true")

	if useZiti {
		identityPath := os.Getenv("ZITI_IDENTITY_PATH")
		if identityPath == "" {
			identityPath = "docker/identities/pdp-dev.json"
		}
		serviceName := os.Getenv("ZITI_SERVICE_NAME")
		if serviceName == "" {
			serviceName = "policy-decision-service"
		}

		log.Printf("[PDP-Server] Đang kết nối mạng ảo OpenZiti overlay bằng Identity: %s...", identityPath)
		if _, err := os.Stat(identityPath); os.IsNotExist(err) {
			log.Fatalf("[PDP-Server] Lỗi cấu hình Ziti: Không tìm thấy file identity tại %s", identityPath)
		}

		zCfg, err := ziti.NewConfigFromFile(identityPath)
		if err != nil {
			log.Fatalf("[PDP-Server] Load cấu hình Ziti thất bại: %v", err)
		}

		zCtx, err := ziti.NewContext(zCfg)
		if err != nil {
			log.Fatalf("[PDP-Server] Tạo Ziti Context thất bại: %v", err)
		}
		defer zCtx.Close()

		if err := zCtx.Authenticate(); err != nil {
			log.Fatalf("[PDP-Server] Xác thực Ziti Controller thất bại: %v", err)
		}

		log.Printf("[PDP-Server] Đang lắng nghe trên OpenZiti Dark Service: '%s'...", serviceName)
		listener, err = zCtx.Listen(serviceName)
		if err != nil {
			log.Fatalf("[PDP-Server] Không thể lắng nghe trên Ziti service %s: %v", serviceName, err)
		}
		log.Println("[PDP-Server] PDP Dark Service đã khởi chạy thành công. Tất cả cổng TCP inbound công cộng đều được đóng!")
	} else {
		grpcPort := 50051
		addr := fmt.Sprintf(":%d", grpcPort)
		log.Printf("[PDP-Server] Đang chạy chế độ local: lắng nghe trên TCP %s...", addr)
		var err error
		listener, err = net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("[PDP-Server] Lắng nghe cổng TCP %s thất bại: %v", addr, err)
		}
	}

	grpcServer, err := server.StartGRPCServer(listener, eng, auditLogger)
	if err != nil {
		log.Fatalf("[PDP-Server] Không thể chạy gRPC server: %v", err)
	}

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

// initRedis khởi tạo UniversalClient hỗ trợ chế độ Cluster, Sentinel hoặc Single.
func initRedis() redis.UniversalClient {
	mode := os.Getenv("REDIS_MODE") // cluster, sentinel, single (mặc định)
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "localhost:6379"
	}

	var rdb redis.UniversalClient
	switch mode {
	case "cluster":
		addrs := strings.Split(redisAddr, ",")
		rdb = redis.NewClusterClient(&redis.ClusterOptions{
			Addrs: addrs,
		})
		log.Printf("[PDP-Server] Khởi tạo Redis CLUSTER tại %v", addrs)
	case "sentinel":
		sentinelAddrs := os.Getenv("REDIS_SENTINEL_ADDRS")
		addrs := strings.Split(sentinelAddrs, ",")
		masterName := os.Getenv("REDIS_MASTER_NAME")
		if masterName == "" {
			masterName = "mymaster"
		}
		rdb = redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    masterName,
			SentinelAddrs: addrs,
		})
		log.Printf("[PDP-Server] Khởi tạo Redis SENTINEL tại %v (Master: %s)", addrs, masterName)
	default:
		rdb = redis.NewClient(&redis.Options{
			Addr: redisAddr,
		})
		log.Printf("[PDP-Server] Khởi tạo Redis SINGLE tại %s", redisAddr)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Printf("[PDP-Server] Cảnh báo: Không thể ping kết nối Redis (%v). Chạy chế độ Polling fallback.", err)
		return nil
	}
	log.Println("[PDP-Server] Ping Redis thành công.")
	return rdb
}
