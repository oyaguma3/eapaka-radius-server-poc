package acct

import (
	"context"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
)

func TestProcessStop(t *testing.T) {
	mr, proc := setupProcessor(t)
	ctx := context.Background()

	// セッション準備
	mr.HSet("sess:550e8400-e29b-41d4-a716-446655440000", "imsi", "001010123456789")
	mr.SAdd("idx:user:001010123456789", "550e8400-e29b-41d4-a716-446655440000")
	mr.Set("acct:seen:sess-123", "start")

	attrs := &radius.AccountingAttributes{
		AcctStatusType: radius.AcctStatusTypeStop,
		AcctSessionID:  "sess-123",
		ClassUUID:      "550e8400-e29b-41d4-a716-446655440000",
		InputOctets:    12345678,
		OutputOctets:   23456789,
		SessionTime:    1800,
	}

	err := proc.ProcessStop(ctx, attrs, "192.168.1.1", "trace-1")
	if err != nil {
		t.Fatalf("ProcessStop failed: %v", err)
	}

	// セッションが削除されていることを確認
	exists := mr.Exists("sess:550e8400-e29b-41d4-a716-446655440000")
	if exists {
		t.Error("session should be deleted after stop")
	}
}

func TestProcessStop_Duplicate(t *testing.T) {
	mr, proc := setupProcessor(t)
	ctx := context.Background()

	mr.Set("acct:seen:sess-123", "stop")

	attrs := &radius.AccountingAttributes{
		AcctStatusType: radius.AcctStatusTypeStop,
		AcctSessionID:  "sess-123",
	}

	// Stop重複 - エラーなしで正常終了
	err := proc.ProcessStop(ctx, attrs, "192.168.1.1", "trace-1")
	if err != nil {
		t.Fatalf("ProcessStop should not return error on duplicate: %v", err)
	}
}

func TestProcessStop_NoSession(t *testing.T) {
	_, proc := setupProcessor(t)
	ctx := context.Background()

	attrs := &radius.AccountingAttributes{
		AcctStatusType: radius.AcctStatusTypeStop,
		AcctSessionID:  "sess-123",
		ClassUUID:      "nonexistent-uuid",
	}

	// セッション不在でもエラーにならない
	err := proc.ProcessStop(ctx, attrs, "192.168.1.1", "trace-1")
	if err != nil {
		t.Fatalf("ProcessStop should not return error for missing session: %v", err)
	}
}
