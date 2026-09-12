package tgbot

import (
	"strconv"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"gorm.io/gorm"
)

const (
	logTypeConsume = 2
	logTypeError   = 5
)

// Stats aggregates monitoring statistics for a time window.
type Stats struct {
	RPM          int
	TPM          int
	TodaySpend   []UserSpend
	AllTimeSpend []GroupSpend
	ErrorCount   int
}

type UserSpend struct {
	Username string
	Quota    int
}

type GroupSpend struct {
	Remark string
	Quota  int
}

type ErrorRow struct {
	StatusCode int
	Count      int
	ModelName  string
	Username   string
}

type BillingRow struct {
	Remark string
	Quota  int
}

func splitExcludeList(raw string) []string {
	if raw == "" {
		return nil
	}
	var out []string
	for item := range strings.SplitSeq(raw, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

func applyMonitorFilters(query *gorm.DB, settings *MonitorSettings) *gorm.DB {
	if settings == nil {
		return query
	}
	if excluded := splitExcludeList(settings.ExcludeUsers); len(excluded) > 0 {
		query = query.Where("username NOT IN ?", excluded)
	}
	if settings.ActiveUsersOnly {
		var activeUsers []string
		if err := model.DB.Table("users").Where("status = ?", 1).Pluck("username", &activeUsers).Error; err != nil {
			common.SysError("tgbot: failed to load active users: " + err.Error())
		} else if len(activeUsers) > 0 {
			query = query.Where("username IN ?", activeUsers)
		}
	}
	return query
}

func applyBillingFilters(query *gorm.DB, settings *BillingSettings) *gorm.DB {
	if settings == nil {
		return query
	}
	if excluded := splitExcludeList(settings.ExcludeUsers); len(excluded) > 0 {
		query = query.Where("logs.username NOT IN ?", excluded)
	}
	if excludedRemarks := splitExcludeList(settings.ExcludeRemarks); len(excludedRemarks) > 0 {
		query = query.Where("users.remark NOT IN ?", excludedRemarks)
	}
	return query
}

// FetchStats collects RPM/TPM and spending stats for the last N minutes.
func FetchStats(minutes int, settings *MonitorSettings) (*Stats, error) {
	stats := &Stats{}
	minutes = max(minutes, 1)
	since := common.GetTimestamp() - int64(minutes)*60

	var requestCount int64
	requestQuery := applyMonitorFilters(
		model.DB.Table("logs").Where("created_at >= ? AND type = ?", since, logTypeConsume), settings)
	if err := requestQuery.Count(&requestCount).Error; err != nil {
		return nil, err
	}
	stats.RPM = int(requestCount) / minutes

	var tokenSum struct {
		Total int64
	}
	tokenQuery := applyMonitorFilters(
		model.DB.Table("logs").Where("created_at >= ? AND type = ?", since, logTypeConsume), settings)
	if err := tokenQuery.
		Select("COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS total").
		Scan(&tokenSum).Error; err != nil {
		return nil, err
	}
	stats.TPM = int(tokenSum.Total) / minutes

	todayStart := time.Now().Truncate(24 * time.Hour).Unix()
	todayQuery := applyMonitorFilters(
		model.DB.Table("logs").Where("created_at >= ? AND type = ?", todayStart, logTypeConsume), settings)
	if err := todayQuery.
		Select("username, COALESCE(SUM(quota), 0) AS quota").
		Group("username").
		Order("quota DESC").
		Limit(20).
		Scan(&stats.TodaySpend).Error; err != nil {
		return nil, err
	}

	// All-time spend by remark — use billing-style join but with monitor filters
	remarkQuery := model.DB.Table("logs").
		Joins("JOIN users ON users.username = logs.username").
		Where("logs.type = ?", logTypeConsume)
	if excluded := splitExcludeList(settings.ExcludeUsers); len(excluded) > 0 {
		remarkQuery = remarkQuery.Where("logs.username NOT IN ?", excluded)
	}
	if excludedRemarks := splitExcludeList(settings.ExcludeRemarks); len(excludedRemarks) > 0 {
		remarkQuery = remarkQuery.Where("users.remark NOT IN ?", excludedRemarks)
	}
	if settings.ActiveUsersOnly {
		remarkQuery = remarkQuery.Where("users.status = ?", 1)
	}
	if err := remarkQuery.
		Select("users.remark AS remark, COALESCE(SUM(logs.quota), 0) AS quota").
		Group("users.remark").
		Order("quota DESC").
		Scan(&stats.AllTimeSpend).Error; err != nil {
		return nil, err
	}

	var errCount int64
	if err := model.DB.Table("logs").
		Where("created_at >= ? AND type = ?", since, logTypeError).
		Count(&errCount).Error; err != nil {
		return nil, err
	}
	stats.ErrorCount = int(errCount)
	return stats, nil
}

// FetchErrors aggregates errors from the last N minutes.
func FetchErrors(minutes int, alertCodes []int, minCount int) ([]ErrorRow, error) {
	since := common.GetTimestamp() - int64(minutes)*60
	query := model.DB.Table("logs").
		Where("created_at >= ? AND type = ?", since, logTypeError)

	// If alert codes specified, filter by content containing the status code
	if len(alertCodes) > 0 {
		var conditions []string
		var args []any
		for _, code := range alertCodes {
			conditions = append(conditions, "content LIKE ?")
			args = append(args, "%"+strconv.Itoa(code)+"%")
		}
		query = query.Where(strings.Join(conditions, " OR "), args...)
	}

	var rows []ErrorRow
	err := query.
		Select("model_name, username, COUNT(*) AS count").
		Group("model_name, username").
		Having("COUNT(*) >= ?", minCount).
		Order("count DESC").
		Limit(20).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// FetchBilling returns all-time consumption grouped by user remark.
func FetchBilling(settings *BillingSettings) ([]BillingRow, error) {
	query := applyBillingFilters(
		model.DB.Table("logs").
			Joins("JOIN users ON users.username = logs.username").
			Where("logs.type = ?", logTypeConsume), settings)
	var rows []BillingRow
	if err := query.
		Select("users.remark AS remark, COALESCE(SUM(logs.quota), 0) AS quota").
		Group("users.remark").
		Order("quota DESC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}
