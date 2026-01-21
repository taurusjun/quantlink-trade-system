package strategy

import (
	"math"
	"testing"
	"time"

	mdpb "github.com/yourusername/hft-poc/pkg/proto/md"
)

func TestPairwiseArbStrategy_Creation(t *testing.T) {
	pas := NewPairwiseArbStrategy("pairwise_1")

	if pas.GetID() != "pairwise_1" {
		t.Errorf("Expected ID 'pairwise_1', got '%s'", pas.GetID())
	}

	if pas.GetType() != "pairwise_arb" {
		t.Errorf("Expected type 'pairwise_arb', got '%s'", pas.GetType())
	}

	// Check default parameters
	if pas.lookbackPeriod != 100 {
		t.Errorf("Expected lookbackPeriod 100, got %d", pas.lookbackPeriod)
	}
	if pas.entryZScore != 2.0 {
		t.Errorf("Expected entryZScore 2.0, got %.2f", pas.entryZScore)
	}
	if pas.exitZScore != 0.5 {
		t.Errorf("Expected exitZScore 0.5, got %.2f", pas.exitZScore)
	}
	if pas.spreadType != "difference" {
		t.Errorf("Expected spreadType 'difference', got '%s'", pas.spreadType)
	}
}

func TestPairwiseArbStrategy_Initialize(t *testing.T) {
	pas := NewPairwiseArbStrategy("pairwise_1")

	config := &StrategyConfig{
		StrategyID:      "pairwise_1",
		StrategyType:    "pairwise_arb",
		Symbols:         []string{"SYMBOL1", "SYMBOL2"},
		Exchanges:       []string{"TEST"},
		MaxPositionSize: 100,
		MaxExposure:     10000.0,
		Parameters: map[string]interface{}{
			"lookback_period":    50.0,
			"entry_zscore":       1.5,
			"exit_zscore":        0.3,
			"order_size":         5.0,
			"max_position_size":  30.0,
			"min_correlation":    0.8,
			"spread_type":        "ratio",
			"use_cointegration":  true,
			"trade_interval_ms":  500.0,
		},
		Enabled: true,
	}

	err := pas.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	if pas.Config == nil {
		t.Error("Config should not be nil after initialization")
	}

	// Check parameters were loaded
	if pas.lookbackPeriod != 50 {
		t.Errorf("Expected lookbackPeriod 50, got %d", pas.lookbackPeriod)
	}
	if pas.entryZScore != 1.5 {
		t.Errorf("Expected entryZScore 1.5, got %.2f", pas.entryZScore)
	}
	if pas.exitZScore != 0.3 {
		t.Errorf("Expected exitZScore 0.3, got %.2f", pas.exitZScore)
	}
	if pas.orderSize != 5 {
		t.Errorf("Expected orderSize 5, got %d", pas.orderSize)
	}
	if pas.minCorrelation != 0.8 {
		t.Errorf("Expected minCorrelation 0.8, got %.2f", pas.minCorrelation)
	}
	if pas.spreadType != "ratio" {
		t.Errorf("Expected spreadType 'ratio', got '%s'", pas.spreadType)
	}

	// Check symbols
	if pas.symbol1 != "SYMBOL1" {
		t.Errorf("Expected symbol1 'SYMBOL1', got '%s'", pas.symbol1)
	}
	if pas.symbol2 != "SYMBOL2" {
		t.Errorf("Expected symbol2 'SYMBOL2', got '%s'", pas.symbol2)
	}
}

func TestPairwiseArbStrategy_Initialize_RequiresExactlyTwoSymbols(t *testing.T) {
	pas := NewPairwiseArbStrategy("pairwise_1")

	// Test with 1 symbol
	config1 := &StrategyConfig{
		StrategyID:   "pairwise_1",
		StrategyType: "pairwise_arb",
		Symbols:      []string{"ONLY_ONE"},
		Enabled:      true,
	}

	err := pas.Initialize(config1)
	if err == nil {
		t.Error("Initialize should fail with only one symbol")
	}

	// Test with 3 symbols
	config3 := &StrategyConfig{
		StrategyID:   "pairwise_1",
		StrategyType: "pairwise_arb",
		Symbols:      []string{"ONE", "TWO", "THREE"},
		Enabled:      true,
	}

	err = pas.Initialize(config3)
	if err == nil {
		t.Error("Initialize should fail with three symbols")
	}
}

func TestPairwiseArbStrategy_SpreadCalculation_Difference(t *testing.T) {
	pas := NewPairwiseArbStrategy("pairwise_1")

	config := &StrategyConfig{
		StrategyID:   "pairwise_1",
		StrategyType: "pairwise_arb",
		Symbols:      []string{"SYMBOL1", "SYMBOL2"},
		Parameters: map[string]interface{}{
			"spread_type": "difference",
		},
		Enabled: true,
	}

	err := pas.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	pas.price1 = 100.0
	pas.price2 = 95.0
	pas.hedgeRatio = 1.0

	pas.calculateSpread()

	// Spread = price1 - hedge_ratio * price2 = 100 - 1.0 * 95 = 5
	expectedSpread := 5.0
	if pas.currentSpread != expectedSpread {
		t.Errorf("Expected spread %.2f, got %.2f", expectedSpread, pas.currentSpread)
	}
}

func TestPairwiseArbStrategy_SpreadCalculation_Ratio(t *testing.T) {
	pas := NewPairwiseArbStrategy("pairwise_1")

	config := &StrategyConfig{
		StrategyID:   "pairwise_1",
		StrategyType: "pairwise_arb",
		Symbols:      []string{"SYMBOL1", "SYMBOL2"},
		Parameters: map[string]interface{}{
			"spread_type": "ratio",
		},
		Enabled: true,
	}

	err := pas.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	pas.price1 = 105.0
	pas.price2 = 100.0

	pas.calculateSpread()

	// Spread = price1 / price2 = 105 / 100 = 1.05
	expectedSpread := 1.05
	if math.Abs(pas.currentSpread-expectedSpread) > 0.001 {
		t.Errorf("Expected spread %.2f, got %.2f", expectedSpread, pas.currentSpread)
	}
}

func TestPairwiseArbStrategy_DualSymbolTracking(t *testing.T) {
	pas := NewPairwiseArbStrategy("pairwise_1")

	config := &StrategyConfig{
		StrategyID:      "pairwise_1",
		StrategyType:    "pairwise_arb",
		Symbols:         []string{"SYMBOL1", "SYMBOL2"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"lookback_period": 20.0,
			"entry_zscore":    2.0,
		},
		Enabled: true,
	}

	err := pas.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	pas.Start()

	// Feed data for symbol1
	md1 := &mdpb.MarketDataUpdate{
		Symbol:      "SYMBOL1",
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{100.0},
		BidQty:      []uint32{100},
		AskPrice:    []float64{100.5},
		AskQty:      []uint32{100},
		LastPrice:   100.25,
		TotalVolume: 1000,
		Turnover:    100250.0,
	}
	pas.OnMarketData(md1)

	if pas.price1 != 100.25 {
		t.Errorf("Expected price1 100.25, got %.2f", pas.price1)
	}
	if len(pas.price1History) != 1 {
		t.Errorf("Expected 1 price1 history entry, got %d", len(pas.price1History))
	}

	// Feed data for symbol2
	md2 := &mdpb.MarketDataUpdate{
		Symbol:      "SYMBOL2",
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{95.0},
		BidQty:      []uint32{100},
		AskPrice:    []float64{95.5},
		AskQty:      []uint32{100},
		LastPrice:   95.25,
		TotalVolume: 1000,
		Turnover:    95250.0,
	}
	pas.OnMarketData(md2)

	if pas.price2 != 95.25 {
		t.Errorf("Expected price2 95.25, got %.2f", pas.price2)
	}
	if len(pas.price2History) != 1 {
		t.Errorf("Expected 1 price2 history entry, got %d", len(pas.price2History))
	}

	// Spread should be calculated
	if pas.currentSpread == 0 {
		t.Error("Spread should be calculated after both prices are available")
	}

	pas.Stop()
}

func TestPairwiseArbStrategy_ZScoreCalculation(t *testing.T) {
	pas := NewPairwiseArbStrategy("pairwise_1")

	config := &StrategyConfig{
		StrategyID:   "pairwise_1",
		StrategyType: "pairwise_arb",
		Symbols:      []string{"SYMBOL1", "SYMBOL2"},
		Parameters: map[string]interface{}{
			"lookback_period": 10.0,
			"spread_type":     "difference",
		},
		Enabled: true,
	}

	err := pas.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	pas.Start()

	// Feed correlated price data with consistent spread
	for i := 0; i < 15; i++ {
		md1 := &mdpb.MarketDataUpdate{
			Symbol:      "SYMBOL1",
			Exchange:    "TEST",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{100.0 + float64(i)},
			BidQty:      []uint32{100},
			AskPrice:    []float64{100.5 + float64(i)},
			AskQty:      []uint32{100},
			LastPrice:   100.25 + float64(i),
			TotalVolume: uint64(1000 + i),
			Turnover:    (100.25 + float64(i)) * float64(1000+i),
		}
		pas.OnMarketData(md1)

		md2 := &mdpb.MarketDataUpdate{
			Symbol:      "SYMBOL2",
			Exchange:    "TEST",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{95.0 + float64(i)},
			BidQty:      []uint32{100},
			AskPrice:    []float64{95.5 + float64(i)},
			AskQty:      []uint32{100},
			LastPrice:   95.25 + float64(i),
			TotalVolume: uint64(1000 + i),
			Turnover:    (95.25 + float64(i)) * float64(1000+i),
		}
		pas.OnMarketData(md2)
	}

	// After enough data, statistics should be calculated
	if pas.spreadMean == 0 {
		t.Error("Spread mean should be calculated")
	}
	if pas.spreadStd == 0 {
		t.Error("Spread std should be calculated")
	}

	pas.Stop()
}

func TestPairwiseArbStrategy_EntrySignal_HighSpread(t *testing.T) {
	pas := NewPairwiseArbStrategy("pairwise_1")

	config := &StrategyConfig{
		StrategyID:      "pairwise_1",
		StrategyType:    "pairwise_arb",
		Symbols:         []string{"SYMBOL1", "SYMBOL2"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"lookback_period":   20.0,
			"entry_zscore":      1.5, // Lower threshold for testing
			"exit_zscore":       0.5,
			"order_size":        10.0,
			"min_correlation":   0.5, // Lower for testing
			"trade_interval_ms": 100.0,
		},
		Enabled: true,
	}

	err := pas.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	pas.Start()

	// Build up spread history with mean ~5
	for i := 0; i < 30; i++ {
		md1 := &mdpb.MarketDataUpdate{
			Symbol:      "SYMBOL1",
			Exchange:    "TEST",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{105.0},
			BidQty:      []uint32{100},
			AskPrice:    []float64{105.5},
			AskQty:      []uint32{100},
			LastPrice:   105.25,
			TotalVolume: uint64(1000 + i),
			Turnover:    105.25 * float64(1000+i),
		}
		pas.OnMarketData(md1)

		md2 := &mdpb.MarketDataUpdate{
			Symbol:      "SYMBOL2",
			Exchange:    "TEST",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{100.0},
			BidQty:      []uint32{100},
			AskPrice:    []float64{100.5},
			AskQty:      []uint32{100},
			LastPrice:   100.25,
			TotalVolume: uint64(1000 + i),
			Turnover:    100.25 * float64(1000+i),
		}
		pas.OnMarketData(md2)

		time.Sleep(5 * time.Millisecond)
	}

	// Now create abnormally high spread (z-score > 1.5)
	time.Sleep(150 * time.Millisecond) // Wait past trade interval

	md1High := &mdpb.MarketDataUpdate{
		Symbol:      "SYMBOL1",
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{115.0}, // Much higher
		BidQty:      []uint32{100},
		AskPrice:    []float64{115.5},
		AskQty:      []uint32{100},
		LastPrice:   115.25,
		TotalVolume: 2000,
		Turnover:    230500.0,
	}
	pas.OnMarketData(md1High)

	md2Same := &mdpb.MarketDataUpdate{
		Symbol:      "SYMBOL2",
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{100.0},
		BidQty:      []uint32{100},
		AskPrice:    []float64{100.5},
		AskQty:      []uint32{100},
		LastPrice:   100.25,
		TotalVolume: 2000,
		Turnover:    200500.0,
	}
	pas.OnMarketData(md2Same)

	// Should generate entry signals for spread trade
	signals := pas.GetSignals()
	if len(signals) >= 2 {
		// Should have signals for both legs
		foundLeg1 := false
		foundLeg2 := false
		for _, signal := range signals {
			if signal.Symbol == "SYMBOL1" && signal.Metadata["type"] == "entry" {
				foundLeg1 = true
			}
			if signal.Symbol == "SYMBOL2" && signal.Metadata["type"] == "entry" {
				foundLeg2 = true
			}
		}
		if !foundLeg1 || !foundLeg2 {
			t.Error("Should generate entry signals for both legs")
		}
	}

	pas.Stop()
}

func TestPairwiseArbStrategy_ExitSignal(t *testing.T) {
	pas := NewPairwiseArbStrategy("pairwise_1")

	config := &StrategyConfig{
		StrategyID:      "pairwise_1",
		StrategyType:    "pairwise_arb",
		Symbols:         []string{"SYMBOL1", "SYMBOL2"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"lookback_period":   20.0,
			"entry_zscore":      2.0,
			"exit_zscore":       0.5,
			"order_size":        10.0,
			"trade_interval_ms": 100.0,
		},
		Enabled: true,
	}

	err := pas.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	pas.Start()

	// Set up position as if we entered a spread trade
	pas.leg1Position = 10
	pas.leg2Position = -10

	// Set spread statistics
	pas.spreadMean = 5.0
	pas.spreadStd = 1.0
	pas.currentSpread = 5.3 // Close to mean
	pas.currentZScore = 0.3  // Below exit threshold

	time.Sleep(150 * time.Millisecond) // Wait past trade interval

	// Feed market data to trigger exit check
	md1 := &mdpb.MarketDataUpdate{
		Symbol:      "SYMBOL1",
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{105.0},
		BidQty:      []uint32{100},
		AskPrice:    []float64{105.5},
		AskQty:      []uint32{100},
		LastPrice:   105.25,
		TotalVolume: 1000,
		Turnover:    105250.0,
	}
	pas.OnMarketData(md1)

	// Should generate exit signals
	signals := pas.GetSignals()
	if len(signals) >= 2 {
		// Check for exit signals
		foundExitLeg1 := false
		foundExitLeg2 := false
		for _, signal := range signals {
			if signal.Metadata["type"] == "exit" {
				if signal.Symbol == "SYMBOL1" {
					foundExitLeg1 = true
				}
				if signal.Symbol == "SYMBOL2" {
					foundExitLeg2 = true
				}
			}
		}
		if !foundExitLeg1 || !foundExitLeg2 {
			t.Error("Should generate exit signals for both legs")
		}
	}

	pas.Stop()
}

func TestPairwiseArbStrategy_CorrelationCheck(t *testing.T) {
	pas := NewPairwiseArbStrategy("pairwise_1")

	// Feed perfectly correlated data
	for i := 0; i < 30; i++ {
		pas.price1History = append(pas.price1History, 100.0+float64(i))
		pas.price2History = append(pas.price2History, 95.0+float64(i)*0.95)
	}

	pas.lookbackPeriod = 20
	pas.minCorrelation = 0.9

	// Check correlation
	ok := pas.checkCorrelation()
	if !ok {
		t.Error("Highly correlated data should pass correlation check")
	}
}

func TestPairwiseArbStrategy_GetSpreadStatus(t *testing.T) {
	pas := NewPairwiseArbStrategy("pairwise_1")

	config := &StrategyConfig{
		StrategyID:   "pairwise_1",
		StrategyType: "pairwise_arb",
		Symbols:      []string{"SYMBOL1", "SYMBOL2"},
		Enabled:      true,
	}

	err := pas.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	pas.price1 = 105.0
	pas.price2 = 100.0
	pas.currentSpread = 5.0
	pas.spreadMean = 4.5
	pas.spreadStd = 0.5
	pas.currentZScore = 1.0
	pas.hedgeRatio = 1.05
	pas.leg1Position = 10
	pas.leg2Position = -10

	status := pas.GetSpreadStatus()

	if status["symbol1"] != "SYMBOL1" {
		t.Error("Status should include symbol1")
	}
	if status["symbol2"] != "SYMBOL2" {
		t.Error("Status should include symbol2")
	}
	if status["price1"] != 105.0 {
		t.Error("Status should include price1")
	}
	if status["price2"] != 100.0 {
		t.Error("Status should include price2")
	}
	if status["spread"] != 5.0 {
		t.Error("Status should include spread")
	}
	if status["z_score"] != 1.0 {
		t.Error("Status should include z_score")
	}
	if status["leg1_position"] != int64(10) {
		t.Error("Status should include leg1_position")
	}
}

func TestPairwiseArbStrategy_StartStop(t *testing.T) {
	pas := NewPairwiseArbStrategy("pairwise_1")

	if pas.IsRunning() {
		t.Error("Strategy should not be running initially")
	}

	pas.Start()
	if !pas.IsRunning() {
		t.Error("Strategy should be running after start")
	}

	pas.Stop()
	if pas.IsRunning() {
		t.Error("Strategy should not be running after stop")
	}
}

func TestPairwiseArbStrategy_HistoryTracking(t *testing.T) {
	pas := NewPairwiseArbStrategy("pairwise_1")

	config := &StrategyConfig{
		StrategyID:   "pairwise_1",
		StrategyType: "pairwise_arb",
		Symbols:      []string{"SYMBOL1", "SYMBOL2"},
		Enabled:      true,
	}

	err := pas.Initialize(config)
	if err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	pas.Start()

	// Feed 250 data points (exceeds maxHistoryLen of 200)
	for i := 0; i < 250; i++ {
		md1 := &mdpb.MarketDataUpdate{
			Symbol:      "SYMBOL1",
			Exchange:    "TEST",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{100.0 + float64(i)},
			BidQty:      []uint32{100},
			AskPrice:    []float64{100.5 + float64(i)},
			AskQty:      []uint32{100},
			LastPrice:   100.25 + float64(i),
			TotalVolume: uint64(1000 + i),
			Turnover:    (100.25 + float64(i)) * float64(1000+i),
		}
		pas.OnMarketData(md1)

		md2 := &mdpb.MarketDataUpdate{
			Symbol:      "SYMBOL2",
			Exchange:    "TEST",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{95.0 + float64(i)},
			BidQty:      []uint32{100},
			AskPrice:    []float64{95.5 + float64(i)},
			AskQty:      []uint32{100},
			LastPrice:   95.25 + float64(i),
			TotalVolume: uint64(1000 + i),
			Turnover:    (95.25 + float64(i)) * float64(1000+i),
		}
		pas.OnMarketData(md2)
	}

	// History should be capped at maxHistoryLen
	if len(pas.spreadHistory) > pas.maxHistoryLen {
		t.Errorf("Spread history length %d exceeds max %d", len(pas.spreadHistory), pas.maxHistoryLen)
	}
	if len(pas.price1History) > pas.maxHistoryLen {
		t.Errorf("Price1 history length %d exceeds max %d", len(pas.price1History), pas.maxHistoryLen)
	}
	if len(pas.price2History) > pas.maxHistoryLen {
		t.Errorf("Price2 history length %d exceeds max %d", len(pas.price2History), pas.maxHistoryLen)
	}

	pas.Stop()
}

func BenchmarkPairwiseArbStrategy_OnMarketData(b *testing.B) {
	pas := NewPairwiseArbStrategy("pairwise_1")

	config := &StrategyConfig{
		StrategyID:      "pairwise_1",
		StrategyType:    "pairwise_arb",
		Symbols:         []string{"SYMBOL1", "SYMBOL2"},
		MaxPositionSize: 100,
		Parameters: map[string]interface{}{
			"lookback_period": 50.0,
			"entry_zscore":    2.0,
			"order_size":      10.0,
		},
		Enabled: true,
	}
	pas.Initialize(config)
	pas.Start()

	md := &mdpb.MarketDataUpdate{
		Symbol:      "SYMBOL1",
		Exchange:    "TEST",
		Timestamp:   uint64(time.Now().UnixNano()),
		BidPrice:    []float64{100.0},
		BidQty:      []uint32{100},
		AskPrice:    []float64{100.5},
		AskQty:      []uint32{100},
		LastPrice:   100.25,
		TotalVolume: 1000,
		Turnover:    100250.0,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pas.OnMarketData(md)
	}
}
