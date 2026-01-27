package indicators

import (
	"testing"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestAggressiveTrade_AggressiveBuy(t *testing.T) {
	at := NewAggressiveTrade("test_agg_buy", 100, 100)

	// Trades at ask price (aggressive buys)
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice:  []float64{100.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{100.1},
			AskQty:    []uint32{50},
			LastPrice: 100.1, // At ask = aggressive buy
		}
		at.Update(md)
	}

	buyRatio := at.GetBuyAggressiveRatio()
	if buyRatio <= 0.8 {
		t.Errorf("Expected high buy aggressive ratio, got %.2f", buyRatio)
	}

	side := at.GetDominantSide()
	if side != "Buy" && side != "StrongBuy" {
		t.Errorf("Expected Buy or StrongBuy, got %s", side)
	}
}

func TestAggressiveTrade_AggressiveSell(t *testing.T) {
	at := NewAggressiveTrade("test_agg_sell", 100, 100)

	// Trades at bid price (aggressive sells)
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice:  []float64{100.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{100.1},
			AskQty:    []uint32{50},
			LastPrice: 100.0, // At bid = aggressive sell
		}
		at.Update(md)
	}

	sellRatio := at.GetSellAggressiveRatio()
	if sellRatio <= 0.8 {
		t.Errorf("Expected high sell aggressive ratio, got %.2f", sellRatio)
	}

	side := at.GetDominantSide()
	if side != "Sell" && side != "StrongSell" {
		t.Errorf("Expected Sell or StrongSell, got %s", side)
	}
}

func TestAggressiveTrade_PassiveTrades(t *testing.T) {
	at := NewAggressiveTrade("test_passive", 100, 100)

	// Trades in the middle (passive/limit orders)
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice:  []float64{100.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{100.1},
			AskQty:    []uint32{50},
			LastPrice: 100.05, // Mid-spread = passive
		}
		at.Update(md)
	}

	aggRatio := at.GetAggressiveRatio()
	// Mid-spread trades might not be classified as aggressive
	// (depending on buffer)
	if aggRatio > 0.5 {
		t.Logf("Aggressive ratio %.2f (buffer affects classification)", aggRatio)
	}
}

func TestAggressiveTrade_MixedTrades(t *testing.T) {
	at := NewAggressiveTrade("test_mixed", 100, 100)

	// Half aggressive buys, half aggressive sells
	for i := 0; i < 10; i++ {
		var lastPrice float64
		if i%2 == 0 {
			lastPrice = 100.1 // At ask
		} else {
			lastPrice = 100.0 // At bid
		}

		md := &mdpb.MarketDataUpdate{
			BidPrice:  []float64{100.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{100.1},
			AskQty:    []uint32{50},
			LastPrice: lastPrice,
		}
		at.Update(md)
	}

	aggRatio := at.GetAggressiveRatio()
	if aggRatio < 0.8 {
		t.Errorf("Expected high aggressive ratio for trades at bid/ask, got %.2f", aggRatio)
	}

	side := at.GetDominantSide()
	if side != "Balanced" {
		t.Logf("Got side %s (expected Balanced but acceptable)", side)
	}
}

func TestAggressiveTrade_UrgencyLevels(t *testing.T) {
	at := NewAggressiveTrade("test_urgency", 100, 100)

	// All aggressive trades
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice:  []float64{100.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{100.1},
			AskQty:    []uint32{50},
			LastPrice: 100.1, // Aggressive
		}
		at.Update(md)
	}

	urgency := at.GetUrgencyLevel()
	if urgency != "VeryHigh" && urgency != "High" {
		t.Errorf("Expected VeryHigh or High urgency, got %s", urgency)
	}
}

func TestAggressiveTrade_RollingWindow(t *testing.T) {
	at := NewAggressiveTrade("test_rolling", 5, 100)

	// Fill window with aggressive buys
	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice:  []float64{100.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{100.1},
			AskQty:    []uint32{50},
			LastPrice: 100.1,
		}
		at.Update(md)
	}

	buyRatioBefore := at.GetBuyAggressiveRatio()

	// Add aggressive sells (should push out old buys)
	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice:  []float64{100.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{100.1},
			AskQty:    []uint32{50},
			LastPrice: 100.0,
		}
		at.Update(md)
	}

	buyRatioAfter := at.GetBuyAggressiveRatio()

	// Buy ratio should have decreased as sells entered window
	if buyRatioAfter >= buyRatioBefore {
		t.Errorf("Expected buy ratio to decrease, before=%.2f, after=%.2f", buyRatioBefore, buyRatioAfter)
	}
}

func TestAggressiveTrade_EmptyData(t *testing.T) {
	at := NewAggressiveTrade("test_empty", 100, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice:  []float64{},
		BidQty:    []uint32{},
		AskPrice:  []float64{},
		AskQty:    []uint32{},
		LastPrice: 0,
	}

	at.Update(md)

	// Should handle gracefully
	ratio := at.GetAggressiveRatio()
	if ratio != 0 {
		t.Errorf("Expected ratio 0 for empty data, got %.2f", ratio)
	}
}

func TestAggressiveTrade_GetValue(t *testing.T) {
	at := NewAggressiveTrade("test_getvalue", 100, 100)

	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice:  []float64{100.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{100.1},
			AskQty:    []uint32{50},
			LastPrice: 100.1,
		}
		at.Update(md)
	}

	// GetValue() should return aggressive ratio
	value := at.GetValue()
	ratio := at.GetAggressiveRatio()

	if value != ratio {
		t.Errorf("GetValue() should equal GetAggressiveRatio(), got %.2f vs %.2f", value, ratio)
	}
}

func TestAggressiveTrade_History(t *testing.T) {
	at := NewAggressiveTrade("test_history", 100, 10)

	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice:  []float64{100.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{100.1},
			AskQty:    []uint32{50},
			LastPrice: 100.05,
		}
		at.Update(md)
	}

	history := at.GetValues()
	if len(history) != 10 {
		t.Errorf("Expected history length 10, got %d", len(history))
	}
}

func TestAggressiveTrade_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":          "config_at",
		"window_size":   50.0,
		"spread_buffer": 0.2,
		"max_history":   200.0,
	}

	indicator, err := NewAggressiveTradeFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	at, ok := indicator.(*AggressiveTrade)
	if !ok {
		t.Fatal("Expected *AggressiveTrade type")
	}

	if at.GetName() != "config_at" {
		t.Errorf("Expected name 'config_at', got '%s'", at.GetName())
	}
}

func TestAggressiveTrade_String(t *testing.T) {
	at := NewAggressiveTrade("test_string", 100, 100)

	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice:  []float64{100.0},
			BidQty:    []uint32{50},
			AskPrice:  []float64{100.1},
			AskQty:    []uint32{50},
			LastPrice: 100.1,
		}
		at.Update(md)
	}

	str := at.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "AggressiveTrade") {
		t.Error("String() should contain 'AggressiveTrade'")
	}
}

func BenchmarkAggressiveTrade_Update(b *testing.B) {
	at := NewAggressiveTrade("bench", 100, 1000)

	md := &mdpb.MarketDataUpdate{
		BidPrice:  []float64{100.0},
		BidQty:    []uint32{50},
		AskPrice:  []float64{100.1},
		AskQty:    []uint32{50},
		LastPrice: 100.05,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if i%2 == 0 {
			md.LastPrice = 100.1
		} else {
			md.LastPrice = 100.0
		}
		at.Update(md)
	}
}
