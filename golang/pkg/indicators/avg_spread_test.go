package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestAvgSpread_Creation(t *testing.T) {
	ind := NewAvgSpread(20, "absolute", 1000)
	if ind == nil {
		t.Fatal("Failed to create AvgSpread indicator")
	}
	if ind.GetName() != "AvgSpread" {
		t.Errorf("Expected name 'AvgSpread', got '%s'", ind.GetName())
	}
}

func TestAvgSpread_AbsoluteCalculation(t *testing.T) {
	ind := NewAvgSpread(3, "absolute", 100)

	updates := []struct {
		bid      float64
		ask      float64
		expected float64
	}{
		{100.0, 100.5, 0.5},           // (0.5) / 1 = 0.5
		{100.0, 101.0, 0.75},          // (0.5+1.0) / 2 = 0.75
		{100.0, 100.8, 0.7666666666666667}, // (0.5+1.0+0.8) / 3 = 0.767
	}

	for i, u := range updates {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{u.bid},
			AskPrice: []float64{u.ask},
		}
		ind.Update(md)

		if got := ind.GetValue(); math.Abs(got-u.expected) > 0.0001 {
			t.Errorf("Update %d: expected %f, got %f", i, u.expected, got)
		}
	}
}

func TestAvgSpread_PercentageCalculation(t *testing.T) {
	ind := NewAvgSpread(2, "percentage", 100)

	// Bid=100, Ask=101 -> Spread=1, Mid=100.5 -> Percentage = 1/100.5 * 100 = 0.995%
	md1 := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{101.0},
	}
	ind.Update(md1)

	expected1 := (1.0 / 100.5) * 100.0
	if got := ind.GetValue(); math.Abs(got-expected1) > 0.001 {
		t.Errorf("Expected %f, got %f", expected1, got)
	}

	// Bid=100, Ask=100.5 -> Spread=0.5, Mid=100.25 -> Percentage = 0.5/100.25 * 100 = 0.499%
	md2 := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{100.5},
	}
	ind.Update(md2)

	// Average of the two
	pct1 := (1.0 / 100.5) * 100.0
	pct2 := (0.5 / 100.25) * 100.0
	expected2 := (pct1 + pct2) / 2.0

	if got := ind.GetValue(); math.Abs(got-expected2) > 0.001 {
		t.Errorf("Expected %f, got %f", expected2, got)
	}
}

func TestAvgSpread_BpsCalculation(t *testing.T) {
	ind := NewAvgSpread(2, "bps", 100)

	// Bid=100, Ask=100.1 -> Spread=0.1, Mid=100.05 -> bps = 0.1/100.05 * 10000 = 9.995 bps
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{100.1},
	}
	ind.Update(md)

	expected := (0.1 / 100.05) * 10000.0
	if got := ind.GetValue(); math.Abs(got-expected) > 0.01 {
		t.Errorf("Expected %f bps, got %f bps", expected, got)
	}
}

func TestAvgSpread_RollingWindow(t *testing.T) {
	ind := NewAvgSpread(3, "absolute", 100)

	spreads := []float64{0.5, 1.0, 0.8, 1.2}
	expectedAvgs := []float64{
		0.5,                       // (0.5) / 1
		0.75,                      // (0.5+1.0) / 2
		0.7666666666666667,        // (0.5+1.0+0.8) / 3
		1.0,                       // (1.0+0.8+1.2) / 3 - first dropped
	}

	for i, spread := range spreads {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0},
			AskPrice: []float64{100.0 + spread},
		}
		ind.Update(md)

		if got := ind.GetValue(); math.Abs(got-expectedAvgs[i]) > 0.0001 {
			t.Errorf("Update %d: expected %f, got %f", i, expectedAvgs[i], got)
		}
	}
}

func TestAvgSpread_InvalidData(t *testing.T) {
	ind := NewAvgSpread(3, "absolute", 100)

	// Missing bid price
	md1 := &mdpb.MarketDataUpdate{
		AskPrice: []float64{100.5},
	}
	ind.Update(md1)

	if ind.GetValue() != 0 {
		t.Errorf("Expected 0 with missing bid, got %f", ind.GetValue())
	}

	// Negative spread (ask < bid)
	md2 := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.5},
		AskPrice: []float64{100.0},
	}
	ind.Update(md2)

	if ind.GetValue() != 0 {
		t.Errorf("Expected 0 with negative spread, got %f", ind.GetValue())
	}
}

func TestAvgSpread_Reset(t *testing.T) {
	ind := NewAvgSpread(3, "absolute", 100)

	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0},
			AskPrice: []float64{100.5},
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

func TestNewAvgSpreadFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(10),
		"spread_type": "percentage",
		"max_history": float64(500),
	}

	ind, err := NewAvgSpreadFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create indicator from config: %v", err)
	}

	as, ok := ind.(*AvgSpread)
	if !ok {
		t.Fatal("Indicator is not of type *AvgSpread")
	}

	if as.GetPeriod() != 10 {
		t.Errorf("Expected period 10, got %d", as.GetPeriod())
	}
	if as.GetSpreadType() != "percentage" {
		t.Errorf("Expected type 'percentage', got '%s'", as.GetSpreadType())
	}
}

func BenchmarkAvgSpread_Update(b *testing.B) {
	ind := NewAvgSpread(20, "absolute", 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{100.5},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ind.Update(md)
	}
}
