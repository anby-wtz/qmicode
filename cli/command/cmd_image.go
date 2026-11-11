package command

import (
	"fmt"
	"qmicode-go/cli/appstate"
	"qmicode-go/cli/term"
)

// AttachImage 处理 /attimg :附加图片
func AttachImage(app *appstate.App) bool {
	path, err := term.Ask("path: ")
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	app.AttachImages = append(app.AttachImages, path)
	fmt.Printf("%sAttach Complete!%s\n", term.Green, term.Reset)
	return false
}
