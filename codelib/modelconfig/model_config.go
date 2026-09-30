package modelconfig

type ModelConfig struct {
	BaseURL string `yaml:"baseurl" json:"baseurl"`
	ApiKey  string `yaml:"apikey" json:"apikey"`
	Model   string `yaml:"model" json:"model"`
}
