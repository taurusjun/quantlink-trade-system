package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestMomentum(t *testing.T) {
	indicator := NewMomentum(3, 100)

	// Prices: [100, 105, 110, 115]
	// Momentum (period=3) = 115 - 100 = 15
	prices := []float64{100.0, 105.0, 110.0, 115.0}

	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: 1000,
			BidPrice:  []float64{price},
			AskPrice:  []float64{price},
		}
		indicator.Update(md)
	}

	if !indicator.IsReady() {
		t.Error("Indicator should be ready after 4 updates (period+1)")
	}

	expected := 15.0
	got := indicator.GetValue()

	if got != expected {
		t.Errorf("Expected momentum %f, got %f", expected, got)
	}
}

func TestMomentumNegative(t *testing.T) {
	indicator := NewMomentum(5, 100)

	// Downtrend: prices decreasing
	// Prices: [120, 118, 116, 114, 112, 110]
	// Momentum = 110 - 120 = -10
	prices := []float64{120.0, 118.0, 116.0, 114.0, 112.0, 110.0}

	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: 1000,
			BidPrice:  []float64{price},
			AskPrice:  []float64{price},
		}
		indicator.Update(md)
	}

	expected := -10.0
	got := indicator.GetValue()

	if got != expected {
		t.Errorf("Expected momentum %f, got %f", expected, got)
	}
}

func TestMomentumZero(t *testing.T) {
	indicator := NewMomentum(3, 100)

	// Sideways: same price
	// Prices: [100, 100, 100, 100]
	// Momentum = 100 - 100 = 0
	prices := []float64{100.0, 100.0, 100.0, 100.0}

	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: 1000,
			BidPrice:  []float64{price},
			AskPrice:  []float64{price},
		}
		indicator.Update(md)
	}

	expected := 0.0
	got := indicator.GetValue()

	if got != expected {
		t.Errorf("Expected momentum %f, got %f", expected, got)
	}
}

func TestMomentumNotReady(t *testing.T) {
	indicator := NewMomentum(5, 100)

	// Add only 4 prices (less than period+1)
	for i := 0; i < 4; i++ {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: 1000,
			BidPrice:  []float64{100.0 + float64(i)},
			AskPrice:  []float64{100.0 + float64(i)},
		}
		indicator.Update(md)
	}

	if indicator.IsReady() {
		t.Error("Indicator should not be ready with insufficient data")
	}

	if indicator.GetValue() != 0.0 {
		t.Error("Indicator should return 0 when not ready")
	}
}

func TestMomentumEmpty(t *testing.T) {
	indicator := NewMomentum(5, 100)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{},
		AskPrice:  []float64{},
	}

	indicator.Update(md)

	if indicator.IsReady() {
		t.Error("Indicator should not be ready with empty data")
	}
}

func TestMomentumFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(14),
		"max_history": float64(500),
	}

	ind, err := NewMomentumFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	indicator := ind.(*Momentum)
	if indicator.period != 14 {
		t.Errorf("Expected period 14, got %d", indicator.period)
	}
	if indicator.maxHistory != 500 {
		t.Errorf("Expected max_history 500, got %d", indicator.maxHistory)
	}
}

func TestMomentumFromConfigInvalidPeriod(t *testing.T) {
	config := map[string]interface{}{
		"period": float64(-1),
	}

	_, err := NewMomentumFromConfig(config)
	if err == nil {
		t.Error("Expected error for invalid period")
	}
}

func TestMomentumReset(t *testing.T) {
	indicator := NewMomentum(3, 100)

	prices := []float64{100.0, 105.0, 110.0, 115.0}
	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: 1000,
			BidPrice:  []float64{price},
			AskPrice:  []float64{price},
		}
		indicator.Update(md)
	}

	if !indicator.IsReady() {
		t.Error("Indicator should be ready after updates")
	}

	indicator.Reset()

	if indicator.IsReady() {
		t.Error("Indicator should not be ready after reset")
	}

	if len(indicator.GetPrices()) != 0 {
		t.Error("Price buffer should be empty after reset")
	}
}

func TestMomentumHistory(t *testing.T) {
	indicator := NewMomentum(3, 3)

	// Add 7 updates, should keep last 3 momentum values
	for i := 0; i < 7; i++ {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: uint64(1000 + i),
			BidPrice:  []float64{100.0 + float64(i*2)},
			AskPrice:  []float64{100.0 + float64(i*2)},
		}
		indicator.Update(md)
	}

	values := indicator.GetValues()
	if len(values) != 3 {
		t.Errorf("Expected 3 values in history, got %d", len(values))
	}
}

func TestMomentumGetPeriod(t *testing.T) {
	indicator := NewMomentum(20, 100)
	if indicator.GetPeriod() != 20 {
		t.Errorf("Expected period 20, got %d", indicator.GetPeriod())
	}
}

func TestMomentumTrendDetection(t *testing.T) {
	// Test momentum as a trend detector
	indicator := NewMomentum(5, 100)

	// Strong uptrend
	for i := 0; i < 6; i++ {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: uint64(1000 + i),
			BidPrice:  []float64{100.0 + float64(i*5)},
			AskPrice:  []float64{100.0 + float64(i*5)},
		}
		indicator.Update(md)
	}

	momentum := indicator.GetValue()

	// In strong uptrend, momentum should be significantly positive
	if momentum <= 0 {
		t.Errorf("In uptrend, momentum should be positive, got %f", momentum)
	}

	// Expect momentum = (100 + 5*5) - 100 = 25
	expectedMomentum := 25.0
	if momentum != expectedMomentum {
		t.Errorf("Expected momentum %f, got %f", expectedMomentum, momentum)
	}
}

func BenchmarkMomentum(b *testing.B) {
	indicator := NewMomentum(10, 1000)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0},
		AskPrice:  []float64{100.0},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indicator.Update(md)
	}
}

func BenchmarkMomentumLargePeriod(b *testing.B) {
	indicator := NewMomentum(200, 1000)

	md := &mdpb.MarketDataUpdate{
		Symbol:    "TEST",
		Exchange:  "EXCHANGE",
		Timestamp: 1000,
		BidPrice:  []float64{100.0},
		AskPrice:  []float64{100.0},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		indicator.Update(md)
	}
}
