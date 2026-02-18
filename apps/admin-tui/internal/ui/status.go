package ui

import (
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// StatusType はステータスメッセージの種類を表す。
type StatusType int

const (
	// StatusInfo は情報メッセージ
	StatusInfo StatusType = iota
	// StatusSuccess は成功メッセージ
	StatusSuccess
	// StatusWarning は警告メッセージ
	StatusWarning
	// StatusError はエラーメッセージ
	StatusError
)

// StatusBar はステータスバーを管理する。
type StatusBar struct {
	view        *tview.TextView
	app         *tview.Application
	clearTimer  *time.Timer
	defaultText string
}

// NewStatusBar は新しいStatusBarを生成する。
func NewStatusBar() *StatusBar {
	view := tview.NewTextView().
		SetDynamicColors(true).
		SetTextAlign(tview.AlignLeft)

	view.SetBackgroundColor(tcell.ColorDarkBlue)
	view.SetTextColor(tcell.ColorWhite)

	return &StatusBar{
		view:        view,
		defaultText: " F1:Help | q:Back/Quit | Ctrl+Q:Exit",
	}
}

// SetApp はtview.Applicationへの参照を設定する。
func (s *StatusBar) SetApp(app *tview.Application) {
	s.app = app
	s.ShowDefault()
}

// ShowDefault はデフォルトのステータスメッセージを表示する。
func (s *StatusBar) ShowDefault() {
	s.view.SetText(s.defaultText)
}

// SetDefaultText はデフォルトのテキストを設定する。
func (s *StatusBar) SetDefaultText(text string) {
	s.defaultText = text
}

// Show はステータスメッセージを表示する。
func (s *StatusBar) Show(statusType StatusType, message string) {
	s.ShowWithDuration(statusType, message, 5*time.Second)
}

// ShowWithDuration は指定された時間後にデフォルトに戻るステータスメッセージを表示する。
func (s *StatusBar) ShowWithDuration(statusType StatusType, message string, duration time.Duration) {
	// 既存のタイマーをキャンセル
	if s.clearTimer != nil {
		s.clearTimer.Stop()
	}

	// 色付きメッセージを生成
	var coloredMessage string
	switch statusType {
	case StatusSuccess:
		coloredMessage = "[green::b] ✓ " + message + " [-::-]"
	case StatusWarning:
		coloredMessage = "[yellow::b] ⚠ " + message + " [-::-]"
	case StatusError:
		coloredMessage = "[red::b] ✗ " + message + " [-::-]"
	default:
		coloredMessage = "[cyan] ℹ " + message + " [-]"
	}

	s.view.SetText(coloredMessage)

	// duration後にデフォルトに戻す
	if duration > 0 {
		s.clearTimer = time.AfterFunc(duration, func() {
			if s.app != nil {
				s.app.QueueUpdateDraw(func() {
					s.ShowDefault()
				})
			}
		})
	}
}

// ShowInfo は情報メッセージを表示する。
func (s *StatusBar) ShowInfo(message string) {
	s.Show(StatusInfo, message)
}

// ShowSuccess は成功メッセージを表示する。
func (s *StatusBar) ShowSuccess(message string) {
	s.Show(StatusSuccess, message)
}

// ShowWarning は警告メッセージを表示する。
func (s *StatusBar) ShowWarning(message string) {
	s.Show(StatusWarning, message)
}

// ShowError はエラーメッセージを表示する。
func (s *StatusBar) ShowError(message string) {
	s.Show(StatusError, message)
}

// ShowPersistent は自動的に消えないメッセージを表示する。
func (s *StatusBar) ShowPersistent(statusType StatusType, message string) {
	s.ShowWithDuration(statusType, message, 0)
}

// GetView は内部のtview.TextViewを返す。
func (s *StatusBar) GetView() *tview.TextView {
	return s.view
}
