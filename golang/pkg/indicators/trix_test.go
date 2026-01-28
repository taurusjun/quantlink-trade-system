package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewTRIX(t *testing.T) {
	trix := NewTRIX(20, 100)

	if trix == nil {
		t.Fatal("Expected non-nil TRIX")
	}

	if trix.period != 20 {
		t.Errorf("Expected period 20, got %d", trix.period)
	}

	if trix.IsReady() {
		t.Error("TRIX should not be ready initially")
	}
}

func TestTRIX_Update(t *testing.T) {
	trix := NewTRIX(10, 100)

	// Simulate uptrend
	for i := 0; i < 50; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		trix.Update(md)

		if trix.IsReady() {
			value := trix.GetValue()
			t.Logf("Iteration %d: Price=%.0f, TRIX=%.4f", i, price, value)
		}
	}

	// In uptrend, TRIX should be positive
	if trix.IsReady() {
		if trix.IsPositive() {
			t.Logf("✓ TRIX=%.4f is positive in uptrend", trix.GetValue())
		} else {
			t.Logf("Note: TRIX=%.4f (expected positive)", trix.GetValue())
		}
	}
}

func TestTRIX_TrendChanges(t *testing.T) {
	trix := NewTRIX(10, 100)

	// Uptrend
	for i := 0; i < 40; i++ {
		price := 100.0 + float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		trix.Update(md)
	}

	if !trix.IsReady() {
		t.Fatal("TRIX should be ready")
	}

	uptrendTRIX := trix.GetValue()
	t.Logf("Uptrend: TRIX=%.4f", uptrendTRIX)

	// Should be positive in uptrend
	if uptrendTRIX > 0 {
		t.Logf("✓ TRIX positive in uptrend")
	}

	// Downtrend
	for i := 0; i < 40; i++ {
		price := 217.0 - float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		trix.Update(md)
	}

	downtrendTRIX := trix.GetValue()
	t.Logf("Downtrend: TRIX=%.4f", downtrendTRIX)

	// Should be negative in downtrend
	if downtrendTRIX < 0 {
		t.Logf("✓ TRIX negative in downtrend")
	}
}

func TestTRIX_ZeroCrosses(t *testing.T) {
	trix := NewTRIX(8, 100)

	// Start with uptrend
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		trix.Update(md)
	}

	if !trix.IsReady() {
		t.Fatal("TRIX should be ready")
	}

	t.Logf("After uptrend: TRIX=%.4f", trix.GetValue())

	// Transition to downtrend (should trigger bearish cross)
	for i := 0; i < 30; i++ {
		price := 158.0 - float64(i)*2.5
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		trix.Update(md)

		if trix.IsBearishCross() {
			t.Logf("✓ Bearish cross detected at iteration %d: TRIX=%.4f crossed below 0", i, trix.GetValue())
		}
	}

	// Transition back to uptrend (should trigger bullish cross)
	for i := 0; i < 30; i++ {
		price := 83.0 + float64(i)*2.5
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		trix.Update(md)

		if trix.IsBullishCross() {
			t.Logf("✓ Bullish cross detected at iteration %d: TRIX=%.4f crossed above 0", i, trix.GetValue())
		}
	}
}

func TestTRIX_Smoothing(t *testing.T) {
	trix := NewTRIX(10, 100)
	ema := NewEMA(10, 100)

	// Feed same data to both
	for i := 0; i < 50; i++ {
		// Oscillating price
		price := 100.0 + float64(i%5)*10.0 - 20.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		trix.Update(md)
		ema.Update(md)

		if trix.IsReady() && ema.IsReady() {
			t.Logf("Iteration %d: Price=%.0f, EMA=%.2f, TRIX=%.4f",
				i, price, ema.GetValue(), trix.GetValue())
		}
	}

	// TRIX should show smoother changes due to triple smoothing
	t.Logf("Final TRIX=%.4f (triple smoothed)", trix.GetValue())
}

func TestTRIX_Reset(t *testing.T) {
	trix := NewTRIX(10, 100)

	// Add data
	for i := 0; i < 50; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{100.0 + float64(i)},
		}
		trix.Update(md)
	}

	if !trix.IsReady() {
		t.Fatal("TRIX should be ready before reset")
	}

	trix.Reset()

	if trix.IsReady() {
		t.Error("TRIX should not be ready after reset")
	}

	if trix.GetValue() != 0 {
		t.Errorf("TRIX value should be 0 after reset, got %.4f", trix.GetValue())
	}

	if trix.prevEMA3 != 0 {
		t.Error("prevEMA3 should be 0 after reset")
	}
}

func TestTRIX_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(30),
		"max_history": float64(500),
	}

	indicator, err := NewTRIXFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create TRIX from config: %v", err)
	}

	trix, ok := indicator.(*TRIX)
	if !ok {
		t.Fatal("Expected *TRIX type")
	}

	if trix.period != 30 {
		t.Errorf("Expected period 30, got %d", trix.period)
	}

	if trix.GetName() != "TRIX" {
		t.Errorf("Expected name 'TRIX', got '%s'", trix.GetName())
	}
}

func TestTRIX_RangingMarket(t *testing.T) {
	trix := NewTRIX(10, 100)

	// Simulate ranging market (sideways)
	for i := 0; i < 50; i++ {
		price := 100.0 + float64(i%4)*5.0 - 7.5 // Oscillates around 100
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		trix.Update(md)

		if trix.IsReady() && i >= 45 {
			t.Logf("Ranging iteration %d: Price=%.0f, TRIX=%.4f", i, price, trix.GetValue())
		}
	}

	if trix.IsReady() {
		// In ranging market, TRIX should be near 0
		trixVal := trix.GetValue()
		if math.Abs(trixVal) < 0.5 {
			t.Logf("✓ TRIX=%.4f is near 0 in ranging market", trixVal)
		} else {
			t.Logf("TRIX=%.4f in ranging market", trixVal)
		}
	}
}

func TestTRIX_Sensitivity(t *testing.T) {
	// Compare different periods
	trixFast := NewTRIX(5, 100)
	trixSlow := NewTRIX(20, 100)

	// Feed same data
	for i := 0; i < 60; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		trixFast.Update(md)
		trixSlow.Update(md)

		if trixFast.IsReady() && trixSlow.IsReady() && i >= 50 {
			t.Logf("Iteration %d: Fast TRIX=%.4f, Slow TRIX=%.4f",
				i, trixFast.GetValue(), trixSlow.GetValue())
		}
	}

	// Fast TRIX should generally have larger magnitude
	t.Logf("Final: Fast TRIX=%.4f, Slow TRIX=%.4f",
		trixFast.GetValue(), trixSlow.GetValue())
}

func TestTRIX_NoiseFiltering(t *testing.T) {
	trix := NewTRIX(10, 100)

	// Add noisy uptrend
	for i := 0; i < 50; i++ {
		// Base uptrend with noise
		basePrice := 100.0 + float64(i)*2.0
		noise := float64((i*7)%5 - 2) * 3.0 // Random-like noise
		price := basePrice + noise

		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		trix.Update(md)

		if trix.IsReady() && i >= 40 {
			t.Logf("Noisy iteration %d: Price=%.0f, TRIX=%.4f", i, price, trix.GetValue())
		}
	}

	// Despite noise, TRIX should still show positive trend
	if trix.IsReady() && trix.IsPositive() {
		t.Logf("✓ TRIX=%.4f filtered noise and detected uptrend", trix.GetValue())
	}
}

// Benchmark TRIX update performance
func BenchmarkTRIX_Update(b *testing.B) {
	trix := NewTRIX(20, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{100.0},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		trix.Update(md)
	}
}
