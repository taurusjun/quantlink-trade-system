package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewAroon(t *testing.T) {
	aroon := NewAroon(25, 100)

	if aroon == nil {
		t.Fatal("Expected non-nil Aroon")
	}

	if aroon.period != 25 {
		t.Errorf("Expected period 25, got %d", aroon.period)
	}

	if aroon.IsReady() {
		t.Error("Aroon should not be ready initially")
	}
}

func TestAroon_Update(t *testing.T) {
	aroon := NewAroon(14, 100)

	// Simulate uptrend (new highs)
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 1},
			AskPrice: []float64{price + 1},
		}
		aroon.Update(md)

		if i >= 13 { // Ready after period
			if !aroon.IsReady() {
				t.Errorf("Aroon should be ready at iteration %d", i)
			}

			aroonUp := aroon.GetAroonUp()
			aroonDown := aroon.GetAroonDown()
			osc := aroon.GetOscillator()

			// Check ranges
			if aroonUp < 0 || aroonUp > 100 {
				t.Errorf("Aroon Up at iteration %d out of range: %.2f", i, aroonUp)
			}
			if aroonDown < 0 || aroonDown > 100 {
				t.Errorf("Aroon Down at iteration %d out of range: %.2f", i, aroonDown)
			}
			if osc < -100 || osc > 100 {
				t.Errorf("Aroon Oscillator at iteration %d out of range: %.2f", i, osc)
			}

			t.Logf("Iteration %d: Price=%.0f, Up=%.2f, Down=%.2f, Osc=%.2f",
				i, price, aroonUp, aroonDown, osc)
		}
	}

	// In strong uptrend, Aroon Up should be high
	if aroon.IsStrongUptrend() {
		t.Logf("✓ Aroon Up=%.2f > 70: Strong uptrend detected", aroon.GetAroonUp())
	}
}

func TestAroon_Trends(t *testing.T) {
	aroon := NewAroon(14, 100)

	// Strong uptrend
	for i := 0; i < 25; i++ {
		price := 100.0 + float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		aroon.Update(md)
	}

	if !aroon.IsReady() {
		t.Fatal("Aroon should be ready")
	}

	t.Logf("Uptrend: Up=%.2f, Down=%.2f, Osc=%.2f",
		aroon.GetAroonUp(), aroon.GetAroonDown(), aroon.GetOscillator())

	if aroon.IsStrongUptrend() {
		t.Logf("✓ Strong uptrend detected (Aroon Up > 70)")
	}

	if aroon.IsTrendingUp() {
		t.Logf("✓ Trending up (Oscillator > 0)")
	}

	// Strong downtrend
	for i := 0; i < 25; i++ {
		price := 220.0 - float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		aroon.Update(md)
	}

	t.Logf("Downtrend: Up=%.2f, Down=%.2f, Osc=%.2f",
		aroon.GetAroonUp(), aroon.GetAroonDown(), aroon.GetOscillator())

	if aroon.IsStrongDowntrend() {
		t.Logf("✓ Strong downtrend detected (Aroon Down > 70)")
	}

	if aroon.IsTrendingDown() {
		t.Logf("✓ Trending down (Oscillator < 0)")
	}
}

func TestAroon_Consolidation(t *testing.T) {
	aroon := NewAroon(14, 100)

	// Initial uptrend to populate
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		aroon.Update(md)
	}

	// Consolidation (sideways movement)
	for i := 0; i < 20; i++ {
		price := 138.0 + float64(i%4)*3.0 - 4.5
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		aroon.Update(md)
	}

	t.Logf("Consolidation: Up=%.2f, Down=%.2f, Osc=%.2f",
		aroon.GetAroonUp(), aroon.GetAroonDown(), aroon.GetOscillator())

	if aroon.IsConsolidating() {
		t.Logf("✓ Consolidation detected (both Up and Down < 50)")
	}
}

func TestAroon_Crossovers(t *testing.T) {
	aroon := NewAroon(10, 100)

	// Start with uptrend
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		aroon.Update(md)
	}

	if !aroon.IsReady() {
		t.Fatal("Aroon should be ready")
	}

	t.Logf("Initial uptrend: Up=%.2f, Down=%.2f", aroon.GetAroonUp(), aroon.GetAroonDown())

	// Transition to downtrend (should trigger bearish cross)
	for i := 0; i < 20; i++ {
		price := 157.0 - float64(i)*4.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		aroon.Update(md)

		if aroon.IsBearishCross() {
			t.Logf("✓ Bearish cross detected: Down (%.2f) crossed above Up (%.2f)",
				aroon.GetAroonDown(), aroon.GetAroonUp())
		}
	}

	// Transition back to uptrend (should trigger bullish cross)
	for i := 0; i < 20; i++ {
		price := 81.0 + float64(i)*4.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		aroon.Update(md)

		if aroon.IsBullishCross() {
			t.Logf("✓ Bullish cross detected: Up (%.2f) crossed above Down (%.2f)",
				aroon.GetAroonUp(), aroon.GetAroonDown())
		}
	}
}

func TestAroon_HighLowDistance(t *testing.T) {
	aroon := NewAroon(10, 100)

	// Create specific pattern: high at start, low at end
	prices := []float64{120, 115, 110, 108, 105, 103, 102, 101, 100, 95, 92, 90}

	for i, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		aroon.Update(md)

		if aroon.IsReady() {
			t.Logf("Period %d: Price=%.0f, Up=%.2f, Down=%.2f",
				i+1, price, aroon.GetAroonUp(), aroon.GetAroonDown())
		}
	}

	// Highest high is at the beginning, lowest low is at the end
	// Aroon Down should be very high (100), Aroon Up should be low
	if aroon.GetAroonDown() > 70 {
		t.Logf("✓ Aroon Down=%.2f is high (low is recent)", aroon.GetAroonDown())
	}
	if aroon.GetAroonUp() < 30 {
		t.Logf("✓ Aroon Up=%.2f is low (high is old)", aroon.GetAroonUp())
	}
}

func TestAroon_Reset(t *testing.T) {
	aroon := NewAroon(14, 100)

	// Add data
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{105.0 + float64(i)},
		}
		aroon.Update(md)
	}

	if !aroon.IsReady() {
		t.Fatal("Aroon should be ready before reset")
	}

	aroon.Reset()

	if aroon.IsReady() {
		t.Error("Aroon should not be ready after reset")
	}

	if aroon.GetAroonUp() != 0 || aroon.GetAroonDown() != 0 || aroon.GetOscillator() != 0 {
		t.Errorf("Aroon values should be 0 after reset, got Up=%.2f, Down=%.2f, Osc=%.2f",
			aroon.GetAroonUp(), aroon.GetAroonDown(), aroon.GetOscillator())
	}

	if len(aroon.highs) != 0 || len(aroon.lows) != 0 {
		t.Error("Price windows should be empty after reset")
	}
}

func TestAroon_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(30),
		"max_history": float64(500),
	}

	indicator, err := NewAroonFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create Aroon from config: %v", err)
	}

	aroon, ok := indicator.(*Aroon)
	if !ok {
		t.Fatal("Expected *Aroon type")
	}

	if aroon.period != 30 {
		t.Errorf("Expected period 30, got %d", aroon.period)
	}

	if aroon.GetName() != "Aroon" {
		t.Errorf("Expected name 'Aroon', got '%s'", aroon.GetName())
	}
}

func TestAroon_EdgeCases(t *testing.T) {
	aroon := NewAroon(5, 100)

	// All same price (no trend)
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0},
			AskPrice: []float64{100.0},
		}
		aroon.Update(md)
	}

	if aroon.IsReady() {
		// Both should be 100 (high and low are always at latest)
		t.Logf("Flat market: Up=%.2f, Down=%.2f", aroon.GetAroonUp(), aroon.GetAroonDown())

		if aroon.GetAroonUp() == 100.0 && aroon.GetAroonDown() == 100.0 {
			t.Logf("✓ Both Aroon Up and Down are 100 in flat market")
		}
	}
}

func TestAroon_OscillatorRange(t *testing.T) {
	aroon := NewAroon(10, 100)

	// Feed various patterns
	for i := 0; i < 50; i++ {
		var price float64
		if i < 15 {
			price = 100.0 + float64(i)*5.0 // Uptrend
		} else if i < 30 {
			price = 170.0 - float64(i-15)*3.0 // Downtrend
		} else {
			price = 125.0 + float64(i%5)*4.0 - 8.0 // Ranging
		}

		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		aroon.Update(md)

		if aroon.IsReady() && i >= 45 {
			osc := aroon.GetOscillator()
			t.Logf("Iteration %d: Price=%.0f, Oscillator=%.2f", i, price, osc)

			// Verify oscillator is within bounds
			if osc < -100 || osc > 100 {
				t.Errorf("Oscillator out of range: %.2f", osc)
			}
		}
	}
}

// Benchmark Aroon update performance
func BenchmarkAroon_Update(b *testing.B) {
	aroon := NewAroon(25, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		aroon.Update(md)
	}
}
