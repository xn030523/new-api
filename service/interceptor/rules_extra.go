package interceptor

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// 这些规则针对生产日志中实测仍然发生的 400 报错，补齐 engine.go 里既有规则未覆盖的分支。

// hoistSystemMessages 把 messages 里 role=system 的消息合并进顶层 system。
// Anthropic 只接受顶层 system 参数，messages 里出现 system 角色会直接 400。
func hoistSystemMessages(data map[string]any) bool {
	msgs, ok := getMessages(data)
	if !ok {
		return false
	}
	var hoisted []any
	kept := make([]any, 0, len(msgs))
	for _, m := range msgs {
		msg, ok := m.(map[string]any)
		if !ok {
			kept = append(kept, m)
			continue
		}
		if role, _ := msg["role"].(string); role != "system" {
			kept = append(kept, m)
			continue
		}
		hoisted = append(hoisted, systemBlocksFromContent(msg["content"])...)
	}
	if len(hoisted) == 0 {
		return false
	}

	existing := systemBlocksFromContent(data["system"])
	// 原有顶层 system 是初始指令，必须排在被上提的消息之前。
	data["system"] = append(existing, hoisted...)
	data["messages"] = kept
	return true
}

// systemBlocksFromContent 把 string / []any / nil 形态的 content 统一成 text block 数组。
func systemBlocksFromContent(content any) []any {
	switch v := content.(type) {
	case string:
		if v == "" {
			return nil
		}
		return []any{map[string]any{"type": "text", "text": v}}
	case []any:
		out := make([]any, 0, len(v))
		for _, block := range v {
			b, ok := block.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := b["type"].(string); t != "text" {
				continue
			}
			if text, _ := b["text"].(string); strings.TrimSpace(text) == "" {
				continue
			}
			out = append(out, b)
		}
		return out
	default:
		return nil
	}
}

// fixEmptySystem 清掉顶层 system 里的空 text block。
// 上游报 "system: text content blocks must be non-empty"；engine.go 的
// stripEmptyText 只遍历 messages[].content，不看顶层 system。
func fixEmptySystem(data map[string]any) bool {
	raw, exists := data["system"]
	if !exists {
		return false
	}
	blocks, ok := raw.([]any)
	if !ok {
		return false
	}
	filtered := make([]any, 0, len(blocks))
	for _, block := range blocks {
		b, ok := block.(map[string]any)
		if !ok {
			filtered = append(filtered, block)
			continue
		}
		if t, _ := b["type"].(string); t == "text" {
			if text, _ := b["text"].(string); strings.TrimSpace(text) == "" {
				continue
			}
		}
		filtered = append(filtered, block)
	}
	if len(filtered) == len(blocks) {
		return false
	}
	// 全空时整个字段删掉，留一个空数组同样会被上游拒绝。
	if len(filtered) == 0 {
		delete(data, "system")
	} else {
		data["system"] = filtered
	}
	return true
}

// stripURLSources 删除 source.type == "url" 的 content block。
// 上游报 "URL sources are not supported"，只有 base64 形态可用。
func stripURLSources(data map[string]any) bool {
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
			if b, ok := block.(map[string]any); ok {
				if source, ok := b["source"].(map[string]any); ok {
					if st, _ := source["type"].(string); st == "url" {
						changed = true
						continue
					}
				}
			}
			filtered = append(filtered, block)
		}
		if len(filtered) != len(content) {
			msg["content"] = filtered
		}
	}
	return changed
}

// fixMessageRoles 把非法 role 归一到 user。
// 上游只接受 user / assistant / system。
func fixMessageRoles(data map[string]any) bool {
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
		role, _ := msg["role"].(string)
		switch role {
		case "user", "assistant", "system":
			continue
		}
		// tool / function 等角色在 Anthropic 语义里等价于携带 tool_result 的 user 消息。
		msg["role"] = "user"
		changed = true
	}
	return changed
}

// fixToolChoice 删掉上游不支持的 tool_choice 形态，让请求退回默认的 auto 行为。
// 覆盖两类报错：不支持的 type，以及 tool_choice 指向 tools 里不存在的名字。
func fixToolChoice(data map[string]any, deniedTypes []string) bool {
	choice, ok := data["tool_choice"].(map[string]any)
	if !ok {
		return false
	}
	choiceType, _ := choice["type"].(string)
	for _, denied := range deniedTypes {
		if choiceType == denied {
			delete(data, "tool_choice")
			return true
		}
	}
	// type=tool 时 name 必须在 tools 里存在，否则上游报 Tool 'x' not found。
	if choiceType != "tool" {
		return false
	}
	name, _ := choice["name"].(string)
	if name == "" {
		delete(data, "tool_choice")
		return true
	}
	tools, ok := getTools(data)
	if !ok {
		delete(data, "tool_choice")
		return true
	}
	for _, t := range tools {
		tool, ok := t.(map[string]any)
		if !ok {
			continue
		}
		if toolName, _ := tool["name"].(string); toolName == name {
			return false
		}
	}
	delete(data, "tool_choice")
	return true
}

// fixDeferLoading 保证至少有一个 tool 的 defer_loading 不是 true。
// 上游报 "At least one tool must have defer_loading=false"。
func fixDeferLoading(data map[string]any) bool {
	tools, ok := getTools(data)
	if !ok || len(tools) == 0 {
		return false
	}
	var first map[string]any
	for _, t := range tools {
		tool, ok := t.(map[string]any)
		if !ok {
			continue
		}
		if deferred, _ := tool["defer_loading"].(bool); !deferred {
			return false
		}
		if first == nil {
			first = tool
		}
	}
	if first == nil {
		return false
	}
	first["defer_loading"] = false
	return true
}

// fixOrphanToolResult 删掉找不到对应 tool_use 的 tool_result block。
// 上游报 "unexpected `tool_use_id` found in `tool_result` blocks"。
// orphanToolResultText 把被丢弃的孤儿 tool_result 还原成可读文本，
// 避免降级过程丢掉工具返回的内容。
func orphanToolResultText(orphans []any) string {
	var sb strings.Builder
	for _, o := range orphans {
		b, ok := o.(map[string]any)
		if !ok {
			continue
		}
		switch c := b["content"].(type) {
		case string:
			sb.WriteString(c)
		case []any:
			for _, item := range c {
				block, ok := item.(map[string]any)
				if !ok {
					continue
				}
				if txt, ok := block["text"].(string); ok {
					sb.WriteString(txt)
				}
			}
		}
	}
	if sb.Len() == 0 {
		return "(tool result omitted)"
	}
	return sb.String()
}

func fixOrphanToolResult(data map[string]any) bool {
	msgs, ok := getMessages(data)
	if !ok {
		return false
	}
	changed := false
	// 只有紧邻的前一条消息里的 tool_use 才是合法来源。
	available := map[string]bool{}
	kept := make([]any, 0, len(msgs))
	for _, m := range msgs {
		msg, ok := m.(map[string]any)
		if !ok {
			kept = append(kept, m)
			continue
		}
		content, isArray := msg["content"].([]any)
		if !isArray {
			kept = append(kept, m)
			available = map[string]bool{}
			continue
		}

		filtered := make([]any, 0, len(content))
		orphans := make([]any, 0, len(content))
		for _, block := range content {
			b, ok := block.(map[string]any)
			if !ok {
				filtered = append(filtered, block)
				continue
			}
			if t, _ := b["type"].(string); t == "tool_result" {
				id, _ := b["tool_use_id"].(string)
				if !available[id] {
					orphans = append(orphans, b)
					continue
				}
			}
			filtered = append(filtered, block)
		}

		// 丢掉全部块会让 messages 整体变空，连锁触发更严重的错误
		// （实测让后续的 reject_empty_messages 误拒了真实请求）；但原样保留
		// 孤儿 tool_result 上游同样会 400。改成把它降级成文本块，
		// 消息不为空，也不含非法的 tool_result。
		if len(filtered) == 0 && len(orphans) > 0 {
			filtered = append(filtered, map[string]any{
				"type": "text",
				"text": orphanToolResultText(orphans),
			})
		}
		// 降级成文本块时块数可能与原来相同，不能靠长度判断是否改动过。
		if len(orphans) > 0 {
			msg["content"] = filtered
			changed = true
		}

		next := map[string]bool{}
		for _, block := range filtered {
			if b, ok := block.(map[string]any); ok {
				if t, _ := b["type"].(string); t == "tool_use" {
					if id, ok := b["id"].(string); ok {
						next[id] = true
					}
				}
			}
		}
		available = next
		kept = append(kept, msg)
	}
	if changed {
		data["messages"] = kept
	}
	return changed
}

// fixThinkingBudget 删掉 adaptive 形态下非法的 thinking.budget_tokens。
// budget_tokens 只属于 enabled 变体；fix_thinking_type 把 type 从 enabled 改成
// adaptive 之后，残留的 budget_tokens 会让上游报
// "thinking.adaptive.budget_tokens: Extra inputs are not permitted"
// （报错里的 adaptive 是 union 变体名，不是多出来的一层路径）。
func fixThinkingBudget(data map[string]any) bool {
	thinking, ok := data["thinking"].(map[string]any)
	if !ok {
		return false
	}
	if t, _ := thinking["type"].(string); t != "adaptive" {
		return false
	}
	if _, exists := thinking["budget_tokens"]; !exists {
		return false
	}
	delete(thinking, "budget_tokens")
	return true
}

// rejectEmptyMessages 在本地就拒掉空 messages，省掉一次注定失败的上游调用。
func rejectEmptyMessages(data map[string]any) error {
	msgs, ok := getMessages(data)
	if !ok || len(msgs) == 0 {
		return fmt.Errorf("messages is empty")
	}
	return nil
}

// stripParamsForModel 无条件删除指定模型上已废弃的采样参数。
// engine.go 的 strip_temperature / strip_top_pk 只在 thinking 开启时生效，
// 而部分模型（如 claude-opus-4-8）是无条件拒绝这些参数的。
func stripParamsForModel(data map[string]any, model string, modelPatterns, params []string) bool {
	if len(modelPatterns) == 0 || len(params) == 0 || model == "" {
		return false
	}
	matched := false
	for _, pattern := range modelPatterns {
		if pattern != "" && strings.Contains(model, pattern) {
			matched = true
			break
		}
	}
	if !matched {
		return false
	}
	changed := false
	for _, param := range params {
		if _, exists := data[param]; exists {
			delete(data, param)
			changed = true
		}
	}
	return changed
}

// truncateUTF8 按字节上限截断，但保证不切断多字节字符，并丢掉本就非法的字节。
// 直接用 s[:n] 会把 UTF-8 字符切成半个，PostgreSQL 写入时报
// "invalid byte sequence for encoding UTF8" (SQLSTATE 22021)，整条日志丢失。
func truncateUTF8(s string, maxBytes int) string {
	if utf8.ValidString(s) && len(s) <= maxBytes {
		return s
	}
	var sb strings.Builder
	sb.Grow(min(len(s), maxBytes) + len(truncatedSuffix))
	truncated := false
	for _, r := range s {
		if r == utf8.RuneError {
			// 非法字节：跳过，不写进库。
			continue
		}
		if sb.Len()+utf8.RuneLen(r) > maxBytes {
			truncated = true
			break
		}
		sb.WriteRune(r)
	}
	if truncated {
		sb.WriteString(truncatedSuffix)
	}
	return sb.String()
}

const truncatedSuffix = "...(truncated)"

// upstreamModelName 取当次请求的模型名。
// Vertex AI 之类的渠道把模型放在 URL 而不是 body 里（body 只有 anthropic_version），
// 所以不能只读 data["model"]，必须回退到 RelayInfo。
func upstreamModelName(data map[string]any, info *relaycommon.RelayInfo) string {
	if model, _ := data["model"].(string); model != "" {
		return model
	}
	if info == nil {
		return ""
	}
	// ChannelMeta 是嵌入指针，未初始化时访问提升字段会 panic。
	if info.ChannelMeta != nil && info.UpstreamModelName != "" {
		return info.UpstreamModelName
	}
	return info.OriginModelName
}

// --- header 层 ---

// defaultDeniedBetaPrefixes 是生产日志里实测被上游拒绝的 anthropic-beta 前缀。
// 用前缀黑名单而非白名单，避免误删将来新增的合法 beta 标记。
var defaultDeniedBetaPrefixes = []string{
	"computer-use-",
	"advisor-tool-",
	"totally-bogus-beta-",
}

// ProcessHeaders 在请求发出前过滤上游不接受的 header 值。
// 必须在 body 拦截之外单独处理：body 拦截跑在 http.Request 构造之前，那时还没有 header。
func ProcessHeaders(channelId int, header http.Header, modelName string) {
	if header == nil {
		return
	}
	for _, rule := range getRules(channelId) {
		if rule.Type != RuleFilterBeta {
			continue
		}
		filterBetaHeader(header, rule.Config)
	}
}

func filterBetaHeader(header http.Header, config map[string]any) {
	const betaHeader = "anthropic-beta"
	raw := header.Get(betaHeader)
	if raw == "" {
		return
	}

	allowed := getConfigStrings(config, "allowed")
	denied := getConfigStrings(config, "denied")
	if len(denied) == 0 {
		denied = defaultDeniedBetaPrefixes
	}

	kept := make([]string, 0, 4)
	for value := range strings.SplitSeq(raw, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if len(allowed) > 0 {
			// 显式白名单模式：只放行列出的值。
			if slicesContainsPrefix(allowed, value) {
				kept = append(kept, value)
			}
			continue
		}
		if !slicesContainsPrefix(denied, value) {
			kept = append(kept, value)
		}
	}

	if len(kept) == 0 {
		header.Del(betaHeader)
		return
	}
	header.Set(betaHeader, strings.Join(kept, ","))
}

func slicesContainsPrefix(patterns []string, value string) bool {
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		if value == pattern || strings.HasPrefix(value, pattern) {
			return true
		}
	}
	return false
}

// getConfigStrings 读取 config 里的字符串列表，兼容单个字符串和逗号分隔写法。
func getConfigStrings(config map[string]any, key string) []string {
	if config == nil {
		return nil
	}
	switch v := config[key].(type) {
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		return out
	case []string:
		return v
	case string:
		var out []string
		for item := range strings.SplitSeq(v, ",") {
			if item = strings.TrimSpace(item); item != "" {
				out = append(out, item)
			}
		}
		return out
	default:
		return nil
	}
}

// fixDanglingToolUse 删掉后面没有对应 tool_result 的 tool_use 块。
// Anthropic 要求每个 tool_use 的下一条消息里必须紧跟同 id 的 tool_result，
// 客户端中断或裁剪历史时会留下悬空的 tool_use，上游直接 400。
// 最后一条消息不检查：它没有"下一条"，助手以 tool_use 结尾是合法的。
func fixDanglingToolUse(data map[string]any) bool {
	msgs, ok := getMessages(data)
	if !ok || len(msgs) < 2 {
		return false
	}
	changed := false
	for i := 0; i < len(msgs)-1; i++ {
		msg, ok := msgs[i].(map[string]any)
		if !ok {
			continue
		}
		content, isArray := msg["content"].([]any)
		if !isArray {
			continue
		}
		answered := toolResultIDs(msgs[i+1])

		filtered := make([]any, 0, len(content))
		dangling := 0
		for _, block := range content {
			b, ok := block.(map[string]any)
			if !ok {
				filtered = append(filtered, block)
				continue
			}
			if t, _ := b["type"].(string); t == "tool_use" {
				id, _ := b["id"].(string)
				if !answered[id] {
					dangling++
					continue
				}
			}
			filtered = append(filtered, block)
		}
		if dangling == 0 {
			continue
		}
		// 与孤儿 tool_result 同样的处理：不能让消息变空。
		if len(filtered) == 0 {
			filtered = append(filtered, map[string]any{
				"type": "text",
				"text": "(tool call omitted)",
			})
		}
		msg["content"] = filtered
		changed = true
	}
	return changed
}

func toolResultIDs(msg any) map[string]bool {
	ids := map[string]bool{}
	m, ok := msg.(map[string]any)
	if !ok {
		return ids
	}
	content, ok := m["content"].([]any)
	if !ok {
		return ids
	}
	for _, block := range content {
		b, ok := block.(map[string]any)
		if !ok {
			continue
		}
		if t, _ := b["type"].(string); t != "tool_result" {
			continue
		}
		if id, ok := b["tool_use_id"].(string); ok {
			ids[id] = true
		}
	}
	return ids
}

// toolNamePattern 是上游对工具名的约束：^[a-zA-Z0-9_-]{1,128}$。
var toolNameIllegal = regexp.MustCompile(`[^a-zA-Z0-9_-]`)

// fixToolName 把不符合上游正则的工具名清洗成合法名字。
// 非法字符替换成下划线，超长截断到 128。
func fixToolName(data map[string]any) bool {
	tools, ok := getTools(data)
	if !ok {
		return false
	}
	changed := false
	for _, item := range tools {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		name, ok := tool["name"].(string)
		if !ok || name == "" {
			continue
		}
		clean := toolNameIllegal.ReplaceAllString(name, "_")
		if len([]rune(clean)) > 128 {
			clean = string([]rune(clean)[:128])
		}
		if clean != name {
			tool["name"] = clean
			changed = true
		}
	}
	return changed
}

// fixTempTopPConflict 处理"temperature 和 top_p 不能同时指定"的模型。
// 保留 temperature，删掉 top_p：temperature 是更常用的采样参数，
// 客户端多数只是把两个都填了默认值。
func fixTempTopPConflict(data map[string]any) bool {
	_, hasTemp := data["temperature"]
	_, hasTopP := data["top_p"]
	if !hasTemp || !hasTopP {
		return false
	}
	delete(data, "top_p")
	return true
}

// rejectLargeBody 在解析 JSON 之前按原始字节数拦截过大的请求。
// reject_long_prompt 只累加 text 字段的长度，base64 图片撑出来的体积它看不到，
// 所以 33MB 的请求体会一路发到上游再被拒，白花一次往返和出网带宽。
// 返回命中的规则类型，供调用方写拦截日志。
func rejectLargeBody(rules []Rule, size int) (string, error) {
	for _, rule := range rules {
		if rule.Type != RuleRejectLargeBody {
			continue
		}
		maxBytes := getConfigInt(rule.Config, "max_bytes", 30*1024*1024)
		if maxBytes > 0 && size > maxBytes {
			return rule.Type, fmt.Errorf(
				"请求体过大：%d 字节，超过上限 %d 字节", size, maxBytes)
		}
		return "", nil
	}
	return "", nil
}

// schemaPropertyKeyPattern 是上游对 input_schema 属性键的约束：
// ^[a-zA-Z0-9_.-]{1,64}$。客户端（MCP 工具）会用中文或特殊字符当键名，直接 400。
var schemaPropertyKeyIllegal = regexp.MustCompile(`[^a-zA-Z0-9_.-]`)

// fixSchemaPropertyKeys 递归清洗所有 input_schema 里的 properties 键名。
// 非法字符替换成下划线，超长截断到 64，撞名时补下划线直到唯一。
func fixSchemaPropertyKeys(data map[string]any) bool {
	tools, ok := getTools(data)
	if !ok {
		return false
	}
	changed := false
	for _, item := range tools {
		tool, ok := item.(map[string]any)
		if !ok {
			continue
		}
		schema, ok := tool["input_schema"].(map[string]any)
		if !ok {
			continue
		}
		if sanitizeSchemaProperties(schema) {
			changed = true
		}
	}
	return changed
}

// sanitizeSchemaProperties 遍历 schema 树，凡是带 properties 的对象都清洗一遍。
// properties 的值本身可能嵌套 properties（对象）或 items（数组），所以递归。
func sanitizeSchemaProperties(node map[string]any) bool {
	changed := false
	if props, ok := node["properties"].(map[string]any); ok {
		for key, val := range props {
			clean := sanitizePropertyKey(key)
			if clean != key {
				// 撞名时补下划线，保证不覆盖已有键。
				for _, exists := props[clean]; exists; _, exists = props[clean] {
					clean = truncateRunes(clean+"_", 64)
				}
				delete(props, key)
				props[clean] = val
				changed = true
			}
			if child, ok := val.(map[string]any); ok {
				if sanitizeSchemaProperties(child) {
					changed = true
				}
			}
		}
	}
	if items, ok := node["items"].(map[string]any); ok {
		if sanitizeSchemaProperties(items) {
			changed = true
		}
	}
	return changed
}

func sanitizePropertyKey(key string) string {
	clean := schemaPropertyKeyIllegal.ReplaceAllString(key, "_")
	clean = truncateRunes(clean, 64)
	if clean == "" {
		return "_"
	}
	return clean
}

func truncateRunes(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen])
}

// fixThinkingDisabled 删掉显式的 thinking.type=disabled。
// 部分模型只接受 adaptive，显式 disabled 会直接 400；
// 不传 thinking 字段与 disabled 同义，删掉最安全。
func fixThinkingDisabled(data map[string]any) bool {
	thinking, ok := data["thinking"].(map[string]any)
	if !ok {
		return false
	}
	if thinking["type"] != "disabled" {
		return false
	}
	delete(data, "thinking")
	return true
}

// stripCacheControlScope 剥掉 cache_control 里的 scope 字段。
// 上游只认 ephemeral 等少数字段，客户端带的 scope 会报 Extra inputs。
func stripCacheControlScope(data map[string]any) bool {
	changed := false
	strip := func(blocks []any) {
		for _, block := range blocks {
			b, ok := block.(map[string]any)
			if !ok {
				continue
			}
			cc, ok := b["cache_control"].(map[string]any)
			if !ok {
				continue
			}
			if _, exists := cc["scope"]; exists {
				delete(cc, "scope")
				changed = true
			}
		}
	}
	if msgs, ok := getMessages(data); ok {
		for _, m := range msgs {
			if msg, ok := m.(map[string]any); ok {
				if content, ok := msg["content"].([]any); ok {
					strip(content)
				}
			}
		}
	}
	// system 既可能是字符串也可能是块数组，都要覆盖。
	if sys, ok := data["system"].([]any); ok {
		strip(sys)
	}
	return changed
}

// fixEmptyContent 把 content 为空的消息补成占位文本块。
// 上游对空数组和空字符串都报 "must have non-empty content"，
// 探活类客户端会发这种消息，与其让它 400 不如补个占位。
func fixEmptyContent(data map[string]any) bool {
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
		empty := false
		switch c := msg["content"].(type) {
		case string:
			empty = strings.TrimSpace(c) == ""
		case []any:
			empty = len(c) == 0
		}
		if empty {
			msg["content"] = []any{map[string]any{"type": "text", "text": "(empty)"}}
			changed = true
		}
	}
	return changed
}
