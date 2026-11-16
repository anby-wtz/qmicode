package chatgpt_runtime

import (
	"context"

	"qmicode-go/app/config"

	"github.com/sonnes/gopi/pkg/ai"
	"github.com/sonnes/gopi/pkg/ai/chatgpt"
)

type ModelInfo struct {
	ID            string
	Name          string
	ContextWindow int
	Images        bool
	Thinking      []string
}

// Endpoint 返回实际使用的 API 地址。
func Endpoint(baseURL string) string {
	if baseURL != "" {
		return baseURL
	}
	return chatgpt.DefaultBaseURL
}

func ListModels(ctx context.Context, conf config.Config, baseURL string) ([]ModelInfo, error) {
	models, err := provider(conf, baseURL).ListModels(ctx)
	if err != nil {
		return nil, mapAuthError(err)
	}

	out := make([]ModelInfo, 0, len(models))
	for _, m := range models {
		info := ModelInfo{
			ID:            m.ID,
			Name:          m.Name,
			ContextWindow: m.ContextWindow,
		}
		for _, input := range m.Input {
			if input == ai.ModalityImage {
				info.Images = true
			}
		}
		for _, level := range m.ThinkingLevels {
			info.Thinking = append(info.Thinking, string(level))
		}
		out = append(out, info)
	}
	return out, nil
}

func FindModel(models []ModelInfo, id string) (ModelInfo, bool) {
	for _, m := range models {
		if m.ID == id {
			return m, true
		}
	}
	return ModelInfo{}, false
}

// NormalizeEffort 只在模型确实支持该档位时保留，否则回落到默认（空）。
func NormalizeEffort(model ModelInfo, effort string) string {
	if effort == "" {
		return ""
	}
	for _, level := range model.Thinking {
		if level == effort {
			return effort
		}
	}
	return ""
}
