package store

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/config"
)

func newTestConfig(addr string) *config.Config {
	return &config.Config{
		RedisHost: splitHost(addr),
		RedisPort: splitPort(addr),
		RedisPass: "",
	}
}

func splitHost(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[:i]
		}
	}
	return addr
}

func splitPort(addr string) string {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return addr[i+1:]
		}
	}
	return ""
}

func TestNewValkeyClient(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	if vc.Client() == nil {
		t.Fatal("Client() returned nil")
	}
}

func TestNewValkeyClientConnectionError(t *testing.T) {
	cfg := newTestConfig("127.0.0.1:59999")
	_, err := NewValkeyClient(cfg)
	if err == nil {
		t.Fatal("expected error for invalid address, got nil")
	}
}

func TestGetClientSecret(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.HSet("client:192.168.1.1", "secret", "testing123")

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	cs := NewClientStore(vc)
	ctx := context.Background()

	secret, err := cs.GetClientSecret(ctx, "192.168.1.1")
	if err != nil {
		t.Fatalf("GetClientSecret failed: %v", err)
	}
	if secret != "testing123" {
		t.Errorf("GetClientSecret = %q, want %q", secret, "testing123")
	}
}

func TestGetClientSecretNotFound(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	cs := NewClientStore(vc)
	ctx := context.Background()

	secret, err := cs.GetClientSecret(ctx, "10.0.0.99")
	if err != nil {
		t.Fatalf("GetClientSecret returned error for missing key: %v", err)
	}
	if secret != "" {
		t.Errorf("GetClientSecret = %q, want empty string", secret)
	}
}

func TestGetClientSecretValkeyError(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}

	mr.Close()

	cs := NewClientStore(vc)
	ctx := context.Background()

	_, err = cs.GetClientSecret(ctx, "192.168.1.1")
	if err == nil {
		t.Fatal("expected error when Valkey is down, got nil")
	}
	if !errors.Is(err, ErrValkeyUnavailable) {
		t.Errorf("expected ErrValkeyUnavailable, got: %v", err)
	}
}
