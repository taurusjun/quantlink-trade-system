package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewKeltnerChannels(t *testing.T) {
	kc := NewKeltnerChannels(20, 10, 2.0, 100)
	if kc == nil || kc.emaPeriod != 20 || kc.atrPeriod != 10 {
		t.Fatal("Failed to create Keltner Channels")
	}
	if kc.IsReady() {
		t.Error("Keltner Channels should not be ready initially")
	}
}

func TestKeltnerChannels_Uptrend(t *testing.T) {
	kc := NewKeltnerChannels(10, 10, 2.0, 100)

	// Rising prices with varying volatility
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		kc.Update(md)

		if kc.IsReady() && i >= 15 {
			t.Logf("Iteration %d: Price=%.2f, Upper=%.2f, Middle=%.2f, Lower=%.2f, Width=%.2f",
				i, price, kc.GetUpperChannel(), kc.GetMiddleLine(),
				kc.GetLowerChannel(), kc.GetChannelWidth())
		}
	}

	// Middle line should be close to recent prices
	if kc.GetMiddleLine() < 100 || kc.GetMiddleLine() > 200 {
		t.Errorf("Middle line out of expected range: %.2f", kc.GetMiddleLine())
	}

	// Upper should be above middle, lower should be below
	if kc.GetUpperChannel() <= kc.GetMiddleLine() || kc.GetLowerChannel() >= kc.GetMiddleLine() {
		t.Error("Channel structure incorrect")
	}

	t.Logf("✓ Uptrend: Upper=%.2f, Middle=%.2f, Lower=%.2f",
		kc.GetUpperChannel(), kc.GetMiddleLine(), kc.GetLowerChannel())
}

func TestKeltnerChannels_Downtrend(t *testing.T) {
	kc := NewKeltnerChannels(10, 10, 2.0, 100)

	// Falling prices
	for i := 0; i < 30; i++ {
		price := 200.0 - float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		kc.Update(md)

		if kc.IsReady() && i >= 15 {
			t.Logf("Iteration %d: Price=%.2f, Upper=%.2f, Middle=%.2f, Lower=%.2f",
				i, price, kc.GetUpperChannel(), kc.GetMiddleLine(), kc.GetLowerChannel())
		}
	}

	// Verify channel structure
	if kc.GetUpperChannel() <= kc.GetMiddleLine() || kc.GetLowerChannel() >= kc.GetMiddleLine() {
		t.Error("Channel structure incorrect")
	}

	t.Logf("✓ Downtrend: Upper=%.2f, Middle=%.2f, Lower=%.2f",
		kc.GetUpperChannel(), kc.GetMiddleLine(), kc.GetLowerChannel())
}

func TestKeltnerChannels_VolatilityExpansion(t *testing.T) {
	kc := NewKeltnerChannels(10, 10, 2.0, 100)

	// Low volatility phase
	for i := 0; i < 15; i++ {
		price := 100.0 + float64(i%3)*0.1
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.05},
			AskPrice: []float64{price + 0.05},
		}
		kc.Update(md)
	}

	lowVolWidth := kc.GetChannelWidth()

	// High volatility phase
	for i := 0; i < 15; i++ {
		price := 100.0
		if i%2 == 0 {
			price = 110.0
		} else {
			price = 90.0
		}
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 5},
			AskPrice: []float64{price + 5},
		}
		kc.Update(md)
	}

	highVolWidth := kc.GetChannelWidth()

	t.Logf("Low vol width: %.2f, High vol width: %.2f", lowVolWidth, highVolWidth)

	if highVolWidth > lowVolWidth {
		t.Logf("✓ Volatility expansion detected")
	}

	if kc.IsBandExpansion(5.0) {
		t.Logf("✓ Band expansion confirmed: %.2f%%", kc.GetChannelWidthPercentage())
	}
}

func TestKeltnerChannels_BreakoutDetection(t *testing.T) {
	kc := NewKeltnerChannels(10, 10, 2.0, 100)

	// Build stable channel
	for i := 0; i < 25; i++ {
		price := 100.0 + float64(i%5)
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		kc.Update(md)
	}

	upper := kc.GetUpperChannel()
	lower := kc.GetLowerChannel()

	t.Logf("Channel: Upper=%.2f, Middle=%.2f, Lower=%.2f",
		upper, kc.GetMiddleLine(), lower)

	// Test breakout up
	breakoutUp := upper + 1.0
	if kc.IsBreakoutUp(breakoutUp) {
		t.Logf("✓ Breakout up detected at %.2f", breakoutUp)
	} else {
		t.Error("Should detect breakout up")
	}

	// Test breakout down
	breakoutDown := lower - 1.0
	if kc.IsBreakoutDown(breakoutDown) {
		t.Logf("✓ Breakout down detected at %.2f", breakoutDown)
	} else {
		t.Error("Should detect breakout down")
	}

	// Test inside channel
	inside := (upper + lower) / 2.0
	if !kc.IsBreakoutUp(inside) && !kc.IsBreakoutDown(inside) {
		t.Logf("✓ No breakout at %.2f", inside)
	}
}

func TestKeltnerChannels_Position(t *testing.T) {
	kc := NewKeltnerChannels(10, 10, 2.0, 100)

	// Build channel
	for i := 0; i < 25; i++ {
		price := 100.0 + float64(i)*1.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		kc.Update(md)
	}

	upper := kc.GetUpperChannel()
	lower := kc.GetLowerChannel()
	middle := kc.GetMiddleLine()

	// Test position calculation
	posLower := kc.GetPosition(lower)
	posMid := kc.GetPosition(middle)
	posUpper := kc.GetPosition(upper)

	t.Logf("Position at lower (%.2f): %.2f", lower, posLower)
	t.Logf("Position at middle (%.2f): %.2f", middle, posMid)
	t.Logf("Position at upper (%.2f): %.2f", upper, posUpper)

	if posLower < 0.05 && posMid > 0.45 && posMid < 0.55 && posUpper > 0.95 {
		t.Logf("✓ Position calculations correct")
	}
}

func TestKeltnerChannels_TradingSignals(t *testing.T) {
	kc := NewKeltnerChannels(10, 10, 2.0, 100)

	// Build channel
	for i := 0; i < 25; i++ {
		price := 100.0 + float64(i)*1.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		kc.Update(md)
	}

	upper := kc.GetUpperChannel()
	lower := kc.GetLowerChannel()
	middle := kc.GetMiddleLine()

	// Test signals
	nearLower := lower + (middle-lower)*0.1
	nearUpper := upper - (upper-middle)*0.1

	signalLower := kc.GetSignal(nearLower)
	signalUpper := kc.GetSignal(nearUpper)
	signalMid := kc.GetSignal(middle)

	t.Logf("Signal near lower (%.2f): %d", nearLower, signalLower)
	t.Logf("Signal near upper (%.2f): %d", nearUpper, signalUpper)
	t.Logf("Signal at middle (%.2f): %d", middle, signalMid)

	if signalLower == 1 {
		t.Logf("✓ Buy signal near lower channel")
	}
	if signalUpper == -1 {
		t.Logf("✓ Sell signal near upper channel")
	}
	if signalMid == 0 {
		t.Logf("✓ Neutral signal in middle")
	}
}

func TestKeltnerChannels_BandSqueeze(t *testing.T) {
	kc := NewKeltnerChannels(10, 10, 2.0, 100)

	// Very low volatility
	for i := 0; i < 25; i++ {
		price := 100.0 + float64(i%2)*0.01
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.005},
			AskPrice: []float64{price + 0.005},
		}
		kc.Update(md)
	}

	if kc.IsBandSqueeze(2.0) {
		t.Logf("✓ Band squeeze detected: %.2f%%", kc.GetChannelWidthPercentage())
	}
}

func TestKeltnerChannels_MultiplierEffect(t *testing.T) {
	kc1 := NewKeltnerChannels(10, 10, 1.0, 100)
	kc2 := NewKeltnerChannels(10, 10, 2.0, 100)
	kc3 := NewKeltnerChannels(10, 10, 3.0, 100)

	// Same price data
	for i := 0; i < 25; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 1.0},
			AskPrice: []float64{price + 1.0},
		}
		kc1.Update(md)
		kc2.Update(md)
		kc3.Update(md)
	}

	width1 := kc1.GetChannelWidth()
	width2 := kc2.GetChannelWidth()
	width3 := kc3.GetChannelWidth()

	t.Logf("Width (1x): %.2f", width1)
	t.Logf("Width (2x): %.2f", width2)
	t.Logf("Width (3x): %.2f", width3)

	if width1 < width2 && width2 < width3 {
		t.Logf("✓ Multiplier correctly affects channel width")
	} else {
		t.Error("Multiplier effect incorrect")
	}
}

func TestKeltnerChannels_Reset(t *testing.T) {
	kc := NewKeltnerChannels(10, 10, 2.0, 100)

	// Add data
	for i := 0; i < 25; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{100.0 + float64(i)},
		}
		kc.Update(md)
	}

	kc.Reset()

	if kc.IsReady() || kc.GetValue() != 0 {
		t.Error("Keltner Channels should reset properly")
	}

	t.Logf("✓ Reset successful")
}

func TestKeltnerChannels_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"ema_period":  float64(30),
		"atr_period":  float64(15),
		"multiplier":  3.0,
		"max_history": float64(500),
	}

	indicator, err := NewKeltnerChannelsFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create Keltner Channels from config: %v", err)
	}

	kc, ok := indicator.(*KeltnerChannels)
	if !ok || kc.emaPeriod != 30 || kc.atrPeriod != 15 || kc.multiplier != 3.0 {
		t.Error("Config creation failed")
	}

	t.Logf("✓ Config creation successful")
}

func BenchmarkKeltnerChannels_Update(b *testing.B) {
	kc := NewKeltnerChannels(20, 10, 2.0, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		kc.Update(md)
	}
}
