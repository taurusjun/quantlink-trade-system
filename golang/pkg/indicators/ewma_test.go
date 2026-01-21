package indicators

import (
	"math"
	"testing"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestEWMA_Basic(t *testing.T) {
	lib := NewIndicatorLibrary()
	config := map[string]interface{}{
		"period":      20.0,
		"max_history": 100.0,
	}

	ind, err := lib.Create("test_ewma", "ewma", config)
	if err != nil {
		t.Fatalf("Failed to create EWMA: %v", err)
	}

	if ind.GetName() != "EWMA" {
		t.Errorf("Expected name 'EWMA', got '%s'", ind.GetName())
	}

	// Should not be ready initially
	if ind.IsReady() {
		t.Error("EWMA should not be ready initially")
	}

	// Update with data
	for i := 0; i < 20; i++ {
		md := createTestMarketDataWithPrice(100.0 + float64(i))
		ind.Update(md)
	}

	// Should be ready now
	if !ind.IsReady() {
		t.Error("EWMA should be ready after 20 updates")
	}

	// Check value is reasonable
	value := ind.GetValue()
	if value < 100.0 || value > 120.0 {
		t.Errorf("EWMA value %f out of reasonable range", value)
	}
}

func TestEWMA_ConvergesToPrice(t *testing.T) {
	lib := NewIndicatorLibrary()
	config := map[string]interface{}{
		"period":      20.0,
		"max_history": 100.0,
	}

	ind, err := lib.Create("test_converge", "ewma", config)
	if err != nil {
		t.Fatalf("Failed to create EWMA: %v", err)
	}

	// Feed constant price
	constantPrice := 100.0
	for i := 0; i < 100; i++ {
		md := createTestMarketDataWithPrice(constantPrice)
		ind.Update(md)
	}

	// EWMA should converge to the constant price
	value := ind.GetValue()
	if math.Abs(value-constantPrice) > 1.0 {
		t.Errorf("EWMA should converge to %.2f, got %.2f", constantPrice, value)
	}
}

func TestEWMA_Reset(t *testing.T) {
	lib := NewIndicatorLibrary()
	config := map[string]interface{}{
		"period":      20.0,
		"max_history": 100.0,
	}

	ind, err := lib.Create("test_reset", "ewma", config)
	if err != nil {
		t.Fatalf("Failed to create EWMA: %v", err)
	}

	// Update with data
	for i := 0; i < 20; i++ {
		md := createTestMarketDataWithPrice(100.0)
		ind.Update(md)
	}

	if !ind.IsReady() {
		t.Error("EWMA should be ready")
	}

	// Reset
	ind.Reset()

	// Check reset worked
	if ind.IsReady() {
		t.Error("EWMA should not be ready after reset")
	}
}

func BenchmarkEWMA_Update(b *testing.B) {
	lib := NewIndicatorLibrary()
	config := map[string]interface{}{
		"period":      20.0,
		"max_history": 100.0,
	}

	ind, _ := lib.Create("bench_ewma", "ewma", config)
	md := createTestMarketDataWithPrice(100.0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ind.Update(md)
	}
}

func createTestMarketDataWithPrice(price float64) *mdpb.MarketDataUpdate {
	return &mdpb.MarketDataUpdate{
		Symbol:      "TEST",
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{price},
		BidQty:      []uint32{100},
		AskPrice:    []float64{price + 0.5},
		AskQty:      []uint32{100},
		LastPrice:   price + 0.25,
		TotalVolume: 1000,
		Turnover:    (price + 0.25) * 1000.0,
	}
}
