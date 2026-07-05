package pectl_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"standalone-policy-engine/internal/pectl"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear env vars that might interfere
	os.Unsetenv("PECTL_SERVER")
	os.Unsetenv("PECTL_TOKEN")
	os.Unsetenv("PECTL_OUTPUT")

	cfg, err := pectl.LoadConfig("")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Server != "http://localhost:8080" {
		t.Errorf("expected default server http://localhost:8080, got: %s", cfg.Server)
	}
	if cfg.Output != "table" {
		t.Errorf("expected default output 'table', got: %s", cfg.Output)
	}
	if cfg.Timeout != 10*time.Second {
		t.Errorf("expected default timeout 10s, got: %v", cfg.Timeout)
	}
}

func TestLoadConfig_EnvOverride(t *testing.T) {
	os.Setenv("PECTL_SERVER", "http://env-server:9090")
	os.Setenv("PECTL_TOKEN", "env-test-token")
	os.Setenv("PECTL_OUTPUT", "json")
	defer func() {
		os.Unsetenv("PECTL_SERVER")
		os.Unsetenv("PECTL_TOKEN")
		os.Unsetenv("PECTL_OUTPUT")
	}()

	cfg, err := pectl.LoadConfig("")
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Server != "http://env-server:9090" {
		t.Errorf("expected server from env, got: %s", cfg.Server)
	}
	if cfg.Token != "env-test-token" {
		t.Errorf("expected token from env, got: %s", cfg.Token)
	}
	if cfg.Output != "json" {
		t.Errorf("expected output 'json' from env, got: %s", cfg.Output)
	}
}

func TestLoadConfig_FromFile(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `server: http://file-server:7070
auth:
  token: file-token
output: yaml
timeout: 30s
`
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		t.Fatalf("failed to write temp config: %v", err)
	}

	os.Unsetenv("PECTL_SERVER")
	os.Unsetenv("PECTL_TOKEN")
	os.Unsetenv("PECTL_OUTPUT")

	cfg, err := pectl.LoadConfig(configPath)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if cfg.Server != "http://file-server:7070" {
		t.Errorf("expected server from file, got: %s", cfg.Server)
	}
	if cfg.Output != "yaml" {
		t.Errorf("expected output 'yaml' from file, got: %s", cfg.Output)
	}
	if cfg.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s from file, got: %v", cfg.Timeout)
	}
}

func TestLoadConfig_InvalidFile(t *testing.T) {
	_, err := pectl.LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Error("expected error for non-existent config file")
	}
}
