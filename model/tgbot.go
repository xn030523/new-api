package model

// TgBotConfigRow is the DB model for tg_bot_config table.
type TgBotConfigRow struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	BotToken  string `json:"bot_token" gorm:"default:''"`
	Enabled   bool   `json:"enabled" gorm:"default:false"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (TgBotConfigRow) TableName() string {
	return "tg_bot_config"
}

// TgBotFeatureRow is the DB model for tg_bot_features table.
// Targets is a JSON array of {chat_id, thread_id, enabled} — one feature
// can push to multiple group/topic combinations.
type TgBotFeatureRow struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	Name      string `json:"name" gorm:"uniqueIndex;not null"`
	Enabled   bool   `json:"enabled" gorm:"default:false"`
	Targets   string `json:"targets" gorm:"type:text"` // JSON array of BotFeatureTarget
	Settings  string `json:"settings" gorm:"type:text"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func (TgBotFeatureRow) TableName() string {
	return "tg_bot_features"
}
