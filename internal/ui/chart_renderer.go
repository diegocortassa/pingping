package ui

import (
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
)

// maxXTicks is the size of the pool of time labels of a chart.
const maxXTicks = 16

// axisSteps are the intervals used for the time ticks, from the shortest to
// the longest. A step is chosen so the labels do not overlap.
var axisSteps = []time.Duration{
	time.Second, 2 * time.Second, 5 * time.Second, 10 * time.Second,
	15 * time.Second, 30 * time.Second,
	time.Minute, 2 * time.Minute, 5 * time.Minute, 10 * time.Minute,
	15 * time.Minute, 30 * time.Minute,
	time.Hour, 2 * time.Hour, 3 * time.Hour, 6 * time.Hour,
	12 * time.Hour, 24 * time.Hour, 48 * time.Hour,
	7 * 24 * time.Hour, 14 * 24 * time.Hour, 30 * 24 * time.Hour,
}

// axisTick is a vertical grid line with its time and its position inside the
// plot, from 0 at the left edge to 1 at the right edge.
type axisTick struct {
	At       time.Time
	Fraction float64
}

// chartRenderer paints the plot area and the axis labels of a chart.
type chartRenderer struct {
	chart *Chart

	raster  *canvas.Raster
	yLabels []*canvas.Text
	xLabels []*canvas.Text
	objects []fyne.CanvasObject

	tickFractions []float64
	size          fyne.Size
	plotX, plotY  float32
	plotW, plotH  float32
}

// newChartRenderer builds the renderer of a chart.
func newChartRenderer(c *Chart) *chartRenderer {
	r := &chartRenderer{
		chart:  c,
		raster: c.raster,
	}
	for i := 0; i <= chartGridLines; i++ {
		label := canvas.NewText("0", theme.Color(theme.ColorNameForeground))
		label.TextSize = theme.Size(theme.SizeNameCaptionText)
		r.yLabels = append(r.yLabels, label)
	}
	for i := 0; i < maxXTicks; i++ {
		label := canvas.NewText("", theme.Color(theme.ColorNameForeground))
		label.TextSize = theme.Size(theme.SizeNameCaptionText)
		label.Hide()
		r.xLabels = append(r.xLabels, label)
	}

	r.objects = append(r.objects, r.raster)
	for _, l := range r.yLabels {
		r.objects = append(r.objects, l)
	}
	for _, l := range r.xLabels {
		r.objects = append(r.objects, l)
	}
	return r
}

// Layout implements fyne.WidgetRenderer.
func (r *chartRenderer) Layout(size fyne.Size) {
	r.size = size
	r.plotX = float32(chartLeftMargin)
	r.plotY = float32(chartTopMargin)
	r.plotW = size.Width - chartLeftMargin - chartRightMargin
	r.plotH = size.Height - chartTopMargin - chartBottomMargin
	if r.plotW < 1 {
		r.plotW = 1
	}
	if r.plotH < 1 {
		r.plotH = 1
	}

	r.raster.Resize(fyne.NewSize(r.plotW, r.plotH))
	r.raster.Move(fyne.NewPos(r.plotX, r.plotY))

	r.updateLabels()
	r.layoutLabels()
}

// MinSize implements fyne.WidgetRenderer.
func (r *chartRenderer) MinSize() fyne.Size {
	return fyne.NewSize(360, 220)
}

// Objects implements fyne.WidgetRenderer.
func (r *chartRenderer) Objects() []fyne.CanvasObject {
	return r.objects
}

// Destroy implements fyne.WidgetRenderer.
func (r *chartRenderer) Destroy() {
}

// Refresh implements fyne.WidgetRenderer.
func (r *chartRenderer) Refresh() {
	r.updateLabels()
	r.layoutLabels()
	r.raster.Refresh()
	canvas.Refresh(r.raster)
}

// updateLabels takes the time snapshot of the frame and recalculates the axis
// labels, the tick positions and the grid lines.
func (r *chartRenderer) updateLabels() {
	c := r.chart

	base := time.Now()
	c.mu.RLock()
	end := c.end
	window := c.window
	highest := c.maxRTT
	c.mu.RUnlock()
	if !end.IsZero() {
		base = end
	}
	c.mu.Lock()
	c.anchor = base
	c.mu.Unlock()

	scale := niceMax(float64(highest) / float64(time.Millisecond))
	for i, label := range r.yLabels {
		fraction := float64(i) / float64(chartGridLines)
		label.Text = formatMillis(scale * fraction)
	}

	start := base.Add(-window)
	layout := axisTimeFormat(start, base)
	ticks := axisTicks(start, base, axisStep(window, r.plotW, layout))
	if len(ticks) > len(r.xLabels) {
		ticks = ticks[:len(r.xLabels)]
	}

	r.tickFractions = r.tickFractions[:0]
	grid := make([]float64, 0, len(ticks))
	for i, label := range r.xLabels {
		if i >= len(ticks) {
			label.Hide()
			continue
		}
		label.Text = ticks[i].At.In(time.Local).Format(layout)
		r.tickFractions = append(r.tickFractions, ticks[i].Fraction)
		grid = append(grid, ticks[i].Fraction)
	}

	c.mu.Lock()
	c.grid = grid
	c.mu.Unlock()
}

// layoutLabels places the axis labels around the plot area.
func (r *chartRenderer) layoutLabels() {
	for i, label := range r.yLabels {
		fraction := float32(i) / float32(chartGridLines)
		y := r.plotY + r.plotH*(1-fraction) - label.MinSize().Height/2
		label.Move(fyne.NewPos(r.plotX-label.MinSize().Width-6, y))
	}

	for i, label := range r.xLabels {
		if i >= len(r.tickFractions) {
			label.Hide()
			continue
		}
		width := label.MinSize().Width
		x := r.plotX + r.plotW*float32(r.tickFractions[i]) - width/2
		if max := r.size.Width - width - 2; x > max {
			x = max
		}
		if x < 2 {
			x = 2
		}
		label.Move(fyne.NewPos(x, r.plotY+r.plotH+4))
		label.Show()
	}
}

// axisTimeFormat returns the layout of the time labels. The date is added
// when the range does not start and end on the same day, the seconds when the
// range is short.
func axisTimeFormat(start, end time.Time) string {
	if start.Year() != end.Year() || start.YearDay() != end.YearDay() {
		return "01-02 15:04"
	}
	if end.Sub(start) < 10*time.Minute {
		return "15:04:05"
	}
	return "15:04"
}

// axisStep returns the interval of the time ticks. It is the smallest interval
// that keeps the labels apart on the given plot width and, at the same time,
// limits their number.
func axisStep(window time.Duration, plotWidth float32, layout string) time.Duration {
	if window <= 0 {
		return time.Minute
	}
	if plotWidth <= 0 {
		plotWidth = 600
	}

	gap := float32(60)
	minimum := time.Minute
	switch layout {
	case "01-02 15:04":
		gap = 110
	case "15:04:05":
		minimum = time.Second
	}

	step := time.Duration(float64(window) * float64(gap) / float64(plotWidth))
	if max := window / 10; step < max {
		step = max
	}
	if step < minimum {
		step = minimum
	}
	for _, candidate := range axisSteps {
		if candidate >= step {
			return candidate
		}
	}
	return axisSteps[len(axisSteps)-1]
}

// axisTicks returns the ticks of the range, aligned to the step and sorted by
// time.
func axisTicks(start, end time.Time, step time.Duration) []axisTick {
	window := end.Sub(start)
	if window <= 0 || step <= 0 {
		return nil
	}

	var ticks []axisTick
	for at := start.Truncate(step); at.Before(end); at = at.Add(step) {
		if at.Before(start) {
			continue
		}
		ticks = append(ticks, axisTick{
			At:       at,
			Fraction: float64(at.Sub(start)) / float64(window),
		})
	}
	return ticks
}

// formatMillis formats a millisecond value for the axis labels.
func formatMillis(value float64) string {
	if value >= 10 {
		return strconv.FormatFloat(value, 'f', 0, 64)
	}
	return strconv.FormatFloat(value, 'f', 1, 64)
}
