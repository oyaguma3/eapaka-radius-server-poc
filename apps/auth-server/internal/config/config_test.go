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
	t.Setenv("VECTOR_API_URL", "http://vector-gateway:8080/api/v1/vector")
}

func TestLoad(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("LISTEN_ADDR", ":1813")
	t.Setenv("RADIUS_SECRET", "testing123")
	t.Setenv("EAP_AKA_PRIME_NETWORK_NAME", "TestNetwork")
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
	if cfg.VectorAPIURL != "http://vector-gateway:8080/api/v1/vector" {
		t.Errorf("VectorAPIURL = %q, want %q", cfg.VectorAPIURL, "http://vector-gateway:8080/api/v1/vector")
	}
	if cfg.ListenAddr != ":1813" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":1813")
	}
	if cfg.RadiusSecret != "testing123" {
		t.Errorf("RadiusSecret = %q, want %q", cfg.RadiusSecret, "testing123")
	}
	if cfg.NetworkName != "TestNetwork" {
		t.Errorf("NetworkName = %q, want %q", cfg.NetworkName, "TestNetwork")
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

	if cfg.ListenAddr != ":1812" {
		t.Errorf("ListenAddr default = %q, want %q", cfg.ListenAddr, ":1812")
	}
	if cfg.NetworkName != "WLAN" {
		t.Errorf("NetworkName default = %q, want %q", cfg.NetworkName, "WLAN")
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
		{name: "missing VECTOR_API_URL", skipEnv: "VECTOR_API_URL"},
	}

	required := map[string]string{
		"REDIS_HOST":     "localhost",
		"REDIS_PORT":     "6379",
		"REDIS_PASS":     "secret",
		"VECTOR_API_URL": "http://vector-gateway:8080/api/v1/vector",
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 必須環境変数をすべてクリアしてからテストする
			for key := range required {
				os.Unsetenv(key)
			}
			// skipEnv以外の必須変数を設定
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

func TestValidateNetworkName(t *testing.T) {
	tests := []struct {
		name        string
		networkName string
		wantErr     bool
	}{
		{name: "valid", networkName: "WLAN", wantErr: false},
		{name: "empty", networkName: "", wantErr: true},
		{name: "whitespace only", networkName: "   ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				NetworkName:  tt.networkName,
				VectorAPIURL: "http://localhost:8080/api/v1/vector",
			}
			err := cfg.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateVectorAPIURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{name: "http", url: "http://localhost:8080/api/v1/vector", wantErr: false},
		{name: "https", url: "https://vector.example.com/api/v1/vector", wantErr: false},
		{name: "no scheme", url: "localhost:8080/api/v1/vector", wantErr: true},
		{name: "ftp scheme", url: "ftp://localhost/vector", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				NetworkName:  "WLAN",
				VectorAPIURL: tt.url,
			}
			err := cfg.validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConstants(t *testing.T) {
	// 定数値が設計書に準拠していることを確認
	if ValkeyConnectTimeout != 3*time.Second {
		t.Errorf("ValkeyConnectTimeout = %v, want %v", ValkeyConnectTimeout, 3*time.Second)
	}
	if ValkeyCommandTimeout != 2*time.Second {
		t.Errorf("ValkeyCommandTimeout = %v, want %v", ValkeyCommandTimeout, 2*time.Second)
	}
	if ValkeyPoolSize != 10 {
		t.Errorf("ValkeyPoolSize = %d, want %d", ValkeyPoolSize, 10)
	}
	if VectorConnectTimeout != 2*time.Second {
		t.Errorf("VectorConnectTimeout = %v, want %v", VectorConnectTimeout, 2*time.Second)
	}
	if VectorRequestTimeout != 5*time.Second {
		t.Errorf("VectorRequestTimeout = %v, want %v", VectorRequestTimeout, 5*time.Second)
	}
	if CBName != "vector-gateway" {
		t.Errorf("CBName = %q, want %q", CBName, "vector-gateway")
	}
	if CBMaxRequests != 3 {
		t.Errorf("CBMaxRequests = %d, want %d", CBMaxRequests, 3)
	}
	if CBInterval != 10*time.Second {
		t.Errorf("CBInterval = %v, want %v", CBInterval, 10*time.Second)
	}
	if CBTimeout != 30*time.Second {
		t.Errorf("CBTimeout = %v, want %v", CBTimeout, 30*time.Second)
	}
	if CBFailureThreshold != 5 {
		t.Errorf("CBFailureThreshold = %d, want %d", CBFailureThreshold, 5)
	}
	if EAPContextTTL != 60*time.Second {
		t.Errorf("EAPContextTTL = %v, want %v", EAPContextTTL, 60*time.Second)
	}
	if SessionTTL != 24*time.Hour {
		t.Errorf("SessionTTL = %v, want %v", SessionTTL, 24*time.Hour)
	}
	if MaxResyncCount != 32 {
		t.Errorf("MaxResyncCount = %d, want %d", MaxResyncCount, 32)
	}
}
