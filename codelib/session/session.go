package session

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
)

type QmiCodeCompactInfoParam struct {
	Content string `json:"content"` // output of cpmpact request
	Role    string `json:"role"`    // always "compact"
}

// fuck fuck fuck stupid openai-go
// the sb developers banned extand message type
// fuck this shit
type LLMMessageUnion json.RawMessage

func (u LLMMessageUnion) MarshalJSON() ([]byte, error) {
	if u == nil {
		return []byte("null"), nil
	}
	return []byte(u), nil
}

func (u *LLMMessageUnion) UnmarshalJSON(data []byte) error {
	*u = append([]byte(nil), data...)
	return nil
}

type LLMSession struct {
	Name string `json:"name"` // session name, for display only

	Messages []openai.ChatCompletionMessageParamUnion `json:"messages"`
	Usage    int                                      `json:"usage"` // total tokens used
	Cost     int                                      `json:"cost"`  // future function

	// non-compacted messages, for humans to see
	EntireMessages []LLMMessageUnion `json:"entire_messages"` // because of the fucking openai-go library, don't directly use it as openai messages
}

func (sess *LLMSession) SetSessionName(name string) {
	sess.Name = name
}

// 压缩信息仅在EntireMessages中体现。这个函数仅用于添加压缩信息，不添加到Messages中
func (sess *LLMSession) AddCompactMessage(content string) {
	data, _ := json.Marshal(QmiCodeCompactInfoParam{
		Content: content,
		Role:    "compact",
	})
	sess.EntireMessages = append(sess.EntireMessages, LLMMessageUnion(data))
}

func (sess *LLMSession) AddUndoMessage() {
	data, _ := json.Marshal(QmiCodeCompactInfoParam{
		Content: "undo",
		Role:    "undo",
	})
	sess.EntireMessages = append(sess.EntireMessages, LLMMessageUnion(data))
}

func (sess *LLMSession) AddMessageOnlyForLLM(msg openai.ChatCompletionMessageParamUnion) {
	sess.Messages = append(sess.Messages, msg)
}

func (sess *LLMSession) AddMessage(msg openai.ChatCompletionMessageParamUnion) {
	sess.Messages = append(sess.Messages, msg)
	data, _ := json.Marshal(msg)
	sess.EntireMessages = append(sess.EntireMessages, LLMMessageUnion(data))
}

// 仅用于删除多余的最后的user消息
func (sess *LLMSession) DeleteLastMessage() {
	if len(sess.Messages) > 0 {
		sess.Messages = sess.Messages[:len(sess.Messages)-1]
		sess.EntireMessages = sess.EntireMessages[:len(sess.EntireMessages)-1]
	}
}

func (sess *LLMSession) GetJSON() (string, error) {
	data, err := json.Marshal(sess.Messages)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (sess *LLMSession) LoadFromJSON(data string) error {
	err := json.Unmarshal([]byte(data), &sess.Messages)
	if err != nil {
		return err
	}
	return nil
}

func (sess *LLMSession) GetMessageString() (string, error) {
	data, err := json.Marshal(sess.Messages)
	if err != nil {
		return "", err
	}

	var context []map[string]any
	json.Unmarshal(data, &context)

	var res strings.Builder

	for _, msg := range context {
		if role, ok := msg["role"]; ok && (role == "user" || role == "assistant") {
			if content, ok := msg["content"]; ok {
				res.WriteString(fmt.Sprintf("%s: \n%s\n\n--------\n\n", role, content))
				continue
			}
		}
	}

	return res.String(), nil
}

func (sess *LLMSession) ClearMessages() {
	sess.Messages = []openai.ChatCompletionMessageParamUnion{}
}

func (sess *LLMSession) SetSystemMessage(content string) {
	// 第一个一定是系统提示词

	if len(sess.Messages) > 0 {
		sess.Messages[0] = openai.SystemMessage(content)
		data, _ := json.Marshal(sess.Messages[0])
		sess.EntireMessages[0] = LLMMessageUnion(data)
	}
}

// string返回值：undo的消息前user说了什么
func (sess *LLMSession) Undo() (string, error) {
	n := len(sess.Messages)
	if n == 0 {
		return "", fmt.Errorf("no messages to undo")
	}
	// Undo到上一个用户消息
	for i := n - 1; i >= 0; i-- {
		if sess.Messages[i].OfUser != nil {
			content := sess.Messages[i].OfUser.Content.OfString.Value
			sess.Messages = sess.Messages[:i]
			sess.AddUndoMessage()
			return content, nil
		}
	}
	return "", fmt.Errorf("no message to undo")

}
