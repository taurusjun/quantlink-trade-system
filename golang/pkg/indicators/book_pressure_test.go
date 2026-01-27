package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestBookPressure_Basic(t *testing.T) {
	bp := NewBookPressure("test_pressure", 5, 0.9, true, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:   []uint32{10, 20, 30, 40, 50},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		AskQty:   []uint32{15, 25, 35, 45, 55},
	}

	bp.Update(md)

	// With weight decay 0.9
	// Buy pressure: 10*1 + 20*0.9 + 30*0.81 + 40*0.729 + 50*0.6561 = 10 + 18 + 24.3 + 29.16 + 32.805 = 114.265
	// Sell pressure: 15*1 + 25*0.9 + 35*0.81 + 45*0.729 + 55*0.6561 = 15 + 22.5 + 28.35 + 32.805 + 36.0855 = 134.7405
	expectedBuy := 114.265
	expectedSell := 134.7405

	buyPressure := bp.GetBuyPressure()
	if buyPressure < expectedBuy-0.01 || buyPressure > expectedBuy+0.01 {
		t.Errorf("Expected buy pressure around %.2f, got %.2f", expectedBuy, buyPressure)
	}

	sellPressure := bp.GetSellPressure()
	if sellPressure < expectedSell-0.01 || sellPressure > expectedSell+0.01 {
		t.Errorf("Expected sell pressure around %.2f, got %.2f", expectedSell, sellPressure)
	}
}

func TestBookPressure_NoVolume(t *testing.T) {
	bp := NewBookPressure("test_no_volume", 5, 0.9, false, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{10, 20, 30},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{15, 25, 35},
	}

	bp.Update(md)

	// Without volume weighting, just count levels with decay
	// Buy: 1 + 0.9 + 0.81 = 2.71
	// Sell: 1 + 0.9 + 0.81 = 2.71
	expectedPressure := 2.71

	buyPressure := bp.GetBuyPressure()
	if buyPressure < expectedPressure-0.01 || buyPressure > expectedPressure+0.01 {
		t.Errorf("Expected buy pressure around %.2f, got %.2f", expectedPressure, buyPressure)
	}
}

func TestBookPressure_BuyingPressure(t *testing.T) {
	bp := NewBookPressure("test_buying", 3, 1.0, true, 100)

	// Strong buying pressure (no decay for simplicity)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{100, 90, 80}, // Total 270
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{20, 20, 20}, // Total 60
	}

	bp.Update(md)

	if !bp.IsBuyingPressure() {
		t.Error("Should indicate buying pressure")
	}

	if bp.IsSellingPressure() {
		t.Error("Should not indicate selling pressure")
	}

	netPressure := bp.GetValue()
	expectedNet := 270.0 - 60.0 // 210.0
	if netPressure != expectedNet {
		t.Errorf("Expected net pressure %.0f, got %.0f", expectedNet, netPressure)
	}
}

func TestBookPressure_SellingPressure(t *testing.T) {
	bp := NewBookPressure("test_selling", 3, 1.0, true, 100)

	// Strong selling pressure
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{10, 10, 10}, // Total 30
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{100, 90, 80}, // Total 270
	}

	bp.Update(md)

	if !bp.IsSellingPressure() {
		t.Error("Should indicate selling pressure")
	}

	if bp.IsBuyingPressure() {
		t.Error("Should not indicate buying pressure")
	}

	netPressure := bp.GetValue()
	expectedNet := 30.0 - 270.0 // -240.0
	if netPressure != expectedNet {
		t.Errorf("Expected net pressure %.0f, got %.0f", expectedNet, netPressure)
	}
}

func TestBookPressure_PressureRatio(t *testing.T) {
	bp := NewBookPressure("test_ratio", 2, 1.0, true, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9},
		BidQty:   []uint32{80, 40}, // Total 120
		AskPrice: []float64{100.1, 100.2},
		AskQty:   []uint32{30, 30}, // Total 60
	}

	bp.Update(md)

	// Ratio = 120 / 60 = 2.0
	expectedRatio := 2.0
	ratio := bp.GetPressureRatio()

	if ratio != expectedRatio {
		t.Errorf("Expected ratio %.1f, got %.1f", expectedRatio, ratio)
	}
}

func TestBookPressure_NormalizedPressure(t *testing.T) {
	bp := NewBookPressure("test_normalized", 2, 1.0, true, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9},
		BidQty:   []uint32{60, 40}, // Total 100
		AskPrice: []float64{100.1, 100.2},
		AskQty:   []uint32{30, 20}, // Total 50
	}

	bp.Update(md)

	// Normalized = (100 - 50) / (100 + 50) = 50 / 150 = 0.3333
	expected := 0.3333333333
	normalized := bp.GetNormalizedPressure()

	if normalized < expected-0.0001 || normalized > expected+0.0001 {
		t.Errorf("Expected normalized pressure %.4f, got %.4f", expected, normalized)
	}
}

func TestBookPressure_PressureStrength(t *testing.T) {
	bp := NewBookPressure("test_strength", 2, 1.0, true, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9},
		BidQty:   []uint32{80, 20}, // Total 100
		AskPrice: []float64{100.1, 100.2},
		AskQty:   []uint32{40, 10}, // Total 50
	}

	bp.Update(md)

	// Strength = |100 - 50| / (100 + 50) = 50 / 150 = 0.3333
	expected := 0.3333333333
	strength := bp.GetPressureStrength()

	if strength < expected-0.0001 || strength > expected+0.0001 {
		t.Errorf("Expected strength %.4f, got %.4f", expected, strength)
	}
}

func TestBookPressure_EmptyData(t *testing.T) {
	bp := NewBookPressure("test_empty", 5, 0.9, true, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{},
		BidQty:   []uint32{},
		AskPrice: []float64{},
		AskQty:   []uint32{},
	}

	bp.Update(md)

	if bp.GetValue() != 0 {
		t.Errorf("Expected pressure 0, got %.2f", bp.GetValue())
	}
}

func TestBookPressure_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":         "config_pressure",
		"levels":       3.0,
		"weight_decay": 0.8,
		"use_volume":   true,
		"max_history":  500.0,
	}

	indicator, err := NewBookPressureFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	bp, ok := indicator.(*BookPressure)
	if !ok {
		t.Fatal("Expected *BookPressure type")
	}

	if bp.GetName() != "config_pressure" {
		t.Errorf("Expected name 'config_pressure', got '%s'", bp.GetName())
	}
}

func TestBookPressure_String(t *testing.T) {
	bp := NewBookPressure("test_string", 5, 0.9, true, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		BidQty:   []uint32{100},
		AskPrice: []float64{100.1},
		AskQty:   []uint32{50},
	}
	bp.Update(md)

	str := bp.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "BookPressure") {
		t.Error("String() should contain 'BookPressure'")
	}
}

func BenchmarkBookPressure_Update(b *testing.B) {
	bp := NewBookPressure("bench", 10, 0.9, true, 1000)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6, 99.5, 99.4, 99.3, 99.2, 99.1},
		BidQty:   []uint32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5, 100.6, 100.7, 100.8, 100.9, 101.0},
		AskQty:   []uint32{15, 25, 35, 45, 55, 65, 75, 85, 95, 105},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bp.Update(md)
	}
}
