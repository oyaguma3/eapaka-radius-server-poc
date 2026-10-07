package config

import (
	"os"
	"strings"
	"testing"
)

var testFingerprint = strings.Repeat("ab", 32)

// setRequired は必須の環境変数を設定する。
func setRequired(t *testing.T) {
	t.Helper()
	t.Setenv("PROVISIONING_API_TLS_CERT", "/certs/server.pem")
	t.Setenv("PROVISIONING_API_TLS_KEY", "/certs/server.key")
	t.Setenv("PROVISIONING_API_ADMIN_CLIENTS", "bff-01="+testFingerprint)
}

func TestLoad_Defaults(t *testing.T) {
	setRequired(t)
	for _, k := range []string{"PROVISIONING_API_LISTEN_ADDR", "PROVISIONING_API_NODE_NAME", "REDIS_HOST", "REDIS_PORT", "REDIS_PASS", "LOG_LEVEL", "LOG_MASK_IMSI", "GIN_MODE"} {
		t.Setenv(k, "")
		_ = os.Unsetenv(k)
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ListenAddr != ":9444" || cfg.RedisAddr() != "valkey:6379" || cfg.LogLevel != "INFO" || !cfg.LogMaskIMSI || cfg.GinMode != "release" {
		t.Errorf("defaults = %+v", cfg)
	}
	if cfg.AdminClients[testFingerprint] != "bff-01" {
		t.Errorf("AdminClients = %v", cfg.AdminClients)
	}
	if host, _ := os.Hostname(); cfg.NodeName != host {
		t.Errorf("NodeName = %q, want hostname %q", cfg.NodeName, host)
	}
}

func TestLoad_Values(t *testing.T) {
	setRequired(t)
	t.Setenv("PROVISIONING_API_LISTEN_ADDR", "127.0.0.1:19444")
	t.Setenv("PROVISIONING_API_NODE_NAME", "minipc-01")
	t.Setenv("REDIS_HOST", "localhost")
	t.Setenv("REDIS_PORT", "16379")
	t.Setenv("LOG_MASK_IMSI", "false")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.ListenAddr != "127.0.0.1:19444" || cfg.NodeName != "minipc-01" || cfg.RedisAddr() != "localhost:16379" || cfg.LogMaskIMSI {
		t.Errorf("cfg = %+v", cfg)
	}
}

func TestLoad_Errors(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
	}{
		{"empty admin clients", map[string]string{"PROVISIONING_API_ADMIN_CLIENTS": " , "}},
		{"bad admin clients", map[string]string{"PROVISIONING_API_ADMIN_CLIENTS": "bff-01=xyz"}},
		{"empty cert", map[string]string{"PROVISIONING_API_TLS_CERT": ""}},
		{"bad bool", map[string]string{"LOG_MASK_IMSI": "maybe"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setRequired(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			if _, err := Load(); err == nil {
				t.Error("Load() expected error")
			}
		})
	}

	t.Run("missing admin clients", func(t *testing.T) {
		setRequired(t)
		_ = os.Unsetenv("PROVISIONING_API_ADMIN_CLIENTS")
		if _, err := Load(); err == nil {
			t.Error("Load() expected error")
		}
	})
}
