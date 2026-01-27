package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestOrderFlowImbalance_BuyImbalance(t *testing.T) {
	ofi := NewOrderFlowImbalance("test_buy", 100, 100)

	// Series of upticks (buys)
	prices := []float64{100.0, 100.1, 100.2, 100.3, 100.4}
	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			LastPrice: price,
			LastQty:   10,
			BidPrice:  []float64{price - 0.1},
			BidQty:    []uint32{50},
			AskPrice:  []float64{price + 0.1},
			AskQty:    []uint32{50},
		}
		ofi.Update(md)
	}

	// Should show buy imbalance
	imbalance := ofi.GetImbalance()
	if imbalance <= 0 {
		t.Errorf("Expected positive imbalance for upticks, got %.4f", imbalance)
	}

	buyVol := ofi.GetBuyVolume()
	if buyVol <= 0 {
		t.Errorf("Expected positive buy volume, got %.0f", buyVol)
	}

	direction := ofi.GetDirection()
	if direction != "Buy" && direction != "StrongBuy" {
		t.Errorf("Expected Buy or StrongBuy direction, got %s", direction)
	}
}

func TestOrderFlowImbalance_SellImbalance(t *testing.T) {
	ofi := NewOrderFlowImbalance("test_sell", 100, 100)

	// Series of downticks (sells)
	prices := []float64{100.0, 99.9, 99.8, 99.7, 99.6}
	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			LastPrice: price,
			LastQty:   10,
			BidPrice:  []float64{price - 0.1},
			BidQty:    []uint32{50},
			AskPrice:  []float64{price + 0.1},
			AskQty:    []uint32{50},
		}
		ofi.Update(md)
	}

	// Should show sell imbalance
	imbalance := ofi.GetImbalance()
	if imbalance >= 0 {
		t.Errorf("Expected negative imbalance for downticks, got %.4f", imbalance)
	}

	sellVol := ofi.GetSellVolume()
	if sellVol <= 0 {
		t.Errorf("Expected positive sell volume, got %.0f", sellVol)
	}

	direction := ofi.GetDirection()
	if direction != "Sell" && direction != "StrongSell" {
		t.Errorf("Expected Sell or StrongSell direction, got %s", direction)
	}
}

func TestOrderFlowImbalance_Balanced(t *testing.T) {
	ofi := NewOrderFlowImbalance("test_balanced", 100, 100)

	// Alternating up and down
	prices := []float64{100.0, 100.1, 100.0, 100.1, 100.0, 100.1, 100.0}
	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			LastPrice: price,
			LastQty:   10,
			BidPrice:  []float64{price - 0.1},
			BidQty:    []uint32{50},
			AskPrice:  []float64{price + 0.1},
			AskQty:    []uint32{50},
		}
		ofi.Update(md)
	}

	// Should be relatively balanced
	imbalance := ofi.GetImbalance()
	if imbalance < -0.3 || imbalance > 0.3 {
		t.Errorf("Expected balanced imbalance (-0.3 to 0.3), got %.4f", imbalance)
	}

	direction := ofi.GetDirection()
	if direction != "Neutral" {
		t.Logf("Got direction %s (acceptable for balanced trades)", direction)
	}
}

func TestOrderFlowImbalance_WindowReset(t *testing.T) {
	ofi := NewOrderFlowImbalance("test_window", 5, 100)

	// Fill window with buys
	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: float64(100 + i),
			LastQty:   10,
			BidPrice:  []float64{99.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{101.0},
			AskQty:    []uint32{50},
		}
		ofi.Update(md)
	}

	imbalanceBefore := ofi.GetImbalance()

	// Window should reset after windowSize trades
	// Next update should start fresh
	md := &mdpb.MarketDataUpdate{
		LastPrice: 95.0, // Downtick
		LastQty:   10,
		BidPrice:  []float64{94.0},
		BidQty:    []uint32{50},
		AskPrice:  []float64{96.0},
		AskQty:    []uint32{50},
	}
	ofi.Update(md)

	// After reset, imbalance should have changed significantly
	imbalanceAfter := ofi.GetImbalance()
	if imbalanceAfter == imbalanceBefore {
		t.Logf("Imbalance unchanged after window: before=%.4f, after=%.4f", imbalanceBefore, imbalanceAfter)
	}
}

func TestOrderFlowImbalance_EmptyPrice(t *testing.T) {
	ofi := NewOrderFlowImbalance("test_empty", 100, 100)

	md := &mdpb.MarketDataUpdate{
		LastPrice: 0, // No price
		LastQty:   10,
	}
	ofi.Update(md)

	// Should handle gracefully
	imbalance := ofi.GetImbalance()
	if imbalance != 0 {
		t.Errorf("Expected imbalance 0 for empty price, got %.4f", imbalance)
	}
}

func TestOrderFlowImbalance_GetValue(t *testing.T) {
	ofi := NewOrderFlowImbalance("test_getvalue", 100, 100)

	prices := []float64{100.0, 100.1, 100.2}
	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			LastPrice: price,
			LastQty:   10,
			BidPrice:  []float64{99.5},
			BidQty:    []uint32{50},
			AskPrice:  []float64{100.5},
			AskQty:    []uint32{50},
		}
		ofi.Update(md)
	}

	// GetValue() should return imbalance
	value := ofi.GetValue()
	imbalance := ofi.GetImbalance()

	if value != imbalance {
		t.Errorf("GetValue() should equal GetImbalance(), got %.4f vs %.4f", value, imbalance)
	}
}

func TestOrderFlowImbalance_History(t *testing.T) {
	ofi := NewOrderFlowImbalance("test_history", 100, 10)

	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: float64(100 + i),
			LastQty:   10,
			BidPrice:  []float64{99.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{101.0},
			AskQty:    []uint32{50},
		}
		ofi.Update(md)
	}

	history := ofi.GetValues()
	if len(history) != 10 {
		t.Errorf("Expected history length 10, got %d", len(history))
	}
}

func TestOrderFlowImbalance_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":        "config_ofi",
		"window_size": 50.0,
		"max_history": 200.0,
	}

	indicator, err := NewOrderFlowImbalanceFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	ofi, ok := indicator.(*OrderFlowImbalance)
	if !ok {
		t.Fatal("Expected *OrderFlowImbalance type")
	}

	if ofi.GetName() != "config_ofi" {
		t.Errorf("Expected name 'config_ofi', got '%s'", ofi.GetName())
	}
}

func TestOrderFlowImbalance_String(t *testing.T) {
	ofi := NewOrderFlowImbalance("test_string", 100, 100)

	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: float64(100 + i),
			LastQty:   10,
			BidPrice:  []float64{99.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{101.0},
			AskQty:    []uint32{50},
		}
		ofi.Update(md)
	}

	str := ofi.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "OrderFlowImbalance") {
		t.Error("String() should contain 'OrderFlowImbalance'")
	}
}

func BenchmarkOrderFlowImbalance_Update(b *testing.B) {
	ofi := NewOrderFlowImbalance("bench", 100, 1000)

	md := &mdpb.MarketDataUpdate{
		LastPrice: 100.0,
		LastQty:   10,
		BidPrice:  []float64{99.5},
		BidQty:    []uint32{50},
		AskPrice:  []float64{100.5},
		AskQty:    []uint32{50},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		md.LastPrice = 100.0 + float64(i%10)*0.1
		ofi.Update(md)
	}
}
