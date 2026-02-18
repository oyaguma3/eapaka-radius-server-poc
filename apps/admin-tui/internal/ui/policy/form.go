package policy

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/model"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/store"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/validation"
	"github.com/rivo/tview"
)

// FormScreen はポリシー登録/編集画面を表す。
type FormScreen struct {
	flex         *tview.Flex
	form         *tview.Form
	rulesList    *tview.List
	app          *ui.App
	policyStore  *store.PolicyStore
	auditLogger  *audit.Logger
	editMode     bool
	originalIMSI string
	policy       *model.Policy
	onSave       func()
	onCancel     func()
}

// NewFormScreen は新しいFormScreenを生成する。
func NewFormScreen(app *ui.App, policyStore *store.PolicyStore, auditLogger *audit.Logger) *FormScreen {
	form := tview.NewForm()
	form.SetBorder(true).
		SetTitle(" Policy Details ").
		SetBorderColor(tcell.ColorBlue)

	rulesList := tview.NewList().
		ShowSecondaryText(true)
	rulesList.SetBorder(true).
		SetTitle(" Rules ").
		SetBorderColor(tcell.ColorBlue)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(form, 10, 0, true).
		AddItem(rulesList, 0, 1, false)

	screen := &FormScreen{
		flex:        flex,
		form:        form,
		rulesList:   rulesList,
		app:         app,
		policyStore: policyStore,
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

// GetFlex は内部のtview.Flexを返す。
func (s *FormScreen) GetFlex() *tview.Flex {
	return s.flex
}

// SetupCreate は新規作成モードでフォームをセットアップする。
func (s *FormScreen) SetupCreate() {
	s.editMode = false
	s.originalIMSI = ""
	s.policy = model.NewPolicy("", "deny")

	s.setupForm()
}

// SetupEdit は編集モードでフォームをセットアップする。
func (s *FormScreen) SetupEdit(ctx context.Context, imsi string) error {
	policy, err := s.policyStore.Get(ctx, imsi)
	if err != nil {
		return err
	}

	s.editMode = true
	s.originalIMSI = imsi
	s.policy = policy.Clone()

	s.setupForm()
	return nil
}

func (s *FormScreen) setupForm() {
	s.form.Clear(true)

	if s.editMode {
		s.flex.SetTitle(" Edit Policy ")
	} else {
		s.flex.SetTitle(" Create Policy ")
	}

	// IMSI
	s.form.AddInputField("IMSI", s.policy.IMSI, 20, nil, nil)
	if s.editMode {
		imsiField := s.form.GetFormItemByLabel("IMSI").(*tview.InputField)
		imsiField.SetDisabled(true)
	}

	// Default action
	defaultOptions := []string{"deny", "allow"}
	defaultIndex := 0
	if s.policy.Default == "allow" {
		defaultIndex = 1
	}
	s.form.AddDropDown("Default Action", defaultOptions, defaultIndex, nil)

	// Buttons
	s.form.AddButton("Add Rule", s.showAddRuleDialog)
	s.form.AddButton("Save", s.handleSave)
	s.form.AddButton("Cancel", s.handleCancel)

	s.updateRulesList()
	s.setupKeyBindings()
}

func (s *FormScreen) updateRulesList() {
	s.rulesList.Clear()

	if len(s.policy.Rules) == 0 {
		s.rulesList.AddItem("(No rules defined)", "", 0, nil)
		return
	}

	for i, rule := range s.policy.Rules {
		idx := i
		mainText := fmt.Sprintf("[%d] NAS: %s", i+1, rule.NasID)
		secondText := fmt.Sprintf("SSIDs: %s", strings.Join(rule.AllowedSSIDs, ", "))
		if rule.VlanID != "" {
			secondText += fmt.Sprintf(" | VLAN: %s", rule.VlanID)
		}
		if rule.SessionTimeout > 0 {
			secondText += fmt.Sprintf(" | Timeout: %ds", rule.SessionTimeout)
		}

		s.rulesList.AddItem(mainText, secondText, 0, func() {
			s.showEditRuleDialog(idx)
		})
	}
}

func (s *FormScreen) showAddRuleDialog() {
	s.showRuleDialog(-1, &model.PolicyRule{
		NasID:        "*",
		AllowedSSIDs: []string{},
	})
}

func (s *FormScreen) showEditRuleDialog(index int) {
	if index < 0 || index >= len(s.policy.Rules) {
		return
	}
	rule := s.policy.Rules[index]
	s.showRuleDialog(index, &rule)
}

func (s *FormScreen) showRuleDialog(index int, rule *model.PolicyRule) {
	ruleForm := tview.NewForm()

	title := " Add Rule "
	if index >= 0 {
		title = " Edit Rule "
	}

	ruleForm.SetTitle(title).
		SetBorder(true).
		SetBorderColor(tcell.ColorTeal)

	ruleForm.AddInputField("NAS ID", rule.NasID, 40, nil, nil)
	ruleForm.AddInputField("Allowed SSIDs", strings.Join(rule.AllowedSSIDs, ","), 40, nil, nil)
	ruleForm.AddInputField("VLAN ID", rule.VlanID, 10, nil, nil)
	ruleForm.AddInputField("Session Timeout", strconv.Itoa(rule.SessionTimeout), 10, nil, nil)

	ruleForm.AddButton("OK", func() {
		nasID := ruleForm.GetFormItemByLabel("NAS ID").(*tview.InputField).GetText()
		ssidsStr := ruleForm.GetFormItemByLabel("Allowed SSIDs").(*tview.InputField).GetText()
		vlanStr := ruleForm.GetFormItemByLabel("VLAN ID").(*tview.InputField).GetText()
		timeoutStr := ruleForm.GetFormItemByLabel("Session Timeout").(*tview.InputField).GetText()

		// Parse SSIDs
		var ssids []string
		for _, ssid := range strings.Split(ssidsStr, ",") {
			ssid = strings.TrimSpace(ssid)
			if ssid != "" {
				ssids = append(ssids, ssid)
			}
		}

		// Parse VLAN ID (string型として保持)
		vlanID := strings.TrimSpace(vlanStr)

		// Parse Session Timeout
		sessionTimeout := 0
		if timeoutStr != "" {
			if t, err := strconv.Atoi(timeoutStr); err == nil {
				sessionTimeout = t
			}
		}

		newRule := model.PolicyRule{
			NasID:          strings.TrimSpace(nasID),
			AllowedSSIDs:   ssids,
			VlanID:         vlanID,
			SessionTimeout: sessionTimeout,
		}

		// Validate
		if errs := validation.ValidatePolicyRule(&newRule); len(errs) > 0 {
			s.app.GetStatusBar().ShowError("Validation error: " + errs[0].Error())
			return
		}

		if index >= 0 {
			s.policy.Rules[index] = newRule
		} else {
			s.policy.Rules = append(s.policy.Rules, newRule)
		}

		s.app.HidePage("rule-dialog")
		s.app.RemovePage("rule-dialog")
		s.updateRulesList()
		s.app.SetFocus(s.form)
	})

	if index >= 0 {
		ruleForm.AddButton("Delete", func() {
			s.policy.Rules = append(s.policy.Rules[:index], s.policy.Rules[index+1:]...)
			s.app.HidePage("rule-dialog")
			s.app.RemovePage("rule-dialog")
			s.updateRulesList()
			s.app.SetFocus(s.form)
		})
	}

	ruleForm.AddButton("Cancel", func() {
		s.app.HidePage("rule-dialog")
		s.app.RemovePage("rule-dialog")
		s.app.SetFocus(s.form)
	})

	s.app.AddPage("rule-dialog", centered(ruleForm, 60, 15), true, true)
	s.app.SetFocus(ruleForm)
}

func (s *FormScreen) handleSave() {
	// フォームからデータを取得
	imsi := s.form.GetFormItemByLabel("IMSI").(*tview.InputField).GetText()
	_, defaultAction := s.form.GetFormItemByLabel("Default Action").(*tview.DropDown).GetCurrentOption()

	s.policy.IMSI = strings.TrimSpace(imsi)
	s.policy.Default = defaultAction

	// バリデーション
	input := &validation.PolicyInput{
		IMSI:    s.policy.IMSI,
		Default: s.policy.Default,
		Rules:   s.policy.Rules,
	}
	if errs := validation.ValidatePolicy(input); len(errs) > 0 {
		s.app.GetStatusBar().ShowError("Validation error: " + errs[0].Error())
		return
	}

	// Default=allow の場合は警告
	if s.policy.Default == "allow" {
		s.showAllowWarningDialog()
		return
	}

	s.save()
}

func (s *FormScreen) showAllowWarningDialog() {
	dialog := ui.NewWarningDialog(
		"Default Allow Warning",
		"Setting default action to 'allow' means the subscriber\nwill have access even without matching rules.\n\nAre you sure you want to continue?",
		func() {
			s.app.HidePage("allow-warning")
			s.app.RemovePage("allow-warning")
			s.save()
		},
		func() {
			s.app.HidePage("allow-warning")
			s.app.RemovePage("allow-warning")
			s.app.SetFocus(s.form)
		},
	)

	s.app.AddPage("allow-warning", dialog.GetModal(), true, true)
}

func (s *FormScreen) save() {
	ctx := context.Background()

	if s.editMode {
		// 更新
		if err := s.policyStore.Update(ctx, s.policy); err != nil {
			s.app.GetStatusBar().ShowError("Failed to update: " + err.Error())
			return
		}
		s.auditLogger.LogUpdate(audit.TargetPolicy, store.PolicyKey(s.policy.IMSI), s.policy.IMSI)
		s.app.GetStatusBar().ShowSuccess("Policy updated: " + s.policy.IMSI)
	} else {
		// 新規作成
		if err := s.policyStore.Create(ctx, s.policy); err != nil {
			s.app.GetStatusBar().ShowError("Failed to create: " + err.Error())
			return
		}
		s.auditLogger.LogCreate(audit.TargetPolicy, store.PolicyKey(s.policy.IMSI), s.policy.IMSI)
		s.app.GetStatusBar().ShowSuccess("Policy created: " + s.policy.IMSI)
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
		if event.Key() == tcell.KeyTab {
			// TabでルールリストにフォーカスTogle
			s.app.SetFocus(s.rulesList)
			return nil
		}
		return event
	})

	s.rulesList.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc || event.Key() == tcell.KeyTab {
			s.app.SetFocus(s.form)
			return nil
		}
		return event
	})
}

func centered(p tview.Primitive, width, height int) tview.Primitive {
	return tview.NewFlex().
		AddItem(nil, 0, 1, false).
		AddItem(tview.NewFlex().SetDirection(tview.FlexRow).
			AddItem(nil, 0, 1, false).
			AddItem(p, height, 1, true).
			AddItem(nil, 0, 1, false), width, 1, true).
		AddItem(nil, 0, 1, false)
}
