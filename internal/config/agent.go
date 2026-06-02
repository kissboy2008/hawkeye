package config

import (
	"os"

	yaml "gopkg.in/yaml.v3"
)

// AgentConfig is the YAML configuration for the agent.
type AgentConfig struct {
	Server struct {
		Listen        string `yaml:"listen"`          // ":32518"
		URL           string `yaml:"url"`             // "https://icloud325.cn:18325" (push target)
		PushIntervalS int    `yaml:"push_interval_s"` // 30
	} `yaml:"server"`

	Auth struct {
		Token string `yaml:"token"` // optional bearer token
	} `yaml:"auth"`

	Collect struct {
		NetworkInterfaces []string `yaml:"network_interfaces"` // empty = all
	} `yaml:"collect"`
}

func LoadAgentConfig(path string) (*AgentConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg AgentConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	setAgentDefaults(&cfg)
	return &cfg, nil
}

func setAgentDefaults(cfg *AgentConfig) {
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = ":32518"
	}
	if cfg.Server.PushIntervalS <= 0 {
		cfg.Server.PushIntervalS = 30
	}
}
