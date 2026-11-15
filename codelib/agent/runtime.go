package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"qmicode-go/codelib"
	"qmicode-go/codelib/modelconfig"
	"qmicode-go/codelib/session"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
	"github.com/openai/openai-go/shared"
)

// AgentRuntime 执行层：对一次会话进行 LLM 请求与工具调度。
// Runtime 自身完全无状态、不持有资源、无生命周期方法：Agent 与 ModelConfig
// 都在每次调用时传入，因此 Agent、ModelConfig、Runtime 三者严格正交、可任意插拔。
type AgentRuntime interface {
	// Request 发起一次流式对话。ctx 被取消时立即停止，并把已产出的部分输出保留为一条 assistant message（若有任何产出）。
	Request(ctx context.Context, ag Agent, config modelconfig.ModelConfig, sess *session.LLMSession, result ChunkResultFuncCallback) ([]ToolCall, error)
	// ExecuteTools 依次执行工具调用。ctx 被取消时，当前及后续的工具都会被标记为 Aborted（写入对应 tool message），保证会话一致性。
	ExecuteTools(ctx context.Context, ag Agent, tcs []ToolCall, sess *session.LLMSession, premsgfunc PreMsgFilterFuncCallback, aftermsgfunc AfterMsgFuncCallback) error
	// Compact 压缩上下文。ctx 被取消时立即停止，不修改会话状态。
	Compact(ctx context.Context, ag Agent, config modelconfig.ModelConfig, sess *session.LLMSession, result ChunkResultFuncCallback) error
}

type LLMChunkContentType int

const (
	ContentReasoning LLMChunkContentType = iota
	ContentAnswer
)

// NewAgentRuntime 创建一个基于 OpenAI Chat Completions API 协议的默认 Runtime。
// 不接收任何依赖：Agent 与 ModelConfig 均由调用方在每次调用时传入。
func NewAgentRuntime() *OpenAIRuntime {
	return &OpenAIRuntime{}
}

// OpenAIRuntime 是基于 OpenAI Chat Completions API 的 Runtime 实现，
// 负责 LLM 流式请求、chunk 分发与工具调度。它是无状态的：
// 提示词与工具一律取自每次调用传入的 Agent，不保存任何引用。
type OpenAIRuntime struct{}

// 检查接口实现
var _ AgentRuntime = (*OpenAIRuntime)(nil)

func newClient(config modelconfig.ModelConfig) openai.Client {
	return openai.NewClient(
		option.WithBaseURL(config.BaseURL),
		option.WithAPIKey(config.ApiKey),
		option.WithHeader("User-Agent", fmt.Sprintf("QmiCode/%s (+%s)", codelib.Version, codelib.Github)),
	)
}

// collectStream 消费一次流式补全：分发 chunk（reasoning / answer）、累积结果，
// 并返回最终的 message 与 usage。Request 和 Compact 共用此逻辑。
func (r *OpenAIRuntime) collectStream(
	client openai.Client,
	ctx context.Context,
	config modelconfig.ModelConfig,
	messages []openai.ChatCompletionMessageParamUnion,
	tools []openai.ChatCompletionToolParam,
	result ChunkResultFuncCallback,
) (*openai.ChatCompletionMessage, int, error) {
	params := openai.ChatCompletionNewParams{
		Model:    config.Model,
		Messages: messages,
		Tools:    tools,
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: param.Opt[bool]{Value: true},
		},
	}

	if config.Options.ReasoningEffort != "" { // string的零值是""
		params.ReasoningEffort = shared.ReasoningEffort(config.Options.ReasoningEffort)
	}

	if config.Options.ExtraBody != nil {
		params.SetExtraFields(config.Options.ExtraBody)
	}

	stream := client.Chat.Completions.NewStreaming(ctx, params)

	var acc openai.ChatCompletionAccumulator
	for stream.Next() {
		chunk := stream.Current()
		acc.AddChunk(chunk)

		for _, choice := range chunk.Choices {
			raw := choice.Delta.RawJSON()
			var rawMap map[string]string
			if err := json.Unmarshal([]byte(raw), &rawMap); err != nil {
				continue
			}
			reasoning, ok := rawMap["reasoning"]
			if ok && reasoning != "" {
				result(reasoning, ContentReasoning)
			}
			reasoning, ok = rawMap["reasoning_content"]
			if ok && reasoning != "" {
				result(reasoning, ContentReasoning)
				continue
			}

			if choice.Delta.Content != "" {
				result(choice.Delta.Content, ContentAnswer)
			}
		}
	}

	if ctx.Err() != nil {
		return nil, 0, ErrInterrupted
	}
	if err := stream.Err(); err != nil {
		return nil, 0, err
	}

	msg := acc.Choices[0].Message
	usage := int(acc.Usage.TotalTokens)
	return &msg, usage, nil
}

func (r *OpenAIRuntime) buildTools(ag Agent) []openai.ChatCompletionToolParam {
	openaiTools := make([]openai.ChatCompletionToolParam, 0)
	for _, t := range ag.ListTools() {
		openaiTools = append(openaiTools, t.ToOpenAITool())
	}
	return openaiTools
}

func openai2toolcall(ts []openai.ChatCompletionMessageToolCall) []session.LLMToolCall {
	tcs := make([]session.LLMToolCall, len(ts))
	for i, t := range ts {
		tcs[i] = session.LLMToolCall{
			ID: t.ID,
			Funtion: session.LLMToolCallFunction{
				Name:      t.Function.Name,
				Arguments: t.Function.Arguments,
			},
		}
	}
	return tcs
}

func (r *OpenAIRuntime) Request(ctx context.Context, ag Agent, config modelconfig.ModelConfig, sess *session.LLMSession, result ChunkResultFuncCallback) ([]ToolCall, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client := newClient(config)
	var partialContent string // 已产出的正文内容，用于中断时保留为 assistant message
	var reasoning string

	prevCallback := result
	result = func(s string, t LLMChunkContentType) {
		switch t {
		case ContentAnswer:
			partialContent += s
			prevCallback(s, ContentAnswer)
			return
		case ContentReasoning:
			reasoning += s
			prevCallback(s, ContentReasoning)
			return
		}
	}

	msgRaw, err := sess.GetOpenAIMessages()
	if err != nil {
		return nil, err
	}
	msg, usage, err := r.collectStream(client, ctx, config, msgRaw, r.buildTools(ag), result)
	if err != nil {
		if errors.Is(err, ErrInterrupted) && partialContent != "" {
			// 用户中断：保留已产出的部分输出为一条 assistant message（不保存未完成的 tool_calls，避免产生非法会话）
			sess.AddMessage(session.NewAssistantMessage(partialContent, &reasoning, []session.LLMToolCall{}))
		}
		return nil, err
	}

	sess.AddMessage(session.NewAssistantMessage(msg.Content, &reasoning, openai2toolcall(msg.ToolCalls)))

	if len(msg.ToolCalls) > 0 {
		return msg.ToolCalls, nil
	}

	sess.Usage = usage

	return nil, nil
}

func (r *OpenAIRuntime) ExecuteTools(ctx context.Context, ag Agent, ts []ToolCall, sess *session.LLMSession, premsgfunc PreMsgFilterFuncCallback, aftermsgfunc AfterMsgFuncCallback) error {
	if ctx == nil {
		ctx = context.Background()
	}
	interrupted := false
	for _, tc := range ts {
		args := make(map[string]any)
		json.Unmarshal([]byte(tc.Function.Arguments), &args)

		// 已中断：当前及后续工具全部标记为 Aborted，保证 assistant.tool_calls 与 tool message 一一对应。
		if ctx.Err() != nil || interrupted {
			sess.AddMessage(
				session.NewToolCallMessage(tc.ID, "Aborted by user"),
			)
			interrupted = true
			continue
		}

		t, ok := ag.FindTool(tc.Function.Name)
		if !ok {
			sess.AddMessage(session.NewToolCallMessage(tc.ID, fmt.Sprintf("Error: tool %s not found", tc.Function.Name)))
			continue
		}

		premsg, err := t.CallPreMsgFunc(args)
		if !premsgfunc(premsg, err) {
			sess.AddMessage(
				session.NewToolCallMessage(tc.ID, "Human Aborted"),
			)
			continue
		}

		res, human, err := t.CallExecFunc(ctx, args)
		if err != nil {
			res = fmt.Sprintf("Error: %v", err)
		}
		aftermsgfunc(human, err)
		sess.AddMessage(
			session.NewToolCallMessage(tc.ID, res),
		)

		if ctx.Err() != nil {
			interrupted = true // 工具执行期间被中断：剩余工具在下一轮循环标记为 Aborted
		}
	}
	if interrupted {
		return ErrInterrupted
	}
	return nil
}

func (r *OpenAIRuntime) Compact(ctx context.Context, ag Agent, config modelconfig.ModelConfig, sess *session.LLMSession, result ChunkResultFuncCallback) error {
	if ctx == nil {
		ctx = context.Background()
	}
	client := newClient(config)
	msgstr, err := sess.GetMessageString()
	if err != nil {
		return err
	}

	msg, _, err := r.collectStream(client, ctx, config, []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(ag.GetCompactPrompt()),
		openai.UserMessage(msgstr),
	}, nil, result)
	if err != nil {
		// 用户中断直接返回
		return err
	}

	sess.AddCompactMessage(msg.Content)

	return nil
}
