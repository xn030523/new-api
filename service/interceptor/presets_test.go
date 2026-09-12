package interceptor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPresetsParseAndCoverKnownRuleTypes(t *testing.T) {
	presets := Presets()
	require.NotEmpty(t, presets, "内置预设必须能解析出来，否则前端载入按钮是空的")

	for _, preset := range presets {
		assert.NotEmpty(t, preset.Name)
		require.NotEmpty(t, preset.Rules, "预设 %s 没有规则", preset.Name)

		for _, rule := range preset.Rules {
			// 预设里出现引擎不认识的类型会被静默忽略，等于配置失效。
			_, known := AllRuleDescriptions[rule.Type]
			assert.True(t, known, "预设 %s 引用了未注册的规则类型 %s", preset.Name, rule.Type)
		}
	}
}

func TestAnthropicPresetKeepsBodySizeGuardFirst(t *testing.T) {
	// 体积检查必须排在最前面，否则超大 body 会先被解析一遍才拒绝。
	presets := Presets()
	require.NotEmpty(t, presets)
	assert.Equal(t, RuleRejectLargeBody, presets[0].Rules[0].Type)
}
