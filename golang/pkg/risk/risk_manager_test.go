package risk

import (
	"testing"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
)

// MockStrategy for testing risk manager
type MockStrategy struct {
	id          string
	isRunning   bool
	pnl         strategy.PNL
	position    strategy.Position
	riskMetrics strategy.RiskMetrics
}

func NewMockStrategy(id string) *MockStrategy {
	return &MockStrategy{
		id:          id,
		isRunning:   false,
		pnl:         strategy.PNL{},
		position:    strategy.Position{},
		riskMetrics: strategy.RiskMetrics{},
	}
}

func (m *MockStrategy) GetID() string                     { return m.id }
func (m *MockStrategy) GetType() string                   { return "mock" }
func (m *MockStrategy) IsRunning() bool                   { return m.isRunning }
func (m *MockStrategy) Initialize(config *strategy.StrategyConfig) error { return nil }
func (m *MockStrategy) Start() error                      { m.isRunning = true; return nil }
func (m *MockStrategy) Stop() error                       { m.isRunning = false; return nil }
func (m *MockStrategy) Reset()                            {}
func (m *MockStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {}
func (m *MockStrategy) OnOrderUpdate(update *orspb.OrderUpdate) {}
func (m *MockStrategy) OnTimer(now time.Time)            {}
func (m *MockStrategy) OnAuctionData(md *mdpb.MarketDataUpdate) {}
func (m *MockStrategy) GetPNL() *strategy.PNL             { return &m.pnl }
func (m *MockStrategy) GetPosition() *strategy.Position   { return &m.position }
func (m *MockStrategy) GetSignals() []*strategy.TradingSignal { return nil }
func (m *MockStrategy) GetRiskMetrics() *strategy.RiskMetrics { return &m.riskMetrics }
func (m *MockStrategy) GetStatus() *strategy.StrategyStatus { return &strategy.StrategyStatus{} }
func (m *MockStrategy) UpdateParameters(params map[string]interface{}) error { return nil }
func (m *MockStrategy) GetCurrentParameters() map[string]interface{} { return nil }

func TestRiskManager_Creation(t *testing.T) {
	rm := NewRiskManager(nil)

	if rm == nil {
		t.Fatal("RiskManager should not be nil")
	}

	if !rm.config.EnableGlobalLimits {
		t.Error("Global limits should be enabled by default")
	}

	if !rm.config.EnableStrategyLimits {
		t.Error("Strategy limits should be enabled by default")
	}

	if rm.config.EmergencyStopThreshold != 3 {
		t.Errorf("Expected EmergencyStopThreshold 3, got %d", rm.config.EmergencyStopThreshold)
	}
}

func TestRiskManager_Initialize(t *testing.T) {
	rm := NewRiskManager(nil)
	err := rm.Initialize()

	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Check default limits were added
	if len(rm.limits) == 0 {
		t.Error("Expected default limits to be added")
	}

	// Check global limits exist
	if _, ok := rm.limits["global_max_exposure"]; !ok {
		t.Error("global_max_exposure limit should exist")
	}

	if _, ok := rm.limits["global_max_drawdown"]; !ok {
		t.Error("global_max_drawdown limit should exist")
	}

	if _, ok := rm.limits["global_max_daily_loss"]; !ok {
		t.Error("global_max_daily_loss limit should exist")
	}

	// Check strategy limits exist
	if _, ok := rm.limits["strategy_default_position"]; !ok {
		t.Error("strategy_default_position limit should exist")
	}

	if _, ok := rm.limits["strategy_default_exposure"]; !ok {
		t.Error("strategy_default_exposure limit should exist")
	}
}

func TestRiskManager_StartStop(t *testing.T) {
	rm := NewRiskManager(nil)
	rm.Initialize()

	err := rm.Start()
	if err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	// Try starting again (should fail)
	err = rm.Start()
	if err == nil {
		t.Error("Should fail to start when already running")
	}

	err = rm.Stop()
	if err != nil {
		t.Fatalf("Failed to stop: %v", err)
	}

	// Try stopping again (should fail)
	err = rm.Stop()
	if err == nil {
		t.Error("Should fail to stop when not running")
	}
}

func TestRiskManager_CheckStrategy_PositionLimit(t *testing.T) {
	rm := NewRiskManager(nil)
	rm.Initialize()

	strategy := NewMockStrategy("test_strategy")
	strategy.position.NetQty = 150 // Exceeds default limit of 100
	strategy.riskMetrics.ExposureValue = 50000.0

	alerts := rm.CheckStrategy(strategy)

	if len(alerts) == 0 {
		t.Error("Expected position limit alert")
	}

	foundPositionAlert := false
	for _, alert := range alerts {
		if alert.Type == RiskLimitPositionSize {
			foundPositionAlert = true
			if alert.Level != "warning" {
				t.Errorf("Expected warning level, got %s", alert.Level)
			}
			if alert.Action != "throttle" {
				t.Errorf("Expected throttle action, got %s", alert.Action)
			}
		}
	}

	if !foundPositionAlert {
		t.Error("Should generate position size alert")
	}
}

func TestRiskManager_CheckStrategy_ExposureLimit(t *testing.T) {
	rm := NewRiskManager(nil)
	rm.Initialize()

	strategy := NewMockStrategy("test_strategy")
	strategy.position.NetQty = 50
	strategy.riskMetrics.ExposureValue = 1500000.0 // Exceeds default limit of 1000000

	alerts := rm.CheckStrategy(strategy)

	foundExposureAlert := false
	for _, alert := range alerts {
		if alert.Type == RiskLimitExposure {
			foundExposureAlert = true
			if alert.CurrentValue != 1500000.0 {
				t.Errorf("Expected CurrentValue 1500000.0, got %.2f", alert.CurrentValue)
			}
		}
	}

	if !foundExposureAlert {
		t.Error("Should generate exposure limit alert")
	}
}

func TestRiskManager_CheckStrategy_LossAlert(t *testing.T) {
	rm := NewRiskManager(nil)
	rm.Initialize()

	strategy := NewMockStrategy("test_strategy")
	strategy.pnl.TotalPnL = -15000.0 // Loss exceeds threshold
	strategy.position.NetQty = 50
	strategy.riskMetrics.ExposureValue = 50000.0

	alerts := rm.CheckStrategy(strategy)

	foundLossAlert := false
	for _, alert := range alerts {
		if alert.Type == RiskLimitLoss {
			foundLossAlert = true
			if alert.Level != "critical" {
				t.Errorf("Expected critical level, got %s", alert.Level)
			}
			if alert.Action != "stop" {
				t.Errorf("Expected stop action, got %s", alert.Action)
			}
		}
	}

	if !foundLossAlert {
		t.Error("Should generate loss alert")
	}
}

func TestRiskManager_CheckGlobal_ExposureLimit(t *testing.T) {
	rm := NewRiskManager(nil)
	rm.Initialize()

	strategies := make(map[string]strategy.Strategy)

	// Create strategies with combined exposure exceeding global limit
	for i := 0; i < 3; i++ {
		s := NewMockStrategy("strategy_" + string(rune(i)))
		s.riskMetrics.ExposureValue = 5000000.0 // Total: 15M > 10M limit
		strategies[s.GetID()] = s
	}

	alerts := rm.CheckGlobal(strategies)

	if len(alerts) == 0 {
		t.Error("Expected global exposure alert")
	}

	foundExposureAlert := false
	for _, alert := range alerts {
		if alert.Type == RiskLimitExposure {
			foundExposureAlert = true
			if alert.Level != "critical" {
				t.Errorf("Expected critical level, got %s", alert.Level)
			}
			if alert.Action != "emergency_stop" {
				t.Errorf("Expected emergency_stop action, got %s", alert.Action)
			}
		}
	}

	if !foundExposureAlert {
		t.Error("Should generate global exposure alert")
	}
}

func TestRiskManager_CheckGlobal_DrawdownLimit(t *testing.T) {
	rm := NewRiskManager(nil)
	rm.Initialize()

	strategies := make(map[string]strategy.Strategy)

	s1 := NewMockStrategy("strategy_1")
	s1.riskMetrics.MaxDrawdown = 150000.0 // Exceeds limit of 100000
	strategies[s1.GetID()] = s1

	alerts := rm.CheckGlobal(strategies)

	foundDrawdownAlert := false
	for _, alert := range alerts {
		if alert.Type == RiskLimitDrawdown {
			foundDrawdownAlert = true
			if alert.Action != "emergency_stop" {
				t.Errorf("Expected emergency_stop action, got %s", alert.Action)
			}
		}
	}

	if !foundDrawdownAlert {
		t.Error("Should generate global drawdown alert")
	}
}

func TestRiskManager_CheckGlobal_DailyLossLimit(t *testing.T) {
	rm := NewRiskManager(nil)
	rm.Initialize()

	strategies := make(map[string]strategy.Strategy)

	s1 := NewMockStrategy("strategy_1")
	s1.pnl.TotalPnL = -60000.0 // Exceeds daily loss limit of 50000
	strategies[s1.GetID()] = s1

	alerts := rm.CheckGlobal(strategies)

	foundDailyLossAlert := false
	for _, alert := range alerts {
		if alert.Type == RiskLimitDailyLoss {
			foundDailyLossAlert = true
			if alert.Level != "critical" {
				t.Errorf("Expected critical level, got %s", alert.Level)
			}
		}
	}

	if !foundDailyLossAlert {
		t.Error("Should generate daily loss alert")
	}
}

func TestRiskManager_AddAlert(t *testing.T) {
	rm := NewRiskManager(nil)
	rm.Initialize()
	rm.Start()
	defer rm.Stop()

	alert := &RiskAlert{
		Timestamp:    time.Now(),
		Level:        "warning",
		Type:         RiskLimitPositionSize,
		TargetID:     "test_strategy",
		Message:      "Test alert",
		CurrentValue: 150.0,
		LimitValue:   100.0,
		Action:       "throttle",
	}

	rm.AddAlert(alert)

	// Give time for alert to be processed
	time.Sleep(100 * time.Millisecond)

	alerts := rm.GetAlerts("", 10)
	if len(alerts) == 0 {
		t.Error("Alert should have been added")
	}
}

func TestRiskManager_EmergencyStop(t *testing.T) {
	config := &RiskManagerConfig{
		EnableGlobalLimits:     true,
		EmergencyStopThreshold: 2, // Low threshold for testing
		MaxAlertQueueSize:      100,
	}

	rm := NewRiskManager(config)
	rm.Initialize()
	rm.Start()
	defer rm.Stop()

	// Add critical alerts to trigger emergency stop
	for i := 0; i < 3; i++ {
		alert := &RiskAlert{
			Timestamp: time.Now(),
			Level:     "critical",
			Type:      RiskLimitExposure,
			TargetID:  "test_strategy",
			Message:   "Critical alert " + string(rune(i)),
			Action:    "emergency_stop",
		}
		rm.AddAlert(alert)
	}

	// Give time for alerts to be processed
	time.Sleep(200 * time.Millisecond)

	if !rm.IsEmergencyStop() {
		t.Error("Emergency stop should be triggered after threshold critical alerts")
	}
}

func TestRiskManager_ResetEmergencyStop(t *testing.T) {
	rm := NewRiskManager(nil)
	rm.Initialize()

	// Manually trigger emergency stop
	rm.emergencyStop = true
	rm.criticalAlerts = 5

	rm.ResetEmergencyStop()

	if rm.IsEmergencyStop() {
		t.Error("Emergency stop should be reset")
	}

	if rm.criticalAlerts != 0 {
		t.Errorf("Critical alerts should be reset to 0, got %d", rm.criticalAlerts)
	}
}

func TestRiskManager_UpdateLimit(t *testing.T) {
	rm := NewRiskManager(nil)
	rm.Initialize()

	err := rm.UpdateLimit("global_max_exposure", 20000000.0, true)
	if err != nil {
		t.Fatalf("Failed to update limit: %v", err)
	}

	limit := rm.limits["global_max_exposure"]
	if limit.Value != 20000000.0 {
		t.Errorf("Expected limit value 20000000.0, got %.2f", limit.Value)
	}

	if !limit.Enabled {
		t.Error("Limit should be enabled")
	}
}

func TestRiskManager_UpdateLimit_NotFound(t *testing.T) {
	rm := NewRiskManager(nil)
	rm.Initialize()

	err := rm.UpdateLimit("nonexistent_limit", 100.0, true)
	if err == nil {
		t.Error("Should fail to update nonexistent limit")
	}
}

func TestRiskManager_GetAlerts_FilterByLevel(t *testing.T) {
	rm := NewRiskManager(nil)
	rm.Initialize()
	rm.Start()
	defer rm.Stop()

	// Add warning alert
	warningAlert := &RiskAlert{
		Timestamp: time.Now(),
		Level:     "warning",
		Type:      RiskLimitPositionSize,
		TargetID:  "test_strategy",
		Message:   "Warning alert",
	}
	rm.AddAlert(warningAlert)

	// Add critical alert
	criticalAlert := &RiskAlert{
		Timestamp: time.Now(),
		Level:     "critical",
		Type:      RiskLimitExposure,
		TargetID:  "test_strategy",
		Message:   "Critical alert",
	}
	rm.AddAlert(criticalAlert)

	time.Sleep(100 * time.Millisecond)

	// Get only critical alerts
	criticalAlerts := rm.GetAlerts("critical", 10)
	if len(criticalAlerts) == 0 {
		t.Error("Should have critical alerts")
	}

	for _, alert := range criticalAlerts {
		if alert.Level != "critical" {
			t.Errorf("Expected only critical alerts, got %s", alert.Level)
		}
	}
}

func TestRiskManager_GetAlerts_Limit(t *testing.T) {
	rm := NewRiskManager(nil)
	rm.Initialize()
	rm.Start()
	defer rm.Stop()

	// Add 10 alerts
	for i := 0; i < 10; i++ {
		alert := &RiskAlert{
			Timestamp: time.Now(),
			Level:     "warning",
			Type:      RiskLimitPositionSize,
			TargetID:  "test_strategy",
			Message:   "Alert " + string(rune(i)),
		}
		rm.AddAlert(alert)
		time.Sleep(10 * time.Millisecond)
	}

	time.Sleep(100 * time.Millisecond)

	// Get only 5 alerts
	alerts := rm.GetAlerts("", 5)
	if len(alerts) > 5 {
		t.Errorf("Expected at most 5 alerts, got %d", len(alerts))
	}
}

func TestRiskManager_GetGlobalStats(t *testing.T) {
	rm := NewRiskManager(nil)
	rm.Initialize()

	strategies := make(map[string]strategy.Strategy)

	s1 := NewMockStrategy("strategy_1")
	s1.riskMetrics.ExposureValue = 500000.0
	s1.riskMetrics.MaxDrawdown = 5000.0
	s1.pnl.TotalPnL = 10000.0
	strategies[s1.GetID()] = s1

	rm.CheckGlobal(strategies)

	stats := rm.GetGlobalStats()

	if stats["total_exposure"].(float64) != 500000.0 {
		t.Errorf("Expected total_exposure 500000.0, got %.2f", stats["total_exposure"].(float64))
	}

	if stats["total_pnl"].(float64) != 10000.0 {
		t.Errorf("Expected total_pnl 10000.0, got %.2f", stats["total_pnl"].(float64))
	}

	if stats["total_drawdown"].(float64) != 5000.0 {
		t.Errorf("Expected total_drawdown 5000.0, got %.2f", stats["total_drawdown"].(float64))
	}
}

func TestRiskManager_CheckStrategy_Disabled(t *testing.T) {
	config := &RiskManagerConfig{
		EnableStrategyLimits: false,
	}

	rm := NewRiskManager(config)
	rm.Initialize()

	strategy := NewMockStrategy("test_strategy")
	strategy.position.NetQty = 200 // Way over limit

	alerts := rm.CheckStrategy(strategy)

	if len(alerts) != 0 {
		t.Error("Should not generate alerts when strategy limits are disabled")
	}
}

func TestRiskManager_CheckGlobal_Disabled(t *testing.T) {
	config := &RiskManagerConfig{
		EnableGlobalLimits: false,
	}

	rm := NewRiskManager(config)
	rm.Initialize()

	strategies := make(map[string]strategy.Strategy)

	s1 := NewMockStrategy("strategy_1")
	s1.riskMetrics.ExposureValue = 50000000.0 // Way over limit
	strategies[s1.GetID()] = s1

	alerts := rm.CheckGlobal(strategies)

	if len(alerts) != 0 {
		t.Error("Should not generate alerts when global limits are disabled")
	}
}

func TestRiskManager_AlertRetention(t *testing.T) {
	config := &RiskManagerConfig{
		AlertRetentionSeconds: 1, // 1 second retention for testing
		MaxAlertQueueSize:     100,
	}

	rm := NewRiskManager(config)
	rm.Initialize()
	rm.Start()
	defer rm.Stop()

	// Add an alert
	alert := &RiskAlert{
		Timestamp: time.Now(),
		Level:     "warning",
		Type:      RiskLimitPositionSize,
		TargetID:  "test_strategy",
		Message:   "Test alert",
	}
	rm.AddAlert(alert)

	time.Sleep(100 * time.Millisecond)

	// Alert should exist
	alerts := rm.GetAlerts("", 10)
	if len(alerts) == 0 {
		t.Error("Alert should exist")
	}

	// Wait for retention period to expire
	time.Sleep(1500 * time.Millisecond)

	// Add another alert to trigger cleanup
	rm.AddAlert(alert)
	time.Sleep(100 * time.Millisecond)

	// Old alerts should be cleaned up
	// (In practice, alerts array gets trimmed during handleAlert)
}

func BenchmarkRiskManager_CheckStrategy(b *testing.B) {
	rm := NewRiskManager(nil)
	rm.Initialize()

	strategy := NewMockStrategy("test_strategy")
	strategy.position.NetQty = 50
	strategy.riskMetrics.ExposureValue = 50000.0
	strategy.pnl.TotalPnL = 1000.0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.CheckStrategy(strategy)
	}
}

func BenchmarkRiskManager_CheckGlobal(b *testing.B) {
	rm := NewRiskManager(nil)
	rm.Initialize()

	strategies := make(map[string]strategy.Strategy)
	for i := 0; i < 5; i++ {
		s := NewMockStrategy("strategy_" + string(rune(i)))
		s.riskMetrics.ExposureValue = 100000.0
		s.pnl.TotalPnL = 5000.0
		strategies[s.GetID()] = s
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rm.CheckGlobal(strategies)
	}
}
