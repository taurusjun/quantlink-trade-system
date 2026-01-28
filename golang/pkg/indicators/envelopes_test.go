package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewEnvelopes(t *testing.T) {
	env := NewEnvelopes(20, 2.5, false, 100)
	if env == nil || env.period != 20 || env.percentage != 2.5 {
		t.Fatal("Failed to create Envelopes")
	}
	if env.IsReady() {
		t.Error("Envelopes should not be ready initially")
	}
}

func TestEnvelopes_SMA(t *testing.T) {
	env := NewEnvelopes(10, 2.5, false, 100)

	// Rising prices
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		env.Update(md)

		if env.IsReady() && i >= 10 {
			t.Logf("Iteration %d: Price=%.2f, Upper=%.2f, Middle=%.2f, Lower=%.2f",
				i, price, env.GetUpperBand(), env.GetMiddleLine(), env.GetLowerBand())
		}
	}

	// Verify band structure
	middle := env.GetMiddleLine()
	upper := env.GetUpperBand()
	lower := env.GetLowerBand()

	// Upper should be 2.5% above middle
	expectedUpper := middle * 1.025
	expectedLower := middle * 0.975

	if upper < expectedUpper-0.5 || upper > expectedUpper+0.5 {
		t.Errorf("Upper band incorrect: %.2f vs expected %.2f", upper, expectedUpper)
	}
	if lower < expectedLower-0.5 || lower > expectedLower+0.5 {
		t.Errorf("Lower band incorrect: %.2f vs expected %.2f", lower, expectedLower)
	}

	t.Logf("✓ SMA Envelopes: Upper=%.2f (%.2f%%), Middle=%.2f, Lower=%.2f",
		upper, (upper/middle-1)*100, middle, lower)
}

func TestEnvelopes_EMA(t *testing.T) {
	env := NewEnvelopes(10, 2.5, true, 100)

	// Rising prices
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		env.Update(md)

		if env.IsReady() && i >= 10 {
			t.Logf("Iteration %d: Price=%.2f, Upper=%.2f, Middle=%.2f, Lower=%.2f",
				i, price, env.GetUpperBand(), env.GetMiddleLine(), env.GetLowerBand())
		}
	}

	if env.IsUsingEMA() {
		t.Logf("✓ Using EMA")
	} else {
		t.Error("Should be using EMA")
	}

	t.Logf("✓ EMA Envelopes: Upper=%.2f, Middle=%.2f, Lower=%.2f",
		env.GetUpperBand(), env.GetMiddleLine(), env.GetLowerBand())
}

func TestEnvelopes_DifferentPercentages(t *testing.T) {
	env1 := NewEnvelopes(10, 1.0, false, 100)
	env2 := NewEnvelopes(10, 2.5, false, 100)
	env3 := NewEnvelopes(10, 5.0, false, 100)

	// Same price data
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		env1.Update(md)
		env2.Update(md)
		env3.Update(md)
	}

	width1 := env1.GetBandWidth()
	width2 := env2.GetBandWidth()
	width3 := env3.GetBandWidth()

	t.Logf("Width (1.0%%): %.2f", width1)
	t.Logf("Width (2.5%%): %.2f", width2)
	t.Logf("Width (5.0%%): %.2f", width3)

	if width1 < width2 && width2 < width3 {
		t.Logf("✓ Percentage correctly affects band width")
	} else {
		t.Error("Percentage effect incorrect")
	}
}

func TestEnvelopes_OverboughtOversold(t *testing.T) {
	env := NewEnvelopes(10, 2.5, false, 100)

	// Build envelope
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{99.5},
			AskPrice: []float64{100.5},
		}
		env.Update(md)
	}

	upper := env.GetUpperBand()
	lower := env.GetLowerBand()

	// Test overbought
	overboughtPrice := upper + 1.0
	if env.IsOverbought(overboughtPrice) {
		t.Logf("✓ Overbought detected at %.2f", overboughtPrice)
	} else {
		t.Error("Should detect overbought")
	}

	// Test oversold
	oversoldPrice := lower - 1.0
	if env.IsOversold(oversoldPrice) {
		t.Logf("✓ Oversold detected at %.2f", oversoldPrice)
	} else {
		t.Error("Should detect oversold")
	}

	// Test normal
	normalPrice := (upper + lower) / 2.0
	if !env.IsOverbought(normalPrice) && !env.IsOversold(normalPrice) {
		t.Logf("✓ Normal range detected")
	}
}

func TestEnvelopes_BreakoutDetection(t *testing.T) {
	env := NewEnvelopes(10, 2.5, false, 100)

	// Build envelope
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{99.5},
			AskPrice: []float64{100.5},
		}
		env.Update(md)
	}

	upper := env.GetUpperBand()
	lower := env.GetLowerBand()

	t.Logf("Bands: Upper=%.2f, Middle=%.2f, Lower=%.2f",
		upper, env.GetMiddleLine(), lower)

	// Test breakout up
	if env.IsBreakoutUp(upper + 0.5) {
		t.Logf("✓ Breakout up detected")
	} else {
		t.Error("Should detect breakout up")
	}

	// Test breakout down
	if env.IsBreakoutDown(lower - 0.5) {
		t.Logf("✓ Breakout down detected")
	} else {
		t.Error("Should detect breakout down")
	}
}

func TestEnvelopes_Position(t *testing.T) {
	env := NewEnvelopes(10, 2.5, false, 100)

	// Build envelope
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{99.5},
			AskPrice: []float64{100.5},
		}
		env.Update(md)
	}

	upper := env.GetUpperBand()
	lower := env.GetLowerBand()
	middle := env.GetMiddleLine()

	// Test positions
	posLower := env.GetPosition(lower)
	posMid := env.GetPosition(middle)
	posUpper := env.GetPosition(upper)

	t.Logf("Position at lower (%.2f): %.2f", lower, posLower)
	t.Logf("Position at middle (%.2f): %.2f", middle, posMid)
	t.Logf("Position at upper (%.2f): %.2f", upper, posUpper)

	if posLower < 0.05 && posMid > 0.45 && posMid < 0.55 && posUpper > 0.95 {
		t.Logf("✓ Position calculations correct")
	}
}

func TestEnvelopes_TradingSignals(t *testing.T) {
	env := NewEnvelopes(10, 2.5, false, 100)

	// Build envelope
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{99.5},
			AskPrice: []float64{100.5},
		}
		env.Update(md)
	}

	upper := env.GetUpperBand()
	lower := env.GetLowerBand()
	middle := env.GetMiddleLine()

	// Test signals
	nearLower := lower + (middle-lower)*0.1
	nearUpper := upper - (upper-middle)*0.1

	signalLower := env.GetSignal(nearLower)
	signalUpper := env.GetSignal(nearUpper)
	signalMid := env.GetSignal(middle)

	t.Logf("Signal near lower (%.2f): %d", nearLower, signalLower)
	t.Logf("Signal near upper (%.2f): %d", nearUpper, signalUpper)
	t.Logf("Signal at middle (%.2f): %d", middle, signalMid)

	if signalLower == 1 {
		t.Logf("✓ Buy signal near lower band")
	}
	if signalUpper == -1 {
		t.Logf("✓ Sell signal near upper band")
	}
	if signalMid == 0 {
		t.Logf("✓ Neutral signal in middle")
	}
}

func TestEnvelopes_BandDistance(t *testing.T) {
	env := NewEnvelopes(10, 2.5, false, 100)

	// Build envelope
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{99.5},
			AskPrice: []float64{100.5},
		}
		env.Update(md)
	}

	middle := env.GetMiddleLine()

	// Test distance at middle (should be equal to upper or lower)
	dist := env.GetBandDistance(middle)
	t.Logf("Distance from middle: %.2f", dist)

	if dist > 0 {
		t.Logf("✓ Band distance calculated")
	}
}

func TestEnvelopes_Reset(t *testing.T) {
	env := NewEnvelopes(10, 2.5, false, 100)

	// Add data
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{99.5},
			AskPrice: []float64{100.5},
		}
		env.Update(md)
	}

	env.Reset()

	if env.IsReady() || env.GetValue() != 0 {
		t.Error("Envelopes should reset properly")
	}

	t.Logf("✓ Reset successful")
}

func TestEnvelopes_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(30),
		"percentage":  5.0,
		"use_ema":     true,
		"max_history": float64(500),
	}

	indicator, err := NewEnvelopesFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create Envelopes from config: %v", err)
	}

	env, ok := indicator.(*Envelopes)
	if !ok || env.period != 30 || env.percentage != 5.0 || !env.useEMA {
		t.Error("Config creation failed")
	}

	t.Logf("✓ Config creation successful")
}

func BenchmarkEnvelopes_Update(b *testing.B) {
	env := NewEnvelopes(20, 2.5, false, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env.Update(md)
	}
}
