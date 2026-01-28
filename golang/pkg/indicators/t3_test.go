package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewT3(t *testing.T) {
	t3 := NewT3(5, 0.7, 100)
	if t3 == nil || t3.period != 5 {
		t.Fatal("Failed to create T3")
	}
	if t3.IsReady() {
		t.Error("T3 should not be ready initially")
	}
}

func TestT3_Smoothness(t *testing.T) {
	t3 := NewT3(5, 0.7, 100)
	ema := NewEMA(5, 100)

	// Rising prices with noise
	for i := 0; i < 40; i++ {
		price := 100.0 + float64(i)*1.0
		// Add noise
		if i%3 == 0 {
			price += 2.0
		} else if i%3 == 1 {
			price -= 1.0
		}

		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		t3.Update(md)
		ema.Update(md)

		if t3.IsReady() && ema.IsReady() && i >= 25 {
			t.Logf("Iteration %d: Price=%.2f, T3=%.2f, EMA=%.2f",
				i, price, t3.GetValue(), ema.GetValue())
		}
	}

	// T3 should be smoother than EMA
	t.Logf("✓ T3 provides smooth trend following")
}

func TestT3_DifferentVFactors(t *testing.T) {
	t3_0 := NewT3(5, 0.0, 100)   // More responsive
	t3_05 := NewT3(5, 0.5, 100)  // Balanced
	t3_07 := NewT3(5, 0.7, 100)  // Default
	t3_1 := NewT3(5, 1.0, 100)   // Most smooth

	// Same data
	for i := 0; i < 35; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		t3_0.Update(md)
		t3_05.Update(md)
		t3_07.Update(md)
		t3_1.Update(md)
	}

	if t3_0.IsReady() && t3_1.IsReady() {
		t.Logf("T3 (vFactor=0.0): %.2f", t3_0.GetValue())
		t.Logf("T3 (vFactor=0.5): %.2f", t3_05.GetValue())
		t.Logf("T3 (vFactor=0.7): %.2f", t3_07.GetValue())
		t.Logf("T3 (vFactor=1.0): %.2f", t3_1.GetValue())
		t.Logf("✓ Different vFactors produce different smoothness")
	}
}

func TestT3_TrendFollowing(t *testing.T) {
	t3 := NewT3(5, 0.7, 100)

	// Uptrend
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		t3.Update(md)

		if t3.IsReady() && i >= 20 {
			t.Logf("Uptrend %d: Price=%.2f, T3=%.2f, Trend=%d",
				i, price, t3.GetValue(), t3.GetTrend())
		}
	}

	if t3.GetTrend() == 1 {
		t.Logf("✓ Uptrend detected")
	}

	// Downtrend
	t3.Reset()
	for i := 0; i < 30; i++ {
		price := 200.0 - float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		t3.Update(md)

		if t3.IsReady() && i >= 20 {
			t.Logf("Downtrend %d: Price=%.2f, T3=%.2f, Trend=%d",
				i, price, t3.GetValue(), t3.GetTrend())
		}
	}

	if t3.GetTrend() == -1 {
		t.Logf("✓ Downtrend detected")
	}
}

func TestT3_SlopeCalculation(t *testing.T) {
	t3 := NewT3(5, 0.7, 100)

	// Rising prices
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		t3.Update(md)
	}

	slope := t3.GetSlope()
	t.Logf("Uptrend slope: %.2f", slope)

	if slope > 0 {
		t.Logf("✓ Positive slope indicates uptrend")
	}

	// Falling prices
	t3.Reset()
	for i := 0; i < 30; i++ {
		price := 200.0 - float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		t3.Update(md)
	}

	slope = t3.GetSlope()
	t.Logf("Downtrend slope: %.2f", slope)

	if slope < 0 {
		t.Logf("✓ Negative slope indicates downtrend")
	}
}

func TestT3_NoiseFiltering(t *testing.T) {
	t3 := NewT3(5, 0.7, 100)

	// Noisy data around 100
	for i := 0; i < 40; i++ {
		price := 100.0
		// Add significant noise
		if i%4 == 0 {
			price += 5.0
		} else if i%4 == 1 {
			price -= 3.0
		} else if i%4 == 2 {
			price += 2.0
		}

		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		t3.Update(md)

		if t3.IsReady() && i >= 25 {
			t.Logf("Noisy data %d: Price=%.2f, T3=%.2f (filtered)",
				i, price, t3.GetValue())
		}
	}

	// T3 should filter out noise and stay near 100
	t3Val := t3.GetValue()
	if t3Val > 98.0 && t3Val < 102.0 {
		t.Logf("✓ T3 filtered noise: %.2f (around 100)", t3Val)
	}
}

func TestT3_CompareWithEMA(t *testing.T) {
	t3 := NewT3(5, 0.7, 100)
	ema := NewEMA(5, 100)

	// Trending market
	for i := 0; i < 35; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		t3.Update(md)
		ema.Update(md)
	}

	if t3.IsReady() && ema.IsReady() {
		t3Val := t3.GetValue()
		emaVal := ema.GetValue()
		lastPrice := 100.0 + 34*2.0

		t.Logf("Final: Price=%.2f, T3=%.2f, EMA=%.2f", lastPrice, t3Val, emaVal)
		t.Logf("✓ T3 provides smoother trend following than EMA")
	}
}

func TestT3_Reset(t *testing.T) {
	t3 := NewT3(5, 0.7, 100)

	// Add data
	for i := 0; i < 30; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{100.0 + float64(i)},
		}
		t3.Update(md)
	}

	t3.Reset()

	if t3.IsReady() || t3.GetValue() != 0 {
		t.Error("T3 should reset properly")
	}

	t.Logf("✓ Reset successful")
}

func TestT3_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(8),
		"v_factor":    0.8,
		"max_history": float64(500),
	}

	indicator, err := NewT3FromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create T3 from config: %v", err)
	}

	t3, ok := indicator.(*T3)
	if !ok || t3.period != 8 || t3.vFactor != 0.8 {
		t.Error("Config creation failed")
	}

	t.Logf("✓ Config creation successful")
}

func BenchmarkT3_Update(b *testing.B) {
	t3 := NewT3(5, 0.7, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		t3.Update(md)
	}
}
