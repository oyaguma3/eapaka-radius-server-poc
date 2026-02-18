package server

import (
	"context"
	"net"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/store"
)

func newTestConfig(addr string) *config.Config {
	for i := len(addr) - 1; i >= 0; i-- {
		if addr[i] == ':' {
			return &config.Config{
				RedisHost: addr[:i],
				RedisPort: addr[i+1:],
				RedisPass: "",
			}
		}
	}
	return &config.Config{RedisHost: addr, RedisPort: "6379", RedisPass: ""}
}

func TestSecretSource_FromValkey(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.HSet("client:192.168.1.1", "secret", "valkeySecret")

	cfg := newTestConfig(mr.Addr())
	vc, err := store.NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	cs := store.NewClientStore(vc)
	ss := NewSecretSource(cs, "fallback")

	ctx := context.Background()
	addr := &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 12345}

	secret, err := ss.RADIUSSecret(ctx, addr)
	if err != nil {
		t.Fatalf("RADIUSSecret failed: %v", err)
	}
	if string(secret) != "valkeySecret" {
		t.Errorf("RADIUSSecret = %q, want %q", secret, "valkeySecret")
	}
}

func TestSecretSource_Fallback(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := newTestConfig(mr.Addr())
	vc, err := store.NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	cs := store.NewClientStore(vc)
	ss := NewSecretSource(cs, "fallbackSecret")

	ctx := context.Background()
	addr := &net.UDPAddr{IP: net.ParseIP("10.0.0.99"), Port: 12345}

	secret, err := ss.RADIUSSecret(ctx, addr)
	if err != nil {
		t.Fatalf("RADIUSSecret failed: %v", err)
	}
	if string(secret) != "fallbackSecret" {
		t.Errorf("RADIUSSecret = %q, want %q", secret, "fallbackSecret")
	}
}

func TestSecretSource_NoSecret(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := newTestConfig(mr.Addr())
	vc, err := store.NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	cs := store.NewClientStore(vc)
	ss := NewSecretSource(cs, "")

	ctx := context.Background()
	addr := &net.UDPAddr{IP: net.ParseIP("10.0.0.99"), Port: 12345}

	secret, err := ss.RADIUSSecret(ctx, addr)
	if err != nil {
		t.Fatalf("RADIUSSecret failed: %v", err)
	}
	if secret != nil {
		t.Errorf("RADIUSSecret = %q, want nil", secret)
	}
}

func TestSecretSource_NilAddr(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := newTestConfig(mr.Addr())
	vc, err := store.NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	cs := store.NewClientStore(vc)
	ss := NewSecretSource(cs, "fallback")

	ctx := context.Background()
	secret, err := ss.RADIUSSecret(ctx, nil)
	if err != nil {
		t.Fatalf("RADIUSSecret failed: %v", err)
	}
	if string(secret) != "fallback" {
		t.Errorf("RADIUSSecret = %q, want %q", secret, "fallback")
	}
}

func TestExtractIP(t *testing.T) {
	tests := []struct {
		name string
		addr net.Addr
		want string
	}{
		{
			name: "UDPAddr",
			addr: &net.UDPAddr{IP: net.ParseIP("192.168.1.1"), Port: 1234},
			want: "192.168.1.1",
		},
		{
			name: "nil",
			addr: nil,
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractIP(tt.addr)
			if got != tt.want {
				t.Errorf("extractIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
