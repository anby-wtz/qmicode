package tool

import (
	"context"

	"github.com/openai/openai-go"
)

type Tool interface {
	ToOpenAITool() openai.ChatCompletionToolParam
	CallPreMsgFunc(map[string]any) (string, error)
	// CallExecFunc 执行工具。ctx 被取消表示用户中断
	CallExecFunc(ctx context.Context, args map[string]any) (string, string, error)

	GetName() string
	GetDescription() string
	GetParameters() map[string]any
}
