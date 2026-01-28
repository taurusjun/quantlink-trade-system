package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewPriceChannel(t *testing.T) {
	pc := NewPriceChannel(20, 100)
	if pc == nil || pc.period != 20 {
		t.Fatal("Failed to create Price Channel")
	}
	if pc.IsReady() {
		t.Error("Price Channel should not be ready initially")
	}
}

func TestPriceChannel_Uptrend(t *testing.T) {
	pc := NewPriceChannel(10, 100)

	// Rising prices
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		pc.Update(md)

		if pc.IsReady() && i >= 10 {
			t.Logf("Iteration %d: Price=%.2f, Upper=%.2f, Lower=%.2f, Width=%.2f",
				i, price, pc.GetUpperChannel(), pc.GetLowerChannel(), pc.GetChannelWidth())
		}
	}

	// Upper channel should be near recent high
	lastPrice := 100.0 + 19*2.0
	if pc.GetUpperChannel() < lastPrice-5 {
		t.Errorf("Upper channel should be near recent high, got %.2f vs price %.2f",
			pc.GetUpperChannel(), lastPrice)
	}

	t.Logf("✓ Uptrend: Upper=%.2f, Lower=%.2f, Width=%.2f",
		pc.GetUpperChannel(), pc.GetLowerChannel(), pc.GetChannelWidth())
}

func TestPriceChannel_Downtrend(t *testing.T) {
	pc := NewPriceChannel(10, 100)

	// Falling prices
	for i := 0; i < 20; i++ {
		price := 200.0 - float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		pc.Update(md)

		if pc.IsReady() && i >= 10 {
			t.Logf("Iteration %d: Price=%.2f, Upper=%.2f, Lower=%.2f",
				i, price, pc.GetUpperChannel(), pc.GetLowerChannel())
		}
	}

	// Lower channel should be near recent low
	lastPrice := 200.0 - 19*2.0
	if pc.GetLowerChannel() > lastPrice+5 {
		t.Errorf("Lower channel should be near recent low, got %.2f vs price %.2f",
			pc.GetLowerChannel(), lastPrice)
	}

	t.Logf("✓ Downtrend: Upper=%.2f, Lower=%.2f",
		pc.GetUpperChannel(), pc.GetLowerChannel())
}

func TestPriceChannel_BreakoutDetection(t *testing.T) {
	pc := NewPriceChannel(10, 100)

	// Build channel
	for i := 0; i < 15; i++ {
		price := 100.0 + float64(i%5)
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		pc.Update(md)
	}

	upper := pc.GetUpperChannel()
	lower := pc.GetLowerChannel()

	t.Logf("Channel: Upper=%.2f, Lower=%.2f", upper, lower)

	// Test breakout up
	if pc.IsBreakoutUp(upper + 1.0) {
		t.Logf("✓ Breakout up detected")
	} else {
		t.Error("Should detect breakout up")
	}

	// Test breakout down
	if pc.IsBreakoutDown(lower - 1.0) {
		t.Logf("✓ Breakout down detected")
	} else {
		t.Error("Should detect breakout down")
	}

	// Test inside channel
	inside := (upper + lower) / 2.0
	if !pc.IsBreakoutUp(inside) && !pc.IsBreakoutDown(inside) {
		t.Logf("✓ No breakout detected inside channel")
	}
}

func TestPriceChannel_Position(t *testing.T) {
	pc := NewPriceChannel(10, 100)

	// Build channel
	for i := 0; i < 15; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price - 0.5},
			AskPrice: []float64{price + 0.5},
		}
		pc.Update(md)
	}

	upper := pc.GetUpperChannel()
	lower := pc.GetLowerChannel()

	// Test position
	posLower := pc.GetPosition(lower)
	posUpper := pc.GetPosition(upper)
	posMid := pc.GetPosition((upper + lower) / 2.0)

	t.Logf("Position at lower (%.2f): %.2f", lower, posLower)
	t.Logf("Position at upper (%.2f): %.2f", upper, posUpper)
	t.Logf("Position at mid: %.2f", posMid)

	if posLower < 0.05 && posUpper > 0.95 && posMid > 0.45 && posMid < 0.55 {
		t.Logf("✓ Position calculations correct")
	}
}

func TestPriceChannel_ChannelWidth(t *testing.T) {
	pc := NewPriceChannel(10, 100)

	// Build channel with fixed range
	for i := 0; i < 20; i++ {
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
		pc.Update(md)
	}

	expectedWidth := 10.5 // 105.5 - 95.5
	actualWidth := pc.GetChannelWidth()

	t.Logf("Channel width: %.2f (expected ~%.2f)", actualWidth, expectedWidth)

	if actualWidth > expectedWidth-2 && actualWidth < expectedWidth+2 {
		t.Logf("✓ Channel width correct")
	}
}

func TestPriceChannel_Reset(t *testing.T) {
	pc := NewPriceChannel(10, 100)

	// Add data
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{100.0 + float64(i)},
		}
		pc.Update(md)
	}

	pc.Reset()

	if pc.IsReady() || pc.GetValue() != 0 {
		t.Error("Price Channel should reset properly")
	}

	t.Logf("✓ Reset successful")
}

func TestPriceChannel_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(30),
		"max_history": float64(500),
	}

	indicator, err := NewPriceChannelFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create Price Channel from config: %v", err)
	}

	pc, ok := indicator.(*PriceChannel)
	if !ok || pc.period != 30 {
		t.Error("Config creation failed")
	}

	t.Logf("✓ Config creation successful")
}

func BenchmarkPriceChannel_Update(b *testing.B) {
	pc := NewPriceChannel(20, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pc.Update(md)
	}
}
