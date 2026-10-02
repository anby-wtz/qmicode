package agent

import (
	"errors"

	"qmicode-go/codelib/session"
	"qmicode-go/codelib/tool"

	"github.com/openai/openai-go"
)

// ErrInterrupted 表示本次请求/工具执行被用户主动中断（context 取消）。
// 调用方可用 errors.Is(err, agent.ErrInterrupted) 区分"用户中断"与真正的错误。
var ErrInterrupted = errors.New("interrupted by user")

// ToolCall 工具调用类型。目前是 OpenAI ChatCompletionMessageToolCall 的别名；
// 将来若引入非 OpenAI Runtime / 中立消息模型，替换为自有结构体即可平滑过渡。
type ToolCall = openai.ChatCompletionMessageToolCall

type ChunkResultFuncCallback func(string, LLMChunkContentType)
type PreMsgFilterFuncCallback func(string, error) bool
type AfterMsgFuncCallback func(string, error)

// Agent 定义层：提示词的加载与工具的定义/注册。
// 不包含任何执行逻辑（LLM 请求、工具调度等属于 AgentRuntime）。
type Agent interface {
	GetName() string

	// GetSysPrompt / GetCompactPrompt 返回已加载的系统提示词 / 压缩提示词。
	GetSysPrompt() string
	GetCompactPrompt() string

	// ListTools 返回当前全部可用工具（含动态注册的工具）的统一视图。
	ListTools() []tool.Tool
	// FindTool 按名称查找工具。
	FindTool(name string) (tool.Tool, bool)
	AddTool(tool tool.Tool) bool      // true: add success, false: add failed or already exist
	RemoveTool(name string) bool     // true: exist and remove success, false: not exist or remove failed

	// GetInitLLMSession 基于本 Agent 的系统提示词创建一个初始会话。
	GetInitLLMSession(name string) *session.LLMSession

	// Destroy 释放加载的资源（工具实现等），只能调用一次。
	Destroy()
}
