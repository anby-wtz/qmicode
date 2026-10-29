package command

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	appsess "qmicode-go/app/session"
	"qmicode-go/cli/appstate"
	"qmicode-go/cli/term"
	"qmicode-go/codelib/agent"
	"qmicode-go/codelib/session"
)

func color(role string) string {
	switch role {
	case "user":
		return term.Cyan
	case "assistant":
		return term.LightGray + term.Bold
	}
	return ""
}

// renderMessages 把会话的完整历史（含 compact 记录和 undo 记录）渲染为带颜色的文本。
func renderMessages(sess *session.LLMSession) (string, error) {
	data, err := json.Marshal(sess.EntireMessages)
	if err != nil {
		return "", err
	}

	var context []map[string]any
	json.Unmarshal(data, &context)

	var res strings.Builder

	for _, msg := range context {
		role, ok := msg["role"].(string)
		if ok {
			switch role {
			case "user", "assistant":
				if content, ok := msg["content"]; ok {
					res.WriteString(fmt.Sprintf("%s%s%s\n", color(role), content, term.Reset))
				}
			case "compact":
				if content, ok := msg["content"]; ok {
					res.WriteString(fmt.Sprintf("%s%sCompact:%s\n", term.Green, term.Bold, term.Reset))
					res.WriteString(fmt.Sprintf("%s%s%s\n", color("assistant"), content, term.Reset))
				}
			case "undo":
				if _, ok := msg["content"]; ok {
					res.WriteString(fmt.Sprintf("%s%sUndo last message%s\n", term.Yellow, term.Bold, term.Reset))
				}
			default:
				continue
			}
		}
	}

	return res.String(), nil
}

// ShowSessionJSON 处理 /shsejson：打印会话中实际发送给 LLM 的消息 JSON。
func ShowSessionJSON(app *appstate.App) bool {
	str, err := app.Sess.GetJSON()
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	fmt.Printf("%sSession JSON:%s\n%s\n", term.Magenta, term.Reset, str)
	return false
}

// Compact 处理 /compact：压缩上下文。
// result 由 cli 包注入（resCallback），用于流式输出 reasoning/answer。
func Compact(app *appstate.App, result agent.ChunkResultFuncCallback) bool {
	if err := app.Rt.Compact(context.Background(), app.Ag, app.Model, app.Sess, result); err != nil {
		if errors.Is(err, agent.ErrInterrupted) {
			fmt.Printf("\n%s[Interrupted]%s Compact aborted, session unchanged.%s\n", term.Yellow, term.Reset, term.Reset)
		} else {
			fmt.Printf("%sError during compact: %v%s\n", term.Red, err.Error(), term.Reset)
		}
		return false
	}
	fmt.Printf("\n%s%sCompact Complete!%s\n", term.Green, term.Bold, term.Reset)
	return false
}

// ShowSession 处理 /shsess：显示当前会话名。
func ShowSession(app *appstate.App) bool {
	fmt.Printf("%sSession name:%s%s\n", term.Magenta, term.Reset, app.Sess.Name)
	return false
}

// ListSession 处理 /lssess：列出全部会话，并标记当前会话。
func ListSession(app *appstate.App) bool {
	names, err := appsess.ListSessions(app.Conf)
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	fmt.Print(term.Yellow)
	for i := range names {
		fmt.Print(names[i])
		if names[i] == app.Sess.Name {
			fmt.Print(term.Green)
			fmt.Print("*(current)")
			fmt.Print(term.Reset)
			fmt.Print(term.Yellow)
		}
		fmt.Println()
	}
	fmt.Print(term.Reset)
	return false
}

// LoadSession 处理 /lsess：加载会话并重绘历史。
func LoadSession(app *appstate.App) bool {
	name, err := term.Ask("name: ")
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	err = appsess.LoadSession(app.Conf, name, app.Sess)
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	fmt.Printf("%sLoaded session: %s%s\n", term.Green, name, term.Reset)
	s, err := renderMessages(app.Sess)
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	fmt.Print("\f\n")                   // 清屏
	fmt.Printf("%s%s\n", s, term.Reset) // 重新打印会话内容
	return false
}

// Undo 处理 /undo：撤销最后一条消息。
func Undo(app *appstate.App) bool {
	if _, err := app.Sess.Undo(); err != nil { // 先不处理undo前user的content
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	fmt.Printf("%sUndo Complete!%s\n", term.Green, term.Reset)
	return false
}

// DebugShowSession 处理 /debugshsess：调试用，打印会话消息摘要。
func DebugShowSession(app *appstate.App) bool {
	res, err := app.Sess.GetMessageString()
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	fmt.Printf("%sSession:%s\n%+v\n", term.Magenta, term.Reset, res)
	return false
}
