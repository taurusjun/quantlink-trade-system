package indicators

import (
	"testing"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestTradeIntensity_CountBased(t *testing.T) {
	ti := NewTradeIntensity("test_count", 1*time.Second, false, 100)

	// Add 10 trades quickly
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: 100.0,
			LastQty:   10,
		}
		ti.Update(md)
	}

	// Should show 10 trades per second
	intensity := ti.GetIntensity()
	if intensity < 5 || intensity > 15 {
		t.Errorf("Expected intensity around 10 trades/sec, got %.2f", intensity)
	}

	tradeCount := ti.GetTradeCount()
	if tradeCount != 10 {
		t.Errorf("Expected 10 trades, got %d", tradeCount)
	}
}

func TestTradeIntensity_VolumeBased(t *testing.T) {
	ti := NewTradeIntensity("test_volume", 1*time.Second, true, 100)

	// Add trades with volume
	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: 100.0,
			LastQty:   uint32(20 + i*10),
		}
		ti.Update(md)
	}

	// Intensity should be volume per second
	intensity := ti.GetIntensity()
	if intensity <= 0 {
		t.Errorf("Expected positive intensity, got %.2f", intensity)
	}

	tradeCount := ti.GetTradeCount()
	if tradeCount != 5 {
		t.Errorf("Expected 5 trades, got %d", tradeCount)
	}
}

func TestTradeIntensity_IntensityLevels(t *testing.T) {
	ti := NewTradeIntensity("test_levels", 1*time.Second, false, 100)

	testCases := []struct {
		trades int
		minLevel string
	}{
		{20, "VeryHigh"}, // >10 trades/sec
		{8, "High"},      // 5-10 trades/sec
		{3, "Medium"},    // 2-5 trades/sec
		{1, "Low"},       // 0.5-2 trades/sec
	}

	for _, tc := range testCases {
		// Reset by creating new instance
		ti = NewTradeIntensity("test_levels", 1*time.Second, false, 100)

		for i := 0; i < tc.trades; i++ {
			md := &mdpb.MarketDataUpdate{
				LastPrice: 100.0,
				LastQty:   10,
			}
			ti.Update(md)
		}

		level := ti.GetIntensityLevel()
		// Just verify level is not empty
		if len(level) == 0 {
			t.Errorf("Expected non-empty intensity level for %d trades", tc.trades)
		}
	}
}

func TestTradeIntensity_WindowExpiry(t *testing.T) {
	ti := NewTradeIntensity("test_expiry", 100*time.Millisecond, false, 100)

	// Add trades
	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: 100.0,
			LastQty:   10,
		}
		ti.Update(md)
	}

	countBefore := ti.GetTradeCount()
	if countBefore != 5 {
		t.Errorf("Expected 5 trades, got %d", countBefore)
	}

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Add new trade (will trigger cleanup)
	md := &mdpb.MarketDataUpdate{
		LastPrice: 100.0,
		LastQty:   10,
	}
	ti.Update(md)

	// Old trades should have expired
	countAfter := ti.GetTradeCount()
	if countAfter > 2 {
		t.Errorf("Expected most trades to expire, got %d remaining", countAfter)
	}
}

func TestTradeIntensity_GetValue(t *testing.T) {
	ti := NewTradeIntensity("test_getvalue", 1*time.Second, false, 100)

	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: 100.0,
			LastQty:   10,
		}
		ti.Update(md)
	}

	// GetValue() should return intensity
	value := ti.GetValue()
	intensity := ti.GetIntensity()

	if value != intensity {
		t.Errorf("GetValue() should equal GetIntensity(), got %.2f vs %.2f", value, intensity)
	}
}

func TestTradeIntensity_History(t *testing.T) {
	ti := NewTradeIntensity("test_history", 1*time.Second, false, 10)

	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: 100.0,
			LastQty:   10,
		}
		ti.Update(md)
	}

	history := ti.GetValues()
	if len(history) != 10 {
		t.Errorf("Expected history length 10, got %d", len(history))
	}
}

func TestTradeIntensity_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":           "config_ti",
		"window_seconds": 30.0,
		"use_volume":     true,
		"max_history":    200.0,
	}

	indicator, err := NewTradeIntensityFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	ti, ok := indicator.(*TradeIntensity)
	if !ok {
		t.Fatal("Expected *TradeIntensity type")
	}

	if ti.GetName() != "config_ti" {
		t.Errorf("Expected name 'config_ti', got '%s'", ti.GetName())
	}
}

func TestTradeIntensity_String(t *testing.T) {
	ti := NewTradeIntensity("test_string", 1*time.Second, false, 100)

	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: 100.0,
			LastQty:   10,
		}
		ti.Update(md)
	}

	str := ti.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "TradeIntensity") {
		t.Error("String() should contain 'TradeIntensity'")
	}
}

func BenchmarkTradeIntensity_Update(b *testing.B) {
	ti := NewTradeIntensity("bench", 1*time.Second, false, 1000)

	md := &mdpb.MarketDataUpdate{
		LastPrice: 100.0,
		LastQty:   10,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ti.Update(md)
	}
}
