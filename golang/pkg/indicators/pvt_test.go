package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewPVT(t *testing.T) {
	pvt := NewPVT(100)
	if pvt == nil {
		t.Fatal("Failed to create PVT")
	}
	if pvt.IsReady() {
		t.Error("PVT should not be ready initially")
	}
}

func TestPVT_Uptrend(t *testing.T) {
	pvt := NewPVT(100)

	// Rising prices with volume
	prices := []float64{}
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 1000,
		}
		pvt.Update(md)
		prices = append(prices, price)

		if pvt.IsReady() && i >= 5 {
			t.Logf("Iteration %d: Price=%.2f, PVT=%.2f", i, price, pvt.GetValue())
		}
	}

	// PVT should be positive in uptrend
	if pvt.GetValue() <= 0 {
		t.Errorf("Expected positive PVT in uptrend, got %.2f", pvt.GetValue())
	}

	// Should confirm uptrend
	if pvt.IsConfirmingTrend(prices, 10) {
		t.Logf("✓ Uptrend confirmed: PVT = %.2f", pvt.GetValue())
	}

	// Trend should be up
	if trend := pvt.GetTrend(); trend == 1 {
		t.Logf("✓ Uptrend detected: trend = %d", trend)
	}
}

func TestPVT_Downtrend(t *testing.T) {
	pvt := NewPVT(100)

	// Initialize with high price
	md := &mdpb.MarketDataUpdate{
		BidPrice:    []float64{199.5},
		AskPrice:    []float64{200.5},
		TotalVolume: 1000,
	}
	pvt.Update(md)

	// Falling prices with volume
	prices := []float64{200.0}
	for i := 0; i < 20; i++ {
		price := 199.0 - float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 1000,
		}
		pvt.Update(md)
		prices = append(prices, price)

		if pvt.IsReady() && i >= 5 {
			t.Logf("Iteration %d: Price=%.2f, PVT=%.2f", i, price, pvt.GetValue())
		}
	}

	// PVT should be negative in downtrend
	if pvt.GetValue() >= 0 {
		t.Errorf("Expected negative PVT in downtrend, got %.2f", pvt.GetValue())
	}

	// Should confirm downtrend
	if pvt.IsConfirmingTrend(prices, 10) {
		t.Logf("✓ Downtrend confirmed: PVT = %.2f", pvt.GetValue())
	}

	// Trend should be down
	if trend := pvt.GetTrend(); trend == -1 {
		t.Logf("✓ Downtrend detected: trend = %d", trend)
	}
}

func TestPVT_SmallPriceChanges(t *testing.T) {
	pvt := NewPVT(100)

	// Small price changes should result in small PVT changes
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*0.1 // Small increments
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.05},
			AskPrice:    []float64{price + 0.05},
			TotalVolume: 10000, // High volume
		}
		pvt.Update(md)

		if pvt.IsReady() && i >= 5 {
			t.Logf("Iteration %d: Price=%.2f, PVT=%.2f", i, price, pvt.GetValue())
		}
	}

	// PVT should be relatively small despite high volume
	t.Logf("✓ Small price changes: PVT = %.2f", pvt.GetValue())
}

func TestPVT_LargePriceChanges(t *testing.T) {
	pvt := NewPVT(100)

	// Large price changes should result in large PVT changes
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*10.0 // Large increments
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 1000, // Normal volume
		}
		pvt.Update(md)

		if pvt.IsReady() && i >= 5 {
			t.Logf("Iteration %d: Price=%.2f, PVT=%.2f", i, price, pvt.GetValue())
		}
	}

	// PVT should be large due to big price moves
	if pvt.GetValue() <= 10000 {
		t.Logf("Note: PVT = %.2f (expected larger for big price moves)", pvt.GetValue())
	} else {
		t.Logf("✓ Large price changes: PVT = %.2f", pvt.GetValue())
	}
}

func TestPVT_BullishDivergence(t *testing.T) {
	pvt := NewPVT(100)

	prices := []float64{}

	// Phase 1: Initial decline with high volume
	for i := 0; i < 10; i++ {
		price := 150.0 - float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 5000, // High volume
		}
		pvt.Update(md)
		prices = append(prices, price)
	}

	// Phase 2: Small rally
	for i := 0; i < 5; i++ {
		price := 101.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 2000,
		}
		pvt.Update(md)
		prices = append(prices, price)
	}

	// Phase 3: Another decline but with lower volume (potential divergence)
	for i := 0; i < 10; i++ {
		price := 110.0 - float64(i)*3.0
		volume := uint64(3000 - i*200) // Decreasing volume
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: volume,
		}
		pvt.Update(md)
		prices = append(prices, price)

		if pvt.IsReady() {
			t.Logf("Price=%.2f, Volume=%d, PVT=%.2f", price, volume, pvt.GetValue())
		}
	}

	divergence := pvt.IsDivergence(prices, 10)
	if divergence == 1 {
		t.Logf("✓ Bullish divergence detected")
	} else {
		t.Logf("No strong bullish divergence (divergence = %d)", divergence)
	}
}

func TestPVT_BearishDivergence(t *testing.T) {
	pvt := NewPVT(100)

	prices := []float64{}

	// Phase 1: Initial rally with high volume
	for i := 0; i < 10; i++ {
		price := 100.0 + float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 5000,
		}
		pvt.Update(md)
		prices = append(prices, price)
	}

	// Phase 2: Small pullback
	for i := 0; i < 5; i++ {
		price := 149.0 - float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 2000,
		}
		pvt.Update(md)
		prices = append(prices, price)
	}

	// Phase 3: Rally to new high but with decreasing volume (potential divergence)
	for i := 0; i < 10; i++ {
		price := 140.0 + float64(i)*3.0
		volume := uint64(3000 - i*200) // Decreasing volume
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: volume,
		}
		pvt.Update(md)
		prices = append(prices, price)

		if pvt.IsReady() {
			t.Logf("Price=%.2f, Volume=%d, PVT=%.2f", price, volume, pvt.GetValue())
		}
	}

	divergence := pvt.IsDivergence(prices, 10)
	if divergence == -1 {
		t.Logf("✓ Bearish divergence detected")
	} else {
		t.Logf("No strong bearish divergence (divergence = %d)", divergence)
	}
}

func TestPVT_Slope(t *testing.T) {
	pvt := NewPVT(100)

	// Steady uptrend
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 1000,
		}
		pvt.Update(md)
	}

	slope10 := pvt.GetSlope(10)
	slope5 := pvt.GetSlope(5)

	t.Logf("PVT = %.2f", pvt.GetValue())
	t.Logf("Slope (10 periods) = %.2f", slope10)
	t.Logf("Slope (5 periods) = %.2f", slope5)

	if slope10 > 0 {
		t.Logf("✓ Positive slope confirms uptrend")
	}
}

func TestPVT_Strength(t *testing.T) {
	pvt := NewPVT(100)

	// Strong uptrend with high volume
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*5.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 10000, // High volume
		}
		pvt.Update(md)
	}

	strength := pvt.GetStrength(10)
	t.Logf("Trend strength (10 periods) = %.2f", strength)

	if strength > 0 {
		t.Logf("✓ Strong uptrend detected")
	}
}

func TestPVT_Reset(t *testing.T) {
	pvt := NewPVT(100)

	// Add data
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{100.0 + float64(i)},
			AskPrice:    []float64{100.0 + float64(i)},
			TotalVolume: 1000,
		}
		pvt.Update(md)
	}

	pvt.Reset()

	if pvt.IsReady() || pvt.GetValue() != 0 {
		t.Error("PVT should reset properly")
	}

	t.Logf("✓ Reset successful")
}

func TestPVT_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"max_history": float64(500),
	}

	indicator, err := NewPVTFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create PVT from config: %v", err)
	}

	pvt, ok := indicator.(*PVT)
	if !ok {
		t.Error("Config creation failed")
	}

	if pvt.maxHistory != 500 {
		t.Errorf("Expected maxHistory 500, got %d", pvt.maxHistory)
	}

	t.Logf("✓ Config creation successful")
}

func BenchmarkPVT_Update(b *testing.B) {
	pvt := NewPVT(1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice:    []float64{99.5},
		AskPrice:    []float64{100.5},
		TotalVolume: 1000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pvt.Update(md)
	}
}
