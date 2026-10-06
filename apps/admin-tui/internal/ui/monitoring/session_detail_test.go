package monitoring

import (
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

func TestSessionDetailScreen_NASIDColumn(t *testing.T) {
	s := NewSessionDetailScreen(ui.NewApp(), nil, nil)
	s.imsi = "440100000000001"
	s.sessions = []*model.Session{
		{UUID: "11111111-aaaa", IMSI: "440100000000001", NasIdentifier: "ap-001", NasIP: "172.30.0.10", ClientIP: "10.0.0.5"},
		{UUID: "22222222-bbbb", IMSI: "440100000000001", NasIP: "192.168.10.1"},
	}
	s.render()

	if got := s.sessionsList.GetCell(0, 1).Text; got != "NAS-ID" {
		t.Errorf("header[1] = %q, want NAS-ID", got)
	}
	if got := s.sessionsList.GetCell(0, 2).Text; got != "NAS IP" {
		t.Errorf("header[2] = %q, want NAS IP", got)
	}
	for i, w := range []string{"ap-001", "-"} {
		if got := s.sessionsList.GetCell(i+1, 1).Text; got != w {
			t.Errorf("row %d NAS-ID = %q, want %q", i+1, got, w)
		}
	}
	if got := s.sessionsList.GetCell(1, 3).Text; got != "10.0.0.5" {
		t.Errorf("row 1 Client IP = %q, want 10.0.0.5", got)
	}
}
