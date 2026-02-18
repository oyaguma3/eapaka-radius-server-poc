package store

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestDuplicateStoreGetEmpty(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ds := NewDuplicateStore(vc)
	ctx := context.Background()

	val, err := ds.Get(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "" {
		t.Errorf("Get = %q, want empty string", val)
	}
}

func TestDuplicateStoreSetAndGet(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ds := NewDuplicateStore(vc)
	ctx := context.Background()

	err = ds.Set(ctx, "acct-sess-1", "start")
	if err != nil {
		t.Fatalf("Set failed: %v", err)
	}

	val, err := ds.Get(ctx, "acct-sess-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "start" {
		t.Errorf("Get = %q, want %q", val, "start")
	}
}

func TestDuplicateStoreOverwrite(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ds := NewDuplicateStore(vc)
	ctx := context.Background()

	_ = ds.Set(ctx, "acct-sess-1", "start")
	_ = ds.Set(ctx, "acct-sess-1", "interim:100:200")

	val, err := ds.Get(ctx, "acct-sess-1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if val != "interim:100:200" {
		t.Errorf("Get = %q, want %q", val, "interim:100:200")
	}
}

func TestDuplicateStoreTTL(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ds := NewDuplicateStore(vc)
	ctx := context.Background()

	_ = ds.Set(ctx, "acct-sess-1", "start")

	// TTLが設定されていることを確認
	ttl := mr.TTL("acct:seen:acct-sess-1")
	if ttl <= 0 {
		t.Error("TTL should be positive")
	}
}
