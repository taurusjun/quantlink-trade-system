package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestResilienceScore_DepthRecovery(t *testing.T) {
	rs := NewResilienceScore("test_recovery", 3, 5, 100)

	// Depth decreases then recovers
	updates := []struct {
		bidQty []uint32
		askQty []uint32
	}{
		{[]uint32{100, 90, 80}, []uint32{100, 90, 80}}, // Total 540
		{[]uint32{80, 70, 60}, []uint32{80, 70, 60}},   // Total 420 (decrease)
		{[]uint32{90, 80, 70}, []uint32{90, 80, 70}},   // Total 480 (recovery)
		{[]uint32{95, 85, 75}, []uint32{95, 85, 75}},   // Total 510 (more recovery)
		{[]uint32{100, 90, 80}, []uint32{100, 90, 80}}, // Total 540 (full recovery)
	}

	for _, upd := range updates {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   upd.bidQty,
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   upd.askQty,
		}
		rs.Update(md)
	}

	// Should show positive recovery rate
	recoveryRate := rs.GetDepthRecoveryRate()
	if recoveryRate <= 0 {
		t.Errorf("Expected positive depth recovery rate, got %.2f", recoveryRate)
	}
}

func TestResilienceScore_SpreadRecovery(t *testing.T) {
	rs := NewResilienceScore("test_spread", 3, 5, 100)

	// Spread widens then narrows
	updates := []struct {
		askPrice float64
	}{
		{100.1},  // Spread = 0.1
		{100.2},  // Spread = 0.2 (widens)
		{100.15}, // Spread = 0.15 (narrows)
		{100.12}, // Spread = 0.12 (more narrowing)
		{100.1},  // Spread = 0.1 (back to original)
	}

	for _, upd := range updates {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{50, 50, 50},
			AskPrice: []float64{upd.askPrice, upd.askPrice + 0.1, upd.askPrice + 0.2},
			AskQty:   []uint32{50, 50, 50},
		}
		rs.Update(md)
	}

	// Should show positive spread recovery rate
	recoveryRate := rs.GetSpreadRecoveryRate()
	if recoveryRate <= 0 {
		t.Errorf("Expected positive spread recovery rate, got %.4f", recoveryRate)
	}
}

func TestResilienceScore_StableOrderbook(t *testing.T) {
	rs := NewResilienceScore("test_stable", 3, 10, 100)

	// Stable orderbook with minimal changes
	for i := 0; i < 15; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{100, 90, 80}, // Consistent
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{100, 90, 80}, // Consistent
		}
		rs.Update(md)
	}

	// Stable orderbook should have high stability score
	score := rs.GetStabilityScore()
	if score < 70 {
		t.Errorf("Expected high stability score for stable orderbook, got %.2f", score)
	}
}

func TestResilienceScore_VolatileOrderbook(t *testing.T) {
	rs := NewResilienceScore("test_volatile", 3, 5, 100)

	// Highly volatile depth changes
	depths := [][]uint32{
		{100, 90, 80},
		{50, 40, 30},
		{150, 130, 110},
		{60, 50, 40},
		{120, 100, 80},
	}

	for _, depth := range depths {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   depth,
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   depth,
		}
		rs.Update(md)
	}

	// Volatile orderbook should have lower stability score
	score := rs.GetStabilityScore()
	if score > 80 {
		t.Errorf("Expected lower stability score for volatile orderbook, got %.2f", score)
	}
}

func TestResilienceScore_ScoreRange(t *testing.T) {
	rs := NewResilienceScore("test_range", 5, 10, 100)

	for i := 0; i < 15; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
			BidQty:   []uint32{uint32(50 + i*5), 40, 30, 20, 10},
			AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
			AskQty:   []uint32{uint32(50 + i*5), 40, 30, 20, 10},
		}
		rs.Update(md)
	}

	score := rs.GetStabilityScore()
	if score < 0 || score > 100 {
		t.Errorf("Score should be in range [0, 100], got %.2f", score)
	}
}

func TestResilienceScore_EmptyData(t *testing.T) {
	rs := NewResilienceScore("test_empty", 5, 10, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{},
		BidQty:   []uint32{},
		AskPrice: []float64{},
		AskQty:   []uint32{},
	}

	rs.Update(md)

	// Should handle gracefully
	score := rs.GetStabilityScore()
	if score != 0 {
		t.Errorf("Expected score 0 for first empty update, got %.2f", score)
	}
}

func TestResilienceScore_GetValue(t *testing.T) {
	rs := NewResilienceScore("test_getvalue", 3, 5, 100)

	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{50, 50, 50},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{50, 50, 50},
		}
		rs.Update(md)
	}

	// GetValue() should return stability score
	value := rs.GetValue()
	score := rs.GetStabilityScore()

	if value != score {
		t.Errorf("GetValue() should equal GetStabilityScore(), got %.2f vs %.2f", value, score)
	}
}

func TestResilienceScore_History(t *testing.T) {
	rs := NewResilienceScore("test_history", 3, 5, 10)

	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{uint32(50 + i), 40, 30},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{uint32(50 + i), 40, 30},
		}
		rs.Update(md)
	}

	history := rs.GetValues()
	if len(history) != 10 {
		t.Errorf("Expected history length 10, got %d", len(history))
	}
}

func TestResilienceScore_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":        "config_resilience",
		"levels":      5.0,
		"window_size": 15.0,
		"max_history": 200.0,
		"alpha":       0.3,
	}

	indicator, err := NewResilienceScoreFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	rs, ok := indicator.(*ResilienceScore)
	if !ok {
		t.Fatal("Expected *ResilienceScore type")
	}

	if rs.GetName() != "config_resilience" {
		t.Errorf("Expected name 'config_resilience', got '%s'", rs.GetName())
	}
}

func TestResilienceScore_String(t *testing.T) {
	rs := NewResilienceScore("test_string", 3, 5, 100)

	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{50, 50, 50},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{50, 50, 50},
		}
		rs.Update(md)
	}

	str := rs.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "ResilienceScore") {
		t.Error("String() should contain 'ResilienceScore'")
	}
}

func BenchmarkResilienceScore_Update(b *testing.B) {
	rs := NewResilienceScore("bench", 10, 20, 1000)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6, 99.5, 99.4, 99.3, 99.2, 99.1},
		BidQty:   []uint32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5, 100.6, 100.7, 100.8, 100.9, 101.0},
		AskQty:   []uint32{15, 25, 35, 45, 55, 65, 75, 85, 95, 105},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		md.BidQty[0] = uint32(10 + (i % 50))
		rs.Update(md)
	}
}
