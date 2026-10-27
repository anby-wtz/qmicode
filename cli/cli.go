package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	appagent "qmicode-go/app/agent"
	"qmicode-go/app/appmcp"
	"qmicode-go/app/config"
	appmc "qmicode-go/app/modelconfig"
	"qmicode-go/cli/appstate"
	"qmicode-go/cli/term"
	"qmicode-go/codelib/agent"
	"slices"
	"strings"
	"time"

	"github.com/mattn/go-colorable"
	"github.com/nyaosorg/go-readline-ny"
	"github.com/nyaosorg/go-readline-ny/completion"
	"github.com/nyaosorg/go-readline-ny/keys"
	"github.com/nyaosorg/go-readline-ny/simplehistory"
	"github.com/openai/openai-go"

	appsess "qmicode-go/app/session"
)

func resCallback(s string, t agent.LLMChunkContentType) {
	switch t {
	case agent.ContentReasoning:
		fmt.Printf("%s%s%s", LightGray+Dim, s, Reset)
		return
	case agent.ContentAnswer:
		fmt.Printf("%s%s%s", LightGray+Bold, s, Reset)
		return
	}
}

func premsgCallback(app *appstate.App) agent.PreMsgFilterFuncCallback {
	return func(s string, e error) bool {
		if e != nil {
			fmt.Printf("%s[Error]%s%s\n", Red, e.Error(), Reset)
			return false
		}
		fmt.Printf("%s[Tool Call]%s\n%s%s%s\n", Yellow, Reset, Blue+Bold, s, Reset)
		if app.Safe.NeedConfirm {
			res, err := term.Ask("Confirm Tool Call? (y/n):")
			if err != nil {
				fmt.Printf("%s[Error]%s%s\n", Red, err.Error(), Reset)
				return false
			}
			if res != "y" && res != "Y" {
				fmt.Printf("%sTool Call Cancelled.%s\n", Yellow, Reset)
				return false
			}
		}
		return true
	}
}

func aftermsgCallback(s string, err error) {
	if err != nil {
		fmt.Printf("%s[Error]%s\n%s\n", Red, err.Error(), Reset)
		return
	}
	fmt.Printf("%s[Tool Result]%s%s\n%s%s\n", Green, Reset, Blue, s, Reset)
}

var __history *simplehistory.Container = nil
var __editor *readline.Editor = nil

func GetDefaultEditorAndHistory(promptWriter func(io.Writer) (int, error)) (*readline.Editor, *simplehistory.Container) {
	if __editor == nil {
		__history = simplehistory.New()

		__editor = &readline.Editor{
			PromptWriter: promptWriter,
			Writer:       colorable.NewColorableStdout(),
			History:      __history,
			ResetColor:   Reset,
			DefaultColor: Cyan,
		}

		__editor.BindKey(keys.CtrlI, &completion.CmdCompletionOrList2{
			Delimiter: "&|><;",
			Enclosure: `"'`,
			Postfix:   "",
			Candidates: func(field []string) (forComp []string, forList []string) {
				allCandidates := slices.Collect(maps.Values(ORDER))

				var matched []string
				currentInput := field[len(field)-1] // 用户当前正在输入的词

				for _, c := range allCandidates {
					if strings.HasPrefix(c, currentInput) {
						matched = append(matched, c)
					}
				}

				return matched, matched
			},
		})
	}

	return __editor, __history
}

func updateCurrentAgent(app *appstate.App, name string) error {
	if name == app.AgentName {
		return nil
	}
	if app.Ag != nil {
		app.Ag.Destroy()
	}
	ag, err := appagent.LoadPathAgent(app.Conf.GetAgentPath(name), name)
	if err != nil {
		return err
	}
	app.Ag = ag
	app.AgentName = name

	return loadMcp(app)
}

func loadMcp(app *appstate.App) error {
	mcpFilePath := app.Conf.GetAgentMcpConfigFilePath(app.AgentName)

	// mcp.yaml 缺失时视为该 Agent 未配置 MCP，直接跳过，
	// 不让“没有 MCP 配置”阻断整个 Agent 的加载。
	if _, statErr := os.Stat(mcpFilePath); errors.Is(statErr, fs.ErrNotExist) {
		return nil
	}

	mcpConfig, err := appmcp.GetMcpListConfigFromFile(mcpFilePath)

	var remoteList []*appmcp.RemoteMcpConfig
	var localList []*appmcp.LocalMcpConfig

	if err != nil {
		fmt.Printf("%sLoad MCP Config Failed: %v%s\n", Red, err, Reset)
		return err
	}

	remoteList = make([]*appmcp.RemoteMcpConfig, len(mcpConfig.Remote))
	for i, v := range mcpConfig.Remote {
		remoteList[i] = &appmcp.RemoteMcpConfig{
			Name:     v.Name,
			Endpoint: v.Endpoint,
		}
	}

	localList = make([]*appmcp.LocalMcpConfig, len(mcpConfig.Local))
	for i, v := range mcpConfig.Local {
		localList[i] = &appmcp.LocalMcpConfig{
			Name:    v.Name,
			Command: v.Cmd,
			Args:    v.Args,
			Workdir: app.Conf.GetAgentPath(app.AgentName),
		}
	}

	err = appmcp.ConfigAllMcp(context.Background(),
		remoteList, localList, app.Ag,
		func(tp appmcp.McpType, name string) {
			fmt.Printf("%sconnecting to MCP %s (%s).%s\n", Green, name, func(tp appmcp.McpType) string {
				switch tp {
				case appmcp.McpTypeRemote:
					return "Remote"
				case appmcp.McpTypeLocal:
					return "Local"
				default:
					return "Unknown"
				}
			}(tp), Reset)
		})

	if err != nil {
		fmt.Printf("%sFailed to connect to MCP: %v%s\n", Red, err, Reset)
		return err
	}

	return nil
}

func Start() {
	conf := config.DefaultConfig

	app := &appstate.App{
		Conf: conf,
		Rt:   agent.NewAgentRuntime(),
	}

	err := updateCurrentAgent(app, conf.GeneralAgentName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sFailed: %v%s\n", Red, err, Reset)
		os.Exit(1)
	}

	defer app.Ag.Destroy()

	app.Sess = app.Ag.GetInitLLMSession(time.Unix(time.Now().Unix(), 0).Format("2006-01-02 15:04:05"))

	app.ModelName, err = appmc.LoadDefaultModel(&app.Model, conf)
	if err != nil {
		fmt.Printf("%sLoad Default Model Failed: %v. Please Set You Model By `/setmodel`%s\n", Red, err, Reset)
	}

	err = app.Safe.LoadFromDefault(conf)
	if err != nil {
		fmt.Printf("%sLoad Default Safe Config Failed: %v%s.\n%sReal Environment and Need Confirm Tool Call will be Set.%s\n", Red, err, Reset, Yellow, Reset)
	}

	editor, history := GetDefaultEditorAndHistory(func(w io.Writer) (int, error) {
		return fmt.Fprintf(w, "\n%s(Agent: %s%s%s, Model: %s%s%s, Usage: %s%d%s)%s %s> %s", Cyan, Bold, app.AgentName, Reset+Cyan, Bold, app.ModelName, Reset+Cyan, Bold, app.Sess.Usage, Reset+Cyan, Reset, Bold, Reset)
	})

	for !app.ExitRequested {
		prompt, err := editor.ReadLine(context.Background())
		if err != nil {
			fmt.Printf("%sERR=%s%s\n", Red, err.Error(), Reset)
			continue
		}

		history.Add(prompt)

		if !process_special_input(app, prompt) {
			continue
		}

		app.Sess.AddMessage(openai.UserMessage(prompt))

		if err := runAgentLoop(app); errors.Is(err, agent.ErrInterrupted) {
			fmt.Printf("\n%s[Interrupted]%s Session saved.%s\n", Yellow, Reset, Reset)
		}

		appsess.SaveSession(conf, app.Sess)

	}
}
