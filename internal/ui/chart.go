package ui

import (
	"image"
	"image/color"
	"image/draw"
	"math"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"github.com/superping/superping/internal/model"
)

// Chart geometry in device independent pixels.
const (
	chartLeftMargin   = 56.0
	chartRightMargin  = 10.0
	chartTopMargin    = 10.0
	chartBottomMargin = 26.0
	chartGridLines    = 4
)

// Chart plots the round trip time of a target over time.
type Chart struct {
	widget.BaseWidget

	mu      sync.RWMutex
	samples []model.Sample
	window  time.Duration
	maxRTT  time.Duration
	colors  chartColors

	raster *canvas.Raster
}

// chartColors holds the colours used to paint the plot.
type chartColors struct {
	background color.Color
	grid       color.Color
	line       color.Color
	failure    color.Color
}

// NewChart returns an empty chart.
func NewChart() *Chart {
	c := &Chart{window: time.Hour}
	c.raster = canvas.NewRaster(c.render)
	c.ExtendBaseWidget(c)
	c.updateTheme()
	return c
}

// SetSamples replaces the plotted data.
func (c *Chart) SetSamples(samples []model.Sample, window time.Duration) {
	c.mu.Lock()
	c.samples = samples
	if window > 0 {
		c.window = window
	}
	c.maxRTT = 0
	for _, s := range samples {
		if s.Success && s.RTT > c.maxRTT {
			c.maxRTT = s.RTT
		}
	}
	c.mu.Unlock()

	c.Refresh()
}

// CreateRenderer implements fyne.Widget.
func (c *Chart) CreateRenderer() fyne.WidgetRenderer {
	c.updateTheme()
	return newChartRenderer(c)
}

// updateTheme refreshes the colours used by the plot.
func (c *Chart) updateTheme() {
	t := c.Theme()
	if t == nil {
		t = theme.DefaultTheme()
	}
	variant := theme.VariantLight
	if current := fyne.CurrentApp(); current != nil {
		variant = current.Settings().ThemeVariant()
	}

	c.mu.Lock()
	c.colors = chartColors{
		background: t.Color(theme.ColorNameInputBackground, variant),
		grid:       t.Color(theme.ColorNameSeparator, variant),
		line:       t.Color(theme.ColorNamePrimary, variant),
		failure:    t.Color(theme.ColorNameError, variant),
	}
	c.mu.Unlock()
}

// render draws the plot area. Fyne calls it whenever the raster is repainted.
func (c *Chart) render(w, h int) image.Image {
	if w <= 0 || h <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}

	c.mu.RLock()
	samples := c.samples
	window := c.window
	maxRTT := c.maxRTT
	colors := c.colors
	c.mu.RUnlock()

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: colors.background}, image.Point{}, draw.Src)

	for i := 1; i < chartGridLines; i++ {
		x := int(float64(w) * float64(i) / chartGridLines)
		drawVLine(img, x, 0, h, colors.grid)
	}

	scale := niceMax(float64(maxRTT) / float64(time.Millisecond))
	if len(samples) == 0 || scale <= 0 {
		return img
	}

	start := time.Now().Add(-window)
	buckets := bucketSamples(samples, start, window, w, scale)

	prevX, prevY := -1, -1
	for x, b := range buckets {
		if b.hasOK {
			y := int(float64(h) - (b.avg/scale)*float64(h))
			if prevX >= 0 {
				drawLine(img, prevX, prevY, x, y, colors.line)
			}
			prevX, prevY = x, y
		}
		if b.hasFail {
			markLoss(img, x, h, colors.failure)
		}
		if !b.hasOK && b.hasFail {
			prevX, prevY = -1, -1
		}
	}
	return img
}

// bucket groups the samples of one pixel column.
type bucket struct {
	avg     float64
	hasOK   bool
	hasFail bool
}

// bucketSamples aggregates the samples per pixel column.
func bucketSamples(samples []model.Sample, start time.Time, window time.Duration, width int, scale float64) []bucket {
	if width <= 0 || window <= 0 {
		return nil
	}
	buckets := make([]bucket, width)
	sums := make([]float64, width)
	counts := make([]int, width)

	for _, s := range samples {
		offset := s.Time.Sub(start).Seconds()
		if offset < 0 {
			continue
		}
		x := int(offset / window.Seconds() * float64(width))
		if x < 0 || x >= width {
			continue
		}
		if !s.Success {
			buckets[x].hasFail = true
			continue
		}
		ms := float64(s.RTT) / float64(time.Millisecond)
		if ms > scale {
			ms = scale
		}
		sums[x] += ms
		counts[x]++
	}
	for x := range buckets {
		if counts[x] == 0 {
			continue
		}
		buckets[x].avg = sums[x] / float64(counts[x])
		buckets[x].hasOK = true
	}
	return buckets
}

// niceMax rounds a value up to a readable axis maximum.
func niceMax(value float64) float64 {
	if value <= 0 {
		return 1
	}
	exp := math.Floor(math.Log10(value))
	base := math.Pow(10, exp)
	for _, m := range []float64{1, 2, 5, 10} {
		if value <= m*base {
			return m * base
		}
	}
	return 10 * base
}

// drawLine draws a line with the given colour.
func drawLine(img *image.RGBA, x0, y0, x1, y1 int, c color.Color) {
	dx := absInt(x1 - x0)
	dy := -absInt(y1 - y0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx + dy
	for {
		plot(img, x0, y0, c)
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

// drawVLine draws a vertical line.
func drawVLine(img *image.RGBA, x, y0, y1 int, c color.Color) {
	for y := y0; y < y1; y++ {
		plot(img, x, y, c)
	}
}

// plot writes a single pixel.
func plot(img *image.RGBA, x, y int, c color.Color) {
	if x < 0 || y < 0 || x >= img.Bounds().Dx() || y >= img.Bounds().Dy() {
		return
	}
	r, g, b, a := c.RGBA()
	img.SetRGBA(x, y, color.RGBA{R: uint8(r >> 8), G: uint8(g >> 8), B: uint8(b >> 8), A: uint8(a >> 8)})
}

// markLoss draws the marker that shows a lost probe.
func markLoss(img *image.RGBA, x, h int, c color.Color) {
	for dy := 0; dy < 4; dy++ {
		for dx := -2; dx <= 2; dx++ {
			plot(img, x+dx, h-2-dy, c)
		}
	}
}

func absInt(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
