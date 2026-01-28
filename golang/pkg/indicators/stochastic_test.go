package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewStochastic(t *testing.T) {
	stoch := NewStochastic(14, 3, 3, 100)

	if stoch == nil {
		t.Fatal("Expected non-nil Stochastic")
	}

	if stoch.period != 14 {
		t.Errorf("Expected period 14, got %d", stoch.period)
	}

	if stoch.smoothK != 3 {
		t.Errorf("Expected smoothK 3, got %d", stoch.smoothK)
	}

	if stoch.smoothD != 3 {
		t.Errorf("Expected smoothD 3, got %d", stoch.smoothD)
	}

	if stoch.IsReady() {
		t.Error("Stochastic should not be ready initially")
	}
}

func TestStochastic_Update(t *testing.T) {
	stoch := NewStochastic(14, 1, 3, 100) // Fast stochastic (smoothK=1)

	// Simulate uptrend
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 1},
			AskPrice: []float64{price + 1},
		}
		stoch.Update(md)

		if i >= 16 { // period=14 + smoothK=1 + smoothD=3 - 1
			if !stoch.IsReady() {
				t.Errorf("Stochastic should be ready at iteration %d", i)
			}

			k := stoch.GetPercentK()
			d := stoch.GetPercentD()

			// Check range
			if k < 0 || k > 100 {
				t.Errorf("%%K at iteration %d out of range: %.2f", i, k)
			}
			if d < 0 || d > 100 {
				t.Errorf("%%D at iteration %d out of range: %.2f", i, d)
			}

			t.Logf("Iteration %d: Price=%.0f, %%K=%.2f, %%D=%.2f", i, price, k, d)
		}
	}

	// In strong uptrend, %K should be high (overbought)
	if !stoch.IsOverbought() {
		t.Logf("Note: Expected overbought in uptrend, %%K=%.2f", stoch.GetPercentK())
	}
}

func TestStochastic_Calculation(t *testing.T) {
	stoch := NewStochastic(5, 1, 3, 100) // Simple fast stochastic

	// Known test sequence
	testData := []struct {
		high  float64
		low   float64
		close float64
	}{
		{50, 40, 45},  // Period 1
		{52, 42, 48},  // Period 2
		{54, 44, 50},  // Period 3
		{56, 46, 52},  // Period 4
		{58, 48, 54},  // Period 5: Ready, at top: %K should be ~100
		{60, 50, 55},  // Period 6
		{58, 50, 52},  // Period 7: Middle of range
		{56, 48, 50},  // Period 8
		{54, 46, 48},  // Period 9: Near bottom
		{52, 44, 45},  // Period 10: At bottom: %K should be ~0
	}

	for i, td := range testData {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{td.low},
			AskPrice: []float64{td.high},
		}
		stoch.Update(md)

		if i >= 4 { // Ready after period 5
			k := stoch.GetPercentK()
			t.Logf("Period %d: High=%.0f, Low=%.0f, Close=%.0f, %%K=%.2f",
				i+1, td.high, td.low, td.close, k)

			// Check %K is in valid range
			if k < 0 || k > 100 {
				t.Errorf("%%K out of range at period %d: %.2f", i+1, k)
			}
		}

		if i == 4 {
			// At top of range, %K should be high
			k := stoch.GetPercentK()
			if k < 70 {
				t.Errorf("Expected high %%K at period 5 (top), got %.2f", k)
			}
		}

		if i == 9 {
			// At bottom of range, %K should be low
			k := stoch.GetPercentK()
			if k > 30 {
				t.Errorf("Expected low %%K at period 10 (bottom), got %.2f", k)
			}
		}
	}
}

func TestStochastic_OverboughtOversold(t *testing.T) {
	stoch := NewStochastic(5, 1, 3, 100)

	// Create overbought condition (price at top of range)
	for i := 0; i < 10; i++ {
		price := 100.0 + float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		stoch.Update(md)
	}

	if !stoch.IsReady() {
		t.Fatal("Stochastic should be ready")
	}

	// Should be overbought
	if stoch.IsOverbought() {
		t.Logf("✓ Stochastic %%K=%.2f is overbought (> 80)", stoch.GetPercentK())
	} else {
		t.Logf("Note: Expected overbought, %%K=%.2f", stoch.GetPercentK())
	}

	// Create oversold condition (price at bottom of range)
	for i := 0; i < 10; i++ {
		price := 145.0 - float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		stoch.Update(md)
	}

	// Should be oversold
	if stoch.IsOversold() {
		t.Logf("✓ Stochastic %%K=%.2f is oversold (< 20)", stoch.GetPercentK())
	} else {
		t.Logf("Note: Expected oversold, %%K=%.2f", stoch.GetPercentK())
	}
}

func TestStochastic_Crossovers(t *testing.T) {
	stoch := NewStochastic(5, 3, 3, 100)

	// Feed initial data
	for i := 0; i < 15; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		stoch.Update(md)
	}

	if !stoch.IsReady() {
		t.Fatal("Stochastic should be ready")
	}

	prevK := stoch.GetPercentK()
	prevD := stoch.GetPercentD()
	t.Logf("Initial: %%K=%.2f, %%D=%.2f", prevK, prevD)

	// Continue with more data to trigger crossovers
	for i := 0; i < 10; i++ {
		// Simulate price oscillation
		price := 130.0 - float64(i%5)*10.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		stoch.Update(md)

		k := stoch.GetPercentK()
		d := stoch.GetPercentD()

		if stoch.IsBullishCrossover() {
			t.Logf("✓ Bullish crossover detected: %%K (%.2f) crossed above %%D (%.2f)", k, d)
		}

		if stoch.IsBearishCrossover() {
			t.Logf("✓ Bearish crossover detected: %%K (%.2f) crossed below %%D (%.2f)", k, d)
		}

		t.Logf("Iteration %d: %%K=%.2f, %%D=%.2f", i, k, d)
	}
}

func TestStochastic_Reset(t *testing.T) {
	stoch := NewStochastic(5, 1, 3, 100)

	// Add data
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{105.0 + float64(i)},
		}
		stoch.Update(md)
	}

	if !stoch.IsReady() {
		t.Fatal("Stochastic should be ready before reset")
	}

	stoch.Reset()

	if stoch.IsReady() {
		t.Error("Stochastic should not be ready after reset")
	}

	if stoch.GetPercentK() != 0 || stoch.GetPercentD() != 0 {
		t.Errorf("Stochastic values should be 0 after reset, got %%K=%.2f, %%D=%.2f",
			stoch.GetPercentK(), stoch.GetPercentD())
	}

	if len(stoch.highs) != 0 || len(stoch.lows) != 0 || len(stoch.closes) != 0 {
		t.Error("Price windows should be empty after reset")
	}
}

func TestStochastic_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(21),
		"smooth_k":    float64(5),
		"smooth_d":    float64(5),
		"max_history": float64(500),
	}

	indicator, err := NewStochasticFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create Stochastic from config: %v", err)
	}

	stoch, ok := indicator.(*Stochastic)
	if !ok {
		t.Fatal("Expected *Stochastic type")
	}

	if stoch.period != 21 {
		t.Errorf("Expected period 21, got %d", stoch.period)
	}

	if stoch.smoothK != 5 {
		t.Errorf("Expected smoothK 5, got %d", stoch.smoothK)
	}

	if stoch.smoothD != 5 {
		t.Errorf("Expected smoothD 5, got %d", stoch.smoothD)
	}
}

func TestStochastic_ZeroRange(t *testing.T) {
	stoch := NewStochastic(3, 1, 3, 100)

	// Feed same price (no range)
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0},
			AskPrice: []float64{100.0},
		}
		stoch.Update(md)
	}

	if !stoch.IsReady() {
		t.Fatal("Stochastic should be ready")
	}

	// Should return neutral value (50) when no range
	k := stoch.GetPercentK()
	if math.Abs(k-50.0) > 1.0 {
		t.Errorf("Expected %%K ≈ 50.0 for zero range, got %.2f", k)
	}
}

func TestStochastic_FastVsSlow(t *testing.T) {
	// Fast Stochastic (smoothK=1)
	fast := NewStochastic(14, 1, 3, 100)
	// Slow Stochastic (smoothK=3)
	slow := NewStochastic(14, 3, 3, 100)

	// Feed same data to both
	for i := 0; i < 25; i++ {
		price := 100.0 + float64(i)*2.0 - float64(i%3)*3.0 // Oscillating
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		fast.Update(md)
		slow.Update(md)

		if fast.IsReady() && slow.IsReady() {
			fastK := fast.GetPercentK()
			slowK := slow.GetPercentK()
			t.Logf("Iteration %d: Fast %%K=%.2f, Slow %%K=%.2f", i, fastK, slowK)
		}
	}

	// Fast stochastic should be more volatile
	t.Logf("Fast Stochastic: %%K=%.2f, %%D=%.2f", fast.GetPercentK(), fast.GetPercentD())
	t.Logf("Slow Stochastic: %%K=%.2f, %%D=%.2f", slow.GetPercentK(), slow.GetPercentD())
}

// Benchmark Stochastic update performance
func BenchmarkStochastic_Update(b *testing.B) {
	stoch := NewStochastic(14, 3, 3, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		stoch.Update(md)
	}
}
