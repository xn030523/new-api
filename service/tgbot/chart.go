package tgbot

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// Dashboard canvas geometry.
const (
	chartWidth      = 1000
	chartHeight     = 620
	chartPadding    = 32
	chartHeaderH    = 96
	chartPanelGap   = 20
	chartBarHeight  = 26
	chartBarGap     = 10
	chartMaxBars    = 8
	chartCornerSize = 6
)

var (
	colorBackground = color.RGBA{0x12, 0x16, 0x21, 0xff}
	colorPanel      = color.RGBA{0x1b, 0x21, 0x30, 0xff}
	colorTextBright = color.RGBA{0xf2, 0xf5, 0xfa, 0xff}
	colorTextMuted  = color.RGBA{0x93, 0x9f, 0xb4, 0xff}
	colorAccent     = color.RGBA{0x4d, 0xa3, 0xff, 0xff}
	colorAccentWarm = color.RGBA{0xff, 0xa5, 0x4d, 0xff}
	colorDanger     = color.RGBA{0xff, 0x6b, 0x6b, 0xff}
	colorBarTrack   = color.RGBA{0x27, 0x2f, 0x42, 0xff}

	// Bar palette cycles per row so adjacent bars stay distinguishable.
	barPalette = []color.RGBA{
		{0x4d, 0xa3, 0xff, 0xff},
		{0x4d, 0xd8, 0xb0, 0xff},
		{0xff, 0xa5, 0x4d, 0xff},
		{0xb4, 0x8b, 0xff, 0xff},
		{0xff, 0x8f, 0xb3, 0xff},
		{0x6f, 0xd5, 0xff, 0xff},
		{0xd8, 0xd0, 0x5a, 0xff},
		{0x8d, 0xe0, 0x6b, 0xff},
	}
)

// Faces are parsed once; opentype.Face is not safe for concurrent use, so the
// renderer holds its own drawer and callers must not share faces across goroutines.
type chartFonts struct {
	title  font.Face
	label  font.Face
	value  font.Face
	metric font.Face
}

// newChartFonts builds the face set. The embedded Go fonts carry no CJK
// glyphs, so a custom fontPath (any TTF/OTF reachable by the process) replaces
// both weights when supplied; failure to load it degrades to the Go fonts
// rather than dropping the chart.
func newChartFonts(fontPath string) (*chartFonts, error) {
	bold, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, fmt.Errorf("tgbot: parse bold font: %w", err)
	}
	regular, err := opentype.Parse(goregular.TTF)
	if err != nil {
		return nil, fmt.Errorf("tgbot: parse regular font: %w", err)
	}
	if fontPath != "" {
		if raw, readErr := os.ReadFile(fontPath); readErr != nil {
			common.SysError("tgbot: read chart font failed, using built-in font: " + readErr.Error())
		} else if custom, parseErr := opentype.Parse(raw); parseErr != nil {
			common.SysError("tgbot: parse chart font failed, using built-in font: " + parseErr.Error())
		} else {
			bold, regular = custom, custom
		}
	}
	newFace := func(f *opentype.Font, size float64) (font.Face, error) {
		return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	}
	title, err := newFace(bold, 22)
	if err != nil {
		return nil, err
	}
	label, err := newFace(regular, 13)
	if err != nil {
		return nil, err
	}
	value, err := newFace(bold, 13)
	if err != nil {
		return nil, err
	}
	metric, err := newFace(bold, 34)
	if err != nil {
		return nil, err
	}
	return &chartFonts{title: title, label: label, value: value, metric: metric}, nil
}

func (f *chartFonts) close() {
	for _, face := range []font.Face{f.title, f.label, f.value, f.metric} {
		if face != nil {
			_ = face.Close()
		}
	}
}

// RenderDashboard draws the monitoring dashboard as a PNG.
func RenderDashboard(stats *Stats, settings *MonitorSettings, title string) ([]byte, error) {
	if stats == nil {
		return nil, fmt.Errorf("tgbot: nil stats")
	}
	fontPath := ""
	if settings != nil {
		fontPath = settings.ChartFontPath
	}
	fonts, err := newChartFonts(fontPath)
	if err != nil {
		return nil, err
	}
	defer fonts.close()

	canvas := image.NewRGBA(image.Rect(0, 0, chartWidth, chartHeight))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{colorBackground}, image.Point{}, draw.Src)

	symbol := "$"
	quotaPerUnit := 500000.0
	maskUsername := false
	if settings != nil {
		if settings.CurrencySymbol != "" {
			symbol = settings.CurrencySymbol
		}
		if settings.QuotaPerUnit > 0 {
			quotaPerUnit = float64(settings.QuotaPerUnit)
		}
		maskUsername = settings.MaskUsername
	}

	drawText(canvas, fonts.title, colorTextBright, chartPadding, chartPadding+22, title)

	metricsTop := chartPadding + 40
	metricW := (chartWidth - 2*chartPadding - 2*chartPanelGap) / 3
	drawMetricCard(canvas, fonts, chartPadding, metricsTop, metricW, chartHeaderH, "RPM", formatCount(stats.RPM), colorAccent)
	drawMetricCard(canvas, fonts, chartPadding+metricW+chartPanelGap, metricsTop, metricW, chartHeaderH, "TPM", formatCount(stats.TPM), colorAccentWarm)
	errColor := colorAccent
	if stats.ErrorCount > 0 {
		errColor = colorDanger
	}
	drawMetricCard(canvas, fonts, chartPadding+2*(metricW+chartPanelGap), metricsTop, metricW, chartHeaderH,
		"ERRORS / MIN", formatCount(stats.ErrorCount), errColor)

	panelsTop := metricsTop + chartHeaderH + chartPanelGap
	panelH := chartHeight - panelsTop - chartPadding
	panelW := (chartWidth - 2*chartPadding - chartPanelGap) / 2

	todayBars := make([]barDatum, 0, chartMaxBars)
	for _, row := range stats.TodaySpend {
		if len(todayBars) == chartMaxBars {
			break
		}
		name := row.Username
		if maskUsername {
			name = MaskName(name)
		}
		todayBars = append(todayBars, barDatum{
			label: name,
			value: float64(row.Quota) / quotaPerUnit,
		})
	}
	drawBarPanel(canvas, fonts, chartPadding, panelsTop, panelW, panelH, "TODAY SPEND / TOP USERS", todayBars, symbol)

	remarkBars := make([]barDatum, 0, chartMaxBars)
	for _, row := range stats.AllTimeSpend {
		if len(remarkBars) == chartMaxBars {
			break
		}
		label := row.Remark
		if label == "" {
			label = "(no remark)"
		}
		remarkBars = append(remarkBars, barDatum{label: label, value: float64(row.Quota) / quotaPerUnit})
	}
	drawBarPanel(canvas, fonts, chartPadding+panelW+chartPanelGap, panelsTop, panelW, panelH, "ALL-TIME SPEND / GROUP", remarkBars, symbol)

	var out bytes.Buffer
	if err := png.Encode(&out, canvas); err != nil {
		return nil, fmt.Errorf("tgbot: encode png: %w", err)
	}
	return out.Bytes(), nil
}

type barDatum struct {
	label string
	value float64
}

func drawMetricCard(dst *image.RGBA, fonts *chartFonts, x, y, w, h int, label, value string, accent color.RGBA) {
	fillPanel(dst, x, y, w, h)
	// Accent stripe on the left edge marks the metric category.
	draw.Draw(dst, image.Rect(x, y+chartCornerSize, x+4, y+h-chartCornerSize), &image.Uniform{accent}, image.Point{}, draw.Src)
	drawText(dst, fonts.label, colorTextMuted, x+20, y+26, label)
	drawText(dst, fonts.metric, colorTextBright, x+20, y+h-22, value)
}

func drawBarPanel(dst *image.RGBA, fonts *chartFonts, x, y, w, h int, title string, bars []barDatum, symbol string) {
	fillPanel(dst, x, y, w, h)
	drawText(dst, fonts.value, colorTextMuted, x+20, y+26, title)

	if len(bars) == 0 {
		drawText(dst, fonts.label, colorTextMuted, x+20, y+56, "no data")
		return
	}

	maxValue := 0.0
	for _, bar := range bars {
		maxValue = math.Max(maxValue, bar.value)
	}
	if maxValue <= 0 {
		maxValue = 1
	}

	labelW := 150
	trackX := x + 20 + labelW
	trackW := w - 40 - labelW - 90
	if trackW < 40 {
		trackW = 40
	}

	rowY := y + 46
	for i, bar := range bars {
		if rowY+chartBarHeight > y+h-8 {
			break
		}
		drawText(dst, fonts.label, colorTextMuted, x+20, rowY+chartBarHeight-8, truncateLabel(bar.label, 16))
		draw.Draw(dst, image.Rect(trackX, rowY+4, trackX+trackW, rowY+chartBarHeight-4),
			&image.Uniform{colorBarTrack}, image.Point{}, draw.Src)
		filled := int(float64(trackW) * bar.value / maxValue)
		if filled < 2 && bar.value > 0 {
			filled = 2
		}
		if filled > 0 {
			draw.Draw(dst, image.Rect(trackX, rowY+4, trackX+filled, rowY+chartBarHeight-4),
				&image.Uniform{barPalette[i%len(barPalette)]}, image.Point{}, draw.Src)
		}
		drawText(dst, fonts.value, colorTextBright, trackX+trackW+12, rowY+chartBarHeight-8,
			fmt.Sprintf("%s%.2f", symbol, bar.value))
		rowY += chartBarHeight + chartBarGap
	}
}

// fillPanel paints a card background with the corner pixels trimmed so the
// rectangle reads as rounded without a full anti-aliasing pass.
func fillPanel(dst *image.RGBA, x, y, w, h int) {
	draw.Draw(dst, image.Rect(x, y, x+w, y+h), &image.Uniform{colorPanel}, image.Point{}, draw.Src)
	for i := range chartCornerSize {
		trim := chartCornerSize - i
		for _, px := range [][2]int{
			{x, y + i}, {x + w - trim, y + i},
			{x, y + h - 1 - i}, {x + w - trim, y + h - 1 - i},
		} {
			draw.Draw(dst, image.Rect(px[0], px[1], px[0]+trim, px[1]+1),
				&image.Uniform{colorBackground}, image.Point{}, draw.Src)
		}
	}
}

func drawText(dst *image.RGBA, face font.Face, col color.RGBA, x, y int, text string) {
	drawer := &font.Drawer{
		Dst:  dst,
		Src:  &image.Uniform{col},
		Face: face,
		Dot:  fixed.P(x, y),
	}
	drawer.DrawString(substituteMissingGlyphs(face, text))
}

// substituteMissingGlyphs replaces runes the face cannot render with '?'.
// Without this, CJK usernames drawn with the glyph-less built-in Go font would
// silently disappear instead of showing that a CJK-capable font is needed.
func substituteMissingGlyphs(face font.Face, text string) string {
	missing := false
	for _, r := range text {
		if _, ok := face.GlyphAdvance(r); !ok {
			missing = true
			break
		}
	}
	if !missing {
		return text
	}
	var sb strings.Builder
	for _, r := range text {
		if _, ok := face.GlyphAdvance(r); ok {
			sb.WriteRune(r)
		} else {
			sb.WriteByte('?')
		}
	}
	return sb.String()
}

func truncateLabel(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes-1]) + "…"
}

func formatCount(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(n)/1_000_000)
	case n >= 10_000:
		return fmt.Sprintf("%.1fK", float64(n)/1000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
