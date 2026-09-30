package tool

import (
	"context"

	"github.com/openai/openai-go"
)

type Tool interface {
	ToOpenAITool() openai.ChatCompletionToolParam
	CallPreMsgFunc(map[string]any) (string, error)
	// CallExecFunc 执行工具。ctx 被取消表示用户中断：Go 侧的阻塞调用（如子进程）应尽快终止，
	// Lua 脚本可通过 qmi_is_interrupted() API 协作式检查并自行退出。
	CallExecFunc(ctx context.Context, args map[string]any) (string, string, error)

	GetName() string
	GetDescription() string
	GetParameters() map[string]any
}
