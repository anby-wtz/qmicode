package term

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mattn/go-colorable"
	"github.com/nyaosorg/go-readline-ny"
	"github.com/nyaosorg/go-readline-ny/completion"
	"github.com/nyaosorg/go-readline-ny/keys"
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

func AskWithCandidates(prompt string, candidates []string) (string, error) {
	editor := &readline.Editor{
		PromptWriter: func(w io.Writer) (int, error) {
			return io.WriteString(w, fmt.Sprintf("%s%s%s", Cyan, prompt, Reset))
		},
		Writer:       colorable.NewColorableStdout(),
		ResetColor:   Reset,
		DefaultColor: Green,
	}

	editor.BindKey(keys.CtrlI, &completion.CmdCompletionOrList2{
		Delimiter: "&|><;",
		Enclosure: `"'`,
		Postfix:   "",
		Candidates: func(field []string) (forComp []string, forList []string) {
			var matched []string
			currentInput := field[len(field)-1] // 用户当前正在输入的词

			for _, c := range candidates {
				if strings.HasPrefix(c, currentInput) {
					matched = append(matched, c)
				}
			}

			return matched, matched
		},
	})
	return editor.ReadLine(context.Background())
}
