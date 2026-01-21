// Package risk provides risk management functionality
package risk

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/yourusername/hft-poc/pkg/strategy"
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
type RiskManagerConfig struct {
	EnableGlobalLimits      bool
	EnableStrategyLimits    bool
	EnablePortfolioLimits   bool
	AlertRetentionSeconds   int
	MaxAlertQueueSize       int
	EmergencyStopThreshold  int // Number of critical alerts to trigger emergency stop
	CheckIntervalMs         int64
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
		config = &RiskManagerConfig{
			EnableGlobalLimits:     true,
			EnableStrategyLimits:   true,
			EnablePortfolioLimits:  true,
			AlertRetentionSeconds:  3600,
			MaxAlertQueueSize:      1000,
			EmergencyStopThreshold: 3,
			CheckIntervalMs:        100,
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

// addDefaultLimits adds default risk limits
func (rm *RiskManager) addDefaultLimits() {
	// Global limits
	rm.limits["global_max_exposure"] = &RiskLimit{
		Type:        RiskLimitExposure,
		Level:       "global",
		TargetID:    "*",
		Value:       10000000.0, // 1000万
		Enabled:     true,
		Description: "Global maximum exposure",
	}

	rm.limits["global_max_drawdown"] = &RiskLimit{
		Type:        RiskLimitDrawdown,
		Level:       "global",
		TargetID:    "*",
		Value:       100000.0, // 10万
		Enabled:     true,
		Description: "Global maximum drawdown",
	}

	rm.limits["global_max_daily_loss"] = &RiskLimit{
		Type:        RiskLimitDailyLoss,
		Level:       "global",
		TargetID:    "*",
		Value:       50000.0, // 5万
		Enabled:     true,
		Description: "Global maximum daily loss",
	}

	// Strategy default limits
	rm.limits["strategy_default_position"] = &RiskLimit{
		Type:        RiskLimitPositionSize,
		Level:       "strategy",
		TargetID:    "*",
		Value:       100,
		Enabled:     true,
		Description: "Default strategy position limit",
	}

	rm.limits["strategy_default_exposure"] = &RiskLimit{
		Type:        RiskLimitExposure,
		Level:       "strategy",
		TargetID:    "*",
		Value:       1000000.0, // 100万
		Enabled:     true,
		Description: "Default strategy exposure limit",
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

	position := s.GetPosition()
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
				Message:      fmt.Sprintf("Position size %d exceeds limit %.0f", position.NetQty, limit.Value),
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

	// Check drawdown
	if pnl.TotalPnL < 0 && absFloat(pnl.TotalPnL) > 10000 {
		alerts = append(alerts, RiskAlert{
			Timestamp:    time.Now(),
			Level:        "critical",
			Type:         RiskLimitLoss,
			TargetID:     s.GetID(),
			Message:      fmt.Sprintf("Loss %.2f exceeds threshold", pnl.TotalPnL),
			CurrentValue: absFloat(pnl.TotalPnL),
			LimitValue:   10000,
			Action:       "stop",
		})
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
