package masterdata

import (
	"errors"
	"slices"
	"testing"

	"github.com/alicebob/miniredis/v2"
)

// seedSession は sess:{UUID} を作り、索引 idx:user:{IMSI} にも加える。
func seedSession(t *testing.T, mr *miniredis.Miniredis, uuid, imsi string, fields ...string) {
	t.Helper()
	mr.HSet(SessionKey(uuid), append([]string{"imsi", imsi}, fields...)...)
	if _, err := mr.SetAdd(UserIndexKey(imsi), uuid); err != nil {
		t.Fatal(err)
	}
}

func TestSessionKeys(t *testing.T) {
	if got := SessionKey("u1"); got != "sess:u1" {
		t.Errorf("SessionKey() = %s", got)
	}
	if got := UserIndexKey("440100000000001"); got != "idx:user:440100000000001" {
		t.Errorf("UserIndexKey() = %s", got)
	}
}

func TestSessionStore_GetListCount(t *testing.T) {
	mr, client := newTestRedis(t)
	defer client.Close()
	ss := NewSessionStore(client)
	ctx := t.Context()

	seedSession(t, mr, "u1", "440100000000001",
		"nas_ip", "192.0.2.1", "nas_identifier", "AP-01", "start_time", "1700000000",
		"client_ip", "10.0.0.5", "acct_id", "A1", "input_octets", "100", "output_octets", "200")
	seedSession(t, mr, "u2", "440100000000002", "start_time", "1700000100")
	// 値を解釈できないセッションは一覧から除く。
	mr.HSet(SessionKey("bad"), "imsi", "440100000000003", "start_time", "x")
	// セッション以外のキーは数えない。
	mr.HSet(SubscriberKey("440100000000001"), "ki", "K")

	got, err := ss.Get(ctx, "u1")
	if err != nil {
		t.Fatal(err)
	}
	if got.IMSI != "440100000000001" || got.NasIP != "192.0.2.1" || got.NasIdentifier != "AP-01" || got.StartTime != 1700000000 ||
		got.ClientIP != "10.0.0.5" || got.AcctSessionID != "A1" || got.InputOctets != 100 || got.OutputOctets != 200 {
		t.Errorf("Get() = %+v", got)
	}
	if _, err := ss.Get(ctx, "missing"); !errors.Is(err, ErrSessionNotFound) {
		t.Errorf("Get(missing) error = %v", err)
	}
	if _, err := ss.Get(ctx, "bad"); err == nil {
		t.Error("Get(bad) want error")
	}

	list, err := ss.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var uuids []string
	for _, s := range list {
		uuids = append(uuids, s.UUID)
	}
	slices.Sort(uuids)
	if !slices.Equal(uuids, []string{"u1", "u2"}) {
		t.Errorf("List() uuids = %v", uuids)
	}
	if n, err := ss.Count(ctx); err != nil || n != 3 {
		t.Errorf("Count() = %d, %v", n, err)
	}
}

func TestSessionStore_ListByIMSI(t *testing.T) {
	mr, client := newTestRedis(t)
	defer client.Close()
	ss := NewSessionStore(client)
	ctx := t.Context()

	seedSession(t, mr, "u1", "440100000000001", "start_time", "1")
	seedSession(t, mr, "u2", "440100000000001", "start_time", "2")
	// 索引に残った、もう存在しないセッション（stale）。索引からは消さない。
	if _, err := mr.SetAdd(UserIndexKey("440100000000001"), "gone"); err != nil {
		t.Fatal(err)
	}
	sessions, stale, err := ss.ListByIMSI(ctx, "440100000000001")
	if err != nil {
		t.Fatal(err)
	}
	if len(sessions) != 2 || !slices.Equal(stale, []string{"gone"}) {
		t.Errorf("ListByIMSI() = %d sessions, stale %v", len(sessions), stale)
	}
	if ok, _ := mr.SIsMember(UserIndexKey("440100000000001"), "gone"); !ok {
		t.Error("stale uuid was removed from the index")
	}
	if n, err := ss.IndexCount(ctx, "440100000000001"); err != nil || n != 3 {
		t.Errorf("IndexCount() = %d, %v", n, err)
	}

	// 索引がない場合は SCAN して IMSI で絞り込む。
	mr.HSet(SessionKey("u3"), "imsi", "440100000000002", "start_time", "3")
	sessions, stale, err = ss.ListByIMSI(ctx, "440100000000002")
	if err != nil || len(sessions) != 1 || sessions[0].UUID != "u3" || stale != nil {
		t.Errorf("ListByIMSI(scan) = %v, %v, %v", sessions, stale, err)
	}
	// 該当がなければ空のスライス。
	sessions, _, err = ss.ListByIMSI(ctx, "440100000000009")
	if err != nil || sessions == nil || len(sessions) != 0 {
		t.Errorf("ListByIMSI(none) = %v, %v", sessions, err)
	}
}
