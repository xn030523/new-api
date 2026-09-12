package model

// ChannelInterceptor 渠道请求拦截器配置
// 在请求发送到上游 API 之前，对请求 JSON 进行清洗/修复
type ChannelInterceptor struct {
	Id        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	ChannelId int    `json:"channel_id" gorm:"index;not null;default:0"` // 0 = 全局
	Name      string `json:"name" gorm:"not null"`
	Rules     string `json:"rules" gorm:"type:text"` // JSON 数组，规则列表
	Enabled   bool   `json:"enabled" gorm:"default:true"`
	Priority  int    `json:"priority" gorm:"default:0"` // 优先级，数字大的先执行
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}

func GetInterceptorsByChannelId(channelId int) ([]*ChannelInterceptor, error) {
	var interceptors []*ChannelInterceptor
	// 查全局(channel_id=0) + 指定渠道的，按优先级降序
	err := DB.Where("(channel_id = ? OR channel_id = 0) AND enabled = ?", channelId, true).
		Order("priority DESC, id ASC").
		Find(&interceptors).Error
	return interceptors, err
}

func GetAllInterceptors() ([]*ChannelInterceptor, error) {
	var interceptors []*ChannelInterceptor
	err := DB.Order("channel_id ASC, priority DESC").Find(&interceptors).Error
	return interceptors, err
}

func GetInterceptorById(id int) (*ChannelInterceptor, error) {
	var interceptor ChannelInterceptor
	err := DB.First(&interceptor, id).Error
	return &interceptor, err
}

func CreateInterceptor(interceptor *ChannelInterceptor) error {
	return DB.Create(interceptor).Error
}

func UpdateInterceptor(interceptor *ChannelInterceptor) error {
	return DB.Save(interceptor).Error
}

func DeleteInterceptor(id int) error {
	return DB.Delete(&ChannelInterceptor{}, id).Error
}

func GetInterceptorChannelIds() ([]int, error) {
	var channelIds []int
	err := DB.Model(&ChannelInterceptor{}).
		Where("enabled = ?", true).
		Distinct("channel_id").
		Pluck("channel_id", &channelIds).Error
	return channelIds, err
}

// 需要在 model/main.go 的 AutoMigrate 列表中加入 &ChannelInterceptor{}
