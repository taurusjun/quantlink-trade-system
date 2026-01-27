package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestBidAskRatio(t *testing.T) {
	indicator := NewBidAskRatio(5, false, 0.01, 100)

	// Bid volume: 300, Ask volume: 250
	// Ratio: 300 / 250 = 1.2
	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{90, 70, 50, 30, 10},
	}

	indicator.Update(md)

	expected := 1.2
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.001 {
		t.Errorf("Expected bid-ask ratio %f, got %f", expected, got)
	}
}

func TestBidAskRatioLog(t *testing.T) {
	indicator := NewBidAskRatio(5, true, 0.01, 100)

	// Bid volume: 300, Ask volume: 250
	// Log ratio: log(300/250) = log(1.2) ≈ 0.1823
	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{90, 70, 50, 30, 10},
	}

	indicator.Update(md)

	expected := 0.1823
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.001 {
		t.Errorf("Expected log bid-ask ratio ~%f, got %f", expected, got)
	}

	// Check symmetry: log(a/b) = -log(b/a)
	// Create indicator with swapped volumes
	indicator2 := NewBidAskRatio(5, true, 0.01, 100)

	md2 := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{90, 70, 50, 30, 10},
		AskQty:    []uint32{100, 80, 60, 40, 20},
	}

	indicator2.Update(md2)

	got2 := indicator2.GetValue()

	// Should be approximately -got
	if math.Abs(got2+got) > 0.001 {
		t.Errorf("Log ratio should be symmetric: got %f and %f", got, got2)
	}
}

func TestBidAskRatioBalanced(t *testing.T) {
	indicator := NewBidAskRatio(5, false, 0.01, 100)

	// Equal volumes
	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{100, 80, 60, 40, 20},
	}

	indicator.Update(md)

	expected := 1.0
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.001 {
		t.Errorf("Expected balanced ratio %f, got %f", expected, got)
	}
}

func TestBidAskRatioLogBalanced(t *testing.T) {
	indicator := NewBidAskRatio(5, true, 0.01, 100)

	// Equal volumes - log(1) = 0
	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{100, 80, 60, 40, 20},
	}

	indicator.Update(md)

	expected := 0.0
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.001 {
		t.Errorf("Expected log ratio 0 for balanced book, got %f", got)
	}
}

func TestBidAskRatioLimitedLevels(t *testing.T) {
	indicator := NewBidAskRatio(2, false, 0.01, 100)

	// Only first 2 levels: bid: 180, ask: 160
	// Ratio: 180 / 160 = 1.125
	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{90, 70, 50, 30, 10},
	}

	indicator.Update(md)

	expected := 1.125
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.001 {
		t.Errorf("Expected ratio with 2 levels %f, got %f", expected, got)
	}
}

func TestBidAskRatioZeroAsk(t *testing.T) {
	indicator := NewBidAskRatio(5, false, 1.0, 100)

	// Ask volume is 0, should use epsilon
	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{0, 0, 0, 0, 0},
	}

	indicator.Update(md)

	// Bid: 300, Ask: epsilon (1.0)
	// Ratio: 300 / 1.0 = 300
	expected := 300.0
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.001 {
		t.Errorf("Expected ratio with epsilon %f, got %f", expected, got)
	}
}

func TestBidAskRatioEmpty(t *testing.T) {
	indicator := NewBidAskRatio(5, false, 0.01, 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{},
		AskQty:    []uint32{},
	}

	indicator.Update(md)

	// Bid: 0, Ask: epsilon
	// Ratio: 0 / epsilon = 0
	expected := 0.0
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.001 {
		t.Errorf("Expected zero ratio %f, got %f", expected, got)
	}
}

func TestBidAskRatioFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"levels":      float64(10),
		"use_log":     true,
		"epsilon":     float64(0.1),
		"max_history": float64(500),
	}

	ind, err := NewBidAskRatioFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	indicator := ind.(*BidAskRatio)
	if indicator.levels != 10 {
		t.Errorf("Expected levels 10, got %d", indicator.levels)
	}
	if !indicator.useLog {
		t.Error("Expected use_log to be true")
	}
	if math.Abs(indicator.epsilon-0.1) > 0.001 {
		t.Errorf("Expected epsilon 0.1, got %f", indicator.epsilon)
	}
	if indicator.maxHistory != 500 {
		t.Errorf("Expected max_history 500, got %d", indicator.maxHistory)
	}
}

func TestBidAskRatioFromConfigInvalidLevels(t *testing.T) {
	config := map[string]interface{}{
		"levels": float64(-1),
	}

	_, err := NewBidAskRatioFromConfig(config)
	if err == nil {
		t.Error("Expected error for invalid levels")
	}
}

func TestBidAskRatioReset(t *testing.T) {
	indicator := NewBidAskRatio(5, false, 0.01, 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{90, 70, 50, 30, 10},
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

func TestBidAskRatioHistory(t *testing.T) {
	indicator := NewBidAskRatio(2, false, 0.01, 3)

	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: uint64(1000 + i),
			BidQty:    []uint32{100 + uint32(i*10), 80 + uint32(i*10)},
			AskQty:    []uint32{90 + uint32(i*10), 70 + uint32(i*10)},
		}
		indicator.Update(md)
	}

	// Should only keep last 3 values
	values := indicator.GetValues()
	if len(values) != 3 {
		t.Errorf("Expected 3 values in history, got %d", len(values))
	}
}

func BenchmarkBidAskRatio(b *testing.B) {
	indicator := NewBidAskRatio(5, false, 0.01, 1000)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{90, 70, 50, 30, 10},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indicator.Update(md)
	}
}

func BenchmarkBidAskRatioLog(b *testing.B) {
	indicator := NewBidAskRatio(10, true, 0.01, 1000)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 90, 80, 70, 60, 50, 40, 30, 20, 10},
		AskQty:    []uint32{95, 85, 75, 65, 55, 45, 35, 25, 15, 5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indicator.Update(md)
	}
}
