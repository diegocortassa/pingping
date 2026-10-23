package ui

import (
	"os"
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"

	"github.com/pingping/pingping/internal/model"
)

func sampleAt(id, address string, at time.Time, rtt time.Duration) model.Sample {
	return model.Sample{TargetID: id, Address: address, Time: at, RTT: rtt, Success: true}
}

func TestGroupSamples(t *testing.T) {
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	samples := []model.Sample{
		sampleAt("t1", "1.1.1.1", at, time.Millisecond),
		sampleAt("t2", "8.8.8.8", at, time.Millisecond),
		sampleAt("t1", "1.1.1.1", at.Add(time.Second), time.Millisecond),
	}

	series := groupSamples(samples)
	if len(series) != 2 {
		t.Fatalf("got %d series, want 2", len(series))
	}
	if series[0].ID != "t1" || len(series[0].Samples) != 2 {
		t.Fatalf("unexpected first series %+v", series[0])
	}
	if series[1].ID != "t2" || len(series[1].Samples) != 1 {
		t.Fatalf("unexpected second series %+v", series[1])
	}
}

func TestGroupSamplesFallsBackToTheAddress(t *testing.T) {
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	series := groupSamples([]model.Sample{sampleAt("", "1.1.1.1", at, time.Millisecond)})
	if len(series) != 1 || series[0].ID != "1.1.1.1" || series[0].Label != "1.1.1.1" {
		t.Fatalf("unexpected series %+v", series)
	}
}

func TestGroupSamplesDisambiguatesTheLabels(t *testing.T) {
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	series := groupSamples([]model.Sample{
		sampleAt("t1", "1.1.1.1", at, time.Millisecond),
		sampleAt("t2", "1.1.1.1", at, time.Millisecond),
	})
	if series[0].Label == series[1].Label {
		t.Fatalf("the labels are the same: %q", series[0].Label)
	}
	if series[0].Label != "1.1.1.1 (t1)" || series[1].Label != "1.1.1.1 (t2)" {
		t.Fatalf("unexpected labels %q and %q", series[0].Label, series[1].Label)
	}
}

func TestSamplesBetween(t *testing.T) {
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	samples := []model.Sample{
		sampleAt("t1", "1.1.1.1", at.Add(-time.Hour), time.Millisecond),
		sampleAt("t1", "1.1.1.1", at, time.Millisecond),
		sampleAt("t1", "1.1.1.1", at.Add(time.Hour), time.Millisecond),
	}

	got := samplesBetween(samples, at, at.Add(time.Hour))
	if len(got) != 2 {
		t.Fatalf("got %d samples, want 2", len(got))
	}
}

func TestLogSummary(t *testing.T) {
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	if got := logSummary(nil, at, at.Add(time.Hour)); got == "" {
		t.Fatal("the summary of an empty range is empty")
	}

	stats := logSummary([]model.Sample{sampleAt("t1", "1.1.1.1", at, 10*time.Millisecond)}, at, at.Add(time.Hour))
	if stats == "" {
		t.Fatal("the summary is empty")
	}
}

func TestIndexOfSeries(t *testing.T) {
	series := []logSeries{{Label: "a"}, {Label: "b"}}
	if got := indexOfSeries(series, "b"); got != 1 {
		t.Fatalf("got %d, want 1", got)
	}
	if got := indexOfSeries(series, "missing"); got != 0 {
		t.Fatalf("got %d, want 0", got)
	}
}

func writeTestLog(t *testing.T, content string) string {
	t.Helper()
	path := t.TempDir() + "/pings-test.csv"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write log: %v", err)
	}
	return path
}

func TestOpenLogShowsAWindow(t *testing.T) {
	app, _ := newTestApp(t)
	before := len(app.fyneApp.Driver().AllWindows())

	path := writeTestLog(t, "timestamp,target_id,address,rtt_ms,success,error\n"+
		"2026-10-21T09:00:00Z,t1,1.1.1.1,12.000,true,\n")
	if err := app.openLog(path); err != nil {
		t.Fatalf("open log: %v", err)
	}

	if got := len(app.fyneApp.Driver().AllWindows()); got != before+1 {
		t.Fatalf("got %d windows, want %d", got, before+1)
	}
}

func TestOpenLogRejectsAnEmptyFile(t *testing.T) {
	app, _ := newTestApp(t)
	path := writeTestLog(t, "timestamp,target_id,address,rtt_ms,success,error\n")
	if err := app.openLog(path); err == nil {
		t.Fatal("expected an error for a log without samples")
	}
}

func TestLogWindowAppliesTheRange(t *testing.T) {
	app, _ := newTestApp(t)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	lw := &logWindow{
		app: app,
		win: app.win,
		series: groupSamples([]model.Sample{
			sampleAt("t1", "1.1.1.1", at.Add(-time.Minute), time.Millisecond),
			sampleAt("t1", "1.1.1.1", at, time.Millisecond),
			sampleAt("t1", "1.1.1.1", at.Add(time.Hour), time.Millisecond),
		}),
		chart:   NewChart(),
		summary: widget.NewLabel(""),
		from:    widget.NewEntry(),
		to:      widget.NewEntry(),
		preset:  widget.NewSelect(logRangeLabels(), nil),
	}
	lw.from.SetText(at.Add(-time.Second).In(time.Local).Format(logTimeLayout))
	lw.to.SetText(at.Add(time.Minute).In(time.Local).Format(logTimeLayout))
	lw.applyRange()

	start, end := lw.chart.bounds()
	if !start.Equal(at.Add(-time.Second)) || !end.Equal(at.Add(time.Minute)) {
		t.Fatalf("got the range %v - %v", start, end)
	}

	lw.chart.mu.RLock()
	count := len(lw.chart.samples)
	lw.chart.mu.RUnlock()
	if count != 1 {
		t.Fatalf("got %d samples in the range, want 1", count)
	}
	if lw.summary.Text == "" {
		t.Fatal("the summary is empty")
	}
}

func TestOpenLogDialogIsSizedToTheWindow(t *testing.T) {
	app, _ := newTestApp(t)
	app.openLogDialog()

	overlays := app.win.Canvas().Overlays().List()
	if len(overlays) != 1 {
		t.Fatalf("got %d overlays, want the file dialog", len(overlays))
	}
	canvas := app.win.Canvas().Size()
	want := fyne.NewSize(canvas.Width*0.8, canvas.Height*0.9)
	objects := overlays[0].(fyne.Widget).CreateRenderer().Objects()
	if got := objects[len(objects)-1].Size(); got != want {
		t.Fatalf("the dialog is %v, want %v (the window is %v)", got, want, canvas)
	}
}

func TestLogWindowGivesRoomToTheTimeFields(t *testing.T) {
	app, _ := newTestApp(t)
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	lw := &logWindow{
		app:  app,
		path: "pings-1.1.1.1-2026-10-21.csv",
		series: groupSamples([]model.Sample{
			sampleAt("t1", "1.1.1.1", at, time.Millisecond),
			sampleAt("t1", "1.1.1.1", at.Add(time.Minute), time.Millisecond),
		}),
	}
	lw.show()

	if lw.from.Size().Width < 200 {
		t.Fatalf("the from field is %v wide", lw.from.Size())
	}
	if lw.to.Size().Width < 200 {
		t.Fatalf("the to field is %v wide", lw.to.Size())
	}
}
