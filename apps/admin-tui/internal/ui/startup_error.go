package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// StartupErrorScreen は起動エラー画面を表す。
type StartupErrorScreen struct {
	modal   *tview.Modal
	onRetry func()
	onExit  func()
}

// NewStartupErrorScreen は新しいStartupErrorScreenを生成する。
func NewStartupErrorScreen(errorMessage string, onRetry, onExit func()) *StartupErrorScreen {
	modal := tview.NewModal().
		SetText("Failed to connect to Valkey:\n\n" + errorMessage + "\n\nPlease check:\n- Valkey is running on 127.0.0.1:6379\n- VALKEY_PASSWORD environment variable is set correctly").
		AddButtons([]string{"Retry", "Exit"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Retry" {
				if onRetry != nil {
					onRetry()
				}
			} else {
				if onExit != nil {
					onExit()
				}
			}
		})

	modal.SetTitle(" Connection Error ").
		SetBorder(true).
		SetBorderColor(tcell.ColorRed)
	modal.SetBackgroundColor(tcell.ColorBlack)

	return &StartupErrorScreen{
		modal:   modal,
		onRetry: onRetry,
		onExit:  onExit,
	}
}

// GetModal は内部のtview.Modalを返す。
func (s *StartupErrorScreen) GetModal() *tview.Modal {
	return s.modal
}
