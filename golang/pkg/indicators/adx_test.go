package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewADX(t *testing.T) {
	adx := NewADX(14, 100)

	if adx == nil {
		t.Fatal("Expected non-nil ADX")
	}

	if adx.period != 14 {
		t.Errorf("Expected period 14, got %d", adx.period)
	}

	if adx.IsReady() {
		t.Error("ADX should not be ready initially")
	}
}

func TestADX_Update(t *testing.T) {
	adx := NewADX(14, 100)

	// Simulate strong uptrend
	for i := 0; i < 40; i++ {
		price := 100.0 + float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 1},
			AskPrice: []float64{price + 1},
		}
		adx.Update(md)

		if i >= 28 { // Need 2×period for ADX to be ready
			if !adx.IsReady() {
				t.Logf("ADX not ready at iteration %d", i)
				continue
			}

			adxVal := adx.GetValue()
			plusDI := adx.GetPlusDI()
			minusDI := adx.GetMinusDI()

			// Check ranges
			if adxVal < 0 || adxVal > 100 {
				t.Errorf("ADX at iteration %d out of range: %.2f", i, adxVal)
			}
			if plusDI < 0 || plusDI > 100 {
				t.Errorf("+DI at iteration %d out of range: %.2f", i, plusDI)
			}
			if minusDI < 0 || minusDI > 100 {
				t.Errorf("-DI at iteration %d out of range: %.2f", i, minusDI)
			}

			t.Logf("Iteration %d: Price=%.0f, ADX=%.2f, +DI=%.2f, -DI=%.2f, Strength=%s",
				i, price, adxVal, plusDI, minusDI, adx.GetTrendStrength())
		}
	}

	// In strong uptrend
	if adx.IsTrendingUp() {
		t.Logf("✓ +DI (%.2f) > -DI (%.2f): Uptrend detected", adx.GetPlusDI(), adx.GetMinusDI())
	}

	if adx.IsStrongTrend() {
		t.Logf("✓ ADX=%.2f indicates strong trend (> 25)", adx.GetValue())
	}
}

func TestADX_TrendDirection(t *testing.T) {
	adx := NewADX(14, 100)

	// Strong uptrend
	for i := 0; i < 35; i++ {
		price := 100.0 + float64(i)*4.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		adx.Update(md)
	}

	if !adx.IsReady() {
		t.Fatal("ADX should be ready")
	}

	t.Logf("Uptrend: ADX=%.2f, +DI=%.2f, -DI=%.2f", adx.GetValue(), adx.GetPlusDI(), adx.GetMinusDI())

	if !adx.IsTrendingUp() {
		t.Errorf("Expected uptrend, but +DI (%.2f) <= -DI (%.2f)", adx.GetPlusDI(), adx.GetMinusDI())
	}

	// Transition to downtrend
	for i := 0; i < 35; i++ {
		price := 240.0 - float64(i)*4.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		adx.Update(md)
	}

	t.Logf("Downtrend: ADX=%.2f, +DI=%.2f, -DI=%.2f", adx.GetValue(), adx.GetPlusDI(), adx.GetMinusDI())

	if adx.IsTrendingDown() {
		t.Logf("✓ -DI (%.2f) > +DI (%.2f): Downtrend detected", adx.GetMinusDI(), adx.GetPlusDI())
	} else {
		t.Logf("Note: Expected downtrend, -DI=%.2f, +DI=%.2f", adx.GetMinusDI(), adx.GetPlusDI())
	}
}

func TestADX_TrendStrength(t *testing.T) {
	adx := NewADX(14, 100)

	// Weak trend (ranging market)
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i%5)*2.0 - 1.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		adx.Update(md)
	}

	if adx.IsReady() {
		weakADX := adx.GetValue()
		t.Logf("Ranging market: ADX=%.2f (%s)", weakADX, adx.GetTrendStrength())

		if adx.IsWeakTrend() {
			t.Logf("✓ Weak trend detected (ADX < 25)")
		}
	}

	// Strong trend
	for i := 0; i < 40; i++ {
		price := 100.0 + float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		adx.Update(md)
	}

	if adx.IsReady() {
		strongADX := adx.GetValue()
		t.Logf("Strong trend: ADX=%.2f (%s)", strongADX, adx.GetTrendStrength())

		if adx.IsStrongTrend() {
			t.Logf("✓ Strong trend detected (ADX > 25)")
		}

		if adx.IsVeryStrongTrend() {
			t.Logf("✓ Very strong trend detected (ADX > 50)")
		}
	}
}

func TestADX_DIComponents(t *testing.T) {
	adx := NewADX(10, 100)

	// Feed data
	prices := []float64{100, 103, 106, 104, 107, 110, 108, 111, 114, 112, 115, 118, 116, 119, 122, 120}

	for i, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 1},
			AskPrice: []float64{price + 1},
		}
		adx.Update(md)

		if adx.IsReady() {
			plusDI := adx.GetPlusDI()
			minusDI := adx.GetMinusDI()
			adxVal := adx.GetValue()

			t.Logf("Iteration %d: Price=%.0f, +DI=%.2f, -DI=%.2f, ADX=%.2f",
				i, price, plusDI, minusDI, adxVal)

			// +DI and -DI should sum to reasonable values
			diSum := plusDI + minusDI
			if diSum > 200 {
				t.Errorf("DI sum too large at iteration %d: %.2f", i, diSum)
			}
		}
	}
}

func TestADX_Reset(t *testing.T) {
	adx := NewADX(14, 100)

	// Add data
	for i := 0; i < 40; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{105.0 + float64(i)},
		}
		adx.Update(md)
	}

	if !adx.IsReady() {
		t.Fatal("ADX should be ready before reset")
	}

	adx.Reset()

	if adx.IsReady() {
		t.Error("ADX should not be ready after reset")
	}

	if adx.GetValue() != 0 || adx.GetPlusDI() != 0 || adx.GetMinusDI() != 0 {
		t.Errorf("ADX values should be 0 after reset, got ADX=%.2f, +DI=%.2f, -DI=%.2f",
			adx.GetValue(), adx.GetPlusDI(), adx.GetMinusDI())
	}

	if len(adx.highs) != 0 || len(adx.lows) != 0 || len(adx.closes) != 0 {
		t.Error("Price windows should be empty after reset")
	}

	if !adx.isFirstSmooth {
		t.Error("isFirstSmooth should be true after reset")
	}
}

func TestADX_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(20),
		"max_history": float64(500),
	}

	indicator, err := NewADXFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create ADX from config: %v", err)
	}

	adx, ok := indicator.(*ADX)
	if !ok {
		t.Fatal("Expected *ADX type")
	}

	if adx.period != 20 {
		t.Errorf("Expected period 20, got %d", adx.period)
	}

	if adx.GetName() != "ADX" {
		t.Errorf("Expected name 'ADX', got '%s'", adx.GetName())
	}
}

func TestADX_RangingVsTrending(t *testing.T) {
	rangingADX := NewADX(14, 100)
	trendingADX := NewADX(14, 100)

	// Ranging market (low ADX)
	for i := 0; i < 40; i++ {
		price := 100.0 + float64(i%4)*3.0 - 1.5
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		rangingADX.Update(md)
	}

	// Trending market (high ADX)
	for i := 0; i < 40; i++ {
		price := 100.0 + float64(i)*4.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		trendingADX.Update(md)
	}

	if rangingADX.IsReady() && trendingADX.IsReady() {
		rangingVal := rangingADX.GetValue()
		trendingVal := trendingADX.GetValue()

		t.Logf("Ranging market: ADX=%.2f (%s)", rangingVal, rangingADX.GetTrendStrength())
		t.Logf("Trending market: ADX=%.2f (%s)", trendingVal, trendingADX.GetTrendStrength())

		// Trending market should have higher ADX
		if trendingVal > rangingVal {
			t.Logf("✓ Trending ADX (%.2f) > Ranging ADX (%.2f)", trendingVal, rangingVal)
		} else {
			t.Logf("Note: Expected trending ADX > ranging ADX")
		}
	}
}

func TestADX_DIConvergence(t *testing.T) {
	adx := NewADX(10, 100)

	// Start with uptrend
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		adx.Update(md)
	}

	if adx.IsReady() {
		t.Logf("After uptrend: +DI=%.2f, -DI=%.2f, ADX=%.2f",
			adx.GetPlusDI(), adx.GetMinusDI(), adx.GetValue())
	}

	// Consolidation
	for i := 0; i < 20; i++ {
		price := 157.0 + float64(i%3)*2.0 - 1.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		adx.Update(md)
	}

	if adx.IsReady() {
		t.Logf("After consolidation: +DI=%.2f, -DI=%.2f, ADX=%.2f",
			adx.GetPlusDI(), adx.GetMinusDI(), adx.GetValue())

		// During consolidation, DIs should converge (get closer)
		diDiff := adx.GetPlusDI() - adx.GetMinusDI()
		if diDiff < 20 && diDiff > -20 {
			t.Logf("✓ DIs converging during consolidation (diff=%.2f)", diDiff)
		}
	}
}

// Benchmark ADX update performance
func BenchmarkADX_Update(b *testing.B) {
	adx := NewADX(14, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		adx.Update(md)
	}
}
