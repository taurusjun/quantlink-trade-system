package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestLiquidityRatio(t *testing.T) {
	indicator := NewLiquidityRatio(5, false, 0.0001, 100)

	// Total volume: 300 + 250 = 550
	// Spread: 101 - 100 = 1
	// Ratio: 550 / 1 = 550
	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0, 99.0, 98.0, 97.0, 96.0},
		AskPrice:  []float64{101.0, 102.0, 103.0, 104.0, 105.0},
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{90, 70, 50, 30, 10},
	}

	indicator.Update(md)

	expected := 550.0
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.01 {
		t.Errorf("Expected liquidity ratio %f, got %f", expected, got)
	}
}

func TestLiquidityRatioNormalized(t *testing.T) {
	indicator := NewLiquidityRatio(5, true, 0.0001, 100)

	// Total volume: 550
	// Spread: 1
	// Mid price: 100.5
	// Normalized spread: 1 / 100.5 = 0.00995
	// Ratio: 550 / 0.00995 = 55276.4
	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0, 99.0, 98.0, 97.0, 96.0},
		AskPrice:  []float64{101.0, 102.0, 103.0, 104.0, 105.0},
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{90, 70, 50, 30, 10},
	}

	indicator.Update(md)

	expected := 55276.4
	got := indicator.GetValue()

	if math.Abs(got-expected) > 2.0 {
		t.Errorf("Expected normalized liquidity ratio ~%f, got %f", expected, got)
	}
}

func TestLiquidityRatioLimitedLevels(t *testing.T) {
	indicator := NewLiquidityRatio(2, false, 0.0001, 100)

	// Only first 2 levels: (100+80) + (90+70) = 340
	// Spread: 1
	// Ratio: 340
	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0, 99.0, 98.0, 97.0, 96.0},
		AskPrice:  []float64{101.0, 102.0, 103.0, 104.0, 105.0},
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{90, 70, 50, 30, 10},
	}

	indicator.Update(md)

	expected := 340.0
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.01 {
		t.Errorf("Expected liquidity ratio %f, got %f", expected, got)
	}
}

func TestLiquidityRatioMinSpread(t *testing.T) {
	indicator := NewLiquidityRatio(1, false, 1.0, 100)

	// Spread is 0.1, but min_spread is 1.0
	// Should use min_spread
	// Volume: 100 + 90 = 190
	// Ratio: 190 / 1.0 = 190
	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0},
		AskPrice:  []float64{100.1},
		BidQty:    []uint32{100},
		AskQty:    []uint32{90},
	}

	indicator.Update(md)

	expected := 190.0
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.01 {
		t.Errorf("Expected liquidity ratio with min_spread %f, got %f", expected, got)
	}
}

func TestLiquidityRatioEmpty(t *testing.T) {
	indicator := NewLiquidityRatio(5, false, 0.0001, 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{},
		AskPrice:  []float64{},
		BidQty:    []uint32{},
		AskQty:    []uint32{},
	}

	indicator.Update(md)

	// With empty data, spread is 0, volume is 0
	// Should use min_spread
	// Ratio: 0 / min_spread = 0
	expected := 0.0
	got := indicator.GetValue()

	if got != expected {
		t.Errorf("Expected zero liquidity ratio %f, got %f", expected, got)
	}
}

func TestLiquidityRatioFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"levels":      float64(10),
		"normalized":  false,
		"min_spread":  float64(0.001),
		"max_history": float64(500),
	}

	ind, err := NewLiquidityRatioFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	indicator := ind.(*LiquidityRatio)
	if indicator.levels != 10 {
		t.Errorf("Expected levels 10, got %d", indicator.levels)
	}
	if indicator.normalized {
		t.Error("Expected normalized to be false")
	}
	if math.Abs(indicator.minSpread-0.001) > 0.0001 {
		t.Errorf("Expected min_spread 0.001, got %f", indicator.minSpread)
	}
	if indicator.maxHistory != 500 {
		t.Errorf("Expected max_history 500, got %d", indicator.maxHistory)
	}
}

func TestLiquidityRatioFromConfigInvalidLevels(t *testing.T) {
	config := map[string]interface{}{
		"levels": float64(-1),
	}

	_, err := NewLiquidityRatioFromConfig(config)
	if err == nil {
		t.Error("Expected error for invalid levels")
	}
}

func TestLiquidityRatioReset(t *testing.T) {
	indicator := NewLiquidityRatio(5, false, 0.0001, 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0},
		AskPrice:  []float64{101.0},
		BidQty:    []uint32{100},
		AskQty:    []uint32{90},
	}

	indicator.Update(md)

	if !indicator.IsReady() {
		t.Error("Indicator should be ready after update")
	}

	indicator.Reset()

	if indicator.IsReady() {
		t.Error("Indicator should not be ready after reset")
	}

	values := indicator.GetValues()
	if len(values) != 0 {
		t.Errorf("Expected 0 values after reset, got %d", len(values))
	}
}

func TestLiquidityRatioHistory(t *testing.T) {
	indicator := NewLiquidityRatio(2, false, 0.0001, 3)

	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: uint64(1000 + i),
			BidPrice:  []float64{100.0 + float64(i), 99.0 + float64(i)},
			AskPrice:  []float64{101.0 + float64(i), 102.0 + float64(i)},
			BidQty:    []uint32{100, 80},
			AskQty:    []uint32{90, 70},
		}
		indicator.Update(md)
	}

	// Should only keep last 3 values
	values := indicator.GetValues()
	if len(values) != 3 {
		t.Errorf("Expected 3 values in history, got %d", len(values))
	}
}

func BenchmarkLiquidityRatio(b *testing.B) {
	indicator := NewLiquidityRatio(5, false, 0.0001, 1000)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0, 99.0, 98.0, 97.0, 96.0},
		AskPrice:  []float64{101.0, 102.0, 103.0, 104.0, 105.0},
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{90, 70, 50, 30, 10},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indicator.Update(md)
	}
}

func BenchmarkLiquidityRatioNormalized(b *testing.B) {
	indicator := NewLiquidityRatio(10, true, 0.0001, 1000)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0, 99.0, 98.0, 97.0, 96.0, 95.0, 94.0, 93.0, 92.0, 91.0},
		AskPrice:  []float64{101.0, 102.0, 103.0, 104.0, 105.0, 106.0, 107.0, 108.0, 109.0, 110.0},
		BidQty:    []uint32{100, 90, 80, 70, 60, 50, 40, 30, 20, 10},
		AskQty:    []uint32{95, 85, 75, 65, 55, 45, 35, 25, 15, 5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indicator.Update(md)
	}
}
