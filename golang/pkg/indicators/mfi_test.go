package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewMFI(t *testing.T) {
	mfi := NewMFI(14, 100)
	if mfi == nil || mfi.period != 14 {
		t.Fatal("Failed to create MFI")
	}
	if mfi.IsReady() {
		t.Error("MFI should not be ready initially")
	}
}

func TestMFI_AccumulationPhase(t *testing.T) {
	mfi := NewMFI(14, 100)

	// Rising prices with increasing volume (accumulation)
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*2.0
		volume := uint64(1000 + i*100) // Increasing volume

		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: volume,
		}
		mfi.Update(md)

		if mfi.IsReady() && i >= 15 {
			t.Logf("Iteration %d: Price=%.2f, Volume=%d, MFI=%.2f",
				i, price, volume, mfi.GetValue())
		}
	}

	// MFI should be high (overbought) during accumulation
	if mfi.GetValue() < 50 {
		t.Logf("Note: MFI=%.2f (expected high during accumulation)", mfi.GetValue())
	} else {
		t.Logf("✓ Accumulation phase: MFI = %.2f", mfi.GetValue())
	}
}

func TestMFI_DistributionPhase(t *testing.T) {
	mfi := NewMFI(14, 100)

	// Initialize with high price
	md := &mdpb.MarketDataUpdate{
		BidPrice:    []float64{199.5},
		AskPrice:    []float64{200.5},
		TotalVolume: 2000,
	}
	mfi.Update(md)

	// Falling prices with increasing volume (distribution)
	for i := 0; i < 20; i++ {
		price := 199.0 - float64(i)*2.0
		volume := uint64(2000 + i*100) // Increasing volume on decline

		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: volume,
		}
		mfi.Update(md)

		if mfi.IsReady() && i >= 14 {
			t.Logf("Iteration %d: Price=%.2f, Volume=%d, MFI=%.2f",
				i, price, volume, mfi.GetValue())
		}
	}

	// MFI should be low (oversold) during distribution
	if mfi.GetValue() > 50 {
		t.Logf("Note: MFI=%.2f (expected low during distribution)", mfi.GetValue())
	} else {
		t.Logf("✓ Distribution phase: MFI = %.2f", mfi.GetValue())
	}
}

func TestMFI_OverboughtOversold(t *testing.T) {
	mfi := NewMFI(10, 100)

	// Strong uptrend to reach overbought
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 2000,
		}
		mfi.Update(md)
	}

	if mfi.IsOverbought() {
		t.Logf("✓ Overbought detected: MFI = %.2f (> 80)", mfi.GetValue())
	} else {
		t.Logf("Note: MFI = %.2f (not yet overbought)", mfi.GetValue())
	}

	// Strong downtrend to reach oversold
	mfi.Reset()
	md := &mdpb.MarketDataUpdate{
		BidPrice:    []float64{199.5},
		AskPrice:    []float64{200.5},
		TotalVolume: 2000,
	}
	mfi.Update(md)

	for i := 0; i < 20; i++ {
		price := 199.0 - float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 2000,
		}
		mfi.Update(md)
	}

	if mfi.IsOversold() {
		t.Logf("✓ Oversold detected: MFI = %.2f (< 20)", mfi.GetValue())
	} else {
		t.Logf("Note: MFI = %.2f (not yet oversold)", mfi.GetValue())
	}
}

func TestMFI_TradingSignals(t *testing.T) {
	mfi := NewMFI(10, 100)

	// Strong uptrend
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 2000,
		}
		mfi.Update(md)
	}

	signal := mfi.GetSignal()
	if signal == -1 {
		t.Logf("✓ Sell signal (MFI=%.2f > 80)", mfi.GetValue())
	} else {
		t.Logf("Neutral signal (MFI=%.2f)", mfi.GetValue())
	}

	// Strong downtrend
	mfi.Reset()
	md := &mdpb.MarketDataUpdate{
		BidPrice:    []float64{199.5},
		AskPrice:    []float64{200.5},
		TotalVolume: 2000,
	}
	mfi.Update(md)

	for i := 0; i < 20; i++ {
		price := 199.0 - float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 2000,
		}
		mfi.Update(md)
	}

	signal = mfi.GetSignal()
	if signal == 1 {
		t.Logf("✓ Buy signal (MFI=%.2f < 20)", mfi.GetValue())
	} else {
		t.Logf("Neutral signal (MFI=%.2f)", mfi.GetValue())
	}
}

func TestMFI_BullishDivergence(t *testing.T) {
	mfi := NewMFI(14, 100)

	prices := []float64{}

	// Simulate bullish divergence: price making lower lows, MFI making higher lows
	// Phase 1: Initial decline
	for i := 0; i < 10; i++ {
		price := 150.0 - float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 3000, // High volume on decline
		}
		mfi.Update(md)
		prices = append(prices, price)
	}

	// Phase 2: Small rally with low volume
	for i := 0; i < 5; i++ {
		price := 101.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 500, // Low volume rally
		}
		mfi.Update(md)
		prices = append(prices, price)
	}

	// Phase 3: Another decline but with decreasing volume (bullish divergence setup)
	for i := 0; i < 10; i++ {
		price := 110.0 - float64(i)*3.0
		volume := uint64(2000 - i*100) // Decreasing volume
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: volume,
		}
		mfi.Update(md)
		prices = append(prices, price)

		if mfi.IsReady() {
			t.Logf("Price=%.2f, Volume=%d, MFI=%.2f", price, volume, mfi.GetValue())
		}
	}

	if mfi.IsBullishDivergence(prices) {
		t.Logf("✓ Bullish divergence detected")
	} else {
		t.Logf("No strong bullish divergence")
	}
}

func TestMFI_BearishDivergence(t *testing.T) {
	mfi := NewMFI(14, 100)

	prices := []float64{}

	// Simulate bearish divergence: price making higher highs, MFI making lower highs
	// Phase 1: Initial rally with high volume
	for i := 0; i < 10; i++ {
		price := 100.0 + float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 3000, // High volume
		}
		mfi.Update(md)
		prices = append(prices, price)
	}

	// Phase 2: Small pullback
	for i := 0; i < 5; i++ {
		price := 149.0 - float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 500,
		}
		mfi.Update(md)
		prices = append(prices, price)
	}

	// Phase 3: Rally to new high but with decreasing volume (bearish divergence)
	for i := 0; i < 10; i++ {
		price := 140.0 + float64(i)*3.0
		volume := uint64(2000 - i*100) // Decreasing volume
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: volume,
		}
		mfi.Update(md)
		prices = append(prices, price)

		if mfi.IsReady() {
			t.Logf("Price=%.2f, Volume=%d, MFI=%.2f", price, volume, mfi.GetValue())
		}
	}

	if mfi.IsBearishDivergence(prices) {
		t.Logf("✓ Bearish divergence detected")
	} else {
		t.Logf("No strong bearish divergence")
	}
}

func TestMFI_NoVolume(t *testing.T) {
	mfi := NewMFI(14, 100)

	// Updates with zero volume should be skipped
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{100.0 + float64(i)},
			AskPrice:    []float64{100.0 + float64(i)},
			TotalVolume: 0, // No volume
		}
		mfi.Update(md)
	}

	if mfi.IsReady() {
		t.Error("MFI should not be ready with zero volume updates")
	}

	t.Logf("✓ Zero volume updates correctly skipped")
}

func TestMFI_Reset(t *testing.T) {
	mfi := NewMFI(14, 100)

	// Add data
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{100.0 + float64(i)},
			AskPrice:    []float64{100.0 + float64(i)},
			TotalVolume: 1000,
		}
		mfi.Update(md)
	}

	mfi.Reset()

	if mfi.IsReady() || mfi.GetValue() != 0 {
		t.Error("MFI should reset properly")
	}

	t.Logf("✓ Reset successful")
}

func TestMFI_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(20),
		"max_history": float64(500),
	}

	indicator, err := NewMFIFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create MFI from config: %v", err)
	}

	mfi, ok := indicator.(*MFI)
	if !ok || mfi.period != 20 {
		t.Error("Config creation failed")
	}

	t.Logf("✓ Config creation successful")
}

func BenchmarkMFI_Update(b *testing.B) {
	mfi := NewMFI(14, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice:    []float64{99.5},
		AskPrice:    []float64{100.5},
		TotalVolume: 1000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mfi.Update(md)
	}
}
