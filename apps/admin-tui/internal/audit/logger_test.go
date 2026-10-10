package audit

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestLogger_Log(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, "testadmin")

	logger.Log(OpCreate, TargetSubscriber, "sub:440101234567890", "440101234567890", "subscriber created")

	output := buf.String()

	// Verify JSON format
	var entry Entry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if entry.Level != "INFO" {
		t.Errorf("expected level INFO, got %s", entry.Level)
	}
	if entry.App != "admin-tui" {
		t.Errorf("expected app admin-tui, got %s", entry.App)
	}
	if entry.EventID != "AUDIT_LOG" {
		t.Errorf("expected event_id AUDIT_LOG, got %s", entry.EventID)
	}
	if entry.Operation != OpCreate {
		t.Errorf("expected operation create, got %s", entry.Operation)
	}
	if entry.TargetType != TargetSubscriber {
		t.Errorf("expected target_type subscriber, got %s", entry.TargetType)
	}
	if entry.TargetKey != "sub:440101234567890" {
		t.Errorf("expected target_key sub:440101234567890, got %s", entry.TargetKey)
	}
	if entry.TargetIMSI != "440101234567890" {
		t.Errorf("expected target_imsi 440101234567890, got %s", entry.TargetIMSI)
	}
	if entry.AdminUser != "testadmin" {
		t.Errorf("expected admin_user testadmin, got %s", entry.AdminUser)
	}
}

func TestLogger_LogWithDetails(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, "admin")

	logger.LogWithDetails(OpSearch, TargetSession, "", "", "session searched", "imsi=440*")

	output := buf.String()

	var entry Entry
	if err := json.Unmarshal([]byte(output), &entry); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	if entry.Details != "imsi=440*" {
		t.Errorf("expected details 'imsi=440*', got '%s'", entry.Details)
	}
}

func TestLogger_LogCreate(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, "admin")

	logger.LogCreate(TargetClient, "client:192.168.1.1", "")

	output := buf.String()
	if !strings.Contains(output, `"operation":"create"`) {
		t.Error("expected operation to be create")
	}
	if !strings.Contains(output, `"msg":"client created"`) {
		t.Error("expected msg to be 'client created'")
	}
}

func TestLogger_LogUpdate(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, "admin")

	logger.LogUpdate(TargetPolicy, "policy:440101234567890", "440101234567890")

	output := buf.String()
	if !strings.Contains(output, `"operation":"update"`) {
		t.Error("expected operation to be update")
	}
}

func TestLogger_LogDelete(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, "admin")

	logger.LogDelete(TargetSubscriber, "sub:440101234567890", "440101234567890")

	output := buf.String()
	if !strings.Contains(output, `"operation":"delete"`) {
		t.Error("expected operation to be delete")
	}
}

func TestLogger_LogImport(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, "admin")

	logger.LogImport(TargetSubscriber, 100, "subscribers.csv")

	output := buf.String()
	if !strings.Contains(output, `"operation":"import"`) {
		t.Error("expected operation to be import")
	}
	if !strings.Contains(output, `"target_key":"subscribers.csv"`) {
		t.Error("expected target_key to be filename")
	}
	if !strings.Contains(output, `"record_count":100`) {
		t.Errorf("expected record_count to be 100: %s", output)
	}
}

func TestLogger_LogExport(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, "admin")

	logger.LogExport(TargetClient, 50, "clients.csv")

	output := buf.String()
	if !strings.Contains(output, `"operation":"export"`) {
		t.Error("expected operation to be export")
	}
	if !strings.Contains(output, `"record_count":50`) {
		t.Errorf("expected record_count to be 50: %s", output)
	}
}

func TestLogger_LogExport_ZeroRecords(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, "admin")

	// 0件のエクスポートも件数を記録する
	logger.LogExport(TargetPolicy, 0, "policies.csv")

	if output := buf.String(); !strings.Contains(output, `"record_count":0`) {
		t.Errorf("expected record_count to be 0: %s", output)
	}
}

func TestLogger_LogSearch(t *testing.T) {
	tests := []struct {
		name       string
		targetType TargetType
		query      string
		count      int
		err        error
		want       []string
		notWant    []string
	}{
		{
			name: "session search records imsi and result count", targetType: TargetSession,
			query: "440101234567890", count: 2,
			want:    []string{`"operation":"search"`, `"target_imsi":"440101234567890"`, `"result_count":2`},
			notWant: []string{`"details"`},
		},
		{
			name: "session search with no result", targetType: TargetSession,
			query: "440101234567890", count: 0,
			want: []string{`"result_count":0`},
		},
		{
			name: "search failure records reason without count", targetType: TargetSession,
			query: "440101234567890", err: errors.New("connection refused"),
			want:    []string{`"target_imsi":"440101234567890"`, `"details":"search failed: connection refused"`},
			notWant: []string{`"result_count"`},
		},
		{
			name: "non-session search records query in details", targetType: TargetSubscriber,
			query: "imsi=440*", count: 10,
			want:    []string{`"details":"imsi=440*"`, `"result_count":10`},
			notWant: []string{`"target_imsi"`},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			NewLoggerWithWriter(&buf, "admin").LogSearch(tt.targetType, tt.query, tt.count, tt.err)
			output := buf.String()
			for _, w := range tt.want {
				if !strings.Contains(output, w) {
					t.Errorf("expected %s in %s", w, output)
				}
			}
			for _, nw := range tt.notWant {
				if strings.Contains(output, nw) {
					t.Errorf("unexpected %s in %s", nw, output)
				}
			}
		})
	}
}

func TestLogger_LogCreate_NoCounts(t *testing.T) {
	var buf bytes.Buffer
	NewLoggerWithWriter(&buf, "admin").LogCreate(TargetSubscriber, "sub:440101234567890", "440101234567890")
	// 件数フィールドは import / export / search 以外では出力しない
	if output := buf.String(); strings.Contains(output, "record_count") || strings.Contains(output, "result_count") {
		t.Errorf("unexpected count fields: %s", output)
	}
}

func TestNewLogger(t *testing.T) {
	logger := NewLogger("admin")
	if logger == nil {
		t.Error("expected logger to be non-nil")
	}
	if logger.adminUser != "admin" {
		t.Errorf("expected adminUser to be 'admin', got '%s'", logger.adminUser)
	}
}

func TestLogStatusChange(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, "admin")

	logger.LogStatusChange("policy:440101234567890", "440101234567890", "active", "suspended")
	logger.LogStatusChange("policy:440101234567890", "440101234567890", "suspended", "active")

	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("lines = %d, want 2", len(lines))
	}
	want := []struct{ op, msg, details string }{
		{"suspend", "policy suspended", "status: active -> suspended"},
		{"resume", "policy resumed", "status: suspended -> active"},
	}
	for i, line := range lines {
		var e Entry
		if err := json.Unmarshal([]byte(line), &e); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if string(e.Operation) != want[i].op || e.Msg != want[i].msg || e.Details != want[i].details ||
			e.TargetType != TargetPolicy || e.TargetIMSI != "440101234567890" {
			t.Errorf("entry[%d] = %+v", i, e)
		}
	}
}
