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
		TodayTotal: 25_700_000,
		ErrorCount: 7,
		Groups: []GroupStat{
			{
				Remark: "team-a", Today: 20_700_000, AllTime: 90_000_000, RPM: 1800, TPM: 1_500_000,
				Users: []UserStat{
					{Name: "alice", Today: 12_500_000, AllTime: 60_000_000, RPM: 1200, TPM: 1_000_000, Remark: "team-a"},
					{Name: "bob", Today: 8_200_000, AllTime: 30_000_000, RPM: 600, TPM: 500_000, Remark: "team-a"},
				},
			},
			{
				Remark: "team-b", Today: 4_100_000, AllTime: 41_000_000, RPM: 300, TPM: 300_000,
				Users: []UserStat{
					{Name: "carol", Today: 4_100_000, AllTime: 41_000_000, RPM: 300, TPM: 300_000, Remark: "team-b"},
				},
			},
			{
				Remark: "", Today: 900_000, AllTime: 5_000_000, RPM: 43, TPM: 56_342,
				Users: []UserStat{
					{Name: "dave", Today: 900_000, AllTime: 5_000_000, RPM: 43, TPM: 56_342},
				},
			},
		},
	}
	settings := &MonitorSettings{
		ChartEnabled: true, MaskUsername: false,
		CurrencySymbol: "$", QuotaPerUnit: 500000,
	}

	data, err := RenderDashboard(stats, settings, "NEW-API MONITOR")
	require.NoError(t, err)
	require.NotEmpty(t, data)

	img, err := png.Decode(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, chartWidth, img.Bounds().Dx())
	// Height is auto-sized from the number of ranking rows, so assert it is at
	// least the enforced minimum rather than a fixed constant.
	assert.GreaterOrEqual(t, img.Bounds().Dy(), 460)

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
