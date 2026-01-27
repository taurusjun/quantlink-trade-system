package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestDepthToSpread_HighLiquidity(t *testing.T) {
	dts := NewDepthToSpread("test_high_liq", 5, true, 100)

	// High depth, tight spread
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:   []uint32{100, 100, 100, 100, 100}, // Total 500
		AskPrice: []float64{100.01, 100.02, 100.03, 100.04, 100.05},
		AskQty:   []uint32{100, 100, 100, 100, 100}, // Total 500
	}

	dts.Update(md)

	// Total depth = 1000
	// Spread = 0.01, mid = 100.005, relative spread = 0.01/100.005 ≈ 0.0001
	// Ratio = 1000 / 0.0001 ≈ 10,000,000
	ratio := dts.GetRatio()
	if ratio < 1000000 {
		t.Errorf("Expected high ratio for high liquidity, got %.2f", ratio)
	}
}

func TestDepthToSpread_LowLiquidity(t *testing.T) {
	dts := NewDepthToSpread("test_low_liq", 5, true, 100)

	// Low depth, wide spread
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:   []uint32{5, 5, 5, 5, 5}, // Total 25
		AskPrice: []float64{101.0, 101.1, 101.2, 101.3, 101.4},
		AskQty:   []uint32{5, 5, 5, 5, 5}, // Total 25
	}

	dts.Update(md)

	// Total depth = 50
	// Spread = 1.0, mid = 100.5, relative spread = 1.0/100.5 ≈ 0.01
	// Ratio = 50 / 0.01 = 5000
	ratio := dts.GetRatio()
	if ratio > 10000 {
		t.Errorf("Expected low ratio for low liquidity, got %.2f", ratio)
	}
}

func TestDepthToSpread_AbsoluteSpread(t *testing.T) {
	dts := NewDepthToSpread("test_absolute", 3, false, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{30, 30, 30}, // Total 90
		AskPrice: []float64{100.5, 100.6, 100.7},
		AskQty:   []uint32{30, 30, 30}, // Total 90
	}

	dts.Update(md)

	// Total depth = 180
	// Absolute spread = 0.5
	// Ratio = 180 / 0.5 = 360
	expected := 360.0
	ratio := dts.GetRatio()

	if ratio < expected-1 || ratio > expected+1 {
		t.Errorf("Expected ratio around %.0f, got %.2f", expected, ratio)
	}
}

func TestDepthToSpread_EmptyData(t *testing.T) {
	dts := NewDepthToSpread("test_empty", 5, true, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{},
		BidQty:   []uint32{},
		AskPrice: []float64{},
		AskQty:   []uint32{},
	}

	dts.Update(md)

	if dts.GetRatio() != 0 {
		t.Errorf("Expected ratio 0 for empty data, got %.2f", dts.GetRatio())
	}
}

func TestDepthToSpread_ZeroDepth(t *testing.T) {
	dts := NewDepthToSpread("test_zero_depth", 5, true, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9},
		BidQty:   []uint32{0, 0}, // Zero depth
		AskPrice: []float64{100.1, 100.2},
		AskQty:   []uint32{0, 0}, // Zero depth
	}

	dts.Update(md)

	if dts.GetRatio() != 0 {
		t.Errorf("Expected ratio 0 for zero depth, got %.2f", dts.GetRatio())
	}
}

func TestDepthToSpread_MinSpread(t *testing.T) {
	dts := NewDepthToSpread("test_min_spread", 3, false, 100)

	// Very tight spread (should use minimum)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{50, 50, 50}, // Total 150
		AskPrice: []float64{100.0001, 100.001, 100.01},
		AskQty:   []uint32{50, 50, 50}, // Total 150
	}

	dts.Update(md)

	// Should use minSpread (0.0001) to avoid extremely high ratios
	ratio := dts.GetRatio()
	if ratio <= 0 {
		t.Errorf("Expected positive ratio, got %.2f", ratio)
	}
}

func TestDepthToSpread_GetValue(t *testing.T) {
	dts := NewDepthToSpread("test_getvalue", 3, true, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{40, 40, 40},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{40, 40, 40},
	}

	dts.Update(md)

	// GetValue() should return ratio
	value := dts.GetValue()
	ratio := dts.GetRatio()

	if value != ratio {
		t.Errorf("GetValue() should equal GetRatio(), got %.2f vs %.2f", value, ratio)
	}
}

func TestDepthToSpread_History(t *testing.T) {
	dts := NewDepthToSpread("test_history", 3, true, 10)

	for i := 0; i < 15; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{30, 30, 30},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{30, 30, 30},
		}
		dts.Update(md)
	}

	history := dts.GetValues()
	if len(history) != 10 {
		t.Errorf("Expected history length 10, got %d", len(history))
	}
}

func TestDepthToSpread_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":        "config_dts",
		"levels":      7.0,
		"normalized":  false,
		"max_history": 300.0,
		"min_spread":  0.001,
	}

	indicator, err := NewDepthToSpreadFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	dts, ok := indicator.(*DepthToSpread)
	if !ok {
		t.Fatal("Expected *DepthToSpread type")
	}

	if dts.GetName() != "config_dts" {
		t.Errorf("Expected name 'config_dts', got '%s'", dts.GetName())
	}
}

func TestDepthToSpread_String(t *testing.T) {
	dts := NewDepthToSpread("test_string", 5, true, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		BidQty:   []uint32{100},
		AskPrice: []float64{100.1},
		AskQty:   []uint32{100},
	}
	dts.Update(md)

	str := dts.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "DepthToSpread") {
		t.Error("String() should contain 'DepthToSpread'")
	}
}

func BenchmarkDepthToSpread_Update(b *testing.B) {
	dts := NewDepthToSpread("bench", 10, true, 1000)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6, 99.5, 99.4, 99.3, 99.2, 99.1},
		BidQty:   []uint32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5, 100.6, 100.7, 100.8, 100.9, 101.0},
		AskQty:   []uint32{15, 25, 35, 45, 55, 65, 75, 85, 95, 105},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dts.Update(md)
	}
}
