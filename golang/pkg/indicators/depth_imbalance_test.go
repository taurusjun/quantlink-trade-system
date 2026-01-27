package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestDepthImbalance_Balanced(t *testing.T) {
	di := NewDepthImbalance("test_balanced", 5, 100)

	// Equal bid and ask depth
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:   []uint32{10, 20, 30, 40, 50},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		AskQty:   []uint32{10, 20, 30, 40, 50},
	}

	di.Update(md)

	// Imbalance should be 0 (perfectly balanced)
	imbalance := di.GetValue()
	if imbalance != 0 {
		t.Errorf("Expected imbalance 0, got %.4f", imbalance)
	}

	if !di.IsBalanced(0.01) {
		t.Error("Should be balanced with threshold 0.01")
	}
}

func TestDepthImbalance_BidDominant(t *testing.T) {
	di := NewDepthImbalance("test_bid_dominant", 5, 100)

	// More bid depth than ask
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:   []uint32{100, 90, 80, 70, 60}, // Total = 400
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		AskQty:   []uint32{20, 20, 20, 20, 20},  // Total = 100
	}

	di.Update(md)

	// Imbalance = (400 - 100) / (400 + 100) = 300 / 500 = 0.6
	expected := 0.6
	imbalance := di.GetValue()

	if imbalance != expected {
		t.Errorf("Expected imbalance %.4f, got %.4f", expected, imbalance)
	}

	if !di.IsBidDominant() {
		t.Error("Should be bid dominant")
	}

	if di.IsAskDominant() {
		t.Error("Should not be ask dominant")
	}
}

func TestDepthImbalance_AskDominant(t *testing.T) {
	di := NewDepthImbalance("test_ask_dominant", 5, 100)

	// More ask depth than bid
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:   []uint32{10, 10, 10, 10, 10}, // Total = 50
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		AskQty:   []uint32{50, 50, 50, 50, 50}, // Total = 250
	}

	di.Update(md)

	// Imbalance = (50 - 250) / (50 + 250) = -200 / 300 = -0.6667
	expected := -0.6666666666666666
	imbalance := di.GetValue()

	if imbalance < expected-0.0001 || imbalance > expected+0.0001 {
		t.Errorf("Expected imbalance around %.4f, got %.4f", expected, imbalance)
	}

	if !di.IsAskDominant() {
		t.Error("Should be ask dominant")
	}

	if di.IsBidDominant() {
		t.Error("Should not be bid dominant")
	}
}

func TestDepthImbalance_PartialLevels(t *testing.T) {
	di := NewDepthImbalance("test_partial", 10, 100)

	// Only 3 levels available
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{30, 20, 10}, // Total = 60
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{20, 20, 20}, // Total = 60
	}

	di.Update(md)

	// Should be balanced
	imbalance := di.GetValue()
	if imbalance != 0 {
		t.Errorf("Expected imbalance 0, got %.4f", imbalance)
	}
}

func TestDepthImbalance_EmptyData(t *testing.T) {
	di := NewDepthImbalance("test_empty", 5, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{},
		BidQty:   []uint32{},
		AskPrice: []float64{},
		AskQty:   []uint32{},
	}

	di.Update(md)

	// Should be 0
	if di.GetValue() != 0 {
		t.Errorf("Expected imbalance 0, got %.4f", di.GetValue())
	}
}

func TestDepthImbalance_GetterMethods(t *testing.T) {
	di := NewDepthImbalance("test_getters", 3, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{30, 20, 10}, // Total = 60
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{15, 15, 10}, // Total = 40
	}

	di.Update(md)

	if di.GetBidDepth() != 60.0 {
		t.Errorf("Expected bid depth 60, got %.0f", di.GetBidDepth())
	}

	if di.GetAskDepth() != 40.0 {
		t.Errorf("Expected ask depth 40, got %.0f", di.GetAskDepth())
	}

	// Imbalance = (60 - 40) / (60 + 40) = 20 / 100 = 0.2
	expectedPercentage := 20.0
	if di.GetImbalancePercentage() != expectedPercentage {
		t.Errorf("Expected percentage %.1f, got %.1f", expectedPercentage, di.GetImbalancePercentage())
	}
}

func TestDepthImbalance_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":        "config_imbalance",
		"levels":      3.0,
		"max_history": 500.0,
	}

	indicator, err := NewDepthImbalanceFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	di, ok := indicator.(*DepthImbalance)
	if !ok {
		t.Fatal("Expected *DepthImbalance type")
	}

	if di.GetName() != "config_imbalance" {
		t.Errorf("Expected name 'config_imbalance', got '%s'", di.GetName())
	}
}

func TestDepthImbalance_String(t *testing.T) {
	di := NewDepthImbalance("test_string", 5, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		BidQty:   []uint32{60},
		AskPrice: []float64{100.1},
		AskQty:   []uint32{40},
	}
	di.Update(md)

	str := di.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "DepthImbalance") {
		t.Error("String() should contain 'DepthImbalance'")
	}
}

func TestDepthImbalance_RangeCheck(t *testing.T) {
	di := NewDepthImbalance("test_range", 5, 100)

	// Extreme imbalance
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		BidQty:   []uint32{1000},
		AskPrice: []float64{100.1},
		AskQty:   []uint32{1},
	}
	di.Update(md)

	imbalance := di.GetValue()
	if imbalance < -1.0 || imbalance > 1.0 {
		t.Errorf("Imbalance should be in range [-1, 1], got %.4f", imbalance)
	}
}

func BenchmarkDepthImbalance_Update(b *testing.B) {
	di := NewDepthImbalance("bench", 10, 1000)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6, 99.5, 99.4, 99.3, 99.2, 99.1},
		BidQty:   []uint32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5, 100.6, 100.7, 100.8, 100.9, 101.0},
		AskQty:   []uint32{15, 25, 35, 45, 55, 65, 75, 85, 95, 105},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		di.Update(md)
	}
}
