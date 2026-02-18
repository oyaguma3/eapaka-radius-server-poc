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

func TestCheckInterimDuplicate(t *testing.T) {
	_, dd := setupDuplicateDetector(t)
	ctx := context.Background()

	// 初回
	isDup, err := dd.CheckInterimDuplicate(ctx, "sess-1", 100, 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isDup {
		t.Error("should not be duplicate for first interim")
	}

	// 同値（重複）
	isDup, err = dd.CheckInterimDuplicate(ctx, "sess-1", 100, 200)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isDup {
		t.Error("should be duplicate for same values")
	}

	// 異なる値（非重複）
	isDup, err = dd.CheckInterimDuplicate(ctx, "sess-1", 200, 400)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if isDup {
		t.Error("should not be duplicate for different values")
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

func TestHasSeenStart(t *testing.T) {
	_, dd := setupDuplicateDetector(t)
	ctx := context.Background()

	// 未登録
	seen, err := dd.HasSeenStart(ctx, "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seen {
		t.Error("should not have seen start")
	}

	// Start登録後
	_, _ = dd.CheckAndMarkStart(ctx, "sess-1")
	seen, err = dd.HasSeenStart(ctx, "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !seen {
		t.Error("should have seen start")
	}
}

func TestHasSeenStart_AfterInterim(t *testing.T) {
	_, dd := setupDuplicateDetector(t)
	ctx := context.Background()

	// Interimを記録
	_, _ = dd.CheckInterimDuplicate(ctx, "sess-1", 100, 200)

	// interim:XXX:YYY形式もStartとみなす
	seen, err := dd.HasSeenStart(ctx, "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !seen {
		t.Error("should have seen start (from interim)")
	}
}

func TestMarkAsStart(t *testing.T) {
	_, dd := setupDuplicateDetector(t)
	ctx := context.Background()

	// 初期状態ではStartを見ていない
	seen, err := dd.HasSeenStart(ctx, "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if seen {
		t.Error("should not have seen start initially")
	}

	// MarkAsStartを直接呼び出し
	if err := dd.MarkAsStart(ctx, "sess-1"); err != nil {
		t.Fatalf("MarkAsStart failed: %v", err)
	}

	// Startを見たことになる
	seen, err = dd.HasSeenStart(ctx, "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !seen {
		t.Error("should have seen start after MarkAsStart")
	}
}

func TestMarkAsStart_Overwrite(t *testing.T) {
	_, dd := setupDuplicateDetector(t)
	ctx := context.Background()

	// 最初にInterimを記録
	_, _ = dd.CheckInterimDuplicate(ctx, "sess-1", 100, 200)

	// MarkAsStartで上書き
	if err := dd.MarkAsStart(ctx, "sess-1"); err != nil {
		t.Fatalf("MarkAsStart failed: %v", err)
	}

	// HasSeenStartはtrueを返す
	seen, err := dd.HasSeenStart(ctx, "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !seen {
		t.Error("should have seen start")
	}

	// CheckAndMarkStartは重複とみなす（startが設定されているため）
	isDup, err := dd.CheckAndMarkStart(ctx, "sess-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !isDup {
		t.Error("should be duplicate after MarkAsStart")
	}
}
