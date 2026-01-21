package indicators

import (
	"math"
	"testing"
	"time"

	mdpb "github.com/yourusername/hft-poc/pkg/proto/md"
)

func TestRSI_Creation(t *testing.T) {
	rsi := NewRSI(14.0, 100)

	if rsi == nil {
		t.Fatal("RSI should not be nil")
	}

	if rsi.GetName() != "RSI" {
		t.Errorf("Expected name 'RSI', got '%s'", rsi.GetName())
	}

	if rsi.period != 14 {
		t.Errorf("Expected period 14, got %d", rsi.period)
	}
}

func TestRSI_IsReady(t *testing.T) {
	rsi := NewRSI(5.0, 100)

	if rsi.IsReady() {
		t.Error("RSI should not be ready initially")
	}

	// Feed data
	for i := 0; i < 6; i++ {
		md := createTestMarketDataRSI(100.0 + float64(i))
		rsi.Update(md)
	}

	if !rsi.IsReady() {
		t.Error("RSI should be ready after period+1 updates")
	}
}

func TestRSI_CalculationOverbought(t *testing.T) {
	rsi := NewRSI(14.0, 100)

	// Feed strongly upward trending data (RSI should approach 100)
	basePrice := 100.0
	for i := 0; i < 30; i++ {
		price := basePrice + float64(i)*2.0 // Strong uptrend
		md := createTestMarketDataRSI(price)
		rsi.Update(md)
	}

	value := rsi.GetValue()

	// In strong uptrend, RSI should be high (>70 is overbought)
	if value < 50.0 {
		t.Errorf("Expected high RSI in uptrend, got %.2f", value)
	}

	if value > 100.0 {
		t.Errorf("RSI should not exceed 100, got %.2f", value)
	}
}

func TestRSI_CalculationOversold(t *testing.T) {
	rsi := NewRSI(14.0, 100)

	// Feed strongly downward trending data (RSI should approach 0)
	basePrice := 200.0
	for i := 0; i < 30; i++ {
		price := basePrice - float64(i)*2.0 // Strong downtrend
		md := createTestMarketDataRSI(price)
		rsi.Update(md)
	}

	value := rsi.GetValue()

	// In strong downtrend, RSI should be low (<30 is oversold)
	if value > 50.0 {
		t.Errorf("Expected low RSI in downtrend, got %.2f", value)
	}

	if value < 0.0 {
		t.Errorf("RSI should not be below 0, got %.2f", value)
	}
}

func TestRSI_NeutralMarket(t *testing.T) {
	rsi := NewRSI(14.0, 100)

	// Feed sideways market data (RSI should be around 50)
	for i := 0; i < 30; i++ {
		price := 100.0 + math.Sin(float64(i)*0.1)*2.0 // Small oscillation
		md := createTestMarketDataRSI(price)
		rsi.Update(md)
	}

	value := rsi.GetValue()

	// In neutral market, RSI should be near 50
	if value < 30.0 || value > 70.0 {
		t.Errorf("Expected RSI near 50 in neutral market, got %.2f", value)
	}
}

func TestRSI_Reset(t *testing.T) {
	rsi := NewRSI(14.0, 100)

	// Feed some data
	for i := 0; i < 20; i++ {
		md := createTestMarketDataRSI(100.0 + float64(i))
		rsi.Update(md)
	}

	if !rsi.IsReady() {
		t.Error("RSI should be ready before reset")
	}

	rsi.Reset()

	if rsi.IsReady() {
		t.Error("RSI should not be ready after reset")
	}

	if rsi.GetValue() != 0.0 {
		t.Errorf("RSI value should be 0 after reset, got %.2f", rsi.GetValue())
	}

	if len(rsi.gains) != 0 {
		t.Errorf("Gains history should be empty after reset, got %d", len(rsi.gains))
	}

	if len(rsi.losses) != 0 {
		t.Errorf("Losses history should be empty after reset, got %d", len(rsi.losses))
	}
}

func TestRSI_GetValues(t *testing.T) {
	rsi := NewRSI(5.0, 100)

	// Before ready
	values := rsi.GetValues()
	if len(values) != 0 {
		t.Error("GetValues should return empty array when not ready")
	}

	// Feed data to make ready
	for i := 0; i < 10; i++ {
		md := createTestMarketDataRSI(100.0 + float64(i))
		rsi.Update(md)
	}

	values = rsi.GetValues()
	if len(values) != 1 {
		t.Errorf("GetValues should return 1 value, got %d", len(values))
	}

	if values[0] != rsi.GetValue() {
		t.Error("GetValues should return current value")
	}
}

func TestRSI_HistoryLimit(t *testing.T) {
	maxHistory := 50
	rsi := NewRSI(14.0, maxHistory)

	// Feed more data than maxHistory
	for i := 0; i < maxHistory+20; i++ {
		md := createTestMarketDataRSI(100.0 + float64(i)*0.5)
		rsi.Update(md)
	}

	if len(rsi.gains) > maxHistory {
		t.Errorf("Gains history should not exceed maxHistory %d, got %d", maxHistory, len(rsi.gains))
	}

	if len(rsi.losses) > maxHistory {
		t.Errorf("Losses history should not exceed maxHistory %d, got %d", maxHistory, len(rsi.losses))
	}
}

func TestRSI_ZeroAvgLoss(t *testing.T) {
	rsi := NewRSI(5.0, 100)

	// Feed only increasing prices (no losses)
	for i := 0; i < 10; i++ {
		price := 100.0 + float64(i)
		md := createTestMarketDataRSI(price)
		rsi.Update(md)
	}

	value := rsi.GetValue()

	// With no losses, RSI should be 100
	if value != 100.0 {
		t.Errorf("Expected RSI 100 with no losses, got %.2f", value)
	}
}

func TestRSI_WildersSmoothing(t *testing.T) {
	rsi := NewRSI(14.0, 100)

	// Feed initial data
	prices := []float64{100, 102, 101, 103, 104, 103, 105, 107, 106, 108, 110, 109, 111, 113, 112}

	for _, price := range prices {
		md := createTestMarketDataRSI(price)
		rsi.Update(md)
	}

	// Check that RSI is using Wilder's smoothing (avgGain and avgLoss should be calculated)
	if rsi.avgGain == 0 && rsi.avgLoss == 0 {
		t.Error("Average gain and loss should be calculated with Wilder's smoothing")
	}

	// Verify RSI is within valid range
	value := rsi.GetValue()
	if value < 0 || value > 100 {
		t.Errorf("RSI value should be in [0, 100], got %.2f", value)
	}
}

func TestNewRSIFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      10.0,
		"max_history": 200.0,
	}

	ind, err := NewRSIFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create RSI from config: %v", err)
	}

	rsi, ok := ind.(*RSI)
	if !ok {
		t.Fatal("Returned indicator should be RSI type")
	}

	if rsi.period != 10 {
		t.Errorf("Expected period 10, got %d", rsi.period)
	}
}

func TestNewRSIFromConfig_Defaults(t *testing.T) {
	config := map[string]interface{}{}

	ind, err := NewRSIFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create RSI from config: %v", err)
	}

	rsi, ok := ind.(*RSI)
	if !ok {
		t.Fatal("Returned indicator should be RSI type")
	}

	// Check defaults
	if rsi.period != 14 {
		t.Errorf("Expected default period 14, got %d", rsi.period)
	}
}

func BenchmarkRSI_Update(b *testing.B) {
	rsi := NewRSI(14.0, 1000)

	md := createTestMarketDataRSI(100.0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rsi.Update(md)
	}
}

func BenchmarkRSI_FullCalculation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		rsi := NewRSI(14.0, 1000)

		// Feed 50 data points
		for j := 0; j < 50; j++ {
			md := createTestMarketDataRSI(100.0 + float64(j)*0.5)
			rsi.Update(md)
		}

		_ = rsi.GetValue()
	}
}

// Helper function to create test market data for RSI
func createTestMarketDataRSI(price float64) *mdpb.MarketDataUpdate {
	return &mdpb.MarketDataUpdate{
		Symbol:      "TEST",
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{price - 0.5},
		BidQty:      []uint32{100},
		AskPrice:    []float64{price + 0.5},
		AskQty:      []uint32{100},
		LastPrice:   price,
		TotalVolume: 1000,
		Turnover:    price * 1000,
	}
}
