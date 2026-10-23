package ui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"

	"github.com/pingping/pingping/assets/icons"
	"github.com/pingping/pingping/internal/history"
	"github.com/pingping/pingping/internal/logstore"
	"github.com/pingping/pingping/internal/model"
)

// logTimeLayout is the layout of the times entered in a log window.
const logTimeLayout = "2006-01-02 15:04:05"

// logRangeOption is a quick time range of a log window, a zero duration means
// the whole file.
type logRangeOption struct {
	Label    string
	Duration time.Duration
}

// logRangeOptions are the quick ranges offered by a log window.
var logRangeOptions = []logRangeOption{
	{"Entire file", 0},
	{"Last 15 minutes", 15 * time.Minute},
	{"Last hour", time.Hour},
	{"Last 6 hours", 6 * time.Hour},
	{"Last 24 hours", 24 * time.Hour},
}

// logSeries is the data of a target inside a log file.
type logSeries struct {
	ID      string
	Label   string
	Samples []model.Sample
}

// openLog reads a log file and shows it in a new window.
func (a *App) openLog(path string) error {
	samples, err := logstore.Read(path)
	if err != nil {
		return err
	}
	if len(samples) == 0 {
		return fmt.Errorf("%s: no usable samples", filepath.Base(path))
	}

	window := &logWindow{
		app:    a,
		path:   path,
		series: groupSamples(samples),
	}
	window.show()
	return nil
}

// openStartupLog opens the log file passed with the -log flag.
func (a *App) openStartupLog() {
	if a.startupLog == "" {
		return
	}
	if err := a.openLog(a.startupLog); err != nil {
		dialog.ShowError(err, a.win)
	}
}

// logWindow shows the graph of an opened log file.
type logWindow struct {
	app    *App
	path   string
	series []logSeries

	selected int
	win      fyne.Window
	chart    *Chart
	summary  *widget.Label
	from, to *widget.Entry
	preset   *widget.Select
}

// show creates and shows the window.
func (lw *logWindow) show() {
	lw.win = lw.app.fyneApp.NewWindow("PingPing - " + filepath.Base(lw.path))
	lw.win.SetIcon(icons.App())
	lw.win.Resize(fyne.NewSize(1020, 640))

	lw.chart = NewChart()
	lw.summary = widget.NewLabel("")
	lw.summary.Wrapping = fyne.TextWrapWord

	lw.from = widget.NewEntry()
	lw.to = widget.NewEntry()
	lw.preset = widget.NewSelect(logRangeLabels(), func(label string) {
		lw.resetRange(label)
	})

	bar := container.NewHBox(
		widget.NewLabel("From"), lw.from,
		widget.NewLabel("To"), lw.to,
		widget.NewButton("Apply", lw.applyRange),
		widget.NewLabel("Range"), lw.preset,
	)
	if len(lw.series) > 1 {
		target := widget.NewSelect(logSeriesLabels(lw.series), func(label string) {
			lw.selected = indexOfSeries(lw.series, label)
			lw.resetRange(lw.preset.Selected)
		})
		target.SetSelected(lw.series[0].Label)
		bar.Add(widget.NewLabel("Target"))
		bar.Add(target)
	}

	lw.win.SetContent(container.NewBorder(bar, lw.summary, nil, nil, lw.chart))
	lw.preset.SetSelected(logRangeOptions[0].Label)
	lw.win.Show()
}

// resetRange fills the time entries with a quick range and applies it.
func (lw *logWindow) resetRange(label string) {
	samples := lw.series[lw.selected].Samples
	start := samples[0].Time
	end := samples[len(samples)-1].Time
	for _, option := range logRangeOptions {
		if option.Label != label || option.Duration <= 0 {
			continue
		}
		start = end.Add(-option.Duration)
		if start.Before(samples[0].Time) {
			start = samples[0].Time
		}
	}
	lw.from.SetText(start.In(time.Local).Format(logTimeLayout))
	lw.to.SetText(end.In(time.Local).Format(logTimeLayout))
	lw.applyRange()
}

// applyRange reads the time entries and shows the samples inside the range.
func (lw *logWindow) applyRange() {
	start, err := time.ParseInLocation(logTimeLayout, strings.TrimSpace(lw.from.Text), time.Local)
	if err != nil {
		dialog.ShowError(errors.New("the from time is not valid, use 2026-10-21 09:00:00"), lw.win)
		return
	}
	end, err := time.ParseInLocation(logTimeLayout, strings.TrimSpace(lw.to.Text), time.Local)
	if err != nil {
		dialog.ShowError(errors.New("the to time is not valid, use 2026-10-21 09:00:00"), lw.win)
		return
	}
	if !end.After(start) {
		dialog.ShowError(errors.New("the to time must be after the from time"), lw.win)
		return
	}

	samples := samplesBetween(lw.series[lw.selected].Samples, start, end)
	lw.chart.SetRange(samples, start, end)
	lw.summary.SetText(logSummary(samples, start, end))
}

// logSummary describes the samples of a log range.
func logSummary(samples []model.Sample, start, end time.Time) string {
	span := fmt.Sprintf("%s - %s", start.In(time.Local).Format(logTimeLayout), end.In(time.Local).Format(logTimeLayout))
	if len(samples) == 0 {
		return "No samples between " + span
	}

	stats := history.StatsOf(samples)
	return fmt.Sprintf("%s | Last %s | Average %s | Min %s | Max %s | Jitter %s | Loss %.1f%% | %d probes",
		span, rttText(stats), avgText(stats), durationText(stats.Min), durationText(stats.Max),
		durationText(stats.Jitter), stats.Loss, stats.Count)
}

// groupSamples splits the samples of a log file by target, keeping the order
// of the file. Files written before the per-target split can hold more than
// one target.
func groupSamples(samples []model.Sample) []logSeries {
	var series []logSeries
	index := make(map[string]int)

	for _, sample := range samples {
		key := sample.TargetID
		if key == "" {
			key = sample.Address
		}
		position, ok := index[key]
		if !ok {
			position = len(series)
			index[key] = position
			label := sample.Address
			if label == "" {
				label = key
			}
			series = append(series, logSeries{ID: key, Label: label})
		}
		series[position].Samples = append(series[position].Samples, sample)
	}

	counts := make(map[string]int, len(series))
	for _, s := range series {
		counts[s.Label]++
	}
	for i := range series {
		if counts[series[i].Label] > 1 {
			series[i].Label = fmt.Sprintf("%s (%s)", series[i].Label, series[i].ID)
		}
	}
	return series
}

// samplesBetween returns the samples inside a closed time range.
func samplesBetween(samples []model.Sample, start, end time.Time) []model.Sample {
	var out []model.Sample
	for _, sample := range samples {
		if sample.Time.Before(start) || sample.Time.After(end) {
			continue
		}
		out = append(out, sample)
	}
	return out
}

// logRangeLabels returns the labels of the quick ranges.
func logRangeLabels() []string {
	labels := make([]string, 0, len(logRangeOptions))
	for _, option := range logRangeOptions {
		labels = append(labels, option.Label)
	}
	return labels
}

// logSeriesLabels returns the labels of the targets of a log file.
func logSeriesLabels(series []logSeries) []string {
	labels := make([]string, 0, len(series))
	for _, s := range series {
		labels = append(labels, s.Label)
	}
	return labels
}

// indexOfSeries returns the position of a series label.
func indexOfSeries(series []logSeries, label string) int {
	for i, s := range series {
		if s.Label == label {
			return i
		}
	}
	return 0
}
