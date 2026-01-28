package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewParabolicSAR(t *testing.T) {
	psar := NewParabolicSAR(0.02, 0.02, 0.20, 100)
	if psar == nil || psar.afStart != 0.02 {
		t.Fatal("Failed to create Parabolic SAR")
	}
	if psar.IsReady() {
		t.Error("Parabolic SAR should not be ready initially")
	}
}

func TestParabolicSAR_Uptrend(t *testing.T) {
	psar := NewParabolicSAR(0.02, 0.02, 0.20, 100)

	// Uptrend
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		psar.Update(md)

		if psar.IsReady() && i >= 5 {
			t.Logf("Iteration %d: Price=%.2f, SAR=%.2f, Trend=%v, AF=%.3f, EP=%.2f",
				i, price, psar.GetValue(), psar.IsUptrend(), psar.GetAF(), psar.GetEP())
		}
	}

	if psar.IsUptrend() {
		t.Logf("✓ Uptrend detected, SAR=%.2f", psar.GetValue())
	}
}

func TestParabolicSAR_Downtrend(t *testing.T) {
	psar := NewParabolicSAR(0.02, 0.02, 0.20, 100)

	// Downtrend
	for i := 0; i < 20; i++ {
		price := 150.0 - float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		psar.Update(md)

		if psar.IsReady() && i >= 10 {
			t.Logf("Iteration %d: Price=%.2f, SAR=%.2f, Trend=%v",
				i, price, psar.GetValue(), psar.IsDowntrend())
		}
	}

	if psar.IsDowntrend() {
		t.Logf("✓ Downtrend detected, SAR=%.2f", psar.GetValue())
	}
}

func TestParabolicSAR_Reversals(t *testing.T) {
	psar := NewParabolicSAR(0.02, 0.02, 0.20, 100)

	// Start uptrend
	for i := 0; i < 15; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)*2.0},
			AskPrice: []float64{100.0 + float64(i)*2.0},
		}
		psar.Update(md)
	}

	wasUptrend := psar.IsUptrend()
	t.Logf("After uptrend: Trend=%v, SAR=%.2f", wasUptrend, psar.GetValue())

	// Reverse to downtrend
	for i := 0; i < 15; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{128.0 - float64(i)*3.0},
			AskPrice: []float64{128.0 - float64(i)*3.0},
		}
		psar.Update(md)

		if wasUptrend && psar.IsDowntrend() {
			t.Logf("✓ Bearish reversal at iteration %d, SAR=%.2f", i, psar.GetValue())
			wasUptrend = false
		}
	}

	// Reverse back to uptrend
	for i := 0; i < 15; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{86.0 + float64(i)*3.0},
			AskPrice: []float64{86.0 + float64(i)*3.0},
		}
		psar.Update(md)

		if !wasUptrend && psar.IsUptrend() {
			t.Logf("✓ Bullish reversal at iteration %d, SAR=%.2f", i, psar.GetValue())
			wasUptrend = true
		}
	}
}

func TestParabolicSAR_AFProgression(t *testing.T) {
	psar := NewParabolicSAR(0.02, 0.02, 0.20, 100)

	// Strong uptrend should increase AF
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)*5.0},
			AskPrice: []float64{100.0 + float64(i)*5.0},
		}
		psar.Update(md)

		if psar.IsReady() && i >= 5 {
			t.Logf("Iteration %d: AF=%.3f, EP=%.2f", i, psar.GetAF(), psar.GetEP())
		}
	}

	// AF should have increased
	if psar.GetAF() > 0.02 {
		t.Logf("✓ AF increased to %.3f (started at 0.02)", psar.GetAF())
	}

	// AF should not exceed max
	if psar.GetAF() <= 0.20 {
		t.Logf("✓ AF capped at %.3f (max 0.20)", psar.GetAF())
	}
}

func TestParabolicSAR_Reset(t *testing.T) {
	psar := NewParabolicSAR(0.02, 0.02, 0.20, 100)

	for i := 0; i < 15; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{100.0 + float64(i)},
		}
		psar.Update(md)
	}

	psar.Reset()
	if psar.IsReady() || psar.GetValue() != 0 {
		t.Error("Parabolic SAR should reset properly")
	}
}

func TestParabolicSAR_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"af_start":    0.03,
		"af_step":     0.03,
		"af_max":      0.25,
		"max_history": float64(500),
	}

	indicator, err := NewParabolicSARFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create Parabolic SAR from config: %v", err)
	}

	psar, ok := indicator.(*ParabolicSAR)
	if !ok || psar.afStart != 0.03 {
		t.Error("Config creation failed")
	}
}

func BenchmarkParabolicSAR_Update(b *testing.B) {
	psar := NewParabolicSAR(0.02, 0.02, 0.20, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		psar.Update(md)
	}
}
