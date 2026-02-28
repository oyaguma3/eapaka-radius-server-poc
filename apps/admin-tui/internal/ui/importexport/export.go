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

// ExportScreen はエクスポート画面を表す。
type ExportScreen struct {
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

// NewExportScreen は新しいExportScreenを生成する。
func NewExportScreen(
	app *ui.App,
	subscriberStore *store.SubscriberStore,
	clientStore *store.ClientStore,
	policyStore *store.PolicyStore,
	auditLogger *audit.Logger,
) *ExportScreen {
	form := tview.NewForm()
	form.SetBorder(true).
		SetTitle(" Export Data ").
		SetBorderColor(tcell.ColorBlue)

	resultView := tview.NewTextView().
		SetDynamicColors(true).
		SetScrollable(true)
	resultView.SetBorder(true).
		SetTitle(" Export Result ").
		SetBorderColor(tcell.ColorGray)

	flex := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(form, 10, 0, true).
		AddItem(resultView, 0, 1, false)

	screen := &ExportScreen{
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
func (s *ExportScreen) SetOnComplete(handler func()) {
	s.onComplete = handler
}

// SetOnCancel はキャンセル時のコールバックを設定する。
func (s *ExportScreen) SetOnCancel(handler func()) {
	s.onCancel = handler
}

// GetFlex は内部のtview.Flexを返す。
func (s *ExportScreen) GetFlex() *tview.Flex {
	return s.flex
}

func (s *ExportScreen) setupForm() {
	s.form.Clear(true)

	s.form.AddDropDown("Data Type", []string{"Subscribers", "RADIUS Clients", "Policies"}, 0, nil)
	s.form.AddInputField("Output File", "", 50, nil, nil)
	s.form.AddButton("Export", s.handleExport)
	s.form.AddButton("Cancel", s.handleCancel)

	s.form.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyEsc {
			s.handleCancel()
			return nil
		}
		return event
	})
}

func (s *ExportScreen) handleExport() {
	_, dataType := s.form.GetFormItemByLabel("Data Type").(*tview.DropDown).GetCurrentOption()
	filePath := s.form.GetFormItemByLabel("Output File").(*tview.InputField).GetText()

	if filePath == "" {
		s.app.GetStatusBar().ShowError("Output file path is required")
		return
	}

	ctx := context.Background()
	var result strings.Builder

	switch dataType {
	case "Subscribers":
		subscribers, err := s.subscriberStore.List(ctx)
		if err != nil {
			result.WriteString("[red]Error loading data: " + err.Error() + "[-]")
			s.resultView.SetText(result.String())
			return
		}

		file, err := os.Create(filePath)
		if err != nil {
			result.WriteString("[red]Error creating file: " + err.Error() + "[-]")
			s.resultView.SetText(result.String())
			return
		}
		defer file.Close()

		if err := csvpkg.WriteSubscriberCSV(file, subscribers); err != nil {
			result.WriteString("[red]Error writing CSV: " + err.Error() + "[-]")
			s.resultView.SetText(result.String())
			return
		}

		s.auditLogger.LogExport(audit.TargetSubscriber, len(subscribers), filePath)
		result.WriteString("[green]Export completed![-]\n\n")
		fmt.Fprintf(&result, "Exported: %d subscribers\n", len(subscribers))
		fmt.Fprintf(&result, "File: %s\n", filePath)
		s.app.GetStatusBar().ShowSuccess(fmt.Sprintf("Exported %d subscribers to %s", len(subscribers), filePath))

	case "RADIUS Clients":
		clients, err := s.clientStore.List(ctx)
		if err != nil {
			result.WriteString("[red]Error loading data: " + err.Error() + "[-]")
			s.resultView.SetText(result.String())
			return
		}

		file, err := os.Create(filePath)
		if err != nil {
			result.WriteString("[red]Error creating file: " + err.Error() + "[-]")
			s.resultView.SetText(result.String())
			return
		}
		defer file.Close()

		if err := csvpkg.WriteClientCSV(file, clients); err != nil {
			result.WriteString("[red]Error writing CSV: " + err.Error() + "[-]")
			s.resultView.SetText(result.String())
			return
		}

		s.auditLogger.LogExport(audit.TargetClient, len(clients), filePath)
		result.WriteString("[green]Export completed![-]\n\n")
		fmt.Fprintf(&result, "Exported: %d clients\n", len(clients))
		fmt.Fprintf(&result, "File: %s\n", filePath)
		s.app.GetStatusBar().ShowSuccess(fmt.Sprintf("Exported %d clients to %s", len(clients), filePath))

	case "Policies":
		policies, err := s.policyStore.List(ctx)
		if err != nil {
			result.WriteString("[red]Error loading data: " + err.Error() + "[-]")
			s.resultView.SetText(result.String())
			return
		}

		file, err := os.Create(filePath)
		if err != nil {
			result.WriteString("[red]Error creating file: " + err.Error() + "[-]")
			s.resultView.SetText(result.String())
			return
		}
		defer file.Close()

		if err := csvpkg.WritePolicyCSV(file, policies); err != nil {
			result.WriteString("[red]Error writing CSV: " + err.Error() + "[-]")
			s.resultView.SetText(result.String())
			return
		}

		s.auditLogger.LogExport(audit.TargetPolicy, len(policies), filePath)
		result.WriteString("[green]Export completed![-]\n\n")
		fmt.Fprintf(&result, "Exported: %d policies\n", len(policies))
		fmt.Fprintf(&result, "File: %s\n", filePath)
		s.app.GetStatusBar().ShowSuccess(fmt.Sprintf("Exported %d policies to %s", len(policies), filePath))
	}

	s.resultView.SetText(result.String())

	// Export成功後、Done/Export Moreボタンを表示
	s.form.Clear(true)
	s.form.SetTitle(" Export Completed ")
	s.form.AddButton("Done", func() {
		if s.onComplete != nil {
			s.onComplete()
		}
	})
	s.form.AddButton("Export More", func() {
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

func (s *ExportScreen) handleCancel() {
	if s.onCancel != nil {
		s.onCancel()
	}
}
