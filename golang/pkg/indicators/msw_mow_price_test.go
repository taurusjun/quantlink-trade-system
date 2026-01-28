package indicators

import (
	"math"
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestMSWPrice_Creation(t *testing.T) {
	ind := NewMSWPrice(5, 1000)
	if ind == nil {
		t.Fatal("Failed to create MSWPrice indicator")
	}
	if ind.GetName() != "MSWPrice" {
		t.Errorf("Expected name 'MSWPrice', got '%s'", ind.GetName())
	}
}

func TestMSWPrice_Calculation(t *testing.T) {
	ind := NewMSWPrice(2, 100)

	// Simple case: Bid=[100,99], BidQty=[10,20], Ask=[101,102], AskQty=[15,25]
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.0},
		BidQty:   []uint32{10, 20},
		AskPrice: []float64{101.0, 102.0},
		AskQty:   []uint32{15, 25},
	}
	ind.Update(md)

	// Calculate expected MSW price manually:
	// Bid side: price=100, vol=10, weight=100*10=1000, weighted_sum=100*1000=100000
	//          price=99, vol=20, weight=99*20=1980, weighted_sum=99*1980=196020
	// Ask side: price=101, vol=15, weight=101*15=1515, weighted_sum=101*1515=153015
	//          price=102, vol=25, weight=102*25=2550, weighted_sum=102*2550=260100
	// Total weighted_sum = 100000+196020+153015+260100 = 709135
	// Total weight = 1000+1980+1515+2550 = 7045
	// MSW = 709135 / 7045 = 100.65...

	expected := 709135.0 / 7045.0

	if math.Abs(ind.GetValue()-expected) > 0.01 {
		t.Errorf("Expected MSW price %f, got %f", expected, ind.GetValue())
	}
}

func TestMOWPrice_Creation(t *testing.T) {
	ind := NewMOWPrice(5, 1000)
	if ind == nil {
		t.Fatal("Failed to create MOWPrice indicator")
	}
	if ind.GetName() != "MOWPrice" {
		t.Errorf("Expected name 'MOWPrice', got '%s'", ind.GetName())
	}
}

func TestMOWPrice_Calculation(t *testing.T) {
	ind := NewMOWPrice(2, 100)

	// Simple case: Bid=[100,99], BidQty=[10,20], Ask=[101,102], AskQty=[15,25]
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.0},
		BidQty:   []uint32{10, 20},
		AskPrice: []float64{101.0, 102.0},
		AskQty:   []uint32{15, 25},
	}
	ind.Update(md)

	// Calculate expected MOW price manually:
	// Bid side: price=100, vol=10, weighted_sum=100*10=1000
	//          price=99, vol=20, weighted_sum=99*20=1980
	// Ask side: price=101, vol=15, weighted_sum=101*15=1515
	//          price=102, vol=25, weighted_sum=102*25=2550
	// Total weighted_sum = 1000+1980+1515+2550 = 7045
	// Total volume = 10+20+15+25 = 70
	// MOW = 7045 / 70 = 100.642857...

	expected := 7045.0 / 70.0

	if math.Abs(ind.GetValue()-expected) > 0.01 {
		t.Errorf("Expected MOW price %f, got %f", expected, ind.GetValue())
	}
}

func TestMSWPrice_Reset(t *testing.T) {
	ind := NewMSWPrice(2, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		BidQty:   []uint32{10},
		AskPrice: []float64{101.0},
		AskQty:   []uint32{15},
	}
	ind.Update(md)

	if ind.GetValue() == 0 {
		t.Error("Value should not be 0 before reset")
	}

	ind.Reset()

	if ind.GetValue() != 0 {
		t.Errorf("Expected value 0 after reset, got %f", ind.GetValue())
	}
}

func TestMOWPrice_Reset(t *testing.T) {
	ind := NewMOWPrice(2, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		BidQty:   []uint32{10},
		AskPrice: []float64{101.0},
		AskQty:   []uint32{15},
	}
	ind.Update(md)

	if ind.GetValue() == 0 {
		t.Error("Value should not be 0 before reset")
	}

	ind.Reset()

	if ind.GetValue() != 0 {
		t.Errorf("Expected value 0 after reset, got %f", ind.GetValue())
	}
}

func BenchmarkMSWPrice_Update(b *testing.B) {
	ind := NewMSWPrice(5, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:   []uint32{10, 20, 30, 40, 50},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		AskQty:   []uint32{15, 25, 35, 45, 55},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ind.Update(md)
	}
}

func BenchmarkMOWPrice_Update(b *testing.B) {
	ind := NewMOWPrice(5, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:   []uint32{10, 20, 30, 40, 50},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		AskQty:   []uint32{15, 25, 35, 45, 55},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ind.Update(md)
	}
}
