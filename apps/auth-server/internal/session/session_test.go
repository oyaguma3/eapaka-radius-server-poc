package session

import (
	"context"
	"errors"
	"regexp"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/auth-server/internal/store"
)

func TestSessionStoreCreate(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	ss := NewSessionStore(vc)
	ctx := context.Background()

	sess := &Session{
		IMSI:         "440101234567890",
		NasIP:        "192.168.1.1",
		StartTime:    1700000000,
		ClientIP:     "10.0.0.100",
		AcctID:       "acct-001",
		InputOctets:  0,
		OutputOctets: 0,
	}

	if err := ss.Create(ctx, "sess-001", sess); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	val := mr.HGet("sess:sess-001", "imsi")
	if val != "440101234567890" {
		t.Errorf("imsi: got %v, want 440101234567890", val)
	}
}

func TestSessionStoreCreateTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	ss := NewSessionStore(vc)
	ctx := context.Background()

	sess := &Session{IMSI: "440101234567890", NasIP: "192.168.1.1"}
	if err := ss.Create(ctx, "sess-ttl", sess); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	ttl := mr.TTL("sess:sess-ttl")
	if ttl != config.SessionTTL {
		t.Errorf("TTL: got %v, want %v", ttl, config.SessionTTL)
	}
}

func TestSessionStoreGet(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	ss := NewSessionStore(vc)
	ctx := context.Background()

	mr.HSet("sess:sess-get", "imsi", "440101234567890")
	mr.HSet("sess:sess-get", "nas_ip", "192.168.1.1")
	mr.HSet("sess:sess-get", "start_time", "1700000000")
	mr.HSet("sess:sess-get", "client_ip", "10.0.0.100")
	mr.HSet("sess:sess-get", "acct_id", "acct-001")
	mr.HSet("sess:sess-get", "input_octets", "1024")
	mr.HSet("sess:sess-get", "output_octets", "2048")

	got, err := ss.Get(ctx, "sess-get")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if got.IMSI != "440101234567890" {
		t.Errorf("IMSI: got %v, want 440101234567890", got.IMSI)
	}
	if got.NasIP != "192.168.1.1" {
		t.Errorf("NasIP: got %v, want 192.168.1.1", got.NasIP)
	}
	if got.StartTime != 1700000000 {
		t.Errorf("StartTime: got %v, want 1700000000", got.StartTime)
	}
	if got.ClientIP != "10.0.0.100" {
		t.Errorf("ClientIP: got %v, want 10.0.0.100", got.ClientIP)
	}
	if got.AcctID != "acct-001" {
		t.Errorf("AcctID: got %v, want acct-001", got.AcctID)
	}
	if got.InputOctets != 1024 {
		t.Errorf("InputOctets: got %v, want 1024", got.InputOctets)
	}
	if got.OutputOctets != 2048 {
		t.Errorf("OutputOctets: got %v, want 2048", got.OutputOctets)
	}
}

func TestSessionStoreGetNotFound(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	ss := NewSessionStore(vc)
	ctx := context.Background()

	_, err := ss.Get(ctx, "nonexistent")
	if !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestSessionStoreAddUserIndex(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	ss := NewSessionStore(vc)
	ctx := context.Background()

	if err := ss.AddUserIndex(ctx, "440101234567890", "sess-001"); err != nil {
		t.Fatalf("AddUserIndex failed: %v", err)
	}

	// Set型であることを確認
	members, err := mr.Members("idx:user:440101234567890")
	if err != nil {
		t.Fatalf("Members failed: %v", err)
	}
	if len(members) != 1 || members[0] != "sess-001" {
		t.Errorf("members: got %v, want [sess-001]", members)
	}

	// 2つ目のセッション追加
	if err := ss.AddUserIndex(ctx, "440101234567890", "sess-002"); err != nil {
		t.Fatalf("AddUserIndex(2nd) failed: %v", err)
	}
	members, err = mr.Members("idx:user:440101234567890")
	if err != nil {
		t.Fatalf("Members failed: %v", err)
	}
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
}

func TestSessionStoreAddUserIndexDuplicate(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	ss := NewSessionStore(vc)
	ctx := context.Background()

	// 同じセッションIDを2回追加してもエラーにならない
	if err := ss.AddUserIndex(ctx, "440101234567890", "sess-dup"); err != nil {
		t.Fatalf("AddUserIndex failed: %v", err)
	}
	if err := ss.AddUserIndex(ctx, "440101234567890", "sess-dup"); err != nil {
		t.Fatalf("AddUserIndex(duplicate) failed: %v", err)
	}

	members, err := mr.Members("idx:user:440101234567890")
	if err != nil {
		t.Fatalf("Members failed: %v", err)
	}
	// Setなので重複なし
	if len(members) != 1 {
		t.Errorf("expected 1 member (no duplicates), got %d", len(members))
	}
}

func TestGenerateSessionID(t *testing.T) {
	id := GenerateSessionID()
	uuidRegex := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)
	if !uuidRegex.MatchString(id) {
		t.Errorf("GenerateSessionID() = %q, not UUID format", id)
	}
}

func TestGenerateSessionIDUniqueness(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 100; i++ {
		id := GenerateSessionID()
		if seen[id] {
			t.Fatalf("duplicate session ID generated: %s", id)
		}
		seen[id] = true
	}
}

func TestSessionStoreValkeyError(t *testing.T) {
	mr := miniredis.RunT(t)
	vc := newTestValkeyClient(t, mr)
	ss := NewSessionStore(vc)
	ctx := context.Background()

	// Valkey停止
	mr.Close()

	_, err := ss.Get(ctx, "any-key")
	if err == nil {
		t.Fatal("expected error when Valkey is down")
	}
	if !errors.Is(err, store.ErrValkeyUnavailable) {
		t.Errorf("expected ErrValkeyUnavailable, got: %v", err)
	}
}
