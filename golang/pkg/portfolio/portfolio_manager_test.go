package portfolio

import (
	"testing"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
)

// MockStrategy implements strategy.Strategy for testing
type MockStrategy struct {
	id           string
	strategyType string
	isRunning    bool
	pnl          strategy.PNL
	position     strategy.EstimatedPosition
	riskMetrics  strategy.RiskMetrics
	controlState *strategy.StrategyControlState
}

func NewMockStrategy(id string) *MockStrategy {
	return &MockStrategy{
		id:           id,
		strategyType: "mock",
		isRunning:    false,
		pnl:          strategy.PNL{},
		position:     strategy.EstimatedPosition{},
		riskMetrics:  strategy.RiskMetrics{},
		controlState: strategy.NewStrategyControlState(true),
	}
}

func (m *MockStrategy) GetID() string                                    { return m.id }
func (m *MockStrategy) GetType() string                                  { return m.strategyType }
func (m *MockStrategy) IsRunning() bool                                  { return m.isRunning }
func (m *MockStrategy) Initialize(config *strategy.StrategyConfig) error { return nil }
func (m *MockStrategy) Start() error                                     { m.isRunning = true; return nil }
func (m *MockStrategy) Stop() error                                      { m.isRunning = false; return nil }
func (m *MockStrategy) Reset()                                           {}
func (m *MockStrategy) OnMarketData(md *mdpb.MarketDataUpdate)           {}
func (m *MockStrategy) OnOrderUpdate(update *orspb.OrderUpdate)          {}
func (m *MockStrategy) OnTimer(now time.Time)                            {}
func (m *MockStrategy) OnAuctionData(md *mdpb.MarketDataUpdate)          {}
func (m *MockStrategy) GetPNL() *strategy.PNL                            { return &m.pnl }
func (m *MockStrategy) GetEstimatedPosition() *strategy.EstimatedPosition { return &m.position }
func (m *MockStrategy) GetPosition() *strategy.EstimatedPosition         { return &m.position }
func (m *MockStrategy) GetSignals() []*strategy.TradingSignal            { return nil }
func (m *MockStrategy) GetRiskMetrics() *strategy.RiskMetrics            { return &m.riskMetrics }
func (m *MockStrategy) GetStatus() *strategy.StrategyStatus              { return &strategy.StrategyStatus{} }
func (m *MockStrategy) UpdateParameters(params map[string]interface{}) error { return nil }
func (m *MockStrategy) GetCurrentParameters() map[string]interface{}     { return nil }
func (m *MockStrategy) GetControlState() *strategy.StrategyControlState  { return m.controlState }
func (m *MockStrategy) GetConfig() *strategy.StrategyConfig              { return nil }
func (m *MockStrategy) CanSendOrder() bool                               { return m.controlState.CanSendNewOrders() }
func (m *MockStrategy) SetLastMarketData(symbol string, md *mdpb.MarketDataUpdate) {}
func (m *MockStrategy) GetLastMarketData(symbol string) *mdpb.MarketDataUpdate { return nil }
func (m *MockStrategy) TriggerFlatten(reason strategy.FlattenReason, aggressive bool) {}
func (m *MockStrategy) GetPendingCancels() []*orspb.OrderUpdate          { return nil }
func (m *MockStrategy) SendOrder()                                       {}
func (m *MockStrategy) OnTradeUpdate()                                   {}
func (m *MockStrategy) CheckSquareoff()                                  {}
func (m *MockStrategy) HandleSquareON()                                  {}
func (m *MockStrategy) HandleSquareoff()                                 {}
func (m *MockStrategy) SetThresholds()                                   {}

func (m *MockStrategy) SetPNL(pnl strategy.PNL) {
	m.pnl = pnl
}

func (m *MockStrategy) SetPosition(pos strategy.EstimatedPosition) {
	m.position = pos
}

func (m *MockStrategy) SetRiskMetrics(metrics strategy.RiskMetrics) {
	m.riskMetrics = metrics
}

func TestPortfolioManager_Creation(t *testing.T) {
	pm := NewPortfolioManager(nil)

	if pm == nil {
		t.Fatal("PortfolioManager should not be nil")
	}

	if pm.config.TotalCapital != 1000000.0 {
		t.Errorf("Expected default capital 1000000.0, got %.2f", pm.config.TotalCapital)
	}

	if pm.config.MinAllocation != 0.05 {
		t.Errorf("Expected MinAllocation 0.05, got %.2f", pm.config.MinAllocation)
	}

	if pm.config.MaxAllocation != 0.50 {
		t.Errorf("Expected MaxAllocation 0.50, got %.2f", pm.config.MaxAllocation)
	}
}

func TestPortfolioManager_Initialize(t *testing.T) {
	config := &PortfolioConfig{
		TotalCapital:         500000.0,
		MinAllocation:        0.10,
		MaxAllocation:        0.40,
		EnableAutoRebalance:  false,
		EnableCorrelationCalc: false,
	}

	pm := NewPortfolioManager(config)
	err := pm.Initialize()

	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	if pm.stats.TotalCapital != 500000.0 {
		t.Errorf("Expected TotalCapital 500000.0, got %.2f", pm.stats.TotalCapital)
	}
}

func TestPortfolioManager_AddStrategy(t *testing.T) {
	pm := NewPortfolioManager(nil)
	pm.Initialize()

	strategy1 := NewMockStrategy("strategy_1")
	err := pm.AddStrategy(strategy1, 0.30) // 30% allocation

	if err != nil {
		t.Fatalf("Failed to add strategy: %v", err)
	}

	// Check strategy was added
	alloc, err := pm.GetAllocation("strategy_1")
	if err != nil {
		t.Fatalf("Failed to get allocation: %v", err)
	}

	if alloc.AllocationPercent != 0.30 {
		t.Errorf("Expected allocation 0.30, got %.2f", alloc.AllocationPercent)
	}

	expectedCapital := 1000000.0 * 0.30
	if alloc.AllocatedCapital != expectedCapital {
		t.Errorf("Expected allocated capital %.2f, got %.2f", expectedCapital, alloc.AllocatedCapital)
	}
}

func TestPortfolioManager_AddStrategy_AllocationLimits(t *testing.T) {
	pm := NewPortfolioManager(nil)
	pm.Initialize()

	strategy1 := NewMockStrategy("strategy_1")

	// Test below minimum
	err := pm.AddStrategy(strategy1, 0.01) // 1% < 5% minimum
	if err == nil {
		t.Error("Should fail with allocation below minimum")
	}

	// Test above maximum
	err = pm.AddStrategy(strategy1, 0.60) // 60% > 50% maximum
	if err == nil {
		t.Error("Should fail with allocation above maximum")
	}
}

func TestPortfolioManager_AddStrategy_TotalAllocationLimit(t *testing.T) {
	pm := NewPortfolioManager(nil)
	pm.Initialize()

	strategy1 := NewMockStrategy("strategy_1")
	strategy2 := NewMockStrategy("strategy_2")
	strategy3 := NewMockStrategy("strategy_3")

	pm.AddStrategy(strategy1, 0.40) // 40%
	pm.AddStrategy(strategy2, 0.35) // 35%

	// Total would be 40 + 35 + 30 = 105% > 100%
	err := pm.AddStrategy(strategy3, 0.30)
	if err == nil {
		t.Error("Should fail when total allocation exceeds 100%")
	}
}

func TestPortfolioManager_RemoveStrategy(t *testing.T) {
	pm := NewPortfolioManager(nil)
	pm.Initialize()

	strategy1 := NewMockStrategy("strategy_1")
	pm.AddStrategy(strategy1, 0.30)

	err := pm.RemoveStrategy("strategy_1")
	if err != nil {
		t.Fatalf("Failed to remove strategy: %v", err)
	}

	// Should not be able to get allocation after removal
	_, err = pm.GetAllocation("strategy_1")
	if err == nil {
		t.Error("Should fail to get allocation for removed strategy")
	}
}

func TestPortfolioManager_UpdateAllocations(t *testing.T) {
	pm := NewPortfolioManager(nil)
	pm.Initialize()

	strategy1 := NewMockStrategy("strategy_1")
	strategy1.Start()
	strategy1.SetPNL(strategy.PNL{
		TotalPnL:      5000.0,
		RealizedPnL:   5000.0,
		UnrealizedPnL: 0,
	})
	strategy1.SetRiskMetrics(strategy.RiskMetrics{
		ExposureValue: 250000.0,
	})
	strategy1.SetPosition(strategy.EstimatedPosition{
		NetQty: 50,
	})

	pm.AddStrategy(strategy1, 0.30)

	err := pm.UpdateAllocations()
	if err != nil {
		t.Fatalf("Failed to update allocations: %v", err)
	}

	// Check portfolio stats
	stats := pm.GetStats()
	if stats.TotalPnL != 5000.0 {
		t.Errorf("Expected TotalPnL 5000.0, got %.2f", stats.TotalPnL)
	}

	expectedReturn := 5000.0 / 1000000.0
	if stats.TotalReturn != expectedReturn {
		t.Errorf("Expected TotalReturn %.4f, got %.4f", expectedReturn, stats.TotalReturn)
	}

	if stats.NumStrategies != 1 {
		t.Errorf("Expected 1 strategy, got %d", stats.NumStrategies)
	}

	if stats.NumActiveStrategies != 1 {
		t.Errorf("Expected 1 active strategy, got %d", stats.NumActiveStrategies)
	}

	// Check strategy allocation
	alloc, _ := pm.GetAllocation("strategy_1")
	if alloc.CurrentPnL != 5000.0 {
		t.Errorf("Expected CurrentPnL 5000.0, got %.2f", alloc.CurrentPnL)
	}

	if alloc.PositionSize != 50 {
		t.Errorf("Expected PositionSize 50, got %d", alloc.PositionSize)
	}
}

func TestPortfolioManager_Rebalance(t *testing.T) {
	pm := NewPortfolioManager(nil)
	pm.Initialize()

	strategy1 := NewMockStrategy("strategy_1")
	strategy2 := NewMockStrategy("strategy_2")
	strategy3 := NewMockStrategy("strategy_3")

	pm.AddStrategy(strategy1, 0.20)
	pm.AddStrategy(strategy2, 0.30)
	pm.AddStrategy(strategy3, 0.40)

	err := pm.Rebalance()
	if err != nil {
		t.Fatalf("Failed to rebalance: %v", err)
	}

	// After rebalancing, each strategy should have equal weight (1/3 ≈ 0.33)
	expectedWeight := 1.0 / 3.0

	alloc1, _ := pm.GetAllocation("strategy_1")
	alloc2, _ := pm.GetAllocation("strategy_2")
	alloc3, _ := pm.GetAllocation("strategy_3")

	tolerance := 0.01
	if abs(alloc1.AllocationPercent-expectedWeight) > tolerance {
		t.Errorf("Strategy 1: Expected allocation %.2f, got %.2f", expectedWeight, alloc1.AllocationPercent)
	}
	if abs(alloc2.AllocationPercent-expectedWeight) > tolerance {
		t.Errorf("Strategy 2: Expected allocation %.2f, got %.2f", expectedWeight, alloc2.AllocationPercent)
	}
	if abs(alloc3.AllocationPercent-expectedWeight) > tolerance {
		t.Errorf("Strategy 3: Expected allocation %.2f, got %.2f", expectedWeight, alloc3.AllocationPercent)
	}
}

func TestPortfolioManager_CalculateCorrelation(t *testing.T) {
	config := &PortfolioConfig{
		TotalCapital:          1000000.0,
		MinAllocation:         0.05,
		MaxAllocation:         0.50,
		EnableCorrelationCalc: true,
		StrategyAllocation:    make(map[string]float64),
	}

	pm := NewPortfolioManager(config)
	pm.Initialize()

	strategy1 := NewMockStrategy("strategy_1")
	strategy2 := NewMockStrategy("strategy_2")

	err := pm.AddStrategy(strategy1, 0.30)
	if err != nil {
		t.Fatalf("Failed to add strategy1: %v", err)
	}
	err = pm.AddStrategy(strategy2, 0.30)
	if err != nil {
		t.Fatalf("Failed to add strategy2: %v", err)
	}

	corr, err := pm.CalculateCorrelation()
	if err != nil {
		t.Fatalf("Failed to calculate correlation: %v", err)
	}

	if corr == nil {
		t.Fatal("Correlation matrix should not be nil")
	}

	if len(corr.StrategyIDs) != 2 {
		t.Errorf("Expected 2 strategy IDs, got %d", len(corr.StrategyIDs))
	}

	if len(corr.Matrix) != 2 {
		t.Errorf("Expected 2x2 matrix, got %dx%d", len(corr.Matrix), len(corr.Matrix[0]))
	}

	// Check diagonal is 1.0 (self-correlation)
	if corr.Matrix[0][0] != 1.0 {
		t.Errorf("Expected diagonal element 1.0, got %.2f", corr.Matrix[0][0])
	}
	if corr.Matrix[1][1] != 1.0 {
		t.Errorf("Expected diagonal element 1.0, got %.2f", corr.Matrix[1][1])
	}
}

func TestPortfolioManager_CalculateCorrelation_Disabled(t *testing.T) {
	config := &PortfolioConfig{
		TotalCapital:          1000000.0,
		MinAllocation:         0.05,
		MaxAllocation:         0.50,
		EnableCorrelationCalc: false,
		StrategyAllocation:    make(map[string]float64),
	}

	pm := NewPortfolioManager(config)
	pm.Initialize()

	strategy1 := NewMockStrategy("strategy_1")
	strategy2 := NewMockStrategy("strategy_2")

	pm.AddStrategy(strategy1, 0.30)
	pm.AddStrategy(strategy2, 0.30)

	_, err := pm.CalculateCorrelation()
	if err == nil {
		t.Error("Should fail when correlation calculation is disabled")
	}
}

func TestPortfolioManager_CalculateCorrelation_InsufficientStrategies(t *testing.T) {
	config := &PortfolioConfig{
		TotalCapital:          1000000.0,
		MinAllocation:         0.05,
		MaxAllocation:         0.50,
		EnableCorrelationCalc: true,
		StrategyAllocation:    make(map[string]float64),
	}

	pm := NewPortfolioManager(config)
	pm.Initialize()

	strategy1 := NewMockStrategy("strategy_1")
	pm.AddStrategy(strategy1, 0.30)

	_, err := pm.CalculateCorrelation()
	if err == nil {
		t.Error("Should fail with only 1 strategy")
	}
}

func TestPortfolioManager_SharpeRatioCalculation(t *testing.T) {
	pm := NewPortfolioManager(nil)
	pm.Initialize()

	// Simulate PnL history with positive returns
	pm.pnlHistory = []float64{0, 100, 250, 350, 500, 600, 750, 850, 1000}

	sharpe := pm.calculateSharpeRatio()

	// Sharpe ratio should be positive for increasing PnL
	if sharpe <= 0 {
		t.Errorf("Expected positive Sharpe ratio for increasing PnL, got %.4f", sharpe)
	}
}

func TestPortfolioManager_StartStop(t *testing.T) {
	config := &PortfolioConfig{
		TotalCapital:         1000000.0,
		EnableAutoRebalance:  false, // Disable for faster test
		EnableCorrelationCalc: false,
	}

	pm := NewPortfolioManager(config)
	pm.Initialize()

	err := pm.Start()
	if err != nil {
		t.Fatalf("Failed to start: %v", err)
	}

	// Try starting again (should fail)
	err = pm.Start()
	if err == nil {
		t.Error("Should fail to start when already running")
	}

	err = pm.Stop()
	if err != nil {
		t.Fatalf("Failed to stop: %v", err)
	}

	// Try stopping again (should fail)
	err = pm.Stop()
	if err == nil {
		t.Error("Should fail to stop when not running")
	}
}

func TestPortfolioManager_GetAllAllocations(t *testing.T) {
	pm := NewPortfolioManager(nil)
	pm.Initialize()

	strategy1 := NewMockStrategy("strategy_1")
	strategy2 := NewMockStrategy("strategy_2")

	pm.AddStrategy(strategy1, 0.30)
	pm.AddStrategy(strategy2, 0.25)

	allocs := pm.GetAllAllocations()

	if len(allocs) != 2 {
		t.Errorf("Expected 2 allocations, got %d", len(allocs))
	}

	if allocs["strategy_1"].AllocationPercent != 0.30 {
		t.Errorf("Expected strategy_1 allocation 0.30, got %.2f", allocs["strategy_1"].AllocationPercent)
	}

	if allocs["strategy_2"].AllocationPercent != 0.25 {
		t.Errorf("Expected strategy_2 allocation 0.25, got %.2f", allocs["strategy_2"].AllocationPercent)
	}
}

func TestPortfolioManager_AllocatedVsFreeCapital(t *testing.T) {
	pm := NewPortfolioManager(nil)
	pm.Initialize()

	strategy1 := NewMockStrategy("strategy_1")
	strategy1.Start() // Active

	strategy2 := NewMockStrategy("strategy_2")
	// strategy2 is inactive

	pm.AddStrategy(strategy1, 0.30)
	pm.AddStrategy(strategy2, 0.20)

	pm.UpdateAllocations()
	stats := pm.GetStats()

	// Only active strategies count as allocated
	expectedAllocated := 1000000.0 * 0.30
	expectedFree := 1000000.0 - expectedAllocated

	if stats.AllocatedCapital != expectedAllocated {
		t.Errorf("Expected AllocatedCapital %.2f, got %.2f", expectedAllocated, stats.AllocatedCapital)
	}

	if stats.FreeCapital != expectedFree {
		t.Errorf("Expected FreeCapital %.2f, got %.2f", expectedFree, stats.FreeCapital)
	}
}

func TestPortfolioManager_PnLHistory(t *testing.T) {
	pm := NewPortfolioManager(nil)
	pm.Initialize()

	strategy1 := NewMockStrategy("strategy_1")
	pm.AddStrategy(strategy1, 0.30)

	// Update allocations multiple times to build PnL history
	for i := 0; i < 5; i++ {
		strategy1.SetPNL(strategy.PNL{
			TotalPnL: float64(i * 1000),
		})
		pm.UpdateAllocations()
		time.Sleep(10 * time.Millisecond)
	}

	if len(pm.pnlHistory) != 5 {
		t.Errorf("Expected 5 PnL history entries, got %d", len(pm.pnlHistory))
	}

	// History should cap at maxPnLHistory (1000)
	// Fill history to capacity
	pm.pnlHistory = make([]float64, pm.maxPnLHistory)
	for i := range pm.pnlHistory {
		pm.pnlHistory[i] = float64(i)
	}

	// Add one more - should trim to maxPnLHistory
	pm.UpdateAllocations()
	if len(pm.pnlHistory) > pm.maxPnLHistory {
		t.Errorf("PnL history should be capped at %d, got %d", pm.maxPnLHistory, len(pm.pnlHistory))
	}
}

func BenchmarkPortfolioManager_UpdateAllocations(b *testing.B) {
	pm := NewPortfolioManager(nil)
	pm.Initialize()

	// Add 5 strategies
	for i := 0; i < 5; i++ {
		strategy := NewMockStrategy("strategy_" + string(rune(i)))
		strategy.Start()
		pm.AddStrategy(strategy, 0.15)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.UpdateAllocations()
	}
}

func BenchmarkPortfolioManager_Rebalance(b *testing.B) {
	pm := NewPortfolioManager(nil)
	pm.Initialize()

	// Add 5 strategies
	for i := 0; i < 5; i++ {
		strategy := NewMockStrategy("strategy_" + string(rune(i)))
		pm.AddStrategy(strategy, 0.15)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pm.Rebalance()
	}
}

// Helper function
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
