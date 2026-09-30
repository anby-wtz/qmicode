package agent

import (
	"context"
	"errors"

	"qmicode-go/codelib/modelconfig"
	"qmicode-go/codelib/session"
	"qmicode-go/codelib/tool"

	"github.com/openai/openai-go"
)

// ErrInterrupted 表示本次请求/工具执行被用户主动中断（context 取消）。
// 调用方可用 errors.Is(err, agent.ErrInterrupted) 区分"用户中断"与真正的错误。
var ErrInterrupted = errors.New("interrupted by user")

type ChunkResultFuncCallback func(string, LLMChunkContentType)
type PreMsgFilterFuncCallback func(string, error) bool
type AfterMsgFuncCallback func(string, error)

type Agent interface {
	GetInitLLMSession(name string) *session.LLMSession
	// Request 发起一次流式对话。ctx 被取消时立即停止，并把已产出的部分输出保留为一条 assistant message（若有任何产出）。
	Request(ctx context.Context, config modelconfig.ModelConfig, sess *session.LLMSession, result ChunkResultFuncCallback) ([]openai.ChatCompletionMessageToolCall, error)
	// ExecuteTools 依次执行工具调用。ctx 被取消时，当前及后续的工具都会被标记为 Aborted（写入对应 tool message），保证会话一致性。
	ExecuteTools(ctx context.Context, tcs []openai.ChatCompletionMessageToolCall, sess *session.LLMSession, premsgfunc PreMsgFilterFuncCallback, aftermsgfunc AfterMsgFuncCallback) error
	// Compact 压缩上下文。ctx 被取消时立即停止，不修改会话状态。
	Compact(ctx context.Context, config modelconfig.ModelConfig, sess *session.LLMSession, result ChunkResultFuncCallback) error
	Destroy()

	GetName() string

	AddTool(tool.Tool) bool      // true: add success, false: add failed or already exist
	RemoveTool(name string) bool // true: exist and remove success, false: not exist or remove failed
}
