package client

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

func TestListScreen_F6OpensFilter(t *testing.T) {
	app := ui.NewApp()
	s := NewListScreen(app, nil)
	s.render()

	if ev := s.table.GetInputCapture()(tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModNone)); ev != nil {
		t.Error("F6 should be consumed")
	}
	if !app.GetPages().HasPage("filter-dialog") {
		t.Error("F6 should open the filter dialog")
	}
}

// TestListScreen_IDColumn は、一覧の先頭の列にサーバー採番の ID を表示し、ID でも絞り込めることを確認する。
func TestListScreen_IDColumn(t *testing.T) {
	app := ui.NewApp()
	s := NewListScreen(app, nil)
	s.clients = []*model.RadiusClient{
		{ID: 3, IP: "10.0.0.1", Name: "AP-01", Secret: "secret01"},
		{ID: 12, IP: "10.0.0.2", Name: "AP-02", Secret: "secret02"},
	}
	s.render()

	if got := s.table.GetCell(0, 0).Text; got != "ID" {
		t.Errorf("header = %q, want ID", got)
	}
	if got := s.table.GetCell(1, 0).Text; got != "3" {
		t.Errorf("row 1 ID = %q, want 3", got)
	}
	if got := s.table.GetCell(1, 1).Text; got != "10.0.0.1" {
		t.Errorf("row 1 IP = %q", got)
	}

	s.SetFilter("12")
	if got := s.GetSelectedIP(); got != "10.0.0.2" {
		t.Errorf("filtered by ID: selected = %q, want 10.0.0.2", got)
	}
}
