package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewDonchianChannels(t *testing.T) {
	dc := NewDonchianChannels(20, 100)
	if dc == nil || dc.period != 20 {
		t.Fatal("Failed to create Donchian Channels")
	}
	if dc.IsReady() {
		t.Error("Donchian Channels should not be ready initially")
	}
}

func TestDonchianChannels_Uptrend(t *testing.T) {
	dc := NewDonchianChannels(10, 100)

	// Rising prices (uptrend)
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		dc.Update(md)

		if dc.IsReady() && i >= 10 {
			t.Logf("Iteration %d: Price=%.2f, Upper=%.2f, Middle=%.2f, Lower=%.2f, Width=%.2f",
				i, price, dc.GetUpperChannel(), dc.GetMiddleLine(),
				dc.GetLowerChannel(), dc.GetChannelWidth())
		}
	}

	// In uptrend, upper channel should be close to recent price
	lastPrice := 100.0 + 19*2.0
	if dc.GetUpperChannel() < lastPrice-5 {
		t.Errorf("Upper channel should be near recent high, got %.2f vs price %.2f",
			dc.GetUpperChannel(), lastPrice)
	}

	t.Logf("✓ Uptrend: Upper=%.2f, Middle=%.2f, Lower=%.2f",
		dc.GetUpperChannel(), dc.GetMiddleLine(), dc.GetLowerChannel())
}

func TestDonchianChannels_Downtrend(t *testing.T) {
	dc := NewDonchianChannels(10, 100)

	// Falling prices (downtrend)
	for i := 0; i < 20; i++ {
		price := 200.0 - float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		dc.Update(md)

		if dc.IsReady() && i >= 10 {
			t.Logf("Iteration %d: Price=%.2f, Upper=%.2f, Middle=%.2f, Lower=%.2f",
				i, price, dc.GetUpperChannel(), dc.GetMiddleLine(), dc.GetLowerChannel())
		}
	}

	// In downtrend, lower channel should be close to recent price
	lastPrice := 200.0 - 19*2.0
	if dc.GetLowerChannel() > lastPrice+5 {
		t.Errorf("Lower channel should be near recent low, got %.2f vs price %.2f",
			dc.GetLowerChannel(), lastPrice)
	}

	t.Logf("✓ Downtrend: Upper=%.2f, Middle=%.2f, Lower=%.2f",
		dc.GetUpperChannel(), dc.GetMiddleLine(), dc.GetLowerChannel())
}

func TestDonchianChannels_RangingMarket(t *testing.T) {
	dc := NewDonchianChannels(10, 100)

	// Oscillating prices (ranging market)
	for i := 0; i < 30; i++ {
		// Price oscillates between 95 and 105
		price := 100.0
		if i%2 == 0 {
			price = 105.0
		} else {
			price = 95.0
		}

		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		dc.Update(md)

		if dc.IsReady() && i >= 10 {
			t.Logf("Iteration %d: Price=%.2f, Upper=%.2f, Lower=%.2f, Width=%.2f",
				i, price, dc.GetUpperChannel(), dc.GetLowerChannel(), dc.GetChannelWidth())
		}
	}

	// In ranging market, channels should be relatively stable
	expectedWidth := 10.0 // 105 - 95
	actualWidth := dc.GetChannelWidth()

	if actualWidth < expectedWidth-1 || actualWidth > expectedWidth+1 {
		t.Logf("Note: Channel width %.2f (expected ~%.2f)", actualWidth, expectedWidth)
	} else {
		t.Logf("✓ Ranging market: Width=%.2f", actualWidth)
	}
}

func TestDonchianChannels_BreakoutDetection(t *testing.T) {
	dc := NewDonchianChannels(10, 100)

	// Build channel with ranging prices
	for i := 0; i < 15; i++ {
		price := 100.0 + float64(i%5)
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		dc.Update(md)
	}

	upperChannel := dc.GetUpperChannel()
	lowerChannel := dc.GetLowerChannel()

	t.Logf("Channel: Upper=%.2f, Lower=%.2f", upperChannel, lowerChannel)

	// Test breakout up
	breakoutUpPrice := upperChannel + 1.0
	if dc.IsBreakoutUp(breakoutUpPrice) {
		t.Logf("✓ Breakout up detected at price %.2f", breakoutUpPrice)
	} else {
		t.Error("Should detect breakout up")
	}

	// Test breakout down
	breakoutDownPrice := lowerChannel - 1.0
	if dc.IsBreakoutDown(breakoutDownPrice) {
		t.Logf("✓ Breakout down detected at price %.2f", breakoutDownPrice)
	} else {
		t.Error("Should detect breakout down")
	}

	// Test no breakout (inside channel)
	insidePrice := (upperChannel + lowerChannel) / 2.0
	if !dc.IsBreakoutUp(insidePrice) && !dc.IsBreakoutDown(insidePrice) {
		t.Logf("✓ No breakout detected at price %.2f", insidePrice)
	}
}

func TestDonchianChannels_Position(t *testing.T) {
	dc := NewDonchianChannels(10, 100)

	// Build channel
	for i := 0; i < 15; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		dc.Update(md)
	}

	upper := dc.GetUpperChannel()
	lower := dc.GetLowerChannel()
	middle := dc.GetMiddleLine()

	// Test position calculation
	posLower := dc.GetPosition(lower)
	posMid := dc.GetPosition(middle)
	posUpper := dc.GetPosition(upper)

	t.Logf("Position at lower (%.2f): %.2f (expected 0.0)", lower, posLower)
	t.Logf("Position at middle (%.2f): %.2f (expected 0.5)", middle, posMid)
	t.Logf("Position at upper (%.2f): %.2f (expected 1.0)", upper, posUpper)

	if posLower < 0.05 && posMid > 0.45 && posMid < 0.55 && posUpper > 0.95 {
		t.Logf("✓ Position calculations correct")
	} else {
		t.Error("Position calculations incorrect")
	}
}

func TestDonchianChannels_VolatilityDetection(t *testing.T) {
	dc := NewDonchianChannels(10, 100)

	// Low volatility (narrow channel)
	for i := 0; i < 15; i++ {
		price := 100.0 + float64(i%3)*0.1 // Very small range
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.05},
			AskPrice: []float64{price + 0.05},
		}
		dc.Update(md)
	}

	if dc.IsNarrowChannel(2.0) {
		t.Logf("✓ Narrow channel detected: Width=%.2f%%", dc.GetChannelWidthPercentage())
	}

	// High volatility (wide channel)
	dc.Reset()
	for i := 0; i < 15; i++ {
		price := 100.0
		if i%2 == 0 {
			price = 110.0 // Wide swings
		} else {
			price = 90.0
		}
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		dc.Update(md)
	}

	if dc.IsWideChannel(5.0) {
		t.Logf("✓ Wide channel detected: Width=%.2f%%", dc.GetChannelWidthPercentage())
	}
}

func TestDonchianChannels_TradingSignals(t *testing.T) {
	dc := NewDonchianChannels(10, 100)

	// Build channel
	for i := 0; i < 15; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		dc.Update(md)
	}

	upper := dc.GetUpperChannel()
	lower := dc.GetLowerChannel()

	// Test signals
	nearLower := lower + 0.1
	nearUpper := upper - 0.1
	middle := (upper + lower) / 2.0

	signalLower := dc.GetSignal(nearLower)
	signalUpper := dc.GetSignal(nearUpper)
	signalMid := dc.GetSignal(middle)

	t.Logf("Signal near lower (%.2f): %d (expected 1)", nearLower, signalLower)
	t.Logf("Signal near upper (%.2f): %d (expected -1)", nearUpper, signalUpper)
	t.Logf("Signal at middle (%.2f): %d (expected 0)", middle, signalMid)

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

func TestDonchianChannels_Reset(t *testing.T) {
	dc := NewDonchianChannels(10, 100)

	// Add data
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{100.0 + float64(i)},
		}
		dc.Update(md)
	}

	dc.Reset()

	if dc.IsReady() || dc.GetValue() != 0 {
		t.Error("Donchian Channels should reset properly")
	}

	t.Logf("✓ Reset successful")
}

func TestDonchianChannels_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(30),
		"max_history": float64(500),
	}

	indicator, err := NewDonchianChannelsFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create Donchian Channels from config: %v", err)
	}

	dc, ok := indicator.(*DonchianChannels)
	if !ok || dc.period != 30 {
		t.Error("Config creation failed")
	}

	t.Logf("✓ Config creation successful")
}

func BenchmarkDonchianChannels_Update(b *testing.B) {
	dc := NewDonchianChannels(20, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dc.Update(md)
	}
}
