package monitoring

import (
	"context"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/format"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/store"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/rivo/tview"
)

// SortField はソートフィールドを表す。
type SortField int

const (
	// SortByIMSI はIMSIでソート
	SortByIMSI SortField = iota
	// SortByStartTime は開始時刻でソート
	SortByStartTime
	// SortByNasIP はNAS IPでソート
	SortByNasIP
)

// nasIDMaxLen はセッション一覧・検索結果に表示するNAS-IDの最大文字数（NAS-Identifierは最大253バイト）
const nasIDMaxLen = 24

// SessionListScreen はセッション一覧画面を表す。
type SessionListScreen struct {
	table        *tview.Table
	app          *ui.App
	sessionStore *store.SessionStore
	sessions     []*model.Session
	filter       *ui.Filter
	pagination   *ui.Pagination
	sortField    SortField
	sortDesc     bool
	onSelect     func(uuid string)
	onBack       func()
}

// NewSessionListScreen は新しいSessionListScreenを生成する。
func NewSessionListScreen(app *ui.App, sessionStore *store.SessionStore) *SessionListScreen {
	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetFixed(1, 0)

	table.SetTitle(" Session List ").
		SetTitleAlign(tview.AlignCenter).
		SetBorder(true).
		SetBorderColor(tcell.ColorBlue)

	screen := &SessionListScreen{
		table:        table,
		app:          app,
		sessionStore: sessionStore,
		filter:       ui.NewFilter("IMSI", "NasIdentifier", "NasIP", "ClientIP"),
		pagination:   ui.NewPagination(ui.DefaultPageSize),
		sortField:    SortByStartTime,
		sortDesc:     true, // 最新順
	}

	screen.setupKeyBindings()
	return screen
}

// SetOnSelect はセッション選択時のコールバックを設定する。
func (s *SessionListScreen) SetOnSelect(handler func(uuid string)) {
	s.onSelect = handler
}

// SetOnBack は戻る時のコールバックを設定する。
func (s *SessionListScreen) SetOnBack(handler func()) {
	s.onBack = handler
}

// GetTable は内部のtview.Tableを返す。
func (s *SessionListScreen) GetTable() *tview.Table {
	return s.table
}

// Load はデータを読み込む。
func (s *SessionListScreen) Load(ctx context.Context) error {
	sessions, err := s.sessionStore.List(ctx)
	if err != nil {
		return err
	}

	s.sessions = sessions
	s.sortSessions()
	s.render()
	return nil
}

// Refresh はデータを再読み込みする。
func (s *SessionListScreen) Refresh(ctx context.Context) error {
	return s.Load(ctx)
}

// SetFilter はフィルタを設定する。
func (s *SessionListScreen) SetFilter(query string) {
	s.filter.SetQuery(query)
	s.pagination.FirstPage()
	s.render()
}

// ClearFilter はフィルタをクリアする。
func (s *SessionListScreen) ClearFilter() {
	s.filter.Clear()
	s.pagination.FirstPage()
	s.render()
}

// GetSelectedUUID は選択されているセッションUUIDを返す。
func (s *SessionListScreen) GetSelectedUUID() string {
	row, _ := s.table.GetSelection()
	if row < 1 || row > len(s.getFilteredSessions()) {
		return ""
	}

	filtered := s.getFilteredSessions()
	pageItems := ui.GetPageItems(filtered, s.pagination)
	idx := row - 1
	if idx < 0 || idx >= len(pageItems) {
		return ""
	}
	return pageItems[idx].UUID
}

// ToggleSort はソート項目を Start Time → NAS IP → IMSI の順に切り替える。
// 向きは項目ごとに固定で、Start Time は新しい順（降順）、NAS IP と IMSI は昇順。
func (s *SessionListScreen) ToggleSort() {
	s.sortField = nextSortField(s.sortField)
	s.sortDesc = s.sortField == SortByStartTime
	s.sortSessions()
	s.render()
}

// nextSortField は次のソート項目を返す（Start Time → NAS IP → IMSI → Start Time）。
func nextSortField(f SortField) SortField {
	switch f {
	case SortByStartTime:
		return SortByNasIP
	case SortByNasIP:
		return SortByIMSI
	default:
		return SortByStartTime
	}
}

// sortSessions は現在のソート項目・向きで並べ替える。
// 値が同じ場合は開始時刻の新しい順に並べ、さらに同じなら UUID 順にして表示順を安定させる。
func (s *SessionListScreen) sortSessions() {
	key := func(sess *model.Session) string {
		switch s.sortField {
		case SortByIMSI:
			return sess.IMSI
		case SortByNasIP:
			return sess.NasIP
		default:
			return ""
		}
	}
	sort.SliceStable(s.sessions, func(i, j int) bool {
		a, b := s.sessions[i], s.sessions[j]
		if s.sortField != SortByStartTime {
			if ka, kb := key(a), key(b); ka != kb {
				if s.sortDesc {
					return ka > kb
				}
				return ka < kb
			}
		}
		if a.StartTime != b.StartTime {
			if s.sortField == SortByStartTime && !s.sortDesc {
				return a.StartTime < b.StartTime
			}
			return a.StartTime > b.StartTime
		}
		return a.UUID < b.UUID
	})
}

func (s *SessionListScreen) getFilteredSessions() []*model.Session {
	return ui.FilterItems(s.sessions, s.filter, func(session *model.Session) []string {
		return []string{session.IMSI, session.NasIdentifier, session.NasIP, session.ClientIP}
	})
}

func (s *SessionListScreen) render() {
	s.table.Clear()

	// ヘッダー
	headers := []string{"IMSI", "NAS-ID", "NAS IP", "Client IP", "Start Time", "Duration", "Traffic"}
	sortIndicators := []string{"", "", "", "", "", "", ""}
	switch s.sortField {
	case SortByIMSI:
		if s.sortDesc {
			sortIndicators[0] = " ▼"
		} else {
			sortIndicators[0] = " ▲"
		}
	case SortByStartTime:
		if s.sortDesc {
			sortIndicators[4] = " ▼"
		} else {
			sortIndicators[4] = " ▲"
		}
	case SortByNasIP:
		if s.sortDesc {
			sortIndicators[2] = " ▼"
		} else {
			sortIndicators[2] = " ▲"
		}
	}

	for col, header := range headers {
		cell := tview.NewTableCell(header + sortIndicators[col]).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1)
		s.table.SetCell(0, col, cell)
	}

	// フィルタ適用
	filtered := s.getFilteredSessions()
	pageItems := ui.GetPageItems(filtered, s.pagination)

	// データ行
	for i, session := range pageItems {
		row := i + 1

		// IMSI
		s.table.SetCell(row, 0, tview.NewTableCell(session.IMSI).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// NAS-ID（プロキシ経由ではNAS IPがプロキシのIPになるため、NASの識別に使う）
		s.table.SetCell(row, 1, tview.NewTableCell(format.Truncate(format.OrDash(session.NasIdentifier), nasIDMaxLen)).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// NAS IP
		s.table.SetCell(row, 2, tview.NewTableCell(session.NasIP).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// Client IP
		s.table.SetCell(row, 3, tview.NewTableCell(session.ClientIP).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// Start Time
		s.table.SetCell(row, 4, tview.NewTableCell(format.DateTimeShort(session.StartTime)).
			SetTextColor(tcell.ColorGray).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// Duration
		s.table.SetCell(row, 5, tview.NewTableCell(format.Elapsed(session.StartTime)).
			SetTextColor(tcell.ColorTeal).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// Traffic
		totalTraffic := session.InputOctets + session.OutputOctets
		s.table.SetCell(row, 6, tview.NewTableCell(format.BytesShort(totalTraffic)).
			SetTextColor(tcell.ColorGreen).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))
	}

	// タイトル更新
	title := " Session List "
	if s.filter.Active {
		title += "[yellow](" + s.filter.FormatFilterStatus() + ")[-] "
	}
	title += "[gray]" + s.pagination.FormatPageInfo() + "[-] "
	s.table.SetTitle(title)

	// 選択を先頭に
	if len(pageItems) > 0 {
		s.table.SetSelectable(true, false)
		s.table.Select(1, 0)
	} else {
		s.table.SetSelectable(false, false)
		emptyCell := tview.NewTableCell("(No data)").
			SetTextColor(tcell.ColorGray).
			SetAlign(tview.AlignCenter).
			SetSelectable(false)
		s.table.SetCell(1, 0, emptyCell)
	}
}

func (s *SessionListScreen) setupKeyBindings() {
	s.table.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		switch event.Key() {
		case tcell.KeyEsc:
			if s.filter.Active {
				s.ClearFilter()
				return nil
			}
			if s.onBack != nil {
				s.onBack()
			}
			return nil
		case tcell.KeyF5:
			go func() {
				s.app.QueueUpdateDraw(func() {
					if err := s.Refresh(context.Background()); err != nil {
						s.app.GetStatusBar().ShowError("Failed to refresh: " + err.Error())
					} else {
						s.app.GetStatusBar().ShowSuccess("Refreshed")
					}
				})
			}()
			return nil
		case tcell.KeyF6:
			s.showFilterDialog()
			return nil
		case tcell.KeyPgUp:
			if s.pagination.PrevPage() {
				s.render()
			}
			return nil
		case tcell.KeyPgDn:
			if s.pagination.NextPage() {
				s.render()
			}
			return nil
		case tcell.KeyEnter:
			if uuid := s.GetSelectedUUID(); uuid != "" && s.onSelect != nil {
				s.onSelect(uuid)
			}
			return nil
		}

		switch event.Rune() {
		case 's':
			s.ToggleSort()
			return nil
		case 'r':
			go func() {
				s.app.QueueUpdateDraw(func() {
					if err := s.Refresh(context.Background()); err != nil {
						s.app.GetStatusBar().ShowError("Failed to refresh: " + err.Error())
					} else {
						s.app.GetStatusBar().ShowSuccess("Refreshed")
					}
				})
			}()
			return nil
		case '/':
			s.showFilterDialog()
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

func (s *SessionListScreen) showFilterDialog() {
	dialog := ui.NewInputDialog(
		"Filter Sessions",
		"IMSI/NAS-ID/IP contains:",
		s.filter.Query,
		func(value string) {
			s.SetFilter(value)
			s.app.HidePage("filter-dialog")
			s.app.RemovePage("filter-dialog")
			s.app.SetFocus(s.table)
		},
		func() {
			s.app.HidePage("filter-dialog")
			s.app.RemovePage("filter-dialog")
			s.app.SetFocus(s.table)
		},
	)

	s.app.AddPage("filter-dialog", centered(dialog.GetForm(), 50, 7), true, true)
	s.app.SetFocus(dialog.GetForm())
}

func centered(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
}
