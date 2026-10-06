package monitoring

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

func testSessions() []*model.Session {
	return []*model.Session{
		{UUID: "u1", IMSI: "440100000000002", NasIP: "10.0.0.2", StartTime: 100},
		{UUID: "u2", IMSI: "440100000000001", NasIP: "10.0.0.1", StartTime: 300},
		{UUID: "u3", IMSI: "440100000000001", NasIP: "10.0.0.2", StartTime: 200},
	}
}

func uuids(ss []*model.Session) []string {
	out := make([]string, len(ss))
	for i, s := range ss {
		out[i] = s.UUID
	}
	return out
}

func TestNextSortField(t *testing.T) {
	if got := nextSortField(SortByStartTime); got != SortByNasIP {
		t.Errorf("StartTime -> %v, want NasIP", got)
	}
	if got := nextSortField(SortByNasIP); got != SortByIMSI {
		t.Errorf("NasIP -> %v, want IMSI", got)
	}
	if got := nextSortField(SortByIMSI); got != SortByStartTime {
		t.Errorf("IMSI -> %v, want StartTime", got)
	}
}

func TestSessionListScreen_ToggleSortDirections(t *testing.T) {
	s := NewSessionListScreen(ui.NewApp(), nil)
	s.sessions = testSessions()
	s.sortSessions()

	// 既定: 開始時刻の新しい順
	if got, want := uuids(s.sessions), []string{"u2", "u3", "u1"}; !equal(got, want) {
		t.Errorf("default order = %v, want %v", got, want)
	}

	tests := []struct {
		field    SortField
		wantDesc bool
		want     []string
	}{
		// NAS IP 昇順。同じ NAS IP は開始時刻の新しい順
		{SortByNasIP, false, []string{"u2", "u3", "u1"}},
		// IMSI 昇順。同じ IMSI は開始時刻の新しい順
		{SortByIMSI, false, []string{"u2", "u3", "u1"}},
		// 一巡して開始時刻の新しい順に戻る
		{SortByStartTime, true, []string{"u2", "u3", "u1"}},
	}
	for _, tt := range tests {
		s.ToggleSort()
		if s.sortField != tt.field || s.sortDesc != tt.wantDesc {
			t.Fatalf("sortField=%v sortDesc=%v, want %v %v", s.sortField, s.sortDesc, tt.field, tt.wantDesc)
		}
		if got := uuids(s.sessions); !equal(got, tt.want) {
			t.Errorf("field %v order = %v, want %v", tt.field, got, tt.want)
		}
	}
}

func TestSessionListScreen_SortIsAscendingForIMSI(t *testing.T) {
	s := NewSessionListScreen(ui.NewApp(), nil)
	s.sessions = []*model.Session{
		{UUID: "a", IMSI: "440100000000009", StartTime: 1},
		{UUID: "b", IMSI: "440100000000001", StartTime: 2},
		{UUID: "c", IMSI: "440100000000005", StartTime: 3},
	}
	s.sortField, s.sortDesc = SortByIMSI, false
	s.sortSessions()
	if got, want := uuids(s.sessions), []string{"b", "c", "a"}; !equal(got, want) {
		t.Errorf("IMSI order = %v, want %v", got, want)
	}
}

func TestSessionListScreen_F6OpensFilter(t *testing.T) {
	app := ui.NewApp()
	s := NewSessionListScreen(app, nil)
	s.render()

	if ev := s.table.GetInputCapture()(tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModNone)); ev != nil {
		t.Error("F6 should be consumed")
	}
	if !app.GetPages().HasPage("filter-dialog") {
		t.Error("F6 should open the filter dialog")
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSortByStartTimeDesc(t *testing.T) {
	got := uuids(sortByStartTimeDesc([]*model.Session{
		{UUID: "b", StartTime: 100},
		{UUID: "c", StartTime: 300},
		{UUID: "a", StartTime: 100},
	}))
	if want := []string{"c", "a", "b"}; !equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

func TestSessionListScreen_NASIDColumn(t *testing.T) {
	s := NewSessionListScreen(ui.NewApp(), nil)
	s.sessions = []*model.Session{
		{UUID: "u1", IMSI: "440100000000001", NasIdentifier: "ap-001", NasIP: "172.30.0.10", StartTime: 300},
		{UUID: "u2", IMSI: "440100000000002", NasIP: "192.168.10.1", StartTime: 200},
		{UUID: "u3", IMSI: "440100000000003", NasIdentifier: "building-a-floor-3-ap-0001", NasIP: "172.30.0.10", StartTime: 100},
	}
	s.render()

	if got := s.table.GetCell(0, 1).Text; got != "NAS-ID" {
		t.Errorf("header[1] = %q, want NAS-ID", got)
	}
	// ソート中の Start Time の表示は列がずれても Start Time の列に付く
	if got := s.table.GetCell(0, 4).Text; got != "Start Time ▼" {
		t.Errorf("header[4] = %q, want %q", got, "Start Time ▼")
	}
	want := []string{"ap-001", "-", "building-a-floor-3-ap..."}
	for i, w := range want {
		if got := s.table.GetCell(i+1, 1).Text; got != w {
			t.Errorf("row %d NAS-ID = %q, want %q", i+1, got, w)
		}
		if got := s.table.GetCell(i+1, 2).Text; got != s.sessions[i].NasIP {
			t.Errorf("row %d NAS IP = %q, want %q", i+1, got, s.sessions[i].NasIP)
		}
	}

	s.ToggleSort() // NAS IP
	if got := s.table.GetCell(0, 2).Text; got != "NAS IP ▲" {
		t.Errorf("header[2] = %q, want %q", got, "NAS IP ▲")
	}
}

func TestSessionListScreen_FilterByNASIdentifier(t *testing.T) {
	s := NewSessionListScreen(ui.NewApp(), nil)
	s.sessions = []*model.Session{
		{UUID: "u1", IMSI: "440100000000001", NasIdentifier: "ap-001", NasIP: "172.30.0.10"},
		{UUID: "u2", IMSI: "440100000000002", NasIdentifier: "ap-002", NasIP: "172.30.0.10"},
		{UUID: "u3", IMSI: "440100000000003", NasIP: "192.168.10.1"},
	}
	s.SetFilter("ap-00")
	if got := uuids(s.getFilteredSessions()); !equal(got, []string{"u1", "u2"}) {
		t.Errorf("filter ap-00 = %v, want [u1 u2]", got)
	}
	s.SetFilter("ap-002")
	if got := uuids(s.getFilteredSessions()); !equal(got, []string{"u2"}) {
		t.Errorf("filter ap-002 = %v, want [u2]", got)
	}
}
