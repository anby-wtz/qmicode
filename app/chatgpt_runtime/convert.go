package chatgpt_runtime

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"qmicode-go/codelib/agent"
	"qmicode-go/codelib/session"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/shared/constant"

	"github.com/sonnes/gopi/pkg/ai"
)

// reasoningExtraKey 是 assistant 消息中存放加密推理块的 Extra 键。
const reasoningExtraKey = "chatgpt_reasoning"

// reasoningItem 是一个推理块在会话中的持久化形式。
type reasoningItem struct {
	ID               string `json:"id"`
	EncryptedContent string `json:"encrypted_content,omitempty"`
	Text             string `json:"text,omitempty"`
}

// signature 返回 gopi 期望的签名形式。
func (r reasoningItem) signature() string {
	data, _ := json.Marshal(struct {
		ID               string `json:"id"`
		EncryptedContent string `json:"encrypted_content,omitempty"`
	}{r.ID, r.EncryptedContent})
	return string(data)
}

func parseSignature(s string) (reasoningItem, bool) {
	var item struct {
		ID               string `json:"id"`
		EncryptedContent string `json:"encrypted_content,omitempty"`
	}
	if err := json.Unmarshal([]byte(s), &item); err != nil || item.ID == "" {
		return reasoningItem{}, false
	}
	return reasoningItem{ID: item.ID, EncryptedContent: item.EncryptedContent}, true
}

func reasoningItemsOf(msg *session.LLMMessage) []reasoningItem {
	if msg == nil || msg.Extra == nil {
		return nil
	}
	raw, ok := msg.Extra[reasoningExtraKey]
	if !ok {
		return nil
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var items []reasoningItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil
	}
	return items
}

func setReasoningItems(msg *session.LLMMessage, items []reasoningItem) {
	if len(items) == 0 {
		return
	}
	if msg.Extra == nil {
		msg.Extra = map[string]any{}
	}
	msg.Extra[reasoningExtraKey] = items
}

func toMessages(sess *session.LLMSession) ([]ai.Message, error) {
	msgs := sess.GetMessageList()
	out := make([]ai.Message, 0, len(msgs))

	if len(msgs) > 0 && msgs[0].Type == session.LLMMessageTypeCompact {
		if msgs[0].Content == nil || msgs[0].Content.Text == nil {
			return nil, fmt.Errorf("compact content is nil")
		}
		out = append(out, ai.UserMessage("What did we do so far?"))
		out = append(out, ai.AssistantMessage(ai.Text{Text: *msgs[0].Content.Text}))
	}

	for _, msg := range msgs {
		if msg.Type != session.LLMMessageTypeSession {
			continue
		}
		content := msg.Content
		if content == nil {
			return nil, fmt.Errorf("message content is nil")
		}

		switch msg.Role {
		case "user":
			parts, err := userContent(content)
			if err != nil {
				return nil, err
			}
			if len(parts) > 0 {
				out = append(out, ai.Message{Role: ai.RoleUser, Content: parts})
			}
		case "assistant":
			if parts := assistantContent(msg, content); len(parts) > 0 {
				out = append(out, ai.Message{Role: ai.RoleAssistant, Content: parts})
			}
		case "tool":
			if content.ToolCallID != nil && content.Text != nil {
				out = append(out, ai.ToolResultMessage(*content.ToolCallID, ai.Text{Text: *content.Text}))
			}
		}
	}

	return out, nil
}

func userContent(content *session.LLMMessageContent) ([]ai.Content, error) {
	var parts []ai.Content

	if content.Text != nil && *content.Text != "" {
		parts = append(parts, ai.Text{Text: *content.Text})
	}
	for _, part := range content.ContentParts {
		if part == nil || part.OfImageURL == nil {
			continue
		}
		file, ok := dataURLFile(*part.OfImageURL)
		if !ok {
			return nil, fmt.Errorf("chatgpt: unsupported attachment, only base64 data URLs are accepted")
		}
		if !supportedMedia(file.MediaType) {
			return nil, fmt.Errorf("chatgpt: unsupported attachment type %q", file.MediaType)
		}
		parts = append(parts, file)
	}
	return parts, nil
}

// supportedMedia 报告 Responses 路由可接受的附件类型。
func supportedMedia(mediaType string) bool {
	return strings.HasPrefix(mediaType, "image/") || mediaType == "application/pdf"
}

// dataURLFile 把 data URL 还原为 gopi 的二进制文件块。
func dataURLFile(url string) (ai.File, bool) {
	rest, ok := strings.CutPrefix(url, "data:")
	if !ok {
		return ai.File{}, false
	}
	meta, data, ok := strings.Cut(rest, ",")
	if !ok {
		return ai.File{}, false
	}
	mediaType, encoding, _ := strings.Cut(meta, ";")
	if encoding != "base64" || mediaType == "" {
		return ai.File{}, false
	}
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return ai.File{}, false
	}
	return ai.File{MediaType: mediaType, Data: raw}, true
}

func assistantContent(msg *session.LLMMessage, content *session.LLMMessageContent) []ai.Content {
	var parts []ai.Content

	for _, item := range reasoningItemsOf(msg) {
		parts = append(parts, ai.ThinkingBlock{Thinking: item.Text, Signature: item.signature()})
	}
	if content.Text != nil && *content.Text != "" {
		parts = append(parts, ai.Text{Text: *content.Text})
	}
	for _, tc := range content.ToolCalls {
		parts = append(parts, ai.ToolCall{
			ID:    tc.ID,
			Name:  tc.Funtion.Name,
			Input: json.RawMessage(tc.Funtion.Arguments),
		})
	}
	return parts
}

func toToolInfos(ag agent.Agent) ([]ai.ToolInfo, error) {
	tools := ag.ListTools()
	out := make([]ai.ToolInfo, 0, len(tools))

	for _, t := range tools {
		schema, err := toSchema(t.GetParameters())
		if err != nil {
			return nil, fmt.Errorf("tool %s: %w", t.GetName(), err)
		}
		out = append(out, ai.ToolInfo{
			Name:        t.GetName(),
			Description: t.GetDescription(),
			InputSchema: schema,
		})
	}
	return out, nil
}

func toSchema(params map[string]any) (*jsonschema.Schema, error) {
	if params == nil {
		return &jsonschema.Schema{Type: "object"}, nil
	}
	data, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}
	var schema jsonschema.Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	return &schema, nil
}

func toToolCalls(msg *ai.Message) []agent.ToolCall {
	calls := msg.ToolCalls()
	out := make([]agent.ToolCall, 0, len(calls))

	for _, c := range calls {
		out = append(out, openai.ChatCompletionMessageToolCall{
			ID:   c.ID,
			Type: constant.Function("function"),
			Function: openai.ChatCompletionMessageToolCallFunction{
				Name:      stripNamespace(c.Name),
				Arguments: string(c.Input),
			},
		})
	}
	return out
}

// toolNamespace 是 ChatGPT 套餐路由上函数工具所在的命名空间（与 gopi 的 ChatGPTPlan 一致）。
const toolNamespace = "gopi."

// stripNamespace 兜底剥掉工具命名空间前缀（命名空间内的函数名本身不带前缀，此为防御）。
func stripNamespace(name string) string {
	return strings.TrimPrefix(name, toolNamespace)
}

func toSessionToolCalls(tcs []agent.ToolCall) []session.LLMToolCall {
	out := make([]session.LLMToolCall, len(tcs))
	for i, tc := range tcs {
		out[i] = session.LLMToolCall{
			ID: tc.ID,
			Funtion: session.LLMToolCallFunction{
				Name:      tc.Function.Name,
				Arguments: tc.Function.Arguments,
			},
		}
	}
	return out
}

// responseText 取出回答正文与推理摘要。
func responseText(msg *ai.Message) (answer string, reasoning string) {
	var answers, thoughts []string
	for _, c := range msg.Content {
		switch v := c.(type) {
		case ai.Text:
			answers = append(answers, v.Text)
		case ai.ThinkingBlock:
			if v.Thinking != "" {
				thoughts = append(thoughts, v.Thinking)
			}
		}
	}
	return strings.Join(answers, ""), strings.Join(thoughts, "\n\n")
}

// responseReasoningItems 取出可回传的加密推理块。
func responseReasoningItems(msg *ai.Message) []reasoningItem {
	var items []reasoningItem
	for _, c := range msg.Content {
		block, ok := c.(ai.ThinkingBlock)
		if !ok || block.Signature == "" {
			continue
		}
		if item, ok := parseSignature(block.Signature); ok {
			item.Text = block.Thinking
			items = append(items, item)
		}
	}
	return items
}

func usageTotal(usage ai.Usage) int {
	return usage.Input + usage.Output + usage.CacheRead + usage.CacheWrite
}
