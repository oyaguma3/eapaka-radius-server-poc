package store

import (
	"context"
	"errors"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

func TestSessionExists(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.HSet("sess:test-uuid", "imsi", "001010123456789")

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ss := NewSessionStore(vc)
	ctx := context.Background()

	exists, err := ss.Exists(ctx, "test-uuid")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Exists = false, want true")
	}

	exists, err = ss.Exists(ctx, "nonexistent")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("Exists = true, want false")
	}
}

func TestSessionGet(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.HSet("sess:test-uuid", "imsi", "001010123456789")
	mr.HSet("sess:test-uuid", "nas_ip", "192.168.1.1")

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ss := NewSessionStore(vc)
	ctx := context.Background()

	m, err := ss.Get(ctx, "test-uuid")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if m["imsi"] != "001010123456789" {
		t.Errorf("imsi = %q, want %q", m["imsi"], "001010123456789")
	}
	if m["nas_ip"] != "192.168.1.1" {
		t.Errorf("nas_ip = %q, want %q", m["nas_ip"], "192.168.1.1")
	}
}

func TestSessionGetNotFound(t *testing.T) {
	mr := miniredis.RunT(t)

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ss := NewSessionStore(vc)
	ctx := context.Background()

	_, err = ss.Get(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error for missing session, got nil")
	}
	if !errors.Is(err, ErrKeyNotFound) {
		t.Errorf("expected ErrKeyNotFound, got: %v", err)
	}
}

func TestSessionUpdateOnStart(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.HSet("sess:test-uuid", "imsi", "001010123456789")

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ss := NewSessionStore(vc)
	ctx := context.Background()

	fields := map[string]any{
		"start_time": int64(1706000000),
		"nas_ip":     "192.168.1.1",
		"acct_id":    "acct-123",
		"client_ip":  "10.0.0.1",
	}
	err = ss.UpdateOnStart(ctx, "test-uuid", fields)
	if err != nil {
		t.Fatalf("UpdateOnStart failed: %v", err)
	}

	// 値を確認
	m, err := ss.Get(ctx, "test-uuid")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if m["nas_ip"] != "192.168.1.1" {
		t.Errorf("nas_ip = %q, want %q", m["nas_ip"], "192.168.1.1")
	}
	if m["acct_id"] != "acct-123" {
		t.Errorf("acct_id = %q, want %q", m["acct_id"], "acct-123")
	}
}

func TestSessionUpdateOnInterim(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.HSet("sess:test-uuid", "imsi", "001010123456789")

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ss := NewSessionStore(vc)
	ctx := context.Background()

	fields := map[string]any{
		"nas_ip":        "192.168.1.2",
		"input_octets":  int64(1000),
		"output_octets": int64(2000),
	}
	err = ss.UpdateOnInterim(ctx, "test-uuid", fields)
	if err != nil {
		t.Fatalf("UpdateOnInterim failed: %v", err)
	}

	m, err := ss.Get(ctx, "test-uuid")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if m["input_octets"] != "1000" {
		t.Errorf("input_octets = %q, want %q", m["input_octets"], "1000")
	}
}

func TestSessionDelete(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.HSet("sess:test-uuid", "imsi", "001010123456789")

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ss := NewSessionStore(vc)
	ctx := context.Background()

	err = ss.Delete(ctx, "test-uuid")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	exists, err := ss.Exists(ctx, "test-uuid")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if exists {
		t.Error("session should be deleted")
	}
}

func TestSessionRemoveUserIndex(t *testing.T) {
	mr := miniredis.RunT(t)
	mr.SAdd("idx:user:001010123456789", "uuid-1", "uuid-2")

	cfg := newTestConfig(mr.Addr())
	vc, err := NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	defer vc.Close()

	ss := NewSessionStore(vc)
	ctx := context.Background()

	err = ss.RemoveUserIndex(ctx, "001010123456789", "uuid-1")
	if err != nil {
		t.Fatalf("RemoveUserIndex failed: %v", err)
	}

	members, err := mr.Members("idx:user:001010123456789")
	if err != nil {
		t.Fatalf("failed to get members: %v", err)
	}
	if len(members) != 1 {
		t.Errorf("members count = %d, want 1", len(members))
	}
}
