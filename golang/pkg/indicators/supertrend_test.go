package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewSupertrend(t *testing.T) {
	st := NewSupertrend(10, 3.0, 100)
	if st == nil || st.period != 10 || st.multiplier != 3.0 {
		t.Fatal("Failed to create Supertrend")
	}
	if st.IsReady() {
		t.Error("Supertrend should not be ready initially")
	}
}

func TestSupertrend_Trends(t *testing.T) {
	st := NewSupertrend(10, 3.0, 100)

	// Uptrend
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 1},
			AskPrice: []float64{price + 1},
		}
		st.Update(md)

		if st.IsReady() && i >= 20 {
			t.Logf("Uptrend %d: Price=%.0f, ST=%.2f, Trend=%v",
				i, price, st.GetValue(), st.IsUptrend())
		}
	}

	if st.IsReady() && st.IsUptrend() {
		t.Logf("✓ Uptrend detected")
	}

	// Downtrend
	for i := 0; i < 30; i++ {
		price := 187.0 - float64(i)*3.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 1},
			AskPrice: []float64{price + 1},
		}
		st.Update(md)

		if st.IsBearishReversal() {
			t.Logf("✓ Bearish reversal detected at iteration %d", i)
		}
	}

	if st.IsDowntrend() {
		t.Logf("✓ Downtrend detected")
	}
}

func TestSupertrend_Reversals(t *testing.T) {
	st := NewSupertrend(7, 2.5, 100)

	// Uptrend
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)*2.0},
			AskPrice: []float64{100.0 + float64(i)*2.0},
		}
		st.Update(md)
	}

	// Downtrend transition
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{138.0 - float64(i)*2.5},
			AskPrice: []float64{138.0 - float64(i)*2.5},
		}
		st.Update(md)
		if st.IsBearishReversal() {
			t.Logf("✓ Bearish reversal at iteration %d", i)
		}
	}

	// Uptrend transition
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{88.0 + float64(i)*2.5},
			AskPrice: []float64{88.0 + float64(i)*2.5},
		}
		st.Update(md)
		if st.IsBullishReversal() {
			t.Logf("✓ Bullish reversal at iteration %d", i)
		}
	}
}

func TestSupertrend_Reset(t *testing.T) {
	st := NewSupertrend(10, 3.0, 100)
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{100.0 + float64(i)},
		}
		st.Update(md)
	}

	st.Reset()
	if st.IsReady() || st.GetValue() != 0 {
		t.Error("Supertrend should reset properly")
	}
}

func TestSupertrend_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(14),
		"multiplier":  2.5,
		"max_history": float64(500),
	}

	indicator, err := NewSupertrendFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create Supertrend from config: %v", err)
	}

	st, ok := indicator.(*Supertrend)
	if !ok || st.period != 14 || st.multiplier != 2.5 {
		t.Error("Config creation failed")
	}
}

func BenchmarkSupertrend_Update(b *testing.B) {
	st := NewSupertrend(10, 3.0, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		st.Update(md)
	}
}
