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
	StrategyID  int                          `yaml:"strategy_id"`
	Account     string                       `yaml:"account"`
	Product     string                       `yaml:"product"`
	Symbols     []string                     `yaml:"symbols"`
	Instruments map[string]InstrumentConfig   `yaml:"instruments"`
	Thresholds  map[string]map[string]float64 `yaml:"thresholds"`
	ExchCosts   ExchangeCostsConfig          `yaml:"exchange_costs"`
}

// InstrumentConfig holds per-instrument configuration.
type InstrumentConfig struct {
	Exchange        string  `yaml:"exchange"`
	TickSize        float64 `yaml:"tick_size"`
	LotSize         float64 `yaml:"lot_size"`
	ContractFactor  float64 `yaml:"contract_factor"`
	PriceMultiplier float64 `yaml:"price_multiplier"`
	PriceFactor     float64 `yaml:"price_factor"`
	SendInLots      bool    `yaml:"send_in_lots"`
	Token           int32   `yaml:"token"`
	ExpiryDate      int32   `yaml:"expiry_date"`
}

// ExchangeCostsConfig holds exchange transaction cost rates.
type ExchangeCostsConfig struct {
	BuyExchTx         float64 `yaml:"buy_exch_tx"`
	SellExchTx        float64 `yaml:"sell_exch_tx"`
	BuyExchContractTx float64 `yaml:"buy_exch_contract_tx"`
	SellExchContractTx float64 `yaml:"sell_exch_contract_tx"`
}

// SystemConfig holds system-level parameters.
type SystemConfig struct {
	LogLevel string `yaml:"log_level"`
	APIPort  int    `yaml:"api_port"` // Web UI / REST API 端口，默认 9201
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
