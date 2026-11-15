package session

// SystemMessage由Agent控制，与Session无关
type LLMMessageType int

const (
	LLMMessageTypeSession LLMMessageType = iota // 对话相关消息，包括user、assistant、tool等
	LLMMessageTypeCompact                       // 压缩
)

type LLMMessage struct {
	Type    LLMMessageType     `json:"type"`
	Role    string             `json:"role"`
	Content *LLMMessageContent `json:"content"`
	Extra   map[string]any     `json:"extra,omitempty"` // 附加信息
}

// 内容类型
type LLMMessageContentType int

type LLMContentPart struct {
	OfImageURL *string `json:"image_url,omitempty"`
}

type LLMToolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type LLMToolCall struct {
	ID      string              `json:"id"`
	Funtion LLMToolCallFunction `json:"function"`
}

type LLMMessageContent struct {
	Text         *string           `json:"text,omitempty"`
	ContentParts []*LLMContentPart `json:"content_parts,omitempty"`
	Reasoning    *string           `json:"reasoning,omitempty"`    // reasoning内容
	ToolCallID   *string           `json:"tool_call_id,omitempty"` // tool call id (仅用于role=tool的消息中存放结果)
	ToolCalls    []LLMToolCall     `json:"tool_calls,omitempty"`   // tool call列表，用于assistant消息
}

func NewUserMessage(text *string, image []string) *LLMMessage {
	res := &LLMMessage{
		Type: LLMMessageTypeSession,
		Role: "user",
		Content: &LLMMessageContent{
			Text:         text,
			ContentParts: []*LLMContentPart{},
		},
	}
	for _, url := range image {
		res.Content.ContentParts = append(res.Content.ContentParts, &LLMContentPart{
			OfImageURL: &url,
		})
	}
	return res
}

func NewAssistantMessage(content string, reasoning *string, toolCalls []LLMToolCall) *LLMMessage {
	return &LLMMessage{
		Type: LLMMessageTypeSession,
		Role: "assistant",
		Content: &LLMMessageContent{
			Text:      &content,
			Reasoning: reasoning,
			ToolCalls: toolCalls,
		},
	}
}

func NewToolCallMessage(toolCallID string, content string) *LLMMessage {
	return &LLMMessage{
		Type: LLMMessageTypeSession,
		Role: "tool",
		Content: &LLMMessageContent{
			Text:       &content,
			ToolCallID: &toolCallID,
		},
	}
}
