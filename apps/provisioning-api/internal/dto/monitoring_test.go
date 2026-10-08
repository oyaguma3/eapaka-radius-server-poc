package dto

import (
	"testing"
	"time"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

func TestAuditAction(t *testing.T) {
	tests := []struct {
		tt   audit.TargetType
		op   audit.Operation
		want string
	}{
		{audit.TargetSubscriber, audit.OpCreate, "subscriber.create"},
		{audit.TargetSubscriber, audit.OpUpdate, "subscriber.update"},
		{audit.TargetSubscriber, audit.OpDelete, "subscriber.delete"},
		{audit.TargetSubscriber, audit.OpRead, "subscriber.keys.read"},
		{audit.TargetClient, audit.OpRead, "client.secret.read"},
		{audit.TargetClient, audit.OpUpdate, "client.update"},
		{audit.TargetPolicy, audit.OpCreate, "policy.create"},
		{audit.TargetPolicy, audit.OpRead, "policy.read"},
	}
	for _, tc := range tests {
		if got := AuditAction(tc.tt, tc.op); got != tc.want {
			t.Errorf("AuditAction(%s, %s) = %s, want %s", tc.tt, tc.op, got, tc.want)
		}
	}
}

func TestNewAuditLogEntry(t *testing.T) {
	at := time.Date(2026, 10, 9, 1, 2, 3, 0, time.UTC)
	actor := audit.Actor{Operator: "alice", MgmtClient: "bff-01", TraceID: "t1"}

	sub := NewAuditLogEntry(audit.StoredEntry{ID: "1-0", Time: at, Actor: actor, Entry: audit.Entry{
		Operation: audit.OpUpdate, TargetType: audit.TargetSubscriber, TargetKey: "sub:001010000000001",
		TargetIMSI: "001010000000001", Details: "amf: 8000 -> b9b9",
	}})
	want := AuditLogEntry{ID: "1-0", Time: at, Operator: "alice", MgmtClient: "bff-01", Action: "subscriber.update",
		Target: "001010000000001", TargetKey: "sub:001010000000001", TraceID: "t1", Details: "amf: 8000 -> b9b9"}
	if sub != want {
		t.Errorf("subscriber = %+v", sub)
	}

	// RADIUSクライアントの対象は ID。
	cl := NewAuditLogEntry(audit.StoredEntry{Entry: audit.Entry{
		Operation: audit.OpRead, TargetType: audit.TargetClient, TargetKey: "client:192.0.2.1", TargetID: "3",
	}})
	if cl.Target != "3" || cl.Action != "client.secret.read" {
		t.Errorf("client = %+v", cl)
	}
	// 対象の識別子がなければキーを使う。
	if e := NewAuditLogEntry(audit.StoredEntry{Entry: audit.Entry{TargetType: audit.TargetPolicy, TargetKey: "policy:x"}}); e.Target != "policy:x" {
		t.Errorf("fallback target = %q", e.Target)
	}
}

func TestNewSession(t *testing.T) {
	s := NewSession(&model.Session{UUID: "u1", IMSI: "001010000000001", NasIP: "192.0.2.1", NasIdentifier: "AP-01",
		StartTime: 1760000000, ClientIP: "10.0.0.5", AcctSessionID: "A1", InputOctets: 10, OutputOctets: 20})
	want := Session{ID: "u1", IMSI: "001010000000001", NasIP: "192.0.2.1", NasIdentifier: "AP-01",
		StartTime: "2025-10-09T08:53:20Z", ClientIP: "10.0.0.5", AcctSessionID: "A1", InputOctets: 10, OutputOctets: 20}
	if s != want {
		t.Errorf("NewSession() = %+v", s)
	}
	if s := NewSession(&model.Session{UUID: "u2"}); s.StartTime != "" {
		t.Errorf("zero start time = %q", s.StartTime)
	}
}
