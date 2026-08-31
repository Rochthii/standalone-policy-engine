package config

import (
	"os"
	"testing"
)

func TestConfig_LoadDefaults(t *testing.T) {
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load config that bai: %v", err)
	}

	if cfg.Server.HTTPPort != 8080 {
		t.Errorf("Kỳ vọng HTTPPort = 8080, thực tế: %d", cfg.Server.HTTPPort)
	}
	if cfg.Server.GRPCPort != 50051 {
		t.Errorf("Kỳ vọng GRPCPort = 50051, thực tế: %d", cfg.Server.GRPCPort)
	}
	if cfg.Engine.StorageMode != "cloud" {
		t.Errorf("Kỳ vọng StorageMode = cloud, thực tế: %s", cfg.Engine.StorageMode)
	}
}

func TestConfig_ProductionValidation(t *testing.T) {
	os.Setenv("APP_ENV", "production")
	os.Setenv("DATABASE_URL", "postgres://postgres:postgres@localhost:5432/db")
	defer func() {
		os.Unsetenv("APP_ENV")
		os.Unsetenv("DATABASE_URL")
	}()

	_, err := Load()
	if err == nil {
		t.Fatal("Kỳ vọng báo lỗi khi chạy production với localhost DB URL")
	}
}
