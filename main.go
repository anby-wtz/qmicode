package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	chatgpt "qmicode-go/app/chatgpt_runtime"
	"qmicode-go/app/config"
	"qmicode-go/cli"
	"qmicode-go/cli/appstate"
	"qmicode-go/gui"

	"github.com/otiai10/copy"
)

const helpStr = `Usage: qmicode [cli|chatgpt|codex|oauth|install|gui|help]`

func main() {
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "install":
			config.DefaultConfig.Init()
			generalAgentPath := config.DefaultConfig.GetAgentPath(config.DefaultConfig.GeneralAgentName)
			err := copy.Copy("./general-agent", generalAgentPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Failed to copy general-agent: %s", err.Error())
				os.Exit(1)
			}
			if err := config.EnsureMcpConfig(generalAgentPath); err != nil {
				fmt.Fprintf(os.Stderr, "Failed to create mcp config: %s\n", err.Error())
				os.Exit(1)
			}
			fmt.Printf("Install qmicode successfully!\n")
			os.Exit(0)
		case "cli":
			runCLI(appstate.ModeOpenAI)
		case "chatgpt", "codex":
			runCLI(appstate.ModeChatGPT)
		case "gui":
			gui.Start()
			return
		case "oauth":
			runOAuth()
		case "help":
			fmt.Println(helpStr)
			os.Exit(0)
		default:
			fmt.Printf("Unknown command: %s\n", os.Args[1])
			fmt.Println(helpStr)
			os.Exit(1)
		}
	}
	runCLI(appstate.ModeOpenAI)
}

func runCLI(mode appstate.Mode) {
	if err := cli.Start(mode); err != nil {
		if errors.Is(err, chatgpt.ErrNotSignedIn) {
			fmt.Fprintf(os.Stderr, "Failed: %s\nRun `qmicode oauth chatgpt` to sign in first.\n", err.Error())
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "Failed: %s\n", err.Error())
		os.Exit(1)
	}
	os.Exit(0)
}

func runOAuth() {
	if len(os.Args) < 3 {
		fmt.Println("Usage: qmicode oauth <provider>")
		os.Exit(1)
	}

	switch os.Args[2] {
	case "chatgpt":
		err := chatgpt.OAuth(context.Background(), func(url string) error {
			fmt.Printf("Please open the following URL in your browser:\n%s\n", url)
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to authenticate with ChatGPT: %s\n", err.Error())
			os.Exit(1)
		}
		fmt.Println("Authenticated with ChatGPT.")
		os.Exit(0)
	default:
		fmt.Printf("Unknown provider: %s\n", os.Args[2])
		os.Exit(1)
	}
}
