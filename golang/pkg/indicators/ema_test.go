package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewEMA(t *testing.T) {
	ema := NewEMA(20, 100)

	if ema == nil {
		t.Fatal("Expected non-nil EMA")
	}

	if ema.period != 20 {
		t.Errorf("Expected period 20, got %d", ema.period)
	}

	expectedAlpha := 2.0 / 21.0
	if math.Abs(ema.alpha-expectedAlpha) > 0.0001 {
		t.Errorf("Expected alpha %.6f, got %.6f", expectedAlpha, ema.alpha)
	}

	if !ema.isFirst {
		t.Error("Expected isFirst to be true initially")
	}
}

func TestEMA_Update(t *testing.T) {
	ema := NewEMA(10, 100)

	prices := []float64{100, 102, 101, 103, 105, 104, 106, 108, 107, 109, 110}

	for i, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		ema.Update(md)

		if i == 0 {
			// First value should equal the price
			if math.Abs(ema.GetValue()-price) > 0.0001 {
				t.Errorf("First EMA should be %.2f, got %.2f", price, ema.GetValue())
			}
		}

		if !ema.IsReady() {
			t.Errorf("EMA should be ready after update %d", i+1)
		}
	}

	// EMA should be less than the last price (price is trending up)
	// but greater than the first price
	finalEMA := ema.GetValue()
	if finalEMA <= prices[0] {
		t.Errorf("EMA %.2f should be > first price %.2f", finalEMA, prices[0])
	}

	if finalEMA >= prices[len(prices)-1] {
		t.Errorf("EMA %.2f should be < last price %.2f", finalEMA, prices[len(prices)-1])
	}
}

func TestEMA_Calculation(t *testing.T) {
	ema := NewEMA(5, 100) // period=5, alpha=2/6=0.333...

	prices := []float64{22, 24, 26, 24, 22}
	expectedEMAs := []float64{
		22.0,                                  // First value
		22.0 + (2.0/6.0)*(24-22),             // 22.667
		22.667 + (2.0/6.0)*(26-22.667),       // 23.778
		23.778 + (2.0/6.0)*(24-23.778),       // 23.852
		23.852 + (2.0/6.0)*(22-23.852),       // 23.235
	}

	for i, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		ema.Update(md)

		actual := ema.GetValue()
		expected := expectedEMAs[i]

		if math.Abs(actual-expected) > 0.01 {
			t.Errorf("EMA[%d] expected %.3f, got %.3f", i, expected, actual)
		}
	}
}

func TestEMA_Reset(t *testing.T) {
	ema := NewEMA(10, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100},
		AskPrice: []float64{100},
	}
	ema.Update(md)

	if !ema.IsReady() {
		t.Error("EMA should be ready after update")
	}

	ema.Reset()

	if ema.IsReady() {
		t.Error("EMA should not be ready after reset")
	}

	if ema.ema != 0 {
		t.Errorf("EMA value should be 0 after reset, got %.2f", ema.ema)
	}

	if !ema.isFirst {
		t.Error("isFirst should be true after reset")
	}
}

func TestEMA_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(30),
		"max_history": float64(500),
	}

	indicator, err := NewEMAFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create EMA from config: %v", err)
	}

	ema, ok := indicator.(*EMA)
	if !ok {
		t.Fatal("Expected *EMA type")
	}

	if ema.period != 30 {
		t.Errorf("Expected period 30, got %d", ema.period)
	}
}

func TestEMA_ZeroPrice(t *testing.T) {
	ema := NewEMA(10, 100)

	// Feed some valid prices
	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0},
			AskPrice: []float64{100.0},
		}
		ema.Update(md)
	}

	value1 := ema.GetValue()

	// Feed zero price (should be ignored)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{0},
		AskPrice: []float64{0},
	}
	ema.Update(md)

	value2 := ema.GetValue()

	// Value should not change
	if value1 != value2 {
		t.Errorf("EMA should ignore zero prices, got %.2f -> %.2f", value1, value2)
	}
}

func TestEMA_Comparison_With_SMA(t *testing.T) {
	// EMA should be more responsive than SMA
	period := 10
	ema := NewEMA(period, 100)
	sma := NewSMA(float64(period), 100)

	// Uptrend: EMA should be higher than SMA
	prices := []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112}

	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		ema.Update(md)
		sma.Update(md)
	}

	emaValue := ema.GetValue()
	smaValue := sma.GetValue()

	// In uptrend, EMA should be higher (more responsive)
	if emaValue <= smaValue {
		t.Errorf("In uptrend, EMA (%.2f) should be > SMA (%.2f)", emaValue, smaValue)
	}

	t.Logf("Uptrend: EMA=%.2f, SMA=%.2f (EMA is more responsive)", emaValue, smaValue)
}

// Benchmark EMA update performance
func BenchmarkEMA_Update(b *testing.B) {
	ema := NewEMA(20, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ema.Update(md)
	}
}
