package indicators

import (
	"testing"
	"time"

	mdpb "github.com/yourusername/hft-poc/pkg/proto/md"
)

// TestIndicatorLibrary tests the indicator library
func TestIndicatorLibrary(t *testing.T) {
	lib := NewIndicatorLibrary()

	// Test Create
	config := map[string]interface{}{
		"period":      20.0,
		"max_history": 100.0,
	}
	ind, err := lib.Create("test_ewma", "ewma", config)
	if err != nil {
		t.Fatalf("Failed to create indicator: %v", err)
	}
	if ind == nil {
		t.Fatal("Created indicator is nil")
	}

	// Test Get
	retrieved, ok := lib.Get("test_ewma")
	if !ok {
		t.Fatal("Failed to get indicator")
	}
	if retrieved.GetName() != "EWMA" {
		t.Errorf("Expected name 'EWMA', got '%s'", retrieved.GetName())
	}

	// Test UpdateAll
	md := createTestMarketData()
	lib.UpdateAll(md)

	// Test GetAllValues
	values := lib.GetAllValues()
	if len(values) != 1 {
		t.Errorf("Expected 1 value, got %d", len(values))
	}
}

func TestIndicatorFactory(t *testing.T) {
	lib := NewIndicatorLibrary()

	// Test factory registration
	factories := []string{"ewma", "order_imbalance", "vwap", "spread", "volatility"}
	for _, name := range factories {
		_, err := lib.Create("test_"+name, name, map[string]interface{}{
			"period":      20.0,
			"max_history": 100.0,
		})
		if err != nil {
			t.Errorf("Failed to create %s indicator: %v", name, err)
		}
	}

	values := lib.GetAllValues()
	if len(values) != len(factories) {
		t.Errorf("Expected %d indicators, got %d", len(factories), len(values))
	}
}

func TestIndicatorReset(t *testing.T) {
	lib := NewIndicatorLibrary()

	config := map[string]interface{}{
		"period":      20.0,
		"max_history": 100.0,
	}
	ind, err := lib.Create("test_reset", "ewma", config)
	if err != nil {
		t.Fatalf("Failed to create indicator: %v", err)
	}

	// Update with data
	md := createTestMarketData()
	ind.Update(md)

	// Reset
	ind.Reset()

	// Check if reset worked
	if ind.IsReady() {
		t.Error("Indicator should not be ready after reset")
	}
}

func TestConcurrentAccess(t *testing.T) {
	lib := NewIndicatorLibrary()

	// Create indicators
	for i := 0; i < 5; i++ {
		name := "test_concurrent_" + string(rune('0'+i))
		_, err := lib.Create(name, "ewma", map[string]interface{}{
			"period":      20.0,
			"max_history": 100.0,
		})
		if err != nil {
			t.Fatalf("Failed to create indicator %s: %v", name, err)
		}
	}

	// Concurrent updates
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			md := createTestMarketData()
			lib.UpdateAll(md)
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify all indicators still exist
	values := lib.GetAllValues()
	if len(values) != 5 {
		t.Errorf("Expected 5 indicators, got %d", len(values))
	}
}

// Helper function to create test market data
func createTestMarketData() *mdpb.MarketDataUpdate {
	return &mdpb.MarketDataUpdate{
		Symbol:      "test",
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{100.0, 99.5, 99.0, 98.5, 98.0},
		BidQty:      []uint32{100, 80, 60, 40, 20},
		AskPrice:    []float64{100.5, 101.0, 101.5, 102.0, 102.5},
		AskQty:      []uint32{90, 70, 50, 30, 10},
		LastPrice:   100.25,
		TotalVolume: 1000,
		Turnover:    100250.0,
	}
}
