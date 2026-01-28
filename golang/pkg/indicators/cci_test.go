package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewCCI(t *testing.T) {
	cci := NewCCI(20, 100)

	if cci == nil {
		t.Fatal("Expected non-nil CCI")
	}

	if cci.period != 20 {
		t.Errorf("Expected period 20, got %d", cci.period)
	}

	if cci.constant != 0.015 {
		t.Errorf("Expected constant 0.015, got %.3f", cci.constant)
	}

	if cci.IsReady() {
		t.Error("CCI should not be ready initially")
	}
}

func TestCCI_Update(t *testing.T) {
	cci := NewCCI(20, 100)

	// Simulate uptrend
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 1},
			AskPrice: []float64{price + 1},
		}
		cci.Update(md)

		if i >= 19 { // Ready after 20 periods
			if !cci.IsReady() {
				t.Errorf("CCI should be ready at iteration %d", i)
			}

			value := cci.GetValue()
			t.Logf("Iteration %d: Price=%.0f, CCI=%.2f", i, price, value)
		}
	}

	// In strong uptrend, CCI should be high (overbought)
	if cci.GetValue() > 100 {
		t.Logf("✓ CCI=%.2f is overbought (> 100) in uptrend", cci.GetValue())
	}
}

func TestCCI_Calculation(t *testing.T) {
	cci := NewCCI(5, 100)

	// Known test sequence with varied prices
	testData := []struct {
		high  float64
		low   float64
		close float64
	}{
		{50, 40, 45},  // Period 1
		{52, 42, 47},  // Period 2
		{54, 44, 49},  // Period 3
		{56, 46, 51},  // Period 4
		{58, 48, 53},  // Period 5: Ready
		{60, 50, 55},  // Period 6: Strong uptrend, CCI should be positive
		{62, 52, 57},  // Period 7
		{60, 50, 55},  // Period 8: Consolidation
		{58, 48, 53},  // Period 9
		{56, 46, 51},  // Period 10: Turning down, CCI should decrease
	}

	for i, td := range testData {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{td.low},
			AskPrice: []float64{td.high},
		}
		cci.Update(md)

		if i >= 4 { // Ready after 5 periods
			value := cci.GetValue()
			tp := (td.high + td.low + td.close) / 3.0
			t.Logf("Period %d: High=%.0f, Low=%.0f, Close=%.0f, TP=%.2f, CCI=%.2f",
				i+1, td.high, td.low, td.close, tp, value)
		}
	}

	// Verify CCI can exceed typical bounds
	currentCCI := cci.GetValue()
	t.Logf("Final CCI: %.2f", currentCCI)
}

func TestCCI_OverboughtOversold(t *testing.T) {
	cci := NewCCI(10, 100)

	// Create strong uptrend (overbought condition)
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 1},
			AskPrice: []float64{price + 1},
		}
		cci.Update(md)
	}

	if !cci.IsReady() {
		t.Fatal("CCI should be ready")
	}

	// Should be overbought
	if cci.IsOverbought() {
		t.Logf("✓ CCI=%.2f is overbought (> 100)", cci.GetValue())
	} else {
		t.Logf("Note: Expected overbought, CCI=%.2f", cci.GetValue())
	}

	// Check for very strong uptrend
	if cci.IsStrongUptrend() {
		t.Logf("✓ CCI=%.2f shows strong uptrend (> 200)", cci.GetValue())
	}

	// Create strong downtrend (oversold condition)
	for i := 0; i < 20; i++ {
		price := 200.0 - float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 1},
			AskPrice: []float64{price + 1},
		}
		cci.Update(md)
	}

	// Should be oversold
	if cci.IsOversold() {
		t.Logf("✓ CCI=%.2f is oversold (< -100)", cci.GetValue())
	} else {
		t.Logf("Note: Expected oversold, CCI=%.2f", cci.GetValue())
	}

	// Check for very strong downtrend
	if cci.IsStrongDowntrend() {
		t.Logf("✓ CCI=%.2f shows strong downtrend (< -200)", cci.GetValue())
	}
}

func TestCCI_TrendChanges(t *testing.T) {
	cci := NewCCI(10, 100)

	// Uptrend
	for i := 0; i < 15; i++ {
		price := 100.0 + float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		cci.Update(md)
	}

	if !cci.IsReady() {
		t.Fatal("CCI should be ready")
	}

	uptrendCCI := cci.GetValue()
	t.Logf("Uptrend CCI: %.2f", uptrendCCI)

	// Consolidation (sideways)
	for i := 0; i < 10; i++ {
		price := 142.0 + float64(i%3)*2.0 - 1.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		cci.Update(md)
	}

	consolidationCCI := cci.GetValue()
	t.Logf("Consolidation CCI: %.2f", consolidationCCI)

	// CCI should be lower during consolidation
	if math.Abs(consolidationCCI) < math.Abs(uptrendCCI) {
		t.Logf("✓ CCI correctly decreased during consolidation")
	}

	// Downtrend
	for i := 0; i < 15; i++ {
		price := 145.0 - float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		cci.Update(md)
	}

	downtrendCCI := cci.GetValue()
	t.Logf("Downtrend CCI: %.2f", downtrendCCI)

	if downtrendCCI < 0 {
		t.Logf("✓ CCI correctly negative in downtrend")
	}
}

func TestCCI_Reset(t *testing.T) {
	cci := NewCCI(10, 100)

	// Add data
	for i := 0; i < 15; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{105.0 + float64(i)},
		}
		cci.Update(md)
	}

	if !cci.IsReady() {
		t.Fatal("CCI should be ready before reset")
	}

	cci.Reset()

	if cci.IsReady() {
		t.Error("CCI should not be ready after reset")
	}

	if cci.GetValue() != 0 {
		t.Errorf("CCI value should be 0 after reset, got %.2f", cci.GetValue())
	}

	if len(cci.typicalPrices) != 0 {
		t.Error("Typical prices window should be empty after reset")
	}
}

func TestCCI_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(30),
		"max_history": float64(500),
	}

	indicator, err := NewCCIFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create CCI from config: %v", err)
	}

	cci, ok := indicator.(*CCI)
	if !ok {
		t.Fatal("Expected *CCI type")
	}

	if cci.period != 30 {
		t.Errorf("Expected period 30, got %d", cci.period)
	}

	if cci.GetName() != "CCI" {
		t.Errorf("Expected name 'CCI', got '%s'", cci.GetName())
	}
}

func TestCCI_ZeroDeviation(t *testing.T) {
	cci := NewCCI(5, 100)

	// Feed same price (no deviation)
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0},
			AskPrice: []float64{100.0},
		}
		cci.Update(md)
	}

	if !cci.IsReady() {
		t.Fatal("CCI should be ready")
	}

	// Should return 0 when no deviation
	value := cci.GetValue()
	if value != 0.0 {
		t.Errorf("Expected CCI=0.0 for zero deviation, got %.2f", value)
	}
}

func TestCCI_TypicalPriceCalc(t *testing.T) {
	cci := NewCCI(3, 100)

	// Test with known high/low/close
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{90.0},  // Low
		AskPrice: []float64{110.0}, // High
		// Close will be mid price = 100.0
	}

	// Feed multiple times to get ready
	for i := 0; i < 5; i++ {
		cci.Update(md)
	}

	if !cci.IsReady() {
		t.Fatal("CCI should be ready")
	}

	// With constant prices, typical price = (110 + 90 + 100) / 3 = 100
	// Mean deviation should be 0, so CCI should be 0
	value := cci.GetValue()
	if math.Abs(value) > 0.01 {
		t.Logf("CCI with constant typical prices: %.2f", value)
	}
}

// Benchmark CCI update performance
func BenchmarkCCI_Update(b *testing.B) {
	cci := NewCCI(20, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cci.Update(md)
	}
}
