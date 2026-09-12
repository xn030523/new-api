package tgbot

import (
	"slices"
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

// groupColumn returns the dialect-correct quoting for the reserved-word
// "group" column so the users query works on SQLite/MySQL/PostgreSQL alike.
func groupColumn() string {
	if common.UsingMainDatabase(common.DatabaseTypePostgreSQL) {
		return `"group"`
	}
	return "`group`"
}

// Stats aggregates monitoring statistics for a time window.
type Stats struct {
	RPM          int
	TPM          int
	TodayTotal   int
	Users        []UserStat
	Groups       []GroupStat
	TodaySpend   []UserSpend
	AllTimeSpend []GroupSpend
	ErrorCount   int
}

// UserStat is one user's row in the monitoring dashboard: today's spend plus
// live RPM/TPM (last 60s), the user's remark group and all-time spend.
type UserStat struct {
	UserID  int
	Name    string
	Today   int
	RPM     int
	TPM     int
	Remark  string
	AllTime int
}

// GroupStat aggregates one remark group for the dashboard.
type GroupStat struct {
	Remark  string
	Users   []UserStat
	Today   int
	AllTime int
	RPM     int
	TPM     int
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
	now := time.Now()
	since := now.Unix() - int64(minutes)*60
	todayStart := now.Truncate(24 * time.Hour).Unix()

	excludeUsers := splitExcludeList(settings.ExcludeUsers)
	excludeRemarks := splitExcludeList(settings.ExcludeRemarks)
	includeGroups := splitExcludeList(settings.IncludeGroups)

	// Load the users that count toward the dashboard, keyed by username. This
	// scopes every later aggregate: active + include_groups + exclude_remarks +
	// exclude_users are all resolved here in one place, cross-DB.
	type userRow struct {
		Id        int
		Username  string
		Remark    string
		UsedQuota int
	}
	userQuery := model.DB.Table("users").
		Select("id, username, remark, used_quota").
		Where("deleted_at IS NULL")
	if settings == nil || settings.ActiveUsersOnly {
		userQuery = userQuery.Where("status = ?", 1)
	}
	if len(includeGroups) > 0 {
		userQuery = userQuery.Where(groupColumn()+" IN ?", includeGroups)
	}
	if len(excludeRemarks) > 0 {
		userQuery = userQuery.Where("COALESCE(remark, '') NOT IN ?", excludeRemarks)
	}
	if len(excludeUsers) > 0 {
		userQuery = userQuery.Where("username NOT IN ?", excludeUsers)
	}
	var userRows []userRow
	if err := userQuery.Scan(&userRows).Error; err != nil {
		return nil, err
	}

	users := make(map[string]*UserStat, len(userRows))
	var usernames []string
	for _, u := range userRows {
		if u.Username == "" {
			continue
		}
		users[u.Username] = &UserStat{
			UserID:  u.Id,
			Name:    u.Username,
			Remark:  u.Remark,
			AllTime: u.UsedQuota,
		}
		usernames = append(usernames, u.Username)
	}
	if len(usernames) == 0 {
		stats.Users = nil
		stats.Groups = nil
		return stats, nil
	}

	// Today's spend per user (00:00 → now), scoped to the loaded users.
	var todayRows []struct {
		Username string
		Quota    int
	}
	if err := model.DB.Table("logs").
		Select("username, COALESCE(SUM(quota), 0) AS quota").
		Where("type = ? AND created_at >= ? AND username IN ?", logTypeConsume, todayStart, usernames).
		Group("username").
		Scan(&todayRows).Error; err != nil {
		return nil, err
	}
	for _, r := range todayRows {
		if u, ok := users[r.Username]; ok {
			u.Today = r.Quota
			stats.TodayTotal += r.Quota
		}
	}

	// Live RPM/TPM per user over the last window, scoped to the loaded users.
	var liveRows []struct {
		Username string
		Reqs     int
		Tokens   int
	}
	if err := model.DB.Table("logs").
		Select("username, COUNT(*) AS reqs, COALESCE(SUM(prompt_tokens + completion_tokens), 0) AS tokens").
		Where("type = ? AND created_at >= ? AND username IN ?", logTypeConsume, since, usernames).
		Group("username").
		Scan(&liveRows).Error; err != nil {
		return nil, err
	}
	for _, r := range liveRows {
		if u, ok := users[r.Username]; ok {
			u.RPM = r.Reqs / minutes
			u.TPM = r.Tokens / minutes
			stats.RPM += u.RPM
			stats.TPM += u.TPM
		}
	}

	// Group users by remark, computing per-group aggregates.
	groupMap := make(map[string]*GroupStat)
	for _, u := range users {
		g, ok := groupMap[u.Remark]
		if !ok {
			g = &GroupStat{Remark: u.Remark}
			groupMap[u.Remark] = g
		}
		g.Users = append(g.Users, *u)
		g.Today += u.Today
		g.AllTime += u.AllTime
		g.RPM += u.RPM
		g.TPM += u.TPM
	}

	// Sort groups: non-empty remark first, then by today's spend desc, empty
	// remark last. Users within a group by today's spend desc.
	groups := make([]GroupStat, 0, len(groupMap))
	for _, g := range groupMap {
		slices.SortFunc(g.Users, func(a, b UserStat) int { return b.Today - a.Today })
		groups = append(groups, *g)
	}
	slices.SortFunc(groups, func(a, b GroupStat) int {
		aEmpty, bEmpty := a.Remark == "", b.Remark == ""
		if aEmpty != bEmpty {
			if aEmpty {
				return 1
			}
			return -1
		}
		return b.Today - a.Today
	})
	stats.Groups = groups

	// Flat user list (today spend desc) for the text fallback and TodaySpend.
	flat := make([]UserStat, 0, len(users))
	for _, u := range users {
		flat = append(flat, *u)
	}
	slices.SortFunc(flat, func(a, b UserStat) int { return b.Today - a.Today })
	stats.Users = flat
	for _, u := range flat {
		if u.Today > 0 {
			stats.TodaySpend = append(stats.TodaySpend, UserSpend{Username: u.Name, Quota: u.Today})
		}
	}
	for _, g := range groups {
		stats.AllTimeSpend = append(stats.AllTimeSpend, GroupSpend{Remark: g.Remark, Quota: g.AllTime})
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
