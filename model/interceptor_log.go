package model

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

func GetAllInterceptorLogs(page int, pageSize int, username string, channelId int, action string, modelName string) ([]*InterceptorLog, int64, error) {
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
