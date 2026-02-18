package subscriber

import (
	"context"
	"time"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/store"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/validation"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/rivo/tview"
)

// FormScreen は加入者登録/編集画面を表す。
type FormScreen struct {
	form            *tview.Form
	app             *ui.App
	subscriberStore *store.SubscriberStore
	auditLogger     *audit.Logger
	editMode        bool
	originalIMSI    string
	originalSQN     string
	onSave          func()
	onCancel        func()
}

// NewFormScreen は新しいFormScreenを生成する。
func NewFormScreen(app *ui.App, subscriberStore *store.SubscriberStore, auditLogger *audit.Logger) *FormScreen {
	form := tview.NewForm()

	form.SetBorder(true).
		SetBorderColor(tcell.ColorBlue)

	screen := &FormScreen{
		form:            form,
		app:             app,
		subscriberStore: subscriberStore,
		auditLogger:     auditLogger,
		editMode:        false,
	}

	return screen
}

// SetOnSave は保存時のコールバックを設定する。
func (s *FormScreen) SetOnSave(handler func()) {
	s.onSave = handler
}

// SetOnCancel はキャンセル時のコールバックを設定する。
func (s *FormScreen) SetOnCancel(handler func()) {
	s.onCancel = handler
}

// GetForm は内部のtview.Formを返す。
func (s *FormScreen) GetForm() *tview.Form {
	return s.form
}

// SetupCreate は新規作成モードでフォームをセットアップする。
func (s *FormScreen) SetupCreate() {
	s.editMode = false
	s.originalIMSI = ""
	s.originalSQN = ""

	s.form.Clear(true)
	s.form.SetTitle(" Create Subscriber ")

	s.form.AddInputField("IMSI", "", 20, nil, nil)
	s.form.AddInputField("Ki", "", 40, nil, nil)
	s.form.AddInputField("OPc", "", 40, nil, nil)
	s.form.AddInputField("AMF", "8000", 10, nil, nil)
	s.form.AddInputField("SQN", "000000000000", 20, nil, nil)

	s.form.AddButton("Save", s.handleSave)
	s.form.AddButton("Cancel", s.handleCancel)

	s.setupKeyBindings()
}

// SetupEdit は編集モードでフォームをセットアップする。
func (s *FormScreen) SetupEdit(ctx context.Context, imsi string) error {
	sub, err := s.subscriberStore.Get(ctx, imsi)
	if err != nil {
		return err
	}

	s.editMode = true
	s.originalIMSI = imsi
	s.originalSQN = sub.SQN

	s.form.Clear(true)
	s.form.SetTitle(" Edit Subscriber ")

	// 編集モードではIMSIは変更不可
	s.form.AddInputField("IMSI", sub.IMSI, 20, nil, nil)
	s.form.AddInputField("Ki", sub.Ki, 40, nil, nil)
	s.form.AddInputField("OPc", sub.OPc, 40, nil, nil)
	s.form.AddInputField("AMF", sub.AMF, 10, nil, nil)
	s.form.AddInputField("SQN", sub.SQN, 20, nil, nil)

	// IMSI入力フィールドを無効化
	imsiField := s.form.GetFormItemByLabel("IMSI").(*tview.InputField)
	imsiField.SetDisabled(true)

	s.form.AddButton("Save", s.handleSave)
	s.form.AddButton("Cancel", s.handleCancel)

	s.setupKeyBindings()
	return nil
}

func (s *FormScreen) handleSave() {
	// フォームからデータを取得
	input := &validation.SubscriberInput{
		IMSI: s.form.GetFormItemByLabel("IMSI").(*tview.InputField).GetText(),
		Ki:   s.form.GetFormItemByLabel("Ki").(*tview.InputField).GetText(),
		OPc:  s.form.GetFormItemByLabel("OPc").(*tview.InputField).GetText(),
		AMF:  s.form.GetFormItemByLabel("AMF").(*tview.InputField).GetText(),
		SQN:  s.form.GetFormItemByLabel("SQN").(*tview.InputField).GetText(),
	}

	// 正規化
	input = validation.NormalizeSubscriberInput(input)

	// バリデーション
	if errs := validation.ValidateSubscriber(input); len(errs) > 0 {
		s.app.GetStatusBar().ShowError("Validation error: " + errs[0].Error())
		return
	}

	// SQN変更チェック（編集モード時）
	if s.editMode && input.SQN != s.originalSQN {
		s.showSQNWarningDialog(input)
		return
	}

	s.save(input)
}

func (s *FormScreen) showSQNWarningDialog(input *validation.SubscriberInput) {
	dialog := ui.NewWarningDialog(
		"SQN Modification Warning",
		"Modifying the SQN value may cause authentication failures.\n\n"+
			"Are you sure you want to change the SQN from\n"+
			s.originalSQN+" to "+input.SQN+"?",
		func() {
			s.app.HidePage("sqn-warning")
			s.app.RemovePage("sqn-warning")
			s.save(input)
		},
		func() {
			s.app.HidePage("sqn-warning")
			s.app.RemovePage("sqn-warning")
			s.app.SetFocus(s.form)
		},
	)

	s.app.AddPage("sqn-warning", dialog.GetModal(), true, true)
}

func (s *FormScreen) save(input *validation.SubscriberInput) {
	ctx := context.Background()

	sub := &model.Subscriber{
		IMSI: input.IMSI,
		Ki:   input.Ki,
		OPc:  input.OPc,
		AMF:  input.AMF,
		SQN:  input.SQN,
	}

	if s.editMode {
		// 更新
		if err := s.subscriberStore.Update(ctx, sub); err != nil {
			s.app.GetStatusBar().ShowError("Failed to update: " + err.Error())
			return
		}
		s.auditLogger.LogUpdate(audit.TargetSubscriber, store.SubscriberKey(sub.IMSI), sub.IMSI)
		s.app.GetStatusBar().ShowSuccess("Subscriber updated: " + sub.IMSI)
	} else {
		// 新規作成
		sub.CreatedAt = time.Now().UTC().Format(time.RFC3339)
		if err := s.subscriberStore.Create(ctx, sub); err != nil {
			s.app.GetStatusBar().ShowError("Failed to create: " + err.Error())
			return
		}
		s.auditLogger.LogCreate(audit.TargetSubscriber, store.SubscriberKey(sub.IMSI), sub.IMSI)
		s.app.GetStatusBar().ShowSuccess("Subscriber created: " + sub.IMSI)
	}

	if s.onSave != nil {
		s.onSave()
	}
}

func (s *FormScreen) handleCancel() {
	if s.onCancel != nil {
		s.onCancel()
	}
}

func (s *FormScreen) setupKeyBindings() {
	s.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			s.handleCancel()
			return nil
		}
		return event
	})
}
