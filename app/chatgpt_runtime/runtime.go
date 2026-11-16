package chatgpt_runtime

import (
	"context"
	"errors"
	"strings"

	"qmicode-go/app/config"
	"qmicode-go/codelib/agent"
	coremc "qmicode-go/codelib/modelconfig"
	"qmicode-go/codelib/session"

	"github.com/sonnes/gopi/pkg/ai"
)

// Runtime 是 ChatGPT 套餐（OAuth + Responses API）的 AgentRuntime 实现。
// 工具调度复用 OpenAIRuntime；Request/Compact 走 Responses API。
type Runtime struct {
	*agent.OpenAIRuntime

	conf    config.Config
	baseURL string
}

var _ agent.AgentRuntime = (*Runtime)(nil)

func New(conf config.Config, baseURL string) (*Runtime, error) {
	if err := checkSignedIn(conf); err != nil {
		return nil, err
	}
	return &Runtime{
		OpenAIRuntime: agent.NewAgentRuntime(),
		conf:          conf,
		baseURL:       baseURL,
	}, nil
}

func (r *Runtime) stream(ctx context.Context, model string, messages []ai.Message, opts ai.Options, result agent.ChunkResultFuncCallback) (*ai.Message, string, string, error) {
	events := provider(r.conf, r.baseURL).StreamText(ctx, ai.Model{ID: model}, messages, opts)

	var answer, reasoning strings.Builder
	for ev, err := range events.Events() {
		if err != nil {
			break
		}
		switch ev.Type {
		case ai.EventThinkDelta:
			if ev.Delta != "" {
				reasoning.WriteString(ev.Delta)
				result(ev.Delta, agent.ContentReasoning)
			}
		case ai.EventTextDelta:
			if ev.Delta != "" {
				answer.WriteString(ev.Delta)
				result(ev.Delta, agent.ContentAnswer)
			}
		}
	}

	msg, err := events.Wait()
	if err != nil {
		if ctx.Err() != nil {
			return nil, answer.String(), reasoning.String(), agent.ErrInterrupted
		}
		return nil, answer.String(), reasoning.String(), mapAuthError(err)
	}
	if msg == nil {
		return nil, answer.String(), reasoning.String(), errors.New("chatgpt: empty response")
	}
	return msg, answer.String(), reasoning.String(), nil
}

func (r *Runtime) options(sess *session.LLMSession, tools []ai.ToolInfo, cfg coremc.ModelConfig) (ai.Options, error) {
	opts := []ai.Options{
		ai.SystemPrompt([]ai.Content{ai.Text{Text: sess.SystemMessage}}),
		// 套餐路由会拒绝部分模型上的 prompt_cache_breakpoint，统一关闭显式缓存断点
		ai.CacheRetention(ai.CacheNone),
		ai.MaxRetries(0),
	}
	if len(tools) > 0 {
		opts = append(opts, ai.Tools(tools))
	}
	if cfg.Options.ReasoningEffort != "" {
		level, err := ai.ParseThinking(cfg.Options.ReasoningEffort)
		if err != nil {
			return nil, err
		}
		opts = append(opts, ai.Thinking(level))
	}
	return ai.JoinOptions(opts...), nil
}

func (r *Runtime) Request(ctx context.Context, ag agent.Agent, cfg coremc.ModelConfig, sess *session.LLMSession, result agent.ChunkResultFuncCallback) ([]agent.ToolCall, error) {
	if ctx == nil {
		ctx = context.Background()
	}

	messages, err := toMessages(sess)
	if err != nil {
		return nil, err
	}
	tools, err := toToolInfos(ag)
	if err != nil {
		return nil, err
	}
	opts, err := r.options(sess, tools, cfg)
	if err != nil {
		return nil, err
	}

	msg, answer, reasoning, err := r.stream(ctx, cfg.Model, messages, opts, result)
	if err != nil {
		if errors.Is(err, agent.ErrInterrupted) && (answer != "" || reasoning != "") {
			sess.AddMessage(session.NewAssistantMessage(answer, &reasoning, []session.LLMToolCall{}))
		}
		return nil, err
	}

	tcs := toToolCalls(msg)
	content, thoughts := responseText(msg)
	if content == "" && answer != "" {
		content = answer
	}
	if thoughts == "" {
		thoughts = reasoning
	}

	assistant := session.NewAssistantMessage(content, &thoughts, toSessionToolCalls(tcs))
	setReasoningItems(assistant, responseReasoningItems(msg))
	sess.AddMessage(assistant)

	if len(tcs) > 0 {
		return tcs, nil
	}

	sess.Usage = usageTotal(msg.Usage)
	return nil, nil
}

func (r *Runtime) Compact(ctx context.Context, ag agent.Agent, cfg coremc.ModelConfig, sess *session.LLMSession, result agent.ChunkResultFuncCallback) error {
	if ctx == nil {
		ctx = context.Background()
	}

	msgstr, err := sess.GetMessageString()
	if err != nil {
		return err
	}

	opts := ai.JoinOptions(
		ai.SystemPrompt([]ai.Content{ai.Text{Text: ag.GetCompactPrompt()}}),
		ai.CacheRetention(ai.CacheNone),
		ai.MaxRetries(0),
	)

	msg, _, _, err := r.stream(ctx, cfg.Model, []ai.Message{ai.UserMessage(msgstr)}, opts, result)
	if err != nil {
		return err
	}

	content, _ := responseText(msg)
	sess.AddCompactMessage(content)
	return nil
}
