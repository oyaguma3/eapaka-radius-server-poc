package session

import (
	"context"
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

func setupManager(t *testing.T) (*miniredis.Miniredis, SessionManager) {
	t.Helper()
	mr := miniredis.RunT(t)
	cfg := newTestConfig(mr.Addr())
	vc, err := store.NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	t.Cleanup(func() { vc.Close() })
	ss := store.NewSessionStore(vc)
	return mr, NewManager(ss)
}

func TestManagerExists(t *testing.T) {
	mr, mgr := setupManager(t)
	mr.HSet("sess:test-uuid", "imsi", "001010123456789")
	ctx := context.Background()

	exists, err := mgr.Exists(ctx, "test-uuid")
	if err != nil {
		t.Fatalf("Exists failed: %v", err)
	}
	if !exists {
		t.Error("Exists = false, want true")
	}
}

func TestManagerGet(t *testing.T) {
	mr, mgr := setupManager(t)
	mr.HSet("sess:test-uuid", "imsi", "001010123456789")
	mr.HSet("sess:test-uuid", "nas_ip", "192.168.1.1")
	mr.HSet("sess:test-uuid", "start_time", "1706000000")
	ctx := context.Background()

	sess, err := mgr.Get(ctx, "test-uuid")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if sess.IMSI != "001010123456789" {
		t.Errorf("IMSI = %q, want %q", sess.IMSI, "001010123456789")
	}
	if sess.NasIP != "192.168.1.1" {
		t.Errorf("NasIP = %q, want %q", sess.NasIP, "192.168.1.1")
	}
	if sess.StartTime != 1706000000 {
		t.Errorf("StartTime = %d, want %d", sess.StartTime, 1706000000)
	}
}

func TestManagerGetNotFound(t *testing.T) {
	_, mgr := setupManager(t)
	ctx := context.Background()

	_, err := mgr.Get(ctx, "nonexistent")
	if err != ErrSessionNotFound {
		t.Errorf("expected ErrSessionNotFound, got: %v", err)
	}
}

func TestManagerUpdateOnStart(t *testing.T) {
	mr, mgr := setupManager(t)
	mr.HSet("sess:test-uuid", "imsi", "001010123456789")
	ctx := context.Background()

	err := mgr.UpdateOnStart(ctx, "test-uuid", &SessionStartData{
		StartTime: 1706000000,
		NasIP:     "192.168.1.1",
		AcctID:    "acct-123",
		ClientIP:  "10.0.0.1",
	})
	if err != nil {
		t.Fatalf("UpdateOnStart failed: %v", err)
	}

	sess, err := mgr.Get(ctx, "test-uuid")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if sess.AcctID != "acct-123" {
		t.Errorf("AcctID = %q, want %q", sess.AcctID, "acct-123")
	}
}

func TestManagerUpdateOnInterim(t *testing.T) {
	mr, mgr := setupManager(t)
	mr.HSet("sess:test-uuid", "imsi", "001010123456789")
	ctx := context.Background()

	err := mgr.UpdateOnInterim(ctx, "test-uuid", &SessionInterimData{
		NasIP:        "192.168.1.2",
		InputOctets:  1000,
		OutputOctets: 2000,
	})
	if err != nil {
		t.Fatalf("UpdateOnInterim failed: %v", err)
	}

	sess, err := mgr.Get(ctx, "test-uuid")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if sess.InputOctets != 1000 {
		t.Errorf("InputOctets = %d, want 1000", sess.InputOctets)
	}
}

func TestManagerDelete(t *testing.T) {
	mr, mgr := setupManager(t)
	mr.HSet("sess:test-uuid", "imsi", "001010123456789")
	ctx := context.Background()

	err := mgr.Delete(ctx, "test-uuid")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	exists, _ := mgr.Exists(ctx, "test-uuid")
	if exists {
		t.Error("session should be deleted")
	}
}

func TestManagerRemoveUserIndex(t *testing.T) {
	mr, mgr := setupManager(t)
	mr.SAdd("idx:user:001010123456789", "uuid-1", "uuid-2")
	ctx := context.Background()

	err := mgr.RemoveUserIndex(ctx, "001010123456789", "uuid-1")
	if err != nil {
		t.Fatalf("RemoveUserIndex failed: %v", err)
	}

	members, _ := mr.Members("idx:user:001010123456789")
	if len(members) != 1 {
		t.Errorf("members count = %d, want 1", len(members))
	}
}
