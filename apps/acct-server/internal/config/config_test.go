package config

import (
	"os"
	"testing"
	"time"
)

// setRequiredEnv は必須環境変数をすべて設定する
func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("REDIS_HOST", "localhost")
	t.Setenv("REDIS_PORT", "6379")
	t.Setenv("REDIS_PASS", "secret")
}

func TestLoad(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("LISTEN_ADDR", ":1814")
	t.Setenv("RADIUS_SECRET", "testing123")
	t.Setenv("LOG_MASK_IMSI", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.RedisHost != "localhost" {
		t.Errorf("RedisHost = %q, want %q", cfg.RedisHost, "localhost")
	}
	if cfg.RedisPort != "6379" {
		t.Errorf("RedisPort = %q, want %q", cfg.RedisPort, "6379")
	}
	if cfg.RedisPass != "secret" {
		t.Errorf("RedisPass = %q, want %q", cfg.RedisPass, "secret")
	}
	if cfg.ListenAddr != ":1814" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":1814")
	}
	if cfg.RadiusSecret != "testing123" {
		t.Errorf("RadiusSecret = %q, want %q", cfg.RadiusSecret, "testing123")
	}
	if cfg.LogMaskIMSI != false {
		t.Errorf("LogMaskIMSI = %v, want %v", cfg.LogMaskIMSI, false)
	}
}

func TestLoadDefaults(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.ListenAddr != ":1813" {
		t.Errorf("ListenAddr default = %q, want %q", cfg.ListenAddr, ":1813")
	}
	if cfg.LogMaskIMSI != true {
		t.Errorf("LogMaskIMSI default = %v, want %v", cfg.LogMaskIMSI, true)
	}
	if cfg.RadiusSecret != "" {
		t.Errorf("RadiusSecret default = %q, want %q", cfg.RadiusSecret, "")
	}
}

func TestLoadMissingRequired(t *testing.T) {
	tests := []struct {
		name    string
		skipEnv string
	}{
		{name: "missing REDIS_HOST", skipEnv: "REDIS_HOST"},
		{name: "missing REDIS_PORT", skipEnv: "REDIS_PORT"},
		{name: "missing REDIS_PASS", skipEnv: "REDIS_PASS"},
	}

	required := map[string]string{
		"REDIS_HOST": "localhost",
		"REDIS_PORT": "6379",
		"REDIS_PASS": "secret",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for key := range required {
				os.Unsetenv(key)
			}
			for key, val := range required {
				if key != tt.skipEnv {
					t.Setenv(key, val)
				}
			}
			_, err := Load()
			if err == nil {
				t.Errorf("Load() should return error when %s is missing", tt.skipEnv)
			}
		})
	}
}

func TestValkeyAddr(t *testing.T) {
	cfg := &Config{
		RedisHost: "redis.example.com",
		RedisPort: "6380",
	}
	got := cfg.ValkeyAddr()
	want := "redis.example.com:6380"
	if got != want {
		t.Errorf("ValkeyAddr() = %q, want %q", got, want)
	}
}

func TestConstants(t *testing.T) {
	if ValkeyConnectTimeout != 3*time.Second {
		t.Errorf("ValkeyConnectTimeout = %v, want %v", ValkeyConnectTimeout, 3*time.Second)
	}
	if ValkeyCommandTimeout != 2*time.Second {
		t.Errorf("ValkeyCommandTimeout = %v, want %v", ValkeyCommandTimeout, 2*time.Second)
	}
	if ValkeyPoolSize != 10 {
		t.Errorf("ValkeyPoolSize = %d, want %d", ValkeyPoolSize, 10)
	}
	if SessionTTL != 24*time.Hour {
		t.Errorf("SessionTTL = %v, want %v", SessionTTL, 24*time.Hour)
	}
	if DuplicateDetectTTL != 24*time.Hour {
		t.Errorf("DuplicateDetectTTL = %v, want %v", DuplicateDetectTTL, 24*time.Hour)
	}
	if ShutdownTimeout != 5*time.Second {
		t.Errorf("ShutdownTimeout = %v, want %v", ShutdownTimeout, 5*time.Second)
	}
}
