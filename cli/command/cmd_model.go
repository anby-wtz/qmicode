package command

import (
	"fmt"

	appmc "qmicode-go/app/modelconfig"
	"qmicode-go/cli/appstate"
	"qmicode-go/cli/term"
)

// SetModel 处理 /setmodel：交互式录入新模型并保存，同时设为默认。
func SetModel(app *appstate.App) bool {
	name, err := term.Ask("name: ")

	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}

	baseURL, err := term.Ask("BaseURL: ")
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	apiKey, err := term.Ask("ApiKey: ")
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	modelName, err := term.Ask("ModelName: ")
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	app.Model.BaseURL = baseURL
	app.Model.ApiKey = apiKey
	app.Model.Model = modelName

	err = appmc.Save(&app.Model, app.Conf, name)
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}

	app.ModelName = name

	// set model to default
	err = appmc.SetModelToDefault(app.Conf, name)
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}

	return false
}

// LoadModel 处理 /lmodel：按名加载已有模型并设为默认。
func LoadModel(app *appstate.App) bool {
	name, err := term.Ask("name: ")
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	err = appmc.Load(&app.Model, app.Conf, name)
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	app.ModelName = name

	// set model to default
	err = appmc.SetModelToDefault(app.Conf, name)
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	return false
}

// ListModel 处理 /lsmodel：列出全部已保存模型。
func ListModel(app *appstate.App) bool {
	names, err := appmc.ListModel(app.Conf)
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	fmt.Print(term.Yellow)
	for i := range names {
		fmt.Println(names[i])
	}
	fmt.Print(term.Reset)
	return false
}

// RemoveModel 处理 /rmmodel：删除模型，并自动切换到列表中的第一个模型。
func RemoveModel(app *appstate.App) bool {
	name, err := term.Ask("name: ")
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	err = appmc.RemoveModel(app.Conf, name)
	if err != nil {
		fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
		return false
	}
	// 自动切换到模型列表的第一个模型
	models, err := appmc.ListModel(app.Conf)
	if err != nil {
		// 警告：无法自动切换模型
		fmt.Printf("%sWarning: Cannot auto switch model%s\n", term.Yellow, term.Reset)
		return false
	}
	if len(models) > 0 {
		err = appmc.Load(&app.Model, app.Conf, models[0])
		if err != nil {
			fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
			return false
		}
		app.ModelName = models[0]
		err = appmc.SetModelToDefault(app.Conf, models[0])
		if err != nil {
			fmt.Printf("%sError: %s%s\n", term.Red, err.Error(), term.Reset)
			return false
		}
	}
	return false
}

// ShowModel 处理 /shmodel：显示当前模型信息。
func ShowModel(app *appstate.App) bool {
	fmt.Printf("%sBaseURL: %s\nModelName: %s%s\n", term.Yellow, app.Model.BaseURL, app.Model.Model, term.Reset)
	return false
}
