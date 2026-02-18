package acct

import (
	"context"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
)

func TestProcessInterim(t *testing.T) {
	mr, proc := setupProcessor(t)
	ctx := context.Background()

	// セッション準備＆Start記録
	mr.HSet("sess:550e8400-e29b-41d4-a716-446655440000", "imsi", "001010123456789")
	mr.Set("acct:seen:sess-123", "start")

	attrs := &radius.AccountingAttributes{
		AcctStatusType:  radius.AcctStatusTypeInterim,
		AcctSessionID:   "sess-123",
		ClassUUID:       "550e8400-e29b-41d4-a716-446655440000",
		NasIPAddress:    "192.168.1.1",
		FramedIPAddress: "10.0.0.1",
		InputOctets:     1000,
		OutputOctets:    2000,
	}

	err := proc.ProcessInterim(ctx, attrs, "192.168.1.1", "trace-1")
	if err != nil {
		t.Fatalf("ProcessInterim failed: %v", err)
	}
}

func TestProcessInterim_Duplicate(t *testing.T) {
	mr, proc := setupProcessor(t)
	ctx := context.Background()

	mr.Set("acct:seen:sess-123", "start")

	attrs := &radius.AccountingAttributes{
		AcctStatusType: radius.AcctStatusTypeInterim,
		AcctSessionID:  "sess-123",
		InputOctets:    1000,
		OutputOctets:   2000,
	}

	// 1回目
	_ = proc.ProcessInterim(ctx, attrs, "192.168.1.1", "trace-1")

	// 2回目（同値→重複）
	err := proc.ProcessInterim(ctx, attrs, "192.168.1.1", "trace-2")
	if err != nil {
		t.Fatalf("ProcessInterim should not return error on duplicate: %v", err)
	}
}

func TestProcessInterim_NoStart(t *testing.T) {
	_, proc := setupProcessor(t)
	ctx := context.Background()

	attrs := &radius.AccountingAttributes{
		AcctStatusType: radius.AcctStatusTypeInterim,
		AcctSessionID:  "sess-new",
		InputOctets:    1000,
		OutputOctets:   2000,
	}

	// StartなしでInterim - ACCT_SEQUENCE_ERRログが出るが正常終了
	err := proc.ProcessInterim(ctx, attrs, "192.168.1.1", "trace-1")
	if err != nil {
		t.Fatalf("ProcessInterim should not return error: %v", err)
	}
}

func TestProcessInterim_WithSession(t *testing.T) {
	mr, proc := setupProcessor(t)
	ctx := context.Background()

	// セッション準備
	mr.HSet("sess:interim-session-uuid", "imsi", "001010111222333", "status", "active")
	mr.Set("acct:seen:sess-interim", "start")

	attrs := &radius.AccountingAttributes{
		AcctStatusType:  radius.AcctStatusTypeInterim,
		AcctSessionID:   "sess-interim",
		ClassUUID:       "interim-session-uuid",
		NasIPAddress:    "192.168.1.2",
		FramedIPAddress: "10.0.0.5",
		InputOctets:     5000,
		OutputOctets:    10000,
	}

	err := proc.ProcessInterim(ctx, attrs, "192.168.1.2", "trace-1")
	if err != nil {
		t.Fatalf("ProcessInterim failed: %v", err)
	}

	// セッションが更新されていることを確認
	nasIP := mr.HGet("sess:interim-session-uuid", "nas_ip")
	if nasIP != "192.168.1.2" {
		t.Errorf("nas_ip = %q, want %q", nasIP, "192.168.1.2")
	}
}

func TestProcessInterim_DifferentOctets(t *testing.T) {
	mr, proc := setupProcessor(t)
	ctx := context.Background()

	mr.Set("acct:seen:sess-123", "start")

	attrs := &radius.AccountingAttributes{
		AcctStatusType: radius.AcctStatusTypeInterim,
		AcctSessionID:  "sess-123",
		InputOctets:    1000,
		OutputOctets:   2000,
	}

	// 1回目
	_ = proc.ProcessInterim(ctx, attrs, "192.168.1.1", "trace-1")

	// 2回目（異なる値→非重複）
	attrs.InputOctets = 2000
	attrs.OutputOctets = 4000
	err := proc.ProcessInterim(ctx, attrs, "192.168.1.1", "trace-2")
	if err != nil {
		t.Fatalf("ProcessInterim should not return error: %v", err)
	}
}
