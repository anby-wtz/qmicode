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

// mcpConfigFileName 是 agent 目录下 MCP 配置的默认文件名。
// 同时被 DefaultConfig.McpConfigFileName 与 EnsureMcpConfig 使用，保持单一来源。
const mcpConfigFileName = "mcp.yaml"

var DefaultConfig = Config{
	AgentsDir:             "agents",
	GeneralAgentName:      "general",
	ModelConfigDir:        "models",
	SessionDir:            "sessions",
	GeneralConfigFileName: "config.yaml",

	McpConfigFileName: mcpConfigFileName,
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

// EnsureMcpConfig 确保 agent 目录下存在 mcp.yaml。
// 若缺失则从 mcp.yaml.example 复制；示例也不存在时写入一个空的 mcp 配置，
// 避免 loadMcp 因文件不存在而让整个 CLI 无法启动。
func EnsureMcpConfig(agentPath string) error {
	mcpPath := filepath.Join(agentPath, mcpConfigFileName)

	if _, err := os.Stat(mcpPath); err == nil {
		return nil
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}

	examplePath := mcpPath + ".example"
	if data, err := os.ReadFile(examplePath); err == nil {
		return os.WriteFile(mcpPath, data, 0644)
	}

	return os.WriteFile(mcpPath, []byte("remote:\nlocal:\n"), 0644)
}

func (config *Config) ListAgents() ([]string, error) {
	dir, err := os.Open(filepath.Join(config.WorkDir, config.AgentsDir))
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	entries, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	return entries, nil
}
