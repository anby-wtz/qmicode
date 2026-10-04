package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"qmicode-go/app/config"
	appmc "qmicode-go/app/modelconfig"
	clisafe "qmicode-go/app/safe"
	appsess "qmicode-go/app/session"
	"qmicode-go/codelib/agent"
	coremc "qmicode-go/codelib/modelconfig"
	"qmicode-go/codelib/session"

	"github.com/mattn/go-colorable"
	"github.com/nyaosorg/go-readline-ny"
)

var ORDER = map[string]string{
	"ORDER_EXIT":                  "/exit",
	"ORDER_QUIT":                  "/quit",
	"ORDER_SHOW_SESSION_JSON":     "/shsejson",
	"ORDER_SET_MODEL":             "/setmodel",
	"ORDER_SAVE_MODEL":            "/smodel",
	"ORDER_LOAD_MODEL":            "/lmodel",
	"ORDER_LIST_MODEL":            "/lsmodel",
	"ORDER_REMOVE_MODEL":          "/rmmodel",
	"ORDER_SHOW_MODEL":            "/shmodel",
	"ORDER_SAVE_MODEL_TO_DEFAULT": "/smtd",
	"ORDER_SET_SAFE":              "/setsafe",
	"ORDER_SHOW_SAFE":             "/shsafe",
	"ORDER_COMPACT":               "/compact",
	"ORDER_SHOW_SESSION":          "/shsess",
	"ORDER_LIST_SESSION":          "/lssess",
	"ORDER_LOAD_SESSION":          "/lsess",
	"ORDER_LOAD_AGENT":            "/lagent",
	"ORDER_LIST_AGENT":            "/lsagent",
}

const (
	ORDER_DEBUG_SHOW_SESSION = "/debugshsess"
)

func getOneLine(prompt string) (string, error) {
	editor := &readline.Editor{
		PromptWriter: func(w io.Writer) (int, error) {
			return io.WriteString(w, fmt.Sprintf("%s%s%s", Cyan, prompt, Reset))
		},
		Writer:       colorable.NewColorableStdout(),
		ResetColor:   Reset,
		DefaultColor: Green,
	}
	return editor.ReadLine(context.Background())
}

func process_special_input(conf config.Config, input string, rt agent.AgentRuntime, sess *session.LLMSession, model *coremc.ModelConfig, sf *clisafe.SafeConfig) bool {
	switch input {
	case ORDER["ORDER_EXIT"], ORDER["ORDER_QUIT"]:
		fmt.Printf("%sExiting...%s\n", Cyan, Reset)
		os.Exit(0)
		return false

	case ORDER["ORDER_SHOW_SESSION_JSON"]:
		str, err := sess.GetJSON()
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		fmt.Printf("%sSession JSON:%s\n%s\n", Magenta, Reset, str)
		return false

	case ORDER["ORDER_SET_MODEL"]:
		baseURL, err := getOneLine("BaseURL: ")
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		apiKey, err := getOneLine("ApiKey: ")
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		modelName, err := getOneLine("ModelName: ")
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		model.BaseURL = baseURL
		model.ApiKey = apiKey
		model.Model = modelName
		return false

	case ORDER["ORDER_SAVE_MODEL"]:
		name, err := getOneLine("name: ")
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		err = appmc.Save(model, conf, name)
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		return false

	case ORDER["ORDER_LOAD_MODEL"]:
		name, err := getOneLine("name: ")
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		err = appmc.Load(model, conf, name)
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		currentModelName = name
		return false

	case ORDER["ORDER_LIST_MODEL"]:
		names, err := appmc.ListModel(conf)
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		fmt.Print(Yellow)
		for i := range names {
			fmt.Println(names[i])
		}
		fmt.Print(Reset)
		return false

	case ORDER["ORDER_REMOVE_MODEL"]:
		name, err := getOneLine("name: ")
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		err = appmc.RemoveModel(conf, name)
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		return false

	case ORDER["ORDER_SHOW_MODEL"]:
		fmt.Printf("%sBaseURL: %s\nModelName: %s%s\n", Yellow, model.BaseURL, model.Model, Reset)
		return false

	case ORDER["ORDER_SAVE_MODEL_TO_DEFAULT"]:
		name, err := getOneLine("name: ")
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		err = appmc.SetModelToDefault(conf, name)
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		return false

	case ORDER["ORDER_SET_SAFE"]:
		confirmToolCall, err := getOneLine("Do you want confirm every tool call before execute? (y/n): ")
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		if confirmToolCall == "y" || confirmToolCall == "Y" {
			safeConfig.NeedConfirm = true
			safeConfig.SaveToDefault(conf)
			fmt.Printf("%sSet to confirm tool call%s\n", Green, Reset)
			return false
		}
		if confirmToolCall == "n" || confirmToolCall == "N" {
			safeConfig.NeedConfirm = false
			safeConfig.SaveToDefault(conf)
			fmt.Printf("%sSet to not confirm tool call%s\n", Green, Reset)
			return false
		}
		fmt.Printf("%sInvalid input%s\n", Red, Reset)
		return false

	case ORDER["ORDER_SHOW_SAFE"]:
		fmt.Printf("%sNeedConfirm: %t%s\n", Yellow, sf.NeedConfirm, Reset)
		return false

	case ORDER["ORDER_COMPACT"]:
		if err := rt.Compact(context.Background(), currentAgent, *model, sess, resCallback); err != nil {
			if errors.Is(err, agent.ErrInterrupted) {
				fmt.Printf("\n%s[Interrupted]%s Compact aborted, session unchanged.%s\n", Yellow, Reset, Reset)
			} else {
				fmt.Printf("%sError during compact: %v%s\n", Red, err.Error(), Reset)
			}
			return false
		}
		fmt.Printf("\n%s%sCompact Complete!%s\n", Green, Bold, Reset)
		return false

	case ORDER["ORDER_SHOW_SESSION"]:
		fmt.Printf("%sSession name:%s%s\n", Magenta, Reset, sess.Name)
		return false

	case ORDER["ORDER_LIST_SESSION"]:
		names, err := appsess.ListSessions(conf)
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		fmt.Print(Yellow)
		for i := range names {
			fmt.Print(names[i])
			if names[i] == sess.Name {
				fmt.Print(Green)
				fmt.Print("*(current)")
				fmt.Print(Reset)
				fmt.Print(Yellow)
			}
			fmt.Println()
		}
		fmt.Print(Reset)
		return false

	case ORDER["ORDER_LOAD_SESSION"]:
		name, err := getOneLine("name: ")
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		err = appsess.LoadSession(conf, name, sess)
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		fmt.Printf("%sLoaded session: %s%s\n", Green, name, Reset)
		s, err := MessageList2String(sess)
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		fmt.Print("\f\n")              // 清屏
		fmt.Printf("%s%s\n", s, Reset) // 重新打印会话内容
		return false

	case ORDER["ORDER_LOAD_AGENT"]:
		name, err := getOneLine("name: ")
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		err = updateCurrentAgent(&conf, name)
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		sess.SetSystemMessage(currentAgent.GetSysPrompt())
		return false

	case ORDER["ORDER_LIST_AGENT"]:
		names, err := conf.ListAgents()
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		fmt.Print(Yellow)
		for i := range names {
			fmt.Print(names[i])
			if names[i] == currentAgentName {
				fmt.Print(Green)
				fmt.Print("*(current)")
				fmt.Print(Reset)
				fmt.Print(Yellow)
			}
			fmt.Println()
		}
		fmt.Print(Reset)
		return false

	case ORDER_DEBUG_SHOW_SESSION:
		res, err := sess.GetMessageString()
		if err != nil {
			fmt.Printf("%sError: %s%s\n", Red, err.Error(), Reset)
			return false
		}
		fmt.Printf("%sSession:%s\n%+v\n", Magenta, Reset, res)
		return false

	}

	if input[0] == '/' {
		fmt.Printf("%sUnknown Command%s\n", Red, Reset)
		return false
	}

	return true
}
