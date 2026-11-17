package gui

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"time"

	"qmicode-go/codelib/agent"
	"qmicode-go/gui/controller"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type responseView struct {
	answer         *widget.Label
	reasoning      *widget.Label
	reasoningPanel *widget.Card
}

func wrappedLabel(text string) *widget.Label {
	label := widget.NewLabel(text)
	label.Wrapping = fyne.TextWrapWord
	return label
}

// addMessage is only called on the UI goroutine. Plain text rendering keeps the
// first version predictable for arbitrary command output; each card can be copied.
func (g *desktopUI) addMessage(title, text string) (*widget.Label, *fyne.Container) {
	label := wrappedLabel(text)
	copyButton := widget.NewButtonWithIcon(g.tr("复制"), theme.ContentCopyIcon(), func() {
		g.window.Clipboard().SetContent(label.Text)
	})
	heading := widget.NewLabelWithStyle(title, fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	heading.SizeName = theme.SizeNameSubHeadingText
	content := container.NewVBox(
		container.NewBorder(nil, nil, nil, copyButton, heading),
		label,
	)
	g.history.Add(widget.NewCard("", "", content))
	g.followOutput()
	return label, content
}

func (g *desktopUI) addAssistantResponse() *responseView {
	answer := wrappedLabel(g.tr("正在生成…"))
	reasoning := wrappedLabel("")
	reasoningPanel := widget.NewCard("", "", reasoning)
	reasoningPanel.Hide()
	heading := widget.NewLabelWithStyle(g.tr("助手"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	heading.SizeName = theme.SizeNameSubHeadingText
	copyButton := widget.NewButtonWithIcon(g.tr("复制"), theme.ContentCopyIcon(), func() {
		g.window.Clipboard().SetContent(answer.Text)
	})
	content := container.NewVBox(
		container.NewBorder(nil, nil, nil, copyButton, heading),
		reasoningPanel,
		answer,
	)
	g.history.Add(widget.NewCard("", "", content))
	g.followOutput()
	return &responseView{answer: answer, reasoning: reasoning, reasoningPanel: reasoningPanel}
}

func (g *desktopUI) followOutput() {
	if g.autoScroll.Checked {
		g.scroll.ScrollToBottom()
	}
}

// Each invocation belongs to one conversation worker. It batches chunks briefly
// to keep the UI responsive while still rendering answer and reasoning live.
func (g *desktopUI) callbacks(ctx context.Context) controller.Callbacks {
	var answer, reasoning strings.Builder
	var lastFlush time.Time
	var response *responseView // written and read only by Fyne's UI goroutine
	flush := func(final bool) {
		text, thoughts := answer.String(), reasoning.String()
		lastFlush = time.Now()
		fyne.Do(func() {
			view := response
			if view == nil {
				return
			}
			if text != "" {
				view.answer.SetText(text)
			} else if final {
				view.answer.SetText(g.tr("（本次响应无正文，可能包含工具调用。）"))
			}
			if thoughts != "" {
				view.reasoning.SetText(thoughts)
				view.reasoningPanel.Show()
			}
			g.followOutput()
		})
	}
	return controller.Callbacks{
		ResponseStart: func() {
			answer.Reset()
			reasoning.Reset()
			lastFlush = time.Time{}
			fyne.DoAndWait(func() {
				if !g.closing && ctx.Err() == nil {
					g.setStatus("正在生成…")
				}
				response = g.addAssistantResponse()
				g.response = response
			})
		},
		Chunk: func(chunk string, kind agent.LLMChunkContentType) {
			switch kind {
			case agent.ContentReasoning:
				reasoning.WriteString(chunk)
			case agent.ContentAnswer:
				answer.WriteString(chunk)
			}
			if time.Since(lastFlush) >= 40*time.Millisecond {
				flush(false)
			}
		},
		ResponseEnd: func() { flush(true) },
		Confirm:     g.confirmTool,
		ToolResult: func(request controller.ToolRequest, result string, err error) {
			fyne.Do(func() {
				if err != nil {
					g.addMessage(g.trf("工具出错 · %s", request.Name), result+"\n"+err.Error())
				} else {
					if result == "" {
						result = g.tr("工具未提供展示文本；模型收到的结果会写入会话。")
					}
					g.addMessage(g.trf("工具结果 · %s", request.Name), result)
				}
			})
		},
	}
}

// toolCallText renders the confirmation body. It is rebuilt from the request so
// applyLanguage can re-render it without keeping a translated copy around.
func (g *desktopUI) toolCallText(request controller.ToolRequest) string {
	arguments := request.Arguments
	var formatted bytes.Buffer
	if json.Indent(&formatted, []byte(arguments), "", "  ") == nil {
		arguments = formatted.String()
	}
	text := g.trf(keyToolCallDetail, request.Name, request.ID, arguments)
	if request.Message != "" {
		text += g.trf(keyToolCallTip, request.Message)
	}
	return text
}

func (g *desktopUI) confirmTool(ctx context.Context, request controller.ToolRequest) bool {
	reply := make(chan bool, 1)
	fyne.Do(func() {
		if ctx.Err() != nil || g.closing {
			return
		}
		text := g.toolCallText(request)
		g.addMessage(g.trf("工具调用 · %s", request.Name), text)
		g.pending = &pendingTool{request: request, reply: reply}
		g.confirmText.SetText(text)
		g.confirmation.Show()
		g.setStatus("等待工具确认；也可以点击“停止”取消本轮")
	})
	select {
	case <-ctx.Done():
		return false
	case allow := <-reply:
		return allow && ctx.Err() == nil
	}
}

func (g *desktopUI) answerTool(allow bool) {
	pending := g.pending
	if pending == nil {
		return
	}
	g.clearConfirmation()
	if allow {
		g.addMessage(g.tr("已允许执行"), pending.request.Name)
		g.setStatus("正在执行工具：%s", pending.request.Name)
	} else {
		g.addMessage(g.tr("已拒绝执行"), pending.request.Name)
		g.setStatus("已拒绝本次工具调用，模型将继续处理…")
	}
	// Never block the event loop, even if the worker has just been cancelled.
	select {
	case pending.reply <- allow:
	default:
	}
}

func (g *desktopUI) clearConfirmation() {
	g.pending = nil
	g.confirmation.Hide()
}
