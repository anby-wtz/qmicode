package cli

import (
	"fmt"
	"strings"

	"qmicode-go/cli/appstate"
	"qmicode-go/cli/command"
	"qmicode-go/cli/term"
)

var ORDER = map[string]string{
	"ORDER_EXIT":              "/exit",
	"ORDER_QUIT":              "/quit",
	"ORDER_SHOW_SESSION_JSON": "/shsejson",
	"ORDER_SET_MODEL":         "/setmodel",
	"ORDER_LOAD_MODEL":        "/lmodel",
	"ORDER_LIST_MODEL":        "/lsmodel",
	"ORDER_REMOVE_MODEL":      "/rmmodel",
	"ORDER_SHOW_MODEL":        "/shmodel",
	"ORDER_SET_SAFE":          "/setsafe",
	"ORDER_SHOW_SAFE":         "/shsafe",
	"ORDER_COMPACT":           "/compact",
	"ORDER_SHOW_SESSION":      "/shsess",
	"ORDER_LIST_SESSION":      "/lssess",
	"ORDER_LOAD_SESSION":      "/lsess",
	"ORDER_RENAME_SESSION":    "/rensess",
	"ORDER_NEW_SESSION":       "/newsess",
	"ORDER_LOAD_AGENT":        "/lagent",
	"ORDER_LIST_AGENT":        "/lsagent",
	"ORDER_UNDO":              "/undo", // undo last message
}

const (
	ORDER_DEBUG_SHOW_SESSION = "/debugshsess"
)

// process_special_input 只负责把输入分派给 command 包中对应的处理函数。
// 返回 true 表示不是命令、应按普通对话继续处理。
func process_special_input(app *appstate.App, input string) bool {
	switch input {
	case ORDER["ORDER_EXIT"], ORDER["ORDER_QUIT"]:
		return command.Exit(app)

	case ORDER["ORDER_SHOW_SESSION_JSON"]:
		return command.ShowSessionJSON(app)

	case ORDER["ORDER_SET_MODEL"]:
		return command.SetModel(app)

	case ORDER["ORDER_LOAD_MODEL"]:
		return command.LoadModel(app)

	case ORDER["ORDER_LIST_MODEL"]:
		return command.ListModel(app)

	case ORDER["ORDER_REMOVE_MODEL"]:
		return command.RemoveModel(app)

	case ORDER["ORDER_SHOW_MODEL"]:
		return command.ShowModel(app)

	case ORDER["ORDER_SET_SAFE"]:
		return command.SetSafe(app)

	case ORDER["ORDER_SHOW_SAFE"]:
		return command.ShowSafe(app)

	case ORDER["ORDER_COMPACT"]:
		return command.Compact(app, resCallback)

	case ORDER["ORDER_SHOW_SESSION"]:
		return command.ShowSession(app)

	case ORDER["ORDER_LIST_SESSION"]:
		return command.ListSession(app)

	case ORDER["ORDER_LOAD_SESSION"]:
		return command.LoadSession(app)

	case ORDER["ORDER_RENAME_SESSION"]:
		return command.RenameSession(app)

	case ORDER["ORDER_NEW_SESSION"]:
		return command.NewSession(app)

	case ORDER["ORDER_LOAD_AGENT"]:
		return command.LoadAgent(app, func(name string) error {
			return updateCurrentAgent(app, name)
		})

	case ORDER["ORDER_LIST_AGENT"]:
		return command.ListAgent(app)

	case ORDER["ORDER_UNDO"]:
		return command.Undo(app)

	case ORDER_DEBUG_SHOW_SESSION:
		return command.DebugShowSession(app)
	}

	// 用户直接回车时 input 为空，不能取 input[0]
	if strings.HasPrefix(input, "/") {
		fmt.Printf("%sUnknown Command%s\n", term.Red, term.Reset)
		return false
	}

	return true
}
