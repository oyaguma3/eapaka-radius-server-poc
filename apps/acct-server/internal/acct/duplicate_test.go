package acct

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

func setupDuplicateDetector(t *testing.T) (*miniredis.Miniredis, DuplicateDetector) {
	t.Helper()
	mr := miniredis.RunT(t)
	cfg := newTestConfig(mr.Addr())
	vc, err := store.NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	t.Cleanup(func() { vc.Close() })
	ds := store.NewDuplicateStore(vc)
	return mr, NewDuplicateDetector(ds)
}

func TestCheckAndMarkStart_New(t *testing.T) {
	_, dd := setupDuplicateDetector(t)
	ctx := context.Background()

	isDup, err := dd.CheckAndMarkStart(ctx, "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isDup {
		t.Error("should not be duplicate for new session")
	}
}

func TestCheckAndMarkStart_Duplicate(t *testing.T) {
	_, dd := setupDuplicateDetector(t)
	ctx := context.Background()

	// 1回目
	_, _ = dd.CheckAndMarkStart(ctx, "sess-1")

	// 2回目（重複）
	isDup, err := dd.CheckAndMarkStart(ctx, "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isDup {
		t.Error("should be duplicate for repeated start")
	}
}

func TestCheckAndMarkStart_AfterStop(t *testing.T) {
	_, dd := setupDuplicateDetector(t)
	ctx := context.Background()

	_ = dd.MarkAsStopped(ctx, "sess-1")

	isDup, err := dd.CheckAndMarkStart(ctx, "sess-1")
	if isDup {
		t.Error("should not be duplicate after stop")
	}
	// SequenceErrorが返ること
	if err == nil {
		t.Fatal("expected SequenceError")
	}
	seqErr, ok := err.(*SequenceError)
	if !ok {
		t.Fatalf("expected *SequenceError, got: %T", err)
	}
	if seqErr.Reason != "start_after_stop" {
		t.Errorf("Reason = %q, want %q", seqErr.Reason, "start_after_stop")
	}
}

func TestCheckInterim(t *testing.T) {
	tests := []struct {
		name       string
		prev       string // 直前の記録（空なら未登録）
		input      uint32
		output     uint32
		wantDup    bool
		wantReason string
		wantStored string
	}{
		{"no start received", "", 100, 200, false, "no_start_received", "interim:100:200"},
		{"after start", "start", 100, 200, false, "", "interim:100:200"},
		{"same values as previous interim", "interim:100:200", 100, 200, true, "", "interim:100:200"},
		{"different values from previous interim", "interim:100:200", 200, 400, false, "", "interim:200:400"},
		{"after stop", "stop", 100, 200, false, "interim_after_stop", "interim:100:200"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr, dd := setupDuplicateDetector(t)
			ctx := context.Background()
			if tt.prev != "" {
				mr.Set("acct:seen:sess-1", tt.prev)
			}

			got, err := dd.CheckInterim(ctx, "sess-1", tt.input, tt.output)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Duplicate != tt.wantDup {
				t.Errorf("Duplicate = %v, want %v", got.Duplicate, tt.wantDup)
			}
			if got.SequenceReason != tt.wantReason {
				t.Errorf("SequenceReason = %q, want %q", got.SequenceReason, tt.wantReason)
			}
			if stored, _ := mr.Get("acct:seen:sess-1"); stored != tt.wantStored {
				t.Errorf("stored = %q, want %q", stored, tt.wantStored)
			}
		})
	}
}

func TestCheckInterim_ValkeyError(t *testing.T) {
	mr, dd := setupDuplicateDetector(t)
	mr.SetError("connection refused")

	if _, err := dd.CheckInterim(context.Background(), "sess-1", 100, 200); err == nil {
		t.Error("expected error when Valkey is unavailable")
	}
}

func TestCheckAndMarkStart_AfterInterim(t *testing.T) {
	mr, dd := setupDuplicateDetector(t)
	mr.Set("acct:seen:sess-1", "interim:100:200")

	// Interim受信済みのセッションへのStartは重複として扱う
	isDup, err := dd.CheckAndMarkStart(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isDup {
		t.Error("start after interim should be duplicate")
	}
}

func TestCheckStopDuplicate(t *testing.T) {
	_, dd := setupDuplicateDetector(t)
	ctx := context.Background()

	// まだStopしていない
	isDup, err := dd.CheckStopDuplicate(ctx, "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isDup {
		t.Error("should not be duplicate before stop")
	}

	// Stopマーク
	_ = dd.MarkAsStopped(ctx, "sess-1")

	// Stop後
	isDup, err = dd.CheckStopDuplicate(ctx, "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isDup {
		t.Error("should be duplicate after stop")
	}
}
