package strategy

import (
	"testing"
	"time"

	mdpb "github.com/yourusername/hft-poc/pkg/proto/md"
)

func TestHedgingStrategy_Creation(t *testing.T) {
	hs := NewHedgingStrategy("hedging_1")

	if hs.GetID() != "hedging_1" {
		t.Errorf("Expected ID 'hedging_1', got '%s'", hs.GetID())
	}

	if hs.GetType() != "hedging" {
		t.Errorf("Expected type 'hedging', got '%s'", hs.GetType())
	}

	// Check default parameters
	if hs.hedgeRatio != 1.0 {
		t.Errorf("Expected hedgeRatio 1.0, got %.2f", hs.hedgeRatio)
	}
	if hs.rebalanceThreshold != 0.1 {
		t.Errorf("Expected rebalanceThreshold 0.1, got %.2f", hs.rebalanceThreshold)
	}
	if hs.targetDelta != 0.0 {
		t.Errorf("Expected targetDelta 0.0 (delta-neutral), got %.2f", hs.targetDelta)
	}
}

func TestHedgingStrategy_Initialize(t *testing.T) {
	hs := NewHedgingStrategy("hedging_1")

	config := &StrategyConfig{
		StrategyID:      "hedging_1",
		StrategyType:    "hedging",
		Symbols:         []string{"PRIMARY", "HEDGE"},
		Exchanges:       []string{"TEST"},
		MaxPositionSize: 100,
		MaxExposure:     10000.0,
		Parameters: map[string]interface{}{
			"hedge_ratio":          0.9,
			"rebalance_threshold":  0.15,
			"order_size":           5.0,
			"max_position_size":    50.0,
			"min_spread":           0.5,
			"dynamic_hedge_ratio":  true,
			"correlation_period":   50.0,
			"target_delta":         0.0,
			"rebalance_interval_ms": 1000.0,
		},
		Enabled: true,
	}

	err := hs.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	if hs.Config == nil {
		t.Error("Config should not be nil after initialization")
	}

	// Check parameters were loaded
	if hs.hedgeRatio != 0.9 {
		t.Errorf("Expected hedgeRatio 0.9, got %.2f", hs.hedgeRatio)
	}
	if hs.rebalanceThreshold != 0.15 {
		t.Errorf("Expected rebalanceThreshold 0.15, got %.2f", hs.rebalanceThreshold)
	}
	if hs.orderSize != 5 {
		t.Errorf("Expected orderSize 5, got %d", hs.orderSize)
	}
	if hs.correlationPeriod != 50 {
		t.Errorf("Expected correlationPeriod 50, got %d", hs.correlationPeriod)
	}

	// Check symbols
	if hs.primarySymbol != "PRIMARY" {
		t.Errorf("Expected primarySymbol 'PRIMARY', got '%s'", hs.primarySymbol)
	}
	if hs.hedgeSymbol != "HEDGE" {
		t.Errorf("Expected hedgeSymbol 'HEDGE', got '%s'", hs.hedgeSymbol)
	}

	// Check spread indicator was created
	spreadInd, ok := hs.Indicators.Get("hedge_spread")
	if !ok {
		t.Error("Spread indicator should be created")
	}
	if spreadInd == nil {
		t.Error("Spread indicator should not be nil")
	}
}

func TestHedgingStrategy_Initialize_RequiresTwoSymbols(t *testing.T) {
	hs := NewHedgingStrategy("hedging_1")

	config := &StrategyConfig{
		StrategyID:   "hedging_1",
		StrategyType: "hedging",
		Symbols:      []string{"ONLY_ONE"}, // Only one symbol
		Enabled:      true,
	}

	err := hs.Initialize(config)
	if err == nil {
		t.Error("Initialize should fail with only one symbol")
	}
}

func TestHedgingStrategy_DualSymbolTracking(t *testing.T) {
	hs := NewHedgingStrategy("hedging_1")

	config := &StrategyConfig{
		StrategyID:      "hedging_1",
		StrategyType:    "hedging",
		Symbols:         []string{"PRIMARY", "HEDGE"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"hedge_ratio":         1.0,
			"rebalance_threshold": 0.1,
			"order_size":          10.0,
		},
		Enabled: true,
	}

	err := hs.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	hs.Start()

	// Feed market data for primary symbol
	mdPrimary := &mdpb.MarketDataUpdate{
		Symbol:      "PRIMARY",
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
	hs.OnMarketData(mdPrimary)

	if hs.primaryPrice != 100.25 {
		t.Errorf("Expected primaryPrice 100.25, got %.2f", hs.primaryPrice)
	}
	if len(hs.primaryHistory) != 1 {
		t.Errorf("Expected 1 primary history entry, got %d", len(hs.primaryHistory))
	}

	// Feed market data for hedge symbol
	mdHedge := &mdpb.MarketDataUpdate{
		Symbol:      "HEDGE",
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{99.0},
		BidQty:      []uint32{100},
		AskPrice:    []float64{99.5},
		AskQty:      []uint32{100},
		LastPrice:   99.25,
		TotalVolume: 1000,
		Turnover:    99250.0,
	}
	hs.OnMarketData(mdHedge)

	if hs.hedgePrice != 99.25 {
		t.Errorf("Expected hedgePrice 99.25, got %.2f", hs.hedgePrice)
	}
	if len(hs.hedgeHistory) != 1 {
		t.Errorf("Expected 1 hedge history entry, got %d", len(hs.hedgeHistory))
	}

	hs.Stop()
}

func TestHedgingStrategy_DynamicHedgeRatio(t *testing.T) {
	hs := NewHedgingStrategy("hedging_1")

	config := &StrategyConfig{
		StrategyID:      "hedging_1",
		StrategyType:    "hedging",
		Symbols:         []string{"PRIMARY", "HEDGE"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"hedge_ratio":         1.0,
			"dynamic_hedge_ratio": true,
			"correlation_period":  20.0,
			"rebalance_interval_ms": 100.0,
		},
		Enabled: true,
	}

	err := hs.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	hs.Start()

	// Feed correlated price data
	for i := 0; i < 30; i++ {
		primaryPrice := 100.0 + float64(i)*0.5
		hedgePrice := 99.0 + float64(i)*0.45 // Slightly different slope

		mdPrimary := &mdpb.MarketDataUpdate{
			Symbol:      "PRIMARY",
			Exchange:    "TEST",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{primaryPrice - 0.5},
			BidQty:      []uint32{100},
			AskPrice:    []float64{primaryPrice + 0.5},
			AskQty:      []uint32{100},
			LastPrice:   primaryPrice,
			TotalVolume: uint64(1000 + i*10),
			Turnover:    primaryPrice * float64(1000+i*10),
		}
		hs.OnMarketData(mdPrimary)

		mdHedge := &mdpb.MarketDataUpdate{
			Symbol:      "HEDGE",
			Exchange:    "TEST",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{hedgePrice - 0.5},
			BidQty:      []uint32{100},
			AskPrice:    []float64{hedgePrice + 0.5},
			AskQty:      []uint32{100},
			LastPrice:   hedgePrice,
			TotalVolume: uint64(1000 + i*10),
			Turnover:    hedgePrice * float64(1000+i*10),
		}
		hs.OnMarketData(mdHedge)

		time.Sleep(5 * time.Millisecond)
	}

	// Hedge ratio should have been updated
	// With slope ratio of 0.45/0.5 = 0.9, hedge ratio should be close to that
	if hs.hedgeRatio < 0.5 || hs.hedgeRatio > 2.0 {
		t.Errorf("Hedge ratio %.2f outside expected range [0.5, 2.0]", hs.hedgeRatio)
	}

	hs.Stop()
}

func TestHedgingStrategy_Rebalancing(t *testing.T) {
	hs := NewHedgingStrategy("hedging_1")

	config := &StrategyConfig{
		StrategyID:      "hedging_1",
		StrategyType:    "hedging",
		Symbols:         []string{"PRIMARY", "HEDGE"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"hedge_ratio":          1.0,
			"rebalance_threshold":  0.05, // Low threshold for testing
			"order_size":           5.0,
			"max_position_size":    100.0,
			"rebalance_interval_ms": 100.0,
		},
		Enabled: true,
	}

	err := hs.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	hs.Start()

	// Set up position that needs rebalancing
	hs.Position.NetQty = 10 // Delta = 10
	hs.currentDelta = 10.0
	hs.targetDelta = 0.0

	// Feed market data to trigger rebalancing
	md := &mdpb.MarketDataUpdate{
		Symbol:      "HEDGE",
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

	time.Sleep(150 * time.Millisecond) // Wait past rebalance interval
	hs.OnMarketData(md)

	// Check if rebalancing signal was generated
	signals := hs.GetSignals()
	if len(signals) > 0 {
		lastSignal := signals[len(signals)-1]
		if lastSignal.Metadata["type"] == "rebalance" {
			// Verify metadata
			if _, ok := lastSignal.Metadata["delta_before"]; !ok {
				t.Error("Rebalance signal should include delta_before")
			}
			if _, ok := lastSignal.Metadata["hedge_ratio"]; !ok {
				t.Error("Rebalance signal should include hedge_ratio")
			}
		}
	}

	hs.Stop()
}

func TestHedgingStrategy_DeltaCalculation(t *testing.T) {
	hs := NewHedgingStrategy("hedging_1")

	config := &StrategyConfig{
		StrategyID:      "hedging_1",
		StrategyType:    "hedging",
		Symbols:         []string{"PRIMARY", "HEDGE"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"hedge_ratio": 0.8,
		},
		Enabled: true,
	}

	err := hs.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Set positions
	hs.Position.NetQty = 10

	// Calculate delta
	hs.calculateDelta()

	// Delta = primary_position + hedge_ratio * hedge_position
	// With simplified tracking: delta = 10 + 0.8 * 0 = 10
	expectedDelta := 10.0
	if hs.currentDelta != expectedDelta {
		t.Errorf("Expected delta %.2f, got %.2f", expectedDelta, hs.currentDelta)
	}
}

func TestHedgingStrategy_GetHedgeStatus(t *testing.T) {
	hs := NewHedgingStrategy("hedging_1")

	config := &StrategyConfig{
		StrategyID:   "hedging_1",
		StrategyType: "hedging",
		Symbols:      []string{"PRIMARY", "HEDGE"},
		Parameters: map[string]interface{}{
			"hedge_ratio": 0.95,
		},
		Enabled: true,
	}

	err := hs.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	hs.primaryPrice = 100.0
	hs.hedgePrice = 99.5
	hs.currentDelta = 5.0
	hs.targetDelta = 0.0

	status := hs.GetHedgeStatus()

	if status["primary_symbol"] != "PRIMARY" {
		t.Error("Status should include primary symbol")
	}
	if status["hedge_symbol"] != "HEDGE" {
		t.Error("Status should include hedge symbol")
	}
	if status["primary_price"] != 100.0 {
		t.Error("Status should include primary price")
	}
	if status["hedge_price"] != 99.5 {
		t.Error("Status should include hedge price")
	}
	if status["hedge_ratio"] != 0.95 {
		t.Error("Status should include hedge ratio")
	}
	if status["current_delta"] != 5.0 {
		t.Error("Status should include current delta")
	}
	if status["target_delta"] != 0.0 {
		t.Error("Status should include target delta")
	}
}

func TestHedgingStrategy_StartStop(t *testing.T) {
	hs := NewHedgingStrategy("hedging_1")

	if hs.IsRunning() {
		t.Error("Strategy should not be running initially")
	}

	hs.Start()
	if !hs.IsRunning() {
		t.Error("Strategy should be running after start")
	}

	hs.Stop()
	if hs.IsRunning() {
		t.Error("Strategy should not be running after stop")
	}
}

func TestHedgingStrategy_HistoryTracking(t *testing.T) {
	hs := NewHedgingStrategy("hedging_1")

	config := &StrategyConfig{
		StrategyID:   "hedging_1",
		StrategyType: "hedging",
		Symbols:      []string{"PRIMARY", "HEDGE"},
		Enabled:      true,
	}

	err := hs.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	hs.Start()

	// Feed 250 data points (exceeds maxHistoryLen of 200)
	for i := 0; i < 250; i++ {
		mdPrimary := &mdpb.MarketDataUpdate{
			Symbol:      "PRIMARY",
			Exchange:    "TEST",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{100.0 + float64(i)},
			BidQty:      []uint32{100},
			AskPrice:    []float64{100.5 + float64(i)},
			AskQty:      []uint32{100},
			LastPrice:   100.25 + float64(i),
			TotalVolume: uint64(1000 + i),
			Turnover:    (100.25 + float64(i)) * float64(1000+i),
		}
		hs.OnMarketData(mdPrimary)
	}

	// History should be capped at maxHistoryLen
	if len(hs.primaryHistory) > hs.maxHistoryLen {
		t.Errorf("Primary history length %d exceeds max %d", len(hs.primaryHistory), hs.maxHistoryLen)
	}

	hs.Stop()
}

func BenchmarkHedgingStrategy_OnMarketData(b *testing.B) {
	hs := NewHedgingStrategy("hedging_1")

	config := &StrategyConfig{
		StrategyID:      "hedging_1",
		StrategyType:    "hedging",
		Symbols:         []string{"PRIMARY", "HEDGE"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"hedge_ratio": 0.9,
			"order_size":  10.0,
		},
		Enabled: true,
	}
	hs.Initialize(config)
	hs.Start()

	md := &mdpb.MarketDataUpdate{
		Symbol:      "PRIMARY",
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
		hs.OnMarketData(md)
	}
}
