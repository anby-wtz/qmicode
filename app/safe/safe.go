package safe

import (
	"fmt"
	"os"
	"qmicode-go/app/config"

	"go.yaml.in/yaml/v3"
)

type SafeEnvironment int

const (
	SafeEnvironmentRealSystem SafeEnvironment = iota
	SafeEnvironmentSandbox
)

type SafeConfig struct {
	Environment SafeEnvironment `yaml:"environment"`
	NeedConfirm bool            `yaml:"needConfirm"`
}

func (save *SafeConfig) SaveToDefault(conf config.Config) error {
	confPath := conf.GetGeneralConfigFilePath()
	dataRaw, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}
	data := make(map[string]any)
	err = yaml.Unmarshal(dataRaw, &data)
	if err != nil {
		return err
	}
	data["safe"] = *save
	dataRaw, err = yaml.Marshal(&data)
	if err != nil {
		return err
	}
	err = os.WriteFile(confPath, dataRaw, 0644)
	if err != nil {
		return err
	}
	return nil
}

func (save *SafeConfig) LoadFromDefault(conf config.Config) error {
	confPath := conf.GetGeneralConfigFilePath()
	dataRaw, err := os.ReadFile(confPath)
	if err != nil {
		return err
	}
	data := make(map[string]any)
	err = yaml.Unmarshal(dataRaw, &data)
	if err != nil {
		return err
	}
	if data["safe"] == nil {
		return fmt.Errorf("no default safe config set")
	}
	save.Environment = SafeEnvironment(data["safe"].(map[string]any)["environment"].(int))
	save.NeedConfirm = data["safe"].(map[string]any)["needConfirm"].(bool)
	return nil
}
