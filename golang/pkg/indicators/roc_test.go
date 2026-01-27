package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestROC(t *testing.T) {
	indicator := NewROC(3, 100)

	// Prices: [100, 105, 110, 120]
	// ROC (period=3) = ((120 - 100) / 100) * 100 = 20%
	prices := []float64{100.0, 105.0, 110.0, 120.0}

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

	expected := 20.0
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.01 {
		t.Errorf("Expected ROC %f%%, got %f%%", expected, got)
	}
}

func TestROCNegative(t *testing.T) {
	indicator := NewROC(5, 100)

	// Downtrend: 10% decline
	// Prices: [100, 98, 96, 94, 92, 90]
	// ROC = ((90 - 100) / 100) * 100 = -10%
	prices := []float64{100.0, 98.0, 96.0, 94.0, 92.0, 90.0}

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

	if math.Abs(got-expected) > 0.01 {
		t.Errorf("Expected ROC %f%%, got %f%%", expected, got)
	}
}

func TestROCZero(t *testing.T) {
	indicator := NewROC(3, 100)

	// No change
	// Prices: [100, 100, 100, 100]
	// ROC = ((100 - 100) / 100) * 100 = 0%
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

	if math.Abs(got-expected) > 0.01 {
		t.Errorf("Expected ROC %f%%, got %f%%", expected, got)
	}
}

func TestROCDouble(t *testing.T) {
	indicator := NewROC(3, 100)

	// 100% increase (doubling)
	// Prices: [50, 60, 70, 100]
	// ROC = ((100 - 50) / 50) * 100 = 100%
	prices := []float64{50.0, 60.0, 70.0, 100.0}

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

	expected := 100.0
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.01 {
		t.Errorf("Expected ROC %f%%, got %f%%", expected, got)
	}
}

func TestROCNotReady(t *testing.T) {
	indicator := NewROC(5, 100)

	// Add only 4 prices (less than period+1)
	for i := 0; i < 4; i++ {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: 1000,
			BidPrice:  []float64{100.0 + float64(i*2)},
			AskPrice:  []float64{100.0 + float64(i*2)},
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

func TestROCEmpty(t *testing.T) {
	indicator := NewROC(5, 100)

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

func TestROCFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(12),
		"max_history": float64(500),
	}

	ind, err := NewROCFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	indicator := ind.(*ROC)
	if indicator.period != 12 {
		t.Errorf("Expected period 12, got %d", indicator.period)
	}
	if indicator.maxHistory != 500 {
		t.Errorf("Expected max_history 500, got %d", indicator.maxHistory)
	}
}

func TestROCFromConfigInvalidPeriod(t *testing.T) {
	config := map[string]interface{}{
		"period": float64(-1),
	}

	_, err := NewROCFromConfig(config)
	if err == nil {
		t.Error("Expected error for invalid period")
	}
}

func TestROCReset(t *testing.T) {
	indicator := NewROC(3, 100)

	prices := []float64{100.0, 105.0, 110.0, 120.0}
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

func TestROCHistory(t *testing.T) {
	indicator := NewROC(3, 3)

	// Add 7 updates, should keep last 3 ROC values
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

func TestROCGetPeriod(t *testing.T) {
	indicator := NewROC(25, 100)
	if indicator.GetPeriod() != 25 {
		t.Errorf("Expected period 25, got %d", indicator.GetPeriod())
	}
}

func TestROCComparisonWithMomentum(t *testing.T) {
	// ROC and Momentum should have similar interpretation but different scales
	roc := NewROC(5, 100)
	momentum := NewMomentum(5, 100)

	// 20% increase
	prices := []float64{100.0, 105.0, 110.0, 115.0, 118.0, 120.0}

	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: 1000,
			BidPrice:  []float64{price},
			AskPrice:  []float64{price},
		}
		roc.Update(md)
		momentum.Update(md)
	}

	rocVal := roc.GetValue()       // Should be 20% = 20.0
	momentumVal := momentum.GetValue() // Should be 20 points

	// ROC is percentage, Momentum is absolute
	if rocVal != 20.0 {
		t.Errorf("Expected ROC 20%%, got %f%%", rocVal)
	}

	if momentumVal != 20.0 {
		t.Errorf("Expected Momentum 20, got %f", momentumVal)
	}

	// ROC = (Momentum / PastPrice) * 100
	// 20.0 = (20 / 100) * 100 ✓
}

func BenchmarkROC(b *testing.B) {
	indicator := NewROC(10, 1000)

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

func BenchmarkROCLargePeriod(b *testing.B) {
	indicator := NewROC(200, 1000)

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
