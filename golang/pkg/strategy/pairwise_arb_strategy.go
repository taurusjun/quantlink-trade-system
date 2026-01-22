// Package strategy provides trading strategy implementations
package strategy

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

// PairwiseArbStrategy implements a statistical arbitrage / pairs trading strategy
// It identifies and trades mean-reverting spread between two correlated instruments
type PairwiseArbStrategy struct {
	*BaseStrategy

	// Strategy parameters
	symbol1           string  // First symbol (e.g., "ag2412")
	symbol2           string  // Second symbol (e.g., "ag2501")
	lookbackPeriod    int     // Period for mean/std calculation (default: 100)
	entryZScore       float64 // Z-score threshold to enter (default: 2.0)
	exitZScore        float64 // Z-score threshold to exit (default: 0.5)
	orderSize         int64   // Size per leg (default: 10)
	maxPositionSize   int64   // Maximum position per leg (default: 50)
	minCorrelation    float64 // Minimum correlation to trade (default: 0.7)
	hedgeRatio        float64 // Current hedge ratio (calculated dynamically)
	spreadType        string  // "ratio" or "difference" (default: "difference")
	useCointegration  bool    // Use cointegration instead of correlation (default: false)

	// State
	price1            float64
	price2            float64
	spreadMean        float64
	spreadStd         float64
	currentSpread     float64
	currentZScore     float64
	lastTradeTime     time.Time
	minTradeInterval  time.Duration

	// Spread history for statistics
	spreadHistory     []float64
	price1History     []float64
	price2History     []float64
	maxHistoryLen     int

	// Position tracking (separate for each leg)
	leg1Position      int64
	leg2Position      int64

	mu sync.RWMutex
}

// NewPairwiseArbStrategy creates a new pairs trading strategy
func NewPairwiseArbStrategy(id string) *PairwiseArbStrategy {
	pas := &PairwiseArbStrategy{
		BaseStrategy:     NewBaseStrategy(id, "pairwise_arb"),
		lookbackPeriod:   100,
		entryZScore:      2.0,
		exitZScore:       0.5,
		orderSize:        10,
		maxPositionSize:  50,
		minCorrelation:   0.7,
		hedgeRatio:       1.0,
		spreadType:       "difference",
		useCointegration: false,
		minTradeInterval: 3 * time.Second,
		maxHistoryLen:    200,
		spreadHistory:    make([]float64, 0, 200),
		price1History:    make([]float64, 0, 200),
		price2History:    make([]float64, 0, 200),
	}

	return pas
}

// Initialize initializes the strategy
func (pas *PairwiseArbStrategy) Initialize(config *StrategyConfig) error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	pas.Config = config

	// Validate we have exactly 2 symbols
	if len(config.Symbols) != 2 {
		return fmt.Errorf("pairwise arbitrage requires exactly 2 symbols")
	}

	pas.symbol1 = config.Symbols[0]
	pas.symbol2 = config.Symbols[1]

	// Load strategy-specific parameters
	if val, ok := config.Parameters["lookback_period"].(float64); ok {
		pas.lookbackPeriod = int(val)
	}
	if val, ok := config.Parameters["entry_zscore"].(float64); ok {
		pas.entryZScore = val
	}
	if val, ok := config.Parameters["exit_zscore"].(float64); ok {
		pas.exitZScore = val
	}
	if val, ok := config.Parameters["order_size"].(float64); ok {
		pas.orderSize = int64(val)
	}
	if val, ok := config.Parameters["max_position_size"].(float64); ok {
		pas.maxPositionSize = int64(val)
	}
	if val, ok := config.Parameters["min_correlation"].(float64); ok {
		pas.minCorrelation = val
	}
	if val, ok := config.Parameters["spread_type"].(string); ok {
		pas.spreadType = val
	}
	if val, ok := config.Parameters["use_cointegration"].(bool); ok {
		pas.useCointegration = val
	}
	if val, ok := config.Parameters["trade_interval_ms"].(float64); ok {
		pas.minTradeInterval = time.Duration(val) * time.Millisecond
	}

	log.Printf("[PairwiseArbStrategy:%s] Initialized %s/%s, entry_z=%.2f, exit_z=%.2f, lookback=%d",
		pas.ID, pas.symbol1, pas.symbol2, pas.entryZScore, pas.exitZScore, pas.lookbackPeriod)

	return nil
}

// OnMarketData handles market data updates
func (pas *PairwiseArbStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	if !pas.IsRunning() {
		return
	}

	// Update indicators
	pas.PrivateIndicators.UpdateAll(md)

	// Track prices for both symbols
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return
	}
	midPrice := (md.BidPrice[0] + md.AskPrice[0]) / 2.0

	if md.Symbol == pas.symbol1 {
		pas.price1 = midPrice
		pas.price1History = append(pas.price1History, midPrice)
		if len(pas.price1History) > pas.maxHistoryLen {
			pas.price1History = pas.price1History[1:]
		}
	} else if md.Symbol == pas.symbol2 {
		pas.price2 = midPrice
		pas.price2History = append(pas.price2History, midPrice)
		if len(pas.price2History) > pas.maxHistoryLen {
			pas.price2History = pas.price2History[1:]
		}
	}

	// Need both prices to calculate spread
	if pas.price1 == 0 || pas.price2 == 0 {
		return
	}

	// Calculate spread
	pas.calculateSpread()

	// Update statistics
	pas.updateStatistics()

	// Update PNL (use average price)
	avgPrice := (pas.price1 + pas.price2) / 2.0
	pas.BaseStrategy.UpdatePNL(avgPrice)
	pas.BaseStrategy.UpdateRiskMetrics(avgPrice)

	// Check if we should trade
	now := time.Now()
	if now.Sub(pas.lastTradeTime) < pas.minTradeInterval {
		return
	}

	// Check correlation before trading
	if !pas.checkCorrelation() {
		return
	}

	// Generate signals based on z-score
	pas.generateSignals(md)
	pas.lastTradeTime = now
}

// calculateSpread calculates current spread between the two instruments
func (pas *PairwiseArbStrategy) calculateSpread() {
	if pas.price1 == 0 || pas.price2 == 0 {
		return
	}

	switch pas.spreadType {
	case "ratio":
		pas.currentSpread = pas.price1 / pas.price2
	case "difference":
		fallthrough
	default:
		pas.currentSpread = pas.price1 - pas.hedgeRatio*pas.price2
	}

	// Add to history
	pas.spreadHistory = append(pas.spreadHistory, pas.currentSpread)
	if len(pas.spreadHistory) > pas.maxHistoryLen {
		pas.spreadHistory = pas.spreadHistory[1:]
	}
}

// updateStatistics updates spread mean, std, and hedge ratio
func (pas *PairwiseArbStrategy) updateStatistics() {
	n := len(pas.spreadHistory)
	if n < pas.lookbackPeriod {
		return
	}

	// Use last lookbackPeriod points
	recent := pas.spreadHistory[n-pas.lookbackPeriod:]

	// Calculate mean
	var sum float64
	for _, val := range recent {
		sum += val
	}
	pas.spreadMean = sum / float64(len(recent))

	// Calculate standard deviation
	var variance float64
	for _, val := range recent {
		diff := val - pas.spreadMean
		variance += diff * diff
	}
	variance /= float64(len(recent))
	pas.spreadStd = math.Sqrt(variance)

	// Calculate z-score
	if pas.spreadStd > 1e-10 {
		pas.currentZScore = (pas.currentSpread - pas.spreadMean) / pas.spreadStd
	}

	// Update hedge ratio if needed
	if pas.useCointegration || pas.spreadType == "difference" {
		pas.updateHedgeRatio()
	}
}

// updateHedgeRatio calculates optimal hedge ratio using regression
func (pas *PairwiseArbStrategy) updateHedgeRatio() {
	n1 := len(pas.price1History)
	n2 := len(pas.price2History)
	if n1 < pas.lookbackPeriod || n2 < pas.lookbackPeriod {
		return
	}

	// Use last lookbackPeriod points
	price1 := pas.price1History[n1-pas.lookbackPeriod:]
	price2 := pas.price2History[n2-pas.lookbackPeriod:]

	// Calculate means
	var mean1, mean2 float64
	for i := range price1 {
		mean1 += price1[i]
		mean2 += price2[i]
	}
	mean1 /= float64(len(price1))
	mean2 /= float64(len(price2))

	// Calculate covariance and variance
	var covariance, variance float64
	for i := range price1 {
		diff1 := price1[i] - mean1
		diff2 := price2[i] - mean2
		covariance += diff1 * diff2
		variance += diff2 * diff2
	}

	if variance > 1e-10 {
		// Beta = Cov(price1, price2) / Var(price2)
		beta := covariance / variance
		pas.hedgeRatio = beta
		// Clamp to reasonable range
		pas.hedgeRatio = math.Max(0.5, math.Min(2.0, pas.hedgeRatio))
	}
}

// checkCorrelation checks if correlation is sufficient for trading
func (pas *PairwiseArbStrategy) checkCorrelation() bool {
	n1 := len(pas.price1History)
	n2 := len(pas.price2History)
	if n1 < pas.lookbackPeriod || n2 < pas.lookbackPeriod {
		return false
	}

	// Use last lookbackPeriod points
	price1 := pas.price1History[n1-pas.lookbackPeriod:]
	price2 := pas.price2History[n2-pas.lookbackPeriod:]

	// Calculate Pearson correlation
	correlation := pas.calculateCorrelation(price1, price2)

	return correlation >= pas.minCorrelation
}

// calculateCorrelation calculates Pearson correlation coefficient
func (pas *PairwiseArbStrategy) calculateCorrelation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	// Calculate means
	var meanX, meanY float64
	for i := range x {
		meanX += x[i]
		meanY += y[i]
	}
	meanX /= float64(len(x))
	meanY /= float64(len(y))

	// Calculate correlation components
	var numerator, varX, varY float64
	for i := range x {
		diffX := x[i] - meanX
		diffY := y[i] - meanY
		numerator += diffX * diffY
		varX += diffX * diffX
		varY += diffY * diffY
	}

	denominator := math.Sqrt(varX * varY)
	if denominator < 1e-10 {
		return 0
	}

	return numerator / denominator
}

// generateSignals generates trading signals based on z-score
func (pas *PairwiseArbStrategy) generateSignals(md *mdpb.MarketDataUpdate) {
	if pas.spreadStd < 1e-10 {
		return
	}

	// Entry signals
	if math.Abs(pas.currentZScore) >= pas.entryZScore {
		// Spread has diverged significantly - enter mean reversion trade
		if pas.currentZScore > 0 {
			// Spread is too high - short spread (sell symbol1, buy symbol2)
			pas.generateSpreadSignals(md, "short", pas.orderSize)
		} else {
			// Spread is too low - long spread (buy symbol1, sell symbol2)
			pas.generateSpreadSignals(md, "long", pas.orderSize)
		}
		return
	}

	// Exit signals
	if pas.leg1Position != 0 && math.Abs(pas.currentZScore) <= pas.exitZScore {
		// Spread has reverted to mean - close positions
		pas.generateExitSignals(md)
	}
}

// generateSpreadSignals generates signals to enter a spread trade
func (pas *PairwiseArbStrategy) generateSpreadSignals(md *mdpb.MarketDataUpdate, direction string, qty int64) {
	// Check position limits
	if math.Abs(float64(pas.leg1Position)) >= float64(pas.maxPositionSize) {
		return
	}

	var signal1Side, signal2Side OrderSide
	if direction == "long" {
		signal1Side = OrderSideBuy
		signal2Side = OrderSideSell
	} else {
		signal1Side = OrderSideSell
		signal2Side = OrderSideBuy
	}

	// Calculate hedge quantity
	hedgeQty := int64(math.Round(float64(qty) * pas.hedgeRatio))

	// Generate signal for leg 1
	signal1 := &TradingSignal{
		StrategyID: pas.ID,
		Symbol:     pas.symbol1,
		Side:       signal1Side,
		Price:      pas.price1,
		Quantity:   qty,
		Signal:     -pas.currentZScore, // Negative z-score means buy, positive means sell
		Confidence: math.Min(1.0, math.Abs(pas.currentZScore)/5.0),
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"type":       "entry",
			"leg":        1,
			"direction":  direction,
			"z_score":    pas.currentZScore,
			"spread":     pas.currentSpread,
			"hedge_ratio": pas.hedgeRatio,
		},
	}
	pas.BaseStrategy.AddSignal(signal1)

	// Generate signal for leg 2
	signal2 := &TradingSignal{
		StrategyID: pas.ID,
		Symbol:     pas.symbol2,
		Side:       signal2Side,
		Price:      pas.price2,
		Quantity:   hedgeQty,
		Signal:     pas.currentZScore, // Opposite direction
		Confidence: math.Min(1.0, math.Abs(pas.currentZScore)/5.0),
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"type":       "entry",
			"leg":        2,
			"direction":  direction,
			"z_score":    pas.currentZScore,
			"spread":     pas.currentSpread,
			"hedge_ratio": pas.hedgeRatio,
		},
	}
	pas.BaseStrategy.AddSignal(signal2)

	log.Printf("[PairwiseArbStrategy:%s] Entering %s spread: z=%.2f, leg1=%v %d, leg2=%v %d",
		pas.ID, direction, pas.currentZScore, signal1Side, qty, signal2Side, hedgeQty)

	// Track positions (simplified - in reality would track per symbol)
	if direction == "long" {
		pas.leg1Position += qty
		pas.leg2Position -= hedgeQty
	} else {
		pas.leg1Position -= qty
		pas.leg2Position += hedgeQty
	}
}

// generateExitSignals generates signals to exit the spread trade
func (pas *PairwiseArbStrategy) generateExitSignals(md *mdpb.MarketDataUpdate) {
	if pas.leg1Position == 0 {
		return
	}

	// Close leg 1
	var signal1Side OrderSide
	qty1 := absInt64(pas.leg1Position)
	if pas.leg1Position > 0 {
		signal1Side = OrderSideSell
	} else {
		signal1Side = OrderSideBuy
	}

	signal1 := &TradingSignal{
		StrategyID: pas.ID,
		Symbol:     pas.symbol1,
		Side:       signal1Side,
		Price:      pas.price1,
		Quantity:   qty1,
		Signal:     0,
		Confidence: 0.9,
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"type":    "exit",
			"leg":     1,
			"z_score": pas.currentZScore,
		},
	}
	pas.BaseStrategy.AddSignal(signal1)

	// Close leg 2
	var signal2Side OrderSide
	qty2 := absInt64(pas.leg2Position)
	if pas.leg2Position > 0 {
		signal2Side = OrderSideSell
	} else {
		signal2Side = OrderSideBuy
	}

	signal2 := &TradingSignal{
		StrategyID: pas.ID,
		Symbol:     pas.symbol2,
		Side:       signal2Side,
		Price:      pas.price2,
		Quantity:   qty2,
		Signal:     0,
		Confidence: 0.9,
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"type":    "exit",
			"leg":     2,
			"z_score": pas.currentZScore,
		},
	}
	pas.BaseStrategy.AddSignal(signal2)

	log.Printf("[PairwiseArbStrategy:%s] Exiting spread: z=%.2f, leg1=%v %d, leg2=%v %d",
		pas.ID, pas.currentZScore, signal1Side, qty1, signal2Side, qty2)

	// Reset positions
	pas.leg1Position = 0
	pas.leg2Position = 0
}

// OnOrderUpdate handles order updates
func (pas *PairwiseArbStrategy) OnOrderUpdate(update *orspb.OrderUpdate) {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	if !pas.IsRunning() {
		return
	}

	// Update position based on order status
	pas.UpdatePosition(update)
}

// OnTimer handles timer events
func (pas *PairwiseArbStrategy) OnTimer(now time.Time) {
	pas.mu.RLock()
	defer pas.mu.RUnlock()

	// Periodic housekeeping
	if !pas.IsRunning() {
		return
	}

	// Log spread status
	if now.Unix()%30 == 0 && pas.spreadStd > 0 {
		log.Printf("[PairwiseArbStrategy:%s] Spread=%.2f (mean=%.2f, std=%.2f), Z=%.2f, Pos=[%d,%d]",
			pas.ID, pas.currentSpread, pas.spreadMean, pas.spreadStd,
			pas.currentZScore, pas.leg1Position, pas.leg2Position)
	}
}

// Start starts the strategy
func (pas *PairwiseArbStrategy) Start() error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	pas.Activate()
	log.Printf("[PairwiseArbStrategy:%s] Started", pas.ID)
	return nil
}

// Stop stops the strategy
func (pas *PairwiseArbStrategy) Stop() error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	pas.Deactivate()
	log.Printf("[PairwiseArbStrategy:%s] Stopped", pas.ID)
	return nil
}

// GetSpreadStatus returns current spread status
func (pas *PairwiseArbStrategy) GetSpreadStatus() map[string]interface{} {
	pas.mu.RLock()
	defer pas.mu.RUnlock()

	return map[string]interface{}{
		"symbol1":        pas.symbol1,
		"symbol2":        pas.symbol2,
		"price1":         pas.price1,
		"price2":         pas.price2,
		"spread":         pas.currentSpread,
		"spread_mean":    pas.spreadMean,
		"spread_std":     pas.spreadStd,
		"z_score":        pas.currentZScore,
		"hedge_ratio":    pas.hedgeRatio,
		"leg1_position":  pas.leg1Position,
		"leg2_position":  pas.leg2Position,
	}
}
