// Admin TUI - EAP-AKA RADIUS PoC管理コンソール
package main

import (
	"context"
	"log"
	"os"

	"github.com/gdamore/tcell/v2"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/config"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/store"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui/client"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui/importexport"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui/monitoring"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui/policy"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/admin-tui/internal/ui/subscriber"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/valkey"
	"github.com/redis/go-redis/v9"
	"github.com/rivo/tview"
)

// Application はアプリケーション全体を管理する。
type Application struct {
	app         *ui.App
	cfg         *config.Config
	redisClient *redis.Client
	auditLogger *audit.Logger

	// Stores
	subscriberStore *store.SubscriberStore
	clientStore     *store.ClientStore
	policyStore     *store.PolicyStore
	sessionStore    *store.SessionStore
	statisticsStore *store.StatisticsStore
}

func main() {
	// 設定読み込み
	cfg := config.Load()

	// 監査ログ初期化
	auditLogger := audit.NewLogger("admin")

	// アプリケーション作成
	application := &Application{
		app:         ui.NewApp(),
		cfg:         cfg,
		auditLogger: auditLogger,
	}

	// Valkey接続
	if err := application.connectValkey(); err != nil {
		application.showStartupError(err.Error())
		return
	}

	// メインメニュー表示
	application.showMainMenu()

	// グローバルキーバインド設定
	application.setupGlobalKeyBindings()

	// アプリケーション実行
	if err := application.app.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func (a *Application) connectValkey() error {
	opts := valkey.TUIOptions().
		WithAddr(a.cfg.ValkeyAddr).
		WithPassword(a.cfg.ValkeyPassword)

	client, err := valkey.NewClient(opts)
	if err != nil {
		return err
	}

	a.redisClient = client

	// Store初期化
	a.subscriberStore = store.NewSubscriberStore(client)
	a.clientStore = store.NewClientStore(client)
	a.policyStore = store.NewPolicyStore(client)
	a.sessionStore = store.NewSessionStore(client)
	a.statisticsStore = store.NewStatisticsStore(
		a.subscriberStore,
		a.clientStore,
		a.policyStore,
		a.sessionStore,
	)

	return nil
}

func (a *Application) showStartupError(errorMessage string) {
	errorScreen := ui.NewStartupErrorScreen(
		errorMessage,
		func() {
			// Retry
			if err := a.connectValkey(); err != nil {
				a.app.GetStatusBar().ShowError("Connection failed: " + err.Error())
			} else {
				a.app.HidePage("startup-error")
				a.app.RemovePage("startup-error")
				a.showMainMenu()
			}
		},
		func() {
			// Exit
			a.app.Stop()
		},
	)

	a.app.AddPage("startup-error", errorScreen.GetModal(), true, true)
	a.app.GetStatusBar().SetApp(a.app.GetApplication())

	if err := a.app.Run(); err != nil {
		log.Fatalf("Application error: %v", err)
	}
}

func (a *Application) showMainMenu() {
	menuItems := ui.GetDefaultMenuItems()

	// アクション設定
	menuItems[0].Action = a.showSubscriberList
	menuItems[1].Action = a.showClientList
	menuItems[2].Action = a.showPolicyList
	menuItems[3].Action = a.showImportExportMenu
	menuItems[4].Action = a.showMonitoringMenu
	menuItems[5].Action = func() {
		a.cleanup()
		a.app.Stop()
	}

	menu := ui.NewMainMenu(menuItems)
	menu.SetOnQuit(func() {
		a.cleanup()
		a.app.Stop()
	})

	a.app.AddPage("main-menu", menu.GetList(), true, true)
	a.app.GetStatusBar().SetApp(a.app.GetApplication())
}

func (a *Application) setupGlobalKeyBindings() {
	a.app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		// Ctrl+Q で終了
		if event.Key() == tcell.KeyCtrlQ {
			a.cleanup()
			a.app.Stop()
			return nil
		}

		// F1 でヘルプ
		if event.Key() == tcell.KeyF1 {
			a.showHelp()
			return nil
		}

		return event
	})
}

func (a *Application) showHelp() {
	helpModal := ui.NewHelpModal(ui.GetDefaultHelpSections(), func() {
		a.app.HidePage("help")
		a.app.RemovePage("help")
	})

	a.app.AddPage("help", helpModal.GetModal(), true, true)
}

// Subscriber Management
func (a *Application) showSubscriberList() {
	screen := subscriber.NewListScreen(a.app, a.subscriberStore, a.policyStore)

	screen.SetOnCreate(func() {
		a.showSubscriberForm(false, "")
	})

	screen.SetOnEdit(func(imsi string) {
		a.showSubscriberForm(true, imsi)
	})

	screen.SetOnDelete(func(imsi string) {
		a.showDeleteConfirm("subscriber", imsi, func() {
			ctx := context.Background()
			if err := a.subscriberStore.Delete(ctx, imsi); err != nil {
				a.app.GetStatusBar().ShowError("Failed to delete: " + err.Error())
				return
			}
			a.auditLogger.LogDelete(audit.TargetSubscriber, store.SubscriberKey(imsi), imsi)
			a.app.GetStatusBar().ShowSuccess("Subscriber deleted: " + imsi)
			_ = screen.Refresh(ctx)
		})
	})

	screen.SetOnBack(func() {
		a.app.SwitchToPage("main-menu")
	})

	a.app.AddPage("subscriber-list", screen.GetTable(), true, false)
	a.app.SwitchToPage("subscriber-list")
	a.app.SetFocus(screen.GetTable())

	// データ読み込み
	go func() {
		a.app.QueueUpdateDraw(func() {
			if err := screen.Load(context.Background()); err != nil {
				a.app.GetStatusBar().ShowError("Failed to load: " + err.Error())
			}
		})
	}()
}

func (a *Application) showSubscriberForm(editMode bool, imsi string) {
	screen := subscriber.NewFormScreen(a.app, a.subscriberStore, a.auditLogger)

	screen.SetOnSave(func() {
		a.app.HidePage("subscriber-form")
		a.app.RemovePage("subscriber-form")
		a.showSubscriberList()
	})

	screen.SetOnCancel(func() {
		a.app.HidePage("subscriber-form")
		a.app.RemovePage("subscriber-form")
		a.app.SwitchToPage("subscriber-list")
	})

	if editMode {
		if err := screen.SetupEdit(context.Background(), imsi); err != nil {
			a.app.GetStatusBar().ShowError("Failed to load subscriber: " + err.Error())
			return
		}
	} else {
		screen.SetupCreate()
	}

	a.app.AddPage("subscriber-form", centered(screen.GetForm(), 60, 15), true, true)
	a.app.SetFocus(screen.GetForm())
}

// Client Management
func (a *Application) showClientList() {
	screen := client.NewListScreen(a.app, a.clientStore)

	screen.SetOnCreate(func() {
		a.showClientForm(false, "")
	})

	screen.SetOnEdit(func(ip string) {
		a.showClientForm(true, ip)
	})

	screen.SetOnDelete(func(ip string) {
		a.showDeleteConfirm("client", ip, func() {
			ctx := context.Background()
			if err := a.clientStore.Delete(ctx, ip); err != nil {
				a.app.GetStatusBar().ShowError("Failed to delete: " + err.Error())
				return
			}
			a.auditLogger.LogDelete(audit.TargetClient, store.ClientKey(ip), "")
			a.app.GetStatusBar().ShowSuccess("Client deleted: " + ip)
			_ = screen.Refresh(ctx)
		})
	})

	screen.SetOnBack(func() {
		a.app.SwitchToPage("main-menu")
	})

	a.app.AddPage("client-list", screen.GetTable(), true, false)
	a.app.SwitchToPage("client-list")
	a.app.SetFocus(screen.GetTable())

	go func() {
		a.app.QueueUpdateDraw(func() {
			if err := screen.Load(context.Background()); err != nil {
				a.app.GetStatusBar().ShowError("Failed to load: " + err.Error())
			}
		})
	}()
}

func (a *Application) showClientForm(editMode bool, ip string) {
	screen := client.NewFormScreen(a.app, a.clientStore, a.auditLogger)

	screen.SetOnSave(func() {
		a.app.HidePage("client-form")
		a.app.RemovePage("client-form")
		a.showClientList()
	})

	screen.SetOnCancel(func() {
		a.app.HidePage("client-form")
		a.app.RemovePage("client-form")
		a.app.SwitchToPage("client-list")
	})

	if editMode {
		if err := screen.SetupEdit(context.Background(), ip); err != nil {
			a.app.GetStatusBar().ShowError("Failed to load client: " + err.Error())
			return
		}
	} else {
		screen.SetupCreate()
	}

	a.app.AddPage("client-form", centered(screen.GetForm(), 60, 12), true, true)
	a.app.SetFocus(screen.GetForm())
}

// Policy Management
func (a *Application) showPolicyList() {
	screen := policy.NewListScreen(a.app, a.policyStore)

	screen.SetOnCreate(func() {
		a.showPolicyForm(false, "")
	})

	screen.SetOnEdit(func(imsi string) {
		a.showPolicyForm(true, imsi)
	})

	screen.SetOnDelete(func(imsi string) {
		a.showDeleteConfirm("policy", imsi, func() {
			ctx := context.Background()
			if err := a.policyStore.Delete(ctx, imsi); err != nil {
				a.app.GetStatusBar().ShowError("Failed to delete: " + err.Error())
				return
			}
			a.auditLogger.LogDelete(audit.TargetPolicy, store.PolicyKey(imsi), imsi)
			a.app.GetStatusBar().ShowSuccess("Policy deleted: " + imsi)
			_ = screen.Refresh(ctx)
		})
	})

	screen.SetOnBack(func() {
		a.app.SwitchToPage("main-menu")
	})

	a.app.AddPage("policy-list", screen.GetTable(), true, false)
	a.app.SwitchToPage("policy-list")
	a.app.SetFocus(screen.GetTable())

	go func() {
		a.app.QueueUpdateDraw(func() {
			if err := screen.Load(context.Background()); err != nil {
				a.app.GetStatusBar().ShowError("Failed to load: " + err.Error())
			}
		})
	}()
}

func (a *Application) showPolicyForm(editMode bool, imsi string) {
	screen := policy.NewFormScreen(a.app, a.policyStore, a.auditLogger)

	screen.SetOnSave(func() {
		a.app.HidePage("policy-form")
		a.app.RemovePage("policy-form")
		a.showPolicyList()
	})

	screen.SetOnCancel(func() {
		a.app.HidePage("policy-form")
		a.app.RemovePage("policy-form")
		a.app.SwitchToPage("policy-list")
	})

	if editMode {
		if err := screen.SetupEdit(context.Background(), imsi); err != nil {
			a.app.GetStatusBar().ShowError("Failed to load policy: " + err.Error())
			return
		}
	} else {
		screen.SetupCreate()
	}

	a.app.AddPage("policy-form", screen.GetFlex(), true, true)
	a.app.SetFocus(screen.GetFlex())
}

// Import/Export
func (a *Application) showImportExportMenu() {
	list := ui.NewMainMenu([]ui.MenuItem{
		{Label: "Import", Description: "Import data from CSV file", Key: '1', Action: a.showImportScreen},
		{Label: "Export", Description: "Export data to CSV file", Key: '2', Action: a.showExportScreen},
		{Label: "Back", Description: "Return to main menu", Key: 'q', Action: func() {
			a.app.SwitchToPage("main-menu")
		}},
	})

	list.GetList().SetTitle(" Import/Export ")
	list.SetOnQuit(func() {
		a.app.SwitchToPage("main-menu")
	})

	a.app.AddPage("import-export-menu", list.GetList(), true, false)
	a.app.SwitchToPage("import-export-menu")
}

func (a *Application) showImportScreen() {
	screen := importexport.NewImportScreen(a.app, a.subscriberStore, a.clientStore, a.policyStore, a.auditLogger)

	screen.SetOnComplete(func() {
		a.app.SwitchToPage("import-export-menu")
	})

	screen.SetOnCancel(func() {
		a.app.HidePage("import-screen")
		a.app.RemovePage("import-screen")
		a.app.SwitchToPage("import-export-menu")
	})

	a.app.AddPage("import-screen", screen.GetFlex(), true, false)
	a.app.SwitchToPage("import-screen")
}

func (a *Application) showExportScreen() {
	screen := importexport.NewExportScreen(a.app, a.subscriberStore, a.clientStore, a.policyStore, a.auditLogger)

	screen.SetOnComplete(func() {
		a.app.SwitchToPage("import-export-menu")
	})

	screen.SetOnCancel(func() {
		a.app.HidePage("export-screen")
		a.app.RemovePage("export-screen")
		a.app.SwitchToPage("import-export-menu")
	})

	a.app.AddPage("export-screen", screen.GetFlex(), true, false)
	a.app.SwitchToPage("export-screen")
}

// Monitoring
func (a *Application) showMonitoringMenu() {
	screen := monitoring.NewMenuScreen()

	screen.SetOnStatistics(a.showStatistics)
	screen.SetOnSessionList(a.showSessionList)
	screen.SetOnBack(func() {
		a.app.SwitchToPage("main-menu")
	})

	a.app.AddPage("monitoring-menu", screen.GetList(), true, false)
	a.app.SwitchToPage("monitoring-menu")
}

func (a *Application) showStatistics() {
	screen := monitoring.NewStatisticsScreen(a.app, a.statisticsStore)

	screen.SetOnBack(func() {
		a.app.HidePage("statistics")
		a.app.RemovePage("statistics")
		a.app.SwitchToPage("monitoring-menu")
	})

	a.app.AddPage("statistics", screen.GetFlex(), true, false)
	a.app.SwitchToPage("statistics")

	go func() {
		a.app.QueueUpdateDraw(func() {
			if err := screen.Load(context.Background()); err != nil {
				a.app.GetStatusBar().ShowError("Failed to load: " + err.Error())
			}
		})
	}()
}

func (a *Application) showSessionList() {
	screen := monitoring.NewSessionListScreen(a.app, a.sessionStore)

	screen.SetOnSelect(func(uuid string) {
		a.showSessionDetail()
	})

	screen.SetOnBack(func() {
		a.app.HidePage("session-list")
		a.app.RemovePage("session-list")
		a.app.SwitchToPage("monitoring-menu")
	})

	a.app.AddPage("session-list", screen.GetTable(), true, false)
	a.app.SwitchToPage("session-list")
	a.app.SetFocus(screen.GetTable())

	go func() {
		a.app.QueueUpdateDraw(func() {
			if err := screen.Load(context.Background()); err != nil {
				a.app.GetStatusBar().ShowError("Failed to load: " + err.Error())
			}
		})
	}()
}

func (a *Application) showSessionDetail() {
	screen := monitoring.NewSessionDetailScreen(a.app, a.sessionStore, a.auditLogger)

	screen.SetOnBack(func() {
		a.app.HidePage("session-detail")
		a.app.RemovePage("session-detail")
		a.app.SwitchToPage("session-list")
	})

	a.app.AddPage("session-detail", screen.GetFlex(), true, false)
	a.app.SwitchToPage("session-detail")

	// 検索ダイアログを表示
	screen.ShowSearchDialog()
}

// Helpers
func (a *Application) showDeleteConfirm(targetType, identifier string, onConfirm func()) {
	dialog := ui.NewConfirmDialog(
		"Confirm Delete",
		"Are you sure you want to delete this "+targetType+"?\n\n"+identifier,
		func() {
			a.app.HidePage("delete-confirm")
			a.app.RemovePage("delete-confirm")
			onConfirm()
		},
		func() {
			a.app.HidePage("delete-confirm")
			a.app.RemovePage("delete-confirm")
		},
	)

	a.app.AddPage("delete-confirm", dialog.GetModal(), true, true)
}

func (a *Application) cleanup() {
	if a.redisClient != nil {
		_ = a.redisClient.Close()
	}
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

func init() {
	// Disable mouse to prevent focus issues
	os.Setenv("TERM", os.Getenv("TERM"))
}
