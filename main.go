package main

import (
	"fmt"
	"os"
	"qmicode-go/app/config"
	"qmicode-go/cli"

	"github.com/otiai10/copy"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "install" {
		config.DefaultConfig.Init()
		generalAgentPath := config.DefaultConfig.GetAgentPath(config.DefaultConfig.GeneralAgentName)
		err := copy.Copy("./general-agent", generalAgentPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to copy general-agent: %s", err.Error())
			os.Exit(1)
		}
		fmt.Printf("Install qmicode successfully!\n")
		os.Exit(0)
	}
	cli.Start()
}
