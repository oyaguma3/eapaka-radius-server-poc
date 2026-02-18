package valkey

import (
	"testing"
	"time"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.Addr != "localhost:6379" {
		t.Errorf("Addr = %q, want %q", opts.Addr, "localhost:6379")
	}
	if opts.Password != "" {
		t.Errorf("Password = %q, want empty", opts.Password)
	}
	if opts.DB != 0 {
		t.Errorf("DB = %d, want %d", opts.DB, 0)
	}
	if opts.ConnectTimeout != 3*time.Second {
		t.Errorf("ConnectTimeout = %v, want %v", opts.ConnectTimeout, 3*time.Second)
	}
	if opts.ReadTimeout != 2*time.Second {
		t.Errorf("ReadTimeout = %v, want %v", opts.ReadTimeout, 2*time.Second)
	}
	if opts.WriteTimeout != 2*time.Second {
		t.Errorf("WriteTimeout = %v, want %v", opts.WriteTimeout, 2*time.Second)
	}
	if opts.PoolSize != 10 {
		t.Errorf("PoolSize = %d, want %d", opts.PoolSize, 10)
	}
	if opts.MinIdleConns != 2 {
		t.Errorf("MinIdleConns = %d, want %d", opts.MinIdleConns, 2)
	}
}

func TestTUIOptions(t *testing.T) {
	opts := TUIOptions()

	if opts.ConnectTimeout != 5*time.Second {
		t.Errorf("ConnectTimeout = %v, want %v", opts.ConnectTimeout, 5*time.Second)
	}
	if opts.ReadTimeout != 5*time.Second {
		t.Errorf("ReadTimeout = %v, want %v", opts.ReadTimeout, 5*time.Second)
	}
	if opts.WriteTimeout != 5*time.Second {
		t.Errorf("WriteTimeout = %v, want %v", opts.WriteTimeout, 5*time.Second)
	}
	if opts.PoolSize != 5 {
		t.Errorf("PoolSize = %d, want %d", opts.PoolSize, 5)
	}
	if opts.MinIdleConns != 1 {
		t.Errorf("MinIdleConns = %d, want %d", opts.MinIdleConns, 1)
	}
}

func TestOptionsBuilder(t *testing.T) {
	opts := DefaultOptions().
		WithAddr("192.168.1.100:6380").
		WithPassword("secret").
		WithDB(1).
		WithTimeouts(5*time.Second, 3*time.Second, 3*time.Second).
		WithPool(20, 5)

	if opts.Addr != "192.168.1.100:6380" {
		t.Errorf("Addr = %q, want %q", opts.Addr, "192.168.1.100:6380")
	}
	if opts.Password != "secret" {
		t.Errorf("Password = %q, want %q", opts.Password, "secret")
	}
	if opts.DB != 1 {
		t.Errorf("DB = %d, want %d", opts.DB, 1)
	}
	if opts.ConnectTimeout != 5*time.Second {
		t.Errorf("ConnectTimeout = %v, want %v", opts.ConnectTimeout, 5*time.Second)
	}
	if opts.ReadTimeout != 3*time.Second {
		t.Errorf("ReadTimeout = %v, want %v", opts.ReadTimeout, 3*time.Second)
	}
	if opts.WriteTimeout != 3*time.Second {
		t.Errorf("WriteTimeout = %v, want %v", opts.WriteTimeout, 3*time.Second)
	}
	if opts.PoolSize != 20 {
		t.Errorf("PoolSize = %d, want %d", opts.PoolSize, 20)
	}
	if opts.MinIdleConns != 5 {
		t.Errorf("MinIdleConns = %d, want %d", opts.MinIdleConns, 5)
	}
}

func TestBuildAddr(t *testing.T) {
	tests := []struct {
		host string
		port int
		want string
	}{
		{"localhost", 6379, "localhost:6379"},
		{"192.168.1.100", 6380, "192.168.1.100:6380"},
		{"valkey.example.com", 16379, "valkey.example.com:16379"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := BuildAddr(tt.host, tt.port)
			if got != tt.want {
				t.Errorf("BuildAddr(%q, %d) = %q, want %q", tt.host, tt.port, got, tt.want)
			}
		})
	}
}
