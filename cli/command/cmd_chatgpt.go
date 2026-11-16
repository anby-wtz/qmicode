package command

import (
	"context"
	"fmt"

	"qmicode-go/app/chatgpt_runtime"
	"qmicode-go/cli/appstate"
	"qmicode-go/cli/term"
)

func printChatGPTModels(models []chatgpt_runtime.ModelInfo) {
	fmt.Printf("%s%-32s %-28s %10s  %-6s %s%s\n", term.Cyan, "ID", "Name", "Context", "Images", "Effort", term.Reset)
	for _, m := range models {
		images := "no"
		if m.Images {
			images = "yes"
		}
		effort := "-"
		if len(m.Thinking) > 0 {
			effort = ""
			for i, t := range m.Thinking {
				if i > 0 {
					effort += ","
				}
				effort += t
			}
		}
		fmt.Printf("%s%-32s %-28s %10d  %-6s %s%s\n", term.Yellow, m.ID, m.Name, m.ContextWindow, images, effort, term.Reset)
	}
}

// SelectChatGPTModel 从远端列表中挑选一个模型，支持补全且不允许自由输入。
func SelectChatGPTModel(models []chatgpt_runtime.ModelInfo) (chatgpt_runtime.ModelInfo, error) {
	ids := make([]string, 0, len(models))
	for _, m := range models {
		ids = append(ids, m.ID)
	}

	for {
		name, err := term.AskWithCandidates("model: ", ids)
		if err != nil {
			return chatgpt_runtime.ModelInfo{}, err
		}
		model, ok := chatgpt_runtime.FindModel(models, name)
		if ok {
			if !model.Images {
				fmt.Printf("%sNote: %s accepts text only, attached images will be rejected by the API.%s\n", term.Yellow, model.ID, term.Reset)
			}
			return model, nil
		}
		fmt.Printf("%sUnknown model: %s%s\n", term.Red, name, term.Reset)
	}
}

// SelectChatGPTEffort 在该模型支持的档位中挑选推理强度，空回车表示默认。
func SelectChatGPTEffort(model chatgpt_runtime.ModelInfo) string {
	if len(model.Thinking) == 0 {
		return ""
	}
	fmt.Printf("%sReasoning effort options: %v (empty = default)%s\n", term.Cyan, model.Thinking, term.Reset)

	effort, err := term.AskWithCandidates("effort: ", model.Thinking)
	if err != nil {
		return ""
	}
	if effort == "" {
		return ""
	}
	if chatgpt_runtime.NormalizeEffort(model, effort) == "" {
		fmt.Printf("%sUnsupported effort: %s, use default%s\n", term.Yellow, effort, term.Reset)
		return ""
	}
	return effort
}

// SetupChatGPT 是 chatgpt/codex 模式的启动流程：校验登录、拉取远端模型列表、确定模型与档位。
func SetupChatGPT(app *appstate.App) error {
	conf := app.Conf

	settings, err := chatgpt_runtime.LoadSettings(conf)
	if err != nil {
		return err
	}

	rt, err := chatgpt_runtime.New(conf, settings.BaseURL)
	if err != nil {
		return err
	}

	models, err := chatgpt_runtime.ListModels(context.Background(), conf, settings.BaseURL)
	if err != nil {
		return err
	}
	if len(models) == 0 {
		return fmt.Errorf("chatgpt: the account lists no model")
	}

	model, ok := chatgpt_runtime.FindModel(models, settings.Model)
	if !ok {
		fmt.Printf("%sNo model selected yet, choose one from the account:%s\n", term.Cyan, term.Reset)
		printChatGPTModels(models)
		model, err = SelectChatGPTModel(models)
		if err != nil {
			return err
		}
		settings.Model = model.ID
		settings.ReasoningEffort = SelectChatGPTEffort(model)
	}

	if effort := chatgpt_runtime.NormalizeEffort(model, settings.ReasoningEffort); effort != settings.ReasoningEffort {
		settings.ReasoningEffort = effort
	}

	if err := chatgpt_runtime.SaveSettings(conf, settings); err != nil {
		return err
	}

	settings.ApplyTo(&app.Model)
	app.ModelName = settings.Model
	app.Rt = rt
	return nil
}

// LoadChatGPTModel 处理 chatgpt 模式下的 /lmodel：重新从远端列表选择模型。
func LoadChatGPTModel(app *appstate.App) bool {
	settings, err := chatgpt_runtime.LoadSettings(app.Conf)
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}

	models, err := chatgpt_runtime.ListModels(context.Background(), app.Conf, settings.BaseURL)
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}

	printChatGPTModels(models)

	model, err := SelectChatGPTModel(models)
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}

	settings.Model = model.ID
	settings.ReasoningEffort = SelectChatGPTEffort(model)

	if err := chatgpt_runtime.SaveSettings(app.Conf, settings); err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}

	settings.ApplyTo(&app.Model)
	app.ModelName = settings.Model
	fmt.Printf("%sModel switched to %s%s\n", term.Green, settings.Model, term.Reset)
	return false
}

// ShowChatGPTModel 处理 chatgpt 模式下的 /shmodel。
func ShowChatGPTModel(app *appstate.App) bool {
	settings, err := chatgpt_runtime.LoadSettings(app.Conf)
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}

	email, err := chatgpt_runtime.AccountEmail(app.Conf)
	if err != nil {
		email = "<unknown>"
	}

	effort := settings.ReasoningEffort
	if effort == "" {
		effort = "<default>"
	}

	fmt.Printf("%sRuntime: ChatGPT (OAuth)\nEndpoint: %s\nModel: %s\nReasoningEffort: %s\nAccount: %s%s\n",
		term.Yellow, chatgpt_runtime.Endpoint(settings.BaseURL), settings.Model, effort, email, term.Reset)
	return false
}
