package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestMidPrice(t *testing.T) {
	indicator := NewMidPrice(100)

	// Test with valid bid/ask
	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0, 99.0},
		AskPrice:  []float64{101.0, 102.0},
	}

	indicator.Update(md)

	expected := 100.5 // (100 + 101) / 2
	got := indicator.GetValue()

	if got != expected {
		t.Errorf("Expected mid price %f, got %f", expected, got)
	}

	// Test IsReady
	if !indicator.IsReady() {
		t.Error("Indicator should be ready after first update")
	}
}

func TestMidPriceEmpty(t *testing.T) {
	indicator := NewMidPrice(100)

	// Test with empty bid/ask
	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{},
		AskPrice:  []float64{},
	}

	indicator.Update(md)

	// Should not add a value
	if indicator.IsReady() {
		t.Error("Indicator should not be ready with empty data")
	}
}

func TestMidPriceHistory(t *testing.T) {
	indicator := NewMidPrice(3)

	prices := []struct {
		bid      float64
		ask      float64
		expected float64
	}{
		{100.0, 101.0, 100.5},
		{102.0, 103.0, 102.5},
		{104.0, 105.0, 104.5},
		{106.0, 107.0, 106.5},
	}

	for _, p := range prices {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: 1000,
			BidPrice:  []float64{p.bid},
			AskPrice:  []float64{p.ask},
		}
		indicator.Update(md)
	}

	// Should only keep last 3 values
	values := indicator.GetValues()
	if len(values) != 3 {
		t.Errorf("Expected 3 values in history, got %d", len(values))
	}

	// Check last value
	expected := 106.5
	got := indicator.GetValue()
	if got != expected {
		t.Errorf("Expected last value %f, got %f", expected, got)
	}
}

func TestMidPriceFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"max_history": float64(500),
	}

	ind, err := NewMidPriceFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	indicator := ind.(*MidPrice)
	if indicator.maxHistory != 500 {
		t.Errorf("Expected max_history 500, got %d", indicator.maxHistory)
	}
}

func TestMidPriceReset(t *testing.T) {
	indicator := NewMidPrice(100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0},
		AskPrice:  []float64{101.0},
	}

	indicator.Update(md)

	if !indicator.IsReady() {
		t.Error("Indicator should be ready after update")
	}

	indicator.Reset()

	if indicator.IsReady() {
		t.Error("Indicator should not be ready after reset")
	}

	values := indicator.GetValues()
	if len(values) != 0 {
		t.Errorf("Expected 0 values after reset, got %d", len(values))
	}
}

func BenchmarkMidPrice(b *testing.B) {
	indicator := NewMidPrice(1000)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0, 99.0},
		AskPrice:  []float64{101.0, 102.0},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indicator.Update(md)
	}
}
