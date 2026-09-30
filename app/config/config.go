package config

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

type Config struct {
	WorkDir               string
	AgentsDir             string
	GeneralAgentName      string
	ModelConfigDir        string
	SessionDir            string
	GeneralConfigFileName string

	McpConfigFileName string
}

var DefaultConfig = Config{
	AgentsDir:             "agents",
	GeneralAgentName:      "general",
	ModelConfigDir:        "models",
	SessionDir:            "sessions",
	GeneralConfigFileName: "config.yaml",

	McpConfigFileName: "mcp.yaml",
}

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		DefaultConfig.WorkDir = filepath.Join(".", ".config", "qmicode")
		return
	}
	DefaultConfig.WorkDir = filepath.Join(homeDir, ".config", "qmicode")
}

func (config *Config) Init() error {
	err := os.MkdirAll(config.WorkDir, 0755)
	if err != nil {
		return err
	}
	err = os.MkdirAll(config.GetAgentPath(config.GeneralAgentName), 0755)
	if err != nil {
		return err
	}
	err = os.MkdirAll(config.GetModelConfigDirPath(), 0755)
	if err != nil {
		return err
	}
	err = os.MkdirAll(config.GetSessionDirPath(), 0755)
	if err != nil {
		return err
	}

	_, err = os.Stat(config.GetGeneralConfigFilePath())
	if errors.Is(err, fs.ErrNotExist) {
		os.Create(config.GetGeneralConfigFilePath())
		return nil
	}
	if err != nil {
		return err
	}
	return nil
}

func (config *Config) GetAgentPath(agentName string) string {
	return filepath.Join(config.WorkDir, config.AgentsDir, agentName)
}

func (config *Config) GetModelConfigDirPath() string {
	return filepath.Join(config.WorkDir, config.ModelConfigDir)
}

func (conf *Config) GetGeneralConfigFilePath() string {
	return filepath.Join(conf.WorkDir, conf.GeneralConfigFileName)
}

func (config *Config) GetSessionDirPath() string {
	return filepath.Join(config.WorkDir, config.SessionDir)
}

func (config *Config) GetSessionPath(sessName string) string {
	return filepath.Join(config.GetSessionDirPath(), sessName+".json")
}

func (config *Config) GetAgentMcpConfigFilePath(agentName string) string {
	return filepath.Join(config.GetAgentPath(agentName), config.McpConfigFileName)
}
