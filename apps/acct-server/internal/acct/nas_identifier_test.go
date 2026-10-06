package acct

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/acct-server/internal/radius"
)

// TestProcess_NASIdentifier は Start / Interim / Stop で、src_ip を持つログすべてに nas_identifier が出ること、
// Start / Interim でセッションの nas_identifier が更新されることを確認する（プロキシ経由でもNASを識別するため）
func TestProcess_NASIdentifier(t *testing.T) {
	tests := []struct {
		name      string
		status    uint32
		seen      string // acct:seen の直前の値（空なら未登録）
		session   bool   // sess:{UUID} を用意するか
		wantEvent []string
		wantSess  string // 処理後のセッションの nas_identifier（session=false なら見ない）
	}{
		{"start", radius.AcctStatusTypeStart, "", true, []string{"ACCT_START"}, "ap-001"},
		{"start session not found", radius.AcctStatusTypeStart, "", false, []string{"ACCT_SESSION_NOT_FOUND", "ACCT_START"}, ""},
		{"duplicate start", radius.AcctStatusTypeStart, "start", true, []string{"ACCT_DUPLICATE_START"}, "auth-value"},
		{"interim", radius.AcctStatusTypeInterim, "start", true, []string{"ACCT_INTERIM"}, "ap-001"},
		{"interim without start", radius.AcctStatusTypeInterim, "", true, []string{"ACCT_SEQUENCE_ERR", "ACCT_INTERIM"}, "ap-001"},
		{"stop", radius.AcctStatusTypeStop, "interim:0:0", true, []string{"ACCT_STOP"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mr, proc := setupProcessor(t)
			if tt.seen != "" {
				mr.Set("acct:seen:sess-nas", tt.seen)
			}
			if tt.session {
				mr.HSet("sess:uuid-nas", "imsi", "001010123456789", "nas_identifier", "auth-value")
			}
			logs := captureLog(t)

			attrs := &radius.AccountingAttributes{
				AcctStatusType: tt.status,
				AcctSessionID:  "sess-nas",
				ClassUUID:      "uuid-nas",
				UserName:       "0001010123456789@example.com",
				NasIdentifier:  "ap-001",
			}
			ctx := context.Background()
			switch tt.status {
			case radius.AcctStatusTypeStart:
				proc.ProcessStart(ctx, attrs, "172.30.0.10", "trace-nas")
			case radius.AcctStatusTypeInterim:
				proc.ProcessInterim(ctx, attrs, "172.30.0.10", "trace-nas")
			case radius.AcctStatusTypeStop:
				proc.ProcessStop(ctx, attrs, "172.30.0.10", "trace-nas")
			}

			events := map[string]bool{}
			for line := range strings.Lines(logs.String()) {
				var e map[string]any
				if err := json.Unmarshal([]byte(line), &e); err != nil {
					t.Fatalf("invalid log line %q: %v", line, err)
				}
				id, _ := e["event_id"].(string)
				events[id] = true
				if _, hasSrc := e["src_ip"]; hasSrc && e["nas_identifier"] != "ap-001" {
					t.Errorf("%s: nas_identifier = %v, want ap-001: %s", id, e["nas_identifier"], line)
				}
			}
			for _, ev := range tt.wantEvent {
				if !events[ev] {
					t.Errorf("%s が出力されていない: %s", ev, logs.String())
				}
			}
			if tt.session && tt.wantSess != "" {
				if got := mr.HGet("sess:uuid-nas", "nas_identifier"); got != tt.wantSess {
					t.Errorf("session nas_identifier = %q, want %q", got, tt.wantSess)
				}
			}
		})
	}
}
