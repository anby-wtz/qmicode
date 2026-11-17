// Package gui implements the minimal native desktop frontend. All widget and
// window state is owned by the Fyne UI goroutine; the controller runs serially
// on background workers.
package gui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"qmicode-go/app/config"
	"qmicode-go/codelib/agent"
	"qmicode-go/gui/controller"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

type desktopUI struct {
	app            fyne.App
	window         fyne.Window
	settingsWindow fyne.Window
	controller     *controller.Controller
	cancel         context.CancelFunc
	busy           bool
	closing        bool
	guiSettings    guiSettings

	info              *widget.Label
	status            *widget.Label
	input             *widget.Entry
	send              *widget.Button
	stop              *widget.Button
	save              *widget.Button
	newSession        *widget.Button
	retry             *widget.Button
	agentButton       *widget.Button
	modelButton       *widget.Button
	loadSessionButton *widget.Button
	safetyButton      *widget.Button
	appearanceButton  *widget.Button
	autoScroll        *widget.Check
	history           *fyne.Container
	scroll            *container.Scroll

	confirmation    *fyne.Container
	confirmText     *widget.Label
	confirmTitle    *widget.Label
	allowToolButton *widget.Button
	denyToolButton  *widget.Button
	pending         *pendingTool
	response        *responseView

	// statusSource keeps the untranslated status so a language switch can
	// re-render the current status instead of guessing a readiness message.
	statusSource statusSource

	// historyPendingRedraw is set when a language switch happened while a turn
	// was streaming; the history is rebuilt once the turn settles instead of
	// discarding the live response view.
	historyPendingRedraw bool
}

// statusSource records where the status line came from. localized marks text
// that lower layers already rendered (controller callbacks), which must go
// through the message rules rather than the string table.
type statusSource struct {
	text      string
	args      []any
	localized bool
	set       bool
}

type pendingTool struct {
	request controller.ToolRequest
	reply   chan bool
}

// setStatus shows a translatable status and remembers its source so a language
// switch can re-render it.
func (g *desktopUI) setStatus(format string, args ...any) {
	g.statusSource = statusSource{text: format, args: args, set: true}
	g.status.SetText(g.trf(format, args...))
}

// setLocalizedStatus shows a status that lower layers already rendered.
func (g *desktopUI) setLocalizedStatus(status string) {
	g.statusSource = statusSource{text: status, localized: true, set: true}
	g.status.SetText(g.localizeMessage(status))
}

// redrawStatus re-renders the last status in the current language.
func (g *desktopUI) redrawStatus() {
	source := g.statusSource
	switch {
	case !source.set:
		g.status.SetText(g.tr("初始化中"))
	case source.localized:
		g.status.SetText(g.localizeMessage(source.text))
	default:
		g.status.SetText(g.trf(source.text, source.args...))
	}
}

// Start must be called from main, so Fyne can own the platform event loop.
func Start() {
	a := app.NewWithID("io.qmitek.qmicode")
	settings, err := loadGUISettings()
	if err != nil {
		fyne.LogError("加载 GUI 字体设置失败，使用默认设置", err)
	}
	appearance, err := buildAppearanceTheme(settings)
	if err != nil {
		fyne.LogError("加载自定义 GUI 字体失败，使用默认字体", err)
		settings.FontFamily = ""
		appearance, _ = buildAppearanceTheme(settings)
	}
	a.Settings().SetTheme(appearance)

	g := &desktopUI{
		app:         a,
		window:      a.NewWindow("QmiCode"),
		guiSettings: settings,
	}
	g.build()
	g.window.Resize(fyne.NewSize(1000, 760))
	g.window.SetCloseIntercept(g.requestClose)
	g.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyReturn, Modifier: fyne.KeyModifierControl,
	}, func(fyne.Shortcut) { g.sendMessage() })
	g.window.Canvas().AddShortcut(&desktop.CustomShortcut{
		KeyName: fyne.KeyS, Modifier: fyne.KeyModifierControl,
	}, func(fyne.Shortcut) { g.saveSession() })
	g.initialize()
	g.window.ShowAndRun()
}

func (g *desktopUI) build() {
	g.info = widget.NewLabel(g.tr("Agent · —    模型 · —    用量 · 0 tokens"))
	g.info.SizeName = theme.SizeNameCaptionText
	g.status = widget.NewLabel(g.tr("初始化中"))
	g.status.Alignment = fyne.TextAlignTrailing
	g.history = container.NewVBox()
	g.scroll = container.NewVScroll(g.history)
	g.autoScroll = widget.NewCheck(g.tr("自动滚动"), nil)
	g.autoScroll.SetChecked(true)

	g.input = widget.NewMultiLineEntry()
	g.input.SetPlaceHolder(g.tr("输入消息…"))
	g.input.Wrapping = fyne.TextWrapWord
	g.input.SetMinRowsVisible(3)
	g.send = widget.NewButtonWithIcon(g.tr("发送"), theme.MailSendIcon(), g.sendMessage)
	g.send.Importance = widget.HighImportance
	g.stop = widget.NewButtonWithIcon(g.tr("停止"), theme.MediaStopIcon(), g.stopTask)
	g.save = widget.NewButtonWithIcon(g.tr("保存"), theme.DocumentSaveIcon(), g.saveSession)
	g.newSession = widget.NewButtonWithIcon(g.tr("新会话"), theme.ContentAddIcon(), g.newConversation)
	g.retry = widget.NewButtonWithIcon(g.tr("重试加载"), theme.ViewRefreshIcon(), g.initialize)
	g.retry.Hide()
	g.agentButton = widget.NewButton(g.tr("Agent"), g.chooseAgent)
	g.modelButton = widget.NewButton(g.tr("模型"), g.chooseModel)
	g.loadSessionButton = widget.NewButton(g.tr("会话"), g.chooseSession)
	g.safetyButton = widget.NewButton(g.tr("安全"), g.openSafetySettings)
	g.appearanceButton = widget.NewButton(g.tr("外观"), g.openSettings)

	g.confirmText = widget.NewLabel("")
	g.confirmText.Wrapping = fyne.TextWrapWord
	confirmScroll := container.NewVScroll(g.confirmText)
	confirmScroll.SetMinSize(fyne.NewSize(0, 130))
	g.allowToolButton = widget.NewButton(g.tr("允许本次执行"), func() { g.answerTool(true) })
	g.allowToolButton.Importance = widget.HighImportance
	g.denyToolButton = widget.NewButton(g.tr("拒绝本次执行"), func() { g.answerTool(false) })
	g.confirmTitle = widget.NewLabelWithStyle(g.tr("工具调用确认"), fyne.TextAlignLeading, fyne.TextStyle{Bold: true})
	g.confirmation = container.NewVBox(
		widget.NewSeparator(),
		g.confirmTitle,
		confirmScroll,
		container.NewHBox(g.allowToolButton, g.denyToolButton),
	)
	g.confirmation.Hide()

	settingsRow := container.NewGridWithColumns(5,
		g.agentButton,
		g.modelButton,
		g.loadSessionButton,
		g.safetyButton,
		g.appearanceButton,
	)
	actionsRow := container.NewBorder(
		nil,
		nil,
		g.autoScroll,
		container.NewHBox(g.newSession, g.save, g.stop, g.send),
		nil,
	)
	footer := container.NewVBox(
		settingsRow,
		actionsRow,
		widget.NewSeparator(),
		container.NewBorder(nil, nil, g.info, container.NewHBox(g.retry, g.status), nil),
	)
	composer := widget.NewCard("", "", container.NewVBox(g.confirmation, g.input, footer))
	g.window.SetContent(container.NewBorder(nil, composer, nil, nil, g.scroll))
}

func (g *desktopUI) setBusy(busy bool) {
	g.busy = busy
	if !busy && g.historyPendingRedraw {
		g.historyPendingRedraw = false
		if g.controller != nil {
			g.displayHistory(g.controller.History())
		}
	}
	g.input.Disable()
	g.send.Disable()
	g.save.Disable()
	g.newSession.Disable()
	g.retry.Disable()
	g.agentButton.Disable()
	g.modelButton.Disable()
	g.loadSessionButton.Disable()
	g.safetyButton.Disable()
	g.appearanceButton.Disable()
	g.stop.Disable()
	if busy {
		if g.cancel != nil && !g.closing {
			g.stop.Enable()
		}
		return
	}
	if g.closing {
		return
	}
	if g.controller == nil {
		g.retry.Show()
		g.retry.Enable()
		return
	}
	g.retry.Hide()
	if g.controller.Info().HasModel {
		g.input.Enable()
		g.send.Enable()
	} else {
		g.input.Disable()
		g.send.Disable()
	}
	g.save.Enable()
	g.newSession.Enable()
	g.agentButton.Enable()
	g.modelButton.Enable()
	g.loadSessionButton.Enable()
	g.safetyButton.Enable()
	g.appearanceButton.Enable()
}

func (g *desktopUI) initialize() {
	if g.busy || g.closing || g.controller != nil {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel
	g.setBusy(true)
	g.setStatus("正在加载配置…")
	go func() {
		defer cancel()
		c, err := controller.New(ctx, config.DefaultConfig, func(status string) {
			fyne.Do(func() {
				if !g.closing && ctx.Err() == nil {
					g.setLocalizedStatus(status)
				}
			})
		})
		var info controller.Info
		if c != nil {
			info = c.Info()
		}
		fyne.Do(func() {
			g.cancel = nil
			g.controller = c
			g.setBusy(false)
			if g.closing {
				g.beginClose()
				return
			}
			if err != nil {
				g.setStatus("加载失败；修正配置后点击“重新加载配置”")
				g.addMessage(g.tr("加载失败"), g.localizeError(err).Error())
				return
			}
			g.showInfo(info)
			if !info.HasModel {
				g.setStatus("请先配置模型；点击“切换模型”添加或选择模型")
			} else if info.NeedConfirm {
				g.setStatus("就绪 · 工具调用需确认")
			} else {
				g.setStatus("就绪 · 工具调用自动执行")
			}
			g.window.Canvas().Focus(g.input)
		})
	}()
}

func (g *desktopUI) showInfo(info controller.Info) {
	modelName := info.ModelName
	if !info.HasModel {
		modelName = g.tr("未配置")
	}
	g.info.SetText(fmt.Sprintf(g.tr("Agent · %s    模型 · %s    用量 · %d tokens"), info.AgentName, modelName, info.Usage))
}

func (g *desktopUI) runManagement(status, success string, reloadHistory bool, operation func(context.Context, *controller.Controller) error) {
	if g.busy || g.closing || g.controller == nil {
		return
	}
	c := g.controller
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel
	g.setBusy(true)
	g.setStatus(status)
	go func() {
		err := operation(ctx, c)
		info := c.Info()
		var history []controller.HistoryItem
		if reloadHistory {
			history = c.History()
		}
		fyne.Do(func() {
			cancel()
			g.cancel = nil
			g.showInfo(info)
			g.setBusy(false)
			if err != nil {
				g.setStatus("操作失败")
				if !g.closing {
					g.showError(err, g.window)
				}
			} else {
				g.setStatus(success)
				if reloadHistory {
					g.displayHistory(history)
				}
			}
			if g.closing {
				g.beginClose()
			}
		})
	}()
}

func (g *desktopUI) chooseAgent() {
	if g.busy || g.controller == nil {
		return
	}
	c := g.controller
	go func() {
		names, err := c.ListAgents()
		fyne.Do(func() {
			if err != nil {
				g.showError(err, g.window)
				return
			}
			if len(names) == 0 {
				dialog.ShowInformation(g.tr("没有 Agent"), g.tr("配置目录中没有可用的 Agent。"), g.window)
				return
			}
			selection := widget.NewSelect(names, nil)
			current := c.Info().AgentName
			selection.SetSelected(current)
			dialog.ShowForm(g.tr("切换 Agent"), g.tr("应用"), g.tr("取消"), []*widget.FormItem{
				widget.NewFormItem(g.tr("Agent"), selection),
			}, func(ok bool) {
				if ok && selection.Selected != "" && selection.Selected != current {
					name := selection.Selected
					g.runManagement("正在切换 Agent 并连接其 MCP 工具…", "Agent 已切换", false,
						func(ctx context.Context, c *controller.Controller) error {
							return c.SwitchAgent(ctx, name, func(status string) {
								fyne.Do(func() { g.setLocalizedStatus(status) })
							})
						})
				}
			}, g.window)
		})
	}()
}

func (g *desktopUI) chooseModel() {
	if g.busy || g.controller == nil {
		return
	}
	c := g.controller
	go func() {
		names, err := c.ListModels()
		fyne.Do(func() {
			if err != nil {
				g.showError(err, g.window)
				return
			}
			if len(names) == 0 {
				g.createModel()
				return
			}
			selection := widget.NewSelect(names, nil)
			current := c.Info().ModelName
			selection.SetSelected(current)
			var chooser *dialog.FormDialog
			create := widget.NewButton(g.tr("添加 OpenAI 兼容模型…"), func() {
				chooser.Hide()
				g.createModel()
			})
			chooser = dialog.NewForm(g.tr("切换模型"), g.tr("应用并设为默认"), g.tr("取消"), []*widget.FormItem{
				widget.NewFormItem(g.tr("模型配置"), selection),
				widget.NewFormItem(g.tr("新建"), create),
			}, func(ok bool) {
				if ok && selection.Selected != "" && selection.Selected != current {
					name := selection.Selected
					g.runManagement("正在切换模型…", "模型已切换并设为默认", false,
						func(_ context.Context, c *controller.Controller) error { return c.SwitchModel(name) })
				}
			}, g.window)
			chooser.Show()
		})
	}()
}

func (g *desktopUI) createModel() {
	if g.busy || g.closing || g.controller == nil {
		return
	}
	name := widget.NewEntry()
	name.SetPlaceHolder(g.tr("例如：local-qwen"))
	baseURL := widget.NewEntry()
	baseURL.SetPlaceHolder("https://api.openai.com/v1")
	apiKey := widget.NewPasswordEntry()
	apiKey.SetPlaceHolder(g.tr("可留空，配置文件会以明文保存"))
	modelID := widget.NewEntry()
	modelID.SetPlaceHolder(g.tr("例如：gpt-4o-mini"))
	for _, item := range []struct {
		entry   *widget.Entry
		message string
	}{
		{name, "配置名称不能为空"},
		{baseURL, "Base URL 不能为空"},
		{modelID, "模型 ID 不能为空"},
	} {
		message := item.message
		item.entry.Validator = func(value string) error {
			if strings.TrimSpace(value) == "" {
				return errors.New(g.tr(message))
			}
			return nil
		}
	}
	dialog.ShowForm(g.tr("添加模型配置"), g.tr("保存并切换"), g.tr("取消"), []*widget.FormItem{
		widget.NewFormItem(g.tr("名称"), name),
		widget.NewFormItem(g.tr("Base URL"), baseURL),
		widget.NewFormItem(g.tr("API Key"), apiKey),
		widget.NewFormItem(g.tr("模型 ID"), modelID),
	}, func(ok bool) {
		if !ok {
			return
		}
		nameValue, baseURLValue := name.Text, baseURL.Text
		apiKeyValue, modelValue := apiKey.Text, modelID.Text
		g.runManagement("正在保存并切换模型…", "模型配置已保存并设为默认", false,
			func(_ context.Context, c *controller.Controller) error {
				return c.CreateModel(nameValue, baseURLValue, apiKeyValue, modelValue)
			})
	}, g.window)
}

func (g *desktopUI) chooseSession() {
	if g.busy || g.controller == nil {
		return
	}
	c := g.controller
	go func() {
		names, err := c.ListSessions()
		fyne.Do(func() {
			if err != nil {
				g.showError(err, g.window)
				return
			}
			if len(names) == 0 {
				dialog.ShowInformation(g.tr("没有已保存会话"), g.tr("发送一条消息后，会话会自动保存到配置目录。"), g.window)
				return
			}
			selection := widget.NewSelect(names, nil)
			current := c.Info().SessionName
			selection.SetSelected(current)
			dialog.ShowForm(g.tr("加载会话"), g.tr("加载"), g.tr("取消"), []*widget.FormItem{
				widget.NewFormItem(g.tr("已保存会话"), selection),
			}, func(ok bool) {
				if ok && selection.Selected != "" && selection.Selected != current {
					name := selection.Selected
					g.runManagement("正在保存当前会话并加载所选会话…", "会话已加载", true,
						func(_ context.Context, c *controller.Controller) error { return c.LoadSession(name) })
				}
			}, g.window)
		})
	}()
}

func (g *desktopUI) openSafetySettings() {
	if g.busy || g.controller == nil {
		return
	}
	needConfirm := g.controller.Info().NeedConfirm
	confirmCheck := widget.NewCheck(g.tr("每次执行工具调用前都要求确认"), nil)
	confirmCheck.SetChecked(needConfirm)
	warning := widget.NewLabel(g.tr("工具会在当前真实系统环境中运行；目前没有沙箱。关闭确认后，模型可能自动执行工具。"))
	warning.Wrapping = fyne.TextWrapWord
	dialog.ShowForm(g.tr("安全配置"), g.tr("保存"), g.tr("取消"), []*widget.FormItem{
		widget.NewFormItem(g.tr("工具执行"), confirmCheck),
		widget.NewFormItem(g.tr("运行环境"), warning),
	}, func(ok bool) {
		if ok && confirmCheck.Checked != needConfirm {
			value := confirmCheck.Checked
			g.runManagement("正在保存安全配置…", "安全配置已保存", false,
				func(_ context.Context, c *controller.Controller) error { return c.SetNeedConfirm(value) })
		}
	}, g.window)
}

func (g *desktopUI) displayHistory(items []controller.HistoryItem) {
	g.history.RemoveAll()
	g.response = nil
	for _, item := range items {
		title := g.tr("助手")
		switch item.Role {
		case "user":
			title = g.tr("你")
		case "tool":
			title = g.tr("工具结果")
		case "compact":
			title = g.tr("上下文摘要")
		}
		text := item.Text
		if item.AttachmentCount > 0 {
			if text != "" {
				text += "\n"
			}
			text += g.trf("（包含 %d 个附件）", item.AttachmentCount)
		}
		if len(item.ToolCallNames) > 0 {
			if text != "" {
				text += "\n"
			}
			text += g.trf("工具调用：%s", strings.Join(item.ToolCallNames, g.listSeparator()))
		}
		if text == "" && item.Reasoning != "" {
			text = g.tr("（此消息只有 reasoning 内容）")
		}
		_, card := g.addMessage(title, text)
		if item.Reasoning != "" {
			reasoning := wrappedLabel(item.Reasoning)
			card.Add(widget.NewCard("", "", reasoning))
		}
	}
	g.followOutput()
}

func (g *desktopUI) sendMessage() {
	if g.busy || g.closing || g.controller == nil {
		return
	}
	if !g.controller.Info().HasModel {
		g.setStatus("请先点击“切换模型”配置模型")
		return
	}
	prompt := g.input.Text
	if strings.TrimSpace(prompt) == "" {
		return
	}
	g.input.SetText("")
	g.addMessage(g.tr("你"), prompt)
	ctx, cancel := context.WithCancel(context.Background())
	g.cancel = cancel
	g.setBusy(true)
	g.setStatus("正在请求模型…")
	c := g.controller
	callbacks := g.callbacks(ctx)
	go func() {
		defer cancel()
		err := c.Run(ctx, prompt, callbacks)
		info := c.Info()
		fyne.Do(func() {
			g.cancel = nil
			g.clearConfirmation()
			g.showInfo(info)
			g.setBusy(false)
			var saveErr *controller.SaveError
			switch {
			case err == nil:
				g.setStatus("本轮完成 · 已保存")
			case errors.As(err, &saveErr):
				g.addMessage(g.tr("会话保存失败"), g.localizeError(err).Error())
				g.setStatus("保存失败；请点击“保存”重试")
			case errors.Is(err, agent.ErrInterrupted):
				g.addMessage(g.tr("已停止"), g.tr("本轮已取消；会话已按现有 Runtime 的结果保存。"))
				g.setStatus("已停止 · 已保存")
			default:
				g.addMessage(g.tr("本轮出错"), g.localizeError(err).Error()+"\n"+g.tr("会话已按现有 Runtime 的结果保存。"))
				g.setStatus("请求出错 · 会话已保存")
			}
			if g.closing {
				g.beginClose()
				return
			}
			g.window.Canvas().Focus(g.input)
		})
	}()
}

func (g *desktopUI) stopTask() {
	if g.cancel != nil {
		g.cancel()
		g.stop.Disable()
		g.clearConfirmation()
		g.setStatus("正在停止并保存，请等待当前任务收尾…")
	}
}

func (g *desktopUI) saveSession() {
	g.sessionOperation(false)
}

func (g *desktopUI) newConversation() {
	g.sessionOperation(true)
}

func (g *desktopUI) sessionOperation(newSession bool) {
	if g.busy || g.closing || g.controller == nil {
		return
	}
	g.setBusy(true)
	g.setStatus("正在保存…")
	c := g.controller
	go func() {
		var err error
		if newSession {
			err = c.NewSession()
		} else {
			err = c.Save()
		}
		info := c.Info()
		fyne.Do(func() {
			g.setBusy(false)
			if err != nil {
				g.addMessage(g.tr("保存失败"), g.localizeError(err).Error())
				g.setStatus("保存失败；可点击“保存”重试")
			} else {
				if newSession {
					g.history.RemoveAll()
					g.response = nil
				}
				g.showInfo(info)
				if newSession {
					g.setStatus("新会话已创建 · 上一会话已保存")
				} else {
					g.setStatus("已保存")
				}
			}
			if g.closing {
				g.beginClose()
			}
		})
	}()
}

func (g *desktopUI) requestClose() {
	if g.closing {
		return
	}
	g.closing = true
	if g.busy {
		g.stopTask()
		g.setBusy(true)
		g.setStatus("正在取消任务并保存，完成后退出…")
		return
	}
	g.beginClose()
}

func (g *desktopUI) beginClose() {
	if g.controller == nil {
		g.releaseAndClose()
		return
	}
	g.setBusy(true)
	g.setStatus("正在保存会话…")
	c := g.controller
	go func() {
		err := c.Save()
		fyne.Do(func() {
			if err != nil {
				g.closing = false
				g.setBusy(false)
				g.setStatus("退出前保存失败")
				g.addMessage(g.tr("保存失败"), g.localizeError(err).Error())
				dialog.ShowConfirm(g.tr("保存失败"), g.localizeError(err).Error()+"\n\n"+g.tr("仍要退出吗？未保存内容可能丢失。"), func(exit bool) {
					if exit && !g.busy {
						g.closing = true
						g.releaseAndClose()
					}
				}, g.window)
				return
			}
			g.releaseAndClose()
		})
	}()
}

func (g *desktopUI) releaseAndClose() {
	g.setBusy(true)
	g.setStatus("正在清理 GUI 子进程并关闭资源…")
	c := g.controller
	go func() {
		// Exit cleanup belongs to the GUI. The shared tool/runtime process
		// management is used as-is and is not replaced or modified.
		if err := killChildProcessTree(); err != nil {
			fyne.LogError("GUI 退出时清理子进程失败", err)
		}
		if c != nil {
			if err := c.Close(); err != nil {
				fyne.LogError("关闭 MCP 连接失败", err)
			}
		}
		fyne.Do(g.closeWindow)
	}()
}

func (g *desktopUI) closeWindow() {
	if g.settingsWindow != nil {
		settingsWindow := g.settingsWindow
		g.settingsWindow = nil
		settingsWindow.Close()
	}
	g.window.SetCloseIntercept(nil)
	g.window.Close()
}
