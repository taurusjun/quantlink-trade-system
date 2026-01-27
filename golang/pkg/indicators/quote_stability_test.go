package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestQuoteStability_StableQuotes(t *testing.T) {
	qs := NewQuoteStability("test_stable", 10, 100)

	// Stable quotes (no changes)
	for i := 0; i < 15; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{50, 40, 30},
		}
		qs.Update(md)
	}

	score := qs.GetStabilityScore()
	if score < 70 {
		t.Errorf("Expected high stability score for stable quotes, got %.2f", score)
	}

	level := qs.GetStabilityLevel()
	if level != "VeryStable" && level != "Stable" {
		t.Errorf("Expected VeryStable or Stable, got %s", level)
	}

	freq := qs.GetChangeFrequency()
	if freq != 0 {
		t.Errorf("Expected change frequency 0, got %.2f", freq)
	}
}

func TestQuoteStability_VolatileQuotes(t *testing.T) {
	qs := NewQuoteStability("test_volatile", 10, 100)

	// Volatile quotes (frequent changes)
	prices := []float64{100.0, 100.1, 100.05, 100.15, 100.02, 100.12, 100.08, 100.18}
	for _, bidPrice := range prices {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{bidPrice, bidPrice - 0.1, bidPrice - 0.2},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{bidPrice + 0.1, bidPrice + 0.2, bidPrice + 0.3},
			AskQty:   []uint32{50, 40, 30},
		}
		qs.Update(md)
	}

	score := qs.GetStabilityScore()
	if score > 60 {
		t.Errorf("Expected low stability score for volatile quotes, got %.2f", score)
	}

	level := qs.GetStabilityLevel()
	if level == "VeryStable" || level == "Stable" {
		t.Errorf("Expected lower stability level, got %s", level)
	}

	freq := qs.GetChangeFrequency()
	if freq == 0 {
		t.Error("Expected non-zero change frequency for volatile quotes")
	}
}

func TestQuoteStability_BidStability(t *testing.T) {
	qs := NewQuoteStability("test_bid_stable", 10, 100)

	// Stable bids, volatile asks
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8}, // Stable
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1 + float64(i)*0.05, 100.2, 100.3}, // Volatile
			AskQty:   []uint32{50, 40, 30},
		}
		qs.Update(md)
	}

	bidStability := qs.GetBidStability()
	askStability := qs.GetAskStability()

	if bidStability <= askStability {
		t.Errorf("Expected bid stability (%.2f) > ask stability (%.2f)", bidStability, askStability)
	}
}

func TestQuoteStability_AskStability(t *testing.T) {
	qs := NewQuoteStability("test_ask_stable", 10, 100)

	// Volatile bids, stable asks
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)*0.05, 99.9, 99.8}, // Volatile
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.2, 100.3}, // Stable
			AskQty:   []uint32{50, 40, 30},
		}
		qs.Update(md)
	}

	bidStability := qs.GetBidStability()
	askStability := qs.GetAskStability()

	if askStability <= bidStability {
		t.Errorf("Expected ask stability (%.2f) > bid stability (%.2f)", askStability, bidStability)
	}
}

func TestQuoteStability_ScoreRange(t *testing.T) {
	qs := NewQuoteStability("test_range", 10, 100)

	for i := 0; i < 15; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i%3)*0.1, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1 + float64(i%3)*0.1, 100.2, 100.3},
			AskQty:   []uint32{50, 40, 30},
		}
		qs.Update(md)
	}

	score := qs.GetStabilityScore()
	if score < 0 || score > 100 {
		t.Errorf("Score should be in range [0, 100], got %.2f", score)
	}

	bidScore := qs.GetBidStability()
	if bidScore < 0 || bidScore > 100 {
		t.Errorf("Bid score should be in range [0, 100], got %.2f", bidScore)
	}

	askScore := qs.GetAskStability()
	if askScore < 0 || askScore > 100 {
		t.Errorf("Ask score should be in range [0, 100], got %.2f", askScore)
	}
}

func TestQuoteStability_EmptyData(t *testing.T) {
	qs := NewQuoteStability("test_empty", 10, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{},
		BidQty:   []uint32{},
		AskPrice: []float64{},
		AskQty:   []uint32{},
	}

	qs.Update(md)

	// Should handle gracefully
	score := qs.GetStabilityScore()
	if score != 0 {
		t.Errorf("Expected score 0 for first empty update, got %.2f", score)
	}
}

func TestQuoteStability_GetValue(t *testing.T) {
	qs := NewQuoteStability("test_getvalue", 10, 100)

	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{50, 40, 30},
		}
		qs.Update(md)
	}

	// GetValue() should return stability score
	value := qs.GetValue()
	score := qs.GetStabilityScore()

	if value != score {
		t.Errorf("GetValue() should equal GetStabilityScore(), got %.2f vs %.2f", value, score)
	}
}

func TestQuoteStability_History(t *testing.T) {
	qs := NewQuoteStability("test_history", 10, 10)

	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{50, 40, 30},
		}
		qs.Update(md)
	}

	history := qs.GetValues()
	if len(history) != 10 {
		t.Errorf("Expected history length 10, got %d", len(history))
	}
}

func TestQuoteStability_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":        "config_qs",
		"window_size": 50.0,
		"max_history": 200.0,
	}

	indicator, err := NewQuoteStabilityFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	qs, ok := indicator.(*QuoteStability)
	if !ok {
		t.Fatal("Expected *QuoteStability type")
	}

	if qs.GetName() != "config_qs" {
		t.Errorf("Expected name 'config_qs', got '%s'", qs.GetName())
	}
}

func TestQuoteStability_String(t *testing.T) {
	qs := NewQuoteStability("test_string", 10, 100)

	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{50, 40, 30},
		}
		qs.Update(md)
	}

	str := qs.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "QuoteStability") {
		t.Error("String() should contain 'QuoteStability'")
	}
}

func BenchmarkQuoteStability_Update(b *testing.B) {
	qs := NewQuoteStability("bench", 100, 1000)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{50, 40, 30},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{50, 40, 30},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		md.BidPrice[0] = 100.0 + float64(i%10)*0.01
		qs.Update(md)
	}
}
