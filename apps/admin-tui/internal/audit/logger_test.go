package audit

import (
	"bytes"
	"encoding/json"
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
}

func TestLogger_LogExport(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, "admin")

	logger.LogExport(TargetClient, 50, "clients.csv")

	output := buf.String()
	if !strings.Contains(output, `"operation":"export"`) {
		t.Error("expected operation to be export")
	}
}

func TestLogger_LogSearch(t *testing.T) {
	var buf bytes.Buffer
	logger := NewLoggerWithWriter(&buf, "admin")

	logger.LogSearch(TargetSession, "imsi=440*", 10)

	output := buf.String()
	if !strings.Contains(output, `"operation":"search"`) {
		t.Error("expected operation to be search")
	}
	if !strings.Contains(output, `"details":"imsi=440*"`) {
		t.Error("expected details to contain query")
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
