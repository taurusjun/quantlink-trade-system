package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestMarketDepth_CumulativeDepth(t *testing.T) {
	md := NewMarketDepth("test_cumulative", 5, 100)

	marketData := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:   []uint32{10, 20, 30, 40, 50},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		AskQty:   []uint32{15, 25, 35, 45, 55},
	}

	md.Update(marketData)

	// Check cumulative bid depths
	expectedBid := []float64{10, 30, 60, 100, 150}
	for i := 0; i < 5; i++ {
		depth := md.GetBidDepth(i)
		if depth != expectedBid[i] {
			t.Errorf("Bid level %d: expected %.0f, got %.0f", i, expectedBid[i], depth)
		}
	}

	// Check cumulative ask depths
	expectedAsk := []float64{15, 40, 75, 120, 175}
	for i := 0; i < 5; i++ {
		depth := md.GetAskDepth(i)
		if depth != expectedAsk[i] {
			t.Errorf("Ask level %d: expected %.0f, got %.0f", i, expectedAsk[i], depth)
		}
	}
}

func TestMarketDepth_TotalDepth(t *testing.T) {
	md := NewMarketDepth("test_total", 5, 100)

	marketData := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{10, 20, 30}, // Total 60
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{15, 25, 35}, // Total 75
	}

	md.Update(marketData)

	expectedTotal := 135.0 // 60 + 75
	if md.GetTotalDepth() != expectedTotal {
		t.Errorf("Expected total depth %.0f, got %.0f", expectedTotal, md.GetTotalDepth())
	}
}

func TestMarketDepth_DepthImbalance(t *testing.T) {
	md := NewMarketDepth("test_imbalance", 3, 100)

	// More bid depth than ask
	marketData := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{60, 40, 20}, // Total 120
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{20, 20, 20}, // Total 60
	}

	md.Update(marketData)

	// Imbalance = (120 - 60) / (120 + 60) = 60 / 180 = 0.3333
	expected := 0.3333333333
	imbalance := md.GetDepthImbalance()

	if imbalance < expected-0.0001 || imbalance > expected+0.0001 {
		t.Errorf("Expected imbalance %.4f, got %.4f", expected, imbalance)
	}
}

func TestMarketDepth_GetDepthAtPrice(t *testing.T) {
	md := NewMarketDepth("test_depth_at_price", 5, 100)

	marketData := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:   []uint32{10, 20, 30, 40, 50}, // Cumulative: 10, 30, 60, 100, 150
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		AskQty:   []uint32{15, 25, 35, 45, 55}, // Cumulative: 15, 40, 75, 120, 175
	}

	md.Update(marketData)

	// Test buy direction (consuming asks)
	depth := md.GetDepthAtPrice(100.3, "buy")
	expected := 75.0 // Cumulative at 100.3
	if depth != expected {
		t.Errorf("Expected depth %.0f for buy at 100.3, got %.0f", expected, depth)
	}

	// Test sell direction (consuming bids)
	depth = md.GetDepthAtPrice(99.8, "sell")
	expected = 60.0 // Cumulative at 99.8
	if depth != expected {
		t.Errorf("Expected depth %.0f for sell at 99.8, got %.0f", expected, depth)
	}
}

func TestMarketDepth_EmptyData(t *testing.T) {
	md := NewMarketDepth("test_empty", 5, 100)

	marketData := &mdpb.MarketDataUpdate{
		BidPrice: []float64{},
		BidQty:   []uint32{},
		AskPrice: []float64{},
		AskQty:   []uint32{},
	}

	md.Update(marketData)

	if md.GetTotalDepth() != 0 {
		t.Errorf("Expected total depth 0, got %.0f", md.GetTotalDepth())
	}

	if md.GetBidDepth(0) != 0 {
		t.Errorf("Expected bid depth 0, got %.0f", md.GetBidDepth(0))
	}

	if md.GetAskDepth(0) != 0 {
		t.Errorf("Expected ask depth 0, got %.0f", md.GetAskDepth(0))
	}
}

func TestMarketDepth_PartialLevels(t *testing.T) {
	md := NewMarketDepth("test_partial", 10, 100)

	// Only 3 levels available
	marketData := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{10, 20, 30},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{15, 25, 35},
	}

	md.Update(marketData)

	// Level 0-2 should have values
	if md.GetBidDepth(2) != 60.0 {
		t.Errorf("Expected bid depth 60 at level 2, got %.0f", md.GetBidDepth(2))
	}

	// Level 3+ should be 0
	if md.GetBidDepth(3) != 0 {
		t.Errorf("Expected bid depth 0 at level 3, got %.0f", md.GetBidDepth(3))
	}
}

func TestMarketDepth_GetBidDepths(t *testing.T) {
	md := NewMarketDepth("test_get_bids", 3, 100)

	marketData := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{10, 20, 30},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{15, 25, 35},
	}

	md.Update(marketData)

	bidDepths := md.GetBidDepths()
	if len(bidDepths) != 3 {
		t.Errorf("Expected 3 bid depths, got %d", len(bidDepths))
	}

	expected := []float64{10, 30, 60}
	for i, depth := range bidDepths {
		if depth != expected[i] {
			t.Errorf("Bid depth %d: expected %.0f, got %.0f", i, expected[i], depth)
		}
	}
}

func TestMarketDepth_GetAskDepths(t *testing.T) {
	md := NewMarketDepth("test_get_asks", 3, 100)

	marketData := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{10, 20, 30},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{15, 25, 35},
	}

	md.Update(marketData)

	askDepths := md.GetAskDepths()
	if len(askDepths) != 3 {
		t.Errorf("Expected 3 ask depths, got %d", len(askDepths))
	}

	expected := []float64{15, 40, 75}
	for i, depth := range askDepths {
		if depth != expected[i] {
			t.Errorf("Ask depth %d: expected %.0f, got %.0f", i, expected[i], depth)
		}
	}
}

func TestMarketDepth_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":        "config_depth",
		"levels":      7.0,
		"max_history": 300.0,
	}

	indicator, err := NewMarketDepthFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	md, ok := indicator.(*MarketDepth)
	if !ok {
		t.Fatal("Expected *MarketDepth type")
	}

	if md.GetName() != "config_depth" {
		t.Errorf("Expected name 'config_depth', got '%s'", md.GetName())
	}
}

func TestMarketDepth_String(t *testing.T) {
	md := NewMarketDepth("test_string", 5, 100)

	marketData := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9},
		BidQty:   []uint32{50, 50},
		AskPrice: []float64{100.1, 100.2},
		AskQty:   []uint32{60, 60},
	}
	md.Update(marketData)

	str := md.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "MarketDepth") {
		t.Error("String() should contain 'MarketDepth'")
	}
}

func BenchmarkMarketDepth_Update(b *testing.B) {
	md := NewMarketDepth("bench", 10, 1000)

	marketData := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6, 99.5, 99.4, 99.3, 99.2, 99.1},
		BidQty:   []uint32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5, 100.6, 100.7, 100.8, 100.9, 101.0},
		AskQty:   []uint32{15, 25, 35, 45, 55, 65, 75, 85, 95, 105},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		md.Update(marketData)
	}
}
