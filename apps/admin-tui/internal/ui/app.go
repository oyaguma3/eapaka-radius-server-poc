// Package ui はTUIアプリケーションのUI層を提供する。
package ui

import (
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
)

// App はTUIアプリケーションを管理する。
type App struct {
	app       *tview.Application
	pages     *tview.Pages
	statusBar *StatusBar
	layout    *tview.Flex
}

// NewApp は新しいAppを生成する。
func NewApp() *App {
	app := tview.NewApplication()
	pages := tview.NewPages()
	statusBar := NewStatusBar()

	layout := tview.NewFlex().
		SetDirection(tview.FlexRow).
		AddItem(pages, 0, 1, true).
		AddItem(statusBar.view, 1, 0, false)

	return &App{
		app:       app,
		pages:     pages,
		statusBar: statusBar,
		layout:    layout,
	}
}

// Run はアプリケーションを実行する。
func (a *App) Run() error {
	return a.app.SetRoot(a.layout, true).EnableMouse(false).Run()
}

// Stop はアプリケーションを停止する。
func (a *App) Stop() {
	a.app.Stop()
}

// GetApplication は内部のtview.Applicationを返す。
func (a *App) GetApplication() *tview.Application {
	return a.app
}

// GetPages は内部のtview.Pagesを返す。
func (a *App) GetPages() *tview.Pages {
	return a.pages
}

// GetStatusBar はステータスバーを返す。
func (a *App) GetStatusBar() *StatusBar {
	return a.statusBar
}

// AddPage はページを追加する。
func (a *App) AddPage(name string, page tview.Primitive, resize, visible bool) {
	a.pages.AddPage(name, page, resize, visible)
}

// SwitchToPage は指定されたページに切り替える。
func (a *App) SwitchToPage(name string) {
	a.pages.SwitchToPage(name)
}

// ShowPage は指定されたページを表示する。
func (a *App) ShowPage(name string) {
	a.pages.ShowPage(name)
}

// HidePage は指定されたページを非表示にする。
func (a *App) HidePage(name string) {
	a.pages.HidePage(name)
}

// RemovePage はページを削除する。
func (a *App) RemovePage(name string) {
	a.pages.RemovePage(name)
}

// SetFocus はフォーカスを設定する。
func (a *App) SetFocus(p tview.Primitive) {
	a.app.SetFocus(p)
}

// QueueUpdateDraw はUIの更新をキューに追加する。
func (a *App) QueueUpdateDraw(f func()) {
	a.app.QueueUpdateDraw(f)
}

// SetInputCapture はグローバルなキー入力ハンドラを設定する。
func (a *App) SetInputCapture(capture func(event *tcell.EventKey) *tcell.EventKey) {
	a.app.SetInputCapture(capture)
}
