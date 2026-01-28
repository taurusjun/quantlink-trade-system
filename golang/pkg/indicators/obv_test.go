package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNewOBV(t *testing.T) {
	obv := NewOBV(100)
	if obv == nil {
		t.Fatal("Failed to create OBV")
	}
	if obv.IsReady() {
		t.Error("OBV should not be ready initially")
	}
}

func TestOBV_PriceUpAddsVolume(t *testing.T) {
	obv := NewOBV(100)

	// Price rising should add volume
	prices := []float64{100.0, 101.0, 102.0, 103.0}
	volumes := []uint64{1000, 1500, 2000, 2500}

	for i, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: volumes[i],
		}
		obv.Update(md)

		if i > 0 && obv.IsReady() {
			t.Logf("Price %.2f, Volume %d, OBV %.2f", price, volumes[i], obv.GetValue())
		}
	}

	// OBV should be positive (accumulation)
	if obv.GetValue() <= 0 {
		t.Errorf("Expected positive OBV for rising prices, got %.2f", obv.GetValue())
	}

	t.Logf("✓ Rising prices: OBV = %.2f (accumulation)", obv.GetValue())
}

func TestOBV_PriceDownSubtractsVolume(t *testing.T) {
	obv := NewOBV(100)

	// Initialize with high price
	md := &mdpb.MarketDataUpdate{
		BidPrice:    []float64{199.5},
		AskPrice:    []float64{200.5},
		TotalVolume: 1000,
	}
	obv.Update(md)

	// Price falling should subtract volume
	prices := []float64{199.0, 198.0, 197.0, 196.0}
	volumes := []uint64{1500, 2000, 2500, 3000}

	for i, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: volumes[i],
		}
		obv.Update(md)

		if obv.IsReady() {
			t.Logf("Price %.2f, Volume %d, OBV %.2f", price, volumes[i], obv.GetValue())
		}
	}

	// OBV should be negative (distribution)
	if obv.GetValue() >= 0 {
		t.Errorf("Expected negative OBV for falling prices, got %.2f", obv.GetValue())
	}

	t.Logf("✓ Falling prices: OBV = %.2f (distribution)", obv.GetValue())
}

func TestOBV_PriceUnchangedKeepsOBV(t *testing.T) {
	obv := NewOBV(100)

	// Initialize
	md := &mdpb.MarketDataUpdate{
		BidPrice:    []float64{99.5},
		AskPrice:    []float64{100.5},
		TotalVolume: 1000,
	}
	obv.Update(md)

	initialOBV := obv.GetValue()

	// Same price - OBV should not change
	md = &mdpb.MarketDataUpdate{
		BidPrice:    []float64{99.5},
		AskPrice:    []float64{100.5},
		TotalVolume: 5000, // Volume doesn't matter
	}
	obv.Update(md)

	if obv.GetValue() != initialOBV {
		t.Errorf("OBV should remain unchanged when price is unchanged, got %.2f != %.2f",
			obv.GetValue(), initialOBV)
	}

	t.Logf("✓ Unchanged price: OBV remains at %.2f", obv.GetValue())
}

func TestOBV_TrendDetection(t *testing.T) {
	obv := NewOBV(100)

	// Rising prices -> uptrend
	for i := 0; i < 5; i++ {
		price := 100.0 + float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 1000,
		}
		obv.Update(md)
	}

	trend := obv.GetTrend()
	if trend != 1 {
		t.Errorf("Expected uptrend (1), got %d", trend)
	}
	t.Logf("✓ Uptrend detected: trend = %d", trend)

	// Falling prices -> downtrend
	obv.Reset()
	md := &mdpb.MarketDataUpdate{
		BidPrice:    []float64{199.5},
		AskPrice:    []float64{200.5},
		TotalVolume: 1000,
	}
	obv.Update(md)

	for i := 0; i < 5; i++ {
		price := 199.0 - float64(i)*2.0
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: 1000,
		}
		obv.Update(md)
	}

	trend = obv.GetTrend()
	if trend != -1 {
		t.Errorf("Expected downtrend (-1), got %d", trend)
	}
	t.Logf("✓ Downtrend detected: trend = %d", trend)
}

func TestOBV_BullishDivergence(t *testing.T) {
	obv := NewOBV(100)

	// Simulate bullish divergence: price falling, but OBV rising
	prices := []float64{105.0, 104.0, 103.0, 102.0, 101.0, 100.0, 99.0, 98.0}
	volumes := []uint64{1000, 500, 200, 100, 1500, 2000, 2500, 3000} // Low volume on downs, high on ups

	priceHistory := []float64{}

	for i, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: volumes[i],
		}
		obv.Update(md)
		priceHistory = append(priceHistory, price)

		if obv.IsReady() {
			t.Logf("Price %.2f, Volume %d, OBV %.2f", price, volumes[i], obv.GetValue())
		}
	}

	// Check for bullish divergence
	divergence := obv.IsDivergence(priceHistory)
	if divergence == 1 {
		t.Logf("✓ Bullish divergence detected (price falling, OBV rising)")
	} else {
		t.Logf("No strong bullish divergence detected (divergence = %d)", divergence)
	}
}

func TestOBV_BearishDivergence(t *testing.T) {
	obv := NewOBV(100)

	// Simulate bearish divergence: price rising, but OBV falling
	prices := []float64{95.0, 96.0, 97.0, 98.0, 99.0, 100.0, 101.0, 102.0}
	volumes := []uint64{3000, 2500, 2000, 1500, 1000, 500, 200, 100} // High volume on early ups, low on later

	priceHistory := []float64{}

	for i, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			TotalVolume: volumes[i],
		}
		obv.Update(md)
		priceHistory = append(priceHistory, price)

		if obv.IsReady() {
			t.Logf("Price %.2f, Volume %d, OBV %.2f", price, volumes[i], obv.GetValue())
		}
	}

	// Check for bearish divergence
	divergence := obv.IsDivergence(priceHistory)
	if divergence == -1 {
		t.Logf("✓ Bearish divergence detected (price rising, OBV falling)")
	} else {
		t.Logf("No strong bearish divergence detected (divergence = %d)", divergence)
	}
}

func TestOBV_VolumeFromBidAsk(t *testing.T) {
	obv := NewOBV(100)

	// Test with TotalVolume = 0, use bid/ask volume instead
	prices := []float64{100.0, 101.0, 102.0}
	bidQtys := []uint32{500, 600, 700}
	askQtys := []uint32{500, 400, 300}

	for i, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{price - 0.5},
			AskPrice:    []float64{price + 0.5},
			BidQty:      []uint32{bidQtys[i]},
			AskQty:      []uint32{askQtys[i]},
			TotalVolume: 0, // No TotalVolume, fallback to bid+ask
		}
		obv.Update(md)

		if i > 0 && obv.IsReady() {
			t.Logf("Price %.2f, BidQty %d, AskQty %d, OBV %.2f",
				price, bidQtys[i], askQtys[i], obv.GetValue())
		}
	}

	// OBV should be positive (accumulation)
	if obv.GetValue() <= 0 {
		t.Errorf("Expected positive OBV, got %.2f", obv.GetValue())
	}

	t.Logf("✓ Volume from bid/ask: OBV = %.2f", obv.GetValue())
}

func TestOBV_Reset(t *testing.T) {
	obv := NewOBV(100)

	// Add some data
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice:    []float64{100.0 + float64(i)},
			AskPrice:    []float64{100.0 + float64(i)},
			TotalVolume: 1000,
		}
		obv.Update(md)
	}

	obv.Reset()

	if obv.IsReady() || obv.GetValue() != 0 {
		t.Error("OBV should reset properly")
	}

	t.Logf("✓ Reset successful")
}

func TestOBV_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"max_history": float64(500),
	}

	indicator, err := NewOBVFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create OBV from config: %v", err)
	}

	obv, ok := indicator.(*OBV)
	if !ok {
		t.Error("Config creation failed")
	}

	if obv.maxHistory != 500 {
		t.Errorf("Expected maxHistory 500, got %d", obv.maxHistory)
	}

	t.Logf("✓ Config creation successful")
}

func BenchmarkOBV_Update(b *testing.B) {
	obv := NewOBV(1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice:    []float64{99.5},
		AskPrice:    []float64{100.5},
		TotalVolume: 1000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		obv.Update(md)
	}
}
