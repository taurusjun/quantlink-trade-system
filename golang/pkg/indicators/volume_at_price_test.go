package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestVolumeAtPrice_BestBid(t *testing.T) {
	vap := NewVolumeAtPrice("test_best_bid", 0, "bid", 0.01, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{50, 30, 20},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{45, 35, 25},
	}

	vap.Update(md)

	// Should get volume at best bid (100.0)
	expected := 50.0
	if vap.GetValue() != expected {
		t.Errorf("Expected volume %.0f, got %.0f", expected, vap.GetValue())
	}

	if vap.GetActualPrice() != 100.0 {
		t.Errorf("Expected actual price 100.0, got %.2f", vap.GetActualPrice())
	}
}

func TestVolumeAtPrice_BestAsk(t *testing.T) {
	vap := NewVolumeAtPrice("test_best_ask", 0, "ask", 0.01, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{50, 30, 20},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{45, 35, 25},
	}

	vap.Update(md)

	// Should get volume at best ask (100.1)
	expected := 45.0
	if vap.GetValue() != expected {
		t.Errorf("Expected volume %.0f, got %.0f", expected, vap.GetValue())
	}

	if vap.GetActualPrice() != 100.1 {
		t.Errorf("Expected actual price 100.1, got %.2f", vap.GetActualPrice())
	}
}

func TestVolumeAtPrice_SpecificPrice(t *testing.T) {
	vap := NewVolumeAtPrice("test_specific", 99.9, "bid", 0.01, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{50, 30, 20},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{45, 35, 25},
	}

	vap.Update(md)

	// Should get volume at 99.9
	expected := 30.0
	if vap.GetValue() != expected {
		t.Errorf("Expected volume %.0f, got %.0f", expected, vap.GetValue())
	}

	if vap.GetActualPrice() != 99.9 {
		t.Errorf("Expected actual price 99.9, got %.2f", vap.GetActualPrice())
	}
}

func TestVolumeAtPrice_WithTolerance(t *testing.T) {
	vap := NewVolumeAtPrice("test_tolerance", 100.05, "ask", 0.1, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{50, 30, 20},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{45, 35, 25},
	}

	vap.Update(md)

	// Should match 100.1 (within tolerance of 0.1)
	expected := 45.0
	if vap.GetValue() != expected {
		t.Errorf("Expected volume %.0f, got %.0f", expected, vap.GetValue())
	}
}

func TestVolumeAtPrice_NoMatch(t *testing.T) {
	vap := NewVolumeAtPrice("test_no_match", 105.0, "bid", 0.01, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{50, 30, 20},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{45, 35, 25},
	}

	vap.Update(md)

	// No price within tolerance
	if vap.GetValue() != 0 {
		t.Errorf("Expected volume 0, got %.0f", vap.GetValue())
	}

	if vap.GetActualPrice() != 0 {
		t.Errorf("Expected actual price 0, got %.2f", vap.GetActualPrice())
	}
}

func TestVolumeAtPrice_EmptyData(t *testing.T) {
	vap := NewVolumeAtPrice("test_empty", 100.0, "bid", 0.01, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{},
		BidQty:   []uint32{},
		AskPrice: []float64{},
		AskQty:   []uint32{},
	}

	vap.Update(md)

	if vap.GetValue() != 0 {
		t.Errorf("Expected volume 0, got %.0f", vap.GetValue())
	}
}

func TestVolumeAtPrice_SetTargetPrice(t *testing.T) {
	vap := NewVolumeAtPrice("test_set_target", 100.0, "bid", 0.01, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{50, 30, 20},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{45, 35, 25},
	}

	vap.Update(md)

	if vap.GetValue() != 50.0 {
		t.Errorf("Expected volume 50, got %.0f", vap.GetValue())
	}

	// Change target price
	vap.SetTargetPrice(99.8)
	vap.Update(md)

	if vap.GetValue() != 20.0 {
		t.Errorf("Expected volume 20 after changing target, got %.0f", vap.GetValue())
	}
}

func TestVolumeAtPrice_History(t *testing.T) {
	vap := NewVolumeAtPrice("test_history", 100.0, "bid", 0.01, 5)

	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0},
			BidQty:   []uint32{uint32(10 + i*10)},
			AskPrice: []float64{100.1},
			AskQty:   []uint32{50},
		}
		vap.Update(md)
	}

	history := vap.GetValues()
	if len(history) != 5 {
		t.Errorf("Expected history length 5, got %d", len(history))
	}

	// Last value should be 10 + 9*10 = 100
	lastValue := history[len(history)-1]
	if lastValue != 100.0 {
		t.Errorf("Expected last value 100, got %.0f", lastValue)
	}
}

func TestVolumeAtPrice_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":         "config_vap",
		"target_price": 99.5,
		"side":         "ask",
		"tolerance":    0.05,
		"max_history":  200.0,
	}

	indicator, err := NewVolumeAtPriceFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	vap, ok := indicator.(*VolumeAtPrice)
	if !ok {
		t.Fatal("Expected *VolumeAtPrice type")
	}

	if vap.GetName() != "config_vap" {
		t.Errorf("Expected name 'config_vap', got '%s'", vap.GetName())
	}
}

func TestVolumeAtPrice_String(t *testing.T) {
	vap := NewVolumeAtPrice("test_string", 100.0, "bid", 0.01, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		BidQty:   []uint32{75},
		AskPrice: []float64{100.1},
		AskQty:   []uint32{50},
	}
	vap.Update(md)

	str := vap.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "VolumeAtPrice") {
		t.Error("String() should contain 'VolumeAtPrice'")
	}
}

func BenchmarkVolumeAtPrice_Update(b *testing.B) {
	vap := NewVolumeAtPrice("bench", 100.0, "bid", 0.01, 1000)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:   []uint32{50, 40, 30, 20, 10},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		AskQty:   []uint32{55, 45, 35, 25, 15},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		vap.Update(md)
	}
}
