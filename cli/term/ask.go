package term

import (
	"context"
	"fmt"
	"io"

	"github.com/mattn/go-colorable"
	"github.com/nyaosorg/go-readline-ny"
)

// Ask 读取一行输入。用于命令执行过程中的交互式追问（如输入模型名）。
func Ask(prompt string) (string, error) {
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
