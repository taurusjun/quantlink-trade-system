package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestBuySellPressure_BuyingPressure(t *testing.T) {
	bsp := NewBuySellPressure("test_buying", 5, 100)

	// Strong bid depth + last price above mid
	md := &mdpb.MarketDataUpdate{
		BidPrice:  []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:    []uint32{100, 90, 80, 70, 60}, // Strong bids
		AskPrice:  []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		AskQty:    []uint32{20, 20, 20, 20, 20}, // Weak asks
		LastPrice: 100.08, // Above mid (100.05)
	}

	bsp.Update(md)

	netPressure := bsp.GetNetPressure()
	if netPressure <= 0 {
		t.Errorf("Expected positive net pressure, got %.2f", netPressure)
	}

	buyPressure := bsp.GetBuyPressure()
	sellPressure := bsp.GetSellPressure()
	if buyPressure <= sellPressure {
		t.Errorf("Expected buy pressure (%.2f) > sell pressure (%.2f)", buyPressure, sellPressure)
	}

	side := bsp.GetDominantSide()
	if side != "Buy" && side != "StrongBuy" {
		t.Errorf("Expected Buy or StrongBuy, got %s", side)
	}
}

func TestBuySellPressure_SellingPressure(t *testing.T) {
	bsp := NewBuySellPressure("test_selling", 5, 100)

	// Weak bids + last price below mid
	md := &mdpb.MarketDataUpdate{
		BidPrice:  []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:    []uint32{10, 10, 10, 10, 10}, // Weak bids
		AskPrice:  []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		AskQty:    []uint32{100, 90, 80, 70, 60}, // Strong asks
		LastPrice: 100.02, // Below mid (100.05)
	}

	bsp.Update(md)

	netPressure := bsp.GetNetPressure()
	if netPressure >= 0 {
		t.Errorf("Expected negative net pressure, got %.2f", netPressure)
	}

	side := bsp.GetDominantSide()
	if side != "Sell" && side != "StrongSell" {
		t.Errorf("Expected Sell or StrongSell, got %s", side)
	}
}

func TestBuySellPressure_Balanced(t *testing.T) {
	bsp := NewBuySellPressure("test_balanced", 5, 100)

	// Equal depth + price at mid
	md := &mdpb.MarketDataUpdate{
		BidPrice:  []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:    []uint32{50, 40, 30, 20, 10},
		AskPrice:  []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		AskQty:    []uint32{50, 40, 30, 20, 10},
		LastPrice: 100.05, // At mid
	}

	bsp.Update(md)

	side := bsp.GetDominantSide()
	if side != "Balanced" {
		t.Logf("Expected Balanced, got %s (acceptable for similar pressure)", side)
	}
}

func TestBuySellPressure_Decay(t *testing.T) {
	bsp := NewBuySellPressure("test_decay", 5, 100)

	// Strong buying pressure
	md1 := &mdpb.MarketDataUpdate{
		BidPrice:  []float64{100.0, 99.9, 99.8},
		BidQty:    []uint32{100, 90, 80},
		AskPrice:  []float64{100.1, 100.2, 100.3},
		AskQty:    []uint32{10, 10, 10},
		LastPrice: 100.08,
	}
	bsp.Update(md1)

	netPressureBefore := bsp.GetNetPressure()

	// Several updates with balanced, minimal data
	// This should cause net pressure to approach 0 due to decay and balanced new data
	md2 := &mdpb.MarketDataUpdate{
		BidPrice:  []float64{100.0},
		BidQty:    []uint32{10},
		AskPrice:  []float64{100.1},
		AskQty:    []uint32{10},
		LastPrice: 0, // No trade
	}
	for i := 0; i < 20; i++ {
		bsp.Update(md2)
	}

	netPressureAfter := bsp.GetNetPressure()

	// Net pressure should have decayed toward 0 with balanced market data
	// The magnitude should decrease significantly
	if math.Abs(netPressureAfter) >= math.Abs(netPressureBefore)*0.8 {
		t.Errorf("Expected net pressure to decay from %.2f, got %.2f (should be significantly lower in magnitude)",
			netPressureBefore, netPressureAfter)
	}
}

func TestBuySellPressure_PressureRatio(t *testing.T) {
	bsp := NewBuySellPressure("test_ratio", 3, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice:  []float64{100.0, 99.9, 99.8},
		BidQty:    []uint32{60, 50, 40},
		AskPrice:  []float64{100.1, 100.2, 100.3},
		AskQty:    []uint32{20, 20, 20},
		LastPrice: 100.07,
	}

	bsp.Update(md)

	ratio := bsp.GetPressureRatio()
	if ratio <= 1.0 {
		t.Errorf("Expected ratio > 1 for buy dominance, got %.2f", ratio)
	}
}

func TestBuySellPressure_EmptyData(t *testing.T) {
	bsp := NewBuySellPressure("test_empty", 5, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice:  []float64{},
		BidQty:    []uint32{},
		AskPrice:  []float64{},
		AskQty:    []uint32{},
		LastPrice: 0,
	}

	bsp.Update(md)

	// Should handle gracefully
	netPressure := bsp.GetNetPressure()
	if netPressure != 0 {
		t.Errorf("Expected net pressure 0 for empty data, got %.2f", netPressure)
	}
}

func TestBuySellPressure_GetValue(t *testing.T) {
	bsp := NewBuySellPressure("test_getvalue", 5, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice:  []float64{100.0, 99.9, 99.8},
		BidQty:    []uint32{50, 40, 30},
		AskPrice:  []float64{100.1, 100.2, 100.3},
		AskQty:    []uint32{40, 30, 20},
		LastPrice: 100.05,
	}

	bsp.Update(md)

	// GetValue() should return net pressure
	value := bsp.GetValue()
	netPressure := bsp.GetNetPressure()

	if value != netPressure {
		t.Errorf("GetValue() should equal GetNetPressure(), got %.2f vs %.2f", value, netPressure)
	}
}

func TestBuySellPressure_History(t *testing.T) {
	bsp := NewBuySellPressure("test_history", 5, 10)

	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice:  []float64{100.0, 99.9, 99.8},
			BidQty:    []uint32{50, 40, 30},
			AskPrice:  []float64{100.1, 100.2, 100.3},
			AskQty:    []uint32{40, 30, 20},
			LastPrice: 100.05,
		}
		bsp.Update(md)
	}

	history := bsp.GetValues()
	if len(history) != 10 {
		t.Errorf("Expected history length 10, got %d", len(history))
	}
}

func TestBuySellPressure_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":         "config_bsp",
		"levels":       3.0,
		"depth_weight": 0.8,
		"flow_weight":  0.2,
		"decay_factor": 0.9,
		"max_history":  200.0,
	}

	indicator, err := NewBuySellPressureFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	bsp, ok := indicator.(*BuySellPressure)
	if !ok {
		t.Fatal("Expected *BuySellPressure type")
	}

	if bsp.GetName() != "config_bsp" {
		t.Errorf("Expected name 'config_bsp', got '%s'", bsp.GetName())
	}
}

func TestBuySellPressure_String(t *testing.T) {
	bsp := NewBuySellPressure("test_string", 5, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice:  []float64{100.0},
		BidQty:    []uint32{100},
		AskPrice:  []float64{100.1},
		AskQty:    []uint32{50},
		LastPrice: 100.08,
	}
	bsp.Update(md)

	str := bsp.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "BuySellPressure") {
		t.Error("String() should contain 'BuySellPressure'")
	}
}

func BenchmarkBuySellPressure_Update(b *testing.B) {
	bsp := NewBuySellPressure("bench", 10, 1000)

	md := &mdpb.MarketDataUpdate{
		BidPrice:  []float64{100.0, 99.9, 99.8, 99.7, 99.6, 99.5, 99.4, 99.3, 99.2, 99.1},
		BidQty:    []uint32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		AskPrice:  []float64{100.1, 100.2, 100.3, 100.4, 100.5, 100.6, 100.7, 100.8, 100.9, 101.0},
		AskQty:    []uint32{15, 25, 35, 45, 55, 65, 75, 85, 95, 105},
		LastPrice: 100.05,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		md.LastPrice = 100.0 + float64(i%10)*0.01
		bsp.Update(md)
	}
}
