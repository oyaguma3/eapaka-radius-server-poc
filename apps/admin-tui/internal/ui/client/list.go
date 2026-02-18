// Package client はRADIUSクライアント管理画面を提供する。
package client

import (
	"context"
	"sort"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/store"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/rivo/tview"
)

// ListScreen はRADIUSクライアント一覧画面を表す。
type ListScreen struct {
	table       *tview.Table
	app         *ui.App
	clientStore *store.ClientStore
	clients     []*model.RadiusClient
	filter      *ui.Filter
	pagination  *ui.Pagination
	onSelect    func(ip string)
	onCreate    func()
	onEdit      func(ip string)
	onDelete    func(ip string)
	onBack      func()
}

// NewListScreen は新しいListScreenを生成する。
func NewListScreen(app *ui.App, clientStore *store.ClientStore) *ListScreen {
	table := tview.NewTable().
		SetBorders(false).
		SetSelectable(true, false).
		SetFixed(1, 0)

	table.SetTitle(" RADIUS Client List ").
		SetTitleAlign(tview.AlignCenter).
		SetBorder(true).
		SetBorderColor(tcell.ColorBlue)

	screen := &ListScreen{
		table:       table,
		app:         app,
		clientStore: clientStore,
		filter:      ui.NewFilter("IP", "Name", "Vendor"),
		pagination:  ui.NewPagination(ui.DefaultPageSize),
	}

	screen.setupKeyBindings()
	return screen
}

// SetOnSelect はクライアント選択時のコールバックを設定する。
func (s *ListScreen) SetOnSelect(handler func(ip string)) {
	s.onSelect = handler
}

// SetOnCreate は新規作成時のコールバックを設定する。
func (s *ListScreen) SetOnCreate(handler func()) {
	s.onCreate = handler
}

// SetOnEdit は編集時のコールバックを設定する。
func (s *ListScreen) SetOnEdit(handler func(ip string)) {
	s.onEdit = handler
}

// SetOnDelete は削除時のコールバックを設定する。
func (s *ListScreen) SetOnDelete(handler func(ip string)) {
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
	clients, err := s.clientStore.List(ctx)
	if err != nil {
		return err
	}

	// IPでソート
	sort.Slice(clients, func(i, j int) bool {
		return clients[i].IP < clients[j].IP
	})

	s.clients = clients
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

// GetSelectedIP は選択されているIPを返す。
func (s *ListScreen) GetSelectedIP() string {
	row, _ := s.table.GetSelection()
	if row < 1 || row > len(s.getFilteredClients()) {
		return ""
	}

	filtered := s.getFilteredClients()
	pageItems := ui.GetPageItems(filtered, s.pagination)
	idx := row - 1
	if idx < 0 || idx >= len(pageItems) {
		return ""
	}
	return pageItems[idx].IP
}

func (s *ListScreen) getFilteredClients() []*model.RadiusClient {
	return ui.FilterItems(s.clients, s.filter, func(client *model.RadiusClient) []string {
		return []string{client.IP, client.Name, client.Vendor}
	})
}

func (s *ListScreen) render() {
	s.table.Clear()

	// ヘッダー
	headers := []string{"IP Address", "Name", "Secret", "Vendor"}
	for col, header := range headers {
		cell := tview.NewTableCell(header).
			SetTextColor(tcell.ColorYellow).
			SetAlign(tview.AlignLeft).
			SetSelectable(false).
			SetExpansion(1)
		s.table.SetCell(0, col, cell)
	}

	// フィルタ適用
	filtered := s.getFilteredClients()
	pageItems := ui.GetPageItems(filtered, s.pagination)

	// データ行
	for i, client := range pageItems {
		row := i + 1

		// IP Address
		s.table.SetCell(row, 0, tview.NewTableCell(client.IP).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// Name
		s.table.SetCell(row, 1, tview.NewTableCell(client.Name).
			SetTextColor(tcell.ColorWhite).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// Secret (masked)
		secretDisplay := "****"
		if len(client.Secret) > 4 {
			secretDisplay = client.Secret[:2] + "****" + client.Secret[len(client.Secret)-2:]
		}
		s.table.SetCell(row, 2, tview.NewTableCell(secretDisplay).
			SetTextColor(tcell.ColorGray).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))

		// Vendor
		vendor := client.Vendor
		if vendor == "" {
			vendor = "-"
		}
		s.table.SetCell(row, 3, tview.NewTableCell(vendor).
			SetTextColor(tcell.ColorGray).
			SetAlign(tview.AlignLeft).
			SetExpansion(1))
	}

	// タイトル更新
	title := " RADIUS Client List "
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
			if ip := s.GetSelectedIP(); ip != "" && s.onEdit != nil {
				s.onEdit(ip)
			}
			return nil
		case tcell.KeyF4:
			if ip := s.GetSelectedIP(); ip != "" && s.onDelete != nil {
				s.onDelete(ip)
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
			if ip := s.GetSelectedIP(); ip != "" && s.onSelect != nil {
				s.onSelect(ip)
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
			if ip := s.GetSelectedIP(); ip != "" && s.onEdit != nil {
				s.onEdit(ip)
			}
			return nil
		case 'd':
			if ip := s.GetSelectedIP(); ip != "" && s.onDelete != nil {
				s.onDelete(ip)
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
		"Filter Clients",
		"IP/Name/Vendor contains:",
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
