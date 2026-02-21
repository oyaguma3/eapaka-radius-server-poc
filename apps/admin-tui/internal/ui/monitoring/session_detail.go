package monitoring

import (
	"context"
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/format"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/store"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/rivo/tview"
)

// SessionDetailScreen はセッション詳細画面を表す。
type SessionDetailScreen struct {
	flex         *tview.Flex
	textView     *tview.TextView
	sessionsList *tview.Table
	app          *ui.App
	sessionStore *store.SessionStore
	auditLogger  *audit.Logger
	imsi         string
	sessions     []*model.Session
	onBack       func()
}

// NewSessionDetailScreen は新しいSessionDetailScreenを生成する。
func NewSessionDetailScreen(app *ui.App, sessionStore *store.SessionStore, auditLogger *audit.Logger) *SessionDetailScreen {
	textView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	textView.SetBorder(true).
		SetTitle(" Session Search ").
		SetBorderColor(tcell.ColorBlue)

	sessionsList := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetFixed(1, 0)
	sessionsList.SetBorder(true).
		SetTitle(" Sessions ").
		SetBorderColor(tcell.ColorGray)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(textView, 8, 0, true).
		AddItem(sessionsList, 0, 1, false)

	screen := &SessionDetailScreen{
		flex:         flex,
		textView:     textView,
		sessionsList: sessionsList,
		app:          app,
		sessionStore: sessionStore,
		auditLogger:  auditLogger,
	}

	screen.setupKeyBindings()
	return screen
}

// SetOnBack は戻る時のコールバックを設定する。
func (s *SessionDetailScreen) SetOnBack(handler func()) {
	s.onBack = handler
}

// GetFlex は内部のtview.Flexを返す。
func (s *SessionDetailScreen) GetFlex() *tview.Flex {
	return s.flex
}

// ShowSearchDialog は検索ダイアログを表示する。
func (s *SessionDetailScreen) ShowSearchDialog() {
	dialog := ui.NewInputDialog(
		"Search Sessions by IMSI",
		"Enter IMSI:",
		s.imsi,
		func(value string) {
			s.imsi = value
			s.app.HidePage("search-dialog")
			s.app.RemovePage("search-dialog")
			go func() {
				ctx := context.Background()
				s.auditLogger.LogSearch(audit.TargetSession, value, 0)
				sessions, err := s.sessionStore.GetByIMSI(ctx, value)
				s.app.QueueUpdateDraw(func() {
					if err != nil {
						s.app.GetStatusBar().ShowError("Search failed: " + err.Error())
					} else {
						s.sessions = sessions
						s.render()
					}
					s.app.SetFocus(s.sessionsList)
				})
			}()
		},
		func() {
			s.app.HidePage("search-dialog")
			s.app.RemovePage("search-dialog")
			if s.imsi == "" && s.onBack != nil {
				s.onBack()
			} else {
				s.app.SetFocus(s.textView)
			}
		},
	)

	s.app.AddPage("search-dialog", centeredDetail(dialog.GetForm(), 50, 7), true, true)
	s.app.SetFocus(dialog.GetForm())
}

// Search は指定されたIMSIのセッションを検索する。
func (s *SessionDetailScreen) Search(ctx context.Context, imsi string) error {
	s.imsi = imsi

	// 監査ログに検索を記録
	s.auditLogger.LogSearch(audit.TargetSession, imsi, 0)

	sessions, err := s.sessionStore.GetByIMSI(ctx, imsi)
	if err != nil {
		s.textView.SetText(fmt.Sprintf("[red]Error: %s[-]", err.Error()))
		return err
	}

	s.sessions = sessions
	s.render()
	return nil
}

func (s *SessionDetailScreen) render() {
	// テキストビュー更新
	var content string
	content += fmt.Sprintf("[yellow]IMSI:[-] %s\n", s.imsi)
	content += fmt.Sprintf("[cyan]Sessions found:[-] %d\n", len(s.sessions))
	content += "\n[gray]Press '/' to search for another IMSI[-]"

	s.textView.SetText(content)

	// セッションリスト更新
	s.sessionsList.Clear()

	// ヘッダー
	headers := []string{"UUID", "NAS IP", "Client IP", "Start Time", "Duration", "In/Out"}
	for col, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1)
		s.sessionsList.SetCell(0, col, cell)
	}

	if len(s.sessions) == 0 {
		s.sessionsList.SetSelectable(false, false)
		s.sessionsList.SetCell(1, 0, tview.NewTableCell("No sessions found").
			SetTextColor(tcell.ColorGray).
			SetSelectable(false))
		return
	}

	s.sessionsList.SetSelectable(true, false)

	// データ行
	for i, session := range s.sessions {
		row := i + 1

		// UUID (短縮)
		uuidDisplay := session.UUID
		if len(uuidDisplay) > 8 {
			uuidDisplay = uuidDisplay[:8] + "..."
		}
		s.sessionsList.SetCell(row, 0, tview.NewTableCell(uuidDisplay).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// NAS IP
		s.sessionsList.SetCell(row, 1, tview.NewTableCell(session.NasIP).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// Client IP
		s.sessionsList.SetCell(row, 2, tview.NewTableCell(session.ClientIP).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// Start Time
		s.sessionsList.SetCell(row, 3, tview.NewTableCell(format.DateTimeShort(session.StartTime)).
			SetTextColor(tcell.ColorGray).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// Duration
		s.sessionsList.SetCell(row, 4, tview.NewTableCell(format.Elapsed(session.StartTime)).
			SetTextColor(tcell.ColorTeal).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// In/Out
		trafficDisplay := fmt.Sprintf("%s/%s",
			format.BytesShort(session.InputOctets),
			format.BytesShort(session.OutputOctets))
		s.sessionsList.SetCell(row, 5, tview.NewTableCell(trafficDisplay).
			SetTextColor(tcell.ColorGreen).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))
	}

	// 選択を先頭に
	if len(s.sessions) > 0 {
		s.sessionsList.Select(1, 0)
	}
}

func (s *SessionDetailScreen) setupKeyBindings() {
	s.textView.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			if s.onBack != nil {
				s.onBack()
			}
			return nil
		case tcell.KeyTab:
			s.app.SetFocus(s.sessionsList)
			return nil
		}

		switch event.Rune() {
		case '/':
			s.ShowSearchDialog()
			return nil
		case 'q':
			if s.onBack != nil {
				s.onBack()
			}
			return nil
		}

		return event
	})

	s.sessionsList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc, tcell.KeyTab:
			s.app.SetFocus(s.textView)
			return nil
		}

		switch event.Rune() {
		case '/':
			s.ShowSearchDialog()
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

func centeredDetail(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
}
