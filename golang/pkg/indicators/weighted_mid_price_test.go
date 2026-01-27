package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestWeightedMidPrice(t *testing.T) {
	indicator := NewWeightedMidPrice(1, 100)

	// Test with valid bid/ask and volumes
	// Bid: 100 @ 10, Ask: 101 @ 20
	// Weighted mid = (100*20 + 101*10) / (10 + 20) = 3010 / 30 = 100.33
	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0},
		AskPrice:  []float64{101.0},
		BidQty:    []uint32{10},
		AskQty:    []uint32{20},
	}

	indicator.Update(md)

	expected := 100.33333333333333
	got := indicator.GetValue()

	if abs(got-expected) > 0.0001 {
		t.Errorf("Expected weighted mid price %f, got %f", expected, got)
	}

	if !indicator.IsReady() {
		t.Error("Indicator should be ready after first update")
	}
}

func TestWeightedMidPriceMultiLevel(t *testing.T) {
	indicator := NewWeightedMidPrice(2, 100)

	// Multi-level orderbook
	// Bid: [100 @ 10, 99 @ 5], Ask: [101 @ 20, 102 @ 15]
	// Total bid vol: 15, Total ask vol: 35
	// Weighted mid = (100*20 + 99*20 + 101*10 + 102*10) / (15 + 35)
	//              = (2000 + 1980 + 1010 + 1020) / 50 = 6010 / 50 = 120.2
	// Wait, the formula should be:
	// bidWeightedPrice = 100*20 + 99*15 = 2000 + 1485 = 3485
	// askWeightedPrice = 101*10 + 102*5 = 1010 + 510 = 1520
	// Total = (3485 + 1520) / 50 = 5005 / 50 = 100.1
	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0, 99.0},
		AskPrice:  []float64{101.0, 102.0},
		BidQty:    []uint32{10, 5},
		AskQty:    []uint32{20, 15},
	}

	indicator.Update(md)

	// Weighted: bidWeighted = 100*20 + 99*15 = 3485
	//          askWeighted = 101*10 + 102*5 = 1520
	//          total = (3485 + 1520) / 50 = 100.1
	expected := 100.1
	got := indicator.GetValue()

	if abs(got-expected) > 0.0001 {
		t.Errorf("Expected weighted mid price %f, got %f", expected, got)
	}
}

func TestWeightedMidPriceEmpty(t *testing.T) {
	indicator := NewWeightedMidPrice(1, 100)

	// Test with empty data
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

	if indicator.IsReady() {
		t.Error("Indicator should not be ready with empty data")
	}
}

func TestWeightedMidPriceZeroVolume(t *testing.T) {
	indicator := NewWeightedMidPrice(1, 100)

	// Test with zero volumes - should fallback to simple mid price
	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0},
		AskPrice:  []float64{101.0},
		BidQty:    []uint32{0},
		AskQty:    []uint32{0},
	}

	indicator.Update(md)

	// Should fallback to simple mid price
	expected := 100.5
	got := indicator.GetValue()

	if abs(got-expected) > 0.0001 {
		t.Errorf("Expected mid price (fallback) %f, got %f", expected, got)
	}
}

func TestWeightedMidPriceFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"levels":      float64(3),
		"max_history": float64(500),
	}

	ind, err := NewWeightedMidPriceFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	indicator := ind.(*WeightedMidPrice)
	if indicator.levels != 3 {
		t.Errorf("Expected levels 3, got %d", indicator.levels)
	}
	if indicator.maxHistory != 500 {
		t.Errorf("Expected max_history 500, got %d", indicator.maxHistory)
	}
}

func TestWeightedMidPriceFromConfigInvalidLevels(t *testing.T) {
	config := map[string]interface{}{
		"levels": float64(-1),
	}

	_, err := NewWeightedMidPriceFromConfig(config)
	if err == nil {
		t.Error("Expected error for invalid levels")
	}
}

func TestWeightedMidPriceReset(t *testing.T) {
	indicator := NewWeightedMidPrice(1, 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0},
		AskPrice:  []float64{101.0},
		BidQty:    []uint32{10},
		AskQty:    []uint32{20},
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

func TestWeightedMidPriceHistory(t *testing.T) {
	indicator := NewWeightedMidPrice(1, 3)

	prices := []struct {
		bid    float64
		ask    float64
		bidQty uint32
		askQty uint32
	}{
		{100.0, 101.0, 10, 20},
		{102.0, 103.0, 15, 25},
		{104.0, 105.0, 12, 18},
		{106.0, 107.0, 20, 10}, // This should push out the first value
	}

	for _, p := range prices {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: 1000,
			BidPrice:  []float64{p.bid},
			AskPrice:  []float64{p.ask},
			BidQty:    []uint32{p.bidQty},
			AskQty:    []uint32{p.askQty},
		}
		indicator.Update(md)
	}

	// Should only keep last 3 values
	values := indicator.GetValues()
	if len(values) != 3 {
		t.Errorf("Expected 3 values in history, got %d", len(values))
	}
}

func BenchmarkWeightedMidPrice(b *testing.B) {
	indicator := NewWeightedMidPrice(1, 1000)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0},
		AskPrice:  []float64{101.0},
		BidQty:    []uint32{10},
		AskQty:    []uint32{20},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indicator.Update(md)
	}
}

func BenchmarkWeightedMidPriceMultiLevel(b *testing.B) {
	indicator := NewWeightedMidPrice(5, 1000)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0, 99.0, 98.0, 97.0, 96.0},
		AskPrice:  []float64{101.0, 102.0, 103.0, 104.0, 105.0},
		BidQty:    []uint32{10, 8, 6, 4, 2},
		AskQty:    []uint32{20, 15, 10, 8, 5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indicator.Update(md)
	}
}
