package tgbot

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/tgclient"
	"golang.org/x/crypto/bcrypt"
)

// ConversationState tracks a multi-step interaction with a Telegram user.
type ConversationState struct {
	Command string
	Step    int
	Data    map[string]any
}

// BotService is the main Telegram bot service.
type BotService struct {
	client        *tgclient.Client
	config        *BotConfig
	features      map[string]*BotFeature
	offset        int64
	stopCh        chan struct{}
	wg            sync.WaitGroup
	conversations sync.Map // tg user ID -> *ConversationState
}

// NewBotService creates a bot service; call Start to run it.
func NewBotService() *BotService {
	return &BotService{
		stopCh: make(chan struct{}),
	}
}

// Start initializes the client from config and launches goroutines.
func (s *BotService) Start(config *BotConfig, features []*BotFeature) error {
	if config == nil || config.BotToken == "" {
		return fmt.Errorf("tgbot: bot token is empty")
	}
	s.config = config
	s.features = make(map[string]*BotFeature)
	for _, f := range features {
		s.features[f.Name] = f
	}
	s.client = tgclient.NewClient(config.BotToken)

	var cmds []tgclient.BotCommand
	if s.featureEnabled(FeatureGetkey) {
		cmds = append(cmds, tgclient.BotCommand{Command: "getkey", Description: "创建 API Key"})
	}
	if s.featureEnabled(FeatureAdduser) {
		cmds = append(cmds, tgclient.BotCommand{Command: "adduser", Description: "创建用户"})
	}
	if s.featureEnabled(FeatureBilling) {
		cmds = append(cmds, tgclient.BotCommand{Command: "billing", Description: "结算账单"})
	}
	cmds = append(cmds, tgclient.BotCommand{Command: "push", Description: "开关监控推送"})
	if err := s.client.SetMyCommands(cmds); err != nil {
		common.SysError("tgbot: failed to set commands: " + err.Error())
	}

	s.wg.Add(2)
	go s.pollLoop()
	go s.monitorLoop()
	common.SysLog("tgbot: bot started")
	return nil
}

// Stop terminates all goroutines.
func (s *BotService) Stop() {
	close(s.stopCh)
	s.wg.Wait()
	common.SysLog("tgbot: bot stopped")
}

// featureEnabled checks if a feature is enabled and has at least one enabled target.
func (s *BotService) featureEnabled(name string) bool {
	f, ok := s.features[name]
	if !ok || !f.Enabled {
		return false
	}
	return len(GetEnabledTargets(f)) > 0
}

// featureMatches checks if a message's chat+thread matches an enabled target of the feature.
func (s *BotService) featureMatches(name string, chatID int64, threadID int) bool {
	f, ok := s.features[name]
	if !ok || !f.Enabled {
		return false
	}
	for _, t := range GetEnabledTargets(f) {
		if t.ChatID == chatID && t.ThreadID == threadID {
			return true
		}
	}
	return false
}

// sendToFeature sends a message to all enabled targets of a feature.
func (s *BotService) sendToFeature(featureName string, text string) {
	f, ok := s.features[featureName]
	if !ok || !f.Enabled {
		return
	}
	for _, target := range GetEnabledTargets(f) {
		if err := s.client.SendMessage(target.ChatID, text, target.ThreadID); err != nil {
			common.SysError(fmt.Sprintf("tgbot: send to %s target %d/%d failed: %v", featureName, target.ChatID, target.ThreadID, err))
		}
	}
}

// sendHTMLToFeature sends an HTML message to all enabled targets of a feature.
func (s *BotService) sendHTMLToFeature(featureName string, html string) {
	f, ok := s.features[featureName]
	if !ok || !f.Enabled {
		return
	}
	for _, target := range GetEnabledTargets(f) {
		if err := s.client.SendHTML(target.ChatID, html, target.ThreadID); err != nil {
			common.SysError(fmt.Sprintf("tgbot: send HTML to %s target %d/%d failed: %v", featureName, target.ChatID, target.ThreadID, err))
		}
	}
}
func (s *BotService) getFeature(name string) *BotFeature {
	f, ok := s.features[name]
	if !ok {
		return nil
	}
	return f
}

// pollLoop runs getUpdates long polling.
func (s *BotService) pollLoop() {
	defer s.wg.Done()
	for {
		select {
		case <-s.stopCh:
			return
		default:
		}
		updates, err := s.client.GetUpdates(s.offset, 30)
		if err != nil {
			common.SysError("tgbot: getUpdates error: " + err.Error())
			select {
			case <-s.stopCh:
				return
			case <-time.After(5 * time.Second):
				continue
			}
		}
		for _, update := range updates {
			if update.UpdateID >= s.offset {
				s.offset = update.UpdateID + 1
			}
			s.dispatch(update)
		}
	}
}

// monitorLoop pushes stats and alerts once per minute.
func (s *BotService) monitorLoop() {
	defer s.wg.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.pushMonitor()
		}
	}
}

func (s *BotService) pushMonitor() {
	if s.config == nil {
		return
	}

	// Error alerts → all enabled intercept targets
	if s.featureEnabled(FeatureIntercept) {
		interceptFeature := s.getFeature(FeatureIntercept)
		settings := GetInterceptSettings(interceptFeature)
		minCount := max(settings.MinCount, 1)
		errors, err := FetchErrors(1, settings.AlertCodes, minCount)
		if err != nil {
			common.SysError("tgbot: fetch errors failed: " + err.Error())
		} else if alert := FormatErrorAlert(errors); alert != "" {
			s.sendToFeature(FeatureIntercept, alert)
		}
	}

	// Monitor push → all enabled monitor targets
	if !s.featureEnabled(FeatureMonitor) {
		return
	}
	monitorFeature := s.getFeature(FeatureMonitor)
	mSettings := GetMonitorSettings(monitorFeature)
	stats, err := FetchStats(1, mSettings)
	if err != nil {
		common.SysError("tgbot: fetch stats failed: " + err.Error())
		return
	}
	s.sendToFeature(FeatureMonitor, FormatStats(stats, mSettings))
}

// dispatch routes one update to the appropriate handler.
func (s *BotService) dispatch(update tgclient.Update) {
	if update.CallbackQuery != nil {
		s.handleCallback(update.CallbackQuery)
		return
	}
	msg := update.Message
	if msg == nil || msg.From == nil {
		return
	}
	text := strings.TrimSpace(msg.Text)
	if text == "" {
		return
	}

	if strings.HasPrefix(text, "/") {
		s.conversations.Delete(msg.From.ID)
		cmd, _, _ := strings.Cut(strings.TrimPrefix(text, "/"), "@")
		cmd, _, _ = strings.Cut(cmd, " ")
		chatID := msg.Chat.ID
		threadID := msg.MessageThreadID
		switch strings.ToLower(cmd) {
		case "getkey":
			if s.featureMatches(FeatureGetkey, chatID, threadID) {
				s.startGetkey(msg)
			}
		case "adduser":
			if s.featureMatches(FeatureAdduser, chatID, threadID) {
				s.startAdduser(msg)
			}
		case "billing":
			if s.featureMatches(FeatureBilling, chatID, threadID) {
				s.startBilling(msg)
			}
		case "push":
			if s.featureMatches(FeatureMonitor, chatID, threadID) {
				s.handlePush(msg)
			}
		}
		return
	}

	if stateValue, ok := s.conversations.Load(msg.From.ID); ok {
		s.advanceConversation(msg, stateValue.(*ConversationState))
	}
}

func (s *BotService) handleCallback(query *tgclient.CallbackQuery) {
	if query.From == nil || query.Message == nil {
		return
	}
	data := query.Data
	prefix, value, _ := strings.Cut(data, ":")

	stateValue, ok := s.conversations.Load(query.From.ID)
	if !ok {
		_ = s.client.AnswerCallbackQuery(query.ID, "会话已过期，请重新开始")
		return
	}
	state := stateValue.(*ConversationState)

	valid := false
	switch state.Command {
	case "getkey":
		valid = prefix == "getkey_group"
	case "adduser":
		valid = prefix == "adduser_group" || prefix == "adduser_remark"
	}
	if !valid {
		_ = s.client.AnswerCallbackQuery(query.ID, "未知操作")
		return
	}

	state.Data[prefix] = value
	state.Step++
	s.conversations.Store(query.From.ID, state)
	_ = s.client.AnswerCallbackQuery(query.ID, "")
	s.promptCurrentStep(query.Message, state)
}

func (s *BotService) promptCurrentStep(msg *tgclient.Message, state *ConversationState) {
	send := func(text string) {
		if err := s.client.SendMessage(msg.Chat.ID, text, msg.MessageThreadID); err != nil {
			common.SysError("tgbot: send prompt failed: " + err.Error())
		}
	}
	switch state.Command {
	case "getkey":
		if state.Step == 1 {
			send("请输入金额（美元）：")
		}
	case "adduser":
		switch state.Step {
		case 1:
			send("请输入用户名：")
		case 2:
			send("请输入密码（至少 8 位）：")
		case 3:
			keyboard := s.remarkKeyboard("adduser_remark")
			if keyboard == nil {
				send("请输入备注：")
				return
			}
			if err := s.client.SendMessageWithKeyboard(msg.Chat.ID, "请选择备注：", msg.MessageThreadID, keyboard); err != nil {
				common.SysError("tgbot: send remark keyboard failed: " + err.Error())
			}
		}
	case "billing":
		if state.Step == 1 {
			send("请输入客户端价格倍率：")
		}
	}
}

func (s *BotService) advanceConversation(msg *tgclient.Message, state *ConversationState) {
	text := strings.TrimSpace(msg.Text)
	fail := func(errText string) {
		_ = s.client.SendMessage(msg.Chat.ID, errText+"\n会话已取消。", msg.MessageThreadID)
		s.conversations.Delete(msg.From.ID)
	}

	switch state.Command {
	case "getkey":
		amount, err := strconv.ParseFloat(text, 64)
		if err != nil || amount <= 0 || amount > 100000 {
			fail("金额无效。")
			return
		}
		state.Data["amount"] = amount
		s.finishGetkey(msg, state)
	case "adduser":
		switch state.Step {
		case 1:
			if len(text) > 20 {
				fail("用户名过长（最多 20 字符）。")
				return
			}
			state.Data["username"] = text
			state.Step = 2
			s.conversations.Store(msg.From.ID, state)
			s.promptCurrentStep(msg, state)
		case 2:
			if len(text) < 8 || len(text) > 128 {
				fail("密码需 8-128 个字符。")
				return
			}
			state.Data["password"] = text
			state.Step = 3
			s.conversations.Store(msg.From.ID, state)
			s.promptCurrentStep(msg, state)
		case 3:
			state.Data["adduser_remark"] = text
			s.finishAdduser(msg, state)
		}
	case "billing":
		multiplier, err := strconv.ParseFloat(text, 64)
		if err != nil || multiplier <= 0 || multiplier > 1000 {
			fail("倍率无效。")
			return
		}
		if state.Step == 0 {
			state.Data["deer_price"] = multiplier
			state.Step = 1
			s.conversations.Store(msg.From.ID, state)
			s.promptCurrentStep(msg, state)
			return
		}
		state.Data["client_price"] = multiplier
		s.finishBilling(msg, state)
	}
}

// --- /getkey ---

func (s *BotService) startGetkey(msg *tgclient.Message) {
	feature := s.getFeature(FeatureGetkey)
	if feature == nil {
		return
	}
	settings := GetGetkeySettings(feature)
	groups, err := loadUserUsableGroups()
	if err != nil {
		common.SysError("tgbot: load usable groups failed: " + err.Error())
	}
	// Filter by allowed groups if configured
	if len(settings.AllowedGroups) > 0 {
		filtered := make(map[string]string)
		for _, g := range settings.AllowedGroups {
			if desc, ok := groups[g]; ok {
				filtered[g] = desc
			}
		}
		groups = filtered
	}
	if len(groups) == 0 {
		_ = s.client.SendMessage(msg.Chat.ID, "没有可用的分组。", msg.MessageThreadID)
		return
	}
	var rows [][]tgclient.InlineKeyboardButton
	for name, desc := range groups {
		label := name
		if desc != "" {
			label = desc + " (" + name + ")"
		}
		rows = append(rows, []tgclient.InlineKeyboardButton{{Text: label, CallbackData: "getkey_group:" + name}})
	}
	s.conversations.Store(msg.From.ID, &ConversationState{Command: "getkey", Step: 0, Data: map[string]any{}})
	if err := s.client.SendMessageWithKeyboard(msg.Chat.ID, "请选择分组：", msg.MessageThreadID,
		&tgclient.InlineKeyboardMarkup{InlineKeyboard: rows}); err != nil {
		common.SysError("tgbot: send group keyboard failed: " + err.Error())
	}
}

func (s *BotService) finishGetkey(msg *tgclient.Message, state *ConversationState) {
	defer s.conversations.Delete(msg.From.ID)
	feature := s.getFeature(FeatureGetkey)
	settings := GetGetkeySettings(feature)
	group, _ := state.Data["getkey_group"].(string)
	amount, _ := state.Data["amount"].(float64)
	quotaPerUnit := 500000
	if settings.QuotaPerUnit > 0 {
		quotaPerUnit = settings.QuotaPerUnit
	}
	quota := int(amount * float64(quotaPerUnit))

	key, err := common.GenerateKey()
	if err != nil {
		common.SysError("tgbot: generate key failed: " + err.Error())
		_ = s.client.SendMessage(msg.Chat.ID, "创建 Key 失败。", msg.MessageThreadID)
		return
	}
	now := common.GetTimestamp()
	token := &model.Token{
		UserId:       1,
		Key:          key,
		Name:         fmt.Sprintf("tg-%s-%d", group, now),
		Status:       1,
		CreatedTime:  now,
		AccessedTime: now,
		ExpiredTime:  -1,
		RemainQuota:  quota,
		Group:        group,
	}
	if err := model.DB.Create(token).Error; err != nil {
		common.SysError("tgbot: create token failed: " + err.Error())
		_ = s.client.SendMessage(msg.Chat.ID, "创建 Key 失败。", msg.MessageThreadID)
		return
	}
	reply := fmt.Sprintf("✅ Key 创建成功\n分组: %s\n金额: $%.2f (%d quota)\n\nsk-%s", group, amount, quota, key)
	if err := s.client.SendMessage(msg.Chat.ID, reply, msg.MessageThreadID); err != nil {
		common.SysError("tgbot: send key failed: " + err.Error())
	}
}

// --- /adduser ---

func (s *BotService) startAdduser(msg *tgclient.Message) {
	feature := s.getFeature(FeatureAdduser)
	if feature == nil {
		return
	}
	settings := GetAdduserSettings(feature)
	groups, err := loadUserUsableGroups()
	if err != nil {
		common.SysError("tgbot: load usable groups failed: " + err.Error())
	}
	if len(settings.AllowedGroups) > 0 {
		filtered := make(map[string]string)
		for _, g := range settings.AllowedGroups {
			if desc, ok := groups[g]; ok {
				filtered[g] = desc
			}
		}
		groups = filtered
	}
	if len(groups) == 0 {
		_ = s.client.SendMessage(msg.Chat.ID, "没有可用的分组。", msg.MessageThreadID)
		return
	}
	var rows [][]tgclient.InlineKeyboardButton
	for name, desc := range groups {
		label := name
		if desc != "" {
			label = desc + " (" + name + ")"
		}
		rows = append(rows, []tgclient.InlineKeyboardButton{{Text: label, CallbackData: "adduser_group:" + name}})
	}
	s.conversations.Store(msg.From.ID, &ConversationState{Command: "adduser", Step: 0, Data: map[string]any{}})
	if err := s.client.SendMessageWithKeyboard(msg.Chat.ID, "请选择用户分组：", msg.MessageThreadID,
		&tgclient.InlineKeyboardMarkup{InlineKeyboard: rows}); err != nil {
		common.SysError("tgbot: send group keyboard failed: " + err.Error())
	}
}

func (s *BotService) remarkKeyboard(callbackPrefix string) *tgclient.InlineKeyboardMarkup {
	feature := s.getFeature(FeatureAdduser)
	if feature == nil {
		return nil
	}
	settings := GetAdduserSettings(feature)

	// Use configured remarks if available
	if len(settings.Remarks) > 0 {
		var rows [][]tgclient.InlineKeyboardButton
		for _, remark := range settings.Remarks {
			rows = append(rows, []tgclient.InlineKeyboardButton{{Text: remark, CallbackData: callbackPrefix + ":" + remark}})
		}
		return &tgclient.InlineKeyboardMarkup{InlineKeyboard: rows}
	}

	// Fallback: query distinct remarks from users table
	var remarks []string
	if err := model.DB.Table("users").
		Where("remark IS NOT NULL AND remark != ''").
		Distinct().
		Pluck("remark", &remarks).Error; err != nil {
		return nil
	}
	if len(remarks) == 0 {
		return nil
	}
	var rows [][]tgclient.InlineKeyboardButton
	for _, remark := range remarks {
		rows = append(rows, []tgclient.InlineKeyboardButton{{Text: remark, CallbackData: callbackPrefix + ":" + remark}})
	}
	return &tgclient.InlineKeyboardMarkup{InlineKeyboard: rows}
}

func (s *BotService) finishAdduser(msg *tgclient.Message, state *ConversationState) {
	defer s.conversations.Delete(msg.From.ID)
	feature := s.getFeature(FeatureAdduser)
	settings := GetAdduserSettings(feature)
	group, _ := state.Data["adduser_group"].(string)
	username, _ := state.Data["username"].(string)
	password, _ := state.Data["password"].(string)
	remark, _ := state.Data["adduser_remark"].(string)
	if group == "" || username == "" || password == "" {
		_ = s.client.SendMessage(msg.Chat.ID, "数据不完整，会话已取消。", msg.MessageThreadID)
		return
	}
	var count int64
	if err := model.DB.Table("users").Where("username = ?", username).Count(&count).Error; err != nil {
		common.SysError("tgbot: check username failed: " + err.Error())
		_ = s.client.SendMessage(msg.Chat.ID, "创建用户失败。", msg.MessageThreadID)
		return
	}
	if count > 0 {
		_ = s.client.SendMessage(msg.Chat.ID, "用户名已存在，会话已取消。", msg.MessageThreadID)
		return
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		common.SysError("tgbot: hash password failed: " + err.Error())
		_ = s.client.SendMessage(msg.Chat.ID, "创建用户失败。", msg.MessageThreadID)
		return
	}
	initialQuota := 100000 * 500000
	if settings.InitialQuota > 0 {
		initialQuota = settings.InitialQuota
	}
	user := &model.User{
		Username:    username,
		Password:    string(hashed),
		DisplayName: username,
		Role:        1,
		Status:      1,
		Group:       group,
		Remark:      remark,
		Quota:       initialQuota,
	}
	if err := model.DB.Create(user).Error; err != nil {
		common.SysError("tgbot: create user failed: " + err.Error())
		_ = s.client.SendMessage(msg.Chat.ID, "创建用户失败。", msg.MessageThreadID)
		return
	}
	reply := fmt.Sprintf("✅ 用户创建成功\n用户名: %s\n分组: %s\n备注: %s", username, group, remark)
	if err := s.client.SendMessage(msg.Chat.ID, reply, msg.MessageThreadID); err != nil {
		common.SysError("tgbot: send adduser confirmation failed: " + err.Error())
	}
}

// --- /billing ---

func (s *BotService) startBilling(msg *tgclient.Message) {
	s.conversations.Store(msg.From.ID, &ConversationState{Command: "billing", Step: 0, Data: map[string]any{}})
	if err := s.client.SendMessage(msg.Chat.ID, "请输入鹿价倍率：", msg.MessageThreadID); err != nil {
		common.SysError("tgbot: send billing prompt failed: " + err.Error())
	}
}

func (s *BotService) finishBilling(msg *tgclient.Message, state *ConversationState) {
	defer s.conversations.Delete(msg.From.ID)
	feature := s.getFeature(FeatureBilling)
	if feature == nil {
		return
	}
	settings := GetBillingSettings(feature)
	deerPrice, _ := state.Data["deer_price"].(float64)
	clientPrice, _ := state.Data["client_price"].(float64)

	rows, err := FetchBilling(settings)
	if err != nil {
		common.SysError("tgbot: fetch billing failed: " + err.Error())
		_ = s.client.SendMessage(msg.Chat.ID, "生成账单失败。", msg.MessageThreadID)
		return
	}
	report := FormatBill(rows, deerPrice, clientPrice, settings.UsdCnyRate, settings.CurrencySymbol)
	if err := s.client.SendHTML(msg.Chat.ID, report, msg.MessageThreadID); err != nil {
		common.SysError("tgbot: send billing report failed: " + err.Error())
	}
}

// --- /push ---

func (s *BotService) handlePush(msg *tgclient.Message) {
	feature := s.getFeature(FeatureMonitor)
	if feature == nil {
		return
	}
	feature.Enabled = !feature.Enabled
	if err := SaveFeature(feature); err != nil {
		common.SysError("tgbot: save feature failed: " + err.Error())
		_ = s.client.SendMessage(msg.Chat.ID, "切换失败。", msg.MessageThreadID)
		return
	}
	status := "关闭"
	if feature.Enabled {
		status = "开启"
	}
	if err := s.client.SendMessage(msg.Chat.ID, "监控推送已"+status+"。", msg.MessageThreadID); err != nil {
		common.SysError("tgbot: send push status failed: " + err.Error())
	}
}

// --- helpers ---

func loadUserUsableGroups() (map[string]string, error) {
	var value string
	if err := model.DB.Model(&model.Option{}).
		Where(&model.Option{Key: "UserUsableGroups"}).
		Select("value").
		Scan(&value).Error; err != nil {
		return nil, err
	}
	if value == "" {
		return map[string]string{"default": "default"}, nil
	}
	groups := map[string]string{}
	if err := common.UnmarshalJsonStr(value, &groups); err != nil {
		return nil, err
	}
	return groups, nil
}
