package audit

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func newTestStore(t *testing.T, maxLen int64) (*Store, *miniredis.Miniredis) {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	return NewStore(rdb, maxLen), mr
}

func TestRecordStoresToStream(t *testing.T) {
	store, _ := newTestStore(t, 1000)
	var out, errLog bytes.Buffer
	l := NewLogger(&out).WithStore(store, slog.New(slog.NewJSONHandler(&errLog, nil)))

	actor := Actor{Operator: "alice", MgmtClient: "bff-01", TraceID: "trace-1"}
	l.Record(t.Context(), actor, Entry{Operation: OpCreate, TargetType: TargetSubscriber, TargetKey: "sub:001010000000001",
		TargetIMSI: "001010000000001", Details: "amf=8000, sqn=000000000000"})
	l.Record(t.Context(), Actor{MgmtClient: "bff-01", TraceID: "trace-2"}, Entry{Operation: OpRead, TargetType: TargetClient,
		TargetKey: "client:192.0.2.1", TargetID: "3"})

	if n := strings.Count(out.String(), `"event_id":"AUDIT_LOG"`); n != 2 {
		t.Errorf("stdout audit lines = %d", n)
	}
	entries, next, err := store.List(t.Context(), "", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 || next != "" {
		t.Fatalf("List() = %d entries, next %q", len(entries), next)
	}
	// 新しい順。
	if e := entries[0]; e.TraceID != "trace-2" || e.Operation != OpRead || e.TargetType != TargetClient || e.TargetID != "3" ||
		e.Operator != "" || e.MgmtClient != "bff-01" || e.Time.IsZero() {
		t.Errorf("entries[0] = %+v", e)
	}
	if e := entries[1]; e.Operator != "alice" || e.TargetIMSI != "001010000000001" || e.TargetKey != "sub:001010000000001" ||
		e.Details != "amf=8000, sqn=000000000000" || e.Operation != OpCreate {
		t.Errorf("entries[1] = %+v", e)
	}
	if errLog.Len() != 0 {
		t.Errorf("error log = %s", errLog.String())
	}
}

func TestStoreListPaging(t *testing.T) {
	store, _ := newTestStore(t, 1000)
	for i := range 5 {
		if err := store.add(t.Context(), Actor{TraceID: fmt.Sprint(i)}, Entry{Operation: OpDelete, TargetType: TargetPolicy}); err != nil {
			t.Fatal(err)
		}
	}
	var got []string
	before := ""
	for range 10 {
		entries, next, err := store.List(t.Context(), before, 2)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			got = append(got, e.TraceID)
		}
		if next == "" {
			break
		}
		before = next
	}
	if strings.Join(got, ",") != "4,3,2,1,0" {
		t.Errorf("paged trace ids = %v", got)
	}
	// 最古のものより前はない。
	if entries, next, err := store.List(t.Context(), "0-0", 10); err != nil || len(entries) != 0 || next != "" {
		t.Errorf("List(0-0) = %v, %q, %v", entries, next, err)
	}
	if _, _, err := store.List(t.Context(), "abc", 10); err == nil {
		t.Error("List(abc) want error")
	}
}

func TestStoreMaxLen(t *testing.T) {
	store, mr := newTestStore(t, 3)
	for range 10 {
		if err := store.add(t.Context(), Actor{}, Entry{Operation: OpDelete, TargetType: TargetPolicy}); err != nil {
			t.Fatal(err)
		}
	}
	// MAXLEN ~ はおおよその上限なので、上限より大きく増え続けないことだけを確かめる。
	entries, err := mr.Stream(StreamKey)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) > 10 || len(entries) < 3 {
		t.Errorf("stream length = %d", len(entries))
	}
}

func TestPreviousID(t *testing.T) {
	tests := map[string]string{
		"5-3": "5-2",
		"5-0": "4-18446744073709551615",
		"0-0": "",
	}
	for in, want := range tests {
		if got, err := previousID(in); err != nil || got != want {
			t.Errorf("previousID(%s) = %q, %v; want %q", in, got, err, want)
		}
	}
	for _, bad := range []string{"", "5", "a-1", "1-2-3", "99999999999999999999-0"} {
		if _, err := previousID(bad); err == nil {
			t.Errorf("previousID(%q) want error", bad)
		}
	}
}

func TestRecordStoreFailure(t *testing.T) {
	store, mr := newTestStore(t, 1000)
	var out, errLog bytes.Buffer
	l := NewLogger(&out).WithStore(store, slog.New(slog.NewJSONHandler(&errLog, nil)))
	mr.SetError("forced error")

	l.Record(t.Context(), Actor{TraceID: "trace-x"}, Entry{Operation: OpCreate, TargetType: TargetPolicy, TargetKey: "policy:1"})
	// 標準出力（正本）には出し、保存の失敗は PROV_AUDIT_STORE_ERR として記録する。
	if !strings.Contains(out.String(), `"trace_id":"trace-x"`) {
		t.Errorf("stdout = %s", out.String())
	}
	var m map[string]any
	if err := json.Unmarshal(errLog.Bytes(), &m); err != nil {
		t.Fatalf("error log = %q", errLog.String())
	}
	if m["event_id"] != "PROV_AUDIT_STORE_ERR" || m["level"] != "ERROR" || m["trace_id"] != "trace-x" || m["error"] == nil {
		t.Errorf("error log = %v", m)
	}
}
