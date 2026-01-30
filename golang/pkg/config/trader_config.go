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
	Strategy  StrategyConfig  `yaml:"strategy"`            // 单策略配置（向后兼容）
	Strategies []StrategyItemConfig `yaml:"strategies,omitempty"` // 多策略配置（新增）
	Session   SessionConfig   `yaml:"session"`
	Risk      RiskConfig      `yaml:"risk"`
	Engine    EngineConfig    `yaml:"engine"`
	Portfolio PortfolioConfig `yaml:"portfolio"`
	API       APIConfig       `yaml:"api"`
	Logging   LoggingConfig   `yaml:"logging"`
}

// StrategyItemConfig 单个策略的配置（用于多策略模式）
type StrategyItemConfig struct {
	ID              string                 `yaml:"id"`                // 策略唯一标识
	Type            string                 `yaml:"type"`              // 策略类型
	Enabled         bool                   `yaml:"enabled"`           // 是否启用
	Symbols         []string               `yaml:"symbols"`           // 交易品种
	Exchanges       []string               `yaml:"exchanges"`         // 交易所
	Allocation      float64                `yaml:"allocation"`        // 资金分配比例 (0-1)
	MaxPositionSize int64                  `yaml:"max_position_size"` // 最大持仓
	MaxExposure     float64                `yaml:"max_exposure"`      // 最大风险敞口
	Parameters      map[string]interface{} `yaml:"parameters"`        // 策略参数
	ModelFile       string                 `yaml:"model_file"`        // 模型文件路径
	HotReload       HotReloadConfig        `yaml:"hot_reload"`        // 热加载配置
}

// SystemConfig contains system-level configuration
type SystemConfig struct {
	StrategyID    string `yaml:"strategy_id"`     // 单策略模式的策略ID（向后兼容）
	Mode          string `yaml:"mode"`            // live, backtest, simulation
	MultiStrategy bool   `yaml:"multi_strategy"`  // 是否启用多策略模式（自动检测）
}

// StrategyConfig contains strategy-specific configuration
type StrategyConfig struct {
	Type            string                 `yaml:"type"` // passive, aggressive, hedging, pairwise_arb
	Symbols         []string               `yaml:"symbols"`
	Exchanges       []string               `yaml:"exchanges"`
	MaxPositionSize int64                  `yaml:"max_position_size"`
	MaxExposure     float64                `yaml:"max_exposure"`
	Parameters      map[string]interface{} `yaml:"parameters"`

	// Model hot reload configuration
	ModelFile string           `yaml:"model_file"` // Path to model file for hot reload
	HotReload HotReloadConfig  `yaml:"hot_reload"` // Hot reload settings
}

// HotReloadConfig contains model hot reload configuration
type HotReloadConfig struct {
	Enabled bool `yaml:"enabled"` // Enable manual hot reload via API
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
	CounterBridgeAddr   string        `yaml:"counter_bridge_addr"`
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
	// Validate system mode
	if c.System.Mode == "" {
		c.System.Mode = "simulation" // default
	}
	if c.System.Mode != "live" && c.System.Mode != "backtest" && c.System.Mode != "simulation" {
		return fmt.Errorf("system.mode must be 'live', 'backtest', or 'simulation'")
	}

	// 检测配置模式：多策略 or 单策略
	if len(c.Strategies) > 0 {
		// 多策略模式
		c.System.MultiStrategy = true
		if err := c.validateMultiStrategy(); err != nil {
			return err
		}
	} else {
		// 单策略模式（向后兼容）
		c.System.MultiStrategy = false
		if err := c.validateSingleStrategy(); err != nil {
			return err
		}
	}

	// Set engine defaults
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

	// Set logging defaults
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

// validateSingleStrategy 验证单策略配置（向后兼容）
func (c *TraderConfig) validateSingleStrategy() error {
	if c.System.StrategyID == "" {
		return fmt.Errorf("system.strategy_id is required")
	}

	if c.Strategy.Type == "" {
		return fmt.Errorf("strategy.type is required")
	}

	if err := validateStrategyType(c.Strategy.Type); err != nil {
		return err
	}

	if len(c.Strategy.Symbols) == 0 {
		return fmt.Errorf("strategy.symbols cannot be empty")
	}

	// Hedging and pairwise_arb strategies require at least 2 symbols
	if (c.Strategy.Type == "hedging" || c.Strategy.Type == "pairwise_arb") && len(c.Strategy.Symbols) < 2 {
		return fmt.Errorf("%s strategy requires at least 2 symbols", c.Strategy.Type)
	}

	return nil
}

// validateMultiStrategy 验证多策略配置
func (c *TraderConfig) validateMultiStrategy() error {
	if len(c.Strategies) == 0 {
		return fmt.Errorf("strategies cannot be empty in multi-strategy mode")
	}

	ids := make(map[string]bool)
	totalAllocation := 0.0
	hasEnabled := false

	for i, s := range c.Strategies {
		// 验证 ID 唯一性
		if s.ID == "" {
			return fmt.Errorf("strategies[%d].id is required", i)
		}
		if ids[s.ID] {
			return fmt.Errorf("duplicate strategy id: %s", s.ID)
		}
		ids[s.ID] = true

		// 验证策略类型
		if s.Type == "" {
			return fmt.Errorf("strategies[%d].type is required", i)
		}
		if err := validateStrategyType(s.Type); err != nil {
			return fmt.Errorf("strategies[%d]: %w", i, err)
		}

		// 验证品种
		if len(s.Symbols) == 0 {
			return fmt.Errorf("strategies[%d].symbols cannot be empty", i)
		}

		// 验证需要多品种的策略
		if (s.Type == "hedging" || s.Type == "pairwise_arb") && len(s.Symbols) < 2 {
			return fmt.Errorf("strategies[%d]: %s strategy requires at least 2 symbols", i, s.Type)
		}

		// 统计分配比例
		if s.Enabled {
			hasEnabled = true
			totalAllocation += s.Allocation
		}

		// 设置默认值
		if c.Strategies[i].Allocation == 0 && c.Strategies[i].Enabled {
			c.Strategies[i].Allocation = 1.0 / float64(len(c.Strategies)) // 默认平均分配
		}
	}

	// 至少有一个启用的策略
	if !hasEnabled {
		return fmt.Errorf("at least one strategy must be enabled")
	}

	// 分配比例警告（允许超过1.0，但记录警告）
	if totalAllocation > 1.1 {
		// 只是警告，不阻止
		fmt.Printf("[Config] Warning: total allocation %.2f exceeds 1.0\n", totalAllocation)
	}

	return nil
}

// validateStrategyType 验证策略类型
func validateStrategyType(strategyType string) error {
	validTypes := []string{"passive", "aggressive", "hedging", "pairwise_arb", "trend_following", "grid", "vwap"}
	for _, t := range validTypes {
		if strategyType == t {
			return nil
		}
	}
	return fmt.Errorf("strategy.type must be one of: %v", validTypes)
}

// GetStrategyConfigs 获取所有策略配置（统一接口）
// 单策略模式会自动转换为多策略格式
func (c *TraderConfig) GetStrategyConfigs() []StrategyItemConfig {
	if c.System.MultiStrategy {
		return c.Strategies
	}

	// 单策略模式：转换为多策略格式
	return []StrategyItemConfig{
		{
			ID:              c.System.StrategyID,
			Type:            c.Strategy.Type,
			Enabled:         true,
			Symbols:         c.Strategy.Symbols,
			Exchanges:       c.Strategy.Exchanges,
			Allocation:      1.0,
			MaxPositionSize: c.Strategy.MaxPositionSize,
			MaxExposure:     c.Strategy.MaxExposure,
			Parameters:      c.Strategy.Parameters,
			ModelFile:       c.Strategy.ModelFile,
			HotReload:       c.Strategy.HotReload,
		},
	}
}

// GetEnabledStrategies 获取所有启用的策略配置
func (c *TraderConfig) GetEnabledStrategies() []StrategyItemConfig {
	all := c.GetStrategyConfigs()
	enabled := make([]StrategyItemConfig, 0, len(all))
	for _, s := range all {
		if s.Enabled {
			enabled = append(enabled, s)
		}
	}
	return enabled
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
