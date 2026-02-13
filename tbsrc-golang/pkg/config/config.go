package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration for the tbsrc-golang trader.
type Config struct {
	ORS      ORSConfig      `yaml:"ors"`
	Strategy StrategyConfig `yaml:"strategy"`
	System   SystemConfig   `yaml:"system"`
}

// StrategyConfig holds strategy-level parameters.
type StrategyConfig struct {
	StrategyID int      `yaml:"strategy_id"`
	Symbols    []string `yaml:"symbols"`
}

// SystemConfig holds system-level parameters.
type SystemConfig struct {
	LogLevel string `yaml:"log_level"`
}

// Load reads a YAML config file and returns a Config.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("config: read %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config: parse %s: %w", path, err)
	}

	if err := cfg.validate(); err != nil {
		return nil, fmt.Errorf("config: validate: %w", err)
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.ORS.MDShmKey == 0 {
		return fmt.Errorf("ors.md_shm_key is required")
	}
	if c.ORS.ReqShmKey == 0 {
		return fmt.Errorf("ors.req_shm_key is required")
	}
	if c.ORS.RespShmKey == 0 {
		return fmt.Errorf("ors.resp_shm_key is required")
	}
	if c.ORS.ClientStoreShmKey == 0 {
		return fmt.Errorf("ors.client_store_shm_key is required")
	}
	return nil
}
