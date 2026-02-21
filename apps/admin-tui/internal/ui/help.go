package ui

import (
	"fmt"

	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// HelpModal はヘルプモーダルを表示する。
type HelpModal struct {
	primitive tview.Primitive
	onClose   func()
}

// HelpSection はヘルプのセクションを表す。
type HelpSection struct {
	Title    string
	Bindings []KeyBinding
}

// formatSection はセクションのテキストを生成する。
func formatSection(section HelpSection) string {
	content := "[yellow::b]" + section.Title + "[::-]\n"
	for _, b := range section.Bindings {
		var keyStr string
		if b.Key != 0 {
			keyStr = keyToString(b.Key)
		} else {
			keyStr = string(b.Rune)
		}
		content += fmt.Sprintf("  [cyan]%-10s[-] %s\n", keyStr, b.Description)
	}
	return content
}

// NewHelpModal は新しいHelpModalを生成する。
func NewHelpModal(sections []HelpSection, onClose func()) *HelpModal {
	// 左カラム: Navigation + Global
	leftContent := ""
	// 右カラム: List Actions
	rightContent := ""

	for _, section := range sections {
		switch section.Title {
		case "List Actions":
			rightContent += formatSection(section)
		default:
			if leftContent != "" {
				leftContent += "\n"
			}
			leftContent += formatSection(section)
		}
	}

	// Policy Form用のF6キーバインド情報を右カラムに追記
	rightContent += "\n[yellow::b]Policy Form[::-]\n"
	rightContent += fmt.Sprintf("  [cyan]%-10s[-] %s\n", "F6", "Toggle Form/Rules focus")

	leftView := tview.NewTextView().
		SetDynamicColors(true).
		SetText(leftContent)

	rightView := tview.NewTextView().
		SetDynamicColors(true).
		SetText(rightContent)

	columns := tview.NewFlex().
		AddItem(leftView, 0, 1, false).
		AddItem(rightView, 0, 1, false)

	columns.SetTitle(" Help ").
		SetTitleAlign(tview.AlignCenter).
		SetBorder(true).
		SetBorderColor(tcell.ColorTeal)

	columns.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyEnter {
			if onClose != nil {
				onClose()
			}
			return nil
		}
		return event
	})

	// centered でモーダル風に表示（幅80, 高さ20）
	wrapper := tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(columns, 20, 1, true).
			AddItem(nil, 0, 1, false), 80, 1, true).
		AddItem(nil, 0, 1, false)

	return &HelpModal{
		primitive: wrapper,
		onClose:   onClose,
	}
}

// GetPrimitive は内部のtview.Primitiveを返す。
func (h *HelpModal) GetPrimitive() tview.Primitive {
	return h.primitive
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
