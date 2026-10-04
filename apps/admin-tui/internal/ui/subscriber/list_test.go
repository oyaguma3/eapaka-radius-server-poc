package subscriber

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

// newTestListScreen はストアなしで一覧画面を作り、加入者を1件表示した状態にする。
func newTestListScreen(t *testing.T) (*ListScreen, *ui.App) {
	t.Helper()
	app := ui.NewApp()
	s := NewListScreen(app, nil, nil)
	s.subscribers = []*model.Subscriber{{IMSI: "440101234567890", Ki: "465B5CE8B199B49FAA5F0A2EE238A6BC", OPc: "CD63CB71954A9F4E48A5994E37A02BAF", AMF: "8000", SQN: "000000000020"}}
	s.render()
	s.table.Select(1, 0)
	return s, app
}

func TestListScreen_EnterSelects(t *testing.T) {
	s, _ := newTestListScreen(t)
	var got string
	s.SetOnSelect(func(imsi string) { got = imsi })

	if ev := s.table.GetInputCapture()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone)); ev != nil {
		t.Error("Enter should be consumed")
	}
	if got != "440101234567890" {
		t.Errorf("onSelect called with %q, want %q", got, "440101234567890")
	}
}

func TestListScreen_F6OpensFilter(t *testing.T) {
	s, app := newTestListScreen(t)

	if ev := s.table.GetInputCapture()(tcell.NewEventKey(tcell.KeyF6, 0, tcell.ModNone)); ev != nil {
		t.Error("F6 should be consumed")
	}
	if !app.GetPages().HasPage("filter-dialog") {
		t.Error("F6 should open the filter dialog")
	}
}
