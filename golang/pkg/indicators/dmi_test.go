package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewDMI(t *testing.T) {
	dmi := NewDMI(14, 100)
	if dmi == nil || dmi.period != 14 {
		t.Fatal("Failed to create DMI")
	}
	if dmi.IsReady() {
		t.Error("DMI should not be ready initially")
	}
}

func TestDMI_Trends(t *testing.T) {
	dmi := NewDMI(14, 100)

	// Uptrend
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		dmi.Update(md)

		if dmi.IsReady() && i >= 20 {
			t.Logf("Uptrend %d: +DI=%.2f, -DI=%.2f, Spread=%.2f",
				i, dmi.GetPlusDI(), dmi.GetMinusDI(), dmi.GetValue())
		}
	}

	if dmi.IsReady() && dmi.IsTrendingUp() {
		t.Logf("✓ Uptrend: +DI (%.2f) > -DI (%.2f)", dmi.GetPlusDI(), dmi.GetMinusDI())
	}

	// Downtrend
	for i := 0; i < 30; i++ {
		price := 187.0 - float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		dmi.Update(md)
	}

	if dmi.IsTrendingDown() {
		t.Logf("✓ Downtrend: -DI (%.2f) > +DI (%.2f)", dmi.GetMinusDI(), dmi.GetPlusDI())
	}
}

func TestDMI_Crossovers(t *testing.T) {
	dmi := NewDMI(10, 100)

	// Uptrend
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)*2.0},
			AskPrice: []float64{100.0 + float64(i)*2.0},
		}
		dmi.Update(md)
	}

	// Downtrend transition
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{138.0 - float64(i)*3.0},
			AskPrice: []float64{138.0 - float64(i)*3.0},
		}
		dmi.Update(md)
		if dmi.IsBearishCross() {
			t.Logf("✓ Bearish cross at iteration %d", i)
		}
	}

	// Uptrend transition
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{78.0 + float64(i)*3.0},
			AskPrice: []float64{78.0 + float64(i)*3.0},
		}
		dmi.Update(md)
		if dmi.IsBullishCross() {
			t.Logf("✓ Bullish cross at iteration %d", i)
		}
	}
}

func TestDMI_Reset(t *testing.T) {
	dmi := NewDMI(14, 100)
	for i := 0; i < 25; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{100.0 + float64(i)},
		}
		dmi.Update(md)
	}

	dmi.Reset()
	if dmi.IsReady() || dmi.GetPlusDI() != 0 || dmi.GetMinusDI() != 0 {
		t.Error("DMI should reset properly")
	}
}

func TestDMI_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(20),
		"max_history": float64(500),
	}

	indicator, err := NewDMIFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create DMI from config: %v", err)
	}

	dmi, ok := indicator.(*DMI)
	if !ok || dmi.period != 20 {
		t.Error("Config creation failed")
	}
}

func BenchmarkDMI_Update(b *testing.B) {
	dmi := NewDMI(14, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dmi.Update(md)
	}
}
