package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestLiquidityScore_ExcellentLiquidity(t *testing.T) {
	ls := NewLiquidityScore("test_excellent", 5, 100)

	// High depth, tight spread
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:   []uint32{200, 180, 160, 140, 120}, // Total 800
		AskPrice: []float64{100.01, 100.02, 100.03, 100.04, 100.05},
		AskQty:   []uint32{200, 180, 160, 140, 120}, // Total 800
	}

	ls.Update(md)

	score := ls.GetScore()
	if score < 80 {
		t.Errorf("Expected excellent liquidity score (>80), got %.2f", score)
	}

	level := ls.GetLevel()
	if level != "Excellent" && level != "Good" {
		t.Errorf("Expected Excellent or Good level, got %s", level)
	}
}

func TestLiquidityScore_PoorLiquidity(t *testing.T) {
	ls := NewLiquidityScore("test_poor", 5, 100)

	// Low depth, wide spread
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
		BidQty:   []uint32{5, 4, 3, 2, 1}, // Total 15
		AskPrice: []float64{101.0, 101.1, 101.2, 101.3, 101.4},
		AskQty:   []uint32{5, 4, 3, 2, 1}, // Total 15
	}

	ls.Update(md)

	score := ls.GetScore()
	if score > 50 {
		t.Errorf("Expected poor liquidity score (<50), got %.2f", score)
	}

	level := ls.GetLevel()
	if level == "Excellent" || level == "Good" {
		t.Errorf("Expected lower level, got %s", level)
	}
}

func TestLiquidityScore_ScoreRange(t *testing.T) {
	ls := NewLiquidityScore("test_range", 5, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{50, 40, 30},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{50, 40, 30},
	}

	ls.Update(md)

	score := ls.GetScore()
	if score < 0 || score > 100 {
		t.Errorf("Score should be in range [0, 100], got %.2f", score)
	}
}

func TestLiquidityScore_EmptyData(t *testing.T) {
	ls := NewLiquidityScore("test_empty", 5, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{},
		BidQty:   []uint32{},
		AskPrice: []float64{},
		AskQty:   []uint32{},
	}

	ls.Update(md)

	score := ls.GetScore()
	if score != 0 {
		t.Errorf("Expected score 0 for empty data, got %.2f", score)
	}

	level := ls.GetLevel()
	if level != "VeryPoor" {
		t.Errorf("Expected VeryPoor level, got %s", level)
	}
}

func TestLiquidityScore_CustomWeights(t *testing.T) {
	config := map[string]interface{}{
		"name":          "test_weights",
		"levels":        5.0,
		"depth_weight":  0.7,
		"spread_weight": 0.3,
		"max_history":   100.0,
	}

	indicator, err := NewLiquidityScoreFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	ls, ok := indicator.(*LiquidityScore)
	if !ok {
		t.Fatal("Expected *LiquidityScore type")
	}

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{100, 80, 60},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{100, 80, 60},
	}

	ls.Update(md)

	// Should use custom weights
	score := ls.GetScore()
	if score < 0 || score > 100 {
		t.Errorf("Score should be in valid range, got %.2f", score)
	}
}

func TestLiquidityScore_LevelClassification(t *testing.T) {
	ls := NewLiquidityScore("test_levels", 5, 100)

	testCases := []struct {
		score    float64
		expected string
	}{
		{90, "Excellent"},
		{70, "Good"},
		{50, "Fair"},
		{30, "Poor"},
		{10, "VeryPoor"},
	}

	for _, tc := range testCases {
		// Manually set score for testing
		ls.mu.Lock()
		ls.lastScore = tc.score
		ls.mu.Unlock()

		level := ls.GetLevel()
		if level != tc.expected {
			t.Errorf("Score %.0f: expected level %s, got %s", tc.score, tc.expected, level)
		}
	}
}

func TestLiquidityScore_History(t *testing.T) {
	ls := NewLiquidityScore("test_history", 5, 10)

	for i := 0; i < 15; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0},
			BidQty:   []uint32{uint32(50 + i*10)},
			AskPrice: []float64{100.1},
			AskQty:   []uint32{uint32(50 + i*10)},
		}
		ls.Update(md)
	}

	history := ls.GetValues()
	if len(history) != 10 {
		t.Errorf("Expected history length 10, got %d", len(history))
	}
}

func TestLiquidityScore_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":        "config_liq",
		"levels":      3.0,
		"max_history": 200.0,
	}

	indicator, err := NewLiquidityScoreFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	ls, ok := indicator.(*LiquidityScore)
	if !ok {
		t.Fatal("Expected *LiquidityScore type")
	}

	if ls.GetName() != "config_liq" {
		t.Errorf("Expected name 'config_liq', got '%s'", ls.GetName())
	}
}

func TestLiquidityScore_String(t *testing.T) {
	ls := NewLiquidityScore("test_string", 5, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0},
		BidQty:   []uint32{100},
		AskPrice: []float64{100.1},
		AskQty:   []uint32{100},
	}
	ls.Update(md)

	str := ls.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "LiquidityScore") {
		t.Error("String() should contain 'LiquidityScore'")
	}
}

func BenchmarkLiquidityScore_Update(b *testing.B) {
	ls := NewLiquidityScore("bench", 10, 1000)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6, 99.5, 99.4, 99.3, 99.2, 99.1},
		BidQty:   []uint32{100, 90, 80, 70, 60, 50, 40, 30, 20, 10},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5, 100.6, 100.7, 100.8, 100.9, 101.0},
		AskQty:   []uint32{100, 90, 80, 70, 60, 50, 40, 30, 20, 10},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ls.Update(md)
	}
}
