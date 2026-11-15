package session

import (
	"fmt"
	"strings"
)

/*
LLM的对话结构体类型
系统提示词由Agent管理，不计入Messages列表
Message采用链表结构，保存上下文和历史记录
向上追溯，要么追溯到根，要么追溯到上一个压缩信息
*/
type LLMSession struct {
	Name string `json:"name"`

	SystemMessage string `json:"system_message"`

	MessagesTree LLMMessageTree `json:"messages_tree"`
	CurrentIndex int            `json:"current_index"` // 当前消息索引

	Usage int `json:"usage"` // 当前上下文总共使用的Token（即占用上下文窗口的大小），非会话以来总共使用的Token
	Cost  int `json:"cost"`  // future function
}

type LLMMessageEntry struct {
	Message   *LLMMessage `json:"message"`
	PrevIndex int         `json:"prev_index"`
}

func NewLLMSession(name string) *LLMSession {
	return &LLMSession{
		Name:         name,
		MessagesTree: LLMMessageTree{},
		CurrentIndex: RootParentIndex,
	}
}

func (sess *LLMSession) SetSessionName(name string) {
	sess.Name = name
}

func (sess *LLMSession) AddCompactMessage(content string) {
	sess.CurrentIndex = sess.MessagesTree.AddNode(
		&LLMMessage{
			Type: LLMMessageTypeCompact,
			Role: "compact",
			Content: &LLMMessageContent{
				Text: &content,
			},
		},
		sess.CurrentIndex,
	)
}

func (sess *LLMSession) AddMessage(msg *LLMMessage) {
	sess.CurrentIndex = sess.MessagesTree.AddNode(msg, sess.CurrentIndex)
}

func (sess *LLMSession) GetMessageList() []*LLMMessage {
	res := make([]*LLMMessage, 0)
	for _, message := range sess.MessagesTree.IterFromChild(sess.CurrentIndex) {
		res = append(res, message)
		if message.Type == LLMMessageTypeCompact {
			break
		}
	}
	// reverse res
	for i, j := 0, len(res)-1; i < j; i, j = i+1, j-1 {
		res[i], res[j] = res[j], res[i]
	}
	return res
}

func (sess *LLMSession) GetMessageString() (string, error) {
	context := sess.GetMessageList()

	var res strings.Builder

	for _, msg := range context {
		if msg.Type == LLMMessageTypeSession && (msg.Role == "user" || msg.Role == "assistant") && msg.Content.Text != nil {
			res.WriteString(fmt.Sprintf("%s: \n%s\n\n--------\n\n", msg.Role, *msg.Content.Text))
		}
	}

	return res.String(), nil
}

func (sess *LLMSession) SetSystemMessage(content string) {
	sess.SystemMessage = content
}

// 返回值：undo的消息前user说了什么
func (sess *LLMSession) Undo() (*LLMMessageContent, error) {
	if sess.CurrentIndex < 0 {
		return nil, fmt.Errorf("no message to undo")
	}
	c := sess.CurrentIndex
	// 找到上一个User消息
	for index, message := range sess.MessagesTree.IterFromChild(c) {
		if message.Type == LLMMessageTypeSession && message.Role == "user" {
			sess.CurrentIndex, _ = sess.MessagesTree.GetParentIndex(index)
			return message.Content, nil
		}
	}
	return nil, fmt.Errorf("no user message to undo")
}
