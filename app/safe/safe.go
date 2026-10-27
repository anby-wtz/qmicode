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
	// safe 缺失或字段不全时，直接类型断言会 panic，这里逐层做安全检查
	safeRaw, ok := data["safe"].(map[string]any)
	if !ok {
		return fmt.Errorf("no default safe config set")
	}
	envRaw, ok := safeRaw["environment"].(int)
	if !ok {
		return fmt.Errorf("invalid safe environment")
	}
	needConfirmRaw, ok := safeRaw["needConfirm"].(bool)
	if !ok {
		return fmt.Errorf("invalid safe needConfirm")
	}
	save.Environment = SafeEnvironment(envRaw)
	save.NeedConfirm = needConfirmRaw
	return nil
}
