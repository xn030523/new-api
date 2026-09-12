package model

import "gorm.io/gorm"

// InterceptorLog 拦截日志
// 记录每次请求被拦截器修改或拒绝的详细信息
type InterceptorLog struct {
	Id           int    `json:"id" gorm:"index:idx_iclog_created_id,priority:2"`
	UserId       int    `json:"user_id" gorm:"index"`
	Username     string `json:"username" gorm:"index;default:''"`
	TokenId      int    `json:"token_id" gorm:"index;default:0"`
	TokenName    string `json:"token_name" gorm:"default:''"`
	ChannelId    int    `json:"channel_id" gorm:"index;default:0"`
	ChannelName  string `json:"channel_name" gorm:"default:''"`
	ModelName    string `json:"model_name" gorm:"default:''"`
	RuleType     string `json:"rule_type" gorm:"default:''"`
	RuleDetail   string `json:"rule_detail" gorm:"type:text"`
	Action       string `json:"action" gorm:"default:'modified'"` // modified / rejected
	RejectReason string `json:"reject_reason" gorm:"type:text"`
	RequestBody  string `json:"request_body" gorm:"type:text"`
	ModifiedBody string `json:"modified_body" gorm:"type:text"`
	CreatedAt    int64  `json:"created_at" gorm:"bigint;index:idx_iclog_created_id,priority:1"`
}

func (InterceptorLog) TableName() string {
	return "interceptor_logs"
}

func CreateInterceptorLog(log *InterceptorLog) error {
	return DB.Create(log).Error
}

func GetAllInterceptorLogs(page int, pageSize int, username string, channelId int,
	action string, modelName string, ruleType string, start int64, end int64) ([]*InterceptorLog, int64, error) {
	var logs []*InterceptorLog
	var total int64

	tx := DB.Model(&InterceptorLog{})
	if username != "" {
		tx = tx.Where("username = ?", username)
	}
	if channelId > 0 {
		tx = tx.Where("channel_id = ?", channelId)
	}
	if action != "" {
		tx = tx.Where("action = ?", action)
	}
	if modelName != "" {
		tx = tx.Where("model_name = ?", modelName)
	}
	if ruleType != "" {
		tx = tx.Where("rule_type = ?", ruleType)
	}
	if start > 0 {
		tx = tx.Where("created_at >= ?", start)
	}
	if end > 0 {
		tx = tx.Where("created_at <= ?", end)
	}

	err := tx.Count(&total).Error
	if err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if offset < 0 {
		offset = 0
	}

	err = tx.Order("created_at DESC, id DESC").
		Limit(pageSize).
		Offset(offset).
		Find(&logs).Error
	return logs, total, err
}

func DeleteInterceptorLog(id int) error {
	return DB.Delete(&InterceptorLog{}, id).Error
}

func ClearInterceptorLogs(before int64) (int64, error) {
	result := DB.Where("created_at < ?", before).Delete(&InterceptorLog{})
	return result.RowsAffected, result.Error
}

// InterceptorLogStatRow 是一个聚合维度上的统计结果。
type InterceptorLogStatRow struct {
	// key 在 MySQL 是保留字，聚合别名用 stat_key 避免加方言引号。
	Key      string `json:"key" gorm:"column:stat_key"`
	Total    int64  `json:"total"`
	Modified int64  `json:"modified"`
	Rejected int64  `json:"rejected"`
}

// InterceptorLogStats 是统计视图的完整数据。
type InterceptorLogStats struct {
	Total     int64                   `json:"total"`
	Modified  int64                   `json:"modified"`
	Rejected  int64                   `json:"rejected"`
	ByUser    []InterceptorLogStatRow `json:"by_user"`
	ByRule    []InterceptorLogStatRow `json:"by_rule"`
	ByModel   []InterceptorLogStatRow `json:"by_model"`
	ByChannel []InterceptorLogStatRow `json:"by_channel"`
}

// interceptorLogStatGroups 把前端可选的维度映射到列名，避免把请求参数直接拼进 SQL。
var interceptorLogStatGroups = map[string]string{
	"username":   "username",
	"rule_type":  "rule_type",
	"model_name": "model_name",
	"channel":    "channel_name",
}

// GetInterceptorLogStats 按用户、规则、模型、渠道聚合拦截量。
// 明细表在 2000 RPM 下会涨到百万行级，逐条翻页看不出问题在哪，
// 所以统计视图只回聚合结果，每个维度取前 limit 名。
func GetInterceptorLogStats(start int64, end int64, limit int) (*InterceptorLogStats, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	stats := &InterceptorLogStats{
		ByUser:    []InterceptorLogStatRow{},
		ByRule:    []InterceptorLogStatRow{},
		ByModel:   []InterceptorLogStatRow{},
		ByChannel: []InterceptorLogStatRow{},
	}

	scope := func() *gorm.DB {
		tx := DB.Model(&InterceptorLog{})
		if start > 0 {
			tx = tx.Where("created_at >= ?", start)
		}
		if end > 0 {
			tx = tx.Where("created_at <= ?", end)
		}
		return tx
	}

	// 三个总数一次查完，避免为每个卡片各跑一遍全表扫描。
	var totals struct {
		Total    int64
		Modified int64
		Rejected int64
	}
	err := scope().Select(
		"COUNT(*) AS total, " +
			"SUM(CASE WHEN action = 'rejected' THEN 1 ELSE 0 END) AS rejected, " +
			"SUM(CASE WHEN action = 'rejected' THEN 0 ELSE 1 END) AS modified").
		Scan(&totals).Error
	if err != nil {
		return nil, err
	}
	stats.Total = totals.Total
	stats.Modified = totals.Modified
	stats.Rejected = totals.Rejected

	groupBy := func(column string) ([]InterceptorLogStatRow, error) {
		rows := []InterceptorLogStatRow{}
		err := scope().
			Select(column + " AS stat_key, " +
				"COUNT(*) AS total, " +
				"SUM(CASE WHEN action = 'rejected' THEN 1 ELSE 0 END) AS rejected, " +
				"SUM(CASE WHEN action = 'rejected' THEN 0 ELSE 1 END) AS modified").
			Group(column).
			Order("total DESC").
			Limit(limit).
			Scan(&rows).Error
		return rows, err
	}

	for _, dim := range []struct {
		column string
		target *[]InterceptorLogStatRow
	}{
		{interceptorLogStatGroups["username"], &stats.ByUser},
		{interceptorLogStatGroups["rule_type"], &stats.ByRule},
		{interceptorLogStatGroups["model_name"], &stats.ByModel},
		{interceptorLogStatGroups["channel"], &stats.ByChannel},
	} {
		rows, err := groupBy(dim.column)
		if err != nil {
			return nil, err
		}
		*dim.target = rows
	}
	return stats, nil
}
