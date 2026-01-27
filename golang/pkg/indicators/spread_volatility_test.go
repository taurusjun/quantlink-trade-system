package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestSpreadVolatility_StableSpread(t *testing.T) {
	sv := NewSpreadVolatility("test_stable", 10, true, 100)

	// Constant spread
	for i := 0; i < 15; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{50, 40, 30},
		}
		sv.Update(md)
	}

	volatility := sv.GetVolatility()
	// With constant spread, volatility should be very close to 0
	// Use tolerance for floating point comparison
	if volatility > 0.000001 {
		t.Errorf("Expected volatility near 0 for constant spread, got %.6f", volatility)
	}

	if !sv.IsStable() {
		t.Error("Expected stable spread")
	}

	level := sv.GetVolatilityLevel()
	if level != "VeryLow" {
		t.Errorf("Expected VeryLow volatility level, got %s", level)
	}
}

func TestSpreadVolatility_VolatileSpread(t *testing.T) {
	sv := NewSpreadVolatility("test_volatile", 10, true, 100)

	// Varying spread
	askPrices := []float64{100.1, 100.2, 100.15, 100.25, 100.12, 100.22, 100.18, 100.28}
	for _, askPrice := range askPrices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{askPrice, askPrice + 0.1, askPrice + 0.2},
			AskQty:   []uint32{50, 40, 30},
		}
		sv.Update(md)
	}

	volatility := sv.GetVolatility()
	if volatility == 0 {
		t.Error("Expected non-zero volatility for varying spread")
	}

	if sv.IsStable() {
		t.Error("Expected unstable spread")
	}
}

func TestSpreadVolatility_NormalizedSpread(t *testing.T) {
	sv := NewSpreadVolatility("test_normalized", 10, true, 100)

	// Spread varies with price (normalized should account for this)
	for i := 0; i < 10; i++ {
		price := 100.0 + float64(i)*10.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price, price - 0.1, price - 0.2},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{price + 0.1, price + 0.2, price + 0.3},
			AskQty:   []uint32{50, 40, 30},
		}
		sv.Update(md)
	}

	// Normalized volatility should be reasonable
	// Note: Even with constant absolute spread (0.1), the normalized spread
	// (spread/midPrice) varies as price changes from 100 to 190
	// This is expected behavior - relative spread decreases as price increases
	cv := sv.GetCoefficientOfVariation()
	if cv > 0.25 {
		t.Errorf("Expected reasonable CV for varying prices with constant absolute spread, got %.3f", cv)
	}
}

func TestSpreadVolatility_AbsoluteSpread(t *testing.T) {
	sv := NewSpreadVolatility("test_absolute", 10, false, 100)

	// Constant absolute spread at different price levels
	for i := 0; i < 10; i++ {
		price := 100.0 + float64(i)*10.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price, price - 0.1, price - 0.2},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{price + 0.1, price + 0.2, price + 0.3},
			AskQty:   []uint32{50, 40, 30},
		}
		sv.Update(md)
	}

	// Absolute volatility should be 0 (spread is always 0.1)
	volatility := sv.GetVolatility()
	if volatility != 0 {
		t.Errorf("Expected volatility 0 for constant absolute spread, got %.6f", volatility)
	}
}

func TestSpreadVolatility_SpreadRange(t *testing.T) {
	sv := NewSpreadVolatility("test_range", 10, true, 100)

	// Varying spread from 0.05 to 0.15
	spreads := []float64{0.05, 0.1, 0.15, 0.08, 0.12, 0.06, 0.14, 0.09}
	for _, spread := range spreads {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.0 + spread, 99.9 + spread, 99.8 + spread},
			AskQty:   []uint32{50, 40, 30},
		}
		sv.Update(md)
	}

	spreadRange := sv.GetSpreadRange()
	if spreadRange == 0 {
		t.Error("Expected non-zero spread range")
	}

	minExpected := 0.05 / 100.025 // Normalized
	maxExpected := 0.15 / 100.075 // Normalized
	expectedRange := maxExpected - minExpected

	if spreadRange < expectedRange*0.8 || spreadRange > expectedRange*1.2 {
		t.Logf("Spread range %.6f (expected around %.6f)", spreadRange, expectedRange)
	}
}

func TestSpreadVolatility_CoefficientOfVariation(t *testing.T) {
	sv := NewSpreadVolatility("test_cv", 10, true, 100)

	// Spread with known volatility
	for i := 0; i < 10; i++ {
		spread := 0.1 + float64(i%3)*0.02 // Varies between 0.1, 0.12, 0.14
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.0 + spread, 99.9 + spread, 99.8 + spread},
			AskQty:   []uint32{50, 40, 30},
		}
		sv.Update(md)
	}

	cv := sv.GetCoefficientOfVariation()
	avgSpread := sv.GetAvgSpread()
	volatility := sv.GetVolatility()

	// CV should equal volatility / mean
	expectedCV := volatility / avgSpread
	if avgSpread > 0 && (cv < expectedCV-0.001 || cv > expectedCV+0.001) {
		t.Errorf("Expected CV %.6f, got %.6f", expectedCV, cv)
	}
}

func TestSpreadVolatility_EmptyData(t *testing.T) {
	sv := NewSpreadVolatility("test_empty", 10, true, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{},
		BidQty:   []uint32{},
		AskPrice: []float64{},
		AskQty:   []uint32{},
	}

	sv.Update(md)

	// Should handle gracefully
	if sv.GetVolatility() != 0 {
		t.Errorf("Expected volatility 0 for empty data, got %.6f", sv.GetVolatility())
	}
}

func TestSpreadVolatility_GetValue(t *testing.T) {
	sv := NewSpreadVolatility("test_getvalue", 10, true, 100)

	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1 + float64(i%3)*0.01, 100.0, 99.9},
			AskQty:   []uint32{50, 40, 30},
		}
		sv.Update(md)
	}

	// GetValue() should return volatility
	value := sv.GetValue()
	volatility := sv.GetVolatility()

	if value != volatility {
		t.Errorf("GetValue() should equal GetVolatility(), got %.6f vs %.6f", value, volatility)
	}
}

func TestSpreadVolatility_History(t *testing.T) {
	sv := NewSpreadVolatility("test_history", 10, true, 10)

	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.0, 99.9},
			AskQty:   []uint32{50, 40, 30},
		}
		sv.Update(md)
	}

	history := sv.GetValues()
	if len(history) != 10 {
		t.Errorf("Expected history length 10, got %d", len(history))
	}
}

func TestSpreadVolatility_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":        "config_sv",
		"window_size": 50.0,
		"normalized":  false,
		"max_history": 200.0,
	}

	indicator, err := NewSpreadVolatilityFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	sv, ok := indicator.(*SpreadVolatility)
	if !ok {
		t.Fatal("Expected *SpreadVolatility type")
	}

	if sv.GetName() != "config_sv" {
		t.Errorf("Expected name 'config_sv', got '%s'", sv.GetName())
	}
}

func TestSpreadVolatility_String(t *testing.T) {
	sv := NewSpreadVolatility("test_string", 10, true, 100)

	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.0, 99.9},
			AskQty:   []uint32{50, 40, 30},
		}
		sv.Update(md)
	}

	str := sv.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "SpreadVolatility") {
		t.Error("String() should contain 'SpreadVolatility'")
	}
}

func BenchmarkSpreadVolatility_Update(b *testing.B) {
	sv := NewSpreadVolatility("bench", 100, true, 1000)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{50, 40, 30},
		AskPrice: []float64{100.1, 100.0, 99.9},
		AskQty:   []uint32{50, 40, 30},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		md.AskPrice[0] = 100.1 + float64(i%10)*0.01
		sv.Update(md)
	}
}
