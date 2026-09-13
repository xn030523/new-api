package interceptor

// Rule 单条拦截规则
type Rule struct {
	Type   string         `json:"type"`   // 规则类型
	Config map[string]any `json:"config"` // 规则参数（可选）
}

// 所有支持的规则类型
const (
	RuleFixContentArray     = "fix_content_array"     // content 字符串 → 数组
	RuleFixToolsArray       = "fix_tools_array"       // tools 确保是数组
	RuleFixSchemaRequired   = "fix_schema_required"   // input_schema.required 修复
	RuleFixSchemaCitations  = "fix_schema_citations"  // citations 修复
	RuleFixSchemaProperties = "fix_schema_properties" // properties 不是 dict → 删
	RuleStripThinking       = "strip_thinking"        // 去掉 thinking block
	RuleStripReasoning      = "strip_reasoning"       // 去掉 reasoning 字段
	RuleStripAnnotations    = "strip_annotations"     // 去掉 annotations 字段
	RuleStripEmptyText      = "strip_empty_text"      // 去掉空 text block
	RuleTrimWhitespace      = "trim_whitespace"       // 修剪尾部空白
	RuleCapMaxTokens        = "cap_max_tokens"        // 限制 max_tokens
	RuleFixSystemField      = "fix_system_field"      // system 字符串 → 数组
	RuleStripTemperature    = "strip_temperature"     // thinking 时删 temperature
	RuleStripTopPK          = "strip_top_pk"          // thinking 时删 top_p/top_k
	RuleFixEffort           = "fix_effort"            // effort max → high
	RuleFilterBeta          = "filter_beta"           // anthropic-beta 白名单
	RuleFilterToolTypes     = "filter_tool_types"     // 过滤不支持的 tool type
	RuleFixThinkingType     = "fix_thinking_type"     // enabled → adaptive
	RuleFixToolUseId        = "fix_tool_use_id"       // tool_use.id 格式修复
	RuleClampTopP           = "clamp_top_p"           // top_p 限制 0-1
	RuleClampTopK           = "clamp_top_k"           // top_k 限制 0-100000000
	RuleRejectLongPrompt    = "reject_long_prompt"    // 超长 prompt 拒绝
	RuleFixSchemaOneOf      = "fix_schema_oneof"      // 删掉 oneOf/allOf/anyOf
	RuleStripAssistPrefill  = "strip_assist_prefill"  // 删末尾 assistant 消息
	RuleFixAdditionalProps  = "fix_additional_props"  // additionalProperties 补 false

	// 针对生产实测残留 400 报错补充的规则
	RuleHoistSystem           = "hoist_system"              // messages 里的 system 消息上提到顶层
	RuleFixEmptySystem        = "fix_empty_system"          // 顶层 system 去掉空 text block
	RuleStripURLSource        = "strip_url_source"          // 删掉 source.type=url 的 block
	RuleFixMessageRoles       = "fix_message_roles"         // 非法 role 归一到 user
	RuleFixToolChoice         = "fix_tool_choice"           // 删掉不支持/悬空的 tool_choice
	RuleFixDeferLoading       = "fix_defer_loading"         // 保证至少一个 tool 不 defer
	RuleFixOrphanToolResult   = "fix_orphan_tool_result"    // 删掉悬空 tool_result
	RuleRejectEmptyMessages   = "reject_empty_messages"     // messages 为空时本地拒绝
	RuleFixDanglingToolUse    = "fix_dangling_tool_use"     // 删掉后面没有 tool_result 的 tool_use
	RuleFixToolName           = "fix_tool_name"             // 清洗非法工具名
	RuleFixTempTopPConflict   = "fix_temp_topp_conflict"    // temperature 与 top_p 互斥时删 top_p
	RuleRejectLargeBody       = "reject_large_body"         // 请求体字节数超限时本地拒绝
	RuleFixSchemaPropertyKeys = "fix_schema_property_keys"  // 清洗 input_schema 属性键
	RuleFixThinkingDisabled   = "fix_thinking_disabled"     // 删掉显式 thinking.disabled
	RuleStripCacheScope       = "strip_cache_control_scope" // 剥掉 cache_control.scope
	RuleFixEmptyContent       = "fix_empty_content"         // 空 content 补占位文本
	RuleStripParamsForModel   = "strip_params_for_model"    // 按模型无条件删采样参数
	RuleFixThinkingBudget     = "fix_thinking_budget"       // adaptive 下删 thinking.budget_tokens

	// 自定义规则类型
	RuleCustomDeletePath   = "custom_delete_path"    // 按路径删字段
	RuleCustomSetValue     = "custom_set_value"      // 按路径设值
	RuleCustomDeleteKeyAll = "custom_delete_key_all" // 递归删指定 key
	RuleCustomEnsureArray  = "custom_ensure_array"   // 确保指定路径是数组
	RuleCustomRenameKey    = "custom_rename_key"     // 重命名字段
	RuleCustomRejectIf     = "custom_reject_if"      // 条件拒绝
)

// AllRuleDescriptions 规则描述（给前端显示用）
var AllRuleDescriptions = map[string]string{
	RuleFixContentArray:       "修复 content 字段为数组格式",
	RuleFixToolsArray:         "修复 tools 字段为数组格式",
	RuleFixSchemaRequired:     "修复 input_schema.required 为数组",
	RuleFixSchemaCitations:    "修复 citations 为数组",
	RuleFixSchemaProperties:   "修复 properties 为字典",
	RuleStripThinking:         "去掉 thinking/redacted_thinking block",
	RuleStripReasoning:        "去掉 reasoning 字段",
	RuleStripAnnotations:      "去掉 annotations 字段",
	RuleStripEmptyText:        "去掉空的 text content block",
	RuleTrimWhitespace:        "修剪 assistant 消息尾部空白",
	RuleCapMaxTokens:          "限制 max_tokens 上限（默认 128000）",
	RuleFixSystemField:        "system 字符串转数组",
	RuleStripTemperature:      "thinking 开启时删除 temperature",
	RuleStripTopPK:            "thinking 开启时删除 top_p/top_k",
	RuleFixEffort:             "非 thinking 时 effort max → high",
	RuleFilterBeta:            "过滤不支持的 anthropic-beta 值",
	RuleFilterToolTypes:       "过滤不支持的 tool type",
	RuleFixThinkingType:       "thinking.type enabled → adaptive",
	RuleFixToolUseId:          "修复 tool_use.id 格式",
	RuleClampTopP:             "top_p 限制在 0-1 范围",
	RuleClampTopK:             "top_k 限制在 0-100000000 范围",
	RuleRejectLongPrompt:      "拒绝超长 prompt（默认 180 万字符）",
	RuleFixSchemaOneOf:        "删掉 oneOf/allOf/anyOf",
	RuleStripAssistPrefill:    "删掉末尾 assistant 消息（不支持 prefill 时）",
	RuleFixAdditionalProps:    "补充 additionalProperties: false",
	RuleHoistSystem:           "把 messages 里的 system 消息上提到顶层 system",
	RuleFixEmptySystem:        "去掉顶层 system 里的空 text block",
	RuleStripURLSource:        "删掉 source.type=url 的图片/文档 block",
	RuleFixMessageRoles:       "非法 role 归一到 user",
	RuleFixToolChoice:         "删掉不支持或指向不存在工具的 tool_choice（config: denied_types）",
	RuleFixDeferLoading:       "保证至少一个 tool 的 defer_loading 为 false",
	RuleFixOrphanToolResult:   "删掉没有对应 tool_use 的 tool_result",
	RuleRejectEmptyMessages:   "messages 为空时本地拒绝，不发上游",
	RuleFixDanglingToolUse:    "删掉后面没有紧跟 tool_result 的 tool_use",
	RuleFixToolName:           "清洗不符合 ^[a-zA-Z0-9_-]{1,128}$ 的工具名",
	RuleFixTempTopPConflict:   "temperature 与 top_p 不能同时指定时删掉 top_p",
	RuleRejectLargeBody:       "请求体字节数超过上限时本地拒绝，不发上游",
	RuleFixSchemaPropertyKeys: "清洗不符合 ^[a-zA-Z0-9_.-]{1,64}$ 的工具属性键",
	RuleFixThinkingDisabled:   "删掉显式 thinking.disabled（与不传同义）",
	RuleStripCacheScope:       "剥掉 cache_control 里的 scope 字段",
	RuleFixEmptyContent:       "消息 content 为空时补占位文本块",
	RuleStripParamsForModel:   "按模型无条件删除采样参数（config: models, params）",
	RuleFixThinkingBudget:     "thinking 为 adaptive 时删掉非法的 budget_tokens",
	RuleCustomDeletePath:      "自定义：按路径删除字段（config: path）",
	RuleCustomSetValue:        "自定义：按路径设置值（config: path, value）",
	RuleCustomDeleteKeyAll:    "自定义：递归删除所有同名字段（config: key）",
	RuleCustomEnsureArray:     "自定义：确保指定路径值为数组（config: path）",
	RuleCustomRenameKey:       "自定义：重命名字段（config: path, old_key, new_key）",
	RuleCustomRejectIf:        "自定义：条件拒绝请求（config: path, op, value）",
}
