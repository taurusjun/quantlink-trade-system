package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestOrderBookVolumeBid(t *testing.T) {
	indicator := NewOrderBookVolume(5, "bid", 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{90, 70, 50, 30, 10},
	}

	indicator.Update(md)

	expected := 300.0 // 100 + 80 + 60 + 40 + 20
	got := indicator.GetValue()

	if got != expected {
		t.Errorf("Expected bid volume %f, got %f", expected, got)
	}

	if indicator.GetBidVolume() != expected {
		t.Errorf("Expected GetBidVolume() %f, got %f", expected, indicator.GetBidVolume())
	}
}

func TestOrderBookVolumeAsk(t *testing.T) {
	indicator := NewOrderBookVolume(5, "ask", 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{90, 70, 50, 30, 10},
	}

	indicator.Update(md)

	expected := 250.0 // 90 + 70 + 50 + 30 + 10
	got := indicator.GetValue()

	if got != expected {
		t.Errorf("Expected ask volume %f, got %f", expected, got)
	}

	if indicator.GetAskVolume() != expected {
		t.Errorf("Expected GetAskVolume() %f, got %f", expected, indicator.GetAskVolume())
	}
}

func TestOrderBookVolumeBoth(t *testing.T) {
	indicator := NewOrderBookVolume(5, "both", 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{90, 70, 50, 30, 10},
	}

	indicator.Update(md)

	expected := 550.0 // 300 + 250
	got := indicator.GetValue()

	if got != expected {
		t.Errorf("Expected total volume %f, got %f", expected, got)
	}
}

func TestOrderBookVolumeLimitedLevels(t *testing.T) {
	indicator := NewOrderBookVolume(3, "both", 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{90, 70, 50, 30, 10},
	}

	indicator.Update(md)

	// Should only count first 3 levels
	expected := 450.0 // (100+80+60) + (90+70+50)
	got := indicator.GetValue()

	if got != expected {
		t.Errorf("Expected volume with 3 levels %f, got %f", expected, got)
	}
}

func TestOrderBookVolumeEmpty(t *testing.T) {
	indicator := NewOrderBookVolume(5, "both", 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{},
		AskQty:    []uint32{},
	}

	indicator.Update(md)

	expected := 0.0
	got := indicator.GetValue()

	if got != expected {
		t.Errorf("Expected zero volume %f, got %f", expected, got)
	}
}

func TestOrderBookVolumeFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"levels":      float64(10),
		"side":        "bid",
		"max_history": float64(500),
	}

	ind, err := NewOrderBookVolumeFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	indicator := ind.(*OrderBookVolume)
	if indicator.levels != 10 {
		t.Errorf("Expected levels 10, got %d", indicator.levels)
	}
	if indicator.side != "bid" {
		t.Errorf("Expected side 'bid', got '%s'", indicator.side)
	}
	if indicator.maxHistory != 500 {
		t.Errorf("Expected max_history 500, got %d", indicator.maxHistory)
	}
}

func TestOrderBookVolumeFromConfigInvalidSide(t *testing.T) {
	config := map[string]interface{}{
		"side": "invalid",
	}

	_, err := NewOrderBookVolumeFromConfig(config)
	if err == nil {
		t.Error("Expected error for invalid side")
	}
}

func TestOrderBookVolumeFromConfigInvalidLevels(t *testing.T) {
	config := map[string]interface{}{
		"levels": float64(-1),
	}

	_, err := NewOrderBookVolumeFromConfig(config)
	if err == nil {
		t.Error("Expected error for invalid levels")
	}
}

func TestOrderBookVolumeReset(t *testing.T) {
	indicator := NewOrderBookVolume(5, "both", 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{90, 70, 50, 30, 10},
	}

	indicator.Update(md)

	if !indicator.IsReady() {
		t.Error("Indicator should be ready after update")
	}

	indicator.Reset()

	if indicator.IsReady() {
		t.Error("Indicator should not be ready after reset")
	}

	if indicator.GetBidVolume() != 0 {
		t.Error("Bid volume should be zero after reset")
	}

	if indicator.GetAskVolume() != 0 {
		t.Error("Ask volume should be zero after reset")
	}
}

func TestOrderBookVolumeHistory(t *testing.T) {
	indicator := NewOrderBookVolume(2, "both", 3)

	volumeSets := [][]uint32{
		{100, 80},
		{120, 90},
		{110, 85},
		{130, 95}, // Should push out the first value
	}

	for _, vols := range volumeSets {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: 1000,
			BidQty:    vols,
			AskQty:    vols,
		}
		indicator.Update(md)
	}

	// Should only keep last 3 values
	values := indicator.GetValues()
	if len(values) != 3 {
		t.Errorf("Expected 3 values in history, got %d", len(values))
	}
}

func BenchmarkOrderBookVolume(b *testing.B) {
	indicator := NewOrderBookVolume(5, "both", 1000)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 80, 60, 40, 20},
		AskQty:    []uint32{90, 70, 50, 30, 10},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indicator.Update(md)
	}
}

func BenchmarkOrderBookVolumeBidOnly(b *testing.B) {
	indicator := NewOrderBookVolume(10, "bid", 1000)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidQty:    []uint32{100, 90, 80, 70, 60, 50, 40, 30, 20, 10},
		AskQty:    []uint32{95, 85, 75, 65, 55, 45, 35, 25, 15, 5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indicator.Update(md)
	}
}
