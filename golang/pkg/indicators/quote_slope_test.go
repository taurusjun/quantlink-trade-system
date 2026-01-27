package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestQuoteSlope_BasicCalculation(t *testing.T) {
	qs := NewQuoteSlope("test_slope", 5, 100)

	// Linear price decline on bids
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.8, 99.6, 99.4, 99.2}, // Price change = 0.8
		BidQty:   []uint32{10, 10, 10, 10, 10},            // Total depth = 50
		AskPrice: []float64{100.1, 100.3, 100.5, 100.7, 100.9}, // Price change = 0.8
		AskQty:   []uint32{10, 10, 10, 10, 10},            // Total depth = 50
	}

	qs.Update(md)

	// Slope = price change / depth = 0.8 / 50 = 0.016
	expectedSlope := 0.016
	bidSlope := qs.GetBidSlope()

	if bidSlope < expectedSlope-0.001 || bidSlope > expectedSlope+0.001 {
		t.Errorf("Expected bid slope around %.6f, got %.6f", expectedSlope, bidSlope)
	}

	askSlope := qs.GetAskSlope()
	if askSlope < expectedSlope-0.001 || askSlope > expectedSlope+0.001 {
		t.Errorf("Expected ask slope around %.6f, got %.6f", expectedSlope, askSlope)
	}

	// Average slope should be similar
	avgSlope := qs.GetAvgSlope()
	if avgSlope < expectedSlope-0.001 || avgSlope > expectedSlope+0.001 {
		t.Errorf("Expected avg slope around %.6f, got %.6f", expectedSlope, avgSlope)
	}
}

func TestQuoteSlope_SteepOrderbook(t *testing.T) {
	qs := NewQuoteSlope("test_steep", 3, 100)

	// Large price changes, small depth = steep slope
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.5, 99.0}, // Price change = 1.0
		BidQty:   []uint32{5, 5, 5},           // Total depth = 15
		AskPrice: []float64{100.1, 100.6, 101.1}, // Price change = 1.0
		AskQty:   []uint32{5, 5, 5},           // Total depth = 15
	}

	qs.Update(md)

	// Steep slope = 1.0 / 15 = 0.0667
	expectedSlope := 0.0666666667
	avgSlope := qs.GetAvgSlope()

	if avgSlope < expectedSlope-0.001 || avgSlope > expectedSlope+0.001 {
		t.Errorf("Expected steep slope around %.6f, got %.6f", expectedSlope, avgSlope)
	}
}

func TestQuoteSlope_FlatOrderbook(t *testing.T) {
	qs := NewQuoteSlope("test_flat", 3, 100)

	// Small price changes, large depth = flat slope
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.95, 99.90}, // Price change = 0.1
		BidQty:   []uint32{100, 100, 100},       // Total depth = 300
		AskPrice: []float64{100.01, 100.06, 100.11}, // Price change = 0.1
		AskQty:   []uint32{100, 100, 100},       // Total depth = 300
	}

	qs.Update(md)

	// Flat slope = 0.1 / 300 = 0.000333
	expectedSlope := 0.000333333
	avgSlope := qs.GetAvgSlope()

	if avgSlope < expectedSlope-0.0001 || avgSlope > expectedSlope+0.0001 {
		t.Errorf("Expected flat slope around %.6f, got %.6f", expectedSlope, avgSlope)
	}
}

func TestQuoteSlope_EmptyData(t *testing.T) {
	qs := NewQuoteSlope("test_empty", 5, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{},
		BidQty:   []uint32{},
		AskPrice: []float64{},
		AskQty:   []uint32{},
	}

	qs.Update(md)

	if qs.GetBidSlope() != 0 {
		t.Errorf("Expected bid slope 0, got %.6f", qs.GetBidSlope())
	}

	if qs.GetAskSlope() != 0 {
		t.Errorf("Expected ask slope 0, got %.6f", qs.GetAskSlope())
	}

	if qs.GetAvgSlope() != 0 {
		t.Errorf("Expected avg slope 0, got %.6f", qs.GetAvgSlope())
	}
}

func TestQuoteSlope_InsufficientLevels(t *testing.T) {
	qs := NewQuoteSlope("test_insufficient", 5, 100)

	// Only 1 level
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		BidQty:   []uint32{10},
		AskPrice: []float64{100.1},
		AskQty:   []uint32{10},
	}

	qs.Update(md)

	// Should be 0 (need at least 2 levels)
	if qs.GetBidSlope() != 0 {
		t.Errorf("Expected bid slope 0 for single level, got %.6f", qs.GetBidSlope())
	}
}

func TestQuoteSlope_GetValue(t *testing.T) {
	qs := NewQuoteSlope("test_getvalue", 3, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{20, 20, 20},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{20, 20, 20},
	}

	qs.Update(md)

	// GetValue() should return average slope
	value := qs.GetValue()
	avgSlope := qs.GetAvgSlope()

	if value != avgSlope {
		t.Errorf("GetValue() should equal GetAvgSlope(), got %.6f vs %.6f", value, avgSlope)
	}
}

func TestQuoteSlope_History(t *testing.T) {
	qs := NewQuoteSlope("test_history", 3, 10)

	for i := 0; i < 15; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{20, 20, 20},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{20, 20, 20},
		}
		qs.Update(md)
	}

	history := qs.GetValues()
	if len(history) != 10 {
		t.Errorf("Expected history length 10, got %d", len(history))
	}
}

func TestQuoteSlope_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":        "config_slope",
		"levels":      7.0,
		"max_history": 500.0,
	}

	indicator, err := NewQuoteSlopeFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	qs, ok := indicator.(*QuoteSlope)
	if !ok {
		t.Fatal("Expected *QuoteSlope type")
	}

	if qs.GetName() != "config_slope" {
		t.Errorf("Expected name 'config_slope', got '%s'", qs.GetName())
	}
}

func TestQuoteSlope_String(t *testing.T) {
	qs := NewQuoteSlope("test_string", 5, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{30, 30, 30},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{30, 30, 30},
	}
	qs.Update(md)

	str := qs.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "QuoteSlope") {
		t.Error("String() should contain 'QuoteSlope'")
	}
}

func BenchmarkQuoteSlope_Update(b *testing.B) {
	qs := NewQuoteSlope("bench", 10, 1000)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6, 99.5, 99.4, 99.3, 99.2, 99.1},
		BidQty:   []uint32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5, 100.6, 100.7, 100.8, 100.9, 101.0},
		AskQty:   []uint32{15, 25, 35, 45, 55, 65, 75, 85, 95, 105},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		qs.Update(md)
	}
}
