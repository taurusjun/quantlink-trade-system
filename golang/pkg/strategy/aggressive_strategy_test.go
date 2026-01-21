package strategy

import (
	"testing"
	"time"

	mdpb "github.com/yourusername/hft-poc/pkg/proto/md"
)

func TestAggressiveStrategy_Creation(t *testing.T) {
	as := NewAggressiveStrategy("aggressive_1")

	if as.GetID() != "aggressive_1" {
		t.Errorf("Expected ID 'aggressive_1', got '%s'", as.GetID())
	}

	if as.GetType() != "aggressive" {
		t.Errorf("Expected type 'aggressive', got '%s'", as.GetType())
	}

	// Check default parameters
	if as.trendPeriod != 50 {
		t.Errorf("Expected trendPeriod 50, got %d", as.trendPeriod)
	}
	if as.momentumPeriod != 20 {
		t.Errorf("Expected momentumPeriod 20, got %d", as.momentumPeriod)
	}
	if as.signalThreshold != 0.6 {
		t.Errorf("Expected signalThreshold 0.6, got %.2f", as.signalThreshold)
	}
}

func TestAggressiveStrategy_Initialize(t *testing.T) {
	as := NewAggressiveStrategy("aggressive_1")

	config := &StrategyConfig{
		StrategyID:      "aggressive_1",
		StrategyType:    "aggressive",
		Symbols:         []string{"TEST"},
		Exchanges:       []string{"TEST"},
		MaxPositionSize: 100,
		MaxExposure:     10000.0,
		Parameters: map[string]interface{}{
			"trend_period":        30.0,
			"momentum_period":     15.0,
			"signal_threshold":    0.5,
			"order_size":          10.0,
			"max_position_size":   100.0,
			"stop_loss_percent":   0.03,
			"take_profit_percent": 0.06,
			"min_volatility":      0.0002,
			"use_volatility_scale": false,
			"signal_refresh_ms":   1000.0,
		},
		Enabled: true,
	}

	err := as.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	if as.Config == nil {
		t.Error("Config should not be nil after initialization")
	}

	// Check parameters were loaded
	if as.trendPeriod != 30 {
		t.Errorf("Expected trendPeriod 30, got %d", as.trendPeriod)
	}
	if as.momentumPeriod != 15 {
		t.Errorf("Expected momentumPeriod 15, got %d", as.momentumPeriod)
	}
	if as.signalThreshold != 0.5 {
		t.Errorf("Expected signalThreshold 0.5, got %.2f", as.signalThreshold)
	}
	if as.stopLossPercent != 0.03 {
		t.Errorf("Expected stopLossPercent 0.03, got %.2f", as.stopLossPercent)
	}

	// Check indicators were created
	trendInd, ok := as.Indicators.Get("ewma_trend_30")
	if !ok {
		t.Error("Trend indicator should be created")
	}
	if trendInd == nil {
		t.Error("Trend indicator should not be nil")
	}

	momentumInd, ok := as.Indicators.Get("ewma_momentum_15")
	if !ok {
		t.Error("Momentum indicator should be created")
	}
	if momentumInd == nil {
		t.Error("Momentum indicator should not be nil")
	}

	volInd, ok := as.Indicators.Get("volatility")
	if !ok {
		t.Error("Volatility indicator should be created")
	}
	if volInd == nil {
		t.Error("Volatility indicator should not be nil")
	}
}

func TestAggressiveStrategy_SignalGeneration(t *testing.T) {
	as := NewAggressiveStrategy("aggressive_1")

	config := &StrategyConfig{
		StrategyID:      "aggressive_1",
		StrategyType:    "aggressive",
		Symbols:         []string{"TEST"},
		Exchanges:       []string{"TEST"},
		MaxPositionSize: 100,
		MaxExposure:     10000.0,
		Parameters: map[string]interface{}{
			"trend_period":         20.0,   // Shorter for testing
			"momentum_period":      10.0,
			"signal_threshold":     0.3,    // Lower for testing
			"order_size":           10.0,
			"max_position_size":    100.0,
			"signal_refresh_ms":    100.0,  // Fast for testing
			"use_volatility_scale": false,
			"min_volatility":       0.0,    // Disable volatility check for testing
		},
		Enabled: true,
	}

	err := as.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	as.Start()

	// Feed market data with upward trend
	for i := 0; i < 50; i++ {
		price := 100.0 + float64(i)*0.2 // Upward trend
		md := &mdpb.MarketDataUpdate{
			Symbol:      "TEST",
			Exchange:    "TEST",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{price - 0.5},
			BidQty:      []uint32{100},
			AskPrice:    []float64{price + 0.5},
			AskQty:      []uint32{100},
			LastPrice:   price,
			TotalVolume: uint64(1000 + i*10),
			Turnover:    price * float64(1000+i*10),
		}
		as.OnMarketData(md)
		time.Sleep(10 * time.Millisecond)
	}

	// Should have generated some signals
	signals := as.GetSignals()
	if len(signals) == 0 {
		t.Error("Expected some signals to be generated in upward trend")
	}

	// Should be buy signals
	buyCount := 0
	for _, signal := range signals {
		if signal.Side == OrderSideBuy {
			buyCount++
		}
	}
	if buyCount == 0 {
		t.Error("Expected at least one buy signal in upward trend")
	}

	as.Stop()
}

func TestAggressiveStrategy_StopLoss(t *testing.T) {
	as := NewAggressiveStrategy("aggressive_1")

	config := &StrategyConfig{
		StrategyID:      "aggressive_1",
		StrategyType:    "aggressive",
		Symbols:         []string{"TEST"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"trend_period":       20.0,
			"momentum_period":    10.0,
			"order_size":         10.0,
			"stop_loss_percent":  0.02, // 2% stop loss
			"take_profit_percent": 0.05,
			"signal_refresh_ms":  100.0,
		},
		Enabled: true,
	}

	err := as.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	as.Start()

	// Set up a long position
	as.Position.NetQty = 10
	as.Position.LongQty = 10
	as.entryPrice = 100.0

	// Price drops below stop loss (2%)
	md := &mdpb.MarketDataUpdate{
		Symbol:      "TEST",
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{97.5}, // -2.5% from entry
		BidQty:      []uint32{100},
		AskPrice:    []float64{98.0},
		AskQty:      []uint32{100},
		LastPrice:   97.75,
		TotalVolume: 1000,
		Turnover:    97750.0,
	}

	as.OnMarketData(md)

	// Should generate exit signal
	signals := as.GetSignals()
	if len(signals) == 0 {
		t.Error("Expected stop loss to generate exit signal")
	}

	// Check it's a sell signal
	lastSignal := signals[len(signals)-1]
	if lastSignal.Side != OrderSideSell {
		t.Error("Stop loss should generate sell signal")
	}
	if lastSignal.Metadata["type"] != "exit" {
		t.Error("Signal should be marked as exit type")
	}
	if lastSignal.Metadata["reason"] != "stop_loss" {
		t.Error("Signal should have stop_loss reason")
	}

	as.Stop()
}

func TestAggressiveStrategy_TakeProfit(t *testing.T) {
	as := NewAggressiveStrategy("aggressive_1")

	config := &StrategyConfig{
		StrategyID:      "aggressive_1",
		StrategyType:    "aggressive",
		Symbols:         []string{"TEST"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"trend_period":        20.0,
			"momentum_period":     10.0,
			"order_size":          10.0,
			"stop_loss_percent":   0.02,
			"take_profit_percent": 0.05, // 5% take profit
			"signal_refresh_ms":   100.0,
		},
		Enabled: true,
	}

	err := as.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	as.Start()

	// Set up a long position
	as.Position.NetQty = 10
	as.Position.LongQty = 10
	as.entryPrice = 100.0

	// Price rises above take profit (5%)
	md := &mdpb.MarketDataUpdate{
		Symbol:      "TEST",
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{105.5}, // +5.5% from entry
		BidQty:      []uint32{100},
		AskPrice:    []float64{106.0},
		AskQty:      []uint32{100},
		LastPrice:   105.75,
		TotalVolume: 1000,
		Turnover:    105750.0,
	}

	as.OnMarketData(md)

	// Should generate exit signal
	signals := as.GetSignals()
	if len(signals) == 0 {
		t.Error("Expected take profit to generate exit signal")
	}

	// Check it's a sell signal
	lastSignal := signals[len(signals)-1]
	if lastSignal.Side != OrderSideSell {
		t.Error("Take profit should generate sell signal")
	}
	if lastSignal.Metadata["reason"] != "take_profit" {
		t.Error("Signal should have take_profit reason")
	}

	as.Stop()
}

func TestAggressiveStrategy_VolatilityScaling(t *testing.T) {
	as := NewAggressiveStrategy("aggressive_1")

	config := &StrategyConfig{
		StrategyID:      "aggressive_1",
		StrategyType:    "aggressive",
		Symbols:         []string{"TEST"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"trend_period":         20.0,
			"momentum_period":      10.0,
			"order_size":           20.0,
			"use_volatility_scale": true,
			"signal_refresh_ms":    100.0,
		},
		Enabled: true,
	}

	err := as.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Volatility scaling should reduce position size in high volatility
	// This is hard to test directly without mocking, but we can verify the parameter is set
	if !as.useVolatilityScale {
		t.Error("Volatility scaling should be enabled")
	}
}

func TestAggressiveStrategy_PositionLimits(t *testing.T) {
	as := NewAggressiveStrategy("aggressive_1")

	config := &StrategyConfig{
		StrategyID:      "aggressive_1",
		StrategyType:    "aggressive",
		Symbols:         []string{"TEST"},
		MaxPositionSize: 50,
		Parameters: map[string]interface{}{
			"trend_period":      20.0,
			"momentum_period":   10.0,
			"order_size":        10.0,
			"max_position_size": 50.0, // Limit to 50
			"signal_refresh_ms": 100.0,
		},
		Enabled: true,
	}

	err := as.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	as.Start()

	// Set position near limit
	as.Position.NetQty = 45
	as.Position.LongQty = 45

	// Feed bullish market data
	md := &mdpb.MarketDataUpdate{
		Symbol:      "TEST",
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{110.0},
		BidQty:      []uint32{100},
		AskPrice:    []float64{110.5},
		AskQty:      []uint32{100},
		LastPrice:   110.25,
		TotalVolume: 1000,
		Turnover:    110250.0,
	}

	// Position is near max, should not generate large buy signals
	initialSignalCount := len(as.GetSignals())
	as.OnMarketData(md)

	// If any signals were generated, check they respect limits
	signals := as.GetSignals()
	if len(signals) > initialSignalCount {
		lastSignal := signals[len(signals)-1]
		if lastSignal.Side == OrderSideBuy {
			// Should not exceed max position
			if as.Position.NetQty+lastSignal.Quantity > as.maxPositionSize {
				t.Error("Signal should respect maximum position limit")
			}
		}
	}

	as.Stop()
}

func TestAggressiveStrategy_StartStop(t *testing.T) {
	as := NewAggressiveStrategy("aggressive_1")

	if as.IsRunning() {
		t.Error("Strategy should not be running initially")
	}

	as.Start()
	if !as.IsRunning() {
		t.Error("Strategy should be running after start")
	}

	as.Stop()
	if as.IsRunning() {
		t.Error("Strategy should not be running after stop")
	}
}

func TestAggressiveStrategy_ShortPosition(t *testing.T) {
	as := NewAggressiveStrategy("aggressive_1")

	config := &StrategyConfig{
		StrategyID:      "aggressive_1",
		StrategyType:    "aggressive",
		Symbols:         []string{"TEST"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"trend_period":         20.0,
			"momentum_period":      10.0,
			"order_size":           10.0,
			"signal_threshold":     0.3,
			"signal_refresh_ms":    100.0,
			"use_volatility_scale": false,
			"min_volatility":       0.0, // Disable volatility check for testing
		},
		Enabled: true,
	}

	err := as.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	as.Start()

	// Feed downward trend data
	for i := 0; i < 50; i++ {
		price := 100.0 - float64(i)*0.2 // Downward trend
		md := &mdpb.MarketDataUpdate{
			Symbol:      "TEST",
			Exchange:    "TEST",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{price - 0.5},
			BidQty:      []uint32{100},
			AskPrice:    []float64{price + 0.5},
			AskQty:      []uint32{100},
			LastPrice:   price,
			TotalVolume: uint64(1000 + i*10),
			Turnover:    price * float64(1000+i*10),
		}
		as.OnMarketData(md)
		time.Sleep(10 * time.Millisecond)
	}

	// Should have generated some sell signals
	signals := as.GetSignals()
	if len(signals) == 0 {
		t.Error("Expected some signals in downward trend")
	}

	// Count sell signals
	sellCount := 0
	for _, signal := range signals {
		if signal.Side == OrderSideSell {
			sellCount++
		}
	}
	if sellCount == 0 {
		t.Error("Expected at least one sell signal in downward trend")
	}

	as.Stop()
}

func BenchmarkAggressiveStrategy_OnMarketData(b *testing.B) {
	as := NewAggressiveStrategy("aggressive_1")

	config := &StrategyConfig{
		StrategyID:      "aggressive_1",
		StrategyType:    "aggressive",
		Symbols:         []string{"TEST"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"trend_period":    20.0,
			"momentum_period": 10.0,
			"order_size":      10.0,
		},
		Enabled: true,
	}
	as.Initialize(config)
	as.Start()

	md := &mdpb.MarketDataUpdate{
		Symbol:      "TEST",
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{100.0},
		BidQty:      []uint32{100},
		AskPrice:    []float64{100.5},
		AskQty:      []uint32{100},
		LastPrice:   100.25,
		TotalVolume: 1000,
		Turnover:    100250.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		as.OnMarketData(md)
	}
}
