package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"standalone-policy-engine/internal/audit"
	"standalone-policy-engine/internal/config"
	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/server"
	"standalone-policy-engine/internal/storage"
	"syscall"

	"github.com/openziti/sdk-golang/ziti"
)

func main() {
	log.Println("[PDP-Server] Đang khởi chạy Standalone Policy Decision Point (Data Plane)...")

	// Nạp cấu hình tập trung và xác thực fail-fast
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[PDP-Server] Lỗi cấu hình hệ thống: %v", err)
	}

	// 1. Khởi tạo Database Storage
	store, err := storage.NewStorage(cfg.Database.URL)
	if err != nil {
		log.Fatalf("[PDP-Server] Khởi tạo DB Storage thất bại: %v", err)
	}
	defer store.Close()
	log.Println("[PDP-Server] Kết nối PostgreSQL thành công.")

	ctxServer, stopServer := context.WithCancel(context.Background())
	defer stopServer()

	// 2. Khởi tạo Core Engine có GC dọn dẹp RAM
	eng := engine.NewEngineWithGC(engine.GCConfig{
		Enabled:     !cfg.Engine.DisableGC,
		Interval:    cfg.Engine.GCInterval,
		IdleTimeout: cfg.Engine.GCIdle,
	})
	eng.StartGC(ctxServer)

	// 3. Khởi tạo Cloud-Native Decoupled Stream Audit Logger
	auditLogger := audit.NewStreamAuditLogger(os.Stdout)
	log.Println("[PDP-Server] Khởi chạy Cloud-Native Stream Audit Logger (Stdout/UDS, Zero-GC) thành công.")

	// 4. Khởi tạo Syncer đồng bộ cache nóng qua PostgreSQL LISTEN/NOTIFY
	syncer := engine.NewSyncer(eng, store)

	if cfg.Engine.StorageMode == "edge" {
		badgerStore, err := storage.NewBadgerStore(cfg.Engine.BadgerDir)
		if err != nil {
			log.Printf("[PDP-Server] Cảnh báo: Khởi tạo BadgerStore thất bại: %v", err)
		} else {
			defer badgerStore.Close()
			syncer.SetBadgerStore(badgerStore)
			log.Printf("[PDP-Server] Chế độ EDGE STORAGE kích hoạt: lưu snapshot tại %s", cfg.Engine.BadgerDir)
		}
	} else {
		log.Println("[PDP-Server] Chạy chế độ CLOUD NATIVE: 100% Stateless Pod (Không tạo file BadgerDB cục bộ).")
	}

	// Đăng ký lazyLoader callback để tự động tải lại Tenant từ Postgres khi bị GC unload
	eng.SetLazyLoader(func(ctx context.Context, tenantID string) error {
		syncer.SyncTenant(ctx, tenantID)
		return nil
	})

	syncer.Start(ctxServer)
	log.Println("[PDP-Server] Khởi chạy Syncer đồng bộ cache nóng thành công.")

	// 6. Khởi tạo net.Listener (TCP truyền thống, Unix Domain Socket hoặc Ziti Dark Service)
	var listener net.Listener
	useZiti := cfg.Server.UseZiti
	socketPath := cfg.Server.SocketPath

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
	} else if socketPath != "" {
		_ = os.Remove(socketPath)
		var err error
		listener, err = net.Listen("unix", socketPath)
		if err != nil {
			log.Fatalf("[PDP-Server] Lắng nghe Unix Domain Socket %s thất bại: %v", socketPath, err)
		}
		log.Printf("[PDP-Server] Đang lắng nghe trên Unix Domain Socket (UDS Sidecar IPC): %s", socketPath)
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
	if socketPath != "" {
		_ = os.Remove(socketPath)
	}
	log.Println("[PDP-Server] Dừng dịch vụ hoàn tất. Tạm biệt!")
}
