package ui

import (
	"strconv"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
)

// chartRenderer paints the plot area and the axis labels of a chart.
type chartRenderer struct {
	chart *Chart

	raster  *canvas.Raster
	yLabels []*canvas.Text
	xLabels []*canvas.Text
	objects []fyne.CanvasObject
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
	for i := 0; i <= chartGridLines; i++ {
		label := canvas.NewText("", theme.Color(theme.ColorNameForeground))
		label.TextSize = theme.Size(theme.SizeNameCaptionText)
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
	plotX := float32(chartLeftMargin)
	plotY := float32(chartTopMargin)
	plotW := size.Width - chartLeftMargin - chartRightMargin
	plotH := size.Height - chartTopMargin - chartBottomMargin
	if plotW < 1 {
		plotW = 1
	}
	if plotH < 1 {
		plotH = 1
	}

	r.raster.Resize(fyne.NewSize(plotW, plotH))
	r.raster.Move(fyne.NewPos(plotX, plotY))

	for i, label := range r.yLabels {
		fraction := float32(i) / float32(chartGridLines)
		y := plotY + plotH*(1-fraction) - label.MinSize().Height/2
		label.Move(fyne.NewPos(plotX-label.MinSize().Width-6, y))
	}

	for i, label := range r.xLabels {
		fraction := float32(i) / float32(chartGridLines)
		x := plotX + plotW*fraction - label.MinSize().Width/2
		label.Move(fyne.NewPos(x, plotY+plotH+4))
	}
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
	r.raster.Refresh()
	canvas.Refresh(r.raster)
}

// updateLabels recalculates the text of the axis labels.
func (r *chartRenderer) updateLabels() {
	c := r.chart

	c.mu.RLock()
	highest := c.maxRTT
	window := c.window
	end := c.end
	c.mu.RUnlock()

	scale := niceMax(float64(highest) / float64(time.Millisecond))
	for i, label := range r.yLabels {
		fraction := float64(i) / float64(chartGridLines)
		label.Text = formatMillis(scale * fraction)
	}

	if end.IsZero() {
		end = time.Now()
	}
	start := end.Add(-window)
	layout := axisTimeFormat(start, end)
	for i, label := range r.xLabels {
		fraction := float64(i) / float64(chartGridLines)
		at := start.Add(time.Duration(fraction * float64(window)))
		label.Text = at.In(time.Local).Format(layout)
	}
}

// axisTimeFormat returns the layout of the time labels. The date is added
// when the range does not start and end on the same day.
func axisTimeFormat(start, end time.Time) string {
	if start.Year() == end.Year() && start.YearDay() == end.YearDay() {
		return "15:04"
	}
	return "01-02 15:04"
}

// formatMillis formats a millisecond value for the axis labels.
func formatMillis(value float64) string {
	if value >= 10 {
		return strconv.FormatFloat(value, 'f', 0, 64)
	}
	return strconv.FormatFloat(value, 'f', 1, 64)
}
