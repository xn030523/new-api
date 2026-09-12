package tgbot

import (
	"fmt"
	"strings"
)

// MaskName masks a username as first char + "***" + last char.
func MaskName(name string) string {
	runes := []rune(name)
	switch len(runes) {
	case 0:
		return "***"
	case 1:
		return string(runes[0]) + "***"
	case 2:
		return string(runes[0]) + "***"
	default:
		return string(runes[0]) + "***" + string(runes[len(runes)-1])
	}
}

// QuotaToMoney converts quota to currency string.
func QuotaToMoney(quota int, quotaPerUnit int, symbol string) string {
	if quotaPerUnit <= 0 {
		quotaPerUnit = 500000
	}
	amount := float64(quota) / float64(quotaPerUnit)
	return fmt.Sprintf("%s%.2f", symbol, amount)
}

// FormatBill renders the billing report as HTML.
func FormatBill(billingData []BillingRow, deerPrice, clientPrice, usdCnyRate float64, symbol string) string {
	var sb strings.Builder
	sb.WriteString("<b>结算账单</b>\n\n")
	if len(billingData) == 0 {
		sb.WriteString("无数据。\n")
		return sb.String()
	}
	var totalQuota int
	var totalDeer, totalClient float64
	for _, row := range billingData {
		remark := row.Remark
		if remark == "" {
			remark = "(无备注)"
		}
		base := float64(row.Quota) / 500000.0
		deer := base * deerPrice
		client := base * clientPrice
		sb.WriteString(fmt.Sprintf("<b>%s</b>\n", remark))
		sb.WriteString(fmt.Sprintf("  消费: %s\n", QuotaToMoney(row.Quota, 500000, symbol)))
		sb.WriteString(fmt.Sprintf("  鹿价: %.2f USD (%.2f CNY)\n", deer, deer*usdCnyRate))
		sb.WriteString(fmt.Sprintf("  客价: %.2f USD (%.2f CNY)\n\n", client, client*usdCnyRate))
		totalQuota += row.Quota
		totalDeer += deer
		totalClient += client
	}
	sb.WriteString("<b>合计</b>\n")
	sb.WriteString(fmt.Sprintf("  消费: %s\n", QuotaToMoney(totalQuota, 500000, symbol)))
	sb.WriteString(fmt.Sprintf("  鹿价: %.2f USD (%.2f CNY)\n", totalDeer, totalDeer*usdCnyRate))
	sb.WriteString(fmt.Sprintf("  客价: %.2f USD (%.2f CNY)\n", totalClient, totalClient*usdCnyRate))
	return sb.String()
}

// FormatStats renders monitoring stats as text.
func FormatStats(stats *Stats, settings *MonitorSettings) string {
	if stats == nil {
		return "暂无数据"
	}
	symbol := "$"
	quotaPerUnit := 500000
	mask := true
	if settings != nil {
		if settings.CurrencySymbol != "" {
			symbol = settings.CurrencySymbol
		}
		if settings.QuotaPerUnit > 0 {
			quotaPerUnit = settings.QuotaPerUnit
		}
		mask = settings.MaskUsername
	}
	var sb strings.Builder
	sb.WriteString("📊 监控面板\n\n")
	sb.WriteString(fmt.Sprintf("RPM: %d\n", stats.RPM))
	sb.WriteString(fmt.Sprintf("TPM: %d\n", stats.TPM))
	sb.WriteString(fmt.Sprintf("错误数: %d\n\n", stats.ErrorCount))

	sb.WriteString("今日消费:\n")
	if len(stats.TodaySpend) == 0 {
		sb.WriteString("  (无)\n")
	}
	for _, u := range stats.TodaySpend {
		name := u.Username
		if mask {
			name = MaskName(name)
		}
		sb.WriteString(fmt.Sprintf("  %s: %s\n", name, QuotaToMoney(u.Quota, quotaPerUnit, symbol)))
	}

	sb.WriteString("\n累计消费（按备注）:\n")
	if len(stats.AllTimeSpend) == 0 {
		sb.WriteString("  (无)\n")
	}
	for _, g := range stats.AllTimeSpend {
		remark := g.Remark
		if remark == "" {
			remark = "(无备注)"
		}
		sb.WriteString(fmt.Sprintf("  %s: %s\n", remark, QuotaToMoney(g.Quota, quotaPerUnit, symbol)))
	}
	return sb.String()
}

// FormatErrorAlert renders aggregated errors as an alert message.
func FormatErrorAlert(errors []ErrorRow) string {
	if len(errors) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("🚨 错误告警\n\n")
	for _, e := range errors {
		model := e.ModelName
		if model == "" {
			model = "(未知模型)"
		}
		username := e.Username
		if username == "" {
			username = "(未知用户)"
		}
		if e.StatusCode > 0 {
			sb.WriteString(fmt.Sprintf("[%d] %s / %s: %d 次\n", e.StatusCode, model, username, e.Count))
		} else {
			sb.WriteString(fmt.Sprintf("%s / %s: %d 次\n", model, username, e.Count))
		}
	}
	return sb.String()
}
