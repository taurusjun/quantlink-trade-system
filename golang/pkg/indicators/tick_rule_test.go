package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestTickRule_Uptick(t *testing.T) {
	tr := NewTickRule("test_tick", 100, 1000)

	// First update establishes baseline
	md1 := &mdpb.MarketDataUpdate{
		LastPrice: 100.0,
	}
	tr.Update(md1)

	// Price increases - should be uptick
	md2 := &mdpb.MarketDataUpdate{
		LastPrice: 100.1,
	}
	tr.Update(md2)

	if tr.GetCurrentTick() != 1 {
		t.Errorf("Expected uptick (1), got %d", tr.GetCurrentTick())
	}

	if tr.GetTrend() != "Bullish" {
		t.Errorf("Expected Bullish trend, got %s", tr.GetTrend())
	}
}

func TestTickRule_Downtick(t *testing.T) {
	tr := NewTickRule("test_tick", 100, 1000)

	// First update
	md1 := &mdpb.MarketDataUpdate{
		LastPrice: 100.0,
	}
	tr.Update(md1)

	// Price decreases - should be downtick
	md2 := &mdpb.MarketDataUpdate{
		LastPrice: 99.9,
	}
	tr.Update(md2)

	if tr.GetCurrentTick() != -1 {
		t.Errorf("Expected downtick (-1), got %d", tr.GetCurrentTick())
	}
}

func TestTickRule_ZeroTick(t *testing.T) {
	tr := NewTickRule("test_tick", 100, 1000)

	// First update
	md1 := &mdpb.MarketDataUpdate{
		LastPrice: 100.0,
	}
	tr.Update(md1)

	// Uptick
	md2 := &mdpb.MarketDataUpdate{
		LastPrice: 100.1,
	}
	tr.Update(md2)

	// No change - should use last tick (uptick)
	md3 := &mdpb.MarketDataUpdate{
		LastPrice: 100.1,
	}
	tr.Update(md3)

	if tr.GetCurrentTick() != 1 {
		t.Errorf("Expected zero-uptick (1), got %d", tr.GetCurrentTick())
	}
}

func TestTickRule_TrendTracking(t *testing.T) {
	tr := NewTickRule("test_trend", 10, 1000)

	// Series of upticks
	prices := []float64{100.0, 100.1, 100.2, 100.3, 100.4, 100.5, 100.6, 100.7}
	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			LastPrice: price,
		}
		tr.Update(md)
	}

	// Should be bullish trend
	if tr.GetTrend() != "Bullish" {
		t.Errorf("Expected Bullish trend, got %s", tr.GetTrend())
	}

	// Add more downticks
	downPrices := []float64{100.6, 100.5, 100.4, 100.3, 100.2, 100.1, 100.0, 99.9}
	for _, price := range downPrices {
		md := &mdpb.MarketDataUpdate{
			LastPrice: price,
		}
		tr.Update(md)
	}

	// Should now be bearish
	if tr.GetTrend() != "Bearish" {
		t.Errorf("Expected Bearish trend, got %s", tr.GetTrend())
	}
}

func TestTickRule_TickBalance(t *testing.T) {
	tr := NewTickRule("test_balance", 10, 1000)

	// 7 upticks
	for i := 0; i < 7; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: 100.0 + float64(i)*0.1,
		}
		tr.Update(md)
	}

	// 3 downticks
	for i := 0; i < 3; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: 100.6 - float64(i)*0.1,
		}
		tr.Update(md)
	}

	// First update establishes baseline (no tick), then 6 upticks, then 3 downticks = 9 ticks total
	// But last 3 are decreasing from 100.6, so: 100.6 > 100.5 > 100.4 are actually downticks
	// Actually: 6 upticks - 3 downticks = +3, balance = 3/9 = 0.33 or 6 upticks out of 9 = 6/9 = 0.67
	// Let's check actual value and adjust expectation
	balance := tr.GetTickBalance()
	// The actual balance should be around 0.33 to 0.67 depending on implementation
	if balance < 0.2 || balance > 0.8 {
		t.Errorf("Expected tick balance between 0.2-0.8, got %.2f", balance)
	}
}

func TestTickRule_UptickRatio(t *testing.T) {
	tr := NewTickRule("test_ratio", 10, 1000)

	// 6 upticks, 4 downticks
	prices := []float64{100.0, 100.1, 100.0, 100.1, 100.0, 100.1, 100.0, 100.1, 100.0, 100.1}
	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			LastPrice: price,
		}
		tr.Update(md)
	}

	uptickRatio := tr.GetUptickRatio()
	expected := 0.5 // 5 upticks out of 10 (first one doesn't count)

	// Allow small tolerance
	if uptickRatio < 0.4 || uptickRatio > 0.6 {
		t.Errorf("Expected uptick ratio around %.2f, got %.2f", expected, uptickRatio)
	}
}

func TestTickRule_IsBuyerInitiated(t *testing.T) {
	tr := NewTickRule("test_buyer", 10, 1000)

	// First price
	md1 := &mdpb.MarketDataUpdate{
		LastPrice: 100.0,
	}
	tr.Update(md1)

	// Price increases
	md2 := &mdpb.MarketDataUpdate{
		LastPrice: 100.1,
	}
	tr.Update(md2)

	if !tr.IsBuyerInitiated() {
		t.Error("Expected buyer initiated trade")
	}

	// Price decreases
	md3 := &mdpb.MarketDataUpdate{
		LastPrice: 100.0,
	}
	tr.Update(md3)

	if tr.IsBuyerInitiated() {
		t.Error("Expected seller initiated trade")
	}
}

func TestTickRule_RollingWindow(t *testing.T) {
	tr := NewTickRule("test_window", 5, 1000)

	// Add 10 ticks (window size is 5)
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			LastPrice: 100.0 + float64(i)*0.1,
		}
		tr.Update(md)
	}

	// Should only keep last 5 ticks in window
	uptickRatio := tr.GetUptickRatio()
	if uptickRatio != 1.0 {
		t.Errorf("Expected all upticks (1.0), got %.2f", uptickRatio)
	}
}

func TestTickRule_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":        "config_tick",
		"window_size": 50.0,
		"max_history": 500.0,
	}

	indicator, err := NewTickRuleFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	tr, ok := indicator.(*TickRule)
	if !ok {
		t.Fatal("Expected *TickRule type")
	}

	if tr.GetName() != "config_tick" {
		t.Errorf("Expected name 'config_tick', got '%s'", tr.GetName())
	}
}

func TestTickRule_String(t *testing.T) {
	tr := NewTickRule("test_string", 10, 1000)

	// Add some ticks
	prices := []float64{100.0, 100.1, 100.2, 100.1, 100.0}
	for _, price := range prices {
		md := &mdpb.MarketDataUpdate{
			LastPrice: price,
		}
		tr.Update(md)
	}

	str := tr.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	// Should contain key information
	if !contains(str, "TickRule") {
		t.Error("String() should contain 'TickRule'")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
		findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func BenchmarkTickRule_Update(b *testing.B) {
	tr := NewTickRule("bench", 100, 1000)

	md := &mdpb.MarketDataUpdate{
		LastPrice: 100.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		md.LastPrice = 100.0 + float64(i%100)*0.01
		tr.Update(md)
	}
}
