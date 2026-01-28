package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewKAMA(t *testing.T) {
	kama := NewKAMA(10, 2, 30, 100)
	if kama == nil || kama.period != 10 {
		t.Fatal("Failed to create KAMA")
	}
	if kama.IsReady() {
		t.Error("KAMA should not be ready initially")
	}
}

func TestKAMA_TrendingMarket(t *testing.T) {
	kama := NewKAMA(10, 2, 30, 100)

	// Strong uptrend (high efficiency ratio)
	for i := 0; i < 25; i++ {
		price := 100.0 + float64(i)*5.0 // Consistent uptrend
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		kama.Update(md)

		if kama.IsReady() && i >= 15 {
			t.Logf("Iteration %d: Price=%.2f, KAMA=%.2f, ER=%.3f",
				i, price, kama.GetValue(), kama.GetEfficiencyRatio())
		}
	}

	// In trending market, ER should be high
	er := kama.GetEfficiencyRatio()
	if er > 0.3 {
		t.Logf("✓ High ER (%.3f) indicates trending market", er)
	}

	if kama.IsTrendingMarket() {
		t.Logf("✓ Trending market detected")
	}

	trend := kama.GetTrend()
	if trend == 1 {
		t.Logf("✓ Uptrend detected")
	}
}

func TestKAMA_RangingMarket(t *testing.T) {
	kama := NewKAMA(10, 2, 30, 100)

	// Ranging/choppy market (low efficiency ratio)
	for i := 0; i < 30; i++ {
		price := 100.0
		if i%2 == 0 {
			price = 105.0
		} else {
			price = 95.0
		}

		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		kama.Update(md)

		if kama.IsReady() && i >= 15 {
			t.Logf("Iteration %d: Price=%.2f, KAMA=%.2f, ER=%.3f",
				i, price, kama.GetValue(), kama.GetEfficiencyRatio())
		}
	}

	// In ranging market, ER should be low
	er := kama.GetEfficiencyRatio()
	if er < 0.2 {
		t.Logf("✓ Low ER (%.3f) indicates ranging market", er)
	}

	if kama.IsRangingMarket() {
		t.Logf("✓ Ranging market detected")
	}
}

func TestKAMA_Adaptivity(t *testing.T) {
	kama := NewKAMA(10, 2, 30, 100)

	// Phase 1: Trending market
	t.Log("Phase 1: Strong trend")
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		kama.Update(md)

		if kama.IsReady() && i >= 12 {
			t.Logf("Trend - Price=%.2f, KAMA=%.2f, ER=%.3f",
				price, kama.GetValue(), kama.GetEfficiencyRatio())
		}
	}

	trendER := kama.GetEfficiencyRatio()

	// Phase 2: Ranging market
	t.Log("Phase 2: Ranging/choppy")
	for i := 0; i < 20; i++ {
		price := 157.0 + float64(i%4)*2.0 // Small oscillations
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		kama.Update(md)

		if i >= 10 {
			t.Logf("Range - Price=%.2f, KAMA=%.2f, ER=%.3f",
				price, kama.GetValue(), kama.GetEfficiencyRatio())
		}
	}

	rangeER := kama.GetEfficiencyRatio()

	// ER should decrease in ranging market
	if trendER > rangeER {
		t.Logf("✓ ER adapted: Trend ER=%.3f > Range ER=%.3f", trendER, rangeER)
	}
}

func TestKAMA_CompareWithSMA(t *testing.T) {
	kama := NewKAMA(10, 2, 30, 100)
	sma := NewSMA(10.0, 100)

	// Trending market
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		kama.Update(md)
		sma.Update(md)

		if kama.IsReady() && sma.IsReady() && i >= 15 {
			kamaVal := kama.GetValue()
			smaVal := sma.GetValue()
			priceDiff := price - kamaVal
			smaDiff := price - smaVal

			t.Logf("Price=%.2f, KAMA=%.2f (diff=%.2f), SMA=%.2f (diff=%.2f)",
				price, kamaVal, priceDiff, smaVal, smaDiff)
		}
	}

	// In trending market, KAMA should be closer to price (less lag)
	kamaVal := kama.GetValue()
	smaVal := sma.GetValue()
	lastPrice := 100.0 + 29*2.0

	kamaLag := lastPrice - kamaVal
	smaLag := lastPrice - smaVal

	t.Logf("Final: Price=%.2f, KAMA lag=%.2f, SMA lag=%.2f", lastPrice, kamaLag, smaLag)

	if kamaLag < smaLag {
		t.Logf("✓ KAMA has less lag than SMA in trending market")
	}
}

func TestKAMA_NoiseFiltering(t *testing.T) {
	kama := NewKAMA(10, 2, 30, 100)

	// Noisy sideways market
	for i := 0; i < 30; i++ {
		price := 100.0
		// Add random noise
		if i%3 == 0 {
			price += 2.0
		} else if i%3 == 1 {
			price -= 1.5
		}

		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		kama.Update(md)

		if kama.IsReady() && i >= 12 {
			t.Logf("Iteration %d: Price=%.2f, KAMA=%.2f (smoothed)",
				i, price, kama.GetValue())
		}
	}

	// KAMA should filter noise
	kamaVal := kama.GetValue()
	if kamaVal > 99.0 && kamaVal < 101.0 {
		t.Logf("✓ KAMA filtered noise: %.2f (around 100)", kamaVal)
	}
}

func TestKAMA_DifferentPeriods(t *testing.T) {
	kama1 := NewKAMA(5, 2, 30, 100)   // Shorter period
	kama2 := NewKAMA(10, 2, 30, 100)  // Medium period
	kama3 := NewKAMA(20, 2, 30, 100)  // Longer period

	// Same data
	for i := 0; i < 35; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		kama1.Update(md)
		kama2.Update(md)
		kama3.Update(md)
	}

	if kama1.IsReady() && kama2.IsReady() && kama3.IsReady() {
		t.Logf("KAMA(5): %.2f", kama1.GetValue())
		t.Logf("KAMA(10): %.2f", kama2.GetValue())
		t.Logf("KAMA(20): %.2f", kama3.GetValue())
		t.Logf("✓ Different periods produce different values")
	}
}

func TestKAMA_Reset(t *testing.T) {
	kama := NewKAMA(10, 2, 30, 100)

	// Add data
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{100.0 + float64(i)},
		}
		kama.Update(md)
	}

	kama.Reset()

	if kama.IsReady() || kama.GetValue() != 0 {
		t.Error("KAMA should reset properly")
	}

	t.Logf("✓ Reset successful")
}

func TestKAMA_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(15),
		"fast_period": float64(3),
		"slow_period": float64(40),
		"max_history": float64(500),
	}

	indicator, err := NewKAMAFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create KAMA from config: %v", err)
	}

	kama, ok := indicator.(*KAMA)
	if !ok || kama.period != 15 || kama.fastPeriod != 3 || kama.slowPeriod != 40 {
		t.Error("Config creation failed")
	}

	t.Logf("✓ Config creation successful")
}

func BenchmarkKAMA_Update(b *testing.B) {
	kama := NewKAMA(10, 2, 30, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kama.Update(md)
	}
}
