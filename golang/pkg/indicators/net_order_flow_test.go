package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestNetOrderFlow_CumulativeBuying(t *testing.T) {
	nof := NewNetOrderFlow("test_buying", false, 100)

	// Series of upticks - need more to reach threshold (>10 for "Buy")
	prices := []float64{100.0, 100.1, 100.2, 100.3, 100.4, 100.5, 100.6, 100.7, 100.8, 100.9, 101.0, 101.1, 101.2}
	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			LastPrice: price,
			BidPrice:  []float64{price - 0.1},
			BidQty:    []uint32{50},
			AskPrice:  []float64{price + 0.1},
			AskQty:    []uint32{50},
		}
		nof.Update(md)
	}

	// Cumulative flow should be positive (12 upticks with volume 1.0 each = 12)
	flow := nof.GetCumulativeFlow()
	if flow <= 0 {
		t.Errorf("Expected positive cumulative flow, got %.2f", flow)
	}

	buyFlow := nof.GetBuyFlow()
	if buyFlow <= 0 {
		t.Errorf("Expected positive buy flow, got %.2f", buyFlow)
	}

	// With flow >= 10, should be "Buy" or "StrongBuy"
	direction := nof.GetFlowDirection()
	if direction != "Buy" && direction != "StrongBuy" {
		t.Errorf("Expected Buy or StrongBuy (flow=%.2f), got %s", flow, direction)
	}
}

func TestNetOrderFlow_CumulativeSelling(t *testing.T) {
	nof := NewNetOrderFlow("test_selling", false, 100)

	// Series of downticks - need more to reach threshold (<-10 for "Sell")
	prices := []float64{100.0, 99.9, 99.8, 99.7, 99.6, 99.5, 99.4, 99.3, 99.2, 99.1, 99.0, 98.9, 98.8}
	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			LastPrice: price,
			BidPrice:  []float64{price - 0.1},
			BidQty:    []uint32{50},
			AskPrice:  []float64{price + 0.1},
			AskQty:    []uint32{50},
		}
		nof.Update(md)
	}

	// Cumulative flow should be negative (12 downticks with volume 1.0 each = -12)
	flow := nof.GetCumulativeFlow()
	if flow >= 0 {
		t.Errorf("Expected negative cumulative flow, got %.2f", flow)
	}

	sellFlow := nof.GetSellFlow()
	if sellFlow <= 0 {
		t.Errorf("Expected positive sell flow, got %.2f", sellFlow)
	}

	// With flow <= -10, should be "Sell" or "StrongSell"
	direction := nof.GetFlowDirection()
	if direction != "Sell" && direction != "StrongSell" {
		t.Errorf("Expected Sell or StrongSell (flow=%.2f), got %s", flow, direction)
	}
}

func TestNetOrderFlow_ResetOnReverse(t *testing.T) {
	nof := NewNetOrderFlow("test_reset", true, 100)

	// Build up buy flow
	prices := []float64{100.0, 100.1, 100.2, 100.3}
	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			LastPrice: price,
			BidPrice:  []float64{price - 0.1},
			BidQty:    []uint32{50},
			AskPrice:  []float64{price + 0.1},
			AskQty:    []uint32{50},
		}
		nof.Update(md)
	}

	flowBefore := nof.GetCumulativeFlow()
	if flowBefore <= 0 {
		t.Errorf("Expected positive flow before reverse, got %.2f", flowBefore)
	}

	// Reverse to sell (downtick)
	md := &mdpb.MarketDataUpdate{
		LastPrice: 100.2,
		BidPrice:  []float64{100.1},
		BidQty:    []uint32{50},
		AskPrice:  []float64{100.3},
		AskQty:    []uint32{50},
	}
	nof.Update(md)

	flowAfter := nof.GetCumulativeFlow()
	// Should have reset
	if flowAfter >= flowBefore {
		t.Errorf("Expected flow to reset after reverse, before=%.2f, after=%.2f", flowBefore, flowAfter)
	}
}

func TestNetOrderFlow_NoResetOnReverse(t *testing.T) {
	nof := NewNetOrderFlow("test_no_reset", false, 100)

	// Build up buy flow
	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: float64(100 + i),
			BidPrice:  []float64{99.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{101.0},
			AskQty:    []uint32{50},
		}
		nof.Update(md)
	}

	// Add sell flow
	for i := 0; i < 3; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: float64(104 - i),
			BidPrice:  []float64{99.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{101.0},
			AskQty:    []uint32{50},
		}
		nof.Update(md)
	}

	flowAfter := nof.GetCumulativeFlow()
	// Should continue accumulating (not reset)
	// First update baseline (no direction), then 4 upticks, then some downticks
	// Net should be positive (more buys than sells)
	if flowAfter <= 0 {
		t.Errorf("Expected positive flow (no reset), got %.2f", flowAfter)
	}

	// Should have accumulated both buy and sell flow
	if nof.GetBuyFlow() <= 0 {
		t.Error("Expected positive buy flow")
	}
	if nof.GetSellFlow() <= 0 {
		t.Error("Expected positive sell flow")
	}
}

func TestNetOrderFlow_ManualReset(t *testing.T) {
	nof := NewNetOrderFlow("test_manual_reset", false, 100)

	// Build up flow
	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: float64(100 + i),
			BidPrice:  []float64{99.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{101.0},
			AskQty:    []uint32{50},
		}
		nof.Update(md)
	}

	if nof.GetCumulativeFlow() == 0 {
		t.Error("Expected non-zero flow before reset")
	}

	// Manual reset
	nof.Reset()

	if nof.GetCumulativeFlow() != 0 {
		t.Errorf("Expected flow 0 after reset, got %.2f", nof.GetCumulativeFlow())
	}

	if nof.GetBuyFlow() != 0 {
		t.Errorf("Expected buy flow 0 after reset, got %.2f", nof.GetBuyFlow())
	}

	if nof.GetSellFlow() != 0 {
		t.Errorf("Expected sell flow 0 after reset, got %.2f", nof.GetSellFlow())
	}
}

func TestNetOrderFlow_FlowStrength(t *testing.T) {
	nof := NewNetOrderFlow("test_strength", false, 100)

	// Build up sell flow
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: float64(100 - i),
			BidPrice:  []float64{99.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{101.0},
			AskQty:    []uint32{50},
		}
		nof.Update(md)
	}

	// Cumulative should be negative
	cumulative := nof.GetCumulativeFlow()
	if cumulative >= 0 {
		t.Errorf("Expected negative cumulative, got %.2f", cumulative)
	}

	// Strength should be positive (absolute value)
	strength := nof.GetFlowStrength()
	if strength <= 0 {
		t.Errorf("Expected positive strength, got %.2f", strength)
	}

	// Strength should equal absolute value of cumulative
	if strength != -cumulative {
		t.Errorf("Expected strength %.2f to equal abs(cumulative) %.2f", strength, -cumulative)
	}
}

func TestNetOrderFlow_EmptyPrice(t *testing.T) {
	nof := NewNetOrderFlow("test_empty", false, 100)

	md := &mdpb.MarketDataUpdate{
		LastPrice: 0, // No price
	}
	nof.Update(md)

	if nof.GetCumulativeFlow() != 0 {
		t.Errorf("Expected flow 0 for empty price, got %.2f", nof.GetCumulativeFlow())
	}
}

func TestNetOrderFlow_GetValue(t *testing.T) {
	nof := NewNetOrderFlow("test_getvalue", false, 100)

	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: float64(100 + i),
			BidPrice:  []float64{99.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{101.0},
			AskQty:    []uint32{50},
		}
		nof.Update(md)
	}

	// GetValue() should return cumulative flow
	value := nof.GetValue()
	cumulative := nof.GetCumulativeFlow()

	if value != cumulative {
		t.Errorf("GetValue() should equal GetCumulativeFlow(), got %.2f vs %.2f", value, cumulative)
	}
}

func TestNetOrderFlow_History(t *testing.T) {
	nof := NewNetOrderFlow("test_history", false, 10)

	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: float64(100 + i),
			BidPrice:  []float64{99.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{101.0},
			AskQty:    []uint32{50},
		}
		nof.Update(md)
	}

	history := nof.GetValues()
	if len(history) != 10 {
		t.Errorf("Expected history length 10, got %d", len(history))
	}
}

func TestNetOrderFlow_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":             "config_nof",
		"reset_on_reverse": true,
		"max_history":      200.0,
	}

	indicator, err := NewNetOrderFlowFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	nof, ok := indicator.(*NetOrderFlow)
	if !ok {
		t.Fatal("Expected *NetOrderFlow type")
	}

	if nof.GetName() != "config_nof" {
		t.Errorf("Expected name 'config_nof', got '%s'", nof.GetName())
	}
}

func TestNetOrderFlow_String(t *testing.T) {
	nof := NewNetOrderFlow("test_string", false, 100)

	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: float64(100 + i),
			BidPrice:  []float64{99.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{101.0},
			AskQty:    []uint32{50},
		}
		nof.Update(md)
	}

	str := nof.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "NetOrderFlow") {
		t.Error("String() should contain 'NetOrderFlow'")
	}
}

func BenchmarkNetOrderFlow_Update(b *testing.B) {
	nof := NewNetOrderFlow("bench", false, 1000)

	md := &mdpb.MarketDataUpdate{
		LastPrice: 100.0,
		BidPrice:  []float64{99.5},
		BidQty:    []uint32{50},
		AskPrice:  []float64{100.5},
		AskQty:    []uint32{50},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		md.LastPrice = 100.0 + float64(i%10)*0.1
		nof.Update(md)
	}
}
