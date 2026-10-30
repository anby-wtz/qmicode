package modelconfig

type ModelConfig struct {
	BaseURL string `yaml:"baseurl" json:"baseurl"`
	ApiKey  string `yaml:"apikey" json:"apikey"`
	Model   string `yaml:"model" json:"model"`

	Options struct {
		ReasoningEffort string                 `yaml:"reasoning_effort" json:"reasoning_effort"`
		ExtraBody       map[string]interface{} `yaml:"extra_body" json:"extra_body"`
	} `yaml:"options" json:"options"`
}
