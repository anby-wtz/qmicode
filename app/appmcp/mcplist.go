package appmcp

import (
	"os"

	"go.yaml.in/yaml/v3"
)

/*
mcp config via yaml file

remote:
  - name: test
    endpoint: https://example.com/mcp
local:
  - name: test
    cmd: ./mymcp
    args:
	 - arg1
	 - arg2

*/

type McpListConfig struct {
	Remote []struct {
		Name     string `yaml:"name"`
		Endpoint string `yaml:"endpoint"`
	} `yaml:"remote"`
	Local []struct {
		Name string   `yaml:"name"`
		Cmd  string   `yaml:"cmd"`
		Args []string `yaml:"args"`
	} `yaml:"local"`
}

func GetMcpListConfigFromFile(file string) (*McpListConfig, error) {
	var config McpListConfig
	fileContent, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	err = yaml.Unmarshal(fileContent, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
