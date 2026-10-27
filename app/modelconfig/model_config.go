package modelconfig

import (
	"fmt"
	"os"
	"path/filepath"

	"qmicode-go/app/config"
	coremc "qmicode-go/codelib/modelconfig"

	"go.yaml.in/yaml/v3"
)

func SaveToFile(agentConfig *coremc.ModelConfig, path string) error {
	data, err := yaml.Marshal(agentConfig)
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

func LoadFromFile(agentConfig *coremc.ModelConfig, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	err = yaml.Unmarshal(data, agentConfig)
	if err != nil {
		return err
	}

	return nil
}

func Save(agentConfig *coremc.ModelConfig, conf config.Config, name string) error {
	return SaveToFile(agentConfig, filepath.Join(conf.GetModelConfigDirPath(), name+".yaml"))
}

func Load(agentConfig *coremc.ModelConfig, conf config.Config, name string) error {
	return LoadFromFile(agentConfig, filepath.Join(conf.GetModelConfigDirPath(), name+".yaml"))
}

func ListModel(conf config.Config) ([]string, error) {
	directory := conf.GetModelConfigDirPath()
	files, err := os.ReadDir(directory)
	if err != nil {
		return nil, err
	}

	var modelNames []string
	for _, file := range files {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".yaml" {
			modelNames = append(modelNames, file.Name()[:len(file.Name())-5])
		}
	}
	return modelNames, nil
}

func RemoveModel(conf config.Config, name string) error {
	path := filepath.Join(conf.GetModelConfigDirPath(), name+".yaml")
	return os.Remove(path)
}

func SetModelToDefault(conf config.Config, name string) error {
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
	data["defaultModel"] = name
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

// return default name
func LoadDefaultModel(model *coremc.ModelConfig, conf config.Config) (string, error) {
	confPath := conf.GetGeneralConfigFilePath()
	dataRaw, err := os.ReadFile(confPath)
	if err != nil {
		return "", err
	}
	data := make(map[string]any)
	err = yaml.Unmarshal(dataRaw, &data)
	if err != nil {
		return "", err
	}
	// 未设置时 data["defaultModel"] 为 nil，直接类型断言会 panic
	raw, ok := data["defaultModel"]
	if !ok || raw == nil {
		return "", fmt.Errorf("no default model set")
	}
	defaultModelName, ok := raw.(string)
	if !ok || defaultModelName == "" {
		return "", fmt.Errorf("no default model set")
	}
	return defaultModelName, Load(model, conf, defaultModelName)
}
