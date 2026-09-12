package interceptor

import (
	"bytes"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

var (
	// 内存缓存：channelId → 规则列表
	ruleCache   = make(map[int][]Rule)
	cacheMu     sync.RWMutex
	cacheLoaded bool

	toolUseIdRegex = regexp.MustCompile(`[^a-zA-Z0-9_-]`)
)

// ReloadCache 从数据库重载拦截规则到内存
func ReloadCache() error {
	interceptors, err := model.GetAllInterceptors()
	if err != nil {
		return err
	}
	newCache := make(map[int][]Rule)
	for _, it := range interceptors {
		if !it.Enabled {
			continue
		}
		var rules []Rule
		if err := common.Unmarshal([]byte(it.Rules), &rules); err != nil {
			common.SysError(fmt.Sprintf("interceptor %d rules parse error: %v", it.Id, err))
			continue
		}
		newCache[it.ChannelId] = append(newCache[it.ChannelId], rules...)
	}
	cacheMu.Lock()
	ruleCache = newCache
	cacheLoaded = true
	cacheMu.Unlock()
	return nil
}

// getRules 获取指定渠道的规则（全局 + 渠道专属）
func getRules(channelId int) []Rule {
	cacheMu.RLock()
	defer cacheMu.RUnlock()
	if !cacheLoaded {
		return nil
	}
	var result []Rule
	// 全局规则
	if global, ok := ruleCache[0]; ok {
		result = append(result, global...)
	}
	// 渠道专属规则
	if channelId != 0 {
		if ch, ok := ruleCache[channelId]; ok {
			result = append(result, ch...)
		}
	}
	return result
}

// ProcessRequest 拦截并处理请求体
// 返回新的 io.Reader 和是否被拒绝的错误
func ProcessRequest(channelId int, requestBody io.Reader, info *relaycommon.RelayInfo) (io.Reader, error) {
	rules := getRules(channelId)
	if len(rules) == 0 {
		return requestBody, nil
	}

	// 读取 body
	bodyBytes, err := io.ReadAll(requestBody)
	if err != nil {
		return nil, fmt.Errorf("interceptor read body failed: %w", err)
	}

	// 体积检查放在解析之前：超大 body 解析本身就是浪费。
	if ruleType, err := rejectLargeBody(rules, len(bodyBytes)); err != nil {
		logInterceptor(channelId, info, ruleType, "rejected", err.Error(), "", "")
		return nil, err
	}

	// 解析 JSON
	var data map[string]any
	if err := common.Unmarshal(bodyBytes, &data); err != nil {
		// 不是 JSON 直接放行
		return bytes.NewReader(bodyBytes), nil
	}

	originalBody := string(bodyBytes)
	modified := false
	var appliedRules []string

	for _, rule := range rules {
		changed, rejectErr := applyRule(data, rule, upstreamModelName(data, info))
		if rejectErr != nil {
			// 记录拒绝日志
			logInterceptor(channelId, info, rule.Type, "rejected", rejectErr.Error(), originalBody, originalBody)
			return nil, rejectErr
		}
		if changed {
			modified = true
			appliedRules = append(appliedRules, rule.Type)
		}
	}

	if !modified {
		return bytes.NewReader(bodyBytes), nil
	}

	newBody, err := common.Marshal(data)
	if err != nil {
		return bytes.NewReader(bodyBytes), nil
	}

	// 记录修改日志
	modifiedBody := string(newBody)
	for _, ruleType := range appliedRules {
		logInterceptor(channelId, info, ruleType, "modified", "", originalBody, modifiedBody)
	}

	return bytes.NewReader(newBody), nil
}

// logInterceptor 异步记录拦截日志
func logInterceptor(channelId int, info *relaycommon.RelayInfo, ruleType, action, rejectReason, originalBody, modifiedBody string) {
	if info == nil {
		return
	}
	// 异步写日志，不阻塞请求
	go func() {
		// 限制 body 长度避免日志表膨胀
		const maxBodyLen = 20000
		origTrunc := truncateUTF8(originalBody, maxBodyLen)
		modTrunc := truncateUTF8(modifiedBody, maxBodyLen)

		// 提取模型名
		modelName := ""
		if info != nil {
			modelName = info.OriginModelName
		}

		username := ""
		userId := 0
		tokenId := 0
		if info != nil {
			userId = info.UserId
			tokenId = info.TokenId
			if userId > 0 {
				if user, err := model.GetUserById(userId, false); err == nil && user != nil {
					username = user.Username
				}
			}
		}

		log := &model.InterceptorLog{
			UserId:       userId,
			Username:     username,
			TokenId:      tokenId,
			ChannelId:    channelId,
			ModelName:    modelName,
			RuleType:     ruleType,
			Action:       action,
			RejectReason: rejectReason,
			RequestBody:  origTrunc,
			ModifiedBody: modTrunc,
			CreatedAt:    time.Now().Unix(),
		}

		if err := model.CreateInterceptorLog(log); err != nil {
			common.SysError(fmt.Sprintf("interceptor log create error: %v", err))
		}
	}()
}

// applyRule 执行单条规则，返回 (是否修改, 拒绝错误)
func applyRule(data map[string]any, rule Rule, modelName string) (bool, error) {
	switch rule.Type {
	case RuleFixContentArray:
		return fixContentArray(data), nil
	case RuleFixToolsArray:
		return fixToolsArray(data), nil
	case RuleFixSchemaRequired:
		return fixSchemaFieldRecursive(data, "required"), nil
	case RuleFixSchemaCitations:
		return fixSchemaFieldRecursive(data, "citations"), nil
	case RuleFixSchemaProperties:
		return fixSchemaProperties(data), nil
	case RuleStripThinking:
		return stripThinking(data), nil
	case RuleStripReasoning:
		return stripReasoning(data), nil
	case RuleStripAnnotations:
		return stripAnnotations(data), nil
	case RuleStripEmptyText:
		return stripEmptyText(data), nil
	case RuleTrimWhitespace:
		return trimWhitespace(data), nil
	case RuleCapMaxTokens:
		cap := getConfigInt(rule.Config, "max", 128000)
		return capMaxTokens(data, cap), nil
	case RuleFixSystemField:
		return fixSystemField(data), nil
	case RuleStripTemperature:
		return stripTemperatureIfThinking(data), nil
	case RuleStripTopPK:
		return stripTopPKIfThinking(data), nil
	case RuleFixEffort:
		return fixEffort(data), nil
	case RuleFilterBeta:
		// 这个在 header 层处理，body 层不做
		return false, nil
	case RuleFilterToolTypes:
		return filterToolTypes(data, rule.Config), nil
	case RuleFixThinkingType:
		return fixThinkingType(data), nil
	case RuleFixToolUseId:
		return fixToolUseId(data), nil
	case RuleClampTopP:
		return clampFloat(data, "top_p", 0, 1), nil
	case RuleClampTopK:
		return clampFloat(data, "top_k", 0, 100000000), nil
	case RuleRejectLongPrompt:
		maxChars := getConfigInt(rule.Config, "max_chars", 1800000)
		return false, rejectLongPrompt(data, maxChars)
	case RuleFixSchemaOneOf:
		return fixSchemaOneOf(data), nil
	case RuleStripAssistPrefill:
		return stripAssistantPrefill(data), nil
	case RuleFixAdditionalProps:
		return fixAdditionalProperties(data), nil

	// 针对生产实测残留 400 报错补充的规则
	case RuleHoistSystem:
		return hoistSystemMessages(data), nil
	case RuleFixEmptySystem:
		return fixEmptySystem(data), nil
	case RuleStripURLSource:
		return stripURLSources(data), nil
	case RuleFixMessageRoles:
		return fixMessageRoles(data), nil
	case RuleFixToolChoice:
		denied := getConfigStrings(rule.Config, "denied_types")
		if len(denied) == 0 {
			denied = []string{"tool", "any"}
		}
		return fixToolChoice(data, denied), nil
	case RuleFixDeferLoading:
		return fixDeferLoading(data), nil
	case RuleFixOrphanToolResult:
		return fixOrphanToolResult(data), nil
	case RuleFixDanglingToolUse:
		return fixDanglingToolUse(data), nil
	case RuleFixToolName:
		return fixToolName(data), nil
	case RuleFixTempTopPConflict:
		return fixTempTopPConflict(data), nil
	case RuleRejectEmptyMessages:
		return false, rejectEmptyMessages(data)
	case RuleFixThinkingBudget:
		return fixThinkingBudget(data), nil
	case RuleStripParamsForModel:
		params := getConfigStrings(rule.Config, "params")
		if len(params) == 0 {
			params = []string{"temperature", "top_p", "top_k"}
		}
		return stripParamsForModel(data, modelName, getConfigStrings(rule.Config, "models"), params), nil

	// 自定义规则
	case RuleCustomDeletePath:
		path, _ := rule.Config["path"].(string)
		if path == "" {
			return false, nil
		}
		return customDeletePath(data, path), nil
	case RuleCustomSetValue:
		path, _ := rule.Config["path"].(string)
		if path == "" {
			return false, nil
		}
		return customSetValue(data, path, rule.Config["value"]), nil
	case RuleCustomDeleteKeyAll:
		key, _ := rule.Config["key"].(string)
		if key == "" {
			return false, nil
		}
		return removeKeysRecursive(data, []string{key}, 0), nil
	case RuleCustomEnsureArray:
		path, _ := rule.Config["path"].(string)
		if path == "" {
			return false, nil
		}
		return customEnsureArray(data, path), nil
	case RuleCustomRenameKey:
		path, _ := rule.Config["path"].(string)
		oldKey, _ := rule.Config["old_key"].(string)
		newKey, _ := rule.Config["new_key"].(string)
		if oldKey == "" || newKey == "" {
			return false, nil
		}
		return customRenameKey(data, path, oldKey, newKey), nil
	case RuleCustomRejectIf:
		path, _ := rule.Config["path"].(string)
		op, _ := rule.Config["op"].(string)
		if path == "" || op == "" {
			return false, nil
		}
		msg, _ := rule.Config["message"].(string)
		if msg == "" {
			msg = "request rejected by custom rule"
		}
		return false, customRejectIf(data, path, op, rule.Config["value"], msg)
	default:
		return false, nil
	}
}

// ==================== 规则实现 ====================

func getMessages(data map[string]any) ([]any, bool) {
	msgs, ok := data["messages"]
	if !ok {
		return nil, false
	}
	arr, ok := msgs.([]any)
	return arr, ok
}

func getTools(data map[string]any) ([]any, bool) {
	tools, ok := data["tools"]
	if !ok {
		return nil, false
	}
	arr, ok := tools.([]any)
	return arr, ok
}

func isThinkingEnabled(data map[string]any) bool {
	thinking, ok := data["thinking"].(map[string]any)
	if !ok {
		return false
	}
	t, _ := thinking["type"].(string)
	return t == "enabled" || t == "adaptive"
}

func getConfigInt(config map[string]any, key string, def int) int {
	if config == nil {
		return def
	}
	v, ok := config[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return def
}

// fixContentArray: messages[N].content 字符串 → 数组
func fixContentArray(data map[string]any) bool {
	msgs, ok := getMessages(data)
	if !ok {
		return false
	}
	changed := false
	for _, m := range msgs {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		content := msg["content"]
		switch content.(type) {
		case string:
			msg["content"] = []any{map[string]any{"type": "text", "text": content}}
			changed = true
		case []any:
			// 已经是数组，检查内部 tool_result 的 content
			arr := content.([]any)
			for _, block := range arr {
				b, ok := block.(map[string]any)
				if !ok {
					continue
				}
				if b["type"] == "tool_result" {
					if tc, ok := b["content"].(string); ok {
						b["content"] = []any{map[string]any{"type": "text", "text": tc}}
						changed = true
					}
				}
			}
		case nil:
			// nil 不处理
		default:
			// 其他类型转文本
			msg["content"] = []any{map[string]any{"type": "text", "text": fmt.Sprintf("%v", content)}}
			changed = true
		}
	}
	return changed
}

// fixToolsArray: tools 不是数组 → 修
func fixToolsArray(data map[string]any) bool {
	tools, exists := data["tools"]
	if !exists || tools == nil {
		return false
	}
	switch t := tools.(type) {
	case []any:
		if len(t) == 0 {
			delete(data, "tools")
			return true
		}
		// 过滤无效 tool
		valid := make([]any, 0, len(t))
		changed := false
		for _, tool := range t {
			tm, ok := tool.(map[string]any)
			if !ok {
				changed = true
				continue
			}
			if tm["name"] == nil {
				// 检查 function.name (OpenAI 格式)
				if fn, ok := tm["function"].(map[string]any); ok && fn["name"] != nil {
					valid = append(valid, tm)
				} else {
					changed = true
				}
			} else {
				valid = append(valid, tm)
			}
		}
		if changed {
			if len(valid) == 0 {
				delete(data, "tools")
			} else {
				data["tools"] = valid
			}
		}
		return changed
	case map[string]any:
		// 对象包成数组
		data["tools"] = []any{t}
		return true
	default:
		delete(data, "tools")
		return true
	}
}

// fixSchemaFieldRecursive: 递归修复必须为数组的字段
func fixSchemaFieldRecursive(data map[string]any, field string) bool {
	return fixFieldRecursive(data, field, 0)
}

func fixFieldRecursive(obj map[string]any, field string, depth int) bool {
	if depth > 20 {
		return false
	}
	changed := false
	for k, v := range obj {
		if k == field {
			switch val := v.(type) {
			case []any:
				// 已是数组
			case string:
				if val != "" {
					obj[k] = []any{val}
				} else {
					delete(obj, k)
				}
				changed = true
			default:
				if v != nil {
					delete(obj, k)
					changed = true
				}
			}
		} else if nested, ok := v.(map[string]any); ok {
			if fixFieldRecursive(nested, field, depth+1) {
				changed = true
			}
		} else if arr, ok := v.([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					if fixFieldRecursive(m, field, depth+1) {
						changed = true
					}
				}
			}
		}
	}
	return changed
}

// fixSchemaProperties: properties 不是 dict → 删
func fixSchemaProperties(data map[string]any) bool {
	return fixPropertiesRecursive(data, 0)
}

func fixPropertiesRecursive(obj map[string]any, depth int) bool {
	if depth > 20 {
		return false
	}
	changed := false
	for k, v := range obj {
		if k == "properties" {
			if _, ok := v.(map[string]any); !ok && v != nil {
				delete(obj, k)
				changed = true
			}
		}
		if nested, ok := v.(map[string]any); ok {
			if fixPropertiesRecursive(nested, depth+1) {
				changed = true
			}
		}
		if arr, ok := v.([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					if fixPropertiesRecursive(m, depth+1) {
						changed = true
					}
				}
			}
		}
	}
	return changed
}

// stripThinking: 去掉 thinking/redacted_thinking block
func stripThinking(data map[string]any) bool {
	msgs, ok := getMessages(data)
	if !ok {
		return false
	}
	changed := false
	for _, m := range msgs {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		content, ok := msg["content"].([]any)
		if !ok {
			continue
		}
		filtered := make([]any, 0, len(content))
		for _, block := range content {
			b, ok := block.(map[string]any)
			if !ok {
				filtered = append(filtered, block)
				continue
			}
			t, _ := b["type"].(string)
			if t == "thinking" || t == "redacted_thinking" {
				changed = true
				continue
			}
			filtered = append(filtered, block)
		}
		if changed {
			msg["content"] = filtered
		}
	}
	return changed
}

// stripReasoning: 去掉 messages 里的 reasoning 字段
func stripReasoning(data map[string]any) bool {
	msgs, ok := getMessages(data)
	if !ok {
		return false
	}
	changed := false
	for _, m := range msgs {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if _, exists := msg["reasoning"]; exists {
			delete(msg, "reasoning")
			changed = true
		}
	}
	return changed
}

// stripAnnotations: 去掉 content block 里的 annotations
func stripAnnotations(data map[string]any) bool {
	msgs, ok := getMessages(data)
	if !ok {
		return false
	}
	changed := false
	for _, m := range msgs {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		content, ok := msg["content"].([]any)
		if !ok {
			continue
		}
		for _, block := range content {
			b, ok := block.(map[string]any)
			if !ok {
				continue
			}
			if _, exists := b["annotations"]; exists {
				delete(b, "annotations")
				changed = true
			}
		}
	}
	return changed
}

// stripEmptyText: 去掉空 text block
func stripEmptyText(data map[string]any) bool {
	msgs, ok := getMessages(data)
	if !ok {
		return false
	}
	changed := false
	for _, m := range msgs {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		content, ok := msg["content"].([]any)
		if !ok {
			continue
		}
		// 每条消息单独判断，否则前一条改过就会连带重写后面所有消息。
		stripped := false
		filtered := make([]any, 0, len(content))
		for _, block := range content {
			b, ok := block.(map[string]any)
			if ok {
				t, _ := b["type"].(string)
				text, _ := b["text"].(string)
				if t == "text" && text == "" {
					stripped = true
					continue
				}
			}
			filtered = append(filtered, block)
		}
		if !stripped {
			continue
		}
		// 上游对空 content 数组和空 text 块都会 400，两边都躲不过去，
		// 所以清空时补一个占位文本块（生产实测：探活请求只带一个空 text 块）。
		if len(filtered) == 0 {
			filtered = append(filtered, map[string]any{
				"type": "text",
				"text": "(empty)",
			})
		}
		msg["content"] = filtered
		changed = true
	}
	return changed
}

// trimWhitespace: 修剪 assistant 消息尾部空白
func trimWhitespace(data map[string]any) bool {
	msgs, ok := getMessages(data)
	if !ok {
		return false
	}
	changed := false
	for i := len(msgs) - 1; i >= 0; i-- {
		msg, ok := msgs[i].(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role != "assistant" {
			continue
		}
		if s, ok := msg["content"].(string); ok {
			trimmed := strings.TrimRight(s, " \t\n\r")
			if trimmed != s {
				msg["content"] = trimmed
				changed = true
			}
		}
	}
	return changed
}

// capMaxTokens: 限制 max_tokens
func capMaxTokens(data map[string]any, cap int) bool {
	mt, ok := data["max_tokens"].(float64)
	if ok && int(mt) > cap {
		data["max_tokens"] = cap
		return true
	}
	return false
}

// fixSystemField: system 字符串 → 数组
func fixSystemField(data map[string]any) bool {
	sys, ok := data["system"].(string)
	if ok {
		data["system"] = []any{map[string]any{"type": "text", "text": sys}}
		return true
	}
	return false
}

// stripTemperatureIfThinking: thinking 时删 temperature
func stripTemperatureIfThinking(data map[string]any) bool {
	if !isThinkingEnabled(data) {
		return false
	}
	if _, exists := data["temperature"]; exists {
		delete(data, "temperature")
		return true
	}
	return false
}

// stripTopPKIfThinking: thinking 时删 top_p/top_k
func stripTopPKIfThinking(data map[string]any) bool {
	if !isThinkingEnabled(data) {
		return false
	}
	changed := false
	for _, key := range []string{"top_p", "top_k"} {
		if _, exists := data[key]; exists {
			delete(data, key)
			changed = true
		}
	}
	return changed
}

// fixEffort: 非 thinking 时 effort max → high
func fixEffort(data map[string]any) bool {
	if isThinkingEnabled(data) {
		return false
	}
	// thinking 关闭时上游同时拒绝 max 和 xhigh，都降到 high。
	if oc, ok := data["output_config"].(map[string]any); ok {
		if effort, _ := oc["effort"].(string); effort == "max" || effort == "xhigh" {
			oc["effort"] = "high"
			return true
		}
	}
	if effort, _ := data["effort"].(string); effort == "max" || effort == "xhigh" {
		data["effort"] = "high"
		return true
	}
	return false
}

// filterToolTypes: 过滤不支持的 tool type
// filterToolTypes 处理 tools 数组里上游不认的 type 值。
// 上游会改动内置工具的 type 命名（例如 computer_20250124 改成
// computer_toolset_20260801），命中 rename_types 的改名，命中 denied_types 的整项删掉，
// 配了 allowed_types 则只放行清单内的 type。三份清单都来自规则参数，
// 上游再改命名只需改配置；都没配时退回内置白名单。
func filterToolTypes(data map[string]any, config map[string]any) bool {
	tools, ok := getTools(data)
	if !ok {
		return false
	}
	denied := getConfigStrings(config, "denied_types")
	allowed := getConfigStrings(config, "allowed_types")
	renames, _ := config["rename_types"].(map[string]any)
	stripFields, _ := config["strip_fields"].(map[string]any)
	if len(denied) == 0 && len(allowed) == 0 && len(renames) == 0 && len(stripFields) == 0 {
		return filterToolTypesByAllowlist(data, tools)
	}

	changed := false
	kept := make([]any, 0, len(tools))
	for _, tool := range tools {
		tm, ok := tool.(map[string]any)
		if !ok {
			kept = append(kept, tool)
			continue
		}
		toolType, _ := tm["type"].(string)
		if toolType == "" {
			kept = append(kept, tool)
			continue
		}
		if newType, ok := renames[toolType].(string); ok && newType != "" {
			tm["type"] = newType
			stripToolFields(tm, stripFields, newType)
			kept = append(kept, tm)
			changed = true
			continue
		}
		if stripToolFields(tm, stripFields, toolType) {
			changed = true
		}
		if slices.Contains(denied, toolType) {
			changed = true
			continue
		}
		// allowed_types 配了就以它为准，取代内置白名单。
		if len(allowed) > 0 && !slices.Contains(allowed, toolType) {
			changed = true
			continue
		}
		kept = append(kept, tool)
	}
	if !changed {
		return false
	}
	// tools 变成空数组同样会被上游拒绝，连带 tool_choice 一起删掉。
	if len(kept) == 0 {
		delete(data, "tools")
		delete(data, "tool_choice")
		return true
	}
	data["tools"] = kept
	return true
}

// stripToolFields 删掉某个 tool type 不接受的字段。
// toolset 类型的条目成员名是固定的，带上 name 会被上游拒绝。
func stripToolFields(tool map[string]any, stripFields map[string]any, toolType string) bool {
	if len(stripFields) == 0 {
		return false
	}
	fields, ok := stripFields[toolType]
	if !ok {
		return false
	}
	changed := false
	for _, f := range getConfigStrings(map[string]any{"f": fields}, "f") {
		if _, exists := tool[f]; exists {
			delete(tool, f)
			changed = true
		}
	}
	return changed
}

func filterToolTypesByAllowlist(data map[string]any, tools []any) bool {
	supported := map[string]bool{
		"custom":               true,
		"computer_20250124":    true,
		"bash_20250124":        true,
		"text_editor_20250124": true,
		"":                     true, // 没有 type 的也放行
	}
	filtered := make([]any, 0, len(tools))
	changed := false
	for _, tool := range tools {
		tm, ok := tool.(map[string]any)
		if !ok {
			continue
		}
		toolType, _ := tm["type"].(string)
		if supported[toolType] {
			filtered = append(filtered, tool)
		} else {
			changed = true
		}
	}
	if changed {
		if len(filtered) == 0 {
			delete(data, "tools")
		} else {
			data["tools"] = filtered
		}
	}
	return changed
}

// fixThinkingType: enabled → adaptive
func fixThinkingType(data map[string]any) bool {
	thinking, ok := data["thinking"].(map[string]any)
	if !ok {
		return false
	}
	if thinking["type"] == "enabled" {
		thinking["type"] = "adaptive"
		return true
	}
	return false
}

// fixToolUseId: tool_use.id 修复为合法字符
func fixToolUseId(data map[string]any) bool {
	msgs, ok := getMessages(data)
	if !ok {
		return false
	}
	changed := false
	for _, m := range msgs {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		content, ok := msg["content"].([]any)
		if !ok {
			continue
		}
		for _, block := range content {
			b, ok := block.(map[string]any)
			if !ok {
				continue
			}
			t, _ := b["type"].(string)
			if t == "tool_use" {
				if id, ok := b["id"].(string); ok {
					cleaned := toolUseIdRegex.ReplaceAllString(id, "_")
					if cleaned != id {
						b["id"] = cleaned
						changed = true
					}
				}
			}
		}
	}
	return changed
}

// clampFloat: 限制浮点字段范围
func clampFloat(data map[string]any, key string, min, max float64) bool {
	v, ok := data[key].(float64)
	if !ok {
		return false
	}
	if v < min {
		data[key] = min
		return true
	}
	if v > max {
		data[key] = max
		return true
	}
	return false
}

// rejectLongPrompt: 超长 prompt 拒绝
func rejectLongPrompt(data map[string]any, maxChars int) error {
	msgs, ok := getMessages(data)
	if !ok {
		return nil
	}
	total := 0
	for _, m := range msgs {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		content := msg["content"]
		switch c := content.(type) {
		case string:
			total += len(c)
		case []any:
			for _, block := range c {
				b, ok := block.(map[string]any)
				if !ok {
					continue
				}
				if text, ok := b["text"].(string); ok {
					total += len(text)
				}
			}
		}
	}
	if total > maxChars {
		return fmt.Errorf("prompt too long: ~%d chars, max %d", total, maxChars)
	}
	return nil
}

// fixSchemaOneOf: 递归删掉 oneOf/allOf/anyOf
func fixSchemaOneOf(data map[string]any) bool {
	return removeKeysRecursive(data, []string{"oneOf", "allOf", "anyOf"}, 0)
}

func removeKeysRecursive(obj map[string]any, keys []string, depth int) bool {
	if depth > 20 {
		return false
	}
	changed := false
	for _, key := range keys {
		if _, exists := obj[key]; exists {
			delete(obj, key)
			changed = true
		}
	}
	for _, v := range obj {
		if nested, ok := v.(map[string]any); ok {
			if removeKeysRecursive(nested, keys, depth+1) {
				changed = true
			}
		}
		if arr, ok := v.([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					if removeKeysRecursive(m, keys, depth+1) {
						changed = true
					}
				}
			}
		}
	}
	return changed
}

// stripAssistantPrefill: 如果最后一条消息是 assistant 就删掉
func stripAssistantPrefill(data map[string]any) bool {
	msgs, ok := getMessages(data)
	if !ok || len(msgs) == 0 {
		return false
	}
	last, ok := msgs[len(msgs)-1].(map[string]any)
	if !ok {
		return false
	}
	if last["role"] == "assistant" {
		data["messages"] = msgs[:len(msgs)-1]
		return true
	}
	return false
}

// fixAdditionalProperties: 递归给 type=object 的 schema 补 additionalProperties: false
func fixAdditionalProperties(data map[string]any) bool {
	return fixAdditionalPropsRecursive(data, 0)
}

func fixAdditionalPropsRecursive(obj map[string]any, depth int) bool {
	if depth > 20 {
		return false
	}
	changed := false
	if obj["type"] == "object" {
		if _, exists := obj["additionalProperties"]; !exists {
			obj["additionalProperties"] = false
			changed = true
		}
	}
	for _, v := range obj {
		if nested, ok := v.(map[string]any); ok {
			if fixAdditionalPropsRecursive(nested, depth+1) {
				changed = true
			}
		}
		if arr, ok := v.([]any); ok {
			for _, item := range arr {
				if m, ok := item.(map[string]any); ok {
					if fixAdditionalPropsRecursive(m, depth+1) {
						changed = true
					}
				}
			}
		}
	}
	return changed
}

// ==================== 自定义规则实现 ====================

// navigatePath 按点分路径导航到父对象，返回 (父map, 最后一段key, 是否找到)
// 路径例子: "a.b.c" → 导航到 data["a"]["b"]，返回该 map 和 "c"
func navigatePath(data map[string]any, path string) (map[string]any, string, bool) {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil, "", false
	}
	current := data
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			return nil, "", false
		}
		current = next
	}
	return current, parts[len(parts)-1], true
}

// getValueAtPath 获取指定路径的值
func getValueAtPath(data map[string]any, path string) (any, bool) {
	parent, key, ok := navigatePath(data, path)
	if !ok {
		return nil, false
	}
	val, exists := parent[key]
	return val, exists
}

// customDeletePath 按路径删除字段
func customDeletePath(data map[string]any, path string) bool {
	parent, key, ok := navigatePath(data, path)
	if !ok {
		return false
	}
	if _, exists := parent[key]; !exists {
		return false
	}
	delete(parent, key)
	return true
}

// customSetValue 按路径设置值
func customSetValue(data map[string]any, path string, value any) bool {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return false
	}
	// 自动创建中间路径
	current := data
	for _, part := range parts[:len(parts)-1] {
		next, ok := current[part].(map[string]any)
		if !ok {
			next = make(map[string]any)
			current[part] = next
		}
		current = next
	}
	lastKey := parts[len(parts)-1]
	current[lastKey] = value
	return true
}

// customEnsureArray 确保指定路径的值是数组
func customEnsureArray(data map[string]any, path string) bool {
	parent, key, ok := navigatePath(data, path)
	if !ok {
		return false
	}
	val, exists := parent[key]
	if !exists || val == nil {
		return false
	}
	if _, isArr := val.([]any); isArr {
		return false
	}
	parent[key] = []any{val}
	return true
}

// customRenameKey 重命名字段
// path 为空时在顶层操作，非空时导航到 path 指向的 map 再操作
func customRenameKey(data map[string]any, path, oldKey, newKey string) bool {
	target := data
	if path != "" {
		parts := strings.Split(path, ".")
		for _, part := range parts {
			next, ok := target[part].(map[string]any)
			if !ok {
				return false
			}
			target = next
		}
	}
	val, exists := target[oldKey]
	if !exists {
		return false
	}
	target[newKey] = val
	delete(target, oldKey)
	return true
}

// customRejectIf 条件拒绝
// op: exists, not_exists, eq, ne, gt, lt, contains
func customRejectIf(data map[string]any, path, op string, value any, msg string) error {
	val, exists := getValueAtPath(data, path)

	matched := false
	switch op {
	case "exists":
		matched = exists
	case "not_exists":
		matched = !exists
	case "eq":
		matched = exists && fmt.Sprintf("%v", val) == fmt.Sprintf("%v", value)
	case "ne":
		matched = exists && fmt.Sprintf("%v", val) != fmt.Sprintf("%v", value)
	case "gt":
		vf, vOk := toFloat(val)
		cf, cOk := toFloat(value)
		matched = vOk && cOk && vf > cf
	case "lt":
		vf, vOk := toFloat(val)
		cf, cOk := toFloat(value)
		matched = vOk && cOk && vf < cf
	case "contains":
		if s, ok := val.(string); ok {
			if sub, ok := value.(string); ok {
				matched = strings.Contains(s, sub)
			}
		}
	}

	if matched {
		return fmt.Errorf("%s", msg)
	}
	return nil
}

func toFloat(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case string:
		// 不转换字符串
		return 0, false
	}
	return 0, false
}
