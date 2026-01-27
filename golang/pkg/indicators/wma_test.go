package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestWMA(t *testing.T) {
	indicator := NewWMA(3, 100)

	// Test with 3 periods: prices [100, 110, 120]
	// WMA = (100*1 + 110*2 + 120*3) / (1+2+3)
	//     = (100 + 220 + 360) / 6
	//     = 680 / 6 = 113.33
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

	expected := 113.33333
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.01 {
		t.Errorf("Expected WMA %f, got %f", expected, got)
	}
}

func TestWMAManualCalculation(t *testing.T) {
	indicator := NewWMA(5, 100)

	// Prices: [100, 102, 104, 106, 108]
	// Weights: [1, 2, 3, 4, 5]
	// WMA = (100*1 + 102*2 + 104*3 + 106*4 + 108*5) / (1+2+3+4+5)
	//     = (100 + 204 + 312 + 424 + 540) / 15
	//     = 1580 / 15 = 105.33
	prices := []float64{100.0, 102.0, 104.0, 106.0, 108.0}

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

	expected := 105.3333
	got := indicator.GetValue()

	if math.Abs(got-expected) > 0.01 {
		t.Errorf("Expected WMA %f, got %f", expected, got)
	}
}

func TestWMAComparison(t *testing.T) {
	// Compare WMA with SMA - WMA should be closer to recent prices
	wma := NewWMA(5, 100)
	sma := NewSMA(5, 100)

	// Uptrend: prices increasing
	prices := []float64{100.0, 102.0, 104.0, 106.0, 108.0}

	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: 1000,
			BidPrice:  []float64{price},
			AskPrice:  []float64{price},
		}
		wma.Update(md)
		sma.Update(md)
	}

	wmaVal := wma.GetValue()
	smaVal := sma.GetValue()

	// In an uptrend, WMA should be higher than SMA (closer to recent higher prices)
	if wmaVal <= smaVal {
		t.Errorf("In uptrend, WMA (%f) should be > SMA (%f)", wmaVal, smaVal)
	}
}

func TestWMANotReady(t *testing.T) {
	indicator := NewWMA(5, 100)

	// Add only 3 prices (less than period)
	for i := 0; i < 3; i++ {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: 1000,
			BidPrice:  []float64{100.0},
			AskPrice:  []float64{100.0},
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

func TestWMAEmpty(t *testing.T) {
	indicator := NewWMA(5, 100)

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

func TestWMAFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(10),
		"max_history": float64(500),
	}

	ind, err := NewWMAFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	indicator := ind.(*WMA)
	if indicator.period != 10 {
		t.Errorf("Expected period 10, got %d", indicator.period)
	}
	if indicator.maxHistory != 500 {
		t.Errorf("Expected max_history 500, got %d", indicator.maxHistory)
	}
}

func TestWMAFromConfigInvalidPeriod(t *testing.T) {
	config := map[string]interface{}{
		"period": float64(-1),
	}

	_, err := NewWMAFromConfig(config)
	if err == nil {
		t.Error("Expected error for invalid period")
	}
}

func TestWMAReset(t *testing.T) {
	indicator := NewWMA(3, 100)

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
}

func TestWMAHistory(t *testing.T) {
	indicator := NewWMA(3, 3)

	// Add 5 updates, should keep last 3 WMA values
	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			Symbol:    "TEST",
			Exchange:  "EXCHANGE",
			Timestamp: uint64(1000 + i),
			BidPrice:  []float64{100.0 + float64(i)},
			AskPrice:  []float64{100.0 + float64(i)},
		}
		indicator.Update(md)
	}

	values := indicator.GetValues()
	if len(values) != 3 {
		t.Errorf("Expected 3 values in history, got %d", len(values))
	}
}

func TestWMAGetPeriod(t *testing.T) {
	indicator := NewWMA(15, 100)
	if indicator.GetPeriod() != 15 {
		t.Errorf("Expected period 15, got %d", indicator.GetPeriod())
	}
}

func BenchmarkWMA(b *testing.B) {
	indicator := NewWMA(20, 1000)

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

func BenchmarkWMALargePeriod(b *testing.B) {
	indicator := NewWMA(200, 1000)

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
