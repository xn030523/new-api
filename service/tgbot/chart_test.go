package tgbot

import (
	"bytes"
	"image/png"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderDashboard(t *testing.T) {
	stats := &Stats{
		RPM:        2143,
		TPM:        1856342,
		ErrorCount: 7,
		TodaySpend: []UserSpend{
			{Username: "alice", Quota: 12_500_000},
			{Username: "bob", Quota: 8_200_000},
			{Username: "carol", Quota: 4_100_000},
			{Username: "dave", Quota: 900_000},
		},
		AllTimeSpend: []GroupSpend{
			{Remark: "team-a", Quota: 90_000_000},
			{Remark: "team-b", Quota: 41_000_000},
			{Remark: "", Quota: 5_000_000},
		},
	}
	settings := &MonitorSettings{
		ChartEnabled: true, MaskUsername: true,
		CurrencySymbol: "$", QuotaPerUnit: 500000,
	}

	data, err := RenderDashboard(stats, settings, "NEW-API MONITOR")
	require.NoError(t, err)
	require.NotEmpty(t, data)

	img, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, chartWidth, img.Bounds().Dx())
	assert.Equal(t, chartHeight, img.Bounds().Dy())

	if out := os.Getenv("TGBOT_CHART_OUT"); out != "" {
		require.NoError(t, os.WriteFile(out, data, 0o600))
	}
}

func TestRenderDashboardEmptyStats(t *testing.T) {
	data, err := RenderDashboard(&Stats{}, &MonitorSettings{ChartEnabled: true}, "EMPTY")
	require.NoError(t, err)
	_, err = png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
}

func TestRenderDashboardNilStats(t *testing.T) {
	_, err := RenderDashboard(nil, nil, "X")
	require.Error(t, err)
}

func TestSubstituteMissingGlyphs(t *testing.T) {
	fonts, err := newChartFonts("")
	require.NoError(t, err)
	defer fonts.close()

	assert.Equal(t, "alice", substituteMissingGlyphs(fonts.label, "alice"))
	// The built-in Go font has no CJK glyphs, so they must become visible '?'.
	assert.Equal(t, "??", substituteMissingGlyphs(fonts.label, "张三"))
}
