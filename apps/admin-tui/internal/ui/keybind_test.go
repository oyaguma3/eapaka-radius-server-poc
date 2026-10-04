package ui

import (
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

func TestIsTextInput(t *testing.T) {
	tests := []struct {
		name string
		p    tview.Primitive
		want bool
	}{
		{"input field", tview.NewInputField(), true},
		{"text area", tview.NewTextArea(), true},
		{"table", tview.NewTable(), false},
		{"list", tview.NewList(), false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTextInput(tt.p); got != tt.want {
				t.Errorf("IsTextInput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestInputDialog_EscCancels(t *testing.T) {
	canceled := false
	d := NewInputDialog("Filter", "contains:", "", nil, func() { canceled = true })

	// フォーム内の入力欄で Esc を押すと Cancel と同じ処理が呼ばれる
	form := d.GetForm()
	// 実際の画面と同じくフォームにフォーカスを与える（入力欄の終了処理がここで組み込まれる）
	form.Focus(func(p tview.Primitive) { p.Focus(func(tview.Primitive) {}) })
	handler := form.InputHandler()
	handler(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone), func(p tview.Primitive) {})

	if !canceled {
		t.Error("Esc should call onCancel")
	}
}
