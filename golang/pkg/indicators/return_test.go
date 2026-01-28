package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestReturn_Creation(t *testing.T) {
	ind := NewReturn("simple", 1, 1000)
	if ind == nil {
		t.Fatal("Failed to create Return indicator")
	}
	if ind.GetName() != "Return" {
		t.Errorf("Expected name 'Return', got '%s'", ind.GetName())
	}
}

func TestReturn_SimpleReturn(t *testing.T) {
	ind := NewReturn("simple", 1, 100) // Period-to-period return

	prices := []float64{100, 105, 110, 108, 112}
	expectedReturns := []float64{
		0.05,  // (105-100)/100 = 5%
		0.0476, // (110-105)/105 ≈ 4.76%
		-0.0182, // (108-110)/110 ≈ -1.82%
		0.0370,  // (112-108)/108 ≈ 3.70%
	}

	for i, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		ind.Update(md)

		if i > 0 && ind.IsReady() {
			got := ind.GetValue()
			expected := expectedReturns[i-1]
			if math.Abs(got-expected) > 0.001 {
				t.Errorf("Update %d: expected return %f, got %f", i, expected, got)
			}
		}
	}
}

func TestReturn_LogReturn(t *testing.T) {
	ind := NewReturn("log", 1, 100)

	prices := []float64{100, 110, 105}

	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		ind.Update(md)
	}

	// Log return from 110 to 105: ln(105/110)
	expected := math.Log(105.0 / 110.0)
	got := ind.GetValue()

	if math.Abs(got-expected) > 0.0001 {
		t.Errorf("Expected log return %f, got %f", expected, got)
	}
}

func TestReturn_MultiPeriod(t *testing.T) {
	ind := NewReturn("simple", 3, 100) // 3-period return

	prices := []float64{100, 102, 105, 110, 112, 115}

	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		ind.Update(md)
	}

	// Latest return: looking back 3 periods from 115 (index 5) to 105 (index 2)
	// (115-105)/105 = 9.52%
	expected := (115.0 - 105.0) / 105.0
	got := ind.GetValue()

	if math.Abs(got-expected) > 0.001 {
		t.Errorf("Expected 3-period return %f, got %f", expected, got)
	}
}

func TestReturn_CumulativeReturn(t *testing.T) {
	ind := NewReturn("cumulative", 1, 100)

	prices := []float64{100, 110, 121} // +10%, then +10%

	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{price},
			AskPrice: []float64{price},
		}
		ind.Update(md)
	}

	// Cumulative: (1 + 0.1) * (1 + 0.1) - 1 = 0.21 = 21%
	expected := 0.21
	got := ind.GetCumulativeReturn()

	if math.Abs(got-expected) > 0.001 {
		t.Errorf("Expected cumulative return %f, got %f", expected, got)
	}
}

func TestReturn_IsReady(t *testing.T) {
	ind := NewReturn("simple", 5, 100)

	if ind.IsReady() {
		t.Error("Indicator should not be ready initially")
	}

	// Need period+1 prices = 6 prices
	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{100.0 + float64(i)},
		}
		ind.Update(md)
	}

	if ind.IsReady() {
		t.Error("Indicator should not be ready with only 5 prices for period 5")
	}

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{105.0},
		AskPrice: []float64{105.0},
	}
	ind.Update(md)

	if !ind.IsReady() {
		t.Error("Indicator should be ready after 6 prices for period 5")
	}
}

func TestReturn_Reset(t *testing.T) {
	ind := NewReturn("simple", 1, 100)

	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)},
			AskPrice: []float64{100.0 + float64(i)},
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
	if ind.GetCumulativeReturn() != 0 {
		t.Errorf("Expected cumulative return 0 after reset, got %f", ind.GetCumulativeReturn())
	}
}

func TestNewReturnFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"return_type":  "log",
		"period":       float64(5),
		"max_history":  float64(500),
	}

	ind, err := NewReturnFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create Return from config: %v", err)
	}

	ret, ok := ind.(*Return)
	if !ok {
		t.Fatal("Indicator is not of type *Return")
	}

	if ret.GetReturnType() != "log" {
		t.Errorf("Expected return_type 'log', got '%s'", ret.GetReturnType())
	}
	if ret.GetPeriod() != 5 {
		t.Errorf("Expected period 5, got %d", ret.GetPeriod())
	}
}

func BenchmarkReturn_Update(b *testing.B) {
	ind := NewReturn("simple", 1, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ind.Update(md)
	}
}
