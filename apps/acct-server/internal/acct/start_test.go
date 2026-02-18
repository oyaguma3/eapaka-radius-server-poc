package acct

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/session"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/store"
)

func setupProcessor(t *testing.T) (*miniredis.Miniredis, *Processor) {
	t.Helper()
	mr := miniredis.RunT(t)
	cfg := newTestConfig(mr.Addr())
	vc, err := store.NewValkeyClient(cfg)
	if err != nil {
		t.Fatalf("NewValkeyClient failed: %v", err)
	}
	t.Cleanup(func() { vc.Close() })

	ss := store.NewSessionStore(vc)
	ds := store.NewDuplicateStore(vc)
	mgr := session.NewManager(ss)
	dd := NewDuplicateDetector(ds)
	ir := session.NewIdentifierResolver(mgr, true)

	return mr, NewProcessor(mgr, dd, ir)
}

func TestProcessStart(t *testing.T) {
	mr, proc := setupProcessor(t)
	ctx := context.Background()

	// セッション準備
	mr.HSet("sess:550e8400-e29b-41d4-a716-446655440000", "imsi", "001010123456789")

	attrs := &radius.AccountingAttributes{
		AcctStatusType:  radius.AcctStatusTypeStart,
		AcctSessionID:   "sess-123",
		ClassUUID:       "550e8400-e29b-41d4-a716-446655440000",
		UserName:        "0001010123456789@example.com",
		NasIPAddress:    "192.168.1.1",
		FramedIPAddress: "10.0.0.1",
	}

	err := proc.ProcessStart(ctx, attrs, "192.168.1.1", "trace-1")
	if err != nil {
		t.Fatalf("ProcessStart failed: %v", err)
	}

	// セッションが更新されていることを確認
	acctID := mr.HGet("sess:550e8400-e29b-41d4-a716-446655440000", "acct_id")
	if acctID != "sess-123" {
		t.Errorf("acct_id = %q, want %q", acctID, "sess-123")
	}
}

func TestProcessStart_Duplicate(t *testing.T) {
	_, proc := setupProcessor(t)
	ctx := context.Background()

	attrs := &radius.AccountingAttributes{
		AcctStatusType: radius.AcctStatusTypeStart,
		AcctSessionID:  "sess-123",
	}

	// 1回目
	_ = proc.ProcessStart(ctx, attrs, "192.168.1.1", "trace-1")

	// 2回目（重複）- エラーなしで正常終了するべき
	err := proc.ProcessStart(ctx, attrs, "192.168.1.1", "trace-2")
	if err != nil {
		t.Fatalf("ProcessStart should not return error on duplicate: %v", err)
	}
}

func TestProcessStart_NoSession(t *testing.T) {
	_, proc := setupProcessor(t)
	ctx := context.Background()

	attrs := &radius.AccountingAttributes{
		AcctStatusType: radius.AcctStatusTypeStart,
		AcctSessionID:  "sess-123",
		ClassUUID:      "nonexistent-uuid",
	}

	// セッション不在でもエラーにならない
	err := proc.ProcessStart(ctx, attrs, "192.168.1.1", "trace-1")
	if err != nil {
		t.Fatalf("ProcessStart should not return error for missing session: %v", err)
	}
}

func TestProcessStart_AfterStop(t *testing.T) {
	_, proc := setupProcessor(t)
	ctx := context.Background()

	attrs := &radius.AccountingAttributes{
		AcctStatusType: radius.AcctStatusTypeStart,
		AcctSessionID:  "sess-restart",
	}

	// Stopをマーク
	_ = proc.duplicateDetector.MarkAsStopped(ctx, "sess-restart")

	// Stop後のStart（SequenceErrorが発生するが処理は継続）
	err := proc.ProcessStart(ctx, attrs, "192.168.1.1", "trace-1")
	if err != nil {
		t.Fatalf("ProcessStart should not return error after stop: %v", err)
	}
}

func TestProcessStart_WithExistingSession(t *testing.T) {
	mr, proc := setupProcessor(t)
	ctx := context.Background()

	// セッション準備（存在するセッション）
	mr.HSet("sess:existing-uuid", "imsi", "001010987654321", "status", "pending")

	attrs := &radius.AccountingAttributes{
		AcctStatusType:  radius.AcctStatusTypeStart,
		AcctSessionID:   "sess-456",
		ClassUUID:       "existing-uuid",
		UserName:        "0001010987654321@example.com",
		NasIPAddress:    "192.168.1.1",
		FramedIPAddress: "10.0.0.2",
	}

	err := proc.ProcessStart(ctx, attrs, "192.168.1.1", "trace-1")
	if err != nil {
		t.Fatalf("ProcessStart failed: %v", err)
	}

	// セッションが更新されていることを確認
	acctID := mr.HGet("sess:existing-uuid", "acct_id")
	if acctID != "sess-456" {
		t.Errorf("acct_id = %q, want %q", acctID, "sess-456")
	}
}
