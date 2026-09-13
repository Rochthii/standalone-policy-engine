package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config chứa toàn bộ cấu hình hệ thống được nạp từ biến môi trường (12-Factor App).
type Config struct {
	AppEnv   string
	Server   ServerConfig
	Database DatabaseConfig
	Redis    RedisConfig
	Engine   EngineConfig
	Audit    AuditConfig
	Security SecurityConfig
}

type ServerConfig struct {
	HTTPPort            int
	GRPCPort            int
	SocketPath          string
	UseZiti             bool
	EvaluationTimeout   time.Duration
	GRPCMaxReceiveBytes int
	GRPCMaxSendBytes    int
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	URL string
}

type EngineConfig struct {
	StorageMode       string // "cloud" hoặc "edge"
	BadgerDir         string
	DisableGC         bool
	GCInterval        time.Duration
	GCIdle            time.Duration
	ReconcileInterval time.Duration
}

type AuditConfig struct {
	SocketPath    string
	SpillDir      string
	QueueCapacity int
	BatchSize     int
	FlushInterval time.Duration
	WriteTimeout  time.Duration
	SpillMaxBytes int64
}

// Load nạp cấu hình từ môi trường và kiểm tra tính hợp lệ (Fail-Fast Validation).
func Load() (*Config, error) {
	appEnv := getEnv("APP_ENV", "development")

	httpPort, err := getEnvInt("HTTP_PORT", 8080)
	if err != nil {
		return nil, fmt.Errorf("HTTP_PORT khong hop le: %w", err)
	}

	grpcPort, err := getEnvInt("GRPC_PORT", 50051)
	if err != nil {
		return nil, fmt.Errorf("GRPC_PORT khong hop le: %w", err)
	}
	evaluationTimeout, err := getEnvDuration("GRPC_EVALUATION_TIMEOUT", 100*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("GRPC_EVALUATION_TIMEOUT khong hop le: %w", err)
	}
	grpcMaxReceiveBytes, err := getEnvInt("GRPC_MAX_RECEIVE_BYTES", 1024*1024)
	if err != nil {
		return nil, fmt.Errorf("GRPC_MAX_RECEIVE_BYTES khong hop le: %w", err)
	}
	grpcMaxSendBytes, err := getEnvInt("GRPC_MAX_SEND_BYTES", 1024*1024)
	if err != nil {
		return nil, fmt.Errorf("GRPC_MAX_SEND_BYTES khong hop le: %w", err)
	}
	if evaluationTimeout <= 0 || grpcMaxReceiveBytes < 1024 || grpcMaxSendBytes < 1024 {
		return nil, errors.New("gRPC timeout phai duong va message limits phai toi thieu 1024 bytes")
	}

	gcInterval, err := getEnvDuration("GC_INTERVAL", 1*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("GC_INTERVAL khong hop le: %w", err)
	}

	gcIdle, err := getEnvDuration("GC_IDLE_TIMEOUT", 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("GC_IDLE_TIMEOUT khong hop le: %w", err)
	}
	reconcileInterval, err := getEnvDuration("SYNC_RECONCILE_INTERVAL", 10*time.Second)
	if err != nil {
		return nil, fmt.Errorf("SYNC_RECONCILE_INTERVAL khong hop le: %w", err)
	}
	if reconcileInterval <= 0 {
		return nil, errors.New("SYNC_RECONCILE_INTERVAL phai lon hon 0")
	}
	auditQueueCapacity, err := getEnvInt("AUDIT_QUEUE_CAPACITY", 8192)
	if err != nil {
		return nil, fmt.Errorf("AUDIT_QUEUE_CAPACITY khong hop le: %w", err)
	}
	auditBatchSize, err := getEnvInt("AUDIT_BATCH_SIZE", 128)
	if err != nil {
		return nil, fmt.Errorf("AUDIT_BATCH_SIZE khong hop le: %w", err)
	}
	auditFlushInterval, err := getEnvDuration("AUDIT_FLUSH_INTERVAL", 100*time.Millisecond)
	if err != nil {
		return nil, fmt.Errorf("AUDIT_FLUSH_INTERVAL khong hop le: %w", err)
	}
	auditWriteTimeout, err := getEnvDuration("AUDIT_WRITE_TIMEOUT", 2*time.Second)
	if err != nil {
		return nil, fmt.Errorf("AUDIT_WRITE_TIMEOUT khong hop le: %w", err)
	}
	if auditQueueCapacity <= 0 || auditBatchSize <= 0 || auditBatchSize > auditQueueCapacity {
		return nil, errors.New("AUDIT_QUEUE_CAPACITY va AUDIT_BATCH_SIZE phai duong, batch khong vuot queue")
	}
	if auditFlushInterval <= 0 || auditWriteTimeout <= 0 {
		return nil, errors.New("AUDIT_FLUSH_INTERVAL va AUDIT_WRITE_TIMEOUT phai lon hon 0")
	}
	auditSpillMaxBytes, err := getEnvInt64("AUDIT_SPILL_MAX_BYTES", 1<<30)
	if err != nil || auditSpillMaxBytes <= 0 {
		return nil, errors.New("AUDIT_SPILL_MAX_BYTES phai la so nguyen duong")
	}

	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/policy_engine?sslmode=disable")
	delegationSecret := getEnv("PDP_SHARED_SECRET", developmentDelegationSecret)
	delegationActiveKeyID, delegationKeys, delegationKeyringExplicit, err := loadDelegationKeyring(delegationSecret)
	if err != nil {
		return nil, err
	}
	auditActiveKeyID, auditKeys, auditKeyringExplicit, err := loadAuditKeyring()
	if err != nil {
		return nil, err
	}
	cfg := &Config{
		AppEnv: appEnv,
		Server: ServerConfig{
			HTTPPort:            httpPort,
			GRPCPort:            grpcPort,
			SocketPath:          getEnv("LISTEN_SOCKET_PATH", ""),
			UseZiti:             strings.EqualFold(getEnv("USE_ZITI", "false"), "true"),
			EvaluationTimeout:   evaluationTimeout,
			GRPCMaxReceiveBytes: grpcMaxReceiveBytes,
			GRPCMaxSendBytes:    grpcMaxSendBytes,
		},
		Database: DatabaseConfig{
			URL: dbURL,
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "localhost:6379"),
		},
		Engine: EngineConfig{
			StorageMode:       strings.ToLower(getEnv("STORAGE_MODE", "cloud")),
			BadgerDir:         getEnv("BADGER_DATA_DIR", "./badger-data"),
			DisableGC:         strings.EqualFold(getEnv("DISABLE_GC", "false"), "true"),
			GCInterval:        gcInterval,
			GCIdle:            gcIdle,
			ReconcileInterval: reconcileInterval,
		},
		Audit: AuditConfig{
			SocketPath:    getEnv("AUDIT_SOCKET_PATH", "/var/run/vector/audit.sock"),
			SpillDir:      getEnv("AUDIT_SPILL_DIR", "./spill-logs"),
			QueueCapacity: auditQueueCapacity,
			BatchSize:     auditBatchSize,
			FlushInterval: auditFlushInterval,
			WriteTimeout:  auditWriteTimeout,
			SpillMaxBytes: auditSpillMaxBytes,
		},
		Security: SecurityConfig{
			JWTSecret:                 getEnv("JWT_SECRET", developmentJWTSecret),
			JWTIssuer:                 getEnv("JWT_ISSUER", "standalone-policy-engine-dev"),
			JWTAudience:               getEnv("JWT_AUDIENCE", "standalone-policy-engine-pdp"),
			DelegationSecret:          delegationSecret,
			DelegationActiveKeyID:     delegationActiveKeyID,
			DelegationKeys:            delegationKeys,
			DelegationKeyringExplicit: delegationKeyringExplicit,
			TLSCertFile:               getEnv("PDP_TLS_CERT", ""),
			TLSKeyFile:                getEnv("PDP_TLS_KEY", ""),
			TLSCAFile:                 getEnv("PDP_TLS_CA", ""),
			AuditActiveKeyID:          auditActiveKeyID,
			AuditKeys:                 auditKeys,
			AuditKeyringExplicit:      auditKeyringExplicit,
		},
	}

	if err := validateProductionConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) (int, error) {
	valStr := getEnv(key, "")
	if valStr == "" {
		return defaultVal, nil
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return 0, err
	}
	return val, nil
}

func getEnvInt64(key string, defaultVal int64) (int64, error) {
	valStr := getEnv(key, "")
	if valStr == "" {
		return defaultVal, nil
	}
	return strconv.ParseInt(valStr, 10, 64)
}

func getEnvDuration(key string, defaultVal time.Duration) (time.Duration, error) {
	valStr := getEnv(key, "")
	if valStr == "" {
		return defaultVal, nil
	}
	d, err := time.ParseDuration(valStr)
	if err != nil {
		return 0, err
	}
	return d, nil
}
