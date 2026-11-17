package gui

import (
	"errors"
	"fmt"
	"regexp"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
)

const (
	languageChinese = "zh-CN"
	languageEnglish = "en-US"

	// Templates containing line breaks are shared so the map key and the call
	// site cannot drift apart.
	keyToolCallDetail = "工具：%s\n调用 ID：%s\n参数：\n%s"
	keyToolCallTip    = "\n\n执行提示：\n%s"
)

// englishText maps every user-visible Chinese string of the GUI to English.
// Chinese is the key so call sites stay readable and a missing entry degrades
// to the original text instead of showing an empty label.
var englishText = map[string]string{
	// 顶部信息与状态
	"Agent · —    模型 · —    用量 · 0 tokens":    "Agent · —    Model · —    Usage · 0 tokens",
	"Agent · %s    模型 · %s    用量 · %d tokens": "Agent · %s    Model · %s    Usage · %d tokens",
	"未配置":     "Not configured",
	"初始化中":    "Initializing",
	"正在加载配置…": "Loading configuration…",
	"加载失败":    "Loading failed",
	"加载失败；修正配置后点击“重新加载配置”":   "Loading failed. Fix the configuration and select Retry loading.",
	"请先配置模型；点击“切换模型”添加或选择模型": "Configure a model first using Model.",
	"请先点击“切换模型”配置模型":         "Configure a model using Model first.",
	"就绪 · 工具调用需确认":           "Ready · tool calls require confirmation",
	"就绪 · 工具调用自动执行":          "Ready · tool calls execute automatically",
	"操作失败":                   "Operation failed",

	// 主窗口控件
	"自动滚动":  "Auto-scroll",
	"输入消息…": "Type a message…",
	"发送":    "Send",
	"停止":    "Stop",
	"保存":    "Save",
	"新会话":   "New chat",
	"重试加载":  "Retry loading",
	"Agent": "Agent",
	"模型":    "Model",
	"会话":    "Sessions",
	"安全":    "Safety",
	"外观":    "Appearance",
	"应用":    "Apply",
	"取消":    "Cancel",
	"加载":    "Load",
	"新建":    "New",
	"名称":    "Name",

	// 工具调用确认
	"工具调用确认":          "Tool confirmation",
	"允许本次执行":          "Allow this call",
	"拒绝本次执行":          "Deny this call",
	keyToolCallDetail: "Tool: %s\nCall ID: %s\nArguments:\n%s",
	keyToolCallTip:    "\n\nTip:\n%s",
	"等待工具确认；也可以点击“停止”取消本轮": "Waiting for tool confirmation. Select Stop to cancel the turn.",
	"已允许执行": "Execution allowed",
	"已拒绝执行": "Execution denied",
	"已拒绝本次工具调用，模型将继续处理…": "Tool call denied; the model will continue…",
	"正在执行工具：%s":          "Running tool: %s",

	// 对话
	"你":     "You",
	"助手":    "Assistant",
	"工具结果":  "Tool result",
	"上下文摘要": "Context summary",
	"复制":    "Copy",
	"正在生成…": "Generating…",
	"（本次响应无正文，可能包含工具调用。）":     "(This response has no text and may contain tool calls.)",
	"（此消息只有 reasoning 内容）":    "(This message contains reasoning only.)",
	"工具出错 · %s":               "Tool failed · %s",
	"工具结果 · %s":               "Tool result · %s",
	"工具调用 · %s":               "Tool call · %s",
	"工具未提供展示文本；模型收到的结果会写入会话。": "The tool returned no display text; its result was added to the conversation.",
	"（包含 %d 个附件）":             "(%d attachments)",
	"工具调用：%s":                 "Tool calls: %s",
	"正在请求模型…":                 "Requesting model…",
	"本轮完成 · 已保存":              "Turn complete · saved",
	"本轮出错":                    "Turn failed",
	"请求出错 · 会话已保存":            "Request failed · session saved",
	"会话保存失败":                  "Session save failed",
	"保存失败":                    "Save failed",
	"保存失败；请点击“保存”重试":          "Save failed. Select Save to retry.",
	"保存失败；可点击“保存”重试":          "Save failed. Select Save to retry.",
	"已停止":                     "Stopped",
	"已停止 · 已保存":               "Stopped · saved",
	"本轮已取消；会话已按现有 Runtime 的结果保存。": "The turn was cancelled; the session was saved with the current runtime result.",
	"会话已按现有 Runtime 的结果保存。":       "The session was saved with the current runtime result.",

	// 保存与退出流程
	"正在保存…":   "Saving…",
	"正在保存会话…": "Saving session…",
	"已保存":     "Saved",
	"新会话已创建 · 上一会话已保存":   "New session created · previous session saved",
	"正在停止并保存，请等待当前任务收尾…": "Stopping and saving; waiting for the current task to finish…",
	"正在取消任务并保存，完成后退出…":   "Cancelling and saving; the app will close when finished…",
	"正在清理 GUI 子进程并关闭资源…": "Cleaning up GUI child processes and closing resources…",
	"退出前保存失败":            "Could not save before exit",
	"仍要退出吗？未保存内容可能丢失。":   "Exit anyway? Unsaved changes may be lost.",

	// Agent
	"没有 Agent": "No agents found",
	"配置目录中没有可用的 Agent。": "No available agents were found in the configuration directory.",
	"切换 Agent": "Switch agent",
	"正在切换 Agent 并连接其 MCP 工具…": "Switching agent and connecting its MCP tools…",
	"Agent 已切换": "Agent switched",

	// 模型
	"切换模型":            "Switch model",
	"应用并设为默认":         "Apply and set as default",
	"模型配置":            "Model configuration",
	"添加 OpenAI 兼容模型…": "Add OpenAI-compatible model…",
	"正在切换模型…":         "Switching model…",
	"模型已切换并设为默认":      "Model switched and set as default",
	"添加模型配置":          "Add model configuration",
	"保存并切换":           "Save and switch",
	"正在保存并切换模型…":      "Saving and switching model…",
	"模型配置已保存并设为默认":    "Model configuration saved and set as default",
	"Base URL":        "Base URL",
	"模型 ID":           "Model ID",
	"API Key":         "API Key",
	"例如：local-qwen":   "e.g. local-qwen",
	"例如：gpt-4o-mini":  "e.g. gpt-4o-mini",
	"可留空，配置文件会以明文保存":  "Optional; the key is stored as plain text in the configuration file.",
	"配置名称不能为空":        "Configuration name is required",
	"Base URL 不能为空":   "Base URL is required",
	"模型 ID 不能为空":      "Model ID is required",

	// 会话
	"没有已保存会话": "No saved sessions",
	"发送一条消息后，会话会自动保存到配置目录。": "The session is saved in the configuration directory after you send a message.",
	"加载会话":  "Load session",
	"已保存会话": "Saved session",
	"正在保存当前会话并加载所选会话…": "Saving the current session and loading the selected session…",
	"会话已加载": "Session loaded",

	// 安全
	"安全配置": "Safety settings",
	"工具执行": "Tool execution",
	"运行环境": "Execution environment",
	"每次执行工具调用前都要求确认": "Require confirmation before each tool call",
	"工具会在当前真实系统环境中运行；目前没有沙箱。关闭确认后，模型可能自动执行工具。": "Tools run in the real system environment; no sandbox is available. If confirmation is disabled, the model may execute tools automatically.",
	"正在保存安全配置…": "Saving safety settings…",
	"安全配置已保存":   "Safety settings saved",

	// 外观设置
	"GUI 设置":    "GUI settings",
	"系统字体":      "System font",
	"系统默认字体":    "System default",
	"字号（10–32）": "Font size (10–32)",
	"背景主题":      "Color theme",
	"界面语言":      "Interface language",
	"浅色":        "Light",
	"深色":        "Dark",
	"跟随系统":      "Follow system",
	"正在读取系统已安装字体列表…":               "Loading installed system fonts…",
	"读取字体列表失败：%s":                  "Could not read the font list: %s",
	"此前保存的字体当前不可用，已改选系统默认字体。":      "The previously selected font is unavailable; using the system default.",
	"找到 %d 个可用的系统字体":               "Found %d available system fonts",
	"选择系统已安装字体；设置会立即应用并保存供下次启动使用。": "Choose an installed system font. Changes apply immediately and are saved for the next launch.",
	"保存并应用":        "Save and apply",
	"GUI 设置已保存并应用": "GUI settings saved and applied",
}

// messageRule rewrites a Chinese message produced outside the UI goroutine
// (controller status callbacks and wrapped errors) into English. A pattern must
// match the whole message; ${1}, ${2} … refer to its capture groups.
type messageRule struct {
	pattern     *regexp.Regexp
	replacement string
}

// messageRules is ordered: more specific patterns must appear before the
// generic ones they would otherwise be swallowed by.
var messageRules = []messageRule{
	{regexp.MustCompile(`(?s)^加载 Agent: (.*)；尚未安装时请在项目目录执行 qmicode install\z`),
		"Could not load the agent: ${1}. If it is not installed, run qmicode install in the project directory."},
	{regexp.MustCompile(`(?s)^加载 Agent: (.*)\z`), "Could not load the agent: ${1}"},
	{regexp.MustCompile(`(?s)^初始化配置目录: (.*)\z`), "Could not initialize the configuration directory: ${1}"},
	{regexp.MustCompile(`(?s)^检查 MCP 配置: (.*)\z`), "Could not check the MCP configuration: ${1}"},
	{regexp.MustCompile(`(?s)^读取 MCP 配置: (.*)\z`), "Could not read the MCP configuration: ${1}"},
	{regexp.MustCompile(`(?s)^连接 MCP (.*?): (.*)\z`), "Could not connect to MCP ${1}: ${2}"},
	{regexp.MustCompile(`(?s)^获取 MCP (.*?) 工具: (.*)\z`), "Could not get tools from MCP ${1}: ${2}"},
	{regexp.MustCompile(`(?s)^注册 MCP (.*?) 工具: (.*)\z`), "Could not register tools from MCP ${1}: ${2}"},
	{regexp.MustCompile(`(?s)^Agent 已切换，但关闭旧 MCP 连接失败：(.*)\z`),
		"Agent switched, but the previous MCP connections could not be closed: ${1}"},
	{regexp.MustCompile(`(?s)^Agent 名称不能为空\z`), "Agent name cannot be empty"},
	{regexp.MustCompile(`(?s)^加载 Agent (".*?")：(.*)\z`), "Could not load agent ${1}: ${2}"},
	{regexp.MustCompile(`(?s)^模型名称不能为空\z`), "Model name cannot be empty"},
	{regexp.MustCompile(`(?s)^未找到模型配置 (".*?")\z`), "Model configuration not found: ${1}"},
	{regexp.MustCompile(`(?s)^加载模型配置 (".*?")：(.*)\z`), "Could not load model configuration ${1}: ${2}"},
	{regexp.MustCompile(`(?s)^模型配置 (".*?") 的 model 为空\z`), "Model configuration ${1} has an empty model field"},
	{regexp.MustCompile(`(?s)^保存默认模型失败：(.*)\z`), "Could not save the default model: ${1}"},
	{regexp.MustCompile(`(?s)^模型配置名称无效\z`), "Invalid model configuration name"},
	{regexp.MustCompile(`(?s)^Base URL 和模型 ID 不能为空\z`), "Base URL and model ID cannot be empty"},
	{regexp.MustCompile(`(?s)^模型配置 (".*?") 已存在\z`), "Model configuration ${1} already exists"},
	{regexp.MustCompile(`(?s)^保存模型配置：(.*)\z`), "Could not save the model configuration: ${1}"},
	{regexp.MustCompile(`(?s)^模型已保存，但设为默认失败：(.*)\z`),
		"The model was saved, but it could not be set as the default: ${1}"},
	{regexp.MustCompile(`(?s)^会话名称不能为空\z`), "Session name cannot be empty"},
	{regexp.MustCompile(`(?s)^未找到会话 (".*?")\z`), "Session not found: ${1}"},
	{regexp.MustCompile(`(?s)^加载会话 (".*?")：(.*)\z`), "Could not load session ${1}: ${2}"},
	{regexp.MustCompile(`(?s)^保存安全配置失败：(.*)\z`), "Could not save the safety settings: ${1}"},
	{regexp.MustCompile(`(?s)^保存会话失败（(.*)）: (.*)\z`), "Could not save the session (${1}): ${2}"},
	{regexp.MustCompile(`(?s)^尚未配置模型，请先添加或切换模型\z`), "No model is configured. Add or switch a model first."},
	{regexp.MustCompile(`(?s)^消息不能为空\z`), "The message cannot be empty"},
	{regexp.MustCompile(`(?s)^解析 GUI 设置失败：(.*)\z`), "Could not parse the GUI settings: ${1}"},
	{regexp.MustCompile(`(?s)^GUI 字号必须在 (.*?) 到 (.*?) 之间\z`), "The GUI font size must be between ${1} and ${2}"},
	{regexp.MustCompile(`(?s)^未知 GUI 主题模式 (".*?")\z`), "Unknown GUI theme mode ${1}"},
	{regexp.MustCompile(`(?s)^未知 GUI 语言 (".*?")\z`), "Unknown GUI language ${1}"},
	{regexp.MustCompile(`(?s)^读取系统字体列表失败：(.*)\z`), "Could not read the system font list: ${1}"},
	{regexp.MustCompile(`(?s)^系统中未找到字体 (".*?")\z`), "System font not found: ${1}"},
	{regexp.MustCompile(`(?s)^系统字体文件类型 (".*?") 目前不受支持\z`), "Unsupported system font file type ${1}"},
	{regexp.MustCompile(`(?s)^读取系统字体文件 (".*?") 失败：(.*)\z`),
		"Could not read the system font file ${1}: ${2}"},
	{regexp.MustCompile(`(?s)^保存 (.*?) 失败：(.*)\z`), "Could not save ${1}: ${2}"},
	{regexp.MustCompile(`(?s)^GUI 退出时读取进程树: (.*)\z`),
		"Could not read the process tree while the GUI exits: ${1}"},
	{regexp.MustCompile(`(?s)^GUI 退出时遍历进程树: (.*)\z`),
		"Could not walk the process tree while the GUI exits: ${1}"},
	{regexp.MustCompile(`(?s)^安全配置读取失败，已默认启用工具调用确认\z`),
		"Could not read the safety settings; tool confirmation is enabled by default"},
	{regexp.MustCompile(`(?s)^尚未配置有效模型，可在 GUI 中添加或切换模型\z`),
		"No valid model is configured. Add or switch a model in the GUI."},
	{regexp.MustCompile(`(?s)^正在加载 Agent：(.*)\z`), "Loading agent: ${1}"},
	{regexp.MustCompile(`(?s)^正在连接 MCP：(.*)\z`), "Connecting to MCP: ${1}"},
}

func (g *desktopUI) isEnglish() bool {
	return g.guiSettings.Language == languageEnglish
}

func (g *desktopUI) tr(text string) string {
	if !g.isEnglish() {
		return text
	}
	if translated, ok := englishText[text]; ok {
		return translated
	}
	return text
}

// trf translates a template and then fills in its verbs.
func (g *desktopUI) trf(format string, args ...any) string {
	return fmt.Sprintf(g.tr(format), args...)
}

// listSeparator joins names the way each language expects.
func (g *desktopUI) listSeparator() string {
	if g.isEnglish() {
		return ", "
	}
	return "、"
}

// localizeMessage translates text that the GUI receives already rendered, such
// as controller status updates and errors returned by lower layers.
func (g *desktopUI) localizeMessage(message string) string {
	if !g.isEnglish() || message == "" {
		return message
	}
	if translated, ok := englishText[message]; ok {
		return translated
	}
	for _, rule := range messageRules {
		if rule.pattern.MatchString(message) {
			return rule.pattern.ReplaceAllString(message, rule.replacement)
		}
	}
	return message
}

func (g *desktopUI) localizeError(err error) error {
	if err == nil {
		return nil
	}
	message := err.Error()
	translated := g.localizeMessage(message)
	if translated == message {
		return err
	}
	return errors.New(translated)
}

func (g *desktopUI) localizeControllerStatus(status string) string {
	return g.localizeMessage(status)
}

func (g *desktopUI) showError(err error, parent fyne.Window) {
	dialog.ShowError(g.localizeError(err), parent)
}

// applyLanguage re-renders every string the window owns. Views that hold
// rendered text (history, pending confirmation) are rebuilt from source data so
// no stale language survives a switch.
func (g *desktopUI) applyLanguage() {
	g.input.SetPlaceHolder(g.tr("输入消息…"))
	g.autoScroll.Text = g.tr("自动滚动")
	g.autoScroll.Refresh()
	g.send.SetText(g.tr("发送"))
	g.stop.SetText(g.tr("停止"))
	g.save.SetText(g.tr("保存"))
	g.newSession.SetText(g.tr("新会话"))
	g.retry.SetText(g.tr("重试加载"))
	g.agentButton.SetText(g.tr("Agent"))
	g.modelButton.SetText(g.tr("模型"))
	g.loadSessionButton.SetText(g.tr("会话"))
	g.safetyButton.SetText(g.tr("安全"))
	g.appearanceButton.SetText(g.tr("外观"))
	g.allowToolButton.SetText(g.tr("允许本次执行"))
	g.denyToolButton.SetText(g.tr("拒绝本次执行"))
	g.confirmTitle.SetText(g.tr("工具调用确认"))
	if g.pending != nil {
		g.confirmText.SetText(g.toolCallText(g.pending.request))
	}
	if g.settingsWindow != nil {
		g.settingsWindow.SetTitle(g.tr("GUI 设置"))
	}

	g.info.SetText(g.tr("Agent · —    模型 · —    用量 · 0 tokens"))
	if g.controller != nil {
		g.showInfo(g.controller.Info())
		// Rebuilding the history replaces the live response view, so a switch
		// during a turn is deferred until the turn settles.
		if g.busy {
			g.historyPendingRedraw = true
		} else {
			g.displayHistory(g.controller.History())
		}
	}
	g.redrawStatus()
}

func languageDisplay(language string) string {
	if language == languageEnglish {
		return "English"
	}
	return "简体中文"
}

func languageValue(display string) string {
	if display == "English" {
		return languageEnglish
	}
	return languageChinese
}
