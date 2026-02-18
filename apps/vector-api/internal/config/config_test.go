package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	// 必須環境変数を設定
	os.Setenv("REDIS_HOST", "localhost")
	os.Setenv("REDIS_PORT", "6379")
	os.Setenv("REDIS_PASS", "testpass")
	defer func() {
		os.Unsetenv("REDIS_HOST")
		os.Unsetenv("REDIS_PORT")
		os.Unsetenv("REDIS_PASS")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.RedisHost != "localhost" {
		t.Errorf("RedisHost = %q, want %q", cfg.RedisHost, "localhost")
	}
	if cfg.RedisPort != "6379" {
		t.Errorf("RedisPort = %q, want %q", cfg.RedisPort, "6379")
	}
	if cfg.RedisPass != "testpass" {
		t.Errorf("RedisPass = %q, want %q", cfg.RedisPass, "testpass")
	}
}

func TestLoadDefaults(t *testing.T) {
	os.Setenv("REDIS_HOST", "localhost")
	os.Setenv("REDIS_PORT", "6379")
	os.Setenv("REDIS_PASS", "testpass")
	defer func() {
		os.Unsetenv("REDIS_HOST")
		os.Unsetenv("REDIS_PORT")
		os.Unsetenv("REDIS_PASS")
	}()

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// デフォルト値の確認
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":8080")
	}
	if cfg.LogLevel != "INFO" {
		t.Errorf("LogLevel = %q, want %q", cfg.LogLevel, "INFO")
	}
	if !cfg.LogMaskIMSI {
		t.Error("LogMaskIMSI = false, want true")
	}
	if cfg.GinMode != "release" {
		t.Errorf("GinMode = %q, want %q", cfg.GinMode, "release")
	}
	if cfg.TestVectorEnabled {
		t.Error("TestVectorEnabled = true, want false")
	}
	if cfg.TestVectorIMSIPrefix != "00101" {
		t.Errorf("TestVectorIMSIPrefix = %q, want %q", cfg.TestVectorIMSIPrefix, "00101")
	}
}

func TestLoadMissingRequired(t *testing.T) {
	// 必須環境変数をクリア
	os.Unsetenv("REDIS_HOST")
	os.Unsetenv("REDIS_PORT")
	os.Unsetenv("REDIS_PASS")

	_, err := Load()
	if err == nil {
		t.Error("Load() expected error for missing required env vars")
	}
}

func TestRedisAddr(t *testing.T) {
	cfg := &Config{
		RedisHost: "192.168.1.100",
		RedisPort: "6380",
	}

	want := "192.168.1.100:6380"
	got := cfg.RedisAddr()
	if got != want {
		t.Errorf("RedisAddr() = %q, want %q", got, want)
	}
}
