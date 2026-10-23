package ui

import (
	"testing"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/test"

	"github.com/pingping/pingping/internal/model"
)

func TestNiceMax(t *testing.T) {
	tests := []struct {
		value float64
		want  float64
	}{
		{0, 1},
		{0.4, 0.5},
		{1, 1},
		{1.5, 2},
		{7, 10},
		{12, 20},
		{42, 50},
		{130, 200},
	}
	for _, testCase := range tests {
		if got := niceMax(testCase.value); got != testCase.want {
			t.Fatalf("niceMax(%v) = %v, want %v", testCase.value, got, testCase.want)
		}
	}
}

func TestBucketSamples(t *testing.T) {
	start := time.Now()
	window := time.Minute
	samples := []model.Sample{
		{TargetID: "t1", Time: start, RTT: 10 * time.Millisecond, Success: true},
		{TargetID: "t1", Time: start, RTT: 30 * time.Millisecond, Success: true},
		{TargetID: "t1", Time: start, RTT: 0, Success: false},
		{TargetID: "t1", Time: start.Add(-time.Hour), RTT: 50 * time.Millisecond, Success: true},
	}

	buckets := bucketSamples(samples, start, window, 10, 100)
	if len(buckets) != 10 {
		t.Fatalf("got %d buckets, want 10", len(buckets))
	}
	if !buckets[0].hasOK {
		t.Fatal("expected the first bucket to hold a sample")
	}
	if buckets[0].avg != 20 {
		t.Fatalf("got average %v, want 20", buckets[0].avg)
	}
	if !buckets[0].hasFail {
		t.Fatal("expected the loss to be recorded")
	}
}

func TestChartRendersSamples(t *testing.T) {
	test.NewApp()

	chart := NewChart()
	now := time.Now()
	chart.SetSamples([]model.Sample{
		{TargetID: "t1", Time: now.Add(-10 * time.Second), RTT: 10 * time.Millisecond, Success: true},
		{TargetID: "t1", Time: now, RTT: 20 * time.Millisecond, Success: true},
	}, time.Minute)

	img := chart.render(200, 100)
	if img.Bounds().Dx() != 200 || img.Bounds().Dy() != 100 {
		t.Fatalf("unexpected size %v", img.Bounds())
	}
}

func TestChartWithoutSamples(t *testing.T) {
	test.NewApp()

	chart := NewChart()
	img := chart.render(120, 60)
	if img.Bounds().Dx() != 120 {
		t.Fatalf("unexpected size %v", img.Bounds())
	}
}

func TestChartWidgetCanBeCreated(t *testing.T) {
	test.NewApp()

	chart := NewChart()
	window := test.NewWindow(chart)
	defer window.Close()
	window.Resize(fyne.NewSize(640, 400))

	chart.SetSamples([]model.Sample{{TargetID: "t1", Time: time.Now(), RTT: time.Millisecond, Success: true}}, time.Minute)
}

func TestChartSetRangeFixesTheTimeBounds(t *testing.T) {
	test.NewApp()
	chart := NewChart()

	start := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	end := start.Add(2 * time.Hour)
	if chart.isLive() != true {
		t.Fatal("a new chart should be live")
	}

	chart.SetRange(nil, start, end)
	if chart.isLive() {
		t.Fatal("a ranged chart should not be live")
	}

	gotStart, gotEnd := chart.bounds()
	if !gotStart.Equal(start) || !gotEnd.Equal(end) {
		t.Fatalf("got %v - %v, want %v - %v", gotStart, gotEnd, start, end)
	}

	chart.SetSamples(nil, time.Hour)
	if !chart.isLive() {
		t.Fatal("SetSamples should switch back to the live mode")
	}
}

func TestChartSetRangeRejectsAnEmptyRange(t *testing.T) {
	test.NewApp()
	chart := NewChart()

	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	chart.SetRange(nil, at, at)

	start, end := chart.bounds()
	if !end.After(start) {
		t.Fatalf("got an empty range %v - %v", start, end)
	}
}

func TestAxisTimeFormat(t *testing.T) {
	day := time.Date(2026, 10, 21, 0, 0, 0, 0, time.UTC)
	if got := axisTimeFormat(day.Add(9*time.Hour), day.Add(10*time.Hour)); got != "15:04" {
		t.Fatalf("got %q, want the time only layout", got)
	}
	if got := axisTimeFormat(day.Add(23*time.Hour), day.Add(25*time.Hour)); got != "01-02 15:04" {
		t.Fatalf("got %q, want the date and time layout", got)
	}
}

func TestAxisTimeFormatAddsTheSecondsToShortRanges(t *testing.T) {
	at := time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)
	if got := axisTimeFormat(at, at.Add(5*time.Minute)); got != "15:04:05" {
		t.Fatalf("got %q, want the seconds layout", got)
	}
}

func TestAxisStepKeepsTheLabelsApart(t *testing.T) {
	tests := []struct {
		window time.Duration
		width  float32
		layout string
		want   time.Duration
	}{
		{time.Hour, 574, "15:04", 10 * time.Minute},
		{24 * time.Hour, 900, "01-02 15:04", 3 * time.Hour},
		{15 * time.Minute, 574, "15:04", 2 * time.Minute},
		{time.Minute, 574, "15:04:05", 10 * time.Second},
	}
	for _, testCase := range tests {
		if got := axisStep(testCase.window, testCase.width, testCase.layout); got != testCase.want {
			t.Fatalf("axisStep(%v, %v) = %v, want %v", testCase.window, testCase.width, got, testCase.want)
		}
	}
}

func TestAxisTicksAlignToTheStep(t *testing.T) {
	start := time.Date(2026, 10, 21, 8, 3, 0, 0, time.UTC)
	end := start.Add(time.Hour)

	ticks := axisTicks(start, end, 10*time.Minute)
	if len(ticks) != 6 {
		t.Fatalf("got %d ticks, want 6", len(ticks))
	}
	first := time.Date(2026, 10, 21, 8, 10, 0, 0, time.UTC)
	if !ticks[0].At.Equal(first) {
		t.Fatalf("the first tick is %v, want %v", ticks[0].At, first)
	}
	if got, want := ticks[0].Fraction, 7.0/60.0; got != want {
		t.Fatalf("the first fraction is %v, want %v", got, want)
	}
	if !ticks[len(ticks)-1].At.Equal(time.Date(2026, 10, 21, 9, 0, 0, 0, time.UTC)) {
		t.Fatalf("the last tick is %v", ticks[len(ticks)-1].At)
	}
	for i := 1; i < len(ticks); i++ {
		if !ticks[i].At.After(ticks[i-1].At) {
			t.Fatalf("the ticks are not sorted: %v", ticks)
		}
	}
}

func TestChartGridFollowsTheTimeTicks(t *testing.T) {
	test.NewApp()

	chart := NewChart()
	window := test.NewWindow(chart)
	defer window.Close()
	window.Resize(fyne.NewSize(640, 400))

	start := time.Date(2026, 10, 21, 9, 3, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	chart.SetRange(nil, start, end)

	chart.mu.RLock()
	grid := append([]float64(nil), chart.grid...)
	chart.mu.RUnlock()
	if len(grid) < 3 {
		t.Fatalf("got %d grid lines, want at least 3", len(grid))
	}
	for i, fraction := range grid {
		if fraction < 0 || fraction >= 1 {
			t.Fatalf("the grid fraction %v is outside the plot", fraction)
		}
		if i > 0 && fraction <= grid[i-1] {
			t.Fatalf("the grid fractions are not sorted: %v", grid)
		}
		at := start.Add(time.Duration(fraction * float64(end.Sub(start))))
		if at.Second() != 0 {
			t.Fatalf("the tick %v is not aligned to a whole minute", at)
		}
	}
}

func TestChartLabelsStayInsideThePlot(t *testing.T) {
	test.NewApp()

	chart := NewChart()
	start := time.Date(2026, 10, 21, 9, 3, 0, 0, time.UTC)
	chart.SetRange(nil, start, start.Add(time.Hour))

	r := newChartRenderer(chart)
	r.Layout(fyne.NewSize(640, 400))

	count := 0
	var previousRight float32
	for _, label := range r.xLabels {
		if !label.Visible() {
			continue
		}
		width := label.MinSize().Width
		x := label.Position().X
		if x < 0 || x+width > 640 {
			t.Fatalf("the label %q is outside the chart at %v", label.Text, label.Position())
		}
		if count > 0 && x < previousRight {
			t.Fatalf("the label %q overlaps the previous one at %v", label.Text, label.Position())
		}
		previousRight = x + width
		count++
	}
	if count < 3 {
		t.Fatalf("got %d labels, want at least 3", count)
	}
	if count != len(r.tickFractions) {
		t.Fatalf("got %d labels and %d ticks", count, len(r.tickFractions))
	}
}

func TestChartTicksSlideWithTheRange(t *testing.T) {
	test.NewApp()

	chart := NewChart()
	start := time.Date(2026, 10, 21, 9, 3, 0, 0, time.UTC)
	end := start.Add(time.Hour)
	chart.SetRange(nil, start, end)

	r := newChartRenderer(chart)
	r.Layout(fyne.NewSize(640, 400))

	r.updateLabels()
	r.layoutLabels()
	first, second := r.tickFractions[0], r.xLabels[0].Position().X

	chart.SetRange(nil, start.Add(2*time.Second), end.Add(2*time.Second))
	r.updateLabels()
	r.layoutLabels()

	if r.tickFractions[0] >= first {
		t.Fatalf("the tick did not slide: %v >= %v", r.tickFractions[0], first)
	}
	if r.xLabels[0].Position().X >= second {
		t.Fatalf("the label did not slide: %v >= %v", r.xLabels[0].Position().X, second)
	}
}
