package strategy

import (
	"testing"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

func TestBaseStrategy_Creation(t *testing.T) {
	bs := NewBaseStrategy("test_strategy", "test")

	if bs.GetID() != "test_strategy" {
		t.Errorf("Expected ID 'test_strategy', got '%s'", bs.GetID())
	}

	if bs.GetType() != "test" {
		t.Errorf("Expected type 'test', got '%s'", bs.GetType())
	}

	// Strategy is auto-activated by default
	if !bs.IsRunning() {
		t.Error("Strategy should be running initially (auto-activated)")
	}
}

func TestBaseStrategy_StartStop(t *testing.T) {
	bs := NewBaseStrategy("test_strategy", "test")

	// Initially running (auto-activated)
	if !bs.IsRunning() {
		t.Error("Strategy should be running initially (auto-activated)")
	}
	if !bs.ControlState.IsActive() {
		t.Error("Strategy should be active initially")
	}

	// Deactivate - strategy is still running but cannot trade
	// 对应 tbsrc: m_Active = false，进程仍在运行
	bs.Deactivate()
	if !bs.IsRunning() {
		// IsRunning() 检查进程状态，不是 Active 状态
		t.Error("Strategy should still be running after deactivate (process is alive)")
	}
	if bs.ControlState.IsActive() {
		t.Error("Strategy should not be active after deactivate")
	}

	// Activate - strategy can trade again
	// 对应 tbsrc: m_Active = true
	bs.Activate()
	if !bs.IsRunning() {
		t.Error("Strategy should be running after activate")
	}
	if !bs.ControlState.IsActive() {
		t.Error("Strategy should be active after activate")
	}

	// Stop - strategy process stops
	// 对应 tbsrc: 退出请求后，持仓平完后调用 CompleteExit()
	bs.TriggerExit("test")
	bs.CompleteExit() // 由于初始持仓为空，可以直接完成退出
	if bs.IsRunning() {
		t.Error("Strategy should not be running after complete exit")
	}
}

func TestBaseStrategy_Position(t *testing.T) {
	bs := NewBaseStrategy("test_strategy", "test")

	// Initial position should be zero
	pos := bs.GetEstimatedPosition()
	if pos.NetQty != 0 {
		t.Errorf("Expected NetQty 0, got %d", pos.NetQty)
	}
	if pos.BuyQty != 0 {
		t.Errorf("Expected BuyQty 0, got %d", pos.BuyQty)
	}
	if pos.SellQty != 0 {
		t.Errorf("Expected SellQty 0, got %d", pos.SellQty)
	}
}

func TestBaseStrategy_UpdatePosition(t *testing.T) {
	bs := NewBaseStrategy("test_strategy", "test")

	// Simulate a buy fill
	update := &orspb.OrderUpdate{
		OrderId:       "order_1",
		ClientOrderId: "client_1",
		Symbol:        "TEST",
		Side:          orspb.OrderSide_BUY,
		Price:         100.0,
		AvgPrice:      100.0,
		Quantity:      10,
		FilledQty:     10,
		Status:        orspb.OrderStatus_FILLED,
		Timestamp:     uint64(time.Now().UnixNano()),
	}

	bs.UpdatePosition(update)

	pos := bs.GetEstimatedPosition()
	if pos.BuyQty != 10 {
		t.Errorf("Expected BuyQty 10, got %d", pos.BuyQty)
	}
	if pos.NetQty != 10 {
		t.Errorf("Expected NetQty 10, got %d", pos.NetQty)
	}
	if pos.BuyAvgPrice != 100.0 {
		t.Errorf("Expected BuyAvgPrice 100.0, got %.2f", pos.BuyAvgPrice)
	}

	// Simulate a sell fill - 这会平掉部分多头持仓（净持仓模型）
	// 在净持仓模型中，卖出时如果有多头持仓，会先平掉多头
	update2 := &orspb.OrderUpdate{
		OrderId:       "order_2",
		ClientOrderId: "client_2",
		Symbol:        "TEST",
		Side:          orspb.OrderSide_SELL,
		Price:         105.0,
		AvgPrice:      105.0,
		Quantity:      5,
		FilledQty:     5,
		Status:        orspb.OrderStatus_FILLED,
		Timestamp:     uint64(time.Now().UnixNano()),
	}

	bs.UpdatePosition(update2)

	pos = bs.GetEstimatedPosition()
	// 净持仓模型：买入10，卖出5平仓，剩余 NetQty=5, BuyQty=5, SellQty=0
	if pos.NetQty != 5 {
		t.Errorf("Expected NetQty 5, got %d", pos.NetQty)
	}
	if pos.BuyQty != 5 {
		t.Errorf("Expected BuyQty 5 (after closing 5 long), got %d", pos.BuyQty)
	}
	if pos.SellQty != 0 {
		t.Errorf("Expected SellQty 0 (no short position), got %d", pos.SellQty)
	}
}

func TestBaseStrategy_PNL(t *testing.T) {
	bs := NewBaseStrategy("test_strategy", "test")

	// Initial PNL should be zero
	pnl := bs.GetPNL()
	if pnl.TotalPnL != 0 {
		t.Errorf("Expected TotalPnL 0, got %.2f", pnl.TotalPnL)
	}

	// Simulate a profitable trade
	// Buy at 100
	buy := &orspb.OrderUpdate{
		OrderId:       "order_1",
		ClientOrderId: "client_1",
		Symbol:        "TEST",
		Side:          orspb.OrderSide_BUY,
		Price:         100.0,
		AvgPrice:      100.0,
		Quantity:      10,
		FilledQty:     10,
		Status:        orspb.OrderStatus_FILLED,
		Timestamp:     uint64(time.Now().UnixNano()),
	}
	bs.UpdatePosition(buy)

	// Sell at 110 (profit = 10 * 10 = 100)
	sell := &orspb.OrderUpdate{
		OrderId:       "order_2",
		ClientOrderId: "client_2",
		Symbol:        "TEST",
		Side:          orspb.OrderSide_SELL,
		Price:         110.0,
		AvgPrice:      110.0,
		Quantity:      10,
		FilledQty:     10,
		Status:        orspb.OrderStatus_FILLED,
		Timestamp:     uint64(time.Now().UnixNano()),
	}
	bs.UpdatePosition(sell)

	// Update PNL with current price (bidPrice, askPrice)
	bs.UpdatePNL(110.0, 110.5)

	pnl = bs.GetPNL()
	if pnl.RealizedPnL != 100.0 {
		t.Errorf("Expected RealizedPnL 100.0, got %.2f", pnl.RealizedPnL)
	}
}

func TestBaseStrategy_Signals(t *testing.T) {
	bs := NewBaseStrategy("test_strategy", "test")

	// Initially no signals
	signals := bs.GetSignals()
	if len(signals) != 0 {
		t.Errorf("Expected 0 signals, got %d", len(signals))
	}

	// Add a signal
	signal := &TradingSignal{
		StrategyID: "test_strategy",
		Symbol:     "TEST",
		Side:       OrderSideBuy,
		Price:      100.0,
		Quantity:   10,
		Signal:     0.8,
		Confidence: 0.9,
		Timestamp:  time.Now(),
	}
	bs.AddSignal(signal)

	signals = bs.GetSignals()
	if len(signals) != 1 {
		t.Errorf("Expected 1 signal, got %d", len(signals))
	}
	if signals[0].Price != 100.0 {
		t.Errorf("Expected price 100.0, got %.2f", signals[0].Price)
	}
}

func TestBaseStrategy_RiskMetrics(t *testing.T) {
	bs := NewBaseStrategy("test_strategy", "test")

	// Set config with limits
	bs.Config = &StrategyConfig{
		MaxPositionSize: 100,
		MaxExposure:     10000.0,
	}

	// Create a position to track
	buy := &orspb.OrderUpdate{
		OrderId:   "order_1",
		Symbol:    "TEST",
		Side:      orspb.OrderSide_BUY,
		Price:     100.0,
		AvgPrice:  100.0,
		Quantity:  10,
		FilledQty: 10,
		Status:    orspb.OrderStatus_FILLED,
		Timestamp: uint64(time.Now().UnixNano()),
	}
	bs.UpdatePosition(buy)

	// Update risk metrics
	bs.UpdateRiskMetrics(100.0)

	metrics := bs.GetRiskMetrics()
	// Check calculated position size
	if metrics.PositionSize != 10 {
		t.Errorf("Expected PositionSize 10, got %d", metrics.PositionSize)
	}
	// Check calculated exposure (position * price)
	expectedExposure := 10.0 * 100.0
	if metrics.ExposureValue != expectedExposure {
		t.Errorf("Expected ExposureValue %.2f, got %.2f", expectedExposure, metrics.ExposureValue)
	}
}

func TestBaseStrategy_Reset(t *testing.T) {
	bs := NewBaseStrategy("test_strategy", "test")

	// Add some state
	bs.AddSignal(&TradingSignal{
		StrategyID: "test_strategy",
		Symbol:     "TEST",
		Price:      100.0,
		Quantity:   10,
	})

	update := &orspb.OrderUpdate{
		OrderId:   "order_1",
		Side:      orspb.OrderSide_BUY,
		Price:     100.0,
		Quantity:  10,
		FilledQty: 10,
		Status:    orspb.OrderStatus_FILLED,
	}
	bs.UpdatePosition(update)

	// Reset
	bs.Reset()

	// Check everything is cleared
	if len(bs.GetSignals()) != 0 {
		t.Error("Signals should be cleared after reset")
	}
	if bs.GetEstimatedPosition().NetQty != 0 {
		t.Error("Position should be zero after reset")
	}
	if bs.GetPNL().TotalPnL != 0 {
		t.Error("PNL should be zero after reset")
	}
}

func TestBaseStrategy_CheckRiskLimits(t *testing.T) {
	bs := NewBaseStrategy("test_strategy", "test")
	bs.Config = &StrategyConfig{
		MaxPositionSize: 10,
		MaxExposure:     1000.0,
	}

	// Position within limits
	bs.EstimatedPosition.NetQty = 5
	bs.UpdateRiskMetrics(100.0)

	if !bs.CheckRiskLimits() {
		t.Error("Should pass risk check with position within limits")
	}

	// Position exceeds limits
	bs.EstimatedPosition.NetQty = 15
	bs.UpdateRiskMetrics(100.0)

	if bs.CheckRiskLimits() {
		t.Error("Should fail risk check with position exceeding limits")
	}
}

// Helper function to create test market data
func createTestMarketData(symbol string, price float64) *mdpb.MarketDataUpdate {
	return &mdpb.MarketDataUpdate{
		Symbol:      symbol,
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{price - 0.5},
		BidQty:      []uint32{100},
		AskPrice:    []float64{price + 0.5},
		AskQty:      []uint32{100},
		LastPrice:   price,
		TotalVolume: 1000,
		Turnover:    price * 1000,
	}
}

func BenchmarkBaseStrategy_UpdatePosition(b *testing.B) {
	bs := NewBaseStrategy("test_strategy", "test")
	update := &orspb.OrderUpdate{
		OrderId:   "order_1",
		Side:      orspb.OrderSide_BUY,
		Price:     100.0,
		Quantity:  10,
		FilledQty: 10,
		Status:    orspb.OrderStatus_FILLED,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bs.UpdatePosition(update)
	}
}

func BenchmarkBaseStrategy_UpdatePNL(b *testing.B) {
	bs := NewBaseStrategy("test_strategy", "test")
	bs.EstimatedPosition.NetQty = 10
	bs.EstimatedPosition.BuyAvgPrice = 100.0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bs.UpdatePNL(105.0, 105.5)
	}
}
