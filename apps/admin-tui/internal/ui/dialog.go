package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// ConfirmDialog は確認ダイアログを表示する。
type ConfirmDialog struct {
	modal     *tview.Modal
	onConfirm func()
	onCancel  func()
}

// NewConfirmDialog は新しいConfirmDialogを生成する。
func NewConfirmDialog(title, message string, onConfirm, onCancel func()) *ConfirmDialog {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"Yes", "No"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Yes" {
				if onConfirm != nil {
					onConfirm()
				}
			} else {
				if onCancel != nil {
					onCancel()
				}
			}
		})

	modal.SetTitle(" " + title + " ").
		SetBorder(true).
		SetBorderColor(tcell.ColorWhite)

	return &ConfirmDialog{
		modal:     modal,
		onConfirm: onConfirm,
		onCancel:  onCancel,
	}
}

// GetModal は内部のtview.Modalを返す。
func (d *ConfirmDialog) GetModal() *tview.Modal {
	return d.modal
}

// WarningDialog は警告ダイアログを表示する。
type WarningDialog struct {
	modal     *tview.Modal
	onConfirm func()
	onCancel  func()
}

// NewWarningDialog は新しいWarningDialogを生成する。
func NewWarningDialog(title, message string, onConfirm, onCancel func()) *WarningDialog {
	modal := tview.NewModal().
		SetText("⚠ WARNING ⚠\n\n" + message).
		AddButtons([]string{"Continue", "Cancel"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if buttonLabel == "Continue" {
				if onConfirm != nil {
					onConfirm()
				}
			} else {
				if onCancel != nil {
					onCancel()
				}
			}
		})

	modal.SetTitle(" " + title + " ").
		SetBorder(true).
		SetBorderColor(tcell.ColorYellow)
	modal.SetBackgroundColor(tcell.ColorBlack)

	return &WarningDialog{
		modal:     modal,
		onConfirm: onConfirm,
		onCancel:  onCancel,
	}
}

// GetModal は内部のtview.Modalを返す。
func (d *WarningDialog) GetModal() *tview.Modal {
	return d.modal
}

// InfoDialog は情報ダイアログを表示する。
type InfoDialog struct {
	modal   *tview.Modal
	onClose func()
}

// NewInfoDialog は新しいInfoDialogを生成する。
func NewInfoDialog(title, message string, onClose func()) *InfoDialog {
	modal := tview.NewModal().
		SetText(message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if onClose != nil {
				onClose()
			}
		})

	modal.SetTitle(" " + title + " ").
		SetBorder(true).
		SetBorderColor(tcell.ColorTeal)

	return &InfoDialog{
		modal:   modal,
		onClose: onClose,
	}
}

// GetModal は内部のtview.Modalを返す。
func (d *InfoDialog) GetModal() *tview.Modal {
	return d.modal
}

// ErrorDialog はエラーダイアログを表示する。
type ErrorDialog struct {
	modal   *tview.Modal
	onClose func()
}

// NewErrorDialog は新しいErrorDialogを生成する。
func NewErrorDialog(title, message string, onClose func()) *ErrorDialog {
	modal := tview.NewModal().
		SetText("✗ ERROR\n\n" + message).
		AddButtons([]string{"OK"}).
		SetDoneFunc(func(buttonIndex int, buttonLabel string) {
			if onClose != nil {
				onClose()
			}
		})

	modal.SetTitle(" " + title + " ").
		SetBorder(true).
		SetBorderColor(tcell.ColorRed)

	return &ErrorDialog{
		modal:   modal,
		onClose: onClose,
	}
}

// GetModal は内部のtview.Modalを返す。
func (d *ErrorDialog) GetModal() *tview.Modal {
	return d.modal
}

// InputDialog は入力ダイアログを表示する。
type InputDialog struct {
	form      *tview.Form
	frame     *tview.Frame
	onSubmit  func(value string)
	onCancel  func()
	inputName string
}

// NewInputDialog は新しいInputDialogを生成する。
func NewInputDialog(title, label, defaultValue string, onSubmit func(value string), onCancel func()) *InputDialog {
	form := tview.NewForm()
	inputName := "input"

	form.AddInputField(label, defaultValue, 20, nil, nil)
	form.AddButton("OK", func() {
		value := form.GetFormItemByLabel(label).(*tview.InputField).GetText()
		if onSubmit != nil {
			onSubmit(value)
		}
	})
	form.AddButton("Cancel", func() {
		if onCancel != nil {
			onCancel()
		}
	})

	form.SetBorder(true).
		SetTitle(" " + title + " ").
		SetTitleAlign(tview.AlignCenter).
		SetBorderColor(tcell.ColorWhite)

	frame := tview.NewFrame(form).
		SetBorders(0, 0, 0, 0, 0, 0)

	return &InputDialog{
		form:      form,
		frame:     frame,
		onSubmit:  onSubmit,
		onCancel:  onCancel,
		inputName: inputName,
	}
}

// GetForm は内部のtview.Formを返す。
func (d *InputDialog) GetForm() *tview.Form {
	return d.form
}

// GetFrame は内部のtview.Frameを返す。
func (d *InputDialog) GetFrame() *tview.Frame {
	return d.frame
}
