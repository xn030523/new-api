package interceptor

import (
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHoistSystemMessages(t *testing.T) {
	data := map[string]any{
		"system": "base instruction",
		"messages": []any{
			map[string]any{"role": "system", "content": "extra rule"},
			map[string]any{"role": "user", "content": "hi"},
			map[string]any{"role": "system", "content": []any{
				map[string]any{"type": "text", "text": "late rule"},
				map[string]any{"type": "text", "text": "   "},
			}},
		},
	}
	require.True(t, hoistSystemMessages(data))

	assert.Equal(t, []any{
		map[string]any{"type": "text", "text": "base instruction"},
		map[string]any{"type": "text", "text": "extra rule"},
		map[string]any{"type": "text", "text": "late rule"},
	}, data["system"])
	assert.Equal(t, []any{map[string]any{"role": "user", "content": "hi"}}, data["messages"])
}

func TestHoistSystemMessagesNoop(t *testing.T) {
	data := map[string]any{"messages": []any{map[string]any{"role": "user", "content": "hi"}}}
	assert.False(t, hoistSystemMessages(data))
}

func TestFixEmptySystem(t *testing.T) {
	data := map[string]any{"system": []any{
		map[string]any{"type": "text", "text": "keep"},
		map[string]any{"type": "text", "text": ""},
	}}
	require.True(t, fixEmptySystem(data))
	assert.Len(t, data["system"], 1)

	// 全空时字段整体删掉，空数组同样会被上游拒绝。
	allEmpty := map[string]any{"system": []any{map[string]any{"type": "text", "text": " "}}}
	require.True(t, fixEmptySystem(allEmpty))
	_, exists := allEmpty["system"]
	assert.False(t, exists)
}

func TestStripURLSources(t *testing.T) {
	data := map[string]any{"messages": []any{
		map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "text", "text": "look"},
			map[string]any{"type": "image", "source": map[string]any{"type": "url", "url": "https://x/y.png"}},
			map[string]any{"type": "image", "source": map[string]any{"type": "base64", "data": "AAA"}},
		}},
	}}
	require.True(t, stripURLSources(data))

	msg := data["messages"].([]any)[0].(map[string]any)
	assert.Len(t, msg["content"], 2)
}

func TestFixMessageRoles(t *testing.T) {
	data := map[string]any{"messages": []any{
		map[string]any{"role": "tool", "content": "x"},
		map[string]any{"role": "assistant", "content": "y"},
	}}
	require.True(t, fixMessageRoles(data))
	assert.Equal(t, "user", data["messages"].([]any)[0].(map[string]any)["role"])
	assert.Equal(t, "assistant", data["messages"].([]any)[1].(map[string]any)["role"])
}

func TestFixToolChoice(t *testing.T) {
	denied := []string{"tool", "any"}

	// 被拒绝的 type 直接删掉整个字段。
	anyChoice := map[string]any{"tool_choice": map[string]any{"type": "any"}}
	require.True(t, fixToolChoice(anyChoice, denied))
	_, exists := anyChoice["tool_choice"]
	assert.False(t, exists)

	// type=tool 但 name 不在 tools 里，同样要删（上游报 Tool not found）。
	dangling := map[string]any{
		"tool_choice": map[string]any{"type": "tool", "name": "web_search"},
		"tools":       []any{map[string]any{"name": "calculator"}},
	}
	require.True(t, fixToolChoice(dangling, []string{}))
	_, exists = dangling["tool_choice"]
	assert.False(t, exists)

	// name 存在时保留。
	valid := map[string]any{
		"tool_choice": map[string]any{"type": "tool", "name": "calculator"},
		"tools":       []any{map[string]any{"name": "calculator"}},
	}
	assert.False(t, fixToolChoice(valid, []string{}))
	assert.NotNil(t, valid["tool_choice"])

	// auto 不在黑名单里，保持原样。
	auto := map[string]any{"tool_choice": map[string]any{"type": "auto"}}
	assert.False(t, fixToolChoice(auto, denied))
	assert.NotNil(t, auto["tool_choice"])
}

func TestFixDeferLoading(t *testing.T) {
	allDeferred := map[string]any{"tools": []any{
		map[string]any{"name": "a", "defer_loading": true},
		map[string]any{"name": "b", "defer_loading": true},
	}}
	require.True(t, fixDeferLoading(allDeferred))
	assert.Equal(t, false, allDeferred["tools"].([]any)[0].(map[string]any)["defer_loading"])

	// 已经有一个 false 时不动。
	mixed := map[string]any{"tools": []any{
		map[string]any{"name": "a", "defer_loading": true},
		map[string]any{"name": "b", "defer_loading": false},
	}}
	assert.False(t, fixDeferLoading(mixed))
}

func TestFixOrphanToolResult(t *testing.T) {
	data := map[string]any{"messages": []any{
		map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "id": "toolu_ok", "name": "a"},
		}},
		map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": "toolu_ok", "content": "fine"},
			map[string]any{"type": "tool_result", "tool_use_id": "toolu_orphan", "content": "drop"},
		}},
	}}
	require.True(t, fixOrphanToolResult(data))

	msgs := data["messages"].([]any)
	require.Len(t, msgs, 2)
	content := msgs[1].(map[string]any)["content"].([]any)
	require.Len(t, content, 1)
	assert.Equal(t, "toolu_ok", content[0].(map[string]any)["tool_use_id"])
}

func TestFixOrphanToolResultNeverEmptiesMessages(t *testing.T) {
	// 消息里混有合法和孤儿 block 时，只删孤儿，消息本身保留。
	data := map[string]any{"messages": []any{
		map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "id": "toolu_ok"},
		}},
		map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": "toolu_ok"},
			map[string]any{"type": "tool_result", "tool_use_id": "toolu_orphan"},
		}},
	}}
	require.True(t, fixOrphanToolResult(data))
	msgs := data["messages"].([]any)
	require.Len(t, msgs, 2)
	assert.Len(t, msgs[1].(map[string]any)["content"], 1)
}

func TestRejectEmptyMessages(t *testing.T) {
	require.Error(t, rejectEmptyMessages(map[string]any{"messages": []any{}}))
	require.Error(t, rejectEmptyMessages(map[string]any{}))
	require.NoError(t, rejectEmptyMessages(map[string]any{
		"messages": []any{map[string]any{"role": "user", "content": "hi"}},
	}))
}

func TestStripParamsForModel(t *testing.T) {
	patterns := []string{"claude-opus-4-8"}
	params := []string{"temperature", "top_p", "top_k"}

	matched := map[string]any{"temperature": 0.7, "top_p": 0.9, "max_tokens": 100}
	require.True(t, stripParamsForModel(matched, "claude-opus-4-8", patterns, params))
	assert.NotContains(t, matched, "temperature")
	assert.NotContains(t, matched, "top_p")
	assert.Contains(t, matched, "max_tokens")

	// 不匹配的模型必须原样放行，否则会影响其他模型。
	other := map[string]any{"temperature": 0.7}
	assert.False(t, stripParamsForModel(other, "claude-sonnet-4-6", patterns, params))
	assert.Contains(t, other, "temperature")

	// Vertex AI 的 body 没有 model 字段，模型名为空时不能瞎删。
	noModel := map[string]any{"temperature": 0.7}
	assert.False(t, stripParamsForModel(noModel, "", patterns, params))
	assert.Contains(t, noModel, "temperature")
}

func TestUpstreamModelNameFallsBackToRelayInfo(t *testing.T) {
	// body 里有 model 时优先用它。
	assert.Equal(t, "from-body", upstreamModelName(map[string]any{"model": "from-body"}, nil))

	// Vertex AI 把模型放 URL 里，body 只有 anthropic_version。
	vertexBody := map[string]any{"anthropic_version": "vertex-2023-10-16"}
	assert.Equal(t, "", upstreamModelName(vertexBody, nil))
	// UpstreamModelName 来自嵌入指针 *ChannelMeta，必须先初始化才能读。
	withUpstream := &relaycommon.RelayInfo{ChannelMeta: &relaycommon.ChannelMeta{}}
	withUpstream.UpstreamModelName = "claude-opus-4-8"
	assert.Equal(t, "claude-opus-4-8", upstreamModelName(vertexBody, withUpstream))

	// ChannelMeta 为 nil 时不能 panic，退回 OriginModelName。
	withOrigin := &relaycommon.RelayInfo{OriginModelName: "origin-model"}
	assert.Equal(t, "origin-model", upstreamModelName(vertexBody, withOrigin))
}

func TestFixOrphanToolResultDowngradesSoleOrphanToText(t *testing.T) {
	// 生产实测回归：首条消息就是孤儿 tool_result。删空它会让 messages 变空，
	// 进而被 reject_empty_messages 误拒；原样保留上游又会 400。
	// 降级成文本块两边都躲开，工具返回的内容也不丢。
	data := map[string]any{"messages": []any{
		map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": "toolu_bdrk_01UK", "content": "sunny"},
		}},
	}}
	require.True(t, fixOrphanToolResult(data))
	msgs := data["messages"].([]any)
	require.Len(t, msgs, 1)
	content := msgs[0].(map[string]any)["content"].([]any)
	require.Len(t, content, 1)
	block := content[0].(map[string]any)
	assert.Equal(t, "text", block["type"])
	assert.Equal(t, "sunny", block["text"])
}

func TestOrphanToolResultTextFromBlockArray(t *testing.T) {
	text := orphanToolResultText([]any{
		map[string]any{"type": "tool_result", "content": []any{
			map[string]any{"type": "text", "text": "line1"},
			map[string]any{"type": "text", "text": "line2"},
		}},
	})
	assert.Equal(t, "line1line2", text)
}

func TestOrphanToolResultTextFallsBackWhenEmpty(t *testing.T) {
	// content 缺失或不是可读结构时不能产出空文本块，上游会拒绝空 text。
	assert.Equal(t, "(tool result omitted)",
		orphanToolResultText([]any{map[string]any{"type": "tool_result"}}))
}

func TestFilterToolTypesRenamesAndDenies(t *testing.T) {
	data := map[string]any{"tools": []any{
		map[string]any{"type": "computer_20250124", "name": "computer"},
		map[string]any{"type": "totally_bogus", "name": "bogus"},
		map[string]any{"type": "custom", "name": "mine"},
	}}
	config := map[string]any{
		"denied_types": []any{"totally_bogus"},
		"rename_types": map[string]any{"computer_20250124": "computer_toolset_20260801"},
	}
	require.True(t, filterToolTypes(data, config))
	tools := data["tools"].([]any)
	require.Len(t, tools, 2)
	assert.Equal(t, "computer_toolset_20260801", tools[0].(map[string]any)["type"])
	assert.Equal(t, "custom", tools[1].(map[string]any)["type"])
}

func TestFilterToolTypesDropsToolChoiceWhenAllRemoved(t *testing.T) {
	// tools 变成空数组同样会被上游拒绝，tool_choice 也会变成悬空引用。
	data := map[string]any{
		"tools":       []any{map[string]any{"type": "computer_20250124"}},
		"tool_choice": map[string]any{"type": "auto"},
	}
	config := map[string]any{"denied_types": []any{"computer_20250124"}}
	require.True(t, filterToolTypes(data, config))
	assert.NotContains(t, data, "tools")
	assert.NotContains(t, data, "tool_choice")
}

func TestFilterToolTypesAllowlistFromConfigKeepsUpstreamTools(t *testing.T) {
	// 内置白名单只有 4 种 type，会误删上游实际支持的 web_search / memory 等工具。
	// 配了 allowed_types 后必须以配置为准。
	data := map[string]any{"tools": []any{
		map[string]any{"type": "web_search_20250305"},
		map[string]any{"type": "memory_20250818"},
		map[string]any{"type": "retired_tool"},
	}}
	config := map[string]any{
		"allowed_types": []any{"web_search_20250305", "memory_20250818", "custom"},
	}
	require.True(t, filterToolTypes(data, config))
	tools := data["tools"].([]any)
	require.Len(t, tools, 2)
	assert.Equal(t, "web_search_20250305", tools[0].(map[string]any)["type"])
	assert.Equal(t, "memory_20250818", tools[1].(map[string]any)["type"])
}

func TestFilterToolTypesRenameWinsOverAllowlist(t *testing.T) {
	// 改名后的 type 在 allowed 清单里，不能因为原名不在清单里就先被删掉。
	data := map[string]any{"tools": []any{
		map[string]any{"type": "computer_20250124"},
	}}
	config := map[string]any{
		"allowed_types": []any{"computer_toolset_20260801"},
		"rename_types":  map[string]any{"computer_20250124": "computer_toolset_20260801"},
	}
	require.True(t, filterToolTypes(data, config))
	tools := data["tools"].([]any)
	require.Len(t, tools, 1)
	assert.Equal(t, "computer_toolset_20260801", tools[0].(map[string]any)["type"])
}

func TestFilterToolTypesFallsBackToAllowlistWithoutConfig(t *testing.T) {
	// 没配参数时保持原有白名单行为，避免升级后旧配置行为突变。
	data := map[string]any{"tools": []any{
		map[string]any{"type": "bash_20250124"},
		map[string]any{"type": "unknown_future_tool"},
	}}
	require.True(t, filterToolTypes(data, nil))
	tools := data["tools"].([]any)
	require.Len(t, tools, 1)
	assert.Equal(t, "bash_20250124", tools[0].(map[string]any)["type"])
}

func TestFilterBetaHeaderDenylist(t *testing.T) {
	header := http.Header{}
	header.Set("anthropic-beta", "context-1m-2025-08-07,computer-use-2025-01-24,advisor-tool-2025-06-01")
	filterBetaHeader(header, nil)
	// 合法 beta 必须留下，只删实测被拒的前缀。
	assert.Equal(t, "context-1m-2025-08-07", header.Get("anthropic-beta"))
}

func TestFilterBetaHeaderRemovesEmptied(t *testing.T) {
	header := http.Header{}
	header.Set("anthropic-beta", "computer-use-2025-01-24")
	filterBetaHeader(header, nil)
	assert.Empty(t, header.Get("anthropic-beta"))
}

func TestFilterBetaHeaderAllowlist(t *testing.T) {
	header := http.Header{}
	header.Set("anthropic-beta", "context-1m-2025-08-07,prompt-caching-2024-07-31")
	filterBetaHeader(header, map[string]any{"allowed": []any{"prompt-caching-"}})
	assert.Equal(t, "prompt-caching-2024-07-31", header.Get("anthropic-beta"))
}

func TestFilterBetaHeaderNoHeader(t *testing.T) {
	header := http.Header{}
	filterBetaHeader(header, nil)
	assert.Empty(t, header.Get("anthropic-beta"))
}

func TestFixEffortHandlesXHigh(t *testing.T) {
	data := map[string]any{"output_config": map[string]any{"effort": "xhigh"}}
	require.True(t, fixEffort(data))
	assert.Equal(t, "high", data["output_config"].(map[string]any)["effort"])

	// thinking 开启时不该改动。
	thinking := map[string]any{
		"thinking":      map[string]any{"type": "adaptive"},
		"output_config": map[string]any{"effort": "xhigh"},
	}
	assert.False(t, fixEffort(thinking))
}

func TestFixThinkingBudget(t *testing.T) {
	// fix_thinking_type 把 enabled 改成 adaptive 后，budget_tokens 就成了非法字段。
	adaptive := map[string]any{"thinking": map[string]any{"type": "adaptive", "budget_tokens": 1024.0}}
	require.True(t, fixThinkingBudget(adaptive))
	assert.NotContains(t, adaptive["thinking"], "budget_tokens")
	assert.Equal(t, "adaptive", adaptive["thinking"].(map[string]any)["type"])

	// enabled 变体下 budget_tokens 是合法的，必须保留。
	enabled := map[string]any{"thinking": map[string]any{"type": "enabled", "budget_tokens": 1024.0}}
	assert.False(t, fixThinkingBudget(enabled))
	assert.Contains(t, enabled["thinking"], "budget_tokens")

	assert.False(t, fixThinkingBudget(map[string]any{}))
}

func TestFixThinkingTypeThenBudget(t *testing.T) {
	// 生产实际链路：两条规则按顺序跑完必须产出合法的 adaptive thinking。
	data := map[string]any{
		"thinking": map[string]any{"type": "enabled", "budget_tokens": 1024.0},
	}
	require.True(t, fixThinkingType(data))
	require.True(t, fixThinkingBudget(data))
	assert.Equal(t, map[string]any{"type": "adaptive"}, data["thinking"])
}

func TestGetConfigStrings(t *testing.T) {
	assert.Equal(t, []string{"a", "b"}, getConfigStrings(map[string]any{"k": []any{"a", " b "}}, "k"))
	assert.Equal(t, []string{"a", "b"}, getConfigStrings(map[string]any{"k": "a, b"}, "k"))
	assert.Nil(t, getConfigStrings(map[string]any{}, "k"))
	assert.Nil(t, getConfigStrings(nil, "k"))
}

func TestTruncateUTF8(t *testing.T) {
	// 短且合法的字符串原样返回。
	assert.Equal(t, "hello", truncateUTF8("hello", 100))

	// 截断点落在多字节字符中间时，不能切出半个字符。
	// "中" 是 3 字节，maxBytes=4 只能放下第一个字。
	out := truncateUTF8("中文测试", 4)
	assert.True(t, utf8.ValidString(out), "结果必须是合法 UTF-8")
	assert.Equal(t, "中"+truncatedSuffix, out)

	// 非法字节必须被丢掉，否则 PostgreSQL 报 SQLSTATE 22021。
	invalid := truncateUTF8("ab\xd1\x2ecd", 100)
	assert.True(t, utf8.ValidString(invalid))
	assert.Equal(t, "ab.cd", invalid)
}

func TestFixDanglingToolUseDropsUnansweredCall(t *testing.T) {
	data := map[string]any{"messages": []any{
		map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "text", "text": "let me check"},
			map[string]any{"type": "tool_use", "id": "toolu_answered"},
			map[string]any{"type": "tool_use", "id": "toolu_dangling"},
		}},
		map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "tool_result", "tool_use_id": "toolu_answered"},
		}},
	}}
	require.True(t, fixDanglingToolUse(data))
	content := data["messages"].([]any)[0].(map[string]any)["content"].([]any)
	require.Len(t, content, 2)
	assert.Equal(t, "text", content[0].(map[string]any)["type"])
	assert.Equal(t, "toolu_answered", content[1].(map[string]any)["id"])
}

func TestFixDanglingToolUseIgnoresLastMessage(t *testing.T) {
	// 最后一条消息没有"下一条"，以 tool_use 结尾是合法的，不能动。
	data := map[string]any{"messages": []any{
		map[string]any{"role": "user", "content": "hi"},
		map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "id": "toolu_pending"},
		}},
	}}
	assert.False(t, fixDanglingToolUse(data))
	content := data["messages"].([]any)[1].(map[string]any)["content"].([]any)
	assert.Len(t, content, 1)
}

func TestFixDanglingToolUseNeverEmptiesMessage(t *testing.T) {
	data := map[string]any{"messages": []any{
		map[string]any{"role": "assistant", "content": []any{
			map[string]any{"type": "tool_use", "id": "toolu_dangling"},
		}},
		map[string]any{"role": "user", "content": "never mind"},
	}}
	require.True(t, fixDanglingToolUse(data))
	content := data["messages"].([]any)[0].(map[string]any)["content"].([]any)
	require.Len(t, content, 1)
	assert.Equal(t, "text", content[0].(map[string]any)["type"])
}

func TestFixToolNameSanitizesIllegalCharacters(t *testing.T) {
	data := map[string]any{"tools": []any{
		map[string]any{"name": "mcp__server::do thing!"},
		map[string]any{"name": "already_legal-1"},
	}}
	require.True(t, fixToolName(data))
	tools := data["tools"].([]any)
	assert.Equal(t, "mcp__server__do_thing_", tools[0].(map[string]any)["name"])
	assert.Equal(t, "already_legal-1", tools[1].(map[string]any)["name"])
}

func TestFixToolNameTruncatesAt128(t *testing.T) {
	long := strings.Repeat("a", 200)
	data := map[string]any{"tools": []any{map[string]any{"name": long}}}
	require.True(t, fixToolName(data))
	assert.Len(t, data["tools"].([]any)[0].(map[string]any)["name"], 128)
}

func TestStripToolFieldsRemovesNameOnToolsetEntry(t *testing.T) {
	// toolset 条目的成员名固定，带 name 会被上游拒绝。
	data := map[string]any{"tools": []any{
		map[string]any{"type": "computer_20250124", "name": "computer", "display_width_px": 1024},
	}}
	config := map[string]any{
		"rename_types": map[string]any{"computer_20250124": "computer_toolset_20260801"},
		"strip_fields": map[string]any{"computer_toolset_20260801": []any{"name"}},
	}
	require.True(t, filterToolTypes(data, config))
	tool := data["tools"].([]any)[0].(map[string]any)
	assert.Equal(t, "computer_toolset_20260801", tool["type"])
	assert.NotContains(t, tool, "name")
	assert.Contains(t, tool, "display_width_px")
}

func TestFixTempTopPConflict(t *testing.T) {
	both := map[string]any{"temperature": 0.7, "top_p": 0.9}
	require.True(t, fixTempTopPConflict(both))
	assert.Contains(t, both, "temperature")
	assert.NotContains(t, both, "top_p")

	// 只有一个时不能动，删掉会改变采样行为。
	only := map[string]any{"top_p": 0.9}
	assert.False(t, fixTempTopPConflict(only))
	assert.Contains(t, only, "top_p")
}

func TestRejectLargeBodyOverLimit(t *testing.T) {
	rules := []Rule{{Type: RuleRejectLargeBody, Config: map[string]any{"max_bytes": 1000}}}

	ruleType, err := rejectLargeBody(rules, 1001)
	require.Error(t, err)
	assert.Equal(t, RuleRejectLargeBody, ruleType)
	assert.Contains(t, err.Error(), "1001")

	// 正好等于上限要放行，边界不能误拒。
	_, err = rejectLargeBody(rules, 1000)
	require.NoError(t, err)
}

func TestRejectLargeBodySkippedWhenRuleAbsent(t *testing.T) {
	// 没配这条规则时，多大的 body 都不拦。
	_, err := rejectLargeBody([]Rule{{Type: RuleFixToolName}}, 999999999)
	assert.NoError(t, err)
}

func TestRejectLargeBodyDefaultLimitIs30MB(t *testing.T) {
	rules := []Rule{{Type: RuleRejectLargeBody}}

	_, err := rejectLargeBody(rules, 30*1024*1024)
	require.NoError(t, err)

	// 生产实测的 33491491 字节必须被拦下。
	_, err = rejectLargeBody(rules, 33491491)
	require.Error(t, err)
}

func TestStripEmptyTextNeverEmptiesContent(t *testing.T) {
	// 生产实测：探活请求只带一个空 text 块，删干净会留下 "content":[]，
	// 上游报 "user messages must have non-empty content"。
	data := map[string]any{"messages": []any{
		map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "text", "text": ""},
		}},
	}}
	require.True(t, stripEmptyText(data))
	content := data["messages"].([]any)[0].(map[string]any)["content"].([]any)
	require.Len(t, content, 1)
	block := content[0].(map[string]any)
	assert.Equal(t, "text", block["type"])
	assert.NotEmpty(t, block["text"])
}

func TestStripEmptyTextKeepsOtherBlocks(t *testing.T) {
	data := map[string]any{"messages": []any{
		map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "text", "text": ""},
			map[string]any{"type": "text", "text": "real"},
		}},
	}}
	require.True(t, stripEmptyText(data))
	content := data["messages"].([]any)[0].(map[string]any)["content"].([]any)
	require.Len(t, content, 1)
	assert.Equal(t, "real", content[0].(map[string]any)["text"])
}

func TestStripEmptyTextLeavesUnaffectedMessagesAlone(t *testing.T) {
	// 前一条消息被改过，不能连带重写后面没有空块的消息。
	clean := map[string]any{"type": "text", "text": "keep"}
	data := map[string]any{"messages": []any{
		map[string]any{"role": "user", "content": []any{
			map[string]any{"type": "text", "text": ""},
			clean,
		}},
		map[string]any{"role": "assistant", "content": []any{clean}},
	}}
	require.True(t, stripEmptyText(data))
	second := data["messages"].([]any)[1].(map[string]any)["content"].([]any)
	require.Len(t, second, 1)
	assert.Equal(t, "keep", second[0].(map[string]any)["text"])
}
