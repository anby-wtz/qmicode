package chatgpt_runtime

import (
	"errors"
	"fmt"
	"os"

	"qmicode-go/app/config"
	coremc "qmicode-go/codelib/modelconfig"

	"go.yaml.in/yaml/v3"
)

type Settings struct {
	BaseURL         string `yaml:"baseurl"`
	Model           string `yaml:"model"`
	ReasoningEffort string `yaml:"reasoning_effort"`
}

func LoadSettings(conf config.Config) (Settings, error) {
	var s Settings

	data, err := os.ReadFile(conf.GetChatgptSettingsPath())
	if errors.Is(err, os.ErrNotExist) {
		return s, nil
	}
	if err != nil {
		return s, err
	}

	if err := yaml.Unmarshal(data, &s); err != nil {
		return Settings{}, fmt.Errorf("parse %s: %w", conf.GetChatgptSettingsPath(), err)
	}
	return s, nil
}

func SaveSettings(conf config.Config, s Settings) error {
	data, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(conf.WorkDir, 0755); err != nil {
		return err
	}
	return os.WriteFile(conf.GetChatgptSettingsPath(), data, 0644)
}

func (s Settings) ApplyTo(cfg *coremc.ModelConfig) {
	cfg.Model = s.Model
	cfg.Options.ReasoningEffort = s.ReasoningEffort
}
