package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestAvgBookSize_Creation(t *testing.T) {
	ind := NewAvgBookSize(20, 5, 1000)
	if ind == nil {
		t.Fatal("Failed to create AvgBookSize indicator")
	}
	if ind.GetName() != "AvgBookSize" {
		t.Errorf("Expected name 'AvgBookSize', got '%s'", ind.GetName())
	}
	if ind.GetPeriod() != 20 {
		t.Errorf("Expected period 20, got %d", ind.GetPeriod())
	}
	if ind.GetNumLevels() != 5 {
		t.Errorf("Expected numLevels 5, got %d", ind.GetNumLevels())
	}
}

func TestAvgBookSize_IsReady(t *testing.T) {
	ind := NewAvgBookSize(5, 5, 100)
	if ind.IsReady() {
		t.Error("Indicator should not be ready initially")
	}

	// Add less than period updates
	for i := 0; i < 4; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0},
			AskPrice: []float64{100.5},
			BidQty:   []uint32{10},
			AskQty:   []uint32{15},
		}
		ind.Update(md)
	}

	if ind.IsReady() {
		t.Error("Indicator should not be ready with less than period updates")
	}

	// Add one more to complete the period
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{100.5},
		BidQty:   []uint32{10},
		AskQty:   []uint32{15},
	}
	ind.Update(md)

	if !ind.IsReady() {
		t.Error("Indicator should be ready after period updates")
	}
}

func TestAvgBookSize_Calculation(t *testing.T) {
	ind := NewAvgBookSize(3, 2, 100) // 3-period average, top 2 levels

	// First update: BidQty=[10,20], AskQty=[15,25] -> BookSize = 70
	md1 := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9},
		AskPrice: []float64{100.1, 100.2},
		BidQty:   []uint32{10, 20},
		AskQty:   []uint32{15, 25},
	}
	ind.Update(md1)

	// After 1 update, avg = 70
	if ind.GetValue() != 70.0 {
		t.Errorf("Expected average 70, got %f", ind.GetValue())
	}

	// Second update: BidQty=[12,18], AskQty=[14,26] -> BookSize = 70
	md2 := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9},
		AskPrice: []float64{100.1, 100.2},
		BidQty:   []uint32{12, 18},
		AskQty:   []uint32{14, 26},
	}
	ind.Update(md2)

	// After 2 updates, avg = (70+70)/2 = 70
	if ind.GetValue() != 70.0 {
		t.Errorf("Expected average 70, got %f", ind.GetValue())
	}

	// Third update: BidQty=[15,15], AskQty=[20,20] -> BookSize = 70
	md3 := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9},
		AskPrice: []float64{100.1, 100.2},
		BidQty:   []uint32{15, 15},
		AskQty:   []uint32{20, 20},
	}
	ind.Update(md3)

	// After 3 updates, avg = (70+70+70)/3 = 70
	expected := 70.0
	if ind.GetValue() != expected {
		t.Errorf("Expected average %f, got %f", expected, ind.GetValue())
	}
}

func TestAvgBookSize_RollingWindow(t *testing.T) {
	ind := NewAvgBookSize(3, 1, 100) // 3-period average, top 1 level

	updates := []struct {
		bidQty uint32
		askQty uint32
		bookSize float64
	}{
		{10, 10, 20.0},
		{20, 20, 40.0},
		{30, 30, 60.0},
		{40, 40, 80.0},
	}

	expectedAvgs := []float64{
		20.0,                    // (20) / 1
		30.0,                    // (20+40) / 2
		40.0,                    // (20+40+60) / 3
		60.0,                    // (40+60+80) / 3 -- first value dropped
	}

	for i, u := range updates {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0},
			AskPrice: []float64{100.1},
			BidQty:   []uint32{u.bidQty},
			AskQty:   []uint32{u.askQty},
		}
		ind.Update(md)

		if ind.GetValue() != expectedAvgs[i] {
			t.Errorf("Update %d: expected average %f, got %f",
				i, expectedAvgs[i], ind.GetValue())
		}
	}
}

func TestAvgBookSize_AllLevels(t *testing.T) {
	ind := NewAvgBookSize(2, 0, 100) // numLevels=0 means all levels

	// First update with 3 levels
	md1 := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		AskPrice: []float64{100.1, 100.2, 100.3},
		BidQty:   []uint32{10, 20, 30},
		AskQty:   []uint32{15, 25, 35},
	}
	ind.Update(md1)

	// BookSize = 10+20+30+15+25+35 = 135
	expected1 := 135.0
	if ind.GetValue() != expected1 {
		t.Errorf("Expected average %f, got %f", expected1, ind.GetValue())
	}

	// Second update with different quantities
	md2 := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		AskPrice: []float64{100.1, 100.2, 100.3},
		BidQty:   []uint32{5, 10, 15},
		AskQty:   []uint32{8, 12, 20},
	}
	ind.Update(md2)

	// BookSize = 5+10+15+8+12+20 = 70
	// Average = (135+70)/2 = 102.5
	expected2 := 102.5
	if ind.GetValue() != expected2 {
		t.Errorf("Expected average %f, got %f", expected2, ind.GetValue())
	}
}

func TestAvgBookSize_Reset(t *testing.T) {
	ind := NewAvgBookSize(3, 2, 100)

	// Add some updates
	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0},
			AskPrice: []float64{100.1},
			BidQty:   []uint32{10},
			AskQty:   []uint32{10},
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

func TestNewAvgBookSizeFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      float64(10),
		"num_levels":  float64(3),
		"max_history": float64(500),
	}

	ind, err := NewAvgBookSizeFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create indicator from config: %v", err)
	}

	abs, ok := ind.(*AvgBookSize)
	if !ok {
		t.Fatal("Indicator is not of type *AvgBookSize")
	}

	if abs.GetPeriod() != 10 {
		t.Errorf("Expected period 10, got %d", abs.GetPeriod())
	}
	if abs.GetNumLevels() != 3 {
		t.Errorf("Expected numLevels 3, got %d", abs.GetNumLevels())
	}
}

func TestNewAvgBookSizeFromConfig_Defaults(t *testing.T) {
	config := map[string]interface{}{}

	ind, err := NewAvgBookSizeFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create indicator from config: %v", err)
	}

	abs, ok := ind.(*AvgBookSize)
	if !ok {
		t.Fatal("Indicator is not of type *AvgBookSize")
	}

	if abs.GetPeriod() != 20 {
		t.Errorf("Expected default period 20, got %d", abs.GetPeriod())
	}
	if abs.GetNumLevels() != 5 {
		t.Errorf("Expected default numLevels 5, got %d", abs.GetNumLevels())
	}
}

func BenchmarkAvgBookSize_Update(b *testing.B) {
	ind := NewAvgBookSize(20, 5, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		BidQty:   []uint32{10, 20, 30, 40, 50},
		AskQty:   []uint32{15, 25, 35, 45, 55},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ind.Update(md)
	}
}

func BenchmarkAvgBookSize_FullCalculation(b *testing.B) {
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		BidQty:   []uint32{10, 20, 30, 40, 50},
		AskQty:   []uint32{15, 25, 35, 45, 55},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ind := NewAvgBookSize(20, 5, 1000)
		for j := 0; j < 100; j++ {
			ind.Update(md)
		}
		_ = ind.GetValue()
	}
}
