// Package policy は認可ポリシー管理画面を提供する。
package policy

import (
	"context"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/model"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/store"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/rivo/tview"
)

// ListScreen はポリシー一覧画面を表す。
type ListScreen struct {
	table       *tview.Table
	app         *ui.App
	policyStore *store.PolicyStore
	policies    []*model.Policy
	filter      *ui.Filter
	pagination  *ui.Pagination
	onSelect    func(imsi string)
	onCreate    func()
	onEdit      func(imsi string)
	onDelete    func(imsi string)
	onBack      func()
}

// NewListScreen は新しいListScreenを生成する。
func NewListScreen(app *ui.App, policyStore *store.PolicyStore) *ListScreen {
	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetFixed(1, 0)

	table.SetTitle(" Authorization Policy List ").
		SetTitleAlign(tview.AlignCenter).
		SetBorder(true).
		SetBorderColor(tcell.ColorBlue)

	screen := &ListScreen{
		table:       table,
		app:         app,
		policyStore: policyStore,
		filter:      ui.NewFilter("IMSI"),
		pagination:  ui.NewPagination(ui.DefaultPageSize),
	}

	screen.setupKeyBindings()
	return screen
}

// SetOnSelect はポリシー選択時のコールバックを設定する。
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
	policies, err := s.policyStore.List(ctx)
	if err != nil {
		return err
	}

	// IMSIでソート
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].IMSI < policies[j].IMSI
	})

	s.policies = policies
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
	if row < 1 || row > len(s.getFilteredPolicies()) {
		return ""
	}

	filtered := s.getFilteredPolicies()
	pageItems := ui.GetPageItems(filtered, s.pagination)
	idx := row - 1
	if idx < 0 || idx >= len(pageItems) {
		return ""
	}
	return pageItems[idx].IMSI
}

func (s *ListScreen) getFilteredPolicies() []*model.Policy {
	return ui.FilterItems(s.policies, s.filter, func(policy *model.Policy) []string {
		return []string{policy.IMSI}
	})
}

func (s *ListScreen) render() {
	s.table.Clear()

	// ヘッダー
	headers := []string{"IMSI", "Default", "Rules"}
	for col, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1)
		s.table.SetCell(0, col, cell)
	}

	// フィルタ適用
	filtered := s.getFilteredPolicies()
	pageItems := ui.GetPageItems(filtered, s.pagination)

	// データ行
	for i, policy := range pageItems {
		row := i + 1

		// IMSI
		s.table.SetCell(row, 0, tview.NewTableCell(policy.IMSI).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// Default action
		defaultColor := tcell.ColorGreen
		if policy.Default == "allow" {
			defaultColor = tcell.ColorYellow
		}
		s.table.SetCell(row, 1, tview.NewTableCell(policy.Default).
			SetTextColor(defaultColor).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// Rules count
		rulesCount := len(policy.Rules)
		rulesDisplay := "No rules"
		if rulesCount == 1 {
			rulesDisplay = "1 rule"
		} else if rulesCount > 1 {
			rulesDisplay = string(rune('0'+rulesCount)) + " rules"
			if rulesCount >= 10 {
				rulesDisplay = "10+ rules"
			}
		}
		s.table.SetCell(row, 2, tview.NewTableCell(rulesDisplay).
			SetTextColor(tcell.ColorGray).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))
	}

	// タイトル更新
	title := " Authorization Policy List "
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

func (s *ListScreen) showFilterDialog() {
	dialog := ui.NewInputDialog(
		"Filter Policies",
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

// centered is defined in form.go
