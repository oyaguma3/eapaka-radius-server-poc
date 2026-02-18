// Package subscriber は加入者管理画面を提供する。
package subscriber

import (
	"context"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/store"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/rivo/tview"
)

// ListScreen は加入者一覧画面を表す。
type ListScreen struct {
	table           *tview.Table
	app             *ui.App
	subscriberStore *store.SubscriberStore
	policyStore     *store.PolicyStore
	subscribers     []*model.Subscriber
	policyIMSIs     map[string]bool
	filter          *ui.Filter
	pagination      *ui.Pagination
	onSelect        func(imsi string)
	onCreate        func()
	onEdit          func(imsi string)
	onDelete        func(imsi string)
	onBack          func()
}

// NewListScreen は新しいListScreenを生成する。
func NewListScreen(app *ui.App, subscriberStore *store.SubscriberStore, policyStore *store.PolicyStore) *ListScreen {
	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetFixed(1, 0)

	table.SetTitle(" Subscriber List ").
		SetTitleAlign(tview.AlignCenter).
		SetBorder(true).
		SetBorderColor(tcell.ColorBlue)

	screen := &ListScreen{
		table:           table,
		app:             app,
		subscriberStore: subscriberStore,
		policyStore:     policyStore,
		filter:          ui.NewFilter("IMSI"),
		pagination:      ui.NewPagination(ui.DefaultPageSize),
	}

	screen.setupKeyBindings()
	return screen
}

// SetOnSelect は加入者選択時のコールバックを設定する。
func (s *ListScreen) SetOnSelect(handler func(imsi string)) {
	s.onSelect = handler
}

// SetOnCreate は新規作成時のコールバックを設定する。
func (s *ListScreen) SetOnCreate(handler func()) {
	s.onCreate = handler
}

// SetOnEdit は編集時のコールバックを設定する。
func (s *ListScreen) SetOnEdit(handler func(imsi string)) {
	s.onEdit = handler
}

// SetOnDelete は削除時のコールバックを設定する。
func (s *ListScreen) SetOnDelete(handler func(imsi string)) {
	s.onDelete = handler
}

// SetOnBack は戻る時のコールバックを設定する。
func (s *ListScreen) SetOnBack(handler func()) {
	s.onBack = handler
}

// GetTable は内部のtview.Tableを返す。
func (s *ListScreen) GetTable() *tview.Table {
	return s.table
}

// Load はデータを読み込む。
func (s *ListScreen) Load(ctx context.Context) error {
	subscribers, err := s.subscriberStore.List(ctx)
	if err != nil {
		return err
	}

	// IMSIでソート
	sort.Slice(subscribers, func(i, j int) bool {
		return subscribers[i].IMSI < subscribers[j].IMSI
	})

	s.subscribers = subscribers

	// ポリシー設定済みIMSIを取得
	policyIMSIs, err := s.policyStore.GetIMSIsWithPolicy(ctx)
	if err != nil {
		// エラーがあっても続行（ポリシーハイライトなしで表示）
		policyIMSIs = make(map[string]bool)
	}
	s.policyIMSIs = policyIMSIs

	s.render()
	return nil
}

// Refresh はデータを再読み込みする。
func (s *ListScreen) Refresh(ctx context.Context) error {
	return s.Load(ctx)
}

// SetFilter はフィルタを設定する。
func (s *ListScreen) SetFilter(query string) {
	s.filter.SetQuery(query)
	s.pagination.FirstPage()
	s.render()
}

// ClearFilter はフィルタをクリアする。
func (s *ListScreen) ClearFilter() {
	s.filter.Clear()
	s.pagination.FirstPage()
	s.render()
}

// GetSelectedIMSI は選択されているIMSIを返す。
func (s *ListScreen) GetSelectedIMSI() string {
	row, _ := s.table.GetSelection()
	if row < 1 || row > len(s.getFilteredSubscribers()) {
		return ""
	}

	filtered := s.getFilteredSubscribers()
	pageItems := ui.GetPageItems(filtered, s.pagination)
	idx := row - 1
	if idx < 0 || idx >= len(pageItems) {
		return ""
	}
	return pageItems[idx].IMSI
}

func (s *ListScreen) getFilteredSubscribers() []*model.Subscriber {
	return ui.FilterItems(s.subscribers, s.filter, func(sub *model.Subscriber) []string {
		return []string{sub.IMSI}
	})
}

func (s *ListScreen) render() {
	s.table.Clear()

	// ヘッダー
	headers := []string{"", "IMSI", "Ki", "OPc", "AMF", "SQN", "Created"}
	for col, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1)
		if col == 0 {
			cell.SetExpansion(0)
		}
		s.table.SetCell(0, col, cell)
	}

	// フィルタ適用
	filtered := s.getFilteredSubscribers()
	pageItems := ui.GetPageItems(filtered, s.pagination)

	// データ行
	for i, sub := range pageItems {
		row := i + 1

		// ポリシー未設定インジケータ
		indicator := " "
		indicatorColor := tcell.ColorWhite
		if !s.policyIMSIs[sub.IMSI] {
			indicator = "!"
			indicatorColor = tcell.ColorYellow
		}

		// インジケータセル
		s.table.SetCell(row, 0, tview.NewTableCell(indicator).
			SetTextColor(indicatorColor).
			SetAlign(tview.AlignCenter))

		// IMSI
		imsiColor := tcell.ColorWhite
		if !s.policyIMSIs[sub.IMSI] {
			imsiColor = tcell.ColorYellow
		}
		s.table.SetCell(row, 1, tview.NewTableCell(sub.IMSI).
			SetTextColor(imsiColor).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// Ki (マスク表示)
		kiDisplay := sub.Ki[:8] + "..." + sub.Ki[len(sub.Ki)-4:]
		s.table.SetCell(row, 2, tview.NewTableCell(kiDisplay).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// OPc (マスク表示)
		opcDisplay := sub.OPc[:8] + "..." + sub.OPc[len(sub.OPc)-4:]
		s.table.SetCell(row, 3, tview.NewTableCell(opcDisplay).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// AMF
		s.table.SetCell(row, 4, tview.NewTableCell(sub.AMF).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// SQN
		s.table.SetCell(row, 5, tview.NewTableCell(sub.SQN).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// Created
		createdDisplay := sub.CreatedAt
		if len(createdDisplay) > 10 {
			createdDisplay = createdDisplay[:10]
		}
		s.table.SetCell(row, 6, tview.NewTableCell(createdDisplay).
			SetTextColor(tcell.ColorGray).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))
	}

	// タイトル更新
	title := " Subscriber List "
	if s.filter.Active {
		title += "[yellow](" + s.filter.FormatFilterStatus() + ")[-] "
	}
	title += "[gray]" + s.pagination.FormatPageInfo() + "[-] "
	s.table.SetTitle(title)

	// 選択を先頭に
	if len(pageItems) > 0 {
		s.table.Select(1, 0)
	}
}

func (s *ListScreen) setupKeyBindings() {
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
		case tcell.KeyF2:
			if s.onCreate != nil {
				s.onCreate()
			}
			return nil
		case tcell.KeyF3:
			if imsi := s.GetSelectedIMSI(); imsi != "" && s.onEdit != nil {
				s.onEdit(imsi)
			}
			return nil
		case tcell.KeyF4:
			if imsi := s.GetSelectedIMSI(); imsi != "" && s.onDelete != nil {
				s.onDelete(imsi)
			}
			return nil
		case tcell.KeyF5:
			s.app.QueueUpdateDraw(func() {
				if err := s.Refresh(context.Background()); err != nil {
					s.app.GetStatusBar().ShowError("Failed to refresh: " + err.Error())
				} else {
					s.app.GetStatusBar().ShowSuccess("Refreshed")
				}
			})
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
			if imsi := s.GetSelectedIMSI(); imsi != "" && s.onSelect != nil {
				s.onSelect(imsi)
			}
			return nil
		}

		switch event.Rune() {
		case 'n':
			if s.onCreate != nil {
				s.onCreate()
			}
			return nil
		case 'e':
			if imsi := s.GetSelectedIMSI(); imsi != "" && s.onEdit != nil {
				s.onEdit(imsi)
			}
			return nil
		case 'd':
			if imsi := s.GetSelectedIMSI(); imsi != "" && s.onDelete != nil {
				s.onDelete(imsi)
			}
			return nil
		case 'r':
			s.app.QueueUpdateDraw(func() {
				if err := s.Refresh(context.Background()); err != nil {
					s.app.GetStatusBar().ShowError("Failed to refresh: " + err.Error())
				} else {
					s.app.GetStatusBar().ShowSuccess("Refreshed")
				}
			})
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

func (s *ListScreen) showFilterDialog() {
	dialog := ui.NewInputDialog(
		"Filter Subscribers",
		"IMSI contains:",
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

// centered はコンポーネントを中央に配置する。
func centered(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
}
