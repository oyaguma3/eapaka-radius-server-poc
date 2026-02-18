package ui

import "github.com/gdamore/tcell/v2"

// キーバインド定義
var (
	// Navigation keys
	KeyUp       = tcell.KeyUp
	KeyDown     = tcell.KeyDown
	KeyLeft     = tcell.KeyLeft
	KeyRight    = tcell.KeyRight
	KeyPageUp   = tcell.KeyPgUp
	KeyPageDown = tcell.KeyPgDn
	KeyHome     = tcell.KeyHome
	KeyEnd      = tcell.KeyEnd
	KeyTab      = tcell.KeyTab
	KeyBacktab  = tcell.KeyBacktab
	KeyEnter    = tcell.KeyEnter
	KeyEscape   = tcell.KeyEsc

	// Action keys
	KeyCreate  = tcell.KeyF2
	KeyEdit    = tcell.KeyF3
	KeyDelete  = tcell.KeyF4
	KeyRefresh = tcell.KeyF5
	KeyFilter  = tcell.KeyF6
	KeyHelp    = tcell.KeyF1
	KeyQuit    = tcell.KeyCtrlQ
)

// Rune keys
const (
	RuneCreate  = 'n'
	RuneEdit    = 'e'
	RuneDelete  = 'd'
	RuneRefresh = 'r'
	RuneFilter  = '/'
	RuneHelp    = '?'
	RuneQuit    = 'q'
	RuneYes     = 'y'
	RuneNo      = 'n'
	RuneConfirm = 'Y'
)

// KeyBinding はキーバインドの情報を表す。
type KeyBinding struct {
	Key         tcell.Key
	Rune        rune
	Description string
}

// GetGlobalKeyBindings はグローバルキーバインドのリストを返す。
func GetGlobalKeyBindings() []KeyBinding {
	return []KeyBinding{
		{KeyHelp, 0, "Help"},
		{KeyQuit, 0, "Quit"},
		{0, RuneQuit, "Back/Quit"},
	}
}

// GetListKeyBindings はリスト画面のキーバインドのリストを返す。
func GetListKeyBindings() []KeyBinding {
	return []KeyBinding{
		{KeyUp, 0, "Move up"},
		{KeyDown, 0, "Move down"},
		{KeyPageUp, 0, "Page up"},
		{KeyPageDown, 0, "Page down"},
		{KeyEnter, 0, "Select/View"},
		{KeyCreate, 0, "Create new"},
		{0, RuneCreate, "Create new"},
		{KeyEdit, 0, "Edit"},
		{0, RuneEdit, "Edit"},
		{KeyDelete, 0, "Delete"},
		{0, RuneDelete, "Delete"},
		{KeyRefresh, 0, "Refresh"},
		{0, RuneRefresh, "Refresh"},
		{KeyFilter, 0, "Filter"},
		{0, RuneFilter, "Filter"},
	}
}

// GetFormKeyBindings はフォーム画面のキーバインドのリストを返す。
func GetFormKeyBindings() []KeyBinding {
	return []KeyBinding{
		{KeyTab, 0, "Next field"},
		{KeyBacktab, 0, "Previous field"},
		{KeyEnter, 0, "Submit/Select"},
		{KeyEscape, 0, "Cancel"},
	}
}

// GetDialogKeyBindings はダイアログのキーバインドのリストを返す。
func GetDialogKeyBindings() []KeyBinding {
	return []KeyBinding{
		{KeyEnter, 0, "Confirm"},
		{KeyEscape, 0, "Cancel"},
		{0, RuneYes, "Yes"},
		{0, RuneNo, "No"},
	}
}

// FormatKeyBindingHint はキーバインドのヒント文字列を生成する。
func FormatKeyBindingHint(bindings []KeyBinding) string {
	hints := ""
	for i, b := range bindings {
		if i > 0 {
			hints += " | "
		}
		if b.Key != 0 {
			hints += keyToString(b.Key) + ": " + b.Description
		} else {
			hints += string(b.Rune) + ": " + b.Description
		}
	}
	return hints
}

// keyToString はキーコードを文字列に変換する。
func keyToString(key tcell.Key) string {
	switch key {
	case tcell.KeyF1:
		return "F1"
	case tcell.KeyF2:
		return "F2"
	case tcell.KeyF3:
		return "F3"
	case tcell.KeyF4:
		return "F4"
	case tcell.KeyF5:
		return "F5"
	case tcell.KeyF6:
		return "F6"
	case tcell.KeyUp:
		return "↑"
	case tcell.KeyDown:
		return "↓"
	case tcell.KeyLeft:
		return "←"
	case tcell.KeyRight:
		return "→"
	case tcell.KeyPgUp:
		return "PgUp"
	case tcell.KeyPgDn:
		return "PgDn"
	case tcell.KeyHome:
		return "Home"
	case tcell.KeyEnd:
		return "End"
	case tcell.KeyTab:
		return "Tab"
	case tcell.KeyBacktab:
		return "Shift+Tab"
	case tcell.KeyEnter:
		return "Enter"
	case tcell.KeyEsc:
		return "Esc"
	case tcell.KeyCtrlQ:
		return "Ctrl+Q"
	default:
		return "?"
	}
}
