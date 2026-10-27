package command

import (
	"fmt"

	"qmicode-go/cli/appstate"
	"qmicode-go/cli/term"
)

// Exit 处理 /exit 与 /quit。
// 不直接调用 os.Exit，而是置位 app.ExitRequested，由 cli.Start 的循环退出，
// 从而保证 cli.Start 中 defer 的 Ag.Destroy() 能正常执行。
func Exit(app *appstate.App) bool {
	fmt.Printf("%sExiting...%s\n", term.Cyan, term.Reset)
	app.ExitRequested = true
	return false
}
