package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestAvgBidQty_Creation(t *testing.T) {
	ind := NewAvgBidQty(20, 5, 1000)
	if ind == nil {
		t.Fatal("Failed to create AvgBidQty indicator")
	}
	if ind.GetName() != "AvgBidQty" {
		t.Errorf("Expected name 'AvgBidQty', got '%s'", ind.GetName())
	}
}

func TestAvgBidQty_Calculation(t *testing.T) {
	ind := NewAvgBidQty(3, 2, 100) // 3-period average, top 2 levels

	updates := []struct {
		bidQty  []uint32
		expected float64
	}{
		{[]uint32{10, 20}, 30.0},  // (30) / 1 = 30
		{[]uint32{20, 30}, 40.0},  // (30+50) / 2 = 40
		{[]uint32{30, 40}, 50.0},  // (30+50+70) / 3 = 50
	}

	for i, u := range updates {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9},
			BidQty:   u.bidQty,
		}
		ind.Update(md)

		if got := ind.GetValue(); got != u.expected {
			t.Errorf("Update %d: expected %f, got %f", i, u.expected, got)
		}
	}
}

func TestAvgAskQty_Creation(t *testing.T) {
	ind := NewAvgAskQty(20, 5, 1000)
	if ind == nil {
		t.Fatal("Failed to create AvgAskQty indicator")
	}
	if ind.GetName() != "AvgAskQty" {
		t.Errorf("Expected name 'AvgAskQty', got '%s'", ind.GetName())
	}
}

func TestAvgAskQty_Calculation(t *testing.T) {
	ind := NewAvgAskQty(2, 2, 100) // 2-period average, top 2 levels

	updates := []struct {
		askQty  []uint32
		expected float64
	}{
		{[]uint32{15, 25}, 40.0},  // (40) / 1 = 40
		{[]uint32{20, 30}, 45.0},  // (40+50) / 2 = 45
		{[]uint32{25, 35}, 55.0},  // (50+60) / 2 = 55 (first dropped)
	}

	for i, u := range updates {
		md := &mdpb.MarketDataUpdate{
			AskPrice: []float64{100.1, 100.2},
			AskQty:   u.askQty,
		}
		ind.Update(md)

		if got := ind.GetValue(); got != u.expected {
			t.Errorf("Update %d: expected %f, got %f", i, u.expected, got)
		}
	}
}

func TestAvgBidQty_Reset(t *testing.T) {
	ind := NewAvgBidQty(3, 2, 100)

	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0},
			BidQty:   []uint32{10},
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

func TestAvgAskQty_Reset(t *testing.T) {
	ind := NewAvgAskQty(3, 2, 100)

	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			AskPrice: []float64{100.1},
			AskQty:   []uint32{15},
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

func BenchmarkAvgBidQty_Update(b *testing.B) {
	ind := NewAvgBidQty(20, 5, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:   []uint32{10, 20, 30, 40, 50},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ind.Update(md)
	}
}

func BenchmarkAvgAskQty_Update(b *testing.B) {
	ind := NewAvgAskQty(20, 5, 1000)
	md := &mdpb.MarketDataUpdate{
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		AskQty:   []uint32{15, 25, 35, 45, 55},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ind.Update(md)
	}
}
