package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestBBD_Creation(t *testing.T) {
	ind := NewBBD("mid", 0, 1000)
	if ind == nil {
		t.Fatal("Failed to create BBD indicator")
	}
	if ind.GetName() != "BBD" {
		t.Errorf("Expected name 'BBD', got '%s'", ind.GetName())
	}
}

func TestBBD_Calculation(t *testing.T) {
	ind := NewBBD("mid", 0, 100)

	// Bid=100, Ask=101 -> Mid=100.5 -> BBD = 100.5 - 100 = 0.5
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{101.0},
	}
	ind.Update(md)

	expected := 0.5
	if ind.GetValue() != expected {
		t.Errorf("Expected BBD %f, got %f", expected, ind.GetValue())
	}
}

func TestBBD_FixedReference(t *testing.T) {
	ind := NewBBD("fixed", 105.0, 100)

	// Bid=100, Ask=101, Fixed=105 -> BBD = 105 - 100 = 5.0
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{101.0},
	}
	ind.Update(md)

	expected := 5.0
	if ind.GetValue() != expected {
		t.Errorf("Expected BBD %f, got %f", expected, ind.GetValue())
	}
}

func TestBAD_Creation(t *testing.T) {
	ind := NewBAD("mid", 0, 1000)
	if ind == nil {
		t.Fatal("Failed to create BAD indicator")
	}
	if ind.GetName() != "BAD" {
		t.Errorf("Expected name 'BAD', got '%s'", ind.GetName())
	}
}

func TestBAD_Calculation(t *testing.T) {
	ind := NewBAD("mid", 0, 100)

	// Bid=100, Ask=101 -> Mid=100.5 -> BAD = 101 - 100.5 = 0.5
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{101.0},
	}
	ind.Update(md)

	expected := 0.5
	if ind.GetValue() != expected {
		t.Errorf("Expected BAD %f, got %f", expected, ind.GetValue())
	}
}

func TestBAD_FixedReference(t *testing.T) {
	ind := NewBAD("fixed", 98.0, 100)

	// Bid=100, Ask=101, Fixed=98 -> BAD = 101 - 98 = 3.0
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{101.0},
	}
	ind.Update(md)

	expected := 3.0
	if ind.GetValue() != expected {
		t.Errorf("Expected BAD %f, got %f", expected, ind.GetValue())
	}
}

func TestBBD_Reset(t *testing.T) {
	ind := NewBBD("mid", 0, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{101.0},
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

func BenchmarkBBD_Update(b *testing.B) {
	ind := NewBBD("mid", 0, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{101.0},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ind.Update(md)
	}
}

func BenchmarkBAD_Update(b *testing.B) {
	ind := NewBAD("mid", 0, 1000)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		AskPrice: []float64{101.0},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ind.Update(md)
	}
}
