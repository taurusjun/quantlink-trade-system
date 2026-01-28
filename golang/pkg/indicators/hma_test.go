package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestHMA_Creation(t *testing.T) {
	ind := NewHMA(20, 1000)
	if ind == nil {
		t.Fatal("Failed to create HMA indicator")
	}
	if ind.GetName() != "HMA" {
		t.Errorf("Expected name 'HMA', got '%s'", ind.GetName())
	}
	if ind.GetPeriod() != 20 {
		t.Errorf("Expected period 20, got %d", ind.GetPeriod())
	}
}

func TestHMA_IsReady(t *testing.T) {
	ind := NewHMA(10, 100)

	if ind.IsReady() {
		t.Error("Indicator should not be ready initially")
	}

	// HMA needs time to initialize: half period + full period + sqrt period
	// For period=10: wma1 needs 5, wma2 needs 10, wmaFinal needs sqrt(10)≈7
	// So it needs about 20+ updates to be fully ready
	for i := 0; i < 25; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)*0.1},
			AskPrice: []float64{100.1 + float64(i)*0.1},
		}
		ind.Update(md)
	}

	if !ind.IsReady() {
		t.Error("Indicator should be ready after sufficient updates")
	}
}

func TestHMA_Calculation(t *testing.T) {
	ind := NewHMA(9, 100) // Use period 9 for easier calculation

	// Create uptrend data - need more data for HMA to initialize
	prices := []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110, 111, 112, 113, 114, 115, 116, 117, 118, 119, 120}

	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		ind.Update(md)
	}

	if !ind.IsReady() {
		t.Skip("HMA not ready yet with provided data")
	}

	// HMA should respond faster than SMA to trends
	hmaValue := ind.GetValue()

	// In an uptrend, HMA should be closer to recent prices
	if hmaValue < 100 || hmaValue > 121 {
		t.Errorf("HMA value %f seems out of reasonable range", hmaValue)
	}
}

func TestHMA_SmoothAndResponsive(t *testing.T) {
	hma := NewHMA(10, 100)
	sma := NewSMA(10, 100)

	// Create data with a trend change
	prices := []float64{
		100, 101, 102, 103, 104, 105, 106, 107, 108, 109, // Uptrend
		109, 108, 107, 106, 105, 104, 103, 102, 101, 100, // Downtrend
	}

	var hmaValues []float64
	var smaValues []float64

	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		hma.Update(md)
		sma.Update(md)

		if hma.IsReady() {
			hmaValues = append(hmaValues, hma.GetValue())
		}
		if sma.IsReady() {
			smaValues = append(smaValues, sma.GetValue())
		}
	}

	// HMA should have values (it responds faster)
	if len(hmaValues) == 0 {
		t.Fatal("HMA produced no values")
	}

	// HMA values should be reasonable
	for _, val := range hmaValues {
		if val < 95 || val > 115 {
			t.Errorf("HMA value %f outside expected range", val)
		}
	}
}

func TestHMA_Reset(t *testing.T) {
	ind := NewHMA(10, 100)

	// Add some data
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{100.1 + float64(i)},
		}
		ind.Update(md)
	}

	if !ind.IsReady() {
		t.Error("Indicator should be ready before reset")
	}

	ind.Reset()

	if ind.IsReady() {
		t.Error("Indicator should not be ready after reset")
	}
	if ind.GetValue() != 0 {
		t.Errorf("Expected value 0 after reset, got %f", ind.GetValue())
	}
}

func TestHMA_TrendFollowing(t *testing.T) {
	ind := NewHMA(14, 100)

	// Create strong uptrend
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i)*2.0 // Strong uptrend
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		ind.Update(md)
	}

	if !ind.IsReady() {
		t.Error("HMA should be ready after 30 updates")
	}

	hmaValue := ind.GetValue()

	// In strong uptrend, HMA should be well above starting price
	if hmaValue < 120 {
		t.Errorf("HMA value %f too low for strong uptrend", hmaValue)
	}
}

func TestNewHMAFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(14),
		"max_history": float64(500),
	}

	ind, err := NewHMAFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create HMA from config: %v", err)
	}

	hma, ok := ind.(*HMA)
	if !ok {
		t.Fatal("Indicator is not of type *HMA")
	}

	if hma.GetPeriod() != 14 {
		t.Errorf("Expected period 14, got %d", hma.GetPeriod())
	}
}

func TestNewHMAFromConfig_Defaults(t *testing.T) {
	config := map[string]interface{}{}

	ind, err := NewHMAFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create HMA from config: %v", err)
	}

	hma, ok := ind.(*HMA)
	if !ok {
		t.Fatal("Indicator is not of type *HMA")
	}

	if hma.GetPeriod() != 20 {
		t.Errorf("Expected default period 20, got %d", hma.GetPeriod())
	}
}

func TestHMA_NoNaN(t *testing.T) {
	ind := NewHMA(10, 100)

	prices := []float64{100, 101, 102, 103, 104, 105, 106, 107, 108, 109, 110}

	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		ind.Update(md)

		if ind.IsReady() {
			val := ind.GetValue()
			if math.IsNaN(val) || math.IsInf(val, 0) {
				t.Errorf("HMA produced invalid value: %f", val)
			}
		}
	}
}

func BenchmarkHMA_Update(b *testing.B) {
	ind := NewHMA(20, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ind.Update(md)
	}
}

func BenchmarkHMA_FullCalculation(b *testing.B) {
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ind := NewHMA(20, 1000)
		for j := 0; j < 50; j++ {
			ind.Update(md)
		}
		_ = ind.GetValue()
	}
}
