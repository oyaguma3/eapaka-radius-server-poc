package config

import (
	"os"
	"testing"
)

func TestLoad(t *testing.T) {
	t.Run("reads VALKEY_PASSWORD from environment", func(t *testing.T) {
		os.Setenv("VALKEY_PASSWORD", "test_password")
		defer os.Unsetenv("VALKEY_PASSWORD")

		cfg := Load()
		if cfg.ValkeyPassword != "test_password" {
			t.Errorf("expected ValkeyPassword to be 'test_password', got '%s'", cfg.ValkeyPassword)
		}
	})

	t.Run("uses fixed ValkeyAddr", func(t *testing.T) {
		cfg := Load()
		if cfg.ValkeyAddr != "127.0.0.1:6379" {
			t.Errorf("expected ValkeyAddr to be '127.0.0.1:6379', got '%s'", cfg.ValkeyAddr)
		}
	})

	t.Run("returns empty password when not set", func(t *testing.T) {
		os.Unsetenv("VALKEY_PASSWORD")
		cfg := Load()
		if cfg.ValkeyPassword != "" {
			t.Errorf("expected ValkeyPassword to be empty, got '%s'", cfg.ValkeyPassword)
		}
	})
}
