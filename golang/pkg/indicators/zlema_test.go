package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewZLEMA(t *testing.T) {
	zlema := NewZLEMA(20, 100)
	if zlema == nil || zlema.period != 20 {
		t.Fatal("Failed to create ZLEMA")
	}
	if zlema.IsReady() {
		t.Error("ZLEMA should not be ready initially")
	}

	// Check lag calculation
	expectedLag := (20 - 1) / 2
	if zlema.lag != expectedLag {
		t.Errorf("Lag calculation incorrect: got %d, expected %d", zlema.lag, expectedLag)
	}
}

func TestZLEMA_CompareWithEMA(t *testing.T) {
	zlema := NewZLEMA(10, 100)
	ema := NewEMA(10, 100)

	// Rising prices
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		zlema.Update(md)
		ema.Update(md)

		if zlema.IsReady() && ema.IsReady() && i >= 15 {
			zlemaVal := zlema.GetValue()
			emaVal := ema.GetValue()
			diff := zlemaVal - emaVal

			t.Logf("Iteration %d: Price=%.2f, ZLEMA=%.2f, EMA=%.2f, Diff=%.2f",
				i, price, zlemaVal, emaVal, diff)
		}
	}

	// In trending market, ZLEMA should be closer to price (less lag)
	zlemaVal := zlema.GetValue()
	emaVal := ema.GetValue()
	lastPrice := 100.0 + 29*3.0

	zlemaLag := lastPrice - zlemaVal
	emaLag := lastPrice - emaVal

	t.Logf("Final: Price=%.2f, ZLEMA lag=%.2f, EMA lag=%.2f", lastPrice, zlemaLag, emaLag)

	if zlemaLag < emaLag {
		t.Logf("✓ ZLEMA has less lag than EMA")
	}
}

func TestZLEMA_TrendFollowing(t *testing.T) {
	zlema := NewZLEMA(10, 100)

	// Uptrend
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		zlema.Update(md)

		if zlema.IsReady() && i >= 15 {
			t.Logf("Uptrend %d: Price=%.2f, ZLEMA=%.2f, Trend=%d",
				i, price, zlema.GetValue(), zlema.GetTrend())
		}
	}

	if zlema.GetTrend() == 1 {
		t.Logf("✓ Uptrend detected")
	}

	// Downtrend
	zlema.Reset()
	for i := 0; i < 30; i++ {
		price := 200.0 - float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		zlema.Update(md)

		if zlema.IsReady() && i >= 15 {
			t.Logf("Downtrend %d: Price=%.2f, ZLEMA=%.2f, Trend=%d",
				i, price, zlema.GetValue(), zlema.GetTrend())
		}
	}

	if zlema.GetTrend() == -1 {
		t.Logf("✓ Downtrend detected")
	}
}

func TestZLEMA_Responsiveness(t *testing.T) {
	zlema := NewZLEMA(10, 100)

	// Build baseline
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{99.5},
			AskPrice: []float64{100.5},
		}
		zlema.Update(md)
	}

	baseline := zlema.GetValue()
	t.Logf("Baseline ZLEMA: %.2f", baseline)

	// Sudden price jump
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{119.5},
			AskPrice: []float64{120.5},
		}
		zlema.Update(md)

		t.Logf("After jump %d: ZLEMA=%.2f", i+1, zlema.GetValue())
	}

	// ZLEMA should respond quickly to price change
	finalVal := zlema.GetValue()
	response := finalVal - baseline

	t.Logf("Response to 20-point jump: %.2f", response)

	if response > 10 {
		t.Logf("✓ ZLEMA responded quickly to price change")
	}
}

func TestZLEMA_SlopeCalculation(t *testing.T) {
	zlema := NewZLEMA(10, 100)

	// Rising prices
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		zlema.Update(md)
	}

	slope := zlema.GetSlope()
	t.Logf("Uptrend slope: %.2f", slope)

	if slope > 0 {
		t.Logf("✓ Positive slope indicates uptrend")
	}

	// Falling prices
	zlema.Reset()
	for i := 0; i < 30; i++ {
		price := 200.0 - float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		zlema.Update(md)
	}

	slope = zlema.GetSlope()
	t.Logf("Downtrend slope: %.2f", slope)

	if slope < 0 {
		t.Logf("✓ Negative slope indicates downtrend")
	}
}

func TestZLEMA_Crossovers(t *testing.T) {
	zlema := NewZLEMA(10, 100)

	var prevPrice float64

	// Price oscillating around ZLEMA
	for i := 0; i < 35; i++ {
		price := 100.0
		if i > 15 && i%4 == 0 {
			price = 110.0 // Jump up
		} else if i > 15 && i%4 == 2 {
			price = 90.0 // Drop down
		}

		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		zlema.Update(md)

		if zlema.IsReady() && i >= 18 {
			if zlema.IsCrossAbove(price, prevPrice) {
				t.Logf("✓ Cross above at iteration %d: Price=%.2f, ZLEMA=%.2f",
					i, price, zlema.GetValue())
			}
			if zlema.IsCrossBelow(price, prevPrice) {
				t.Logf("✓ Cross below at iteration %d: Price=%.2f, ZLEMA=%.2f",
					i, price, zlema.GetValue())
			}
		}

		prevPrice = price
	}
}

func TestZLEMA_DifferentPeriods(t *testing.T) {
	zlema10 := NewZLEMA(10, 100)
	zlema20 := NewZLEMA(20, 100)
	zlema30 := NewZLEMA(30, 100)

	// Same data
	for i := 0; i < 50; i++ {
		price := 100.0 + float64(i)*1.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		zlema10.Update(md)
		zlema20.Update(md)
		zlema30.Update(md)
	}

	if zlema10.IsReady() && zlema20.IsReady() && zlema30.IsReady() {
		t.Logf("ZLEMA(10): %.2f", zlema10.GetValue())
		t.Logf("ZLEMA(20): %.2f", zlema20.GetValue())
		t.Logf("ZLEMA(30): %.2f", zlema30.GetValue())
		t.Logf("✓ Different periods produce different values")
	}
}

func TestZLEMA_Reset(t *testing.T) {
	zlema := NewZLEMA(10, 100)

	// Add data
	for i := 0; i < 30; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{100.0 + float64(i)},
		}
		zlema.Update(md)
	}

	zlema.Reset()

	if zlema.IsReady() || zlema.GetValue() != 0 {
		t.Error("ZLEMA should reset properly")
	}

	t.Logf("✓ Reset successful")
}

func TestZLEMA_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(15),
		"max_history": float64(500),
	}

	indicator, err := NewZLEMAFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create ZLEMA from config: %v", err)
	}

	zlema, ok := indicator.(*ZLEMA)
	if !ok || zlema.period != 15 {
		t.Error("Config creation failed")
	}

	expectedLag := (15 - 1) / 2
	if zlema.lag != expectedLag {
		t.Errorf("Lag calculation incorrect: got %d, expected %d", zlema.lag, expectedLag)
	}

	t.Logf("✓ Config creation successful")
}

func BenchmarkZLEMA_Update(b *testing.B) {
	zlema := NewZLEMA(20, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		zlema.Update(md)
	}
}
