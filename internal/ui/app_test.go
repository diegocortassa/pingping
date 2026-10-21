package ui

import (
	"strings"
	"testing"
	"time"

	"fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"

	"github.com/superping/superping/internal/config"
	"github.com/superping/superping/internal/history"
	"github.com/superping/superping/internal/logstore"
	"github.com/superping/superping/internal/model"
	"github.com/superping/superping/internal/monitor"
)

func newTestApp(t *testing.T, targets ...config.Target) (*App, *history.History) {
	t.Helper()
	test.NewApp()

	dir := t.TempDir()
	cfg := &config.Config{
		Targets:          targets,
		GraphWindow:      config.Duration(time.Hour),
		LogRetentionDays: 30,
		DataDir:          dir,
	}
	hist := history.New(cfg.GraphWindow.Duration())
	store, err := logstore.New(dir, 0)
	if err != nil {
		t.Fatalf("logstore: %v", err)
	}
	t.Cleanup(func() { store.Close() })

	mon := monitor.New(monitor.WithPingerFactory(func(config.Target, config.ICMode) monitor.Pinger {
		return nil
	}))

	ui := New(cfg, dir+"/config.json", mon, hist, store)
	ui.setup(test.NewApp())
	return ui, hist
}

func target(id, address string) config.Target {
	return config.Target{
		ID:       id,
		Address:  address,
		Interval: config.Duration(time.Second),
		Timeout:  config.Duration(time.Second),
		Enabled:  true,
	}
}

func TestAppBuildsTheTargetTable(t *testing.T) {
	app, _ := newTestApp(t, target("t1", "1.1.1.1"), target("t2", "8.8.8.8"))

	rows, cols := app.table.Length()
	if rows != 3 {
		t.Fatalf("got %d rows, want 3", rows)
	}
	if cols != columnCount {
		t.Fatalf("got %d columns, want %d", cols, columnCount)
	}
}

func TestAppShowsTheStatsOfATarget(t *testing.T) {
	app, hist := newTestApp(t, target("t1", "1.1.1.1"))

	hist.Add(model.Sample{TargetID: "t1", Address: "1.1.1.1", Time: time.Now(), RTT: 12 * time.Millisecond, Success: true})
	app.onSample(model.Sample{TargetID: "t1", Address: "1.1.1.1", Time: time.Now(), RTT: 12 * time.Millisecond, Success: true})

	label := app.labels["t1"]
	if label == nil {
		t.Fatal("the summary label is missing")
	}
	if got := label.Text; got == "" || got == "Waiting for the first probe..." {
		t.Fatalf("unexpected summary %q", got)
	}
}

func TestAppStatusText(t *testing.T) {
	app, hist := newTestApp(t, target("t1", "1.1.1.1"))

	hist.Add(model.Sample{TargetID: "t1", Time: time.Now(), RTT: 12 * time.Millisecond, Success: true})
	app.onSample(model.Sample{TargetID: "t1", Time: time.Now(), RTT: 12 * time.Millisecond, Success: true})

	if got := statusText(app.rows[0], hist.Stats("t1")); got != "up" {
		t.Fatalf("got %q, want up", got)
	}
}

func TestAppShowsTheErrorOfATarget(t *testing.T) {
	app, hist := newTestApp(t, target("t1", "1.1.1.1"))

	hist.Add(model.Sample{TargetID: "t1", Time: time.Now(), Offline: true, Error: "socket: permission denied"})
	app.onSample(model.Sample{TargetID: "t1", Time: time.Now(), Offline: true, Error: "socket: permission denied"})

	if got := statusText(app.rows[0], hist.Stats("t1")); got != "error" {
		t.Fatalf("got status %q, want error", got)
	}
	if got := app.labels["t1"].Text; !strings.Contains(got, "socket: permission denied") {
		t.Fatalf("the error is missing in %q", got)
	}
}

func TestAppRowSelection(t *testing.T) {
	app, _ := newTestApp(t, target("t1", "1.1.1.1"), target("t2", "8.8.8.8"))

	app.onRowSelected(widget.TableCellID{Row: 2, Col: 0})
	if app.selectedRow != 2 {
		t.Fatalf("got row %d, want 2", app.selectedRow)
	}
}

func TestAppOnSampleIgnoresUnknownTargets(t *testing.T) {
	app, _ := newTestApp(t, target("t1", "1.1.1.1"))
	app.onSample(model.Sample{TargetID: "unknown", Time: time.Now(), Success: true})
}

func TestAppChartsFollowTheSamples(t *testing.T) {
	app, hist := newTestApp(t, target("t1", "1.1.1.1"))

	hist.Add(model.Sample{TargetID: "t1", Time: time.Now(), RTT: 12 * time.Millisecond, Success: true})
	app.onSample(model.Sample{TargetID: "t1", Time: time.Now(), RTT: 12 * time.Millisecond, Success: true})

	chart := app.charts["t1"]
	if chart == nil {
		t.Fatal("the chart is missing")
	}
	if got := len(chart.samples); got != 1 {
		t.Fatalf("got %d samples, want 1", got)
	}
}
