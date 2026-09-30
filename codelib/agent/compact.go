package agent

import (
	"context"
	"encoding/json"
	"qmicode-go/codelib/modelconfig"
	"qmicode-go/codelib/session"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/packages/param"
)

// Compact 压缩上下文，并自动清理并设置历史记录
func (a *PathAgent) Compact(ctx context.Context, config modelconfig.ModelConfig, sess *session.LLMSession, result ChunkResultFuncCallback) error {
	if ctx == nil {
		ctx = context.Background()
	}
	client := openai.NewClient(
		option.WithBaseURL(config.BaseURL),
		option.WithAPIKey(config.ApiKey),
	)
	msgstr, err := sess.GetMessageString()
	if err != nil {
		return err
	}
	stream := client.Chat.Completions.NewStreaming(ctx, openai.ChatCompletionNewParams{
		Model: config.Model,
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.SystemMessage(a.CptPrompt),
			openai.UserMessage(msgstr),
		},
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
			err := json.Unmarshal([]byte(raw), &rawMap)
			if err != nil {
				continue
			}
			reasoning, ok := rawMap["reasoning"]
			if ok && reasoning != "" {
				result(reasoning, ContentReasoning)
				continue
			}
			reasoning, ok = rawMap["reasoning_content"]
			if ok && reasoning != "" {
				result(reasoning, ContentReasoning)
				continue
			}

			if choice.Delta.Content != "" {
				result(choice.Delta.Content, ContentAnswer)
				continue
			}
		}
	}
	if ctx.Err() != nil {
		// 用户中断：不修改会话状态，直接返回
		return ErrInterrupted
	}
	if err := stream.Err(); err != nil {
		return err
	}

	msg := acc.Choices[0].Message
	sess.AddCompactMessage(msg.Content)

	sess.ClearMessages()
	sess.AddMessage(openai.SystemMessage(a.SysPrompt))

	// 压缩信息已经在上文中增加，这里只需要向Message中增加即可
	sess.AddMessageOnlyForLLM(openai.UserMessage("What did we do so far?"))
	sess.AddMessageOnlyForLLM(msg.ToParam())

	return nil

}
