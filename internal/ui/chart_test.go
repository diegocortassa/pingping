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
