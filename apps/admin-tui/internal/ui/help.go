package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// HelpModal はヘルプモーダルを表示する。
type HelpModal struct {
	modal   *tview.Modal
	onClose func()
}

// HelpSection はヘルプのセクションを表す。
type HelpSection struct {
	Title    string
	Bindings []KeyBinding
}

// NewHelpModal は新しいHelpModalを生成する。
func NewHelpModal(sections []HelpSection, onClose func()) *HelpModal {
	content := ""

	for i, section := range sections {
		if i > 0 {
			content += "\n"
		}
		content += "[::b]" + section.Title + "[::-]\n"
		for _, b := range section.Bindings {
			var keyStr string
			if b.Key != 0 {
				keyStr = keyToString(b.Key)
			} else {
				keyStr = string(b.Rune)
			}
			content += "  " + keyStr + "  " + b.Description + "\n"
		}
	}

	modal := tview.NewModal().
		SetText(content).
		AddButtons([]string{"Close"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if onClose != nil {
				onClose()
			}
		})

	modal.SetTitle(" Help ").
		SetBorder(true).
		SetBorderColor(tcell.ColorTeal)

	return &HelpModal{
		modal:   modal,
		onClose: onClose,
	}
}

// GetModal は内部のtview.Modalを返す。
func (h *HelpModal) GetModal() *tview.Modal {
	return h.modal
}

// GetDefaultHelpSections はデフォルトのヘルプセクションを返す。
func GetDefaultHelpSections() []HelpSection {
	return []HelpSection{
		{
			Title: "Navigation",
			Bindings: []KeyBinding{
				{KeyUp, 0, "Move up"},
				{KeyDown, 0, "Move down"},
				{KeyPageUp, 0, "Page up"},
				{KeyPageDown, 0, "Page down"},
				{KeyTab, 0, "Next field/item"},
				{KeyBacktab, 0, "Previous field/item"},
				{KeyEnter, 0, "Select/Confirm"},
				{KeyEscape, 0, "Back/Cancel"},
			},
		},
		{
			Title: "List Actions",
			Bindings: []KeyBinding{
				{KeyCreate, 0, "Create new"},
				{0, RuneCreate, "Create new (alt)"},
				{KeyEdit, 0, "Edit selected"},
				{0, RuneEdit, "Edit selected (alt)"},
				{KeyDelete, 0, "Delete selected"},
				{0, RuneDelete, "Delete selected (alt)"},
				{KeyRefresh, 0, "Refresh list"},
				{0, RuneRefresh, "Refresh list (alt)"},
				{KeyFilter, 0, "Filter"},
				{0, RuneFilter, "Filter (alt)"},
			},
		},
		{
			Title: "Global",
			Bindings: []KeyBinding{
				{KeyHelp, 0, "Show this help"},
				{0, RuneHelp, "Show this help (alt)"},
				{0, RuneQuit, "Back/Quit"},
				{KeyQuit, 0, "Exit application"},
			},
		},
	}
}

// GetListHelpSections はリスト画面のヘルプセクションを返す。
func GetListHelpSections() []HelpSection {
	return GetDefaultHelpSections()
}

// GetFormHelpSections はフォーム画面のヘルプセクションを返す。
func GetFormHelpSections() []HelpSection {
	return []HelpSection{
		{
			Title: "Form Navigation",
			Bindings: []KeyBinding{
				{KeyTab, 0, "Next field"},
				{KeyBacktab, 0, "Previous field"},
				{KeyEnter, 0, "Submit form / Activate button"},
				{KeyEscape, 0, "Cancel and go back"},
			},
		},
		{
			Title: "Global",
			Bindings: []KeyBinding{
				{KeyHelp, 0, "Show this help"},
				{0, RuneQuit, "Back/Quit"},
				{KeyQuit, 0, "Exit application"},
			},
		},
	}
}
