package indicators

import (
	"math"
	"testing"
	"time"

	mdpb "github.com/yourusername/hft-poc/pkg/proto/md"
)

func TestMACD_Creation(t *testing.T) {
	macd := NewMACD(12.0, 26.0, 9.0, 100)

	if macd == nil {
		t.Fatal("MACD should not be nil")
	}

	if macd.GetName() != "MACD" {
		t.Errorf("Expected name 'MACD', got '%s'", macd.GetName())
	}

	if macd.fastPeriod != 12 {
		t.Errorf("Expected fastPeriod 12, got %d", macd.fastPeriod)
	}

	if macd.slowPeriod != 26 {
		t.Errorf("Expected slowPeriod 26, got %d", macd.slowPeriod)
	}

	if macd.signalPeriod != 9 {
		t.Errorf("Expected signalPeriod 9, got %d", macd.signalPeriod)
	}
}

func TestMACD_IsReady(t *testing.T) {
	macd := NewMACD(5.0, 10.0, 3.0, 100)

	if macd.IsReady() {
		t.Error("MACD should not be ready initially")
	}

	// Feed data (need slowPeriod + signalPeriod updates)
	for i := 0; i < 15; i++ {
		md := createTestMarketDataMACD(100.0 + float64(i))
		macd.Update(md)
	}

	if !macd.IsReady() {
		t.Error("MACD should be ready after slowPeriod + signalPeriod updates")
	}
}

func TestMACD_LineCalculation(t *testing.T) {
	macd := NewMACD(5.0, 10.0, 3.0, 100)

	// Feed upward trending data
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)
		md := createTestMarketDataMACD(price)
		macd.Update(md)
	}

	macdLine := macd.GetMACDLine()

	// In uptrend, MACD line should be positive (fast EMA > slow EMA)
	if macdLine <= 0 {
		t.Errorf("Expected positive MACD line in uptrend, got %.4f", macdLine)
	}
}

func TestMACD_SignalLineCalculation(t *testing.T) {
	macd := NewMACD(5.0, 10.0, 3.0, 100)

	// Feed data until ready
	for i := 0; i < 20; i++ {
		price := 100.0 + float64(i)*0.5
		md := createTestMarketDataMACD(price)
		macd.Update(md)
	}

	signalLine := macd.GetSignalLine()

	// Signal line should be calculated
	if signalLine == 0 && macd.IsReady() {
		t.Error("Signal line should be calculated when MACD is ready")
	}
}

func TestMACD_HistogramCalculation(t *testing.T) {
	macd := NewMACD(5.0, 10.0, 3.0, 100)

	// Feed data
	for i := 0; i < 25; i++ {
		price := 100.0 + float64(i)*0.5
		md := createTestMarketDataMACD(price)
		macd.Update(md)
	}

	histogram := macd.GetHistogram()
	macdLine := macd.GetMACDLine()
	signalLine := macd.GetSignalLine()

	// Histogram = MACD line - Signal line
	expectedHistogram := macdLine - signalLine
	tolerance := 0.0001

	if math.Abs(histogram-expectedHistogram) > tolerance {
		t.Errorf("Histogram calculation incorrect: expected %.4f, got %.4f", expectedHistogram, histogram)
	}
}

func TestMACD_Crossover_BullishSignal(t *testing.T) {
	macd := NewMACD(5.0, 10.0, 3.0, 100)

	// Feed downtrend then uptrend to create bullish crossover
	// Downtrend
	for i := 0; i < 15; i++ {
		price := 120.0 - float64(i)*0.5
		md := createTestMarketDataMACD(price)
		macd.Update(md)
	}

	histogramBefore := macd.GetHistogram()

	// Uptrend
	for i := 0; i < 15; i++ {
		price := 112.5 + float64(i)*0.7
		md := createTestMarketDataMACD(price)
		macd.Update(md)
	}

	histogramAfter := macd.GetHistogram()

	// In bullish crossover, histogram should increase (become more positive)
	if histogramAfter <= histogramBefore {
		t.Errorf("Expected histogram to increase in bullish crossover, before=%.4f, after=%.4f",
			histogramBefore, histogramAfter)
	}
}

func TestMACD_Crossover_BearishSignal(t *testing.T) {
	macd := NewMACD(5.0, 10.0, 3.0, 100)

	// Feed uptrend then downtrend to create bearish crossover
	// Uptrend
	for i := 0; i < 15; i++ {
		price := 100.0 + float64(i)*0.5
		md := createTestMarketDataMACD(price)
		macd.Update(md)
	}

	histogramBefore := macd.GetHistogram()

	// Downtrend
	for i := 0; i < 15; i++ {
		price := 107.0 - float64(i)*0.7
		md := createTestMarketDataMACD(price)
		macd.Update(md)
	}

	histogramAfter := macd.GetHistogram()

	// In bearish crossover, histogram should decrease (become more negative)
	if histogramAfter >= histogramBefore {
		t.Errorf("Expected histogram to decrease in bearish crossover, before=%.4f, after=%.4f",
			histogramBefore, histogramAfter)
	}
}

func TestMACD_GetValues(t *testing.T) {
	macd := NewMACD(5.0, 10.0, 3.0, 100)

	// Before ready
	values := macd.GetValues()
	if len(values) != 0 {
		t.Error("GetValues should return empty array when not ready")
	}

	// Feed data to make ready
	for i := 0; i < 20; i++ {
		md := createTestMarketDataMACD(100.0 + float64(i))
		macd.Update(md)
	}

	values = macd.GetValues()
	if len(values) != 3 {
		t.Errorf("GetValues should return 3 values [MACD, Signal, Histogram], got %d", len(values))
	}

	if values[0] != macd.GetMACDLine() {
		t.Error("First value should be MACD line")
	}

	if values[1] != macd.GetSignalLine() {
		t.Error("Second value should be Signal line")
	}

	if values[2] != macd.GetHistogram() {
		t.Error("Third value should be Histogram")
	}
}

func TestMACD_Reset(t *testing.T) {
	macd := NewMACD(5.0, 10.0, 3.0, 100)

	// Feed some data
	for i := 0; i < 20; i++ {
		md := createTestMarketDataMACD(100.0 + float64(i))
		macd.Update(md)
	}

	if !macd.IsReady() {
		t.Error("MACD should be ready before reset")
	}

	macd.Reset()

	if macd.IsReady() {
		t.Error("MACD should not be ready after reset")
	}

	if macd.GetMACDLine() != 0.0 {
		t.Errorf("MACD line should be 0 after reset, got %.4f", macd.GetMACDLine())
	}

	if macd.GetSignalLine() != 0.0 {
		t.Errorf("Signal line should be 0 after reset, got %.4f", macd.GetSignalLine())
	}

	if macd.GetHistogram() != 0.0 {
		t.Errorf("Histogram should be 0 after reset, got %.4f", macd.GetHistogram())
	}

	if macd.dataPoints != 0 {
		t.Errorf("Data points should be 0 after reset, got %d", macd.dataPoints)
	}
}

func TestMACD_EMAInitialization(t *testing.T) {
	macd := NewMACD(5.0, 10.0, 3.0, 100)

	// First update initializes EMAs with first price
	firstPrice := 100.0
	md := createTestMarketDataMACD(firstPrice)
	macd.Update(md)

	if macd.fastEMA != firstPrice {
		t.Errorf("Fast EMA should be initialized to first price %.2f, got %.2f", firstPrice, macd.fastEMA)
	}

	if macd.slowEMA != firstPrice {
		t.Errorf("Slow EMA should be initialized to first price %.2f, got %.2f", firstPrice, macd.slowEMA)
	}
}

func TestMACD_AlphaCalculation(t *testing.T) {
	fastPeriod := 12.0
	slowPeriod := 26.0
	signalPeriod := 9.0

	macd := NewMACD(fastPeriod, slowPeriod, signalPeriod, 100)

	// Alpha = 2 / (period + 1)
	expectedFastAlpha := 2.0 / (fastPeriod + 1.0)
	expectedSlowAlpha := 2.0 / (slowPeriod + 1.0)
	expectedSignalAlpha := 2.0 / (signalPeriod + 1.0)

	tolerance := 0.0001

	if math.Abs(macd.fastAlpha-expectedFastAlpha) > tolerance {
		t.Errorf("Fast alpha incorrect: expected %.6f, got %.6f", expectedFastAlpha, macd.fastAlpha)
	}

	if math.Abs(macd.slowAlpha-expectedSlowAlpha) > tolerance {
		t.Errorf("Slow alpha incorrect: expected %.6f, got %.6f", expectedSlowAlpha, macd.slowAlpha)
	}

	if math.Abs(macd.signalAlpha-expectedSignalAlpha) > tolerance {
		t.Errorf("Signal alpha incorrect: expected %.6f, got %.6f", expectedSignalAlpha, macd.signalAlpha)
	}
}

func TestMACD_SignalLagsBehindMACD(t *testing.T) {
	macd := NewMACD(5.0, 10.0, 3.0, 100)

	// Feed rapid uptrend
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i)*2.0
		md := createTestMarketDataMACD(price)
		macd.Update(md)
	}

	macdLine := macd.GetMACDLine()
	signalLine := macd.GetSignalLine()

	// In rapid uptrend, signal line should lag behind MACD line
	// (MACD line should be greater than signal line)
	if macdLine <= signalLine {
		t.Errorf("In uptrend, MACD line should be > signal line, got MACD=%.4f, Signal=%.4f",
			macdLine, signalLine)
	}
}

func TestNewMACDFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"fast_period":   10.0,
		"slow_period":   20.0,
		"signal_period": 5.0,
		"max_history":   500.0,
	}

	ind, err := NewMACDFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create MACD from config: %v", err)
	}

	macd, ok := ind.(*MACD)
	if !ok {
		t.Fatal("Returned indicator should be MACD type")
	}

	if macd.fastPeriod != 10 {
		t.Errorf("Expected fastPeriod 10, got %d", macd.fastPeriod)
	}

	if macd.slowPeriod != 20 {
		t.Errorf("Expected slowPeriod 20, got %d", macd.slowPeriod)
	}

	if macd.signalPeriod != 5 {
		t.Errorf("Expected signalPeriod 5, got %d", macd.signalPeriod)
	}
}

func TestNewMACDFromConfig_Defaults(t *testing.T) {
	config := map[string]interface{}{}

	ind, err := NewMACDFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create MACD from config: %v", err)
	}

	macd, ok := ind.(*MACD)
	if !ok {
		t.Fatal("Returned indicator should be MACD type")
	}

	// Check defaults (12, 26, 9)
	if macd.fastPeriod != 12 {
		t.Errorf("Expected default fastPeriod 12, got %d", macd.fastPeriod)
	}

	if macd.slowPeriod != 26 {
		t.Errorf("Expected default slowPeriod 26, got %d", macd.slowPeriod)
	}

	if macd.signalPeriod != 9 {
		t.Errorf("Expected default signalPeriod 9, got %d", macd.signalPeriod)
	}
}

func TestMACD_GetValue(t *testing.T) {
	macd := NewMACD(5.0, 10.0, 3.0, 100)

	// Feed data
	for i := 0; i < 15; i++ {
		md := createTestMarketDataMACD(100.0 + float64(i))
		macd.Update(md)
	}

	value := macd.GetValue()
	macdLine := macd.GetMACDLine()

	// GetValue should return MACD line
	if value != macdLine {
		t.Errorf("GetValue should return MACD line, got %.4f, expected %.4f", value, macdLine)
	}
}

func BenchmarkMACD_Update(b *testing.B) {
	macd := NewMACD(12.0, 26.0, 9.0, 1000)

	md := createTestMarketDataMACD(100.0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		macd.Update(md)
	}
}

func BenchmarkMACD_FullCalculation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		macd := NewMACD(12.0, 26.0, 9.0, 1000)

		// Feed 50 data points
		for j := 0; j < 50; j++ {
			md := createTestMarketDataMACD(100.0 + float64(j)*0.5)
			macd.Update(md)
		}

		_ = macd.GetMACDLine()
		_ = macd.GetSignalLine()
		_ = macd.GetHistogram()
	}
}

// Helper function to create test market data for MACD
func createTestMarketDataMACD(price float64) *mdpb.MarketDataUpdate {
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
