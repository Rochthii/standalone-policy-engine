package pectl

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/viper"
)

// Config represents the configuration for pectl.
type Config struct {
	Server  string        `mapstructure:"server"`
	Token   string        `mapstructure:"token"`
	Output  string        `mapstructure:"output"`
	Timeout time.Duration `mapstructure:"timeout"`
}

// LoadConfig loads the configuration from file, environment variables, and flags.
func LoadConfig(configFile string) (*Config, error) {
	v := viper.New()

	// 1. Setup defaults
	v.SetDefault("server", "http://localhost:8080")
	v.SetDefault("output", "table")
	v.SetDefault("timeout", "10s")

	// 2. Setup env variables
	v.SetEnvPrefix("PECTL")
	v.AutomaticEnv()
	// Bind specific env vars
	_ = v.BindEnv("server", "PECTL_SERVER")
	_ = v.BindEnv("token", "PECTL_TOKEN")
	_ = v.BindEnv("output", "PECTL_OUTPUT")

	// 3. Setup config file path
	if configFile != "" {
		v.SetConfigFile(configFile)
	} else {
		home, err := os.UserHomeDir()
		if err == nil {
			v.AddConfigPath(filepath.Join(home, ".pectl"))
			v.SetConfigName("config")
			v.SetConfigType("yaml")
		}
	}

	// Read config file if exists
	if err := v.ReadInConfig(); err != nil {
		// It is fine if config file is not found, unless explicitly passed
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok && configFile != "" {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	var cfg Config
	// Need to parse timeout string manually or let viper handle it
	timeoutStr := v.GetString("timeout")
	duration, err := time.ParseDuration(timeoutStr)
	if err != nil {
		// fallback to default duration
		duration = 10 * time.Second
	}
	cfg.Timeout = duration

	cfg.Server = v.GetString("server")
	cfg.Token = v.GetString("token")
	cfg.Output = v.GetString("output")

	return &cfg, nil
}
