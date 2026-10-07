package ui_test

import (
	"strings"
	"testing"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui/client"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui/subscriber"
	"github.com/rivo/tview"
)

func TestFormHeight(t *testing.T) {
	// 枠2 + 余白2 + 入力欄ごとに2行 + ボタン1行
	for n, want := range map[int]int{0: 5, 4: 13, 5: 15} {
		if got := ui.FormHeight(n); got != want {
			t.Errorf("FormHeight(%d) = %d, want %d", n, got, want)
		}
	}
}

// drawForm はフォームを幅 width・高さ height で仮想画面に描画し、画面の各行の文字列を返す。
func drawForm(t *testing.T, form *tview.Form, width, height int) []string {
	t.Helper()
	screen := tcell.NewSimulationScreen("UTF-8")
	if err := screen.Init(); err != nil {
		t.Fatal(err)
	}
	defer screen.Fini()
	screen.SetSize(width, height)

	form.SetRect(0, 0, width, height)
	form.Draw(screen)
	screen.Show()

	cells, w, h := screen.GetContents()
	lines := make([]string, h)
	for y := range h {
		var b strings.Builder
		for x := range w {
			if r := cells[y*w+x].Runes; len(r) > 0 {
				b.WriteRune(r[0])
			} else {
				b.WriteRune(' ')
			}
		}
		lines[y] = b.String()
	}
	return lines
}

// hasButtons は Save / Cancel ボタンの行が表示されているかを返す。
func hasButtons(lines []string) bool {
	for _, l := range lines {
		if strings.Contains(l, "Save") && strings.Contains(l, "Cancel") {
			return true
		}
	}
	return false
}

// TestForms_ButtonsVisible は、main.go と同じ幅 60・FormHeight の高さで、
// 加入者・RADIUSクライアントの登録画面の Save / Cancel ボタンが枠内に表示されることを確認する。
func TestForms_ButtonsVisible(t *testing.T) {
	app := ui.NewApp()

	clientForm := client.NewFormScreen(app, nil, nil)
	clientForm.SetupCreate()
	subscriberForm := subscriber.NewFormScreen(app, nil, nil)
	subscriberForm.SetupCreate()

	for name, form := range map[string]*tview.Form{"client": clientForm.GetForm(), "subscriber": subscriberForm.GetForm()} {
		t.Run(name, func(t *testing.T) {
			height := ui.FormHeight(form.GetFormItemCount())
			if lines := drawForm(t, form, 60, height); !hasButtons(lines) {
				t.Errorf("buttons are not visible at height %d:\n%s", height, strings.Join(lines, "\n"))
			}
			// 1行でも足りなければボタンは表示されない（修正前のクライアント画面は高さ 12 だった）
			if lines := drawForm(t, form, 60, height-1); hasButtons(lines) {
				t.Errorf("buttons are visible at height %d; FormHeight is larger than needed", height-1)
			}
		})
	}
}
