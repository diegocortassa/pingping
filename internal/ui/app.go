// Package ui implements the Fyne user interface of SuperPinger.
package ui

import (
	"fmt"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/superping/superping/assets/icons"
	"github.com/superping/superping/internal/config"
	"github.com/superping/superping/internal/history"
	"github.com/superping/superping/internal/logstore"
	"github.com/superping/superping/internal/model"
	"github.com/superping/superping/internal/monitor"
)

// Columns of the target table.
const (
	colAddress = iota
	colInterval
	colStatus
	colLast
	colAverage
	colLoss
	columnCount
)

// tableHeaders are the column titles of the target table.
var tableHeaders = []string{"Address", "Interval", "Status", "Last", "Average", "Loss"}

// App is the graphical user interface of SuperPinger.
type App struct {
	cfg     *config.Config
	cfgPath string
	mon     *monitor.Monitor
	hist    *history.History
	store   *logstore.Writer

	fyneApp fyne.App
	win     fyne.Window
	table   *widget.Table
	tabs    *container.AppTabs
	status  *widget.Label
	toolbar *fyne.Container

	rows        []config.Target
	selectedRow int
	charts      map[string]*Chart
	labels      map[string]*widget.Label
	stop        func()
}

// New returns the user interface.
func New(cfg *config.Config, cfgPath string, mon *monitor.Monitor, hist *history.History, store *logstore.Writer) *App {
	return &App{
		cfg:     cfg,
		cfgPath: cfgPath,
		mon:     mon,
		hist:    hist,
		store:   store,
		charts:  make(map[string]*Chart),
		labels:  make(map[string]*widget.Label),
	}
}

// Run starts the graphical user interface and blocks until the application is
// closed.
func (a *App) Run() {
	a.setup(app.NewWithID("com.superping.superping"))
	a.win.ShowAndRun()
	if a.stop != nil {
		a.stop()
	}
}

// setup builds the window, the widgets and the system tray.
func (a *App) setup(fyneApp fyne.App) {
	a.fyneApp = fyneApp
	a.fyneApp.SetIcon(icons.App())

	a.win = a.fyneApp.NewWindow("SuperPinger")
	a.win.SetIcon(icons.App())
	a.win.Resize(fyne.NewSize(1020, 640))
	a.win.SetMainMenu(fyne.NewMainMenu(a.mainMenu(), a.helpMenu()))
	a.win.SetCloseIntercept(func() { a.win.Hide() })

	a.buildContent()
	a.setupTray()
	a.rebuild()

	samples, unsubscribe := a.mon.Subscribe(256)
	a.stop = unsubscribe
	go func() {
		for s := range samples {
			a.onSample(s)
		}
	}()
}

// mainMenu returns the menu bar of the window.
func (a *App) mainMenu() *fyne.Menu {
	return fyne.NewMenu("SuperPinger",
		fyne.NewMenuItem("Add target", a.addTarget),
		fyne.NewMenuItem("Settings", a.showSettings),
		fyne.NewMenuItem("Quit", func() { a.fyneApp.Quit() }),
	)
}

// helpMenu returns the help menu of the window.
func (a *App) helpMenu() *fyne.Menu {
	return fyne.NewMenu("Help",
		fyne.NewMenuItem("About", a.showAbout),
	)
}

// setupTray installs the system tray icon and its menu.
func (a *App) setupTray() {
	desk, ok := a.fyneApp.(desktop.App)
	if !ok {
		return
	}
	desk.SetSystemTrayIcon(icons.Tray())
	desk.SetSystemTrayMenu(fyne.NewMenu("SuperPinger",
		fyne.NewMenuItem("Show", func() { a.win.Show() }),
		fyne.NewMenuItem("Hide", func() { a.win.Hide() }),
		fyne.NewMenuItem("Quit", func() { a.fyneApp.Quit() }),
	))
	desk.SetSystemTrayWindow(a.win)
}

// buildContent creates the widgets of the main window.
func (a *App) buildContent() {
	a.toolbar = container.NewHBox(
		widget.NewButtonWithIcon("Add", theme.ContentAddIcon(), a.addTarget),
		widget.NewButtonWithIcon("Edit", theme.DocumentCreateIcon(), a.editTarget),
		widget.NewButtonWithIcon("Remove", theme.DeleteIcon(), a.removeTarget),
	)

	a.table = widget.NewTable(
		func() (int, int) { return len(a.rows) + 1, columnCount },
		func() fyne.CanvasObject { return widget.NewLabel("template") },
		a.updateCell,
	)
	for i, width := range []float32{190, 80, 80, 90, 90, 70} {
		a.table.SetColumnWidth(i, width)
	}
	a.table.OnSelected = a.onRowSelected

	a.tabs = container.NewAppTabs()
	a.status = widget.NewLabel("")

	split := container.NewHSplit(a.table, a.tabs)
	a.win.SetContent(container.NewBorder(
		container.NewVBox(a.toolbar, widget.NewSeparator()),
		a.status,
		nil, nil,
		split,
	))
}

// rebuild refreshes the target list and the charts.
func (a *App) rebuild() {
	a.rows = a.cfg.Targets
	a.selectedRow = 0

	items := make([]*container.TabItem, 0, len(a.rows))
	a.charts = make(map[string]*Chart)
	a.labels = make(map[string]*widget.Label)

	for _, t := range a.rows {
		chart := NewChart()
		chart.SetSamples(a.hist.Samples(t.ID), a.cfg.GraphWindow.Duration())

		label := widget.NewLabel(a.statsText(t.ID))
		label.Wrapping = fyne.TextWrapWord
		a.charts[t.ID] = chart
		a.labels[t.ID] = label

		content := container.NewBorder(nil, label, nil, nil, chart)
		items = append(items, container.NewTabItem(t.Address, content))
	}
	a.tabs.SetItems(items)
	a.table.Refresh()
	a.updateStatus()
}

// updateCell fills a cell of the target table.
func (a *App) updateCell(id widget.TableCellID, obj fyne.CanvasObject) {
	label, ok := obj.(*widget.Label)
	if !ok {
		return
	}
	if id.Row == 0 {
		if id.Col < len(tableHeaders) {
			label.SetText(tableHeaders[id.Col])
			label.TextStyle = fyne.TextStyle{Bold: true}
		}
		return
	}
	if id.Row > len(a.rows) {
		return
	}

	target := a.rows[id.Row-1]
	stats := a.hist.Stats(target.ID)

	switch id.Col {
	case colAddress:
		label.SetText(target.Address)
		label.TextStyle = fyne.TextStyle{}
	case colInterval:
		label.SetText(target.Interval.String())
		label.TextStyle = fyne.TextStyle{}
	case colStatus:
		label.SetText(statusText(target, stats))
		label.TextStyle = fyne.TextStyle{}
	case colLast:
		label.SetText(rttText(stats))
		label.TextStyle = fyne.TextStyle{}
	case colAverage:
		label.SetText(avgText(stats))
		label.TextStyle = fyne.TextStyle{}
	case colLoss:
		label.SetText(lossText(stats))
		label.TextStyle = fyne.TextStyle{}
	}
}

// onRowSelected shows the chart of the selected target.
func (a *App) onRowSelected(id widget.TableCellID) {
	a.selectedRow = id.Row
	if id.Row == 0 || id.Row > len(a.rows) {
		return
	}
	a.tabs.SelectIndex(id.Row - 1)
}

// statsText returns the summary line of a target.
func (a *App) statsText(targetID string) string {
	stats := a.hist.Stats(targetID)
	if stats.Count == 0 {
		return "Waiting for the first probe..."
	}

	summary := fmt.Sprintf("Last %s | Average %s | Min %s | Max %s | Jitter %s | Loss %.1f%% | %d probes",
		rttText(stats), avgText(stats), durationText(stats.Min), durationText(stats.Max),
		durationText(stats.Jitter), stats.Loss, stats.Count)
	if stats.LastError != "" {
		summary += " | Error: " + stats.LastError
	}
	return summary
}

// updateStatus refreshes the status bar.
func (a *App) updateStatus() {
	path := a.store.Path()
	if path == "" {
		path = a.cfg.LogDir()
	}
	a.status.SetText(fmt.Sprintf("%d target(s) monitored | logs: %s", len(a.rows), path))
}

// statusText describes the state of a target.
func statusText(target config.Target, stats history.Stats) string {
	if !target.Enabled {
		return "paused"
	}
	if stats.Count == 0 {
		return "waiting"
	}
	if stats.LastOK {
		return "up"
	}
	if stats.LastOffline {
		return "error"
	}
	return "down"
}

// rttText formats the round trip time of the last probe.
func rttText(stats history.Stats) string {
	if stats.Count == 0 || !stats.LastOK {
		return "-"
	}
	return durationText(stats.Last)
}

// avgText formats the average round trip time.
func avgText(stats history.Stats) string {
	if stats.Count == 0 || stats.Avg == 0 {
		return "-"
	}
	return durationText(stats.Avg)
}

// lossText formats the packet loss.
func lossText(stats history.Stats) string {
	if stats.Count == 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", stats.Loss)
}

// durationText formats a duration with a sensible precision.
func durationText(d time.Duration) string {
	switch {
	case d <= 0:
		return "-"
	case d < time.Millisecond:
		return fmt.Sprintf("%dus", d.Microseconds())
	case d < time.Second:
		return fmt.Sprintf("%.1fms", float64(d)/float64(time.Millisecond))
	default:
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
}

// onSample updates the user interface with a new sample.
func (a *App) onSample(s model.Sample) {
	fyne.Do(func() {
		chart, ok := a.charts[s.TargetID]
		if !ok {
			return
		}
		chart.SetSamples(a.hist.Samples(s.TargetID), a.cfg.GraphWindow.Duration())
		if label, ok := a.labels[s.TargetID]; ok {
			label.SetText(a.statsText(s.TargetID))
		}
		a.table.Refresh()
	})
}
