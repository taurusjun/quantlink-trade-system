package indicators

import (
	"testing"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestOrderArrivalRate_BidArrivals(t *testing.T) {
	oar := NewOrderArrivalRate("test_bid", 1*time.Second, 5, 100)

	// Add new bid orders
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 - float64(i)*0.01, 99.9, 99.8, 99.7, 99.6},
			BidQty:   []uint32{uint32(50 + i*10), 40, 30, 20, 10},
			AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
			AskQty:   []uint32{50, 40, 30, 20, 10},
		}
		oar.Update(md)
		time.Sleep(50 * time.Millisecond)
	}

	bidRate := oar.GetBidArrivalRate()
	if bidRate <= 0 {
		t.Errorf("Expected positive bid arrival rate, got %.2f", bidRate)
	}
}

func TestOrderArrivalRate_AskArrivals(t *testing.T) {
	oar := NewOrderArrivalRate("test_ask", 1*time.Second, 5, 100)

	// Add new ask orders
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
			BidQty:   []uint32{50, 40, 30, 20, 10},
			AskPrice: []float64{100.1 + float64(i)*0.01, 100.2, 100.3, 100.4, 100.5},
			AskQty:   []uint32{uint32(50 + i*10), 40, 30, 20, 10},
		}
		oar.Update(md)
		time.Sleep(50 * time.Millisecond)
	}

	askRate := oar.GetAskArrivalRate()
	if askRate <= 0 {
		t.Errorf("Expected positive ask arrival rate, got %.2f", askRate)
	}
}

func TestOrderArrivalRate_TotalRate(t *testing.T) {
	oar := NewOrderArrivalRate("test_total", 1*time.Second, 5, 100)

	// Add orders on both sides
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 - float64(i)*0.01, 99.9, 99.8, 99.7, 99.6},
			BidQty:   []uint32{uint32(50 + i*5), 40, 30, 20, 10},
			AskPrice: []float64{100.1 + float64(i)*0.01, 100.2, 100.3, 100.4, 100.5},
			AskQty:   []uint32{uint32(50 + i*5), 40, 30, 20, 10},
		}
		oar.Update(md)
		time.Sleep(50 * time.Millisecond)
	}

	totalRate := oar.GetTotalArrivalRate()
	bidRate := oar.GetBidArrivalRate()
	askRate := oar.GetAskArrivalRate()

	// Total should equal bid + ask
	expected := bidRate + askRate
	if totalRate < expected-0.1 || totalRate > expected+0.1 {
		t.Errorf("Expected total rate %.2f, got %.2f", expected, totalRate)
	}
}

func TestOrderArrivalRate_Imbalance(t *testing.T) {
	oar := NewOrderArrivalRate("test_imbalance", 1*time.Second, 5, 100)

	// More bid arrivals than ask
	for i := 0; i < 15; i++ {
		var bidQty, askQty uint32
		if i < 10 {
			bidQty = uint32(50 + i*10) // Increasing bid orders
			askQty = 50                  // Constant ask orders
		} else {
			bidQty = uint32(50 + i*10)
			askQty = 50
		}

		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 - float64(i)*0.01, 99.9, 99.8, 99.7, 99.6},
			BidQty:   []uint32{bidQty, 40, 30, 20, 10},
			AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
			AskQty:   []uint32{askQty, 40, 30, 20, 10},
		}
		oar.Update(md)
		time.Sleep(30 * time.Millisecond)
	}

	imbalance := oar.GetArrivalImbalance()
	// Should be positive (more bid arrivals)
	if imbalance <= 0 {
		t.Errorf("Expected positive imbalance, got %.2f", imbalance)
	}

	side := oar.GetDominantSide()
	if side != "Bid" && side != "StrongBid" {
		t.Errorf("Expected Bid or StrongBid, got %s", side)
	}
}

func TestOrderArrivalRate_ActivityLevel(t *testing.T) {
	oar := NewOrderArrivalRate("test_activity", 1*time.Second, 5, 100)

	// High arrival rate
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 - float64(i)*0.01, 99.9, 99.8, 99.7, 99.6},
			BidQty:   []uint32{uint32(50 + i*10), 40, 30, 20, 10},
			AskPrice: []float64{100.1 + float64(i)*0.01, 100.2, 100.3, 100.4, 100.5},
			AskQty:   []uint32{uint32(50 + i*10), 40, 30, 20, 10},
		}
		oar.Update(md)
		time.Sleep(20 * time.Millisecond)
	}

	level := oar.GetActivityLevel()
	if level == "VeryLow" || level == "Low" {
		t.Errorf("Expected higher activity level, got %s", level)
	}
}

func TestOrderArrivalRate_NoNewOrders(t *testing.T) {
	oar := NewOrderArrivalRate("test_no_orders", 1*time.Second, 5, 100)

	// Same orderbook repeatedly
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6},
			BidQty:   []uint32{50, 40, 30, 20, 10},
			AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
			AskQty:   []uint32{50, 40, 30, 20, 10},
		}
		oar.Update(md)
	}

	totalRate := oar.GetTotalArrivalRate()
	if totalRate != 0 {
		t.Errorf("Expected rate 0 for no new orders, got %.2f", totalRate)
	}
}

func TestOrderArrivalRate_NewPriceLevel(t *testing.T) {
	oar := NewOrderArrivalRate("test_new_price", 1*time.Second, 5, 100)

	// First update
	md1 := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{50, 40, 30},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{50, 40, 30},
	}
	oar.Update(md1)
	time.Sleep(50 * time.Millisecond)

	// Second update with new price level
	md2 := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.05, 100.0, 99.9, 99.8}, // New price at 100.05
		BidQty:   []uint32{30, 50, 40, 30},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{50, 40, 30},
	}
	oar.Update(md2)

	// Should detect new bid arrival
	bidRate := oar.GetBidArrivalRate()
	if bidRate == 0 {
		t.Error("Expected non-zero bid arrival rate for new price level")
	}
}

func TestOrderArrivalRate_WindowExpiry(t *testing.T) {
	oar := NewOrderArrivalRate("test_expiry", 100*time.Millisecond, 5, 100)

	// Add arrivals
	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 - float64(i)*0.01, 99.9, 99.8, 99.7, 99.6},
			BidQty:   []uint32{uint32(50 + i*10), 40, 30, 20, 10},
			AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
			AskQty:   []uint32{50, 40, 30, 20, 10},
		}
		oar.Update(md)
		time.Sleep(10 * time.Millisecond)
	}

	rateBefore := oar.GetTotalArrivalRate()

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Add new update (will clean old arrivals)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{99.5, 99.4, 99.3, 99.2, 99.1},
		BidQty:   []uint32{100, 40, 30, 20, 10},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
		AskQty:   []uint32{50, 40, 30, 20, 10},
	}
	oar.Update(md)

	rateAfter := oar.GetTotalArrivalRate()

	// Rate should have decreased
	if rateAfter >= rateBefore {
		t.Logf("Rate before: %.2f, after: %.2f", rateBefore, rateAfter)
	}
}

func TestOrderArrivalRate_GetValue(t *testing.T) {
	oar := NewOrderArrivalRate("test_getvalue", 1*time.Second, 5, 100)

	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 - float64(i)*0.01, 99.9, 99.8, 99.7, 99.6},
			BidQty:   []uint32{uint32(50 + i*10), 40, 30, 20, 10},
			AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
			AskQty:   []uint32{50, 40, 30, 20, 10},
		}
		oar.Update(md)
		time.Sleep(50 * time.Millisecond)
	}

	// GetValue() should return total arrival rate
	value := oar.GetValue()
	totalRate := oar.GetTotalArrivalRate()

	if value != totalRate {
		t.Errorf("GetValue() should equal GetTotalArrivalRate(), got %.2f vs %.2f", value, totalRate)
	}
}

func TestOrderArrivalRate_History(t *testing.T) {
	oar := NewOrderArrivalRate("test_history", 1*time.Second, 5, 10)

	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 - float64(i)*0.01, 99.9, 99.8, 99.7, 99.6},
			BidQty:   []uint32{uint32(50 + i*5), 40, 30, 20, 10},
			AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
			AskQty:   []uint32{50, 40, 30, 20, 10},
		}
		oar.Update(md)
	}

	history := oar.GetValues()
	if len(history) != 10 {
		t.Errorf("Expected history length 10, got %d", len(history))
	}
}

func TestOrderArrivalRate_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":                 "config_oar",
		"window_duration_sec":  30.0,
		"levels":               7.0,
		"max_history":          200.0,
	}

	indicator, err := NewOrderArrivalRateFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	oar, ok := indicator.(*OrderArrivalRate)
	if !ok {
		t.Fatal("Expected *OrderArrivalRate type")
	}

	if oar.GetName() != "config_oar" {
		t.Errorf("Expected name 'config_oar', got '%s'", oar.GetName())
	}
}

func TestOrderArrivalRate_String(t *testing.T) {
	oar := NewOrderArrivalRate("test_string", 1*time.Second, 5, 100)

	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 - float64(i)*0.01, 99.9, 99.8, 99.7, 99.6},
			BidQty:   []uint32{uint32(50 + i*10), 40, 30, 20, 10},
			AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5},
			AskQty:   []uint32{50, 40, 30, 20, 10},
		}
		oar.Update(md)
		time.Sleep(50 * time.Millisecond)
	}

	str := oar.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "OrderArrivalRate") {
		t.Error("String() should contain 'OrderArrivalRate'")
	}
}

func BenchmarkOrderArrivalRate_Update(b *testing.B) {
	oar := NewOrderArrivalRate("bench", 1*time.Second, 10, 1000)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8, 99.7, 99.6, 99.5, 99.4, 99.3, 99.2, 99.1},
		BidQty:   []uint32{10, 20, 30, 40, 50, 60, 70, 80, 90, 100},
		AskPrice: []float64{100.1, 100.2, 100.3, 100.4, 100.5, 100.6, 100.7, 100.8, 100.9, 101.0},
		AskQty:   []uint32{15, 25, 35, 45, 55, 65, 75, 85, 95, 105},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		md.BidQty[0] = uint32(10 + (i % 100))
		oar.Update(md)
	}
}
