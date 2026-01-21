package indicators

import (
	"math"
	"testing"
	"time"

	mdpb "github.com/yourusername/hft-poc/pkg/proto/md"
)

func TestSMA_Creation(t *testing.T) {
	sma := NewSMA(20.0, 100)

	if sma == nil {
		t.Fatal("SMA should not be nil")
	}

	if sma.GetName() != "SMA" {
		t.Errorf("Expected name 'SMA', got '%s'", sma.GetName())
	}

	if sma.GetPeriod() != 20 {
		t.Errorf("Expected period 20, got %d", sma.GetPeriod())
	}
}

func TestSMA_IsReady(t *testing.T) {
	sma := NewSMA(5.0, 100)

	if sma.IsReady() {
		t.Error("SMA should not be ready initially")
	}

	// Feed less than period data points
	for i := 0; i < 4; i++ {
		md := createTestMarketDataSMA(100.0 + float64(i))
		sma.Update(md)
	}

	if sma.IsReady() {
		t.Error("SMA should not be ready with less than period data points")
	}

	// Feed one more to complete the period
	md := createTestMarketDataSMA(104.0)
	sma.Update(md)

	if !sma.IsReady() {
		t.Error("SMA should be ready after period data points")
	}
}

func TestSMA_Calculation(t *testing.T) {
	sma := NewSMA(5.0, 100)

	prices := []float64{100.0, 102.0, 104.0, 103.0, 105.0}
	expectedSMA := 0.0
	for _, price := range prices {
		expectedSMA += price
	}
	expectedSMA /= 5.0

	for _, price := range prices {
		md := createTestMarketDataSMA(price)
		sma.Update(md)
	}

	if !sma.IsReady() {
		t.Fatal("SMA should be ready")
	}

	value := sma.GetValue()
	tolerance := 0.0001

	if math.Abs(value-expectedSMA) > tolerance {
		t.Errorf("Expected SMA %.4f, got %.4f", expectedSMA, value)
	}
}

func TestSMA_RollingWindow(t *testing.T) {
	sma := NewSMA(3.0, 100)

	// Feed initial 3 prices: [100, 101, 102] -> SMA = 101
	prices := []float64{100.0, 101.0, 102.0}
	for _, price := range prices {
		md := createTestMarketDataSMA(price)
		sma.Update(md)
	}

	firstSMA := sma.GetValue()
	expectedFirst := 101.0
	if math.Abs(firstSMA-expectedFirst) > 0.0001 {
		t.Errorf("Expected first SMA %.4f, got %.4f", expectedFirst, firstSMA)
	}

	// Add one more price: 103
	// Window becomes [101, 102, 103] -> SMA = 102
	md := createTestMarketDataSMA(103.0)
	sma.Update(md)

	secondSMA := sma.GetValue()
	expectedSecond := 102.0
	if math.Abs(secondSMA-expectedSecond) > 0.0001 {
		t.Errorf("Expected second SMA %.4f, got %.4f", expectedSecond, secondSMA)
	}
}

func TestSMA_TrendDetection(t *testing.T) {
	sma := NewSMA(5.0, 100)

	// Uptrend: prices increasing
	for i := 0; i < 10; i++ {
		price := 100.0 + float64(i)*2.0
		md := createTestMarketDataSMA(price)
		sma.Update(md)
	}

	if !sma.IsReady() {
		t.Fatal("SMA should be ready")
	}

	// In uptrend, SMA should be increasing
	values := sma.GetValues()
	if len(values) < 2 {
		t.Fatal("Should have at least 2 SMA values")
	}

	// Check if trend is upward
	isIncreasing := true
	for i := 1; i < len(values); i++ {
		if values[i] < values[i-1] {
			isIncreasing = false
			break
		}
	}

	if !isIncreasing {
		t.Error("SMA should be increasing in uptrend")
	}
}

func TestSMA_Reset(t *testing.T) {
	sma := NewSMA(5.0, 100)

	// Feed some data
	for i := 0; i < 5; i++ {
		md := createTestMarketDataSMA(100.0 + float64(i))
		sma.Update(md)
	}

	if !sma.IsReady() {
		t.Fatal("SMA should be ready before reset")
	}

	sma.Reset()

	if sma.IsReady() {
		t.Error("SMA should not be ready after reset")
	}

	if sma.GetValue() != 0.0 {
		t.Errorf("SMA value should be 0 after reset, got %.4f", sma.GetValue())
	}

	if len(sma.GetPrices()) != 0 {
		t.Errorf("Price window should be empty after reset, got %d prices", len(sma.GetPrices()))
	}
}

func TestSMA_GetValues(t *testing.T) {
	sma := NewSMA(3.0, 100)

	// Feed data
	prices := []float64{100.0, 101.0, 102.0, 103.0, 104.0}
	for _, price := range prices {
		md := createTestMarketDataSMA(price)
		sma.Update(md)
	}

	values := sma.GetValues()
	// Should have 3 values: [101, 102, 103]
	if len(values) != 3 {
		t.Errorf("Expected 3 SMA values, got %d", len(values))
	}
}

func TestNewSMAFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      10.0,
		"max_history": 500.0,
	}

	ind, err := NewSMAFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create SMA from config: %v", err)
	}

	sma, ok := ind.(*SMA)
	if !ok {
		t.Fatal("Returned indicator should be SMA type")
	}

	if sma.GetPeriod() != 10 {
		t.Errorf("Expected period 10, got %d", sma.GetPeriod())
	}
}

func TestNewSMAFromConfig_Defaults(t *testing.T) {
	config := map[string]interface{}{}

	ind, err := NewSMAFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create SMA from config: %v", err)
	}

	sma, ok := ind.(*SMA)
	if !ok {
		t.Fatal("Returned indicator should be SMA type")
	}

	// Check default period (20)
	if sma.GetPeriod() != 20 {
		t.Errorf("Expected default period 20, got %d", sma.GetPeriod())
	}
}

func TestSMA_AccuracyWithKnownValues(t *testing.T) {
	sma := NewSMA(5.0, 100)

	// Test with known sequence
	prices := []float64{10.0, 20.0, 30.0, 40.0, 50.0}
	// Expected SMA = (10 + 20 + 30 + 40 + 50) / 5 = 30.0

	for _, price := range prices {
		md := createTestMarketDataSMA(price)
		sma.Update(md)
	}

	value := sma.GetValue()
	expected := 30.0

	if math.Abs(value-expected) > 0.0001 {
		t.Errorf("Expected SMA %.4f, got %.4f", expected, value)
	}
}

func TestSMA_SinglePeriod(t *testing.T) {
	sma := NewSMA(1.0, 100)

	md := createTestMarketDataSMA(100.0)
	sma.Update(md)

	if !sma.IsReady() {
		t.Error("SMA(1) should be ready after 1 data point")
	}

	value := sma.GetValue()
	if math.Abs(value-100.0) > 0.0001 {
		t.Errorf("SMA(1) should equal the price, expected 100.0, got %.4f", value)
	}
}

func BenchmarkSMA_Update(b *testing.B) {
	sma := NewSMA(20.0, 1000)
	md := createTestMarketDataSMA(100.0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sma.Update(md)
	}
}

func BenchmarkSMA_FullCalculation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		sma := NewSMA(20.0, 1000)

		// Feed 50 data points
		for j := 0; j < 50; j++ {
			md := createTestMarketDataSMA(100.0 + float64(j)*0.5)
			sma.Update(md)
		}

		_ = sma.GetValue()
	}
}

// Helper function to create test market data for SMA
func createTestMarketDataSMA(price float64) *mdpb.MarketDataUpdate {
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
