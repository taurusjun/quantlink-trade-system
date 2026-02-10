// Package risk provides risk management functionality
package risk

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
)

// RiskLimitType represents different types of risk limits
type RiskLimitType int

const (
	RiskLimitPositionSize RiskLimitType = iota
	RiskLimitExposure
	RiskLimitDrawdown
	RiskLimitLoss
	RiskLimitDailyLoss
	RiskLimitOrderRate
)

// RiskLimit represents a risk limit configuration
type RiskLimit struct {
	Type        RiskLimitType
	Level       string  // "strategy", "portfolio", "global"
	TargetID    string  // strategy_id or "*" for global
	Value       float64 // Limit value
	Enabled     bool
	Description string
}

// RiskAlert represents a risk alert
type RiskAlert struct {
	Timestamp   time.Time
	Level       string // "warning", "critical"
	Type        RiskLimitType
	TargetID    string
	Message     string
	CurrentValue float64
	LimitValue   float64
	Action      string // "throttle", "stop", "emergency_stop"
}

// RiskManagerConfig represents risk manager configuration
// 对应 C++: ThresholdSet 中的风控参数
type RiskManagerConfig struct {
	EnableGlobalLimits      bool
	EnableStrategyLimits    bool
	EnablePortfolioLimits   bool
	AlertRetentionSeconds   int
	MaxAlertQueueSize       int
	EmergencyStopThreshold  int   // Number of critical alerts to trigger emergency stop
	CheckIntervalMs         int64

	// === 风控限制参数 (对应 C++ ThresholdSet) ===
	// 策略级别
	MaxPosition    int64   `yaml:"max_position"`    // C++: MAX_SIZE - 最大持仓
	MaxOrders      int64   `yaml:"max_orders"`      // C++: MAX_ORDERS / m_maxOrderCount - 最大订单数
	MaxTradedQty   float64 `yaml:"max_traded_qty"`  // C++: m_maxTradedQty - 最大成交量
	StopLoss       float64 `yaml:"stop_loss"`       // C++: STOP_LOSS - 止损（触发暂停）
	MaxLoss        float64 `yaml:"max_loss"`        // C++: MAX_LOSS - 最大亏损（触发退出）
	UpnlLoss       float64 `yaml:"upnl_loss"`       // C++: UPNL_LOSS - 未实现盈亏限制
	MaxExposure    float64 `yaml:"max_exposure"`    // 最大敞口

	// 全局级别
	GlobalMaxExposure  float64 `yaml:"global_max_exposure"`   // 全局最大敞口
	GlobalMaxDrawdown  float64 `yaml:"global_max_drawdown"`   // 全局最大回撤
	GlobalMaxDailyLoss float64 `yaml:"global_max_daily_loss"` // 全局每日最大亏损

	// 止损恢复时间 (对应 C++: 15 mins 后恢复)
	StopLossRecoverySeconds int64 `yaml:"stop_loss_recovery_seconds"` // C++: 900秒 (15分钟)
}

// RiskManager manages risk limits and alerts
type RiskManager struct {
	config      *RiskManagerConfig
	limits      map[string]*RiskLimit // limit_id -> RiskLimit
	alerts      []*RiskAlert
	alertQueue  chan *RiskAlert

	// Global statistics
	globalStats struct {
		TotalExposure    float64
		TotalPnL         float64
		TotalDrawdown    float64
		DailyPnL         float64
		OrderCount       int64
		LastResetTime    time.Time
	}

	// Emergency stop state
	emergencyStop   bool
	criticalAlerts  int

	mu              sync.RWMutex
	stopChan        chan struct{}
	wg              sync.WaitGroup
	isRunning       bool
}

// NewRiskManager creates a new risk manager
func NewRiskManager(config *RiskManagerConfig) *RiskManager {
	if config == nil {
		// 默认配置 - 与 C++ ThresholdSet 默认值对应
		// C++ 默认值设置为极大值表示禁用，Go 这里设置合理的默认值
		config = &RiskManagerConfig{
			EnableGlobalLimits:     true,
			EnableStrategyLimits:   true,
			EnablePortfolioLimits:  true,
			AlertRetentionSeconds:  3600,
			MaxAlertQueueSize:      1000,
			EmergencyStopThreshold: 3,
			CheckIntervalMs:        100,

			// 策略级别默认值
			MaxPosition:    100,       // 默认最大持仓 100 手
			MaxOrders:      1000,      // 默认最大订单数
			MaxTradedQty:   10000,     // 默认最大成交量
			StopLoss:       10000,     // 默认止损 1万元（触发暂停）
			MaxLoss:        50000,     // 默认最大亏损 5万元（触发退出）
			UpnlLoss:       20000,     // 默认未实现盈亏 2万元
			MaxExposure:    1000000,   // 默认最大敞口 100万元

			// 全局级别默认值
			GlobalMaxExposure:  10000000, // 全局最大敞口 1000万元
			GlobalMaxDrawdown:  100000,   // 全局最大回撤 10万元
			GlobalMaxDailyLoss: 50000,    // 全局每日最大亏损 5万元

			// 止损恢复时间
			StopLossRecoverySeconds: 900, // C++: 15分钟 = 900秒
		}
	}

	return &RiskManager{
		config:     config,
		limits:     make(map[string]*RiskLimit),
		alerts:     make([]*RiskAlert, 0),
		alertQueue: make(chan *RiskAlert, config.MaxAlertQueueSize),
		stopChan:   make(chan struct{}),
	}
}

// Initialize initializes the risk manager
func (rm *RiskManager) Initialize() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Set default global limits
	rm.addDefaultLimits()

	// Initialize global stats
	rm.globalStats.LastResetTime = time.Now()

	log.Println("[RiskManager] Initialized")
	return nil
}

// addDefaultLimits adds default risk limits from config
// 对应 C++: ExecutionStrategy 中对 ThresholdSet 参数的使用
func (rm *RiskManager) addDefaultLimits() {
	// === Global limits (全局风控) ===
	rm.limits["global_max_exposure"] = &RiskLimit{
		Type:        RiskLimitExposure,
		Level:       "global",
		TargetID:    "*",
		Value:       rm.config.GlobalMaxExposure,
		Enabled:     true,
		Description: "Global maximum exposure",
	}

	rm.limits["global_max_drawdown"] = &RiskLimit{
		Type:        RiskLimitDrawdown,
		Level:       "global",
		TargetID:    "*",
		Value:       rm.config.GlobalMaxDrawdown,
		Enabled:     true,
		Description: "Global maximum drawdown",
	}

	rm.limits["global_max_daily_loss"] = &RiskLimit{
		Type:        RiskLimitDailyLoss,
		Level:       "global",
		TargetID:    "*",
		Value:       rm.config.GlobalMaxDailyLoss,
		Enabled:     true,
		Description: "Global maximum daily loss",
	}

	// === Strategy default limits (策略级风控) ===
	// C++: MAX_SIZE - 最大持仓限制
	rm.limits["strategy_default_position"] = &RiskLimit{
		Type:        RiskLimitPositionSize,
		Level:       "strategy",
		TargetID:    "*",
		Value:       float64(rm.config.MaxPosition),
		Enabled:     true,
		Description: "Default strategy position limit (C++: MAX_SIZE)",
	}

	// C++: Exposure - 敞口限制
	rm.limits["strategy_default_exposure"] = &RiskLimit{
		Type:        RiskLimitExposure,
		Level:       "strategy",
		TargetID:    "*",
		Value:       rm.config.MaxExposure,
		Enabled:     true,
		Description: "Default strategy exposure limit",
	}

	// C++: STOP_LOSS - 止损限制（触发平仓暂停）
	rm.limits["strategy_default_stop_loss"] = &RiskLimit{
		Type:        RiskLimitLoss,
		Level:       "strategy",
		TargetID:    "*",
		Value:       rm.config.StopLoss,
		Enabled:     true,
		Description: "Default strategy stop loss (C++: STOP_LOSS, triggers pause)",
	}

	// C++: MAX_LOSS - 最大亏损限制（触发退出）
	rm.limits["strategy_default_max_loss"] = &RiskLimit{
		Type:        RiskLimitLoss,
		Level:       "strategy",
		TargetID:    "*",
		Value:       rm.config.MaxLoss,
		Enabled:     true,
		Description: "Default strategy max loss (C++: MAX_LOSS, triggers exit)",
	}

	// C++: UPNL_LOSS - 未实现盈亏限制
	rm.limits["strategy_default_upnl_loss"] = &RiskLimit{
		Type:        RiskLimitLoss,
		Level:       "strategy",
		TargetID:    "*",
		Value:       rm.config.UpnlLoss,
		Enabled:     true,
		Description: "Default strategy unrealized PnL loss (C++: UPNL_LOSS)",
	}

	// C++: MAX_ORDERS / m_maxOrderCount - 最大订单数
	rm.limits["strategy_default_max_orders"] = &RiskLimit{
		Type:        RiskLimitOrderRate,
		Level:       "strategy",
		TargetID:    "*",
		Value:       float64(rm.config.MaxOrders),
		Enabled:     true,
		Description: "Default strategy max orders (C++: MAX_ORDERS)",
	}
}

// Start starts the risk manager
func (rm *RiskManager) Start() error {
	rm.mu.Lock()
	if rm.isRunning {
		rm.mu.Unlock()
		return fmt.Errorf("risk manager already running")
	}
	rm.isRunning = true
	rm.mu.Unlock()

	// Start alert processor
	rm.wg.Add(1)
	go rm.processAlerts()

	log.Println("[RiskManager] Started")
	return nil
}

// Stop stops the risk manager
func (rm *RiskManager) Stop() error {
	rm.mu.Lock()
	if !rm.isRunning {
		rm.mu.Unlock()
		return fmt.Errorf("risk manager not running")
	}
	rm.isRunning = false
	rm.mu.Unlock()

	close(rm.stopChan)
	rm.wg.Wait()

	log.Println("[RiskManager] Stopped")
	return nil
}

// CheckStrategy checks a strategy against risk limits
func (rm *RiskManager) CheckStrategy(s strategy.Strategy) []RiskAlert {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	alerts := make([]RiskAlert, 0)

	if !rm.config.EnableStrategyLimits {
		return alerts
	}

	position := s.GetEstimatedPosition() // Estimated position (NOT real CTP!)
	pnl := s.GetPNL()
	riskMetrics := s.GetRiskMetrics()

	// Check position size
	if limit, ok := rm.limits["strategy_default_position"]; ok && limit.Enabled {
		if float64(absInt64(position.NetQty)) > limit.Value {
			alerts = append(alerts, RiskAlert{
				Timestamp:    time.Now(),
				Level:        "warning",
				Type:         RiskLimitPositionSize,
				TargetID:     s.GetID(),
				Message:      fmt.Sprintf("Estimated position size %d exceeds limit %.0f", position.NetQty, limit.Value),
				CurrentValue: float64(absInt64(position.NetQty)),
				LimitValue:   limit.Value,
				Action:       "throttle",
			})
		}
	}

	// Check exposure
	if limit, ok := rm.limits["strategy_default_exposure"]; ok && limit.Enabled {
		if riskMetrics.ExposureValue > limit.Value {
			alerts = append(alerts, RiskAlert{
				Timestamp:    time.Now(),
				Level:        "warning",
				Type:         RiskLimitExposure,
				TargetID:     s.GetID(),
				Message:      fmt.Sprintf("Exposure %.2f exceeds limit %.2f", riskMetrics.ExposureValue, limit.Value),
				CurrentValue: riskMetrics.ExposureValue,
				LimitValue:   limit.Value,
				Action:       "throttle",
			})
		}
	}

	// Check STOP_LOSS - 止损限制（触发暂停，对应 C++: m_netPNL < STOP_LOSS * -1）
	if limit, ok := rm.limits["strategy_default_stop_loss"]; ok && limit.Enabled {
		if pnl.TotalPnL < 0 && absFloat(pnl.TotalPnL) > limit.Value {
			alerts = append(alerts, RiskAlert{
				Timestamp:    time.Now(),
				Level:        "critical",
				Type:         RiskLimitLoss,
				TargetID:     s.GetID(),
				Message:      fmt.Sprintf("Stop loss triggered: PnL %.2f exceeds limit %.2f (C++: STOP_LOSS)", pnl.TotalPnL, limit.Value),
				CurrentValue: absFloat(pnl.TotalPnL),
				LimitValue:   limit.Value,
				Action:       "stop", // 触发暂停，可恢复
			})
		}
	}

	// Check MAX_LOSS - 最大亏损限制（触发退出，对应 C++: m_netPNL < MAX_LOSS * -1）
	if limit, ok := rm.limits["strategy_default_max_loss"]; ok && limit.Enabled {
		if pnl.TotalPnL < 0 && absFloat(pnl.TotalPnL) > limit.Value {
			alerts = append(alerts, RiskAlert{
				Timestamp:    time.Now(),
				Level:        "critical",
				Type:         RiskLimitLoss,
				TargetID:     s.GetID(),
				Message:      fmt.Sprintf("Max loss triggered: PnL %.2f exceeds limit %.2f (C++: MAX_LOSS)", pnl.TotalPnL, limit.Value),
				CurrentValue: absFloat(pnl.TotalPnL),
				LimitValue:   limit.Value,
				Action:       "emergency_stop", // 触发退出，不可恢复
			})
		}
	}

	// Check UPNL_LOSS - 未实现盈亏限制（对应 C++: m_unrealisedPNL < UPNL_LOSS * -1）
	if limit, ok := rm.limits["strategy_default_upnl_loss"]; ok && limit.Enabled {
		if pnl.UnrealizedPnL < 0 && absFloat(pnl.UnrealizedPnL) > limit.Value {
			alerts = append(alerts, RiskAlert{
				Timestamp:    time.Now(),
				Level:        "warning",
				Type:         RiskLimitLoss,
				TargetID:     s.GetID(),
				Message:      fmt.Sprintf("Unrealized PnL loss: %.2f exceeds limit %.2f (C++: UPNL_LOSS)", pnl.UnrealizedPnL, limit.Value),
				CurrentValue: absFloat(pnl.UnrealizedPnL),
				LimitValue:   limit.Value,
				Action:       "throttle", // 触发警告
			})
		}
	}

	return alerts
}

// CheckGlobal checks global risk limits
func (rm *RiskManager) CheckGlobal(strategies map[string]strategy.Strategy) []RiskAlert {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	alerts := make([]RiskAlert, 0)

	if !rm.config.EnableGlobalLimits {
		return alerts
	}

	// Calculate global statistics
	var totalExposure float64
	var totalPnL float64
	var maxDrawdown float64

	for _, s := range strategies {
		riskMetrics := s.GetRiskMetrics()
		pnl := s.GetPNL()

		totalExposure += riskMetrics.ExposureValue
		totalPnL += pnl.TotalPnL

		if riskMetrics.MaxDrawdown > maxDrawdown {
			maxDrawdown = riskMetrics.MaxDrawdown
		}
	}

	rm.globalStats.TotalExposure = totalExposure
	rm.globalStats.TotalPnL = totalPnL
	rm.globalStats.TotalDrawdown = maxDrawdown

	// Check global exposure
	if limit, ok := rm.limits["global_max_exposure"]; ok && limit.Enabled {
		if totalExposure > limit.Value {
			alerts = append(alerts, RiskAlert{
				Timestamp:    time.Now(),
				Level:        "critical",
				Type:         RiskLimitExposure,
				TargetID:     "*",
				Message:      fmt.Sprintf("Global exposure %.2f exceeds limit %.2f", totalExposure, limit.Value),
				CurrentValue: totalExposure,
				LimitValue:   limit.Value,
				Action:       "emergency_stop",
			})
		}
	}

	// Check global drawdown
	if limit, ok := rm.limits["global_max_drawdown"]; ok && limit.Enabled {
		if maxDrawdown > limit.Value {
			alerts = append(alerts, RiskAlert{
				Timestamp:    time.Now(),
				Level:        "critical",
				Type:         RiskLimitDrawdown,
				TargetID:     "*",
				Message:      fmt.Sprintf("Global drawdown %.2f exceeds limit %.2f", maxDrawdown, limit.Value),
				CurrentValue: maxDrawdown,
				LimitValue:   limit.Value,
				Action:       "emergency_stop",
			})
		}
	}

	// Check daily loss
	if totalPnL < 0 {
		if limit, ok := rm.limits["global_max_daily_loss"]; ok && limit.Enabled {
			if absFloat(totalPnL) > limit.Value {
				alerts = append(alerts, RiskAlert{
					Timestamp:    time.Now(),
					Level:        "critical",
					Type:         RiskLimitDailyLoss,
					TargetID:     "*",
					Message:      fmt.Sprintf("Daily loss %.2f exceeds limit %.2f", totalPnL, limit.Value),
					CurrentValue: absFloat(totalPnL),
					LimitValue:   limit.Value,
					Action:       "emergency_stop",
				})
			}
		}
	}

	return alerts
}

// AddAlert adds an alert
func (rm *RiskManager) AddAlert(alert *RiskAlert) {
	select {
	case rm.alertQueue <- alert:
		// Track critical alerts
		if alert.Level == "critical" {
			rm.mu.Lock()
			rm.criticalAlerts++
			if rm.criticalAlerts >= rm.config.EmergencyStopThreshold {
				rm.emergencyStop = true
				log.Printf("[RiskManager] EMERGENCY STOP triggered! Critical alerts: %d", rm.criticalAlerts)
			}
			rm.mu.Unlock()
		}
	default:
		log.Println("[RiskManager] Alert queue full, dropping alert")
	}
}

// processAlerts processes alerts from the queue
func (rm *RiskManager) processAlerts() {
	defer rm.wg.Done()

	for {
		select {
		case alert := <-rm.alertQueue:
			rm.handleAlert(alert)
		case <-rm.stopChan:
			return
		}
	}
}

// handleAlert handles a single alert
func (rm *RiskManager) handleAlert(alert *RiskAlert) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// Store alert
	rm.alerts = append(rm.alerts, alert)

	// Trim old alerts
	retention := time.Duration(rm.config.AlertRetentionSeconds) * time.Second
	cutoff := time.Now().Add(-retention)

	validAlerts := make([]*RiskAlert, 0)
	for _, a := range rm.alerts {
		if a.Timestamp.After(cutoff) {
			validAlerts = append(validAlerts, a)
		}
	}
	rm.alerts = validAlerts

	// Log alert
	log.Printf("[RiskManager] %s ALERT: %s - %s (value=%.2f, limit=%.2f, action=%s)",
		alert.Level, alert.TargetID, alert.Message, alert.CurrentValue, alert.LimitValue, alert.Action)
}

// IsEmergencyStop returns true if emergency stop is active
func (rm *RiskManager) IsEmergencyStop() bool {
	rm.mu.RLock()
	defer rm.mu.RUnlock()
	return rm.emergencyStop
}

// GetAlerts returns recent alerts
func (rm *RiskManager) GetAlerts(level string, limit int) []*RiskAlert {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make([]*RiskAlert, 0)
	count := 0

	// Iterate from newest to oldest
	for i := len(rm.alerts) - 1; i >= 0 && count < limit; i-- {
		alert := rm.alerts[i]
		if level == "" || alert.Level == level {
			result = append(result, alert)
			count++
		}
	}

	return result
}

// GetGlobalStats returns global statistics
func (rm *RiskManager) GetGlobalStats() map[string]interface{} {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return map[string]interface{}{
		"total_exposure":  rm.globalStats.TotalExposure,
		"total_pnl":       rm.globalStats.TotalPnL,
		"total_drawdown":  rm.globalStats.TotalDrawdown,
		"daily_pnl":       rm.globalStats.DailyPnL,
		"order_count":     rm.globalStats.OrderCount,
		"emergency_stop":  rm.emergencyStop,
		"critical_alerts": rm.criticalAlerts,
	}
}

// ResetEmergencyStop resets emergency stop (use with caution!)
func (rm *RiskManager) ResetEmergencyStop() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.emergencyStop = false
	rm.criticalAlerts = 0
	log.Println("[RiskManager] Emergency stop reset")
}

// UpdateLimit updates a risk limit
func (rm *RiskManager) UpdateLimit(limitID string, value float64, enabled bool) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	limit, ok := rm.limits[limitID]
	if !ok {
		return fmt.Errorf("limit %s not found", limitID)
	}

	limit.Value = value
	limit.Enabled = enabled
	log.Printf("[RiskManager] Updated limit %s: value=%.2f, enabled=%v", limitID, value, enabled)
	return nil
}

// Helper functions
func absInt64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
