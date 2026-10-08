package audit

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRecord(t *testing.T) {
	var buf bytes.Buffer
	l := NewLogger(&buf)
	actor := Actor{Operator: "alice", MgmtClient: "bff-01", TraceID: "trace-1"}

	l.Record(t.Context(), actor, Entry{Operation: OpCreate, TargetType: TargetSubscriber, TargetKey: "sub:001010000000001", TargetIMSI: "001010000000001", Details: "amf=8000"})
	var e map[string]any
	if err := json.Unmarshal(buf.Bytes(), &e); err != nil {
		t.Fatalf("not JSON: %s", buf.String())
	}
	want := map[string]any{
		"level": "INFO", "msg": "subscriber created", "app": "provisioning-api", "event_id": "AUDIT_LOG",
		"trace_id": "trace-1", "operation": "create", "target_type": "subscriber", "target_key": "sub:001010000000001",
		"target_imsi": "001010000000001", "admin_user": "alice", "mgmt_client": "bff-01", "details": "amf=8000",
	}
	for k, v := range want {
		if e[k] != v {
			t.Errorf("%s = %v, want %v", k, e[k], v)
		}
	}

	// IMSI と details がなければ出力しない。admin_user は空文字でも出力する
	buf.Reset()
	l.Record(t.Context(), Actor{MgmtClient: "bff-01"}, Entry{Operation: OpRead, TargetType: TargetClient, TargetKey: "client:192.168.10.1", TargetID: "3"})
	e = nil
	if err := json.Unmarshal(buf.Bytes(), &e); err != nil {
		t.Fatalf("not JSON: %s", buf.String())
	}
	if e["msg"] != "client secret read" || e["admin_user"] != "" || e["target_id"] != "3" {
		t.Errorf("entry = %v", e)
	}
	for _, k := range []string{"target_imsi", "details"} {
		if _, ok := e[k]; ok {
			t.Errorf("entry has %s", k)
		}
	}

	for op, msg := range map[Operation]string{OpUpdate: "policy updated", OpDelete: "policy deleted"} {
		buf.Reset()
		l.Record(t.Context(), actor, Entry{Operation: op, TargetType: TargetPolicy})
		if err := json.Unmarshal(buf.Bytes(), &e); err != nil || e["msg"] != msg {
			t.Errorf("msg = %v, want %s", e["msg"], msg)
		}
	}
}
