package main

import (
	"fmt"
	"os"
	"qmicode-go/app/config"
	"qmicode-go/cli"

	"github.com/otiai10/copy"
)

const helpStr = `Usage: qmicode [command]`

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
			fmt.Printf("Install qmicode successfully!\n")
			os.Exit(0)
		case "cli":
			cli.Start()
			os.Exit(0)
		case "gui":
			// todo
			os.Exit(0)
		case "help":
			fmt.Println(helpStr)
			os.Exit(0)
		default:
			fmt.Printf("Unknown command: %s\n", os.Args[1])
			fmt.Println(helpStr)
			os.Exit(1)
		}
	}
	cli.Start()
}
