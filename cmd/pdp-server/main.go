package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"standalone-policy-engine/internal/audit"
	"standalone-policy-engine/internal/config"
	"standalone-policy-engine/internal/engine"
	"standalone-policy-engine/internal/security"
	"standalone-policy-engine/internal/server"
	"standalone-policy-engine/internal/storage"
	"syscall"
	"time"

	"github.com/openziti/sdk-golang/ziti"
)

func main() {
	log.Println("[PDP-Server] Đang khởi chạy Standalone Policy Decision Point (Data Plane)...")

	// Nạp cấu hình tập trung và xác thực fail-fast
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[PDP-Server] Lỗi cấu hình hệ thống: %v", err)
	}

	ctxServer, stopServer := context.WithCancel(context.Background())
	defer stopServer()

	// 2. Khởi tạo Core Engine có GC dọn dẹp RAM
	eng := engine.NewEngineWithGC(engine.GCConfig{
		Enabled:     !cfg.Engine.DisableGC,
		Interval:    cfg.Engine.GCInterval,
		IdleTimeout: cfg.Engine.GCIdle,
	})
	eng.StartGC(ctxServer)

	var store *storage.Storage
	var syncer *engine.Syncer
	var auditLogger *audit.AuditLogger
	if cfg.Engine.StorageMode == "edge" {
		badgerStore, err := storage.NewBadgerStore(cfg.Engine.BadgerDir)
		if err != nil {
			log.Fatalf("[PDP-Server] Không thể mở Edge Storage: %v", err)
		}
		defer badgerStore.Close()
		if err := engine.RestoreEdgeSnapshots(eng, badgerStore); err != nil {
			log.Fatalf("[PDP-Server] Không thể khôi phục Edge snapshot: %v", err)
		}
		log.Printf("[PDP-Server] Edge snapshot đã khôi phục từ %s; delegated requests bị từ chối khi không có revocation store.", cfg.Engine.BadgerDir)
	} else {
		store, err = storage.NewStorage(cfg.Database.URL)
		if err != nil {
			log.Fatalf("[PDP-Server] Khởi tạo DB Storage thất bại: %v", err)
		}
		defer store.Close()
		log.Println("[PDP-Server] Kết nối PostgreSQL thành công.")

		auditCrypto, err := security.NewEnvelopeCryptoWithKeyring(cfg.Security.AuditActiveKeyID, cfg.Security.AuditKeys)
		if err != nil {
			log.Fatalf("[PDP-Server] Cấu hình audit encryption thất bại: %v", err)
		}
		var auditWriter audit.BatchWriter = store
		if cfg.Audit.ArchiveBucket != "" {
			archiveSink, err := audit.NewS3ArchiveSink(ctxServer, audit.S3ArchiveConfig{
				Bucket:              cfg.Audit.ArchiveBucket,
				Prefix:              cfg.Audit.ArchivePrefix,
				Region:              cfg.Audit.ArchiveRegion,
				ExpectedBucketOwner: cfg.Audit.ArchiveExpectedBucketOwner,
			})
			if err != nil {
				log.Fatalf("[PDP-Server] Cau hinh immutable audit archive that bai: %v", err)
			}
			auditWriter, err = audit.NewArchivingBatchWriter(store, archiveSink)
			if err != nil {
				log.Fatalf("[PDP-Server] Cau hinh immutable audit writer that bai: %v", err)
			}
			log.Printf("[PDP-Server] Immutable audit archive enabled for S3 bucket %s.", cfg.Audit.ArchiveBucket)
		}
		auditLogger, err = audit.NewBatchAuditLogger(auditWriter, audit.BatchConfig{
			QueueCapacity: cfg.Audit.QueueCapacity,
			BatchSize:     cfg.Audit.BatchSize,
			FlushInterval: cfg.Audit.FlushInterval,
			WriteTimeout:  cfg.Audit.WriteTimeout,
			SpillDir:      cfg.Audit.SpillDir,
			SpillMaxBytes: cfg.Audit.SpillMaxBytes,
			Crypto:        auditCrypto,
		})
		if err != nil {
			log.Fatalf("[PDP-Server] Cấu hình Audit Logger thất bại: %v", err)
		}
		auditLogger.Start(ctxServer)
		log.Println("[PDP-Server] Khởi chạy bounded PostgreSQL Audit Logger thành công.")

		syncer = engine.NewSyncer(eng, store, cfg.Engine.ReconcileInterval)
		eng.SetLazyLoader(func(ctx context.Context, tenantID string) error {
			return syncer.SyncTenant(ctx, tenantID)
		})
		syncer.Start(ctxServer)
		log.Println("[PDP-Server] Chạy chế độ CLOUD NATIVE: 100% Stateless Pod (Không tạo file BadgerDB cục bộ).")
		log.Println("[PDP-Server] Khởi chạy Syncer đồng bộ cache nóng thành công.")
	}

	// 6. Khởi tạo net.Listener (TCP truyền thống, Unix Domain Socket hoặc Ziti Dark Service)
	var listener net.Listener
	useZiti := cfg.Server.UseZiti
	socketPath := cfg.Server.SocketPath

	if useZiti {
		identityPath := cfg.Server.ZitiIdentityPath
		serviceName := cfg.Server.ZitiServiceName

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
		grpcPort := cfg.Server.GRPCPort
		addr := fmt.Sprintf(":%d", grpcPort)
		log.Printf("[PDP-Server] Đang chạy chế độ local: lắng nghe trên TCP %s...", addr)
		var err error
		listener, err = net.Listen("tcp", addr)
		if err != nil {
			log.Fatalf("[PDP-Server] Lắng nghe cổng TCP %s thất bại: %v", addr, err)
		}
	}

	var revocationStore security.RevocationStore
	if store != nil {
		revocationStore = store
	}
	grpcServer, revocationSyncer, err := server.StartGRPCServerWithRevocations(ctxServer, listener, eng, auditLogger, cfg.Security, cfg.Server, revocationStore)
	if err != nil {
		log.Fatalf("[PDP-Server] Không thể chạy gRPC server: %v", err)
	}
	var healthServer *http.Server
	if store != nil {
		healthServer, err = server.StartReadinessServer(cfg.Server.HTTPPort, store, syncer)
	} else {
		healthServer, err = server.StartReadinessServer(cfg.Server.HTTPPort, nil, nil)
	}
	if err != nil {
		log.Fatalf("[PDP-Server] Không thể chạy health server: %v", err)
	}
	log.Printf("[PDP-Server] Health endpoints đang lắng nghe tại cổng :%d...", cfg.Server.HTTPPort)

	// Lắng nghe tín hiệu dừng chương trình (Graceful Shutdown)
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("[PDP-Server] Đang tắt an toàn dịch vụ...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	if err := healthServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("[PDP-Server] Health server shutdown lỗi: %v", err)
	}
	grpcServer.GracefulStop()
	if auditLogger != nil {
		auditLogger.Stop()
	}
	stopServer()
	revocationSyncer.Stop()
	if syncer != nil {
		syncer.Stop()
	}
	if socketPath != "" {
		_ = os.Remove(socketPath)
	}
	log.Println("[PDP-Server] Dừng dịch vụ hoàn tất. Tạm biệt!")
}
