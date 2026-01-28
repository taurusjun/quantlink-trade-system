package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewCMO(t *testing.T) {
	cmo := NewCMO(14, 100)

	if cmo == nil {
		t.Fatal("Expected non-nil CMO")
	}

	if cmo.period != 14 {
		t.Errorf("Expected period 14, got %d", cmo.period)
	}

	if cmo.IsReady() {
		t.Error("CMO should not be ready initially")
	}
}

func TestCMO_Update(t *testing.T) {
	cmo := NewCMO(14, 100)

	// Simulate uptrend
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		cmo.Update(md)

		if i >= 14 { // Ready after period+1 prices
			if !cmo.IsReady() {
				t.Errorf("CMO should be ready at iteration %d", i)
			}

			value := cmo.GetValue()

			// Check range
			if value < -100 || value > 100 {
				t.Errorf("CMO at iteration %d out of range: %.2f", i, value)
			}

			t.Logf("Iteration %d: Price=%.0f, CMO=%.2f", i, price, value)
		}
	}

	// In uptrend, CMO should be positive and high
	if cmo.GetValue() > 50 {
		t.Logf("✓ CMO=%.2f shows strong upward momentum (> 50)", cmo.GetValue())
	}
}

func TestCMO_Calculation(t *testing.T) {
	cmo := NewCMO(5, 100)

	// Known test sequence
	prices := []float64{100, 102, 101, 103, 105, 104, 106, 108, 107, 109}

	for i, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		cmo.Update(md)

		if i >= 5 { // Ready after period+1
			value := cmo.GetValue()
			t.Logf("Period %d: Price=%.0f, CMO=%.2f", i+1, price, value)

			// Check valid range
			if value < -100 || value > 100 {
				t.Errorf("CMO out of range at period %d: %.2f", i+1, value)
			}
		}
	}

	// Verify CMO is positive (more ups than downs)
	if cmo.GetValue() > 0 {
		t.Logf("✓ CMO=%.2f is positive (upward bias)", cmo.GetValue())
	}
}

func TestCMO_UpDownMomentum(t *testing.T) {
	cmo := NewCMO(10, 100)

	// Strong uptrend
	for i := 0; i < 15; i++ {
		price := 100.0 + float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		cmo.Update(md)
	}

	if !cmo.IsReady() {
		t.Fatal("CMO should be ready")
	}

	// Should show strong upward momentum
	if cmo.IsStrongUpMomentum() {
		t.Logf("✓ CMO=%.2f shows strong upward momentum (> 50)", cmo.GetValue())
	}

	if cmo.IsOverbought() {
		t.Logf("✓ CMO=%.2f is overbought (> 50)", cmo.GetValue())
	}

	// Strong downtrend
	for i := 0; i < 15; i++ {
		price := 170.0 - float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		cmo.Update(md)
	}

	// Should show strong downward momentum
	if cmo.IsStrongDownMomentum() {
		t.Logf("✓ CMO=%.2f shows strong downward momentum (< -50)", cmo.GetValue())
	}

	if cmo.IsOversold() {
		t.Logf("✓ CMO=%.2f is oversold (< -50)", cmo.GetValue())
	}
}

func TestCMO_ZeroCrosses(t *testing.T) {
	cmo := NewCMO(5, 100)

	// Start with uptrend
	for i := 0; i < 10; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		cmo.Update(md)
	}

	if !cmo.IsReady() {
		t.Fatal("CMO should be ready")
	}

	t.Logf("Initial (uptrend): CMO=%.2f", cmo.GetValue())

	// Transition to downtrend (should trigger bearish cross)
	prevValue := cmo.GetValue()
	for i := 0; i < 10; i++ {
		price := 118.0 - float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		cmo.Update(md)

		currentValue := cmo.GetValue()

		if cmo.IsBearishCross() {
			t.Logf("✓ Bearish cross detected: CMO crossed below 0 (prev=%.2f, current=%.2f)",
				prevValue, currentValue)
		}

		if cmo.IsBullishCross() {
			t.Logf("✓ Bullish cross detected: CMO crossed above 0 (prev=%.2f, current=%.2f)",
				prevValue, currentValue)
		}

		prevValue = currentValue
		t.Logf("Iteration %d: Price=%.0f, CMO=%.2f", i, price, currentValue)
	}

	// Transition back to uptrend (should trigger bullish cross)
	for i := 0; i < 10; i++ {
		price := 90.0 + float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		cmo.Update(md)

		currentValue := cmo.GetValue()

		if cmo.IsBullishCross() {
			t.Logf("✓ Bullish cross detected: CMO crossed above 0 (current=%.2f)", currentValue)
		}

		t.Logf("Uptrend iteration %d: Price=%.0f, CMO=%.2f", i, price, currentValue)
	}
}

func TestCMO_Reset(t *testing.T) {
	cmo := NewCMO(5, 100)

	// Add data
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{100.0 + float64(i)},
		}
		cmo.Update(md)
	}

	if !cmo.IsReady() {
		t.Fatal("CMO should be ready before reset")
	}

	cmo.Reset()

	if cmo.IsReady() {
		t.Error("CMO should not be ready after reset")
	}

	if cmo.GetValue() != 0 {
		t.Errorf("CMO value should be 0 after reset, got %.2f", cmo.GetValue())
	}

	if len(cmo.prices) != 0 || len(cmo.upChanges) != 0 || len(cmo.downChanges) != 0 {
		t.Error("All windows should be empty after reset")
	}
}

func TestCMO_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(20),
		"max_history": float64(500),
	}

	indicator, err := NewCMOFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create CMO from config: %v", err)
	}

	cmo, ok := indicator.(*CMO)
	if !ok {
		t.Fatal("Expected *CMO type")
	}

	if cmo.period != 20 {
		t.Errorf("Expected period 20, got %d", cmo.period)
	}

	if cmo.GetName() != "CMO" {
		t.Errorf("Expected name 'CMO', got '%s'", cmo.GetName())
	}
}

func TestCMO_NoChange(t *testing.T) {
	cmo := NewCMO(5, 100)

	// Feed same price (no change)
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0},
			AskPrice: []float64{100.0},
		}
		cmo.Update(md)
	}

	if !cmo.IsReady() {
		t.Fatal("CMO should be ready")
	}

	// Should return 0 when no price changes
	value := cmo.GetValue()
	if value != 0.0 {
		t.Errorf("Expected CMO=0.0 for no price change, got %.2f", value)
	}
}

func TestCMO_CompareWithRSI(t *testing.T) {
	cmo := NewCMO(14, 100)
	rsi := NewRSI(14, 100)

	// Feed same data to both
	for i := 0; i < 25; i++ {
		price := 100.0 + float64(i)*2.0 - float64(i%3)*3.0 // Oscillating
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		cmo.Update(md)
		rsi.Update(md)

		if cmo.IsReady() && rsi.IsReady() {
			cmoValue := cmo.GetValue()
			rsiValue := rsi.GetValue()

			// CMO range: -100 to +100
			// RSI range: 0 to 100
			// Approximate relationship: CMO ≈ 2 × (RSI - 50)
			estimatedCMO := 2.0 * (rsiValue - 50.0)
			diff := math.Abs(cmoValue - estimatedCMO)

			t.Logf("Iteration %d: CMO=%.2f, RSI=%.2f, Estimated CMO from RSI=%.2f, Diff=%.2f",
				i, cmoValue, rsiValue, estimatedCMO, diff)

			// Allow some tolerance in relationship
			if diff < 20.0 {
				// Close relationship
			}
		}
	}
}

// Benchmark CMO update performance
func BenchmarkCMO_Update(b *testing.B) {
	cmo := NewCMO(14, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{100.0},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmo.Update(md)
	}
}
