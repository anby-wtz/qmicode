package session

import (
	"fmt"

	"github.com/openai/openai-go"
)

func (sess *LLMSession) GetOpenAIMessages() ([]openai.ChatCompletionMessageParamUnion, error) {
	msgs := sess.GetMessageList()
	res := []openai.ChatCompletionMessageParamUnion{}
	res = append(res, openai.SystemMessage(sess.SystemMessage))
	if len(msgs) == 0 {
		return res, nil
	}
	// 处理压缩
	if msgs[0].Type == LLMMessageTypeCompact {
		if msgs[0].Content == nil || msgs[0].Content.Text == nil {
			return nil, fmt.Errorf("compact content is nil")
		}
		res = append(res, openai.UserMessage("What did we do so far?"))
		res = append(res, openai.AssistantMessage(*msgs[0].Content.Text))
	}
	for _, msg := range msgs {
		if msg.Type == LLMMessageTypeSession {
			content := msg.Content
			if content == nil {
				return nil, fmt.Errorf("message content is nil")
			}
			switch msg.Role {
			case "user":
				res = append(res, HoldUserMessage(content))
			case "assistant":
				res = append(res, HoldAssistantMessage(content))
			case "tool":
				if content.ToolCallID != nil && content.Text != nil {
					res = append(res, openai.ToolMessage(*content.Text, *content.ToolCallID))
				}
			}
		}
	}
	return res, nil
}

func HoldUserMessage(content *LLMMessageContent) openai.ChatCompletionMessageParamUnion {
	parts := make([]openai.ChatCompletionContentPartUnionParam, 0)
	if content.Text != nil && *content.Text != "" {
		parts = append(parts, openai.TextContentPart(*content.Text))
	}
	if content.ContentParts != nil {
		for _, part := range content.ContentParts {
			if part.OfImageURL != nil {
				parts = append(parts, openai.ImageContentPart(openai.ChatCompletionContentPartImageImageURLParam{
					URL:    *part.OfImageURL,
					Detail: string(openai.ImageURLDeltaDetailAuto),
				}))
			}
		}
	}
	return openai.UserMessage(parts)
}

func HoldAssistantMessage(content *LLMMessageContent) openai.ChatCompletionMessageParamUnion {
	text := ""
	if content.Text != nil {
		text = *content.Text
	}
	res := openai.AssistantMessage(text)
	if content.Reasoning != nil {
		res.SetExtraFields(
			map[string]any{
				"reasoning":         *content.Reasoning,
				"reasoning_content": *content.Reasoning,
			}, // 兼容两个字段
		)
	}
	params := make([]openai.ChatCompletionMessageToolCallParam, len(content.ToolCalls))
	for i, toolCall := range content.ToolCalls {
		params[i] = openai.ChatCompletionMessageToolCallParam{
			ID:       toolCall.ID,
			Type:     "function",
			Function: openai.ChatCompletionMessageToolCallFunctionParam{Name: toolCall.Funtion.Name, Arguments: toolCall.Funtion.Arguments},
		}
	}
	res.OfAssistant.ToolCalls = params
	return res
}
