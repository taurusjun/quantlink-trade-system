package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestPriceImpactBuy(t *testing.T) {
	// Simulate buying 150 units
	// Orderbook: Ask: [100@101, 200@102]
	// Fill: 100@101 + 50@102 = 10100 + 5100 = 15200
	// Avg price: 15200 / 150 = 101.33
	// Mid price: (100+101)/2 = 100.5
	// Relative impact: (101.33 - 100.5) / 100.5 = 0.826%
	indicator := NewPriceImpact(150, "buy", true, 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0},
		AskPrice:  []float64{101.0, 102.0},
		BidQty:    []uint32{200},
		AskQty:    []uint32{100, 200},
	}

	indicator.Update(md)

	expected := 0.00826 // Approximately 0.826%
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.0001 {
		t.Errorf("Expected relative impact %f, got %f", expected, got)
	}
}

func TestPriceImpactSell(t *testing.T) {
	// Simulate selling 150 units
	// Orderbook: Bid: [100@100, 200@99]
	// Fill: 100@100 + 50@99 = 10000 + 4950 = 14950
	// Avg price: 14950 / 150 = 99.67
	// Mid price: (100+101)/2 = 100.5
	// Relative impact: (99.67 - 100.5) / 100.5 = -0.826%
	indicator := NewPriceImpact(150, "sell", true, 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0, 99.0},
		AskPrice:  []float64{101.0},
		BidQty:    []uint32{100, 200},
		AskQty:    []uint32{200},
	}

	indicator.Update(md)

	expected := -0.00826 // Approximately -0.826%
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.0001 {
		t.Errorf("Expected relative impact %f, got %f", expected, got)
	}
}

func TestPriceImpactAbsolute(t *testing.T) {
	// Test absolute impact (not relative)
	indicator := NewPriceImpact(150, "buy", false, 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0},
		AskPrice:  []float64{101.0, 102.0},
		BidQty:    []uint32{200},
		AskQty:    []uint32{100, 200},
	}

	indicator.Update(md)

	// Avg exec: 101.33, Mid: 100.5, Absolute impact: 0.83
	expected := 0.8333
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.01 {
		t.Errorf("Expected absolute impact %f, got %f", expected, got)
	}
}

func TestPriceImpactInsufficientLiquidity(t *testing.T) {
	// Order size exceeds available liquidity
	indicator := NewPriceImpact(500, "buy", true, 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0},
		AskPrice:  []float64{101.0, 102.0},
		BidQty:    []uint32{200},
		AskQty:    []uint32{100, 150}, // Total: 250, need 500
	}

	indicator.Update(md)

	// Should use last price for remaining volume
	// Fill: 100@101 + 150@102 = 10100 + 15300 = 25400
	// Remaining: 250@102 = 25500
	// Total: 50900, Avg: 101.8
	// Mid: 100.5, Impact: (101.8-100.5)/100.5 = 1.29%
	got := indicator.GetValue()

	if got <= 0 {
		t.Error("Should have positive impact even with insufficient liquidity")
	}
}

func TestPriceImpactEmpty(t *testing.T) {
	indicator := NewPriceImpact(100, "buy", true, 100)

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

func TestPriceImpactFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"volume":      float64(200),
		"side":        "sell",
		"relative":    false,
		"max_history": float64(500),
	}

	ind, err := NewPriceImpactFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	indicator := ind.(*PriceImpact)
	if indicator.volume != 200 {
		t.Errorf("Expected volume 200, got %f", indicator.volume)
	}
	if indicator.side != "sell" {
		t.Errorf("Expected side 'sell', got '%s'", indicator.side)
	}
	if indicator.relative {
		t.Error("Expected relative to be false")
	}
	if indicator.maxHistory != 500 {
		t.Errorf("Expected max_history 500, got %d", indicator.maxHistory)
	}
}

func TestPriceImpactFromConfigInvalidVolume(t *testing.T) {
	config := map[string]interface{}{
		"volume": float64(-10),
	}

	_, err := NewPriceImpactFromConfig(config)
	if err == nil {
		t.Error("Expected error for invalid volume")
	}
}

func TestPriceImpactFromConfigInvalidSide(t *testing.T) {
	config := map[string]interface{}{
		"side": "invalid",
	}

	_, err := NewPriceImpactFromConfig(config)
	if err == nil {
		t.Error("Expected error for invalid side")
	}
}

func TestPriceImpactSetVolume(t *testing.T) {
	indicator := NewPriceImpact(100, "buy", true, 100)

	if indicator.GetVolume() != 100 {
		t.Errorf("Expected initial volume 100, got %f", indicator.GetVolume())
	}

	indicator.SetVolume(200)

	if indicator.GetVolume() != 200 {
		t.Errorf("Expected updated volume 200, got %f", indicator.GetVolume())
	}

	// Should not update with invalid volume
	indicator.SetVolume(-50)

	if indicator.GetVolume() != 200 {
		t.Error("Volume should not change with invalid value")
	}
}

func TestPriceImpactReset(t *testing.T) {
	indicator := NewPriceImpact(100, "buy", true, 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0},
		AskPrice:  []float64{101.0},
		BidQty:    []uint32{200},
		AskQty:    []uint32{200},
	}

	indicator.Update(md)

	if !indicator.IsReady() {
		t.Error("Indicator should be ready after update")
	}

	indicator.Reset()

	if indicator.IsReady() {
		t.Error("Indicator should not be ready after reset")
	}
}

func TestPriceImpactHistory(t *testing.T) {
	indicator := NewPriceImpact(100, "buy", true, 3)

	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: uint64(1000 + i),
			BidPrice:  []float64{100.0 + float64(i)},
			AskPrice:  []float64{101.0 + float64(i)},
			BidQty:    []uint32{200},
			AskQty:    []uint32{200},
		}
		indicator.Update(md)
	}

	// Should only keep last 3 values
	values := indicator.GetValues()
	if len(values) != 3 {
		t.Errorf("Expected 3 values in history, got %d", len(values))
	}
}

func BenchmarkPriceImpactBuy(b *testing.B) {
	indicator := NewPriceImpact(100, "buy", true, 1000)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0},
		AskPrice:  []float64{101.0, 102.0, 103.0, 104.0, 105.0},
		BidQty:    []uint32{200},
		AskQty:    []uint32{50, 50, 50, 50, 50},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indicator.Update(md)
	}
}

func BenchmarkPriceImpactSell(b *testing.B) {
	indicator := NewPriceImpact(100, "sell", true, 1000)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0, 99.0, 98.0, 97.0, 96.0},
		AskPrice:  []float64{101.0},
		BidQty:    []uint32{50, 50, 50, 50, 50},
		AskQty:    []uint32{200},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indicator.Update(md)
	}
}
