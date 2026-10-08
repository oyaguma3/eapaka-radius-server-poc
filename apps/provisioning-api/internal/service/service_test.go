package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/dto"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/masterdata"
	"github.com/redis/go-redis/v9"
)

const (
	testIMSI = "001010000000001"
	testKi   = "465b5ce8b199b49faa5f0a2ee238a6bc"
	testOPc  = "cd63cb71954a9f4e48a5994e37a02baf"
	testIP   = "192.168.10.1"
)

var testActor = audit.Actor{Operator: "alice", MgmtClient: "bff-01", TraceID: "trace-1"}

// newTestService は miniredis を使う Service と、監査ログの出力先を返す。
func newTestService(t *testing.T) (*Service, *miniredis.Miniredis, *bytes.Buffer) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	var buf bytes.Buffer
	store := audit.NewStore(rdb, 1000)
	svc := New(rdb, audit.NewLogger(&buf).WithStore(store, slog.New(slog.NewJSONHandler(io.Discard, nil))), store)
	svc.now = func() time.Time { return time.Date(2026, 10, 7, 12, 0, 0, 0, time.FixedZone("JST", 9*3600)) }
	return svc, mr, &buf
}

// auditEntries は監査ログの出力を1行ずつ JSON として読む。
func auditEntries(t *testing.T, buf *bytes.Buffer) []map[string]any {
	t.Helper()
	var entries []map[string]any
	for line := range strings.Lines(buf.String()) {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("audit log is not JSON: %q", line)
		}
		entries = append(entries, m)
	}
	return entries
}

// lastAudit は最後の監査ログを返す。
func lastAudit(t *testing.T, buf *bytes.Buffer) map[string]any {
	t.Helper()
	entries := auditEntries(t, buf)
	if len(entries) == 0 {
		t.Fatal("no audit log")
	}
	return entries[len(entries)-1]
}

// validationError は err が ValidationError であることを確かめて返す。
func validationError(t *testing.T, err error) *ValidationError {
	t.Helper()
	var ve *ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("error = %v, want *ValidationError", err)
	}
	return ve
}

// paramNames は不正だった項目の名前を返す。
func paramNames(ve *ValidationError) []string {
	var names []string
	for _, p := range ve.Params() {
		names = append(names, p.Param)
	}
	return names
}

func ptr[T any](v T) *T { return &v }

func set[T any](v T) dto.Optional[T] { return dto.Optional[T]{Set: true, Value: v} }

func TestValidationError_Cause(t *testing.T) {
	p := []dto.InvalidParam{{Param: "x"}}
	tests := []struct {
		name string
		v    ValidationError
		want string
	}{
		{"query", ValidationError{Query: p, Missing: p}, dto.CauseInvalidQueryParam},
		{"missing", ValidationError{Missing: p, Mandatory: p}, dto.CauseMandatoryIEMissing},
		{"mandatory", ValidationError{Mandatory: p, Optional: p}, dto.CauseMandatoryIEIncorrect},
		{"optional", ValidationError{Optional: p}, dto.CauseOptionalIEIncorrect},
		{"detail only", ValidationError{Detail: "empty"}, dto.CauseMandatoryIEMissing},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.v.Cause(); got != tt.want {
				t.Errorf("Cause() = %s, want %s", got, tt.want)
			}
			if !strings.Contains(tt.v.Error(), tt.want) {
				t.Errorf("Error() = %s", tt.v.Error())
			}
		})
	}
	if err := (&ValidationError{}).orNil(); err != nil {
		t.Errorf("orNil() = %v, want nil", err)
	}
}

func TestParseListQuery(t *testing.T) {
	tests := []struct {
		name       string
		q          dto.ListQuery
		wantLimit  int
		wantParams []string
	}{
		{"default", dto.ListQuery{}, 50, nil},
		{"all valid", dto.ListQuery{Prefix: "00101", Cursor: testIMSI, Limit: "500"}, 500, nil},
		{"limit 1", dto.ListQuery{Limit: "1"}, 1, nil},
		{"limit 0", dto.ListQuery{Limit: "0"}, 0, []string{"limit"}},
		{"limit 501", dto.ListQuery{Limit: "501"}, 0, []string{"limit"}},
		{"limit not a number", dto.ListQuery{Limit: "abc"}, 0, []string{"limit"}},
		{"bad prefix and cursor", dto.ListQuery{Prefix: "0010a", Cursor: "1234567890123456"}, 0, []string{"prefix", "cursor"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, limit, err := parseListQuery(tt.q)
			if tt.wantParams == nil {
				if err != nil || limit != tt.wantLimit {
					t.Fatalf("parseListQuery() = %d, %v, want %d, nil", limit, err, tt.wantLimit)
				}
				return
			}
			ve := validationError(t, err)
			if ve.Cause() != dto.CauseInvalidQueryParam || !slices.Equal(paramNames(ve), tt.wantParams) {
				t.Errorf("cause = %s, params = %v, want %v", ve.Cause(), paramNames(ve), tt.wantParams)
			}
		})
	}
}

func TestCounts(t *testing.T) {
	svc, mr, _ := newTestService(t)
	ctx := context.Background()
	mr.HSet(masterdata.SubscriberKey(testIMSI), "ki", "K")
	mr.HSet(masterdata.SubscriberKey("001010000000002"), "ki", "K")
	mr.HSet(masterdata.ClientKey(testIP), "secret", "s")
	mr.HSet(masterdata.PolicyKey(testIMSI), "default", "deny")

	c, err := svc.Counts(ctx)
	if err != nil {
		t.Fatalf("Counts() error = %v", err)
	}
	if c.Subscribers != 2 || c.Clients != 1 || c.Policies != 1 {
		t.Errorf("Counts() = %+v", c)
	}

	mr.SetError("forced error")
	if _, err := svc.Counts(ctx); err == nil {
		t.Error("Counts() expected error")
	}
}

func TestReason(t *testing.T) {
	if got := reason(errors.New("plain")); got != "plain" {
		t.Errorf("reason() = %q", got)
	}
}

func TestAuditLogsAndSessions(t *testing.T) {
	svc, mr, _ := newTestService(t)
	ctx := context.Background()

	// 監査ログ: 変更操作が Stream にも保存され、新しい順に読める。
	if _, err := svc.CreateSubscriber(ctx, testActor, dto.SubscriberCreate{IMSI: ptr(testIMSI), Ki: ptr(testKi), OPc: ptr(testOPc)}); err != nil {
		t.Fatal(err)
	}
	if err := svc.DeleteSubscriber(ctx, testActor, testIMSI); err != nil {
		t.Fatal(err)
	}
	entries, next, err := svc.AuditLogs(ctx, dto.AuditLogQuery{Limit: "1"})
	if err != nil || len(entries) != 1 || entries[0].Operation != audit.OpDelete || next == "" {
		t.Fatalf("AuditLogs(limit=1) = %+v, %q, %v", entries, next, err)
	}
	entries, next, err = svc.AuditLogs(ctx, dto.AuditLogQuery{Before: next})
	if err != nil || len(entries) != 1 || entries[0].Operation != audit.OpCreate || entries[0].Operator != "alice" || next != "" {
		t.Errorf("AuditLogs(before) = %+v, %q, %v", entries, next, err)
	}
	var ve *ValidationError
	if _, _, err := svc.AuditLogs(ctx, dto.AuditLogQuery{Before: "x", Limit: "abc"}); !errors.As(err, &ve) || len(ve.Query) != 2 {
		t.Errorf("AuditLogs(invalid) error = %v", err)
	}

	// セッション: 接続開始の新しい順、limit と total、IMSI での絞り込み。
	mr.HSet(masterdata.SessionKey("u1"), "imsi", testIMSI, "start_time", "100")
	mr.HSet(masterdata.SessionKey("u2"), "imsi", "001010000000002", "start_time", "300")
	mr.HSet(masterdata.SessionKey("u3"), "imsi", testIMSI, "start_time", "200")
	sessions, total, err := svc.Sessions(ctx, dto.SessionQuery{Limit: "2"})
	if err != nil || total != 3 || len(sessions) != 2 || sessions[0].UUID != "u2" || sessions[1].UUID != "u3" {
		t.Errorf("Sessions(limit=2) = %v, %d, %v", sessions, total, err)
	}
	sessions, total, err = svc.Sessions(ctx, dto.SessionQuery{IMSI: testIMSI})
	if err != nil || total != 2 || sessions[0].UUID != "u3" || sessions[1].UUID != "u1" {
		t.Errorf("Sessions(imsi) = %v, %d, %v", sessions, total, err)
	}
	if _, _, err := svc.Sessions(ctx, dto.SessionQuery{IMSI: "1", Limit: "0"}); !errors.As(err, &ve) || len(ve.Query) != 2 {
		t.Errorf("Sessions(invalid) error = %v", err)
	}
	c, err := svc.Counts(ctx)
	if err != nil || c.Sessions != 3 {
		t.Errorf("Counts() = %+v, %v", c, err)
	}

	mr.SetError("forced error")
	if _, _, err := svc.Sessions(ctx, dto.SessionQuery{IMSI: testIMSI}); err == nil {
		t.Error("Sessions(error) want error")
	}
	if _, err := svc.Counts(ctx); err == nil {
		t.Error("Counts(error) want error")
	}
}
