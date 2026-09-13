package tgbot

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

// Dashboard canvas geometry. Height is computed per render from the number of
// ranking rows, mirroring the Python dashboard's auto-sizing behaviour.
const (
	chartWidth      = 1180
	chartPadding    = 34
	chartCardTop    = 96
	chartCardHeight = 108
	chartCardGap    = 20
	chartRowHeight  = 40
	chartRankTop    = 40 // gap between KPI cards and the ranking area
	chartFooterH    = 52
	chartCornerSize = 6
	chartLabelWidth = 240
	chartValueWidth = 360
)

var (
	colorBackground  = color.RGBA{0x0f, 0x14, 0x20, 0xff}
	colorPanel       = color.RGBA{0x1a, 0x21, 0x30, 0xff}
	colorTextBright  = color.RGBA{0xe6, 0xea, 0xf2, 0xff}
	colorTextMuted   = color.RGBA{0x8a, 0x94, 0xa6, 0xff}
	colorAccent      = color.RGBA{0x4c, 0x9a, 0xff, 0xff}
	colorAccentWarm  = color.RGBA{0xff, 0xb0, 0x20, 0xff}
	colorAccentGreen = color.RGBA{0x00, 0xd6, 0x8f, 0xff}
	colorDanger      = color.RGBA{0xff, 0x6b, 0x6b, 0xff}
	colorGrid        = color.RGBA{0x2a, 0x33, 0x46, 0xff}
	colorEmptyGroup  = color.RGBA{0x5a, 0x64, 0x78, 0xff}

	// Group palette: each non-empty remark takes the next colour; the empty
	// remark always renders in grey (colorEmptyGroup).
	groupPalette = []color.RGBA{
		{0x4c, 0x9a, 0xff, 0xff},
		{0x00, 0xd6, 0x8f, 0xff},
		{0xff, 0xb0, 0x20, 0xff},
		{0xff, 0x6b, 0x81, 0xff},
		{0xa6, 0x6c, 0xff, 0xff},
		{0x3e, 0xd6, 0xc5, 0xff},
		{0xf7, 0x78, 0x25, 0xff},
		{0xe2, 0xe8, 0xf0, 0xff},
		{0x94, 0xa3, 0xb8, 0xff},
		{0xfa, 0xcc, 0x15, 0xff},
	}
)

// Faces are parsed once; opentype.Face is not safe for concurrent use, so the
// renderer holds its own drawer and callers must not share faces across goroutines.
type chartFonts struct {
	title  font.Face
	sub    font.Face
	label  font.Face
	value  font.Face
	metric font.Face
	header font.Face
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
		} else if custom, parseErr := parseFontFile(raw); parseErr != nil {
			common.SysError("tgbot: parse chart font failed, using built-in font: " + parseErr.Error())
		} else {
			bold, regular = custom, custom
		}
	}
	newFace := func(f *opentype.Font, size float64) (font.Face, error) {
		return opentype.NewFace(f, &opentype.FaceOptions{Size: size, DPI: 72, Hinting: font.HintingFull})
	}
	title, err := newFace(bold, 26)
	if err != nil {
		return nil, err
	}
	sub, err := newFace(regular, 13)
	if err != nil {
		return nil, err
	}
	label, err := newFace(regular, 14)
	if err != nil {
		return nil, err
	}
	value, err := newFace(bold, 14)
	if err != nil {
		return nil, err
	}
	metric, err := newFace(bold, 32)
	if err != nil {
		return nil, err
	}
	header, err := newFace(bold, 15)
	if err != nil {
		return nil, err
	}
	return &chartFonts{title: title, sub: sub, label: label, value: value, metric: metric, header: header}, nil
}

// parseFontFile parses a font from raw bytes, supporting both single-font
// files (TTF/OTF) and TrueType/OpenType collections (TTC — e.g. the system
// NotoSansCJK-Regular.ttc). opentype.Parse rejects collections, so a ttc must
// go through sfnt.ParseCollection and take the first face. opentype.Font is an
// alias for sfnt.Font, so the returned pointer is usable directly.
func parseFontFile(raw []byte) (*opentype.Font, error) {
	if f, err := opentype.Parse(raw); err == nil {
		return f, nil
	}
	coll, err := sfnt.ParseCollection(raw)
	if err != nil {
		return nil, err
	}
	if coll.NumFonts() == 0 {
		return nil, fmt.Errorf("tgbot: empty font collection")
	}
	return coll.Font(0)
}

func (f *chartFonts) close() {
	for _, face := range []font.Face{f.title, f.sub, f.label, f.value, f.metric, f.header} {
		if face != nil {
			_ = face.Close()
		}
	}
}

// rankEntry is one row of the ranking area: either a group header or a user.
type rankEntry struct {
	isHeader bool
	text     string     // header text
	user     UserStat   // user row
	color    color.RGBA // bar / swatch colour for the group
}

// RenderDashboard draws the monitoring dashboard as a PNG. The layout mirrors
// the Python dashboard: four KPI cards, a per-remark grouped ranking with a
// header line and one bar per user (today's spend), a colour legend, and a
// footer with per-group and grand totals.
func RenderDashboard(stats *Stats, settings *MonitorSettings, title string) ([]byte, error) {
	if stats == nil {
		return nil, fmt.Errorf("tgbot: nil stats")
	}
	fontPath := ""
	symbol := "$"
	quotaPerUnit := 500000.0
	if settings != nil {
		fontPath = settings.ChartFontPath
		if settings.CurrencySymbol != "" {
			symbol = settings.CurrencySymbol
		}
		if settings.QuotaPerUnit > 0 {
			quotaPerUnit = float64(settings.QuotaPerUnit)
		}
	}
	fonts, err := newChartFonts(fontPath)
	if err != nil {
		return nil, err
	}
	defer fonts.close()

	// Assign a colour per group and flatten into ranking entries.
	entries := make([]rankEntry, 0)
	colorIdx := 0
	maxToday := 1.0
	for _, g := range stats.Groups {
		var col color.RGBA
		if g.Remark == "" {
			col = colorEmptyGroup
		} else {
			col = groupPalette[colorIdx%len(groupPalette)]
			colorIdx++
		}
		remarkLabel := g.Remark
		if remarkLabel == "" {
			remarkLabel = "无备注"
		}
		entries = append(entries, rankEntry{
			isHeader: true,
			color:    col,
			text: fmt.Sprintf("备注[%s] · %d位 · 今日 %s · 累计 %s · RPM %d · TPM %d",
				remarkLabel, len(g.Users), money(g.Today, quotaPerUnit, symbol),
				money(g.AllTime, quotaPerUnit, symbol), g.RPM, g.TPM),
		})
		for _, u := range g.Users {
			entries = append(entries, rankEntry{user: u, color: col})
			if v := float64(u.Today) / quotaPerUnit; v > maxToday {
				maxToday = v
			}
		}
	}

	// Compute canvas height from the number of ranking rows.
	rankRows := max(len(entries), 1)
	rankAreaTop := chartPadding + chartCardTop + chartCardHeight + chartRankTop
	legendH := 30
	height := rankAreaTop + rankRows*chartRowHeight + legendH + chartFooterH + chartPadding
	height = max(height, 460)

	canvas := image.NewRGBA(image.Rect(0, 0, chartWidth, height))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{colorBackground}, image.Point{}, draw.Src)

	// Title + subtitle.
	if title == "" {
		title = "New-API 实时监控"
	}
	drawText(canvas, fonts.title, colorTextBright, chartPadding, chartPadding+28, title)

	// KPI cards.
	cardsTop := chartPadding + chartCardTop
	cardW := (chartWidth - 2*chartPadding - 3*chartCardGap) / 4
	kpis := []struct {
		label string
		value string
		sub   string
		col   color.RGBA
	}{
		{"当前 RPM", formatCount(stats.RPM), "请求/分钟", colorAccentGreen},
		{"当前 TPM", formatCount(stats.TPM), "Token/分钟", colorAccent},
		{"今日消费", money(stats.TodayTotal, quotaPerUnit, symbol), "Today", colorTextBright},
		{"备注分组", fmt.Sprintf("%d", len(stats.Groups)), "个", colorAccentWarm},
	}
	for i, k := range kpis {
		x := chartPadding + i*(cardW+chartCardGap)
		drawMetricCard(canvas, fonts, x, cardsTop, cardW, chartCardHeight, k.label, k.value, k.sub, k.col)
	}

	// Ranking area.
	if len(entries) == 0 {
		drawText(canvas, fonts.label, colorTextMuted, chartPadding, rankAreaTop+30, "暂无消费记录")
	} else {
		trackX := chartPadding + chartLabelWidth
		trackW := chartWidth - chartPadding - trackX - chartValueWidth
		if trackW < 60 {
			trackW = 60
		}
		rowY := rankAreaTop
		for _, e := range entries {
			if e.isHeader {
				drawText(canvas, fonts.header, colorTextBright, chartPadding, rowY+chartRowHeight/2+5,
					truncateLabel(e.text, 70))
				rowY += chartRowHeight
				continue
			}
			u := e.user
			barTop := rowY + 8
			barBot := rowY + chartRowHeight - 12
			// Name label.
			drawText(canvas, fonts.label, colorTextMuted, chartPadding+16, rowY+chartRowHeight/2+5,
				truncateLabel(u.Name, 18))
			// Bar track + fill.
			draw.Draw(canvas, image.Rect(trackX, barTop, trackX+trackW, barBot),
				&image.Uniform{colorGrid}, image.Point{}, draw.Src)
			today := float64(u.Today) / quotaPerUnit
			filled := int(float64(trackW) * today / maxToday)
			if filled < 2 && u.Today > 0 {
				filled = 2
			}
			if filled > 0 {
				draw.Draw(canvas, image.Rect(trackX, barTop, trackX+filled, barBot),
					&image.Uniform{e.color}, image.Point{}, draw.Src)
			}
			// Value column: today / all-time + RPM/TPM.
			val := fmt.Sprintf("今 %s / 累计 %s   RPM %d  TPM %s",
				money(u.Today, quotaPerUnit, symbol), money(u.AllTime, quotaPerUnit, symbol),
				u.RPM, kfmt(u.TPM))
			drawText(canvas, fonts.label, colorTextBright, trackX+trackW+14, rowY+chartRowHeight/2+5, val)
			rowY += chartRowHeight
		}

		// Legend: one swatch per group.
		legendY := rowY + 6
		lx := chartPadding
		for _, g := range stats.Groups {
			var col color.RGBA
			if g.Remark == "" {
				col = colorEmptyGroup
			} else {
				// Recompute the same colour order used above.
				col = legendColor(stats.Groups, g.Remark)
			}
			label := g.Remark
			if label == "" {
				label = "无备注"
			}
			draw.Draw(canvas, image.Rect(lx, legendY, lx+16, legendY+16),
				&image.Uniform{col}, image.Point{}, draw.Src)
			drawText(canvas, fonts.sub, colorTextMuted, lx+22, legendY+13, label)
			lx += 22 + textWidth(fonts.sub, label) + 26
		}
	}

	// Footer: per-group totals + grand total.
	var footParts []string
	for _, g := range stats.Groups {
		label := g.Remark
		if label == "" {
			label = "无备注"
		}
		footParts = append(footParts, fmt.Sprintf("[%s] 今%s/累计%s",
			label, money(g.Today, quotaPerUnit, symbol), money(g.AllTime, quotaPerUnit, symbol)))
	}
	footer := strings.Join(footParts, "   ")
	if footer != "" {
		footer += "   |   "
	}
	footer += fmt.Sprintf("今日合计 %s", money(stats.TodayTotal, quotaPerUnit, symbol))
	drawText(canvas, fonts.sub, colorTextMuted, chartPadding, height-chartPadding, truncateLabel(footer, 120))

	var out bytes.Buffer
	if err := png.Encode(&out, canvas); err != nil {
		return nil, fmt.Errorf("tgbot: encode png: %w", err)
	}
	return out.Bytes(), nil
}

// legendColor recomputes the palette colour assigned to a remark, matching the
// order used when the entries were built (non-empty remarks consume palette
// slots in group order; the empty remark is always grey).
func legendColor(groups []GroupStat, remark string) color.RGBA {
	idx := 0
	for _, g := range groups {
		if g.Remark == "" {
			continue
		}
		if g.Remark == remark {
			return groupPalette[idx%len(groupPalette)]
		}
		idx++
	}
	return colorEmptyGroup
}

func drawMetricCard(dst *image.RGBA, fonts *chartFonts, x, y, w, h int, label, value, sub string, accent color.RGBA) {
	fillPanel(dst, x, y, w, h)
	drawText(dst, fonts.sub, colorTextMuted, x+18, y+26, label)
	drawText(dst, fonts.metric, accent, x+18, y+h-34, value)
	drawText(dst, fonts.sub, colorTextMuted, x+18, y+h-12, sub)
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

func textWidth(face font.Face, text string) int {
	d := &font.Drawer{Face: face}
	return d.MeasureString(substituteMissingGlyphs(face, text)).Ceil()
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

// money formats a raw quota value into a currency string.
func money(quota int, quotaPerUnit float64, symbol string) string {
	if quotaPerUnit <= 0 {
		quotaPerUnit = 500000
	}
	return fmt.Sprintf("%s%.2f", symbol, float64(quota)/quotaPerUnit)
}

// kfmt abbreviates large numbers: 1.2M / 34k / 567.
func kfmt(v int) string {
	switch {
	case v >= 1_000_000:
		return fmt.Sprintf("%.1fM", float64(v)/1_000_000)
	case v >= 1_000:
		return fmt.Sprintf("%.0fk", float64(v)/1_000)
	default:
		return fmt.Sprintf("%d", v)
	}
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
