package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// TraderConfig is the complete configuration for the trader
type TraderConfig struct {
	System    SystemConfig    `yaml:"system"`
	Strategy  StrategyConfig  `yaml:"strategy"`
	Session   SessionConfig   `yaml:"session"`
	Risk      RiskConfig      `yaml:"risk"`
	Engine    EngineConfig    `yaml:"engine"`
	Portfolio PortfolioConfig `yaml:"portfolio"`
	API       APIConfig       `yaml:"api"`
	Logging   LoggingConfig   `yaml:"logging"`
}

// SystemConfig contains system-level configuration
type SystemConfig struct {
	StrategyID string `yaml:"strategy_id"`
	Mode       string `yaml:"mode"` // live, backtest, simulation
}

// StrategyConfig contains strategy-specific configuration
type StrategyConfig struct {
	Type            string                 `yaml:"type"` // passive, aggressive, hedging, pairwise_arb
	Symbols         []string               `yaml:"symbols"`
	Exchanges       []string               `yaml:"exchanges"`
	MaxPositionSize int64                  `yaml:"max_position_size"`
	MaxExposure     float64                `yaml:"max_exposure"`
	Parameters      map[string]interface{} `yaml:"parameters"`
}

// SessionConfig contains trading session configuration
type SessionConfig struct {
	StartTime    string `yaml:"start_time"`    // HH:MM:SS
	EndTime      string `yaml:"end_time"`      // HH:MM:SS
	Timezone     string `yaml:"timezone"`      // e.g., "Asia/Shanghai"
	AutoStart    bool   `yaml:"auto_start"`    // Auto-start session manager
	AutoStop     bool   `yaml:"auto_stop"`     // Auto-stop at end time
	AutoActivate bool   `yaml:"auto_activate"` // Auto-activate strategy (if false, wait for manual activation)
}

// RiskConfig contains risk management configuration
type RiskConfig struct {
	MaxDrawdown     float64 `yaml:"max_drawdown"`
	StopLoss        float64 `yaml:"stop_loss"`
	MaxLoss         float64 `yaml:"max_loss"`
	DailyLossLimit  float64 `yaml:"daily_loss_limit"`
	MaxRejectCount  int     `yaml:"max_reject_count"`
	CheckIntervalMs int64   `yaml:"check_interval_ms"`
}

// EngineConfig contains strategy engine configuration
type EngineConfig struct {
	ORSGatewayAddr      string        `yaml:"ors_gateway_addr"`
	NATSAddr            string        `yaml:"nats_addr"`
	OrderQueueSize      int           `yaml:"order_queue_size"`
	TimerInterval       time.Duration `yaml:"timer_interval"`
	MaxConcurrentOrders int           `yaml:"max_concurrent_orders"`
}

// PortfolioConfig contains portfolio management configuration
type PortfolioConfig struct {
	TotalCapital         float64            `yaml:"total_capital"`
	StrategyAllocation   map[string]float64 `yaml:"strategy_allocation"`
	RebalanceIntervalSec int                `yaml:"rebalance_interval_sec"`
	MinAllocation        float64            `yaml:"min_allocation"`
	MaxAllocation        float64            `yaml:"max_allocation"`
	EnableAutoRebalance  bool               `yaml:"enable_auto_rebalance"`
	EnableCorrelation    bool               `yaml:"enable_correlation_calc"`
}

// APIConfig contains HTTP REST API configuration
type APIConfig struct {
	Enabled bool   `yaml:"enabled"` // enable HTTP API server
	Port    int    `yaml:"port"`    // API server port
	Host    string `yaml:"host"`    // API server host (default: localhost)
}

// LoggingConfig contains logging configuration
type LoggingConfig struct {
	Level       string `yaml:"level"`        // debug, info, warn, error
	File        string `yaml:"file"`         // log file path
	MaxSizeMB   int    `yaml:"max_size_mb"`  // max size before rotation
	MaxBackups  int    `yaml:"max_backups"`  // max number of old log files
	MaxAgeDays  int    `yaml:"max_age_days"` // max age of old log files
	Compress    bool   `yaml:"compress"`     // compress rotated files
	Console     bool   `yaml:"console"`      // also log to console
	JSONFormat  bool   `yaml:"json_format"`  // use JSON format
}

// LoadTraderConfig loads configuration from a YAML file
func LoadTraderConfig(filepath string) (*TraderConfig, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config TraderConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Validate configuration
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// Validate validates the configuration
func (c *TraderConfig) Validate() error {
	// Validate system config
	if c.System.StrategyID == "" {
		return fmt.Errorf("system.strategy_id is required")
	}
	if c.System.Mode == "" {
		c.System.Mode = "simulation" // default
	}
	if c.System.Mode != "live" && c.System.Mode != "backtest" && c.System.Mode != "simulation" {
		return fmt.Errorf("system.mode must be 'live', 'backtest', or 'simulation'")
	}

	// Validate strategy config
	if c.Strategy.Type == "" {
		return fmt.Errorf("strategy.type is required")
	}
	validTypes := []string{"passive", "aggressive", "hedging", "pairwise_arb"}
	validType := false
	for _, t := range validTypes {
		if c.Strategy.Type == t {
			validType = true
			break
		}
	}
	if !validType {
		return fmt.Errorf("strategy.type must be one of: passive, aggressive, hedging, pairwise_arb")
	}

	if len(c.Strategy.Symbols) == 0 {
		return fmt.Errorf("strategy.symbols cannot be empty")
	}

	// Hedging and pairwise_arb strategies require at least 2 symbols
	if (c.Strategy.Type == "hedging" || c.Strategy.Type == "pairwise_arb") && len(c.Strategy.Symbols) < 2 {
		return fmt.Errorf("%s strategy requires at least 2 symbols", c.Strategy.Type)
	}

	// Set defaults
	if c.Engine.OrderQueueSize == 0 {
		c.Engine.OrderQueueSize = 100
	}
	if c.Engine.TimerInterval == 0 {
		c.Engine.TimerInterval = 5 * time.Second
	}
	if c.Engine.MaxConcurrentOrders == 0 {
		c.Engine.MaxConcurrentOrders = 10
	}

	if c.Risk.CheckIntervalMs == 0 {
		c.Risk.CheckIntervalMs = 100
	}

	if c.Logging.Level == "" {
		c.Logging.Level = "info"
	}
	if c.Logging.MaxSizeMB == 0 {
		c.Logging.MaxSizeMB = 100
	}
	if c.Logging.MaxBackups == 0 {
		c.Logging.MaxBackups = 10
	}
	if c.Logging.MaxAgeDays == 0 {
		c.Logging.MaxAgeDays = 30
	}

	return nil
}

// SaveTraderConfig saves configuration to a YAML file
func SaveTraderConfig(filepath string, config *TraderConfig) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
