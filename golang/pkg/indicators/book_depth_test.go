package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestBookDepth_Basic(t *testing.T) {
	bd := NewBookDepth("test_book_depth", 5, "both", 100)

	// Test initial state
	if bd.GetBidDepth() != 0 {
		t.Errorf("Expected initial bid depth 0, got %.2f", bd.GetBidDepth())
	}
	if bd.GetAskDepth() != 0 {
		t.Errorf("Expected initial ask depth 0, got %.2f", bd.GetAskDepth())
	}

	// Create market data with 5 levels
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:   []uint32{10, 20, 30, 40, 50},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		AskQty:   []uint32{15, 25, 35, 45, 55},
	}

	bd.Update(md)

	// Check bid depth (10+20+30+40+50 = 150)
	expectedBid := 150.0
	if bd.GetBidDepth() != expectedBid {
		t.Errorf("Expected bid depth %.2f, got %.2f", expectedBid, bd.GetBidDepth())
	}

	// Check ask depth (15+25+35+45+55 = 175)
	expectedAsk := 175.0
	if bd.GetAskDepth() != expectedAsk {
		t.Errorf("Expected ask depth %.2f, got %.2f", expectedAsk, bd.GetAskDepth())
	}

	// Check total depth (150+175 = 325)
	expectedTotal := 325.0
	if bd.GetTotalDepth() != expectedTotal {
		t.Errorf("Expected total depth %.2f, got %.2f", expectedTotal, bd.GetTotalDepth())
	}
}

func TestBookDepth_BidOnly(t *testing.T) {
	bd := NewBookDepth("test_bid", 3, "bid", 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{10, 20, 30},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{15, 25, 35},
	}

	bd.Update(md)

	// Should only calculate bid side
	expected := 60.0 // 10+20+30
	if bd.GetBidDepth() != expected {
		t.Errorf("Expected bid depth %.2f, got %.2f", expected, bd.GetBidDepth())
	}

	// Value should be bid depth for "bid" mode
	if bd.GetValue() != expected {
		t.Errorf("Expected value %.2f, got %.2f", expected, bd.GetValue())
	}
}

func TestBookDepth_AskOnly(t *testing.T) {
	bd := NewBookDepth("test_ask", 3, "ask", 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{10, 20, 30},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{15, 25, 35},
	}

	bd.Update(md)

	// Should only calculate ask side
	expected := 75.0 // 15+25+35
	if bd.GetAskDepth() != expected {
		t.Errorf("Expected ask depth %.2f, got %.2f", expected, bd.GetAskDepth())
	}

	// Value should be ask depth for "ask" mode
	if bd.GetValue() != expected {
		t.Errorf("Expected value %.2f, got %.2f", expected, bd.GetValue())
	}
}

func TestBookDepth_PartialLevels(t *testing.T) {
	bd := NewBookDepth("test_partial", 5, "both", 100)

	// Only 3 levels available
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{10, 20, 30},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{15, 25, 35},
	}

	bd.Update(md)

	// Should only sum 3 levels
	expectedBid := 60.0
	expectedAsk := 75.0
	expectedTotal := 135.0

	if bd.GetBidDepth() != expectedBid {
		t.Errorf("Expected bid depth %.2f, got %.2f", expectedBid, bd.GetBidDepth())
	}
	if bd.GetAskDepth() != expectedAsk {
		t.Errorf("Expected ask depth %.2f, got %.2f", expectedAsk, bd.GetAskDepth())
	}
	if bd.GetTotalDepth() != expectedTotal {
		t.Errorf("Expected total depth %.2f, got %.2f", expectedTotal, bd.GetTotalDepth())
	}
}

func TestBookDepth_EmptyMarketData(t *testing.T) {
	bd := NewBookDepth("test_empty", 5, "both", 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{},
		BidQty:   []uint32{},
		AskPrice: []float64{},
		AskQty:   []uint32{},
	}

	bd.Update(md)

	// Should remain at 0
	if bd.GetBidDepth() != 0 {
		t.Errorf("Expected bid depth 0, got %.2f", bd.GetBidDepth())
	}
	if bd.GetAskDepth() != 0 {
		t.Errorf("Expected ask depth 0, got %.2f", bd.GetAskDepth())
	}
}

func TestBookDepth_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":        "config_depth",
		"levels":      3.0,
		"side":        "bid",
		"max_history": 500.0,
	}

	indicator, err := NewBookDepthFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	bd, ok := indicator.(*BookDepth)
	if !ok {
		t.Fatal("Expected *BookDepth type")
	}

	if bd.GetName() != "config_depth" {
		t.Errorf("Expected name 'config_depth', got '%s'", bd.GetName())
	}
}

func TestBookDepth_History(t *testing.T) {
	bd := NewBookDepth("test_history", 5, "both", 10)

	// Add multiple updates
	for i := 0; i < 15; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0},
			BidQty:   []uint32{uint32(10 + i)},
			AskPrice: []float64{100.1},
			AskQty:   []uint32{uint32(15 + i)},
		}
		bd.Update(md)
	}

	// Should only keep last 10 values
	history := bd.GetValues()
	if len(history) != 10 {
		t.Errorf("Expected history length 10, got %d", len(history))
	}

	// Last value should be (10+14) + (15+14) = 24 + 29 = 53
	lastValue := history[len(history)-1]
	expected := 53.0
	if lastValue != expected {
		t.Errorf("Expected last value %.2f, got %.2f", expected, lastValue)
	}
}

func BenchmarkBookDepth_Update(b *testing.B) {
	bd := NewBookDepth("bench", 10, "both", 1000)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6, 99.5, 99.4, 99.3, 99.2, 99.1},
		BidQty:   []uint32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5, 100.6, 100.7, 100.8, 100.9, 101.0},
		AskQty:   []uint32{15, 25, 35, 45, 55, 65, 75, 85, 95, 105},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bd.Update(md)
	}
}
