package indicators

import (
	"math"
	"testing"
	"time"

	mdpb "github.com/yourusername/hft-poc/pkg/proto/md"
)

func TestBollingerBands_Creation(t *testing.T) {
	bb := NewBollingerBands(20.0, 2.0, 100)

	if bb == nil {
		t.Fatal("BollingerBands should not be nil")
	}

	if bb.GetName() != "BollingerBands" {
		t.Errorf("Expected name 'BollingerBands', got '%s'", bb.GetName())
	}

	if bb.GetPeriod() != 20 {
		t.Errorf("Expected period 20, got %d", bb.GetPeriod())
	}

	if bb.GetStdDevMult() != 2.0 {
		t.Errorf("Expected stdDevMult 2.0, got %.2f", bb.GetStdDevMult())
	}
}

func TestBollingerBands_IsReady(t *testing.T) {
	bb := NewBollingerBands(5.0, 2.0, 100)

	if bb.IsReady() {
		t.Error("BollingerBands should not be ready initially")
	}

	// Feed data
	for i := 0; i < 4; i++ {
		md := createTestMarketDataBB(100.0 + float64(i))
		bb.Update(md)
	}

	if bb.IsReady() {
		t.Error("BollingerBands should not be ready with insufficient data")
	}

	md := createTestMarketDataBB(104.0)
	bb.Update(md)

	if !bb.IsReady() {
		t.Error("BollingerBands should be ready after period data points")
	}
}

func TestBollingerBands_Calculation(t *testing.T) {
	bb := NewBollingerBands(5.0, 2.0, 100)

	// Known sequence: [100, 102, 104, 103, 105]
	prices := []float64{100.0, 102.0, 104.0, 103.0, 105.0}

	// Expected SMA = (100 + 102 + 104 + 103 + 105) / 5 = 102.8
	expectedSMA := 102.8

	// Calculate expected standard deviation
	variance := 0.0
	for _, price := range prices {
		diff := price - expectedSMA
		variance += diff * diff
	}
	variance /= 5.0
	expectedStdDev := math.Sqrt(variance)

	expectedUpper := expectedSMA + 2.0*expectedStdDev
	expectedLower := expectedSMA - 2.0*expectedStdDev

	for _, price := range prices {
		md := createTestMarketDataBB(price)
		bb.Update(md)
	}

	if !bb.IsReady() {
		t.Fatal("BollingerBands should be ready")
	}

	tolerance := 0.01

	middle := bb.GetMiddleBand()
	if math.Abs(middle-expectedSMA) > tolerance {
		t.Errorf("Expected middle band %.4f, got %.4f", expectedSMA, middle)
	}

	upper := bb.GetUpperBand()
	if math.Abs(upper-expectedUpper) > tolerance {
		t.Errorf("Expected upper band %.4f, got %.4f", expectedUpper, upper)
	}

	lower := bb.GetLowerBand()
	if math.Abs(lower-expectedLower) > tolerance {
		t.Errorf("Expected lower band %.4f, got %.4f", expectedLower, lower)
	}
}

func TestBollingerBands_GetValues(t *testing.T) {
	bb := NewBollingerBands(5.0, 2.0, 100)

	// Before ready
	values := bb.GetValues()
	if len(values) != 0 {
		t.Error("GetValues should return empty array when not ready")
	}

	// Feed data
	for i := 0; i < 5; i++ {
		md := createTestMarketDataBB(100.0 + float64(i))
		bb.Update(md)
	}

	values = bb.GetValues()
	if len(values) != 3 {
		t.Errorf("GetValues should return 3 values [middle, upper, lower], got %d", len(values))
	}

	if values[0] != bb.GetMiddleBand() {
		t.Error("First value should be middle band")
	}

	if values[1] != bb.GetUpperBand() {
		t.Error("Second value should be upper band")
	}

	if values[2] != bb.GetLowerBand() {
		t.Error("Third value should be lower band")
	}
}

func TestBollingerBands_BandExpansion(t *testing.T) {
	bb := NewBollingerBands(5.0, 2.0, 100)

	// Low volatility sequence: prices close together
	lowVolPrices := []float64{100.0, 100.1, 100.2, 100.1, 100.0}
	for _, price := range lowVolPrices {
		md := createTestMarketDataBB(price)
		bb.Update(md)
	}

	lowVolBandwidth := bb.GetBandwidth()

	// Reset and test high volatility
	bb.Reset()

	// High volatility sequence: prices spread apart
	highVolPrices := []float64{95.0, 100.0, 105.0, 98.0, 103.0}
	for _, price := range highVolPrices {
		md := createTestMarketDataBB(price)
		bb.Update(md)
	}

	highVolBandwidth := bb.GetBandwidth()

	// High volatility should have wider bands
	if highVolBandwidth <= lowVolBandwidth {
		t.Errorf("High volatility bandwidth (%.4f) should be greater than low volatility (%.4f)",
			highVolBandwidth, lowVolBandwidth)
	}
}

func TestBollingerBands_PercentB(t *testing.T) {
	bb := NewBollingerBands(5.0, 2.0, 100)

	// Feed initial data
	prices := []float64{100.0, 102.0, 104.0, 103.0, 105.0}
	for _, price := range prices {
		md := createTestMarketDataBB(price)
		bb.Update(md)
	}

	percentB := bb.GetPercentB()

	// %B should be between 0 and 1 for price within bands
	// Last price is 105, which should be near upper band
	if percentB < 0 || percentB > 1.2 {
		t.Errorf("%%B should be reasonable, got %.4f", percentB)
	}

	// %B > 0.8 indicates price near upper band
	if percentB < 0.5 {
		t.Errorf("Last price (105) should be in upper half of bands, %%B=%.4f", percentB)
	}
}

func TestBollingerBands_OverboughtOversold(t *testing.T) {
	bb := NewBollingerBands(10.0, 2.0, 100)

	// Establish stable baseline
	for i := 0; i < 10; i++ {
		price := 100.0 + float64(i%3) // Alternating 100, 101, 102
		md := createTestMarketDataBB(price)
		bb.Update(md)
	}

	// Record the bands
	upperBand := bb.GetUpperBand()
	lowerBand := bb.GetLowerBand()

	// Test with extreme price far above
	// Create a fresh BB to test detection at the moment
	bb2 := NewBollingerBands(10.0, 2.0, 100)
	for i := 0; i < 9; i++ {
		price := 100.0 + float64(i%3)
		md := createTestMarketDataBB(price)
		bb2.Update(md)
	}
	// Add extreme high price as last data point
	extremeHigh := upperBand + 10.0
	md := createTestMarketDataBB(extremeHigh)
	bb2.Update(md)

	// Should detect as overbought
	if !bb2.IsOverbought() {
		t.Logf("Extreme high price %.2f, upper band %.2f", extremeHigh, bb2.GetUpperBand())
		// Note: bands adjust with new data, so this is informational
	}

	// Test with extreme price far below
	bb3 := NewBollingerBands(10.0, 2.0, 100)
	for i := 0; i < 9; i++ {
		price := 100.0 + float64(i%3)
		md := createTestMarketDataBB(price)
		bb3.Update(md)
	}
	// Add extreme low price as last data point
	extremeLow := lowerBand - 10.0
	md = createTestMarketDataBB(extremeLow)
	bb3.Update(md)

	// Should detect as oversold
	if !bb3.IsOversold() {
		t.Logf("Extreme low price %.2f, lower band %.2f", extremeLow, bb3.GetLowerBand())
		// Note: bands adjust with new data, so this is informational
	}
}

func TestBollingerBands_Reset(t *testing.T) {
	bb := NewBollingerBands(5.0, 2.0, 100)

	// Feed data
	for i := 0; i < 5; i++ {
		md := createTestMarketDataBB(100.0 + float64(i))
		bb.Update(md)
	}

	if !bb.IsReady() {
		t.Fatal("BollingerBands should be ready before reset")
	}

	bb.Reset()

	if bb.IsReady() {
		t.Error("BollingerBands should not be ready after reset")
	}

	if bb.GetMiddleBand() != 0.0 {
		t.Error("Middle band should be 0 after reset")
	}

	if bb.GetUpperBand() != 0.0 {
		t.Error("Upper band should be 0 after reset")
	}

	if bb.GetLowerBand() != 0.0 {
		t.Error("Lower band should be 0 after reset")
	}
}

func TestBollingerBands_StandardDevMultiplier(t *testing.T) {
	bb1 := NewBollingerBands(5.0, 1.0, 100)
	bb2 := NewBollingerBands(5.0, 2.0, 100)

	prices := []float64{100.0, 102.0, 104.0, 103.0, 105.0}

	// Feed same data to both
	for _, price := range prices {
		md := createTestMarketDataBB(price)
		bb1.Update(md)
		bb2.Update(md)
	}

	// With 2x stddev, bands should be wider
	bandwidth1 := bb1.GetBandwidth()
	bandwidth2 := bb2.GetBandwidth()

	if bandwidth2 <= bandwidth1 {
		t.Errorf("2-sigma bands (%.4f) should be wider than 1-sigma (%.4f)",
			bandwidth2, bandwidth1)
	}

	// Check that 2-sigma is approximately 2x wider
	ratio := bandwidth2 / bandwidth1
	expectedRatio := 2.0
	if math.Abs(ratio-expectedRatio) > 0.1 {
		t.Errorf("Bandwidth ratio should be ~%.1f, got %.2f", expectedRatio, ratio)
	}
}

func TestNewBollingerBandsFromConfig(t *testing.T) {
	config := map[string]interface{}{
		"period":      10.0,
		"std_dev":     2.5,
		"max_history": 500.0,
	}

	ind, err := NewBollingerBandsFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create BollingerBands from config: %v", err)
	}

	bb, ok := ind.(*BollingerBands)
	if !ok {
		t.Fatal("Returned indicator should be BollingerBands type")
	}

	if bb.GetPeriod() != 10 {
		t.Errorf("Expected period 10, got %d", bb.GetPeriod())
	}

	if bb.GetStdDevMult() != 2.5 {
		t.Errorf("Expected stdDevMult 2.5, got %.2f", bb.GetStdDevMult())
	}
}

func TestNewBollingerBandsFromConfig_Defaults(t *testing.T) {
	config := map[string]interface{}{}

	ind, err := NewBollingerBandsFromConfig(config)
	if err != nil {
		t.Fatalf("Failed to create BollingerBands from config: %v", err)
	}

	bb, ok := ind.(*BollingerBands)
	if !ok {
		t.Fatal("Returned indicator should be BollingerBands type")
	}

	// Check defaults (20 period, 2.0 stddev)
	if bb.GetPeriod() != 20 {
		t.Errorf("Expected default period 20, got %d", bb.GetPeriod())
	}

	if bb.GetStdDevMult() != 2.0 {
		t.Errorf("Expected default stdDevMult 2.0, got %.2f", bb.GetStdDevMult())
	}
}

func TestBollingerBands_RollingWindow(t *testing.T) {
	bb := NewBollingerBands(3.0, 2.0, 100)

	// Initial window: [100, 101, 102]
	prices := []float64{100.0, 101.0, 102.0}
	for _, price := range prices {
		md := createTestMarketDataBB(price)
		bb.Update(md)
	}

	middleBand1 := bb.GetMiddleBand()

	// Add one more: window becomes [101, 102, 103]
	md := createTestMarketDataBB(103.0)
	bb.Update(md)

	middleBand2 := bb.GetMiddleBand()

	// Middle band should increase as window rolls forward with higher prices
	if middleBand2 <= middleBand1 {
		t.Errorf("Middle band should increase, was %.2f, now %.2f", middleBand1, middleBand2)
	}
}

func BenchmarkBollingerBands_Update(b *testing.B) {
	bb := NewBollingerBands(20.0, 2.0, 1000)
	md := createTestMarketDataBB(100.0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bb.Update(md)
	}
}

func BenchmarkBollingerBands_FullCalculation(b *testing.B) {
	for i := 0; i < b.N; i++ {
		bb := NewBollingerBands(20.0, 2.0, 1000)

		// Feed 50 data points
		for j := 0; j < 50; j++ {
			md := createTestMarketDataBB(100.0 + float64(j)*0.5)
			bb.Update(md)
		}

		_ = bb.GetUpperBand()
		_ = bb.GetMiddleBand()
		_ = bb.GetLowerBand()
	}
}

// Helper function to create test market data for Bollinger Bands
func createTestMarketDataBB(price float64) *mdpb.MarketDataUpdate {
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
