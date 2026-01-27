package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestCumulativeVolume_Basic(t *testing.T) {
	cv := NewCumulativeVolume("test_cumvol", false, 100)

	// Test initial state
	if cv.GetCumulativeVolume() != 0 {
		t.Errorf("Expected initial volume 0, got %d", cv.GetCumulativeVolume())
	}

	// First update
	md1 := &mdpb.MarketDataUpdate{
		TotalVolume: 100,
	}
	cv.Update(md1)

	if cv.GetCumulativeVolume() != 100 {
		t.Errorf("Expected cumulative volume 100, got %d", cv.GetCumulativeVolume())
	}

	// Second update - volume increases
	md2 := &mdpb.MarketDataUpdate{
		TotalVolume: 250,
	}
	cv.Update(md2)

	// Delta = 250 - 100 = 150, cumulative = 100 + 150 = 250
	expected := int64(250)
	if cv.GetCumulativeVolume() != expected {
		t.Errorf("Expected cumulative volume %d, got %d", expected, cv.GetCumulativeVolume())
	}
}

func TestCumulativeVolume_Reset(t *testing.T) {
	cv := NewCumulativeVolume("test_reset", false, 100)

	// Add some volume
	md := &mdpb.MarketDataUpdate{
		TotalVolume: 500,
	}
	cv.Update(md)

	if cv.GetCumulativeVolume() == 0 {
		t.Error("Expected non-zero volume before reset")
	}

	// Reset
	cv.Reset()

	if cv.GetCumulativeVolume() != 0 {
		t.Errorf("Expected volume 0 after reset, got %d", cv.GetCumulativeVolume())
	}
}

func TestCumulativeVolume_OvernightReset(t *testing.T) {
	cv := NewCumulativeVolume("test_overnight", false, 100)

	// First update
	md1 := &mdpb.MarketDataUpdate{
		TotalVolume: 1000,
	}
	cv.Update(md1)

	// Simulate overnight reset (total volume goes back to small number)
	md2 := &mdpb.MarketDataUpdate{
		TotalVolume: 50,
	}
	cv.Update(md2)

	// Should handle negative delta gracefully by using current volume
	expected := int64(1050) // 1000 + 50
	if cv.GetCumulativeVolume() != expected {
		t.Errorf("Expected cumulative volume %d, got %d", expected, cv.GetCumulativeVolume())
	}
}

func TestCumulativeVolume_ZeroVolume(t *testing.T) {
	cv := NewCumulativeVolume("test_zero", false, 100)

	md := &mdpb.MarketDataUpdate{
		TotalVolume: 0,
	}
	cv.Update(md)

	if cv.GetCumulativeVolume() != 0 {
		t.Errorf("Expected volume 0, got %d", cv.GetCumulativeVolume())
	}
}

func TestCumulativeVolume_GetValue(t *testing.T) {
	cv := NewCumulativeVolume("test_getvalue", false, 100)

	md := &mdpb.MarketDataUpdate{
		TotalVolume: 123,
	}
	cv.Update(md)

	value := cv.GetValue()
	expected := 123.0

	if value != expected {
		t.Errorf("Expected GetValue() %.0f, got %.0f", expected, value)
	}
}

func TestCumulativeVolume_History(t *testing.T) {
	cv := NewCumulativeVolume("test_history", false, 10)

	// Add multiple updates
	for i := 1; i <= 15; i++ {
		md := &mdpb.MarketDataUpdate{
			TotalVolume: uint64(i * 100),
		}
		cv.Update(md)
	}

	// Should only keep last 10 values
	history := cv.GetValues()
	if len(history) != 10 {
		t.Errorf("Expected history length 10, got %d", len(history))
	}

	// Last value should be cumulative of all
	lastValue := history[len(history)-1]
	expected := 1500.0 // Sum of deltas: 100+100+100...+100 (15 times)

	if lastValue != expected {
		t.Errorf("Expected last value %.0f, got %.0f", expected, lastValue)
	}
}

func TestCumulativeVolume_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":              "config_cumvol",
		"reset_on_new_bar":  true,
		"max_history":       500.0,
	}

	indicator, err := NewCumulativeVolumeFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	cv, ok := indicator.(*CumulativeVolume)
	if !ok {
		t.Fatal("Expected *CumulativeVolume type")
	}

	if cv.GetName() != "config_cumvol" {
		t.Errorf("Expected name 'config_cumvol', got '%s'", cv.GetName())
	}
}

func TestCumulativeVolume_String(t *testing.T) {
	cv := NewCumulativeVolume("test_string", false, 100)

	md := &mdpb.MarketDataUpdate{
		TotalVolume: 456,
	}
	cv.Update(md)

	str := cv.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "CumulativeVolume") {
		t.Error("String() should contain 'CumulativeVolume'")
	}
}

func BenchmarkCumulativeVolume_Update(b *testing.B) {
	cv := NewCumulativeVolume("bench", false, 1000)

	md := &mdpb.MarketDataUpdate{
		TotalVolume: 1000,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		md.TotalVolume = uint64(1000 + i)
		cv.Update(md)
	}
}
