package backtest

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// BacktestConfig represents the backtest configuration
type BacktestConfig struct {
	Backtest BacktestSettings `yaml:"backtest"`
	Strategy StrategySettings `yaml:"strategy"`
	Engine   EngineSettings   `yaml:"engine"`
}

// BacktestSettings contains backtest-specific settings
type BacktestSettings struct {
	Name      string        `yaml:"name"`
	Mode      string        `yaml:"mode"`
	StartDate string        `yaml:"start_date"`
	EndDate   string        `yaml:"end_date"`
	StartTime string        `yaml:"start_time"`
	EndTime   string        `yaml:"end_time"`
	Data      DataSettings  `yaml:"data"`
	Replay    ReplaySettings `yaml:"replay"`
	Initial   InitialSettings `yaml:"initial"`
	OrderSim  OrderSimSettings `yaml:"order_simulation"`
	Output    OutputSettings `yaml:"output"`
}

// DataSettings contains data source settings
type DataSettings struct {
	SourceType string   `yaml:"source_type"` // csv, parquet, database
	DataPath   string   `yaml:"data_path"`
	Symbols    []string `yaml:"symbols"`
}

// ReplaySettings contains replay behavior settings
type ReplaySettings struct {
	Mode  string  `yaml:"mode"`  // realtime, fast, instant
	Speed float64 `yaml:"speed"` // multiplier for fast mode
}

// InitialSettings contains initial capital settings
type InitialSettings struct {
	Capital float64 `yaml:"capital"`
}

// OrderSimSettings contains order simulation settings
type OrderSimSettings struct {
	FillDelayMs    int     `yaml:"fill_delay_ms"`
	SlippageBps    float64 `yaml:"slippage_bps"`
	CommissionRate float64 `yaml:"commission_rate"`
}

// OutputSettings contains output settings
type OutputSettings struct {
	ResultDir      string `yaml:"result_dir"`
	SaveTrades     bool   `yaml:"save_trades"`
	SaveDailyPNL   bool   `yaml:"save_daily_pnl"`
	GenerateReport bool   `yaml:"generate_report"`
	ReportFormat   string `yaml:"report_format"` // markdown, json, html
}

// StrategySettings contains strategy configuration
type StrategySettings struct {
	Type       string                 `yaml:"type"`
	Symbols    []string               `yaml:"symbols"`
	Parameters map[string]interface{} `yaml:"parameters"`
}

// EngineSettings contains engine configuration
type EngineSettings struct {
	NATSAddr string `yaml:"nats_addr"`
}

// LoadBacktestConfig loads backtest configuration from YAML file
func LoadBacktestConfig(configFile string) (*BacktestConfig, error) {
	data, err := os.ReadFile(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config BacktestConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Validate config
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return &config, nil
}

// Validate validates the configuration
func (c *BacktestConfig) Validate() error {
	// Validate dates
	if c.Backtest.StartDate == "" {
		return fmt.Errorf("start_date is required")
	}
	if c.Backtest.EndDate == "" {
		return fmt.Errorf("end_date is required")
	}

	// Parse dates to ensure they're valid
	startDate, err := time.Parse("2006-01-02", c.Backtest.StartDate)
	if err != nil {
		return fmt.Errorf("invalid start_date format (expected YYYY-MM-DD): %w", err)
	}
	endDate, err := time.Parse("2006-01-02", c.Backtest.EndDate)
	if err != nil {
		return fmt.Errorf("invalid end_date format (expected YYYY-MM-DD): %w", err)
	}

	if endDate.Before(startDate) {
		return fmt.Errorf("end_date must be after start_date")
	}

	// Validate data settings
	if c.Backtest.Data.DataPath == "" {
		return fmt.Errorf("data_path is required")
	}
	if len(c.Backtest.Data.Symbols) == 0 {
		return fmt.Errorf("at least one symbol is required")
	}

	// Validate replay mode
	switch c.Backtest.Replay.Mode {
	case "realtime", "fast", "instant":
		// Valid
	default:
		return fmt.Errorf("invalid replay mode: %s (must be realtime, fast, or instant)", c.Backtest.Replay.Mode)
	}

	// Validate initial capital
	if c.Backtest.Initial.Capital <= 0 {
		return fmt.Errorf("initial capital must be positive")
	}

	// Validate strategy
	if c.Strategy.Type == "" {
		return fmt.Errorf("strategy type is required")
	}
	if len(c.Strategy.Symbols) == 0 {
		return fmt.Errorf("strategy symbols are required")
	}

	// Validate NATS address
	if c.Engine.NATSAddr == "" {
		return fmt.Errorf("NATS address is required")
	}

	return nil
}

// GetReplayMode returns the ReplayMode based on configuration
func (c *BacktestConfig) GetReplayMode() ReplayMode {
	switch c.Backtest.Replay.Mode {
	case "realtime":
		return ReplayModeRealtime
	case "fast":
		return ReplayModeFast
	case "instant":
		return ReplayModeInstant
	default:
		return ReplayModeFast // Default
	}
}

// GetReplaySpeed returns the replay speed multiplier
func (c *BacktestConfig) GetReplaySpeed() float64 {
	if c.Backtest.Replay.Speed <= 0 {
		return 10.0 // Default 10x
	}
	return c.Backtest.Replay.Speed
}

// GetFillDelay returns the fill delay duration
func (c *BacktestConfig) GetFillDelay() time.Duration {
	if c.Backtest.OrderSim.FillDelayMs <= 0 {
		return 10 * time.Millisecond // Default 10ms
	}
	return time.Duration(c.Backtest.OrderSim.FillDelayMs) * time.Millisecond
}

// GetSlippage returns the slippage in basis points
func (c *BacktestConfig) GetSlippage() float64 {
	return c.Backtest.OrderSim.SlippageBps
}

// GetCommissionRate returns the commission rate
func (c *BacktestConfig) GetCommissionRate() float64 {
	if c.Backtest.OrderSim.CommissionRate <= 0 {
		return 0.0003 // Default 0.03%
	}
	return c.Backtest.OrderSim.CommissionRate
}
