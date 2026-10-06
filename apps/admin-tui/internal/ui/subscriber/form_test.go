package subscriber

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/masterdata"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/validation"
	"github.com/redis/go-redis/v9"
	"github.com/rivo/tview"
)

const formTestIMSI = "440101234567890"

// newTestEditForm は miniredis に加入者を登録し、その加入者の編集画面を開いた状態にする。
// 登録する SQN は Vector API が書き戻す形式（小文字）にする。
func newTestEditForm(t *testing.T) (*FormScreen, *ui.App, *miniredis.Miniredis) {
	t.Helper()

	mr := miniredis.RunT(t)
	client := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = client.Close() })

	mr.HSet(masterdata.SubscriberKey(formTestIMSI),
		"ki", "465B5CE8B199B49FAA5F0A2EE238A6BC",
		"opc", "CD63CB71954A9F4E48A5994E37A02BAF",
		"amf", "8000",
		"sqn", "00000000004a",
		"created_at", "2026-01-01T00:00:00Z",
	)

	app := ui.NewApp()
	s := NewFormScreen(app, masterdata.NewSubscriberStore(client), audit.NewLoggerWithWriter(&bytes.Buffer{}, "test"))
	if err := s.SetupEdit(context.Background(), formTestIMSI); err != nil {
		t.Fatalf("SetupEdit failed: %v", err)
	}
	return s, app, mr
}

// setField はフォームの入力欄に値を設定する。
func setField(s *FormScreen, label, value string) {
	s.form.GetFormItemByLabel(label).(*tview.InputField).SetText(value)
}

// pressContinue は SQN 変更の警告ダイアログで Continue を押す。
func pressContinue(t *testing.T, app *ui.App) {
	t.Helper()

	modal, ok := app.GetPages().GetPage("sqn-warning").(*tview.Modal)
	if !ok {
		t.Fatal("SQN warning dialog is not shown")
	}
	// フォーカスを最初のボタン（Continue）に移してから Enter を送る
	var setFocus func(p tview.Primitive)
	setFocus = func(p tview.Primitive) { p.Focus(setFocus) }
	modal.Focus(setFocus)
	modal.InputHandler()(tcell.NewEventKey(tcell.KeyEnter, 0, tcell.ModNone), setFocus)
}

func statusText(app *ui.App) string {
	return app.GetStatusBar().GetView().GetText(true)
}

func TestFormScreen_EditWithoutSQNChange_KeepsSQN(t *testing.T) {
	s, app, mr := newTestEditForm(t)
	saved := false
	s.SetOnSave(func() { saved = true })

	// 編集画面を開いている間に、認証で Vector API が SQN を進めた
	key := masterdata.SubscriberKey(formTestIMSI)
	mr.HSet(key, "sqn", "00000000006a")

	// SQN 以外（AMF）だけを変えて保存する
	setField(s, "AMF", "b9b9")
	s.handleSave()

	// SQN は大文字小文字の違いだけなので変更とみなさず、警告ダイアログを出さない
	if app.GetPages().HasPage("sqn-warning") {
		t.Error("SQN warning dialog must not be shown when SQN is not changed")
	}
	if !saved {
		t.Fatalf("onSave not called: %s", statusText(app))
	}
	if got := mr.HGet(key, "amf"); got != "B9B9" {
		t.Errorf("amf = %s, want B9B9", got)
	}
	// 認証で進んだ SQN を巻き戻さない
	if got := mr.HGet(key, "sqn"); got != "00000000006a" {
		t.Errorf("sqn = %s, want 00000000006a (not rolled back)", got)
	}
}

func TestFormScreen_EditWithSQNChange_UpdatesSQN(t *testing.T) {
	s, app, mr := newTestEditForm(t)
	saved := false
	s.SetOnSave(func() { saved = true })

	setField(s, "SQN", "000000000100")
	s.handleSave()

	// SQN を変えたときは警告ダイアログを出し、Continue で保存する
	if !app.GetPages().HasPage("sqn-warning") {
		t.Fatal("SQN warning dialog should be shown")
	}
	if saved {
		t.Fatal("must not save before confirmation")
	}
	pressContinue(t, app)

	if !saved {
		t.Fatalf("onSave not called: %s", statusText(app))
	}
	if got := mr.HGet(masterdata.SubscriberKey(formTestIMSI), "sqn"); got != "000000000100" {
		t.Errorf("sqn = %s, want 000000000100", got)
	}
}

func TestFormScreen_EditWithSQNChange_ConflictWithAuthentication(t *testing.T) {
	s, app, mr := newTestEditForm(t)
	saved := false
	s.SetOnSave(func() { saved = true })

	setField(s, "Ki", "00112233445566778899AABBCCDDEEFF")
	setField(s, "SQN", "000000000100")
	s.handleSave()

	// 警告ダイアログを確認している間に、認証で Vector API が SQN を進めた
	key := masterdata.SubscriberKey(formTestIMSI)
	mr.HSet(key, "sqn", "00000000006a")
	pressContinue(t, app)

	// 何も書き換えず、開き直すよう案内する
	if saved {
		t.Error("onSave must not be called when SQN was changed while editing")
	}
	if got := statusText(app); !strings.Contains(got, "SQN was changed by authentication while editing") {
		t.Errorf("status = %q", got)
	}
	if got := mr.HGet(key, "sqn"); got != "00000000006a" {
		t.Errorf("sqn = %s, want 00000000006a", got)
	}
	if got := mr.HGet(key, "ki"); got != "465B5CE8B199B49FAA5F0A2EE238A6BC" {
		t.Errorf("ki must not be updated: %s", got)
	}
}

func TestFormScreen_EditSubscriberDeleted(t *testing.T) {
	s, app, mr := newTestEditForm(t)
	saved := false
	s.SetOnSave(func() { saved = true })

	// 編集画面を開いている間に加入者が削除された
	key := masterdata.SubscriberKey(formTestIMSI)
	mr.Del(key)
	s.handleSave()

	if saved {
		t.Error("onSave must not be called")
	}
	if got := statusText(app); !strings.Contains(got, "Failed to update: subscriber not found") {
		t.Errorf("status = %q", got)
	}
	// 一部のフィールドだけの Hash を作らない
	if mr.Exists(key) {
		t.Errorf("key %s must not be created", key)
	}
}

func TestFormScreen_SQNChanged(t *testing.T) {
	s := &FormScreen{editMode: true, originalSQN: "00000000004a"}

	for _, tt := range []struct {
		sqn  string
		want bool
	}{
		{"00000000004A", false}, // 大文字小文字の違いだけ
		{"00000000004a", false},
		{"000000000100", true},
	} {
		in := &validation.SubscriberInput{SQN: tt.sqn}
		if got := s.sqnChanged(in); got != tt.want {
			t.Errorf("sqnChanged(%s) = %v, want %v", tt.sqn, got, tt.want)
		}
	}

	// 新規作成モードでは常に false
	create := &FormScreen{editMode: false}
	if create.sqnChanged(&validation.SubscriberInput{SQN: "000000000100"}) {
		t.Error("sqnChanged must be false in create mode")
	}
}

// pressCancel は SQN 変更の警告ダイアログを Esc で閉じる（Cancel と同じ扱い）。
func pressCancel(t *testing.T, app *ui.App) {
	t.Helper()

	modal, ok := app.GetPages().GetPage("sqn-warning").(*tview.Modal)
	if !ok {
		t.Fatal("SQN warning dialog is not shown")
	}
	var setFocus func(p tview.Primitive)
	setFocus = func(p tview.Primitive) { p.Focus(setFocus) }
	modal.Focus(setFocus)
	modal.InputHandler()(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone), setFocus)
}

func TestFormScreen_EditWithSQNChange_Cancel(t *testing.T) {
	s, app, mr := newTestEditForm(t)
	saved := false
	s.SetOnSave(func() { saved = true })

	setField(s, "SQN", "000000000100")
	s.handleSave()
	pressCancel(t, app)

	if saved {
		t.Error("onSave must not be called after Cancel")
	}
	if app.GetPages().HasPage("sqn-warning") {
		t.Error("SQN warning dialog should be closed")
	}
	if got := mr.HGet(masterdata.SubscriberKey(formTestIMSI), "sqn"); got != "00000000004a" {
		t.Errorf("sqn = %s, want 00000000004a", got)
	}
}

func TestFormScreen_EditValidationError(t *testing.T) {
	s, app, mr := newTestEditForm(t)
	saved := false
	s.SetOnSave(func() { saved = true })

	setField(s, "SQN", "XYZ")
	s.handleSave()

	if saved || app.GetPages().HasPage("sqn-warning") {
		t.Error("invalid input must not be saved")
	}
	if got := statusText(app); !strings.Contains(got, "Validation error") {
		t.Errorf("status = %q", got)
	}
	if got := mr.HGet(masterdata.SubscriberKey(formTestIMSI), "sqn"); got != "00000000004a" {
		t.Errorf("sqn = %s, want 00000000004a", got)
	}
}

func TestFormScreen_Create(t *testing.T) {
	s, app, mr := newTestEditForm(t)
	saved := false
	s.SetOnSave(func() { saved = true })

	s.SetupCreate()
	if s.GetForm().GetTitle() != " Create Subscriber " {
		t.Errorf("title = %q", s.GetForm().GetTitle())
	}
	const imsi = "440101234567891"
	setField(s, "IMSI", imsi)
	setField(s, "Ki", "465b5ce8b199b49faa5f0a2ee238a6bc")
	setField(s, "OPc", "cd63cb71954a9f4e48a5994e37a02baf")
	s.handleSave()

	// 新規作成では SQN の警告ダイアログを出さず、入力値（大文字に正規化）で作成する
	if app.GetPages().HasPage("sqn-warning") {
		t.Error("SQN warning dialog must not be shown in create mode")
	}
	if !saved {
		t.Fatalf("onSave not called: %s", statusText(app))
	}
	key := masterdata.SubscriberKey(imsi)
	if got := mr.HGet(key, "sqn"); got != "000000000000" {
		t.Errorf("sqn = %s, want 000000000000", got)
	}
	if got := mr.HGet(key, "ki"); got != "465B5CE8B199B49FAA5F0A2EE238A6BC" {
		t.Errorf("ki = %s", got)
	}
	if mr.HGet(key, "created_at") == "" {
		t.Error("created_at should be set")
	}

	// 同じ IMSI で作成するとエラー
	saved = false
	s.SetupCreate()
	setField(s, "IMSI", imsi)
	setField(s, "Ki", "465b5ce8b199b49faa5f0a2ee238a6bc")
	setField(s, "OPc", "cd63cb71954a9f4e48a5994e37a02baf")
	s.handleSave()
	if saved {
		t.Error("onSave must not be called for duplicate IMSI")
	}
	if got := statusText(app); !strings.Contains(got, "Failed to create") {
		t.Errorf("status = %q", got)
	}
}

func TestFormScreen_EscCancels(t *testing.T) {
	s, _, _ := newTestEditForm(t)
	canceled := false
	s.SetOnCancel(func() { canceled = true })

	if ev := s.form.GetInputCapture()(tcell.NewEventKey(tcell.KeyEsc, 0, tcell.ModNone)); ev != nil {
		t.Error("Esc should be consumed")
	}
	if !canceled {
		t.Error("onCancel should be called by Esc")
	}

	// Esc 以外のキーはそのまま渡す
	if ev := s.form.GetInputCapture()(tcell.NewEventKey(tcell.KeyTab, 0, tcell.ModNone)); ev == nil {
		t.Error("Tab should be passed through")
	}
}

func TestFormScreen_SetupEdit_NotFound(t *testing.T) {
	s, _, mr := newTestEditForm(t)
	mr.Del(masterdata.SubscriberKey(formTestIMSI))

	if err := s.SetupEdit(context.Background(), formTestIMSI); err == nil {
		t.Error("SetupEdit should fail for unknown IMSI")
	}
}
