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
	HTTPPort   int
	GRPCPort   int
	SocketPath string
	UseZiti    bool
}

type DatabaseConfig struct {
	URL string
}

type RedisConfig struct {
	URL string
}

type EngineConfig struct {
	StorageMode string // "cloud" hoặc "edge"
	BadgerDir   string
	DisableGC   bool
	GCInterval  time.Duration
	GCIdle      time.Duration
}

type AuditConfig struct {
	SocketPath string
}

type SecurityConfig struct {
	JWTSecret string
	LogKEK    string
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

	gcInterval, err := getEnvDuration("GC_INTERVAL", 1*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("GC_INTERVAL khong hop le: %w", err)
	}

	gcIdle, err := getEnvDuration("GC_IDLE_TIMEOUT", 24*time.Hour)
	if err != nil {
		return nil, fmt.Errorf("GC_IDLE_TIMEOUT khong hop le: %w", err)
	}

	dbURL := getEnv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/policy_engine?sslmode=disable")
	if appEnv == "production" && (dbURL == "" || strings.Contains(dbURL, "localhost")) {
		return nil, errors.New("DATABASE_URL tren Production khong duoc de trong hoac tro vao localhost")
	}

	cfg := &Config{
		AppEnv: appEnv,
		Server: ServerConfig{
			HTTPPort:   httpPort,
			GRPCPort:   grpcPort,
			SocketPath: getEnv("LISTEN_SOCKET_PATH", ""),
			UseZiti:    strings.EqualFold(getEnv("USE_ZITI", "false"), "true"),
		},
		Database: DatabaseConfig{
			URL: dbURL,
		},
		Redis: RedisConfig{
			URL: getEnv("REDIS_URL", "localhost:6379"),
		},
		Engine: EngineConfig{
			StorageMode: strings.ToLower(getEnv("STORAGE_MODE", "cloud")),
			BadgerDir:   getEnv("BADGER_DATA_DIR", "./badger-data"),
			DisableGC:   strings.EqualFold(getEnv("DISABLE_GC", "false"), "true"),
			GCInterval:  gcInterval,
			GCIdle:      gcIdle,
		},
		Audit: AuditConfig{
			SocketPath: getEnv("AUDIT_SOCKET_PATH", "/var/run/vector/audit.sock"),
		},
		Security: SecurityConfig{
			JWTSecret: getEnv("JWT_SECRET", "standalone-policy-engine-super-secret-key-32b"),
			LogKEK:    getEnv("LOG_KEK", "01234567890123456789012345678901"),
		},
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
