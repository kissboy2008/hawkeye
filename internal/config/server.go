package config

import (
	"os"

	yaml "gopkg.in/yaml.v3"
)

// ServerConfig is the YAML configuration for the core server.
type ServerConfig struct {
	Server struct {
		Listen      string   `yaml:"listen"`       // ":18325"
		Mode        string   `yaml:"mode"`         // "release" or "debug"
		CORSOrigins []string `yaml:"cors_origins"` // ["*"] or ["https://example.com"]
	} `yaml:"server"`

	Database struct {
		Path               string `yaml:"path"`                 // "./data/monitoring.db"
		RetentionDays      int    `yaml:"retention_days"`       // 7
		HourlyRetentionDays int   `yaml:"hourly_retention_days"` // 30
	} `yaml:"database"`

	Poller struct {
		IntervalS       int `yaml:"interval_s"`        // 30 (heartbeat check interval)
		TimeoutS        int `yaml:"timeout_s"`         // 10 (kept for backward compat)
		OfflineTimeoutS int `yaml:"offline_timeout_s"` // 90 (time before marking agent offline)
	} `yaml:"poller"`

	Alerts struct {
		WechatWebhook   string `yaml:"wechat_webhook"`    // global default webhook
		CheckIntervalS  int    `yaml:"check_interval_s"`  // 30
	} `yaml:"alerts"`

	Probes struct {
		CheckIntervalS int `yaml:"check_interval_s"` // 60
	} `yaml:"probes"`

	Auth struct {
		Token string `yaml:"token"` // optional, leave empty for no auth
	} `yaml:"auth"`

	Backup struct {
		Enabled    bool   `yaml:"enabled"`     // true to enable daily backup
		Dir        string `yaml:"dir"`         // backup directory, default "./data/backups"
		MaxKeep    int    `yaml:"max_keep"`    // max number of backups to keep, default 7
		IntervalH  int    `yaml:"interval_h"`  // backup interval in hours, default 24
	} `yaml:"backup"`
}

func LoadServerConfig(path string) (*ServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg ServerConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	setServerDefaults(&cfg)
	return &cfg, nil
}

func setServerDefaults(cfg *ServerConfig) {
	if cfg.Server.Listen == "" {
		cfg.Server.Listen = ":18325"
	}
	if cfg.Server.Mode == "" {
		cfg.Server.Mode = "release"
	}
	if cfg.Database.Path == "" {
		cfg.Database.Path = "./data/monitoring.db"
	}
	if cfg.Database.RetentionDays == 0 {
		cfg.Database.RetentionDays = 7
	}
	if cfg.Database.HourlyRetentionDays == 0 {
		cfg.Database.HourlyRetentionDays = 30
	}
	if cfg.Poller.IntervalS == 0 {
		cfg.Poller.IntervalS = 30
	}
	if cfg.Poller.TimeoutS == 0 {
		cfg.Poller.TimeoutS = 10
	}
	if cfg.Poller.OfflineTimeoutS == 0 {
		cfg.Poller.OfflineTimeoutS = 90
	}
	if cfg.Alerts.CheckIntervalS == 0 {
		cfg.Alerts.CheckIntervalS = 30
	}
	if cfg.Probes.CheckIntervalS == 0 {
		cfg.Probes.CheckIntervalS = 60
	}
	if len(cfg.Server.CORSOrigins) == 0 {
		cfg.Server.CORSOrigins = []string{"*"}
	}
}
