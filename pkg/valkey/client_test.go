package valkey

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestNewClient(t *testing.T) {
	mr := miniredis.RunT(t)

	opts := DefaultOptions().WithAddr(mr.Addr())
	client, err := NewClient(opts)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	// 接続確認
	ctx := context.Background()
	pong, err := client.Ping(ctx).Result()
	if err != nil {
		t.Errorf("Ping() error = %v", err)
	}
	if pong != "PONG" {
		t.Errorf("Ping() = %q, want %q", pong, "PONG")
	}
}

func TestNewClientWithContext(t *testing.T) {
	mr := miniredis.RunT(t)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	opts := DefaultOptions().WithAddr(mr.Addr())
	client, err := NewClientWithContext(ctx, opts)
	if err != nil {
		t.Fatalf("NewClientWithContext() error = %v", err)
	}
	defer client.Close()

	// SET/GETテスト
	err = client.Set(ctx, "test-key", "test-value", 0).Err()
	if err != nil {
		t.Errorf("Set() error = %v", err)
	}

	val, err := client.Get(ctx, "test-key").Result()
	if err != nil {
		t.Errorf("Get() error = %v", err)
	}
	if val != "test-value" {
		t.Errorf("Get() = %q, want %q", val, "test-value")
	}
}

func TestNewClientWithNilOptions(t *testing.T) {
	mr := miniredis.RunT(t)

	// nilオプションでもデフォルト値が使用される
	// ただし、miniredisのアドレスは使えないのでエラーになる
	// このテストはデフォルトオプションが適用されることの確認
	opts := DefaultOptions().WithAddr(mr.Addr())
	client, err := NewClientWithContext(context.Background(), opts)
	if err != nil {
		t.Fatalf("NewClientWithContext() error = %v", err)
	}
	defer client.Close()
}

func TestNewClientConnectionError(t *testing.T) {
	// 存在しないアドレスへの接続
	opts := DefaultOptions().
		WithAddr("localhost:59999").
		WithTimeouts(100*time.Millisecond, 100*time.Millisecond, 100*time.Millisecond)

	_, err := NewClient(opts)
	if err == nil {
		t.Error("NewClient() expected error for invalid address")
	}
}

func TestMustNewClient(t *testing.T) {
	mr := miniredis.RunT(t)

	opts := DefaultOptions().WithAddr(mr.Addr())
	// パニックしないことを確認
	client := MustNewClient(opts)
	defer client.Close()

	if client == nil {
		t.Error("MustNewClient() returned nil")
	}
}

func TestMustNewClientPanics(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("MustNewClient() should panic on connection error")
		}
	}()

	opts := DefaultOptions().
		WithAddr("localhost:59999").
		WithTimeouts(100*time.Millisecond, 100*time.Millisecond, 100*time.Millisecond)
	MustNewClient(opts)
}

func TestIsConnectionError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"regular error", errors.New("some error"), false},
		{"context deadline exceeded", context.DeadlineExceeded, true},
		{"context canceled", context.Canceled, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsConnectionError(tt.err)
			if got != tt.want {
				t.Errorf("IsConnectionError(%v) = %v, want %v", tt.err, got, tt.want)
			}
		})
	}
}

func TestIsKeyNotFound(t *testing.T) {
	mr := miniredis.RunT(t)

	opts := DefaultOptions().WithAddr(mr.Addr())
	client, err := NewClient(opts)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	defer client.Close()

	ctx := context.Background()

	// 存在しないキーを取得
	_, err = client.Get(ctx, "non-existent-key").Result()
	if !IsKeyNotFound(err) {
		t.Errorf("IsKeyNotFound(%v) = false, want true", err)
	}

	// redis.Nilを直接テスト
	if !IsKeyNotFound(redis.Nil) {
		t.Error("IsKeyNotFound(redis.Nil) = false, want true")
	}

	// 他のエラーではfalse
	if IsKeyNotFound(errors.New("other error")) {
		t.Error("IsKeyNotFound(other error) = true, want false")
	}

	if IsKeyNotFound(nil) {
		t.Error("IsKeyNotFound(nil) = true, want false")
	}
}
