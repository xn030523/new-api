package tgbot

import (
	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

// Re-export model types for convenience.
type BotConfig = model.TgBotConfigRow
type BotFeature = model.TgBotFeatureRow

// BotFeatureTarget is one push destination for a feature.
type BotFeatureTarget struct {
	ChatID   int64 `json:"chat_id"`
	ThreadID int   `json:"thread_id"`
	Enabled  bool  `json:"enabled"`
}

// GetTargets parses the targets JSON array.
func GetTargets(f *BotFeature) []BotFeatureTarget {
	if f.Targets == "" {
		return nil
	}
	var targets []BotFeatureTarget
	_ = common.UnmarshalJsonStr(f.Targets, &targets)
	return targets
}

// GetEnabledTargets returns only enabled targets.
func GetEnabledTargets(f *BotFeature) []BotFeatureTarget {
	targets := GetTargets(f)
	var enabled []BotFeatureTarget
	for _, t := range targets {
		if t.Enabled && t.ChatID != 0 {
			enabled = append(enabled, t)
		}
	}
	return enabled
}

// Feature names
const (
	FeatureMonitor   = "monitor"
	FeatureGetkey    = "getkey"
	FeatureAdduser   = "adduser"
	FeatureBilling   = "billing"
	FeatureIntercept = "intercept"
)

var AllFeatures = []string{FeatureMonitor, FeatureGetkey, FeatureAdduser, FeatureBilling, FeatureIntercept}

var FeatureDescriptions = map[string]string{
	FeatureMonitor:   "监控推送（每分钟 RPM/TPM/消费统计）",
	FeatureGetkey:    "创建 API Key（/getkey 交互命令）",
	FeatureAdduser:   "创建用户（/adduser 交互命令）",
	FeatureBilling:   "结算账单（/billing 交互命令）",
	FeatureIntercept: "错误告警（429/400 等错误推送）",
}

// MonitorSettings holds settings for the monitor feature.
type MonitorSettings struct {
	ChartEnabled    bool   `json:"chart_enabled"`
	ChartFontPath   string `json:"chart_font_path"`
	ExcludeUsers    string `json:"exclude_users"`
	ExcludeRemarks  string `json:"exclude_remarks"`
	IncludeGroups   string `json:"include_groups"`
	ActiveUsersOnly bool   `json:"active_users_only"`
	MaskUsername    bool   `json:"mask_username"`
	CurrencySymbol  string `json:"currency_symbol"`
	QuotaPerUnit    int    `json:"quota_per_unit"`
}

// GetkeySettings holds settings for the getkey feature.
type GetkeySettings struct {
	AllowedGroups []string `json:"allowed_groups"`
	QuotaPerUnit  int      `json:"quota_per_unit"`
}

// AdduserSettings holds settings for the adduser feature.
type AdduserSettings struct {
	AllowedGroups []string `json:"allowed_groups"`
	DefaultGroup  string   `json:"default_group"`
	Remarks       []string `json:"remarks"`
	InitialQuota  int      `json:"initial_quota"`
}

// BillingSettings holds settings for the billing feature.
type BillingSettings struct {
	BillingRate    float64 `json:"billing_rate"`
	UsdCnyRate     float64 `json:"usd_cny_rate"`
	CurrencySymbol string  `json:"currency_symbol"`
	ExcludeUsers   string  `json:"exclude_users"`
	ExcludeRemarks string  `json:"exclude_remarks"`
}

// InterceptSettings holds settings for the intercept feature.
type InterceptSettings struct {
	AlertCodes []int `json:"alert_codes"`
	MinCount   int   `json:"min_count"`
}

// DefaultSettings returns default settings JSON for a feature.
func DefaultSettings(name string) string {
	switch name {
	case FeatureMonitor:
		b, _ := common.Marshal(MonitorSettings{
			ChartEnabled: true, ActiveUsersOnly: true, MaskUsername: true,
			CurrencySymbol: "$", QuotaPerUnit: 500000,
		})
		return string(b)
	case FeatureGetkey:
		b, _ := common.Marshal(GetkeySettings{QuotaPerUnit: 500000})
		return string(b)
	case FeatureAdduser:
		b, _ := common.Marshal(AdduserSettings{InitialQuota: 100000 * 500000})
		return string(b)
	case FeatureBilling:
		b, _ := common.Marshal(BillingSettings{
			BillingRate: 2.5, UsdCnyRate: 6.72, CurrencySymbol: "$",
		})
		return string(b)
	case FeatureIntercept:
		b, _ := common.Marshal(InterceptSettings{AlertCodes: []int{429, 400}, MinCount: 1})
		return string(b)
	}
	return "{}"
}

// LoadBotConfig loads the singleton bot config, creating defaults if absent.
func LoadBotConfig() (*BotConfig, error) {
	config := &BotConfig{}
	if err := model.DB.Where("id = 1").First(config).Error; err != nil {
		config = &BotConfig{Id: 1}
		if err := model.DB.Create(config).Error; err != nil {
			return nil, err
		}
	}
	return config, nil
}

// SaveBotConfig persists the bot config.
func SaveBotConfig(c *BotConfig) error {
	return model.DB.Save(c).Error
}

// LoadAllFeatures loads all feature configs, creating defaults for missing ones.
func LoadAllFeatures() ([]*BotFeature, error) {
	var features []*BotFeature
	if err := model.DB.Find(&features).Error; err != nil {
		return nil, err
	}
	existing := make(map[string]bool)
	for _, f := range features {
		existing[f.Name] = true
	}
	for _, name := range AllFeatures {
		if !existing[name] {
			f := &BotFeature{Name: name, Settings: DefaultSettings(name)}
			if err := model.DB.Create(f).Error; err != nil {
				common.SysError("tgbot: create default feature " + name + ": " + err.Error())
				continue
			}
			features = append(features, f)
		}
	}
	return features, nil
}

// LoadFeature loads a single feature config by name.
func LoadFeature(name string) (*BotFeature, error) {
	var feature BotFeature
	if err := model.DB.Where("name = ?", name).First(&feature).Error; err != nil {
		return nil, err
	}
	return &feature, nil
}

// SaveFeature persists a feature config.
func SaveFeature(f *BotFeature) error {
	return model.DB.Save(f).Error
}

// GetMonitorSettings parses monitor feature settings.
func GetMonitorSettings(f *BotFeature) *MonitorSettings {
	s := &MonitorSettings{}
	_ = common.UnmarshalJsonStr(f.Settings, s)
	return s
}

// GetGetkeySettings parses getkey feature settings.
func GetGetkeySettings(f *BotFeature) *GetkeySettings {
	s := &GetkeySettings{}
	_ = common.UnmarshalJsonStr(f.Settings, s)
	return s
}

// GetAdduserSettings parses adduser feature settings.
func GetAdduserSettings(f *BotFeature) *AdduserSettings {
	s := &AdduserSettings{}
	_ = common.UnmarshalJsonStr(f.Settings, s)
	return s
}

// GetBillingSettings parses billing feature settings.
func GetBillingSettings(f *BotFeature) *BillingSettings {
	s := &BillingSettings{}
	_ = common.UnmarshalJsonStr(f.Settings, s)
	return s
}

// GetInterceptSettings parses intercept feature settings.
func GetInterceptSettings(f *BotFeature) *InterceptSettings {
	s := &InterceptSettings{}
	_ = common.UnmarshalJsonStr(f.Settings, s)
	return s
}
