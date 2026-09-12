package interceptor

import (
	"embed"

	"github.com/QuantumNous/new-api/common"
)

//go:embed presets/*.json
var presetFS embed.FS

// Preset 是一组可以整套载入的规则。
type Preset struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Rules       []Rule `json:"rules"`
}

// presetFiles 把预设文件映射到展示名称。
// 这些规则是按 Anthropic / Vertex 的报错逐条实测出来的，不做默认启用：
// hoist_system 之类会把 system 消息搬到顶层，套到 OpenAI 格式的渠道上会破坏请求。
var presetFiles = []struct {
	file        string
	name        string
	description string
}{
	{
		file:        "presets/anthropic.json",
		name:        "Anthropic / Vertex",
		description: "按生产环境上游 400 报错逐条验证的规则集，适用于 Anthropic 与 Vertex AI 渠道",
	},
}

// Presets 返回内置预设。文件内容在编译期嵌入，解析失败的预设直接跳过，
// 不能因为一个坏文件让整个接口不可用。
func Presets() []Preset {
	result := make([]Preset, 0, len(presetFiles))
	for _, meta := range presetFiles {
		data, err := presetFS.ReadFile(meta.file)
		if err != nil {
			continue
		}
		var rules []Rule
		if err := common.Unmarshal(data, &rules); err != nil {
			continue
		}
		result = append(result, Preset{
			Name:        meta.name,
			Description: meta.description,
			Rules:       rules,
		})
	}
	return result
}
