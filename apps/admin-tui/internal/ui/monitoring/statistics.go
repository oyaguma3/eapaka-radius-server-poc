package monitoring

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/format"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/store"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/rivo/tview"
)

// StatisticsScreen は統計ダッシュボード画面を表す。
type StatisticsScreen struct {
	flex            *tview.Flex
	textView        *tview.TextView
	app             *ui.App
	statisticsStore *store.StatisticsStore
	onBack          func()
}

// NewStatisticsScreen は新しいStatisticsScreenを生成する。
func NewStatisticsScreen(app *ui.App, statisticsStore *store.StatisticsStore) *StatisticsScreen {
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)

	textView.SetTitle(" Statistics Dashboard ").
		SetTitleAlign(tview.AlignCenter).
		SetBorder(true).
		SetBorderColor(tcell.ColorBlue)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(textView, 0, 1, true)

	screen := &StatisticsScreen{
		flex:            flex,
		textView:        textView,
		app:             app,
		statisticsStore: statisticsStore,
	}

	screen.setupKeyBindings()
	return screen
}

// SetOnBack は戻る時のコールバックを設定する。
func (s *StatisticsScreen) SetOnBack(handler func()) {
	s.onBack = handler
}

// GetFlex は内部のtview.Flexを返す。
func (s *StatisticsScreen) GetFlex() *tview.Flex {
	return s.flex
}

// Load はデータを読み込む。
func (s *StatisticsScreen) Load(ctx context.Context) error {
	stats, err := s.statisticsStore.Get(ctx)
	if err != nil {
		s.textView.SetText("[red]Error loading statistics: " + err.Error() + "[-]")
		return err
	}

	s.render(stats)
	return nil
}

// Refresh はデータを再読み込みする（キャッシュをクリア）。
func (s *StatisticsScreen) Refresh(ctx context.Context) error {
	s.statisticsStore.ClearCache()
	return s.Load(ctx)
}

func (s *StatisticsScreen) render(stats *store.Statistics) {
	var content string

	content += "[yellow::b]System Statistics[-::-]\n\n"

	content += fmt.Sprintf("  [cyan]Subscribers:[-]    %d\n", stats.SubscriberCount)
	content += fmt.Sprintf("  [cyan]RADIUS Clients:[-] %d\n", stats.ClientCount)
	content += fmt.Sprintf("  [cyan]Policies:[-]       %d\n", stats.PolicyCount)
	content += fmt.Sprintf("  [cyan]Active Sessions:[-] %d\n", stats.SessionCount)

	content += "\n"
	content += "[gray]Last updated: " + format.DateTime(stats.UpdatedAt) + "[-]\n"
	content += "[gray](Statistics are cached for 1 minute. Press 'r' to force refresh)[-]\n"

	content += "\n\n"
	content += "[yellow]Key bindings:[-]\n"
	content += "  r - Refresh statistics\n"
	content += "  q/Esc - Back to menu\n"

	s.textView.SetText(content)
}

func (s *StatisticsScreen) setupKeyBindings() {
	s.textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			if s.onBack != nil {
				s.onBack()
			}
			return nil
		}

		switch event.Rune() {
		case 'r':
			s.app.QueueUpdateDraw(func() {
				if err := s.Refresh(context.Background()); err != nil {
					s.app.GetStatusBar().ShowError("Failed to refresh: " + err.Error())
				} else {
					s.app.GetStatusBar().ShowSuccess("Statistics refreshed")
				}
			})
			return nil
		case 'q':
			if s.onBack != nil {
				s.onBack()
			}
			return nil
		}

		return event
	})
}
