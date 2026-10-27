package command

import (
	"fmt"

	"qmicode-go/cli/appstate"
	"qmicode-go/cli/term"
)

// SetSafe 处理 /setsafe：交互式设置执行工具调用前是否需要确认，并持久化。
func SetSafe(app *appstate.App) bool {
	confirmToolCall, err := term.Ask("Do you want confirm every tool call before execute? (y/n): ")
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	if confirmToolCall == "y" || confirmToolCall == "Y" {
		app.Safe.NeedConfirm = true
		app.Safe.SaveToDefault(app.Conf)
		fmt.Printf("%sSet to confirm tool call%s\n", term.Green, term.Reset)
		return false
	}
	if confirmToolCall == "n" || confirmToolCall == "N" {
		app.Safe.NeedConfirm = false
		app.Safe.SaveToDefault(app.Conf)
		fmt.Printf("%sSet to not confirm tool call%s\n", term.Green, term.Reset)
		return false
	}
	fmt.Printf("%sInvalid input%s\n", term.Red, term.Reset)
	return false
}

// ShowSafe 处理 /shsafe：显示当前安全配置。
func ShowSafe(app *appstate.App) bool {
	fmt.Printf("%sNeedConfirm: %t%s\n", term.Yellow, app.Safe.NeedConfirm, term.Reset)
	return false
}
