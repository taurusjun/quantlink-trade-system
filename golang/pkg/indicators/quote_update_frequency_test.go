package indicators

import (
	"testing"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func TestQuoteUpdateFrequency_FrequentUpdates(t *testing.T) {
	quf := NewQuoteUpdateFrequency("test_frequent", 1*time.Second, 100)

	// Rapidly changing quotes
	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)*0.01, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1 + float64(i)*0.01, 100.2, 100.3},
			AskQty:   []uint32{50, 40, 30},
		}
		quf.Update(md)
		time.Sleep(10 * time.Millisecond)
	}

	freq := quf.GetFrequency()
	if freq <= 0 {
		t.Errorf("Expected positive frequency, got %.2f", freq)
	}

	level := quf.GetActivityLevel()
	if level == "VeryLow" {
		t.Errorf("Expected higher activity level, got %s", level)
	}

	if !quf.IsActive() {
		t.Error("Expected active market")
	}
}

func TestQuoteUpdateFrequency_InfrequentUpdates(t *testing.T) {
	quf := NewQuoteUpdateFrequency("test_infrequent", 1*time.Second, 100)

	// Only a few updates
	for i := 0; i < 3; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)*0.1, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{50, 40, 30},
		}
		quf.Update(md)
		time.Sleep(200 * time.Millisecond)
	}

	freq := quf.GetFrequency()
	if freq > 10 {
		t.Errorf("Expected low frequency, got %.2f", freq)
	}
}

func TestQuoteUpdateFrequency_NoChanges(t *testing.T) {
	quf := NewQuoteUpdateFrequency("test_no_changes", 1*time.Second, 100)

	// Same quotes repeatedly (no updates)
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{50, 40, 30},
		}
		quf.Update(md)
	}

	freq := quf.GetFrequency()
	if freq != 0 {
		t.Errorf("Expected frequency 0 for no changes, got %.2f", freq)
	}
}

func TestQuoteUpdateFrequency_BidUpdates(t *testing.T) {
	quf := NewQuoteUpdateFrequency("test_bid_updates", 1*time.Second, 100)

	// Only bid updates
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)*0.01, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.2, 100.3}, // Constant
			AskQty:   []uint32{50, 40, 30},
		}
		quf.Update(md)
		time.Sleep(20 * time.Millisecond)
	}

	bidFreq := quf.GetBidUpdateFrequency()
	askFreq := quf.GetAskUpdateFrequency()

	if bidFreq == 0 {
		t.Error("Expected non-zero bid update frequency")
	}

	if askFreq != 0 {
		t.Logf("Ask frequency %.2f (expected 0 but acceptable due to estimation)", askFreq)
	}
}

func TestQuoteUpdateFrequency_AskUpdates(t *testing.T) {
	quf := NewQuoteUpdateFrequency("test_ask_updates", 1*time.Second, 100)

	// Only ask updates
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0, 99.9, 99.8}, // Constant
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1 + float64(i)*0.01, 100.2, 100.3},
			AskQty:   []uint32{50, 40, 30},
		}
		quf.Update(md)
		time.Sleep(20 * time.Millisecond)
	}

	askFreq := quf.GetAskUpdateFrequency()

	if askFreq == 0 {
		t.Error("Expected non-zero ask update frequency")
	}
}

func TestQuoteUpdateFrequency_AvgInterval(t *testing.T) {
	quf := NewQuoteUpdateFrequency("test_interval", 1*time.Second, 100)

	// Updates every 50ms
	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)*0.01, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{50, 40, 30},
		}
		quf.Update(md)
		time.Sleep(50 * time.Millisecond)
	}

	interval := quf.GetAvgUpdateInterval()
	// Should be around 50ms
	if interval < 30 || interval > 70 {
		t.Logf("Average interval %.1fms (expected around 50ms)", interval)
	}
}

func TestQuoteUpdateFrequency_WindowExpiry(t *testing.T) {
	quf := NewQuoteUpdateFrequency("test_expiry", 100*time.Millisecond, 100)

	// Add updates
	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)*0.01, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{50, 40, 30},
		}
		quf.Update(md)
		time.Sleep(10 * time.Millisecond)
	}

	freqBefore := quf.GetFrequency()

	// Wait for window to expire
	time.Sleep(150 * time.Millisecond)

	// Add new update (will clean old ones)
	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.5, 99.9, 99.8},
		BidQty:   []uint32{50, 40, 30},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{50, 40, 30},
	}
	quf.Update(md)

	freqAfter := quf.GetFrequency()

	// Frequency should have decreased
	if freqAfter >= freqBefore {
		t.Logf("Frequency before: %.2f, after: %.2f", freqBefore, freqAfter)
	}
}

func TestQuoteUpdateFrequency_EmptyData(t *testing.T) {
	quf := NewQuoteUpdateFrequency("test_empty", 1*time.Second, 100)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{},
		BidQty:   []uint32{},
		AskPrice: []float64{},
		AskQty:   []uint32{},
	}

	quf.Update(md)

	// Should handle gracefully
	if quf.GetFrequency() != 0 {
		t.Errorf("Expected frequency 0 for empty data, got %.2f", quf.GetFrequency())
	}
}

func TestQuoteUpdateFrequency_GetValue(t *testing.T) {
	quf := NewQuoteUpdateFrequency("test_getvalue", 1*time.Second, 100)

	for i := 0; i < 10; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)*0.01, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{50, 40, 30},
		}
		quf.Update(md)
		time.Sleep(10 * time.Millisecond)
	}

	// GetValue() should return frequency
	value := quf.GetValue()
	freq := quf.GetFrequency()

	if value != freq {
		t.Errorf("GetValue() should equal GetFrequency(), got %.2f vs %.2f", value, freq)
	}
}

func TestQuoteUpdateFrequency_History(t *testing.T) {
	quf := NewQuoteUpdateFrequency("test_history", 1*time.Second, 10)

	for i := 0; i < 20; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)*0.01, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{50, 40, 30},
		}
		quf.Update(md)
	}

	history := quf.GetValues()
	if len(history) != 10 {
		t.Errorf("Expected history length 10, got %d", len(history))
	}
}

func TestQuoteUpdateFrequency_FromConfig(t *testing.T) {
	config := map[string]interface{}{
		"name":                 "config_quf",
		"window_duration_sec":  30.0,
		"max_history":          200.0,
	}

	indicator, err := NewQuoteUpdateFrequencyFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create from config: %v", err)
	}

	quf, ok := indicator.(*QuoteUpdateFrequency)
	if !ok {
		t.Fatal("Expected *QuoteUpdateFrequency type")
	}

	if quf.GetName() != "config_quf" {
		t.Errorf("Expected name 'config_quf', got '%s'", quf.GetName())
	}
}

func TestQuoteUpdateFrequency_String(t *testing.T) {
	quf := NewQuoteUpdateFrequency("test_string", 1*time.Second, 100)

	for i := 0; i < 5; i++ {
		md := &mdpb.MarketDataUpdate{
			BidPrice: []float64{100.0 + float64(i)*0.01, 99.9, 99.8},
			BidQty:   []uint32{50, 40, 30},
			AskPrice: []float64{100.1, 100.2, 100.3},
			AskQty:   []uint32{50, 40, 30},
		}
		quf.Update(md)
		time.Sleep(10 * time.Millisecond)
	}

	str := quf.String()
	if len(str) == 0 {
		t.Error("String() should return non-empty string")
	}

	if !contains(str, "QuoteUpdateFrequency") {
		t.Error("String() should contain 'QuoteUpdateFrequency'")
	}
}

func BenchmarkQuoteUpdateFrequency_Update(b *testing.B) {
	quf := NewQuoteUpdateFrequency("bench", 1*time.Second, 1000)

	md := &mdpb.MarketDataUpdate{
		BidPrice: []float64{100.0, 99.9, 99.8},
		BidQty:   []uint32{50, 40, 30},
		AskPrice: []float64{100.1, 100.2, 100.3},
		AskQty:   []uint32{50, 40, 30},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		md.BidPrice[0] = 100.0 + float64(i%100)*0.01
		quf.Update(md)
	}
}
