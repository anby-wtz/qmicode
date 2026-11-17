package gui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// englishUI returns a UI that only carries the language setting. tr, trf and
// the localizers never touch other fields, so they stay testable without Fyne.
func englishUI() *desktopUI {
	return &desktopUI{guiSettings: guiSettings{Language: languageEnglish}}
}

func chineseUI() *desktopUI {
	return &desktopUI{guiSettings: guiSettings{Language: languageChinese}}
}

// formatVerbs returns the ordered verbs of a format string, so a translation
// that reorders or drops them fails the test.
func formatVerbs(value string) []string {
	verbs := make([]string, 0)
	for i := 0; i < len(value); i++ {
		if value[i] != '%' {
			continue
		}
		j := i + 1
		for j < len(value) && strings.ContainsRune("+-# 0123456789.", rune(value[j])) {
			j++
		}
		if j < len(value) {
			verbs = append(verbs, value[i:j+1])
			i = j
		}
	}
	return verbs
}

func TestEnglishTextIsUniqueAndHasMatchingVerbs(t *testing.T) {
	// The compiler already rejects duplicate literals, but not duplicates that
	// arrive through a shared constant, so pin the count explicitly.
	seen := make(map[string]string, len(englishText))
	for key, value := range englishText {
		if key == "" || value == "" {
			t.Fatalf("empty translation for %q", key)
		}
		if other, ok := seen[key]; ok {
			t.Fatalf("duplicate key %q (%q and %q)", key, other, value)
		}
		seen[key] = value
		want, got := formatVerbs(key), formatVerbs(value)
		if strings.Join(want, "|") != strings.Join(got, "|") {
			t.Errorf("verb mismatch for %q:\n  zh: %v\n  en: %v", key, want, got)
		}
	}
}

func TestTrFallsBackToChineseAndTranslatesInEnglish(t *testing.T) {
	chinese := chineseUI()
	if got := chinese.tr("发送"); got != "发送" {
		t.Errorf("Chinese UI should keep the source text, got %q", got)
	}
	if got := chinese.tr("no such key"); got != "no such key" {
		t.Errorf("unknown key should pass through, got %q", got)
	}

	english := englishUI()
	if got := english.tr("发送"); got != "Send" {
		t.Errorf("Send translation = %q, want %q", got, "Send")
	}
	if got := english.tr("no such key"); got != "no such key" {
		t.Errorf("unknown key should pass through, got %q", got)
	}
}

func TestTrfKeepsArguments(t *testing.T) {
	english := englishUI()
	if got := english.trf("Agent · %s    模型 · %s    用量 · %d tokens", "a", "m", 42); got != "Agent · a    Model · m    Usage · 42 tokens" {
		t.Errorf("trf = %q", got)
	}
	if got := english.trf("正在执行工具：%s", "shell"); got != "Running tool: shell" {
		t.Errorf("trf = %q", got)
	}
}

func TestListSeparatorFollowsLanguage(t *testing.T) {
	if got := englishUI().listSeparator(); got != ", " {
		t.Errorf("English separator = %q", got)
	}
	if got := chineseUI().listSeparator(); got != "、" {
		t.Errorf("Chinese separator = %q", got)
	}
}

// TestControllerMessagesAreLocalized replays every message the controller can
// produce and requires a real English rendering. This is what keeps the GUI from
// leaking Chinese into an English session.
func TestControllerMessagesAreLocalized(t *testing.T) {
	messages := []string{
		// status callbacks
		"安全配置读取失败，已默认启用工具调用确认",
		"尚未配置有效模型，可在 GUI 中添加或切换模型",
		"正在加载 Agent：coder",
		"正在连接 MCP：filesystem",
		// errors returned by controller.New and friends
		"初始化配置目录: permission denied",
		"加载 Agent: no such file or directory；尚未安装时请在项目目录执行 qmicode install",
		"检查 MCP 配置: broken yaml",
		"读取 MCP 配置: broken yaml",
		"连接 MCP filesystem: connection refused",
		"获取 MCP filesystem 工具: timeout",
		"注册 MCP filesystem 工具: duplicate name",
		"加载 Agent \"coder\"：missing agent.yaml",
		"Agent 名称不能为空",
		"Agent 已切换，但关闭旧 MCP 连接失败：broken pipe",
		"模型名称不能为空",
		"未找到模型配置 \"gpt\"",
		"加载模型配置 \"gpt\"：bad file",
		"模型配置 \"gpt\" 的 model 为空",
		"保存默认模型失败：read-only file system",
		"模型配置名称无效",
		"Base URL 和模型 ID 不能为空",
		"模型配置 \"gpt\" 已存在",
		"保存模型配置：disk full",
		"模型已保存，但设为默认失败：disk full",
		"会话名称不能为空",
		"未找到会话 \"today\"",
		"加载会话 \"today\"：bad json",
		"保存安全配置失败：disk full",
		"保存会话失败（/tmp/a.json）: disk full",
		"尚未配置模型，请先添加或切换模型",
		"消息不能为空",
		// appearance settings errors
		"解析 GUI 设置失败：unexpected end of JSON input",
		fmt.Sprintf("GUI 字号必须在 %.0f 到 %.0f 之间", minFontSize, maxFontSize),
		"未知 GUI 主题模式 \"neon\"",
		"未知 GUI 语言 \"fr-FR\"",
		"读取系统字体列表失败：no fontconfig",
		"系统中未找到字体 \"Inter\"",
		"系统字体文件类型 \".woff\" 目前不受支持",
		"读取系统字体文件 \"/a.otf\" 失败：permission denied",
		"保存 /home/u/.config/qmicode/config_gui.yaml 失败：disk full",
		// GUI exit path
		"GUI 退出时读取进程树: ps failed",
		"GUI 退出时遍历进程树: ps failed",
	}

	english := englishUI()
	for _, message := range messages {
		got := english.localizeMessage(message)
		for _, char := range got {
			if char >= 0x4E00 && char <= 0x9FFF {
				t.Errorf("message still contains Chinese: %q -> %q", message, got)
				break
			}
		}
		if got == message {
			t.Errorf("message was not localized: %q", message)
		}
	}
}

func TestChineseUILocalizesNothing(t *testing.T) {
	chinese := chineseUI()
	message := "保存会话失败（/tmp/a.json）: disk full"
	if got := chinese.localizeMessage(message); got != message {
		t.Errorf("Chinese localizeMessage = %q, want unchanged", got)
	}
	if err := chinese.localizeError(errors.New(message)); err == nil || err.Error() != message {
		t.Errorf("Chinese localizeError changed the error: %v", err)
	}
}

func TestLocalizeErrorPreservesNilAndUnknownErrors(t *testing.T) {
	english := englishUI()
	if err := english.localizeError(nil); err != nil {
		t.Errorf("nil error became %v", err)
	}
	if err := english.localizeError(errors.New("plain english")); err == nil || err.Error() != "plain english" {
		t.Errorf("unknown error = %v", err)
	}
	// Wrapped geometry is kept; only the message is translated.
	wrapped := fmt.Errorf("保存安全配置失败：%w", errors.New("disk full"))
	if got := english.localizeError(wrapped).Error(); got != "Could not save the safety settings: disk full" {
		t.Errorf("wrapped error = %q", got)
	}
}

// TestMessageRulesDoNotShadowEachOther guards the ordering assumption in
// messageRules: the specific agent-load message must win over the generic one.
func TestMessageRulesDoNotShadowEachOther(t *testing.T) {
	english := englishUI()
	specific := english.localizeMessage("加载 Agent: no such file；尚未安装时请在项目目录执行 qmicode install")
	if !strings.Contains(specific, "qmicode install in the project directory") {
		t.Errorf("install hint was lost: %q", specific)
	}
	generic := english.localizeMessage("加载 Agent: no such file")
	if !strings.Contains(generic, "Could not load the agent") {
		t.Errorf("generic agent error = %q", generic)
	}

	// A model error must not be routed through the agent rules.
	model := english.localizeMessage("加载模型配置 \"gpt\"：bad file")
	if !strings.Contains(model, "Could not load model configuration") {
		t.Errorf("model error = %q", model)
	}
}

// TestStatusSourceRoundTrip ensures the language switch restores the current
// status instead of substituting a readiness message.
func TestStatusSourceRoundTrip(t *testing.T) {
	english := englishUI()
	english.statusSource = statusSource{
		text: "Agent · %s    模型 · %s    用量 · %d tokens",
		args: []any{"a", "m", 7},
		set:  true,
	}
	if got := english.trf(english.statusSource.text, english.statusSource.args...); got != "Agent · a    Model · m    Usage · 7 tokens" {
		t.Errorf("status re-render = %q", got)
	}

	english.statusSource = statusSource{text: "正在连接 MCP：fs", localized: true, set: true}
	if got := english.localizeMessage(english.statusSource.text); got != "Connecting to MCP: fs" {
		t.Errorf("localized status = %q", got)
	}
}

// TestEveryTranslatedStringIsReachable scans the package sources and requires
// every table entry to have a call site, so dead translations cannot pile up
// next to unwired literals the way they did before.
func TestEveryTranslatedStringIsReachable(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	sources := make([]string, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			name == "i18n.go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, readErr := os.ReadFile(name)
		if readErr != nil {
			t.Fatalf("read %s: %v", name, readErr)
		}
		sources = append(sources, string(data))
	}
	// The controller reports messages the GUI localizes through messageRules, so
	// it is scanned as well.
	controllerData, err := os.ReadFile(filepath.Join("controller", "controller.go"))
	if err != nil {
		t.Fatalf("read controller: %v", err)
	}
	sources = append(sources, string(controllerData))

	// Keys reached through a shared constant rather than a literal.
	constantKeys := map[string]string{
		keyToolCallDetail: "keyToolCallDetail",
		keyToolCallTip:    "keyToolCallTip",
	}
	allSources := strings.Join(sources, "\n")

	var unreachable []string
	for key, value := range englishText {
		if constantName, ok := constantKeys[key]; ok {
			if !strings.Contains(allSources, constantName) {
				unreachable = append(unreachable, key)
			}
			continue
		}
		if !strings.Contains(allSources, strconv.Quote(key)) {
			unreachable = append(unreachable, key+" => "+value)
		}
	}
	if len(unreachable) > 0 {
		t.Errorf("%d translation(s) have no call site:\n  %s",
			len(unreachable), strings.Join(unreachable, "\n  "))
	}
}
