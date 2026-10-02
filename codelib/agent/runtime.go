package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"qmicode-go/codelib/modelconfig"
	"qmicode-go/codelib/session"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

// AgentRuntime 执行层：基于某个 Agent（提示词 + 工具定义）对一次会话进行 LLM 请求与工具调度。
// Runtime 不持有任何资源，无生命周期方法；ModelConfig 在每次调用模型时传入，
// 因此 Agent、ModelConfig、Runtime 三者可任意插拔组合。
type AgentRuntime interface {
	// Request 发起一次流式对话。ctx 被取消时立即停止，并把已产出的部分输出保留为一条 assistant message（若有任何产出）。
	Request(ctx context.Context, config modelconfig.ModelConfig, sess *session.LLMSession, result ChunkResultFuncCallback) ([]ToolCall, error)
	// ExecuteTools 依次执行工具调用。ctx 被取消时，当前及后续的工具都会被标记为 Aborted（写入对应 tool message），保证会话一致性。
	ExecuteTools(ctx context.Context, tcs []ToolCall, sess *session.LLMSession, premsgfunc PreMsgFilterFuncCallback, aftermsgfunc AfterMsgFuncCallback) error
	// Compact 压缩上下文。ctx 被取消时立即停止，不修改会话状态。
	Compact(ctx context.Context, config modelconfig.ModelConfig, sess *session.LLMSession, result ChunkResultFuncCallback) error
}

type LLMChunkContentType int

const (
	ContentReasoning LLMChunkContentType = iota
	ContentAnswer
)

// NewAgentRuntime 创建一个基于 OpenAI Chat Completions API 协议的默认 Runtime。
func NewAgentRuntime(ag Agent) *OpenAIRuntime {
	return &OpenAIRuntime{agent: ag}
}

// OpenAIRuntime 是基于 OpenAI Chat Completions API 的 Runtime 实现，
// 负责 LLM 流式请求、chunk 分发与工具调度；提示词和工具均取自所绑定的 Agent。
type OpenAIRuntime struct {
	agent Agent
}

// 检查接口实现
var _ AgentRuntime = (*OpenAIRuntime)(nil)

func newClient(config modelconfig.ModelConfig) openai.Client {
	return openai.NewClient(
		option.WithBaseURL(config.BaseURL),
		option.WithAPIKey(config.ApiKey),
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
	stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model:    config.Model,
		Messages: messages,
		Tools:    tools,
		StreamOptions: openai.ChatCompletionStreamOptionsParam{
			IncludeUsage: param.Opt[bool]{Value: true},
		},
	})

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
			if !ok || reasoning == "" {
				reasoning, ok = rawMap["reasoning_content"]
			}
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

func (r *OpenAIRuntime) buildTools() []openai.ChatCompletionToolParam {
	openaiTools := make([]openai.ChatCompletionToolParam, 0)
	for _, t := range r.agent.ListTools() {
		openaiTools = append(openaiTools, t.ToOpenAITool())
	}
	return openaiTools
}

func (r *OpenAIRuntime) Request(ctx context.Context, config modelconfig.ModelConfig, sess *session.LLMSession, result ChunkResultFuncCallback) ([]ToolCall, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	client := newClient(config)
	var partialContent string // 已产出的正文内容，用于中断时保留为 assistant message

	prevCallback := result
	result = func(s string, t LLMChunkContentType) {
		if t == ContentAnswer {
			partialContent += s
		}
		prevCallback(s, t)
	}

	msg, usage, err := r.collectStream(client, ctx, config, sess.Messages, r.buildTools(), result)
	if err != nil {
		if errors.Is(err, ErrInterrupted) && partialContent != "" {
			// 用户中断：保留已产出的部分输出为一条 assistant message（不保存未完成的 tool_calls，避免产生非法会话）
			sess.AddMessage(openai.AssistantMessage(partialContent))
		}
		return nil, err
	}

	sess.AddMessage(msg.ToParam())

	if len(msg.ToolCalls) > 0 {
		return msg.ToolCalls, nil
	}

	sess.Usage = usage

	return nil, nil
}

func (r *OpenAIRuntime) ExecuteTools(ctx context.Context, ts []ToolCall, sess *session.LLMSession, premsgfunc PreMsgFilterFuncCallback, aftermsgfunc AfterMsgFuncCallback) error {
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
				openai.ToolMessage("Aborted by user", tc.ID),
			)
			interrupted = true
			continue
		}

		t, ok := r.agent.FindTool(tc.Function.Name)
		if !ok {
			sess.AddMessage(openai.ToolMessage(fmt.Sprintf("Error: tool %s not found", tc.Function.Name), tc.ID))
			continue
		}

		premsg, err := t.CallPreMsgFunc(args)
		if !premsgfunc(premsg, err) {
			sess.AddMessage(
				openai.ToolMessage("Human Aborted", tc.ID),
			)
			continue
		}

		res, human, err := t.CallExecFunc(ctx, args)
		if err != nil {
			res = fmt.Sprintf("Error: %v", err)
		}
		aftermsgfunc(human, err)
		sess.AddMessage(
			openai.ToolMessage(res, tc.ID),
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

func (r *OpenAIRuntime) Compact(ctx context.Context, config modelconfig.ModelConfig, sess *session.LLMSession, result ChunkResultFuncCallback) error {
	if ctx == nil {
		ctx = context.Background()
	}
	client := newClient(config)
	msgstr, err := sess.GetMessageString()
	if err != nil {
		return err
	}

	msg, _, err := r.collectStream(client, ctx, config, []openai.ChatCompletionMessageParamUnion{
		openai.SystemMessage(r.agent.GetCompactPrompt()),
		openai.UserMessage(msgstr),
	}, nil, result)
	if err != nil {
		// 用户中断：不修改会话状态，直接返回
		return err
	}

	sess.AddCompactMessage(msg.Content)
	sess.ClearMessages()
	sess.AddMessage(openai.SystemMessage(r.agent.GetSysPrompt()))

	// 压缩信息已经在上文中增加，这里只需要向 Message 中增加即可
	sess.AddMessageOnlyForLLM(openai.UserMessage("What did we do so far?"))
	sess.AddMessageOnlyForLLM(msg.ToParam())

	return nil
}
