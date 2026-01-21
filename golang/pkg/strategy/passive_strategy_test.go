package strategy

import (
	"testing"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestPassiveStrategy_Creation(t *testing.T) {
	ps := NewPassiveStrategy("passive_1")

	if ps.GetID() != "passive_1" {
		t.Errorf("Expected ID 'passive_1', got '%s'", ps.GetID())
	}

	if ps.GetType() != "passive" {
		t.Errorf("Expected type 'passive', got '%s'", ps.GetType())
	}
}

func TestPassiveStrategy_Initialize(t *testing.T) {
	ps := NewPassiveStrategy("passive_1")

	config := &StrategyConfig{
		StrategyID:      "passive_1",
		StrategyType:    "passive",
		Symbols:         []string{"TEST"},
		Exchanges:       []string{"TEST"},
		MaxPositionSize: 100,
		MaxExposure:     10000.0,
		Parameters: map[string]interface{}{
			"spread_multiplier":   0.5,
			"order_size":          10.0,
			"max_inventory":       100.0,
			"inventory_skew":      0.5,
			"min_spread":          1.0,
			"order_refresh_ms":    1000.0,
			"use_order_imbalance": true,
		},
		Enabled: true,
	}

	err := ps.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	if ps.Config == nil {
		t.Error("Config should not be nil after initialization")
	}
}

func TestPassiveStrategy_SignalGeneration(t *testing.T) {
	ps := NewPassiveStrategy("passive_1")

	config := &StrategyConfig{
		StrategyID:      "passive_1",
		StrategyType:    "passive",
		Symbols:         []string{"TEST"},
		Exchanges:       []string{"TEST"},
		MaxPositionSize: 100,
		MaxExposure:     10000.0,
		Parameters: map[string]interface{}{
			"spread_multiplier":   0.5,
			"order_size":          10.0,
			"max_inventory":       100.0,
			"inventory_skew":      0.5,
			"min_spread":          0.1, // Lower for testing (test spread is 0.5)
			"order_refresh_ms":    100.0, // Short interval for testing
			"use_order_imbalance": true,
		},
		Enabled: true,
	}

	err := ps.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	ps.Start()

	// Feed market data
	for i := 0; i < 50; i++ {
		md := &mdpb.MarketDataUpdate{
			Symbol:      "TEST",
			Exchange:    "TEST",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{100.0, 99.5, 99.0, 98.5, 98.0},
			BidQty:      []uint32{100, 80, 60, 40, 20},
			AskPrice:    []float64{100.5, 101.0, 101.5, 102.0, 102.5},
			AskQty:      []uint32{95, 75, 55, 45, 35},
			LastPrice:   100.25,
			TotalVolume: uint64(1000 + i*10),
			Turnover:    100.25 * float64(1000+i*10),
		}
		ps.OnMarketData(md)
		time.Sleep(10 * time.Millisecond)
	}

	// Wait for signal generation to complete
	time.Sleep(200 * time.Millisecond)

	// Should have generated some signals
	signals := ps.GetSignals()
	if len(signals) == 0 {
		t.Error("Expected some signals to be generated")
	}

	ps.Stop()
}

func TestPassiveStrategy_InventoryManagement(t *testing.T) {
	ps := NewPassiveStrategy("passive_1")

	config := &StrategyConfig{
		StrategyID:      "passive_1",
		StrategyType:    "passive",
		Symbols:         []string{"TEST"},
		Exchanges:       []string{"TEST"},
		MaxPositionSize: 10, // Small limit for testing
		MaxExposure:     10000.0,
		Parameters: map[string]interface{}{
			"spread_multiplier":   0.5,
			"order_size":          5.0,
			"max_inventory":       10.0,
			"inventory_skew":      0.5,
			"min_spread":          1.0,
			"order_refresh_ms":    100.0,
			"use_order_imbalance": true,
		},
		Enabled: true,
	}

	err := ps.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Set position near max
	ps.Position.NetQty = 9
	ps.Position.LongQty = 9

	ps.Start()

	// Feed market data
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

	ps.OnMarketData(md)

	// Strategy should adapt to large inventory
	// (具体行为取决于策略实现)

	ps.Stop()
}

func TestPassiveStrategy_StartStop(t *testing.T) {
	ps := NewPassiveStrategy("passive_1")

	if ps.IsRunning() {
		t.Error("Strategy should not be running initially")
	}

	ps.Start()
	if !ps.IsRunning() {
		t.Error("Strategy should be running after start")
	}

	ps.Stop()
	if ps.IsRunning() {
		t.Error("Strategy should not be running after stop")
	}
}

func TestPassiveStrategy_Reset(t *testing.T) {
	ps := NewPassiveStrategy("passive_1")

	config := &StrategyConfig{
		StrategyID:   "passive_1",
		StrategyType: "passive",
		Symbols:      []string{"TEST"},
		Parameters: map[string]interface{}{
			"order_size": 10.0,
		},
	}
	ps.Initialize(config)

	// Add some state
	ps.AddSignal(&TradingSignal{
		StrategyID: "passive_1",
		Symbol:     "TEST",
		Price:      100.0,
		Quantity:   10,
	})

	// Reset
	ps.Reset()

	// Check state is cleared
	if len(ps.GetSignals()) != 0 {
		t.Error("Signals should be cleared after reset")
	}
}

func BenchmarkPassiveStrategy_OnMarketData(b *testing.B) {
	ps := NewPassiveStrategy("passive_1")

	config := &StrategyConfig{
		StrategyID:      "passive_1",
		StrategyType:    "passive",
		Symbols:         []string{"TEST"},
		Exchanges:       []string{"TEST"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"spread_multiplier": 0.5,
			"order_size":        10.0,
			"max_inventory":     100.0,
		},
		Enabled: true,
	}
	ps.Initialize(config)
	ps.Start()

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
		ps.OnMarketData(md)
	}
}
