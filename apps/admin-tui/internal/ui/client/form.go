package client

import (
	"context"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/store"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/validation"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
	"github.com/rivo/tview"
)

// FormScreen はRADIUSクライアント登録/編集画面を表す。
type FormScreen struct {
	form        *tview.Form
	app         *ui.App
	clientStore *store.ClientStore
	auditLogger *audit.Logger
	editMode    bool
	originalIP  string
	onSave      func()
	onCancel    func()
}

// NewFormScreen は新しいFormScreenを生成する。
func NewFormScreen(app *ui.App, clientStore *store.ClientStore, auditLogger *audit.Logger) *FormScreen {
	form := tview.NewForm()

	form.SetBorder(true).
		SetBorderColor(tcell.ColorBlue)

	screen := &FormScreen{
		form:        form,
		app:         app,
		clientStore: clientStore,
		auditLogger: auditLogger,
		editMode:    false,
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
	s.originalIP = ""

	s.form.Clear(true)
	s.form.SetTitle(" Create RADIUS Client ")

	s.form.AddInputField("IP Address", "", 20, nil, nil)
	s.form.AddInputField("Secret", "", 40, nil, nil)
	s.form.AddInputField("Name", "", 40, nil, nil)
	s.form.AddInputField("Vendor", "", 40, nil, nil)

	s.form.AddButton("Save", s.handleSave)
	s.form.AddButton("Cancel", s.handleCancel)

	s.setupKeyBindings()
}

// SetupEdit は編集モードでフォームをセットアップする。
func (s *FormScreen) SetupEdit(ctx context.Context, ip string) error {
	client, err := s.clientStore.Get(ctx, ip)
	if err != nil {
		return err
	}

	s.editMode = true
	s.originalIP = ip

	s.form.Clear(true)
	s.form.SetTitle(" Edit RADIUS Client ")

	// 編集モードではIPは変更不可
	s.form.AddInputField("IP Address", client.IP, 20, nil, nil)
	s.form.AddInputField("Secret", client.Secret, 40, nil, nil)
	s.form.AddInputField("Name", client.Name, 40, nil, nil)
	s.form.AddInputField("Vendor", client.Vendor, 40, nil, nil)

	// IP入力フィールドを無効化
	ipField := s.form.GetFormItemByLabel("IP Address").(*tview.InputField)
	ipField.SetDisabled(true)

	s.form.AddButton("Save", s.handleSave)
	s.form.AddButton("Cancel", s.handleCancel)

	s.setupKeyBindings()
	return nil
}

func (s *FormScreen) handleSave() {
	// フォームからデータを取得
	input := &validation.ClientInput{
		IP:     s.form.GetFormItemByLabel("IP Address").(*tview.InputField).GetText(),
		Secret: s.form.GetFormItemByLabel("Secret").(*tview.InputField).GetText(),
		Name:   s.form.GetFormItemByLabel("Name").(*tview.InputField).GetText(),
		Vendor: s.form.GetFormItemByLabel("Vendor").(*tview.InputField).GetText(),
	}

	// 正規化
	input = validation.NormalizeClientInput(input)

	// バリデーション
	if errs := validation.ValidateClient(input); len(errs) > 0 {
		s.app.GetStatusBar().ShowError("Validation error: " + errs[0].Error())
		return
	}

	s.save(input)
}

func (s *FormScreen) save(input *validation.ClientInput) {
	ctx := context.Background()

	client := &model.RadiusClient{
		IP:     input.IP,
		Secret: input.Secret,
		Name:   input.Name,
		Vendor: input.Vendor,
	}

	if s.editMode {
		// 更新
		if err := s.clientStore.Update(ctx, client); err != nil {
			s.app.GetStatusBar().ShowError("Failed to update: " + err.Error())
			return
		}
		s.auditLogger.LogUpdate(audit.TargetClient, store.ClientKey(client.IP), "")
		s.app.GetStatusBar().ShowSuccess("Client updated: " + client.IP)
	} else {
		// 新規作成
		if err := s.clientStore.Create(ctx, client); err != nil {
			s.app.GetStatusBar().ShowError("Failed to create: " + err.Error())
			return
		}
		s.auditLogger.LogCreate(audit.TargetClient, store.ClientKey(client.IP), "")
		s.app.GetStatusBar().ShowSuccess("Client created: " + client.IP)
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
