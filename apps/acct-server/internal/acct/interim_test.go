package acct

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
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

// captureLog はテスト中のslog出力をバッファに取り込む
func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewJSONHandler(&buf, nil)))
	t.Cleanup(func() { slog.SetDefault(prev) })
	return &buf
}

func TestProcessInterim_SequenceAndDuplicateLogs(t *testing.T) {
	tests := []struct {
		name       string
		prev       string // acct:seen の直前の値（空なら未登録）
		wantEvent  string // 出力されるべき event_id（空なら ACCT_SEQUENCE_ERR / ACCT_DUPLICATE_START が出ないこと）
		wantReason string
	}{
		{"interim after start has no sequence error", "start", "", ""},
		{"interim without start", "", "ACCT_SEQUENCE_ERR", "no_start_received"},
		{"interim after stop", "stop", "ACCT_SEQUENCE_ERR", "interim_after_stop"},
		{"duplicate interim", "interim:1000:2000", "ACCT_DUPLICATE_START", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr, proc := setupProcessor(t)
			if tt.prev != "" {
				mr.Set("acct:seen:sess-123", tt.prev)
			}
			logs := captureLog(t)

			attrs := &radius.AccountingAttributes{
				AcctStatusType: radius.AcctStatusTypeInterim,
				AcctSessionID:  "sess-123",
				InputOctets:    1000,
				OutputOctets:   2000,
			}
			if err := proc.ProcessInterim(context.Background(), attrs, "192.168.1.1", "trace-1"); err != nil {
				t.Fatalf("ProcessInterim failed: %v", err)
			}

			out := logs.String()
			for _, ev := range []string{"ACCT_SEQUENCE_ERR", "ACCT_DUPLICATE_START"} {
				has := strings.Contains(out, `"event_id":"`+ev+`"`)
				if has != (ev == tt.wantEvent) {
					t.Errorf("%s logged = %v, want %v: %s", ev, has, ev == tt.wantEvent, out)
				}
			}
			if tt.wantReason != "" && !strings.Contains(out, `"reason":"`+tt.wantReason+`"`) {
				t.Errorf("reason %q not logged: %s", tt.wantReason, out)
			}
			// 重複以外は ACCT_INTERIM まで処理される
			if has := strings.Contains(out, `"event_id":"ACCT_INTERIM"`); has == (tt.wantEvent == "ACCT_DUPLICATE_START") {
				t.Errorf("ACCT_INTERIM logged = %v: %s", has, out)
			}
		})
	}
}

func TestProcessInterim_SessionNotFound(t *testing.T) {
	mr, proc := setupProcessor(t)
	mr.Set("acct:seen:sess-123", "start")
	logs := captureLog(t)

	attrs := &radius.AccountingAttributes{
		AcctStatusType: radius.AcctStatusTypeInterim,
		AcctSessionID:  "sess-123",
		ClassUUID:      "missing-session-uuid",
		InputOctets:    1000,
		OutputOctets:   2000,
	}
	if err := proc.ProcessInterim(context.Background(), attrs, "192.168.1.1", "trace-1"); err != nil {
		t.Fatalf("ProcessInterim failed: %v", err)
	}

	// 不在のセッションキーを作らない
	if mr.Exists("sess:missing-session-uuid") {
		t.Error("session key must not be created for a missing session")
	}
	out := logs.String()
	if !strings.Contains(out, `"event_id":"ACCT_SESSION_NOT_FOUND"`) || !strings.Contains(out, `"class_uuid":"missing-session-uuid"`) {
		t.Errorf("ACCT_SESSION_NOT_FOUND not logged: %s", out)
	}
}

func TestProcessInterim_ValkeyError(t *testing.T) {
	mr, proc := setupProcessor(t)
	logs := captureLog(t)
	mr.SetError("connection refused")

	attrs := &radius.AccountingAttributes{
		AcctStatusType: radius.AcctStatusTypeInterim,
		AcctSessionID:  "sess-123",
		ClassUUID:      "some-session-uuid",
		InputOctets:    1000,
		OutputOctets:   2000,
	}
	// Valkey障害時もエラーを返さず、ログを出して処理を継続する
	if err := proc.ProcessInterim(context.Background(), attrs, "192.168.1.1", "trace-1"); err != nil {
		t.Fatalf("ProcessInterim should not return error: %v", err)
	}
	if out := logs.String(); !strings.Contains(out, `"event_id":"VALKEY_CONN_ERR"`) {
		t.Errorf("VALKEY_CONN_ERR not logged: %s", out)
	}
}
