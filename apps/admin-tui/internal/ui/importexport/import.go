// Package importexport はインポート/エクスポート画面を提供する。
package importexport

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/audit"
	csvpkg "github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/csv"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/store"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/rivo/tview"
)

// ImportScreen はインポート画面を表す。
type ImportScreen struct {
	form            *tview.Form
	resultView      *tview.TextView
	flex            *tview.Flex
	app             *ui.App
	subscriberStore *store.SubscriberStore
	clientStore     *store.ClientStore
	policyStore     *store.PolicyStore
	auditLogger     *audit.Logger
	onComplete      func()
	onCancel        func()
}

// NewImportScreen は新しいImportScreenを生成する。
func NewImportScreen(
	app *ui.App,
	subscriberStore *store.SubscriberStore,
	clientStore *store.ClientStore,
	policyStore *store.PolicyStore,
	auditLogger *audit.Logger,
) *ImportScreen {
	form := tview.NewForm()
	form.SetBorder(true).
		SetTitle(" Import Data ").
		SetBorderColor(tcell.ColorBlue)

	resultView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	resultView.SetBorder(true).
		SetTitle(" Import Result ").
		SetBorderColor(tcell.ColorGray)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(form, 12, 0, true).
		AddItem(resultView, 0, 1, false)

	screen := &ImportScreen{
		form:            form,
		resultView:      resultView,
		flex:            flex,
		app:             app,
		subscriberStore: subscriberStore,
		clientStore:     clientStore,
		policyStore:     policyStore,
		auditLogger:     auditLogger,
	}

	screen.setupForm()
	return screen
}

// SetOnComplete は完了時のコールバックを設定する。
func (s *ImportScreen) SetOnComplete(handler func()) {
	s.onComplete = handler
}

// SetOnCancel はキャンセル時のコールバックを設定する。
func (s *ImportScreen) SetOnCancel(handler func()) {
	s.onCancel = handler
}

// GetFlex は内部のtview.Flexを返す。
func (s *ImportScreen) GetFlex() *tview.Flex {
	return s.flex
}

func (s *ImportScreen) setupForm() {
	s.form.Clear(true)

	s.form.AddDropDown("Data Type", []string{"Subscribers", "RADIUS Clients", "Policies"}, 0, nil)
	s.form.AddInputField("File Path", "", 50, nil, nil)
	s.form.AddButton("Validate", s.handleValidate)
	s.form.AddButton("Import", s.handleImport)
	s.form.AddButton("Cancel", s.handleCancel)

	s.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			s.handleCancel()
			return nil
		}
		return event
	})
}

func (s *ImportScreen) handleValidate() {
	_, dataType := s.form.GetFormItemByLabel("Data Type").(*tview.DropDown).GetCurrentOption()
	filePath := s.form.GetFormItemByLabel("File Path").(*tview.InputField).GetText()

	if filePath == "" {
		s.app.GetStatusBar().ShowError("File path is required")
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		s.resultView.SetText("[red]Error: " + err.Error() + "[-]")
		return
	}
	defer file.Close()

	var result strings.Builder
	result.WriteString("[yellow]Validation Result[-]\n\n")

	switch dataType {
	case "Subscribers":
		subscribers, errs := csvpkg.ParseSubscriberCSV(file)
		if len(errs) > 0 {
			result.WriteString("[red]Validation failed:[-]\n")
			for _, e := range errs {
				result.WriteString("  - " + e.Error() + "\n")
			}
		} else {
			result.WriteString("[green]Validation passed![-]\n\n")
			fmt.Fprintf(&result, "Records to import: %d\n", len(subscribers))
		}

	case "RADIUS Clients":
		clients, errs := csvpkg.ParseClientCSV(file)
		if len(errs) > 0 {
			result.WriteString("[red]Validation failed:[-]\n")
			for _, e := range errs {
				result.WriteString("  - " + e.Error() + "\n")
			}
		} else {
			result.WriteString("[green]Validation passed![-]\n\n")
			fmt.Fprintf(&result, "Records to import: %d\n", len(clients))
		}

	case "Policies":
		policies, errs := csvpkg.ParsePolicyCSV(file)
		if len(errs) > 0 {
			result.WriteString("[red]Validation failed:[-]\n")
			for _, e := range errs {
				result.WriteString("  - " + e.Error() + "\n")
			}
		} else {
			result.WriteString("[green]Validation passed![-]\n\n")
			fmt.Fprintf(&result, "Records to import: %d\n", len(policies))
		}
	}

	s.resultView.SetText(result.String())
}

func (s *ImportScreen) handleImport() {
	_, dataType := s.form.GetFormItemByLabel("Data Type").(*tview.DropDown).GetCurrentOption()
	filePath := s.form.GetFormItemByLabel("File Path").(*tview.InputField).GetText()

	if filePath == "" {
		s.app.GetStatusBar().ShowError("File path is required")
		return
	}

	file, err := os.Open(filePath)
	if err != nil {
		s.resultView.SetText("[red]Error: " + err.Error() + "[-]")
		return
	}
	defer file.Close()

	ctx := context.Background()
	var result strings.Builder

	switch dataType {
	case "Subscribers":
		subscribers, errs := csvpkg.ParseSubscriberCSV(file)
		if len(errs) > 0 {
			result.WriteString("[red]Validation failed - Import aborted:[-]\n")
			for _, e := range errs {
				result.WriteString("  - " + e.Error() + "\n")
			}
			s.resultView.SetText(result.String())
			return
		}

		// TxPipelineで一括挿入
		if err := s.subscriberStore.BulkCreate(ctx, subscribers); err != nil {
			result.WriteString("[red]Import failed: " + err.Error() + "[-]")
			s.resultView.SetText(result.String())
			return
		}

		s.auditLogger.LogImport(audit.TargetSubscriber, len(subscribers), filePath)
		result.WriteString("[green]Import completed![-]\n\n")
		fmt.Fprintf(&result, "Imported: %d subscribers\n", len(subscribers))
		s.app.GetStatusBar().ShowSuccess(fmt.Sprintf("Imported %d subscribers", len(subscribers)))

	case "RADIUS Clients":
		clients, errs := csvpkg.ParseClientCSV(file)
		if len(errs) > 0 {
			result.WriteString("[red]Validation failed - Import aborted:[-]\n")
			for _, e := range errs {
				result.WriteString("  - " + e.Error() + "\n")
			}
			s.resultView.SetText(result.String())
			return
		}

		if err := s.clientStore.BulkCreate(ctx, clients); err != nil {
			result.WriteString("[red]Import failed: " + err.Error() + "[-]")
			s.resultView.SetText(result.String())
			return
		}

		s.auditLogger.LogImport(audit.TargetClient, len(clients), filePath)
		result.WriteString("[green]Import completed![-]\n\n")
		fmt.Fprintf(&result, "Imported: %d clients\n", len(clients))
		s.app.GetStatusBar().ShowSuccess(fmt.Sprintf("Imported %d clients", len(clients)))

	case "Policies":
		policies, errs := csvpkg.ParsePolicyCSV(file)
		if len(errs) > 0 {
			result.WriteString("[red]Validation failed - Import aborted:[-]\n")
			for _, e := range errs {
				result.WriteString("  - " + e.Error() + "\n")
			}
			s.resultView.SetText(result.String())
			return
		}

		if err := s.policyStore.BulkCreate(ctx, policies); err != nil {
			result.WriteString("[red]Import failed: " + err.Error() + "[-]")
			s.resultView.SetText(result.String())
			return
		}

		s.auditLogger.LogImport(audit.TargetPolicy, len(policies), filePath)
		result.WriteString("[green]Import completed![-]\n\n")
		fmt.Fprintf(&result, "Imported: %d policies\n", len(policies))
		s.app.GetStatusBar().ShowSuccess(fmt.Sprintf("Imported %d policies", len(policies)))
	}

	s.resultView.SetText(result.String())

	// Import成功後、Done/Import Moreボタンを表示
	s.form.Clear(true)
	s.form.SetTitle(" Import Completed ")
	s.form.AddButton("Done", func() {
		if s.onComplete != nil {
			s.onComplete()
		}
	})
	s.form.AddButton("Import More", func() {
		go func() {
			s.app.QueueUpdateDraw(func() {
				s.setupForm()
				s.resultView.SetText("")
				s.app.SetFocus(s.form)
				s.app.Sync()
			})
		}()
	})

	// InputCaptureを再登録（form.Clear(true)で消失するため）
	s.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			s.handleCancel()
			return nil
		}
		return event
	})
	s.app.SetFocus(s.form)
	s.app.Sync()
}

func (s *ImportScreen) handleCancel() {
	if s.onCancel != nil {
		s.onCancel()
	}
}
