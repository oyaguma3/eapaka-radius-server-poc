package policy

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/rivo/tview"
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

func TestListScreen_StatusColumnAndToggle(t *testing.T) {
	app := ui.NewApp()
	s := NewListScreen(app, nil)
	s.policies = []*model.Policy{
		{IMSI: "001010000000001", Default: "deny", Status: model.PolicyStatusActive},
		{IMSI: "001010000000002", Default: "deny", Status: model.PolicyStatusSuspended},
	}
	s.render()

	if got := s.table.GetCell(0, 3).Text; got != "Status" {
		t.Errorf("header[3] = %q, want Status", got)
	}
	if c := s.table.GetCell(2, 3); c.Text != "suspended" {
		t.Errorf("row 2 status = %q, want suspended", c.Text)
	} else if fg, _, _ := c.Style.Decompose(); fg != tcell.ColorRed {
		t.Errorf("row 2 status color = %v, want red", fg)
	}

	var got []string
	s.SetOnToggleStatus(func(p *model.Policy) { got = append(got, p.IMSI+" "+p.Status) })

	// F7 は選択中（先頭）の行、s は選択を移した行
	if ev := s.table.GetInputCapture()(tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone)); ev != nil {
		t.Error("F7 should be consumed")
	}
	s.SelectIMSI("001010000000002")
	if ev := s.table.GetInputCapture()(tcell.NewEventKey(tcell.KeyRune, 's', tcell.ModNone)); ev != nil {
		t.Error("s should be consumed")
	}
	want := []string{"001010000000001 active", "001010000000002 suspended"}
	if len(got) != 2 || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("toggled = %v, want %v", got, want)
	}

	// 選択がなければ呼ばない
	s.policies = nil
	s.render()
	s.table.GetInputCapture()(tcell.NewEventKey(tcell.KeyF7, 0, tcell.ModNone))
	if len(got) != 2 {
		t.Errorf("toggled without selection: %v", got)
	}
}

func TestFormScreen_StatusIsReadOnly(t *testing.T) {
	app := ui.NewApp()
	s := NewFormScreen(app, nil, nil)
	s.editMode = true
	s.policy = &model.Policy{IMSI: "001010000000001", Default: "deny", Status: model.PolicyStatusSuspended}
	s.setupForm()

	field, ok := s.form.GetFormItemByLabel("Status").(*tview.InputField)
	if !ok {
		t.Fatal("Status field not found")
	}
	if field.GetText() != "suspended" {
		t.Errorf("Status = %q, want suspended", field.GetText())
	}
	// 読み取り専用なので、キー入力で値が変わらない
	field.InputHandler()(tcell.NewEventKey(tcell.KeyRune, 'x', tcell.ModNone), func(tview.Primitive) {})
	if field.GetText() != "suspended" {
		t.Errorf("Status changed by input: %q", field.GetText())
	}

	// 新規作成では active を表示する
	s.SetupCreate()
	if got := s.form.GetFormItemByLabel("Status").(*tview.InputField).GetText(); got != "active" {
		t.Errorf("create Status = %q, want active", got)
	}
}
