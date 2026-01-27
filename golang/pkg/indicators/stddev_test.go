package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestStdDev(t *testing.T) {
	indicator := NewStdDev(3, 100)

	// Prices: [100, 110, 120]
	// Mean = (100 + 110 + 120) / 3 = 110
	// Variance = ((100-110)² + (110-110)² + (120-110)²) / 3
	//          = (100 + 0 + 100) / 3 = 66.67
	// StdDev = sqrt(66.67) = 8.165
	prices := []float64{100.0, 110.0, 120.0}

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
		t.Error("Indicator should be ready after 3 updates")
	}

	expected := 8.165
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.01 {
		t.Errorf("Expected StdDev %f, got %f", expected, got)
	}

	// Check mean
	expectedMean := 110.0
	gotMean := indicator.GetMean()

	if math.Abs(gotMean-expectedMean) > 0.01 {
		t.Errorf("Expected mean %f, got %f", expectedMean, gotMean)
	}
}

func TestStdDevConstantPrices(t *testing.T) {
	indicator := NewStdDev(5, 100)

	// All same price - should have zero standard deviation
	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: 1000,
			BidPrice:  []float64{100.0},
			AskPrice:  []float64{100.0},
		}
		indicator.Update(md)
	}

	expected := 0.0
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.001 {
		t.Errorf("Expected StdDev %f for constant prices, got %f", expected, got)
	}
}

func TestStdDevHighVolatility(t *testing.T) {
	indicator := NewStdDev(5, 100)

	// High volatility prices
	prices := []float64{100.0, 150.0, 80.0, 140.0, 90.0}

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

	// Mean = (100 + 150 + 80 + 140 + 90) / 5 = 112
	// Should have relatively high standard deviation
	stddev := indicator.GetValue()

	if stddev < 20 {
		t.Errorf("Expected high volatility (StdDev > 20), got %f", stddev)
	}
}

func TestStdDevLowVolatility(t *testing.T) {
	indicator := NewStdDev(5, 100)

	// Low volatility prices (all close together)
	prices := []float64{100.0, 101.0, 99.0, 100.5, 99.5}

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

	// Should have relatively low standard deviation
	stddev := indicator.GetValue()

	if stddev > 5 {
		t.Errorf("Expected low volatility (StdDev < 5), got %f", stddev)
	}
}

func TestStdDevNotReady(t *testing.T) {
	indicator := NewStdDev(10, 100)

	// Add only 5 prices (less than period)
	for i := 0; i < 5; i++ {
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

func TestStdDevEmpty(t *testing.T) {
	indicator := NewStdDev(10, 100)

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

func TestStdDevFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(30),
		"max_history": float64(500),
	}

	ind, err := NewStdDevFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	indicator := ind.(*StdDev)
	if indicator.period != 30 {
		t.Errorf("Expected period 30, got %d", indicator.period)
	}
	if indicator.maxHistory != 500 {
		t.Errorf("Expected max_history 500, got %d", indicator.maxHistory)
	}
}

func TestStdDevFromConfigInvalidPeriod(t *testing.T) {
	config := map[string]interface{}{
		"period": float64(-1),
	}

	_, err := NewStdDevFromConfig(config)
	if err == nil {
		t.Error("Expected error for invalid period")
	}
}

func TestStdDevReset(t *testing.T) {
	indicator := NewStdDev(3, 100)

	prices := []float64{100.0, 110.0, 120.0}
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

	if indicator.sum != 0 {
		t.Error("Sum should be zero after reset")
	}
}

func TestStdDevHistory(t *testing.T) {
	indicator := NewStdDev(3, 3)

	// Add 6 updates, should keep last 3 StdDev values
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

	values := indicator.GetValues()
	if len(values) != 3 {
		t.Errorf("Expected 3 values in history, got %d", len(values))
	}
}

func TestStdDevGetPeriod(t *testing.T) {
	indicator := NewStdDev(50, 100)
	if indicator.GetPeriod() != 50 {
		t.Errorf("Expected period 50, got %d", indicator.GetPeriod())
	}
}

func TestStdDevAsVolatilityMeasure(t *testing.T) {
	// Compare high vs low volatility scenarios
	highVol := NewStdDev(5, 100)
	lowVol := NewStdDev(5, 100)

	// High volatility: large price swings
	highPrices := []float64{100.0, 120.0, 90.0, 130.0, 85.0}

	// Low volatility: small price movements
	lowPrices := []float64{100.0, 101.0, 99.5, 100.5, 100.0}

	for _, price := range highPrices {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: 1000,
			BidPrice:  []float64{price},
			AskPrice:  []float64{price},
		}
		highVol.Update(md)
	}

	for _, price := range lowPrices {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: 1000,
			BidPrice:  []float64{price},
			AskPrice:  []float64{price},
		}
		lowVol.Update(md)
	}

	highVolVal := highVol.GetValue()
	lowVolVal := lowVol.GetValue()

	// High volatility should have much larger StdDev
	if highVolVal <= lowVolVal*3 {
		t.Errorf("High volatility (%f) should be >> low volatility (%f)", highVolVal, lowVolVal)
	}
}

func TestStdDevComparisonWithManualCalculation(t *testing.T) {
	indicator := NewStdDev(4, 100)

	// Known dataset: [10, 12, 23, 23]
	// Mean = (10 + 12 + 23 + 23) / 4 = 17
	// Variance = ((10-17)² + (12-17)² + (23-17)² + (23-17)²) / 4
	//          = (49 + 25 + 36 + 36) / 4 = 146 / 4 = 36.5
	// StdDev = sqrt(36.5) = 6.04
	prices := []float64{10.0, 12.0, 23.0, 23.0}

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

	expected := 6.04
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.01 {
		t.Errorf("Expected StdDev %f, got %f", expected, got)
	}
}

func BenchmarkStdDev(b *testing.B) {
	indicator := NewStdDev(20, 1000)

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

func BenchmarkStdDevLargePeriod(b *testing.B) {
	indicator := NewStdDev(200, 1000)

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
