package indicators

import (
	"math"
	"testing"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestVWAP_Basic(t *testing.T) {
	lib := NewIndicatorLibrary()
	config := map[string]interface{}{
		"period":      20.0,
		"max_history": 100.0,
	}

	ind, err := lib.Create("test_vwap", "vwap", config)
	if err != nil {
		t.Fatalf("Failed to create VWAP: %v", err)
	}

	// Should not be ready initially
	if ind.IsReady() {
		t.Error("VWAP should not be ready initially")
	}

	// Update with data
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			Symbol:      "TEST",
			Exchange:    "TEST",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{100.0},
			BidQty:      []uint32{100},
			AskPrice:    []float64{100.5},
			AskQty:      []uint32{100},
			LastPrice:   100.0 + float64(i)*0.1,
			TotalVolume: uint64(1000 + i*100),
			Turnover:    (100.0 + float64(i)*0.1) * float64(1000+i*100),
		}
		ind.Update(md)
	}

	// Should be ready now
	if !ind.IsReady() {
		t.Error("VWAP should be ready after period updates")
	}

	// Check value is reasonable
	value := ind.GetValue()
	if value < 100.0 || value > 102.0 {
		t.Errorf("VWAP value %f out of reasonable range", value)
	}
}

func TestVWAP_Calculation(t *testing.T) {
	lib := NewIndicatorLibrary()
	config := map[string]interface{}{
		"period":      5.0,
		"max_history": 100.0,
	}

	ind, err := lib.Create("test_calc", "vwap", config)
	if err != nil {
		t.Fatalf("Failed to create VWAP: %v", err)
	}

	// Add 5 data points with known values
	prices := []float64{100.0, 101.0, 102.0, 103.0, 104.0}
	volumes := []uint64{1000, 2000, 1500, 2500, 2000}

	var totalPV, totalV float64
	for i, price := range prices {
		volume := volumes[i]
		totalPV += price * float64(volume)
		totalV += float64(volume)

		md := &mdpb.MarketDataUpdate{
			Symbol:      "TEST",
			Exchange:    "TEST",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{price},
			BidQty:      []uint32{100},
			AskPrice:    []float64{price + 0.5},
			AskQty:      []uint32{100},
			LastPrice:   price,
			TotalVolume: volume,
			Turnover:    price * float64(volume),
		}
		ind.Update(md)
	}

	expectedVWAP := totalPV / totalV
	actualVWAP := ind.GetValue()

	if math.Abs(actualVWAP-expectedVWAP) > 0.01 {
		t.Errorf("Expected VWAP %.2f, got %.2f", expectedVWAP, actualVWAP)
	}
}

func TestVWAP_Reset(t *testing.T) {
	lib := NewIndicatorLibrary()
	config := map[string]interface{}{
		"period":      10.0,
		"max_history": 100.0,
	}

	ind, err := lib.Create("test_reset", "vwap", config)
	if err != nil {
		t.Fatalf("Failed to create VWAP: %v", err)
	}

	// Update with data
	for i := 0; i < 15; i++ {
		md := &mdpb.MarketDataUpdate{
			Symbol:      "TEST",
			Exchange:    "TEST",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{100.0},
			BidQty:      []uint32{100},
			AskPrice:    []float64{100.5},
			AskQty:      []uint32{100},
			LastPrice:   100.25,
			TotalVolume: 1000,
			Turnover:    100250.0,
		}
		ind.Update(md)
	}

	if !ind.IsReady() {
		t.Error("VWAP should be ready")
	}

	// Reset
	ind.Reset()

	if ind.IsReady() {
		t.Error("VWAP should not be ready after reset")
	}
}

func BenchmarkVWAP_Update(b *testing.B) {
	lib := NewIndicatorLibrary()
	config := map[string]interface{}{
		"period":      20.0,
		"max_history": 100.0,
	}

	ind, _ := lib.Create("bench_vwap", "vwap", config)
	md := &mdpb.MarketDataUpdate{
		Symbol:      "TEST",
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{100.0},
		BidQty:      []uint32{100},
		AskPrice:    []float64{100.5},
		AskQty:      []uint32{100},
		LastPrice:   100.25,
		TotalVolume: 1000,
		Turnover:    100250.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ind.Update(md)
	}
}
