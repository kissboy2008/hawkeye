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
		// CPUSampleMS 是 cpu.Percent 的采样窗口（毫秒），默认 1000。
		// 每次采集会阻塞这么久，容器/低配机器上可以调小到 200 换取更低开销。
		CPUSampleMS int `yaml:"cpu_sample_ms"`
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
