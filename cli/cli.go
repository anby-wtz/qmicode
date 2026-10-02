package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"qmicode-go/app/appmcp"
	appagent "qmicode-go/app/agent"
	"qmicode-go/app/config"
	appmc "qmicode-go/app/modelconfig"
	clisafe "qmicode-go/app/safe"
	"qmicode-go/codelib/agent"
	coremc "qmicode-go/codelib/modelconfig"
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

var safeConfig clisafe.SafeConfig

func premsgCallback(s string, e error) bool {
	if e != nil {
		fmt.Printf("%s[Error]%s%s\n", Red, e.Error(), Reset)
		return false
	}
	fmt.Printf("%s[Tool Call]%s\n%s%s%s\n", Yellow, Reset, Blue+Bold, s, Reset)
	if safeConfig.NeedConfirm {
		res, err := getOneLine("Confirm Tool Call? (y/n):")
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

func Start() {
	conf := config.DefaultConfig

	general_agent, err := appagent.LoadPathAgent(conf.GetAgentPath(conf.GeneralAgentName), conf.GeneralAgentName)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sFailed: %v%s\n", Red, err, Reset)
		os.Exit(1)
	}

	defer general_agent.Destroy()

	rt := agent.NewAgentRuntime(general_agent)

	sess := general_agent.GetInitLLMSession(time.Unix(time.Now().Unix(), 0).Format("2006-01-02 15:04:05"))

	var modelConfig coremc.ModelConfig

	err = appmc.LoadDefaultModel(&modelConfig, conf)
	if err != nil {
		fmt.Printf("%sLoad Default Model Failed: %v. Please Set You Model By `/setmodel`%s\n", Red, err, Reset)
	}

	// load mcp for test

	// err = appmcp.ConfigAllMcp(context.Background(), []*appmcp.RemoteMcpConfig{
	// 	{
	// 		Name:     "exa",
	// 		Endpoint: "https://mcp.exa.ai/mcp",
	// 	},
	// }, []*appmcp.LocalMcpConfig{}, general_agent, func(tp appmcp.McpType, name string) {
	// 	fmt.Printf("%sconnecting to MCP %s (%s).%s\n", Green, name, func(tp appmcp.McpType) string {
	// 		switch tp {
	// 		case appmcp.McpTypeRemote:
	// 			return "Remote"
	// 		case appmcp.McpTypeLocal:
	// 			return "Local"
	// 		default:
	// 			return "Unknown"
	// 		}
	// 	}(tp), Reset)
	// })

	mcpConfig, err := appmcp.GetMcpListConfigFromFile(
		conf.GetAgentMcpConfigFilePath(conf.GeneralAgentName),
	)

	var remoteList []*appmcp.RemoteMcpConfig
	var localList []*appmcp.LocalMcpConfig

	if err != nil {
		fmt.Printf("%sLoad MCP Config Failed: %v%s\n", Red, err, Reset)
		goto FinishMcpConnect // we can't directly exit just because of mcp failure.
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
			Workdir: conf.GetAgentPath(conf.GeneralAgentName),
		}
	}

	err = appmcp.ConfigAllMcp(context.Background(),
		remoteList, localList, general_agent,
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
		goto FinishMcpConnect
	}

FinishMcpConnect:

	err = safeConfig.LoadFromDefault(conf)
	if err != nil {
		fmt.Printf("%sLoad Default Safe Config Failed: %v%s.\n%sReal Environment and Need Confirm Tool Call will be Set.%s\n", Red, err, Reset, Yellow, Reset)
	}

	editor, history := GetDefaultEditorAndHistory(func(w io.Writer) (int, error) {
		return fmt.Fprintf(w, "\n%s(Usage: %d)%s %s> %s", Cyan, sess.Usage, Reset, Bold, Reset)
	})

	for {
		prompt, err := editor.ReadLine(context.Background())
		if err != nil {
			fmt.Printf("%sERR=%s%s\n", Red, err.Error(), Reset)
			continue
		}

		history.Add(prompt)

		if !process_special_input(conf, prompt, rt, sess, &modelConfig, &safeConfig) {
			continue
		}

		sess.AddMessage(openai.UserMessage(prompt))

		if err := runAgentLoop(rt, &modelConfig, sess, conf); errors.Is(err, agent.ErrInterrupted) {
			fmt.Printf("\n%s[Interrupted]%s Session saved.%s\n", Yellow, Reset, Reset)
		}

		appsess.SaveSession(conf, sess)

	}
}
