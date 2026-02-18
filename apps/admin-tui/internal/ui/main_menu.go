package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// MenuItem はメニュー項目を表す。
type MenuItem struct {
	Label       string
	Description string
	Key         rune
	Action      func()
}

// MainMenu はメインメニューを表示する。
type MainMenu struct {
	list     *tview.List
	items    []MenuItem
	onSelect func(index int)
	onQuit   func()
}

// NewMainMenu は新しいMainMenuを生成する。
func NewMainMenu(items []MenuItem) *MainMenu {
	list := tview.NewList().
		ShowSecondaryText(true)

	menu := &MainMenu{
		list:  list,
		items: items,
	}

	for i, item := range items {
		idx := i // クロージャ用にコピー
		list.AddItem(item.Label, item.Description, item.Key, func() {
			if menu.onSelect != nil {
				menu.onSelect(idx)
			}
			if items[idx].Action != nil {
				items[idx].Action()
			}
		})
	}

	list.SetTitle(" Admin TUI - Main Menu ").
		SetTitleAlign(tview.AlignCenter).
		SetBorder(true).
		SetBorderColor(tcell.ColorBlue)

	list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Rune() == 'q' {
			if menu.onQuit != nil {
				menu.onQuit()
			}
			return nil
		}
		return event
	})

	return menu
}

// SetOnSelect は項目選択時のコールバックを設定する。
func (m *MainMenu) SetOnSelect(handler func(index int)) {
	m.onSelect = handler
}

// SetOnQuit は終了時のコールバックを設定する。
func (m *MainMenu) SetOnQuit(handler func()) {
	m.onQuit = handler
}

// GetList は内部のtview.Listを返す。
func (m *MainMenu) GetList() *tview.List {
	return m.list
}

// GetDefaultMenuItems はデフォルトのメニュー項目を返す。
func GetDefaultMenuItems() []MenuItem {
	return []MenuItem{
		{
			Label:       "Subscriber Management",
			Description: "Manage subscriber data (IMSI, Ki, OPc, etc.)",
			Key:         '1',
		},
		{
			Label:       "RADIUS Client Management",
			Description: "Manage RADIUS client (NAS) configurations",
			Key:         '2',
		},
		{
			Label:       "Authorization Policy Management",
			Description: "Manage access control policies for subscribers",
			Key:         '3',
		},
		{
			Label:       "Import/Export",
			Description: "Import or export data as CSV files",
			Key:         '4',
		},
		{
			Label:       "Monitoring",
			Description: "View statistics and active sessions",
			Key:         '5',
		},
		{
			Label:       "Exit",
			Description: "Exit the application",
			Key:         'q',
		},
	}
}
