package command

import (
	"fmt"

	"qmicode-go/cli/appstate"
	"qmicode-go/cli/term"
)

// ListAgent 处理 /lsagent：列出全部 Agent，并标记当前使用的那个。
func ListAgent(app *appstate.App) bool {
	names, err := app.Conf.ListAgents()
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	fmt.Print(term.Yellow)
	for i := range names {
		fmt.Print(names[i])
		if names[i] == app.AgentName {
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

// LoadAgent 处理 /lagent：按名切换 Agent。
func LoadAgent(app *appstate.App, reload func(name string) error) bool {
	agents, err := app.Conf.ListAgents()
	var name string
	if err != nil {
		name, err = term.Ask("name: ")
	} else {
		name, err = term.AskWithCandidates("name: ", agents)
	}
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	err = reload(name)
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	app.Sess.SetSystemMessage(app.Ag.GetSysPrompt())
	return false
}
