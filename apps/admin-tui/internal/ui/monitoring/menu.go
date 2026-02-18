// Package monitoring はモニタリング画面を提供する。
package monitoring

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// MenuScreen はモニタリングメニュー画面を表す。
type MenuScreen struct {
	list          *tview.List
	onStatistics  func()
	onSessionList func()
	onBack        func()
}

// NewMenuScreen は新しいMenuScreenを生成する。
func NewMenuScreen() *MenuScreen {
	list := tview.NewList().
		ShowSecondaryText(true)

	list.SetTitle(" Monitoring ").
		SetTitleAlign(tview.AlignCenter).
		SetBorder(true).
		SetBorderColor(tcell.ColorBlue)

	screen := &MenuScreen{
		list: list,
	}

	screen.setupMenu()
	return screen
}

// SetOnStatistics は統計ダッシュボード選択時のコールバックを設定する。
func (s *MenuScreen) SetOnStatistics(handler func()) {
	s.onStatistics = handler
}

// SetOnSessionList はセッションリスト選択時のコールバックを設定する。
func (s *MenuScreen) SetOnSessionList(handler func()) {
	s.onSessionList = handler
}

// SetOnBack は戻る時のコールバックを設定する。
func (s *MenuScreen) SetOnBack(handler func()) {
	s.onBack = handler
}

// GetList は内部のtview.Listを返す。
func (s *MenuScreen) GetList() *tview.List {
	return s.list
}

func (s *MenuScreen) setupMenu() {
	s.list.Clear()

	s.list.AddItem("Statistics Dashboard", "View system statistics and counts", '1', func() {
		if s.onStatistics != nil {
			s.onStatistics()
		}
	})

	s.list.AddItem("Session List", "View active sessions", '2', func() {
		if s.onSessionList != nil {
			s.onSessionList()
		}
	})

	s.list.AddItem("Back", "Return to main menu", 'q', func() {
		if s.onBack != nil {
			s.onBack()
		}
	})

	s.list.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Rune() == 'q' {
			if s.onBack != nil {
				s.onBack()
			}
			return nil
		}
		return event
	})
}
