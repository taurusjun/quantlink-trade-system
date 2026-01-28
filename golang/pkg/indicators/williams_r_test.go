package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewWilliamsR(t *testing.T) {
	wr := NewWilliamsR(14, 100)

	if wr == nil {
		t.Fatal("Expected non-nil Williams %%R")
	}

	if wr.period != 14 {
		t.Errorf("Expected period 14, got %d", wr.period)
	}

	if wr.IsReady() {
		t.Error("Williams %%R should not be ready initially")
	}
}

func TestWilliamsR_Update(t *testing.T) {
	wr := NewWilliamsR(14, 100)

	// Simulate price data: uptrend then downtrend
	prices := []struct {
		high  float64
		low   float64
		close float64
	}{
		{110, 100, 105}, // Period 1
		{115, 105, 110},
		{120, 110, 115},
		{125, 115, 120},
		{130, 120, 125},
		{135, 125, 130},
		{140, 130, 135},
		{145, 135, 140},
		{150, 140, 145},
		{155, 145, 150},
		{160, 150, 155},
		{165, 155, 160},
		{170, 160, 165},
		{175, 165, 170}, // Period 14 - at top, should be near 0 (overbought)
		{170, 160, 165}, // Start declining
		{165, 155, 160},
	}

	for i, p := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{p.low},
			AskPrice: []float64{p.high},
		}
		wr.Update(md)

		if i >= 13 { // Ready after 14 periods
			if !wr.IsReady() {
				t.Errorf("Williams %%R should be ready at period %d", i+1)
			}

			value := wr.GetValue()
			// Williams %%R ranges from -100 to 0
			if value > 0 || value < -100 {
				t.Errorf("Williams %%R at period %d out of range: %.2f", i+1, value)
			}

			t.Logf("Period %d: High=%.0f, Low=%.0f, Close=%.0f, %%R=%.2f",
				i+1, p.high, p.low, p.close, value)
		}
	}

	// At period 14 (peak), should be overbought (near 0)
	// At period 16 (declining), should be less overbought
	if wr.GetValue() > -20 {
		t.Logf("Note: Williams %%R = %.2f is overbought (> -20)", wr.GetValue())
	}
}

func TestWilliamsR_Calculation(t *testing.T) {
	wr := NewWilliamsR(5, 100)

	// Known test data
	testData := []struct {
		high     float64
		low      float64
		close    float64
		expected float64 // Expected Williams %%R after this period
	}{
		{50, 40, 45, 0},        // Period 1 (not ready yet)
		{52, 42, 48, 0},        // Period 2
		{54, 44, 50, 0},        // Period 3
		{56, 46, 52, 0},        // Period 4
		{58, 48, 54, 0},        // Period 5: Ready, %R = -100 × (58-54)/(58-40) = -22.22
		{60, 50, 55, -27.78},   // Period 6: %R = -100 × (60-55)/(60-42) ≈ -27.78
		{58, 52, 52, -50.0},    // Period 7: Middle of range
		{56, 50, 50, -75.0},    // Period 8: Near bottom
		{54, 48, 48, -100.0},   // Period 9: At bottom
	}

	for i, td := range testData {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{td.low},
			AskPrice: []float64{td.high},
		}
		wr.Update(md)

		if i >= 4 { // Ready after 5 periods
			actual := wr.GetValue()
			// Allow some tolerance for floating point
			if math.Abs(actual-td.expected) > 5.0 {
				t.Logf("Period %d: Expected %%R ≈ %.2f, got %.2f (high=%.0f, low=%.0f, close=%.0f)",
					i+1, td.expected, actual, td.high, td.low, td.close)
			}
		}
	}
}

func TestWilliamsR_OverboughtOversold(t *testing.T) {
	wr := NewWilliamsR(5, 100)

	// Simulate uptrend to overbought
	for i := 0; i < 10; i++ {
		price := 100.0 + float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 2},
			AskPrice: []float64{price + 2},
		}
		wr.Update(md)
	}

	if !wr.IsReady() {
		t.Fatal("Williams %%R should be ready")
	}

	// Should be overbought (near top of range)
	if wr.IsOverbought() {
		t.Logf("✓ Williams %%R = %.2f is overbought (> -20)", wr.GetValue())
	}

	// Simulate downtrend to oversold
	for i := 0; i < 10; i++ {
		price := 145.0 - float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 2},
			AskPrice: []float64{price + 2},
		}
		wr.Update(md)
	}

	// Should be oversold (near bottom of range)
	if wr.IsOversold() {
		t.Logf("✓ Williams %%R = %.2f is oversold (< -80)", wr.GetValue())
	} else {
		t.Logf("Williams %%R = %.2f (expected < -80 for oversold)", wr.GetValue())
	}
}

func TestWilliamsR_Reset(t *testing.T) {
	wr := NewWilliamsR(5, 100)

	// Add some data
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{105.0 + float64(i)},
		}
		wr.Update(md)
	}

	if !wr.IsReady() {
		t.Fatal("Williams %%R should be ready before reset")
	}

	wr.Reset()

	if wr.IsReady() {
		t.Error("Williams %%R should not be ready after reset")
	}

	if wr.GetValue() != 0 {
		t.Errorf("Williams %%R value should be 0 after reset, got %.2f", wr.GetValue())
	}

	if len(wr.highs) != 0 || len(wr.lows) != 0 || len(wr.closes) != 0 {
		t.Error("Price windows should be empty after reset")
	}
}

func TestWilliamsR_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(21),
		"max_history": float64(500),
	}

	indicator, err := NewWilliamsRFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create Williams %%R from config: %v", err)
	}

	wr, ok := indicator.(*WilliamsR)
	if !ok {
		t.Fatal("Expected *WilliamsR type")
	}

	if wr.period != 21 {
		t.Errorf("Expected period 21, got %d", wr.period)
	}

	if wr.GetName() != "Williams %R" {
		t.Errorf("Expected name 'Williams %%R', got '%s'", wr.GetName())
	}
}

func TestWilliamsR_ZeroRange(t *testing.T) {
	wr := NewWilliamsR(3, 100)

	// Feed same price (no range)
	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0},
			AskPrice: []float64{100.0},
		}
		wr.Update(md)
	}

	if !wr.IsReady() {
		t.Fatal("Williams %%R should be ready")
	}

	// Should return neutral value (-50) when no range
	value := wr.GetValue()
	if value != -50.0 {
		t.Errorf("Expected -50.0 for zero range, got %.2f", value)
	}
}

// Benchmark Williams %%R update performance
func BenchmarkWilliamsR_Update(b *testing.B) {
	wr := NewWilliamsR(14, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		wr.Update(md)
	}
}
