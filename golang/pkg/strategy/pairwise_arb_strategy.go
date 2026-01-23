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
	"github.com/yourusername/quantlink-trade-system/pkg/strategy/spread"
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
	lastTradeTime     time.Time
	minTradeInterval  time.Duration

	// Spread analyzer (encapsulates spread calculation and statistics)
	spreadAnalyzer    *spread.SpreadAnalyzer

	// Position tracking (separate for each leg)
	leg1Position      int64
	leg2Position      int64

	mu sync.RWMutex
}

// NewPairwiseArbStrategy creates a new pairs trading strategy
func NewPairwiseArbStrategy(id string) *PairwiseArbStrategy {
	maxHistoryLen := 200

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
		// SpreadAnalyzer 将在 Initialize 中创建（需要知道 symbol 名称）
		spreadAnalyzer:   nil,
	}

	// 预创建一个临时的 SpreadAnalyzer（将在 Initialize 时重新创建）
	pas.spreadAnalyzer = spread.NewSpreadAnalyzer("", "", spread.SpreadTypeDifference, maxHistoryLen)

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

	// Load strategy-specific parameters (load spread_type first)
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

	// 初始化 SpreadAnalyzer（现在知道 symbol 和 spread_type 了）
	spreadType := spread.SpreadTypeDifference
	if pas.spreadType == "ratio" {
		spreadType = spread.SpreadTypeRatio
	}
	pas.spreadAnalyzer = spread.NewSpreadAnalyzer(pas.symbol1, pas.symbol2, spreadType, 200)
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
		pas.spreadAnalyzer.UpdatePrice1(midPrice, int64(md.Timestamp))
	} else if md.Symbol == pas.symbol2 {
		pas.price2 = midPrice
		pas.spreadAnalyzer.UpdatePrice2(midPrice, int64(md.Timestamp))
	}

	// Need both prices to calculate spread
	if pas.price1 == 0 || pas.price2 == 0 {
		return
	}

	// Calculate spread and update statistics using SpreadAnalyzer
	pas.spreadAnalyzer.CalculateSpread()
	pas.spreadAnalyzer.UpdateAll(pas.lookbackPeriod)

	// Update PNL (use average price)
	avgPrice := (pas.price1 + pas.price2) / 2.0
	pas.BaseStrategy.UpdatePNL(avgPrice)
	pas.BaseStrategy.UpdateRiskMetrics(avgPrice)

	// Get current statistics from SpreadAnalyzer
	spreadStats := pas.spreadAnalyzer.GetStats()

	// Update condition state for UI display
	indicators := map[string]float64{
		"z_score":         spreadStats.ZScore,
		"entry_threshold": pas.entryZScore,
		"exit_threshold":  pas.exitZScore,
		"spread":          spreadStats.CurrentSpread,
		"spread_mean":     spreadStats.Mean,
		"spread_std":      spreadStats.Std,
		"correlation":     spreadStats.Correlation,
		"min_correlation": pas.minCorrelation,
		"hedge_ratio":     spreadStats.HedgeRatio,
		"price1":          pas.price1,
		"price2":          pas.price2,
	}

	// Conditions are met if:
	// 1. Z-score exceeds entry threshold
	// 2. Correlation is above minimum
	// 3. Enough history data
	conditionsMet := spreadStats.Std > 1e-10 &&
		math.Abs(spreadStats.ZScore) >= pas.entryZScore &&
		spreadStats.Correlation >= pas.minCorrelation &&
		pas.spreadAnalyzer.IsReady(pas.lookbackPeriod)

	// Update control state with current conditions
	pas.ControlState.UpdateConditions(conditionsMet, spreadStats.ZScore, indicators)

	// Check if we should trade
	now := time.Now()
	if now.Sub(pas.lastTradeTime) < pas.minTradeInterval {
		return
	}

	// Check correlation before trading
	if spreadStats.Correlation < pas.minCorrelation {
		return
	}

	// Generate signals based on z-score
	pas.generateSignals(md)
	pas.lastTradeTime = now
}


// generateSignals generates trading signals based on z-score
func (pas *PairwiseArbStrategy) generateSignals(md *mdpb.MarketDataUpdate) {
	// Get current statistics from SpreadAnalyzer
	spreadStats := pas.spreadAnalyzer.GetStats()

	if spreadStats.Std < 1e-10 {
		return
	}

	// Entry signals
	if math.Abs(spreadStats.ZScore) >= pas.entryZScore {
		// Spread has diverged significantly - enter mean reversion trade
		if spreadStats.ZScore > 0 {
			// Spread is too high - short spread (sell symbol1, buy symbol2)
			pas.generateSpreadSignals(md, "short", pas.orderSize)
		} else {
			// Spread is too low - long spread (buy symbol1, sell symbol2)
			pas.generateSpreadSignals(md, "long", pas.orderSize)
		}
		return
	}

	// Exit signals
	if pas.leg1Position != 0 && math.Abs(spreadStats.ZScore) <= pas.exitZScore {
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

	// Get current statistics from SpreadAnalyzer
	spreadStats := pas.spreadAnalyzer.GetStats()

	var signal1Side, signal2Side OrderSide
	if direction == "long" {
		signal1Side = OrderSideBuy
		signal2Side = OrderSideSell
	} else {
		signal1Side = OrderSideSell
		signal2Side = OrderSideBuy
	}

	// Calculate hedge quantity using current hedge ratio
	hedgeQty := int64(math.Round(float64(qty) * spreadStats.HedgeRatio))

	// Generate signal for leg 1
	signal1 := &TradingSignal{
		StrategyID: pas.ID,
		Symbol:     pas.symbol1,
		Side:       signal1Side,
		Price:      pas.price1,
		Quantity:   qty,
		Signal:     -spreadStats.ZScore, // Negative z-score means buy, positive means sell
		Confidence: math.Min(1.0, math.Abs(spreadStats.ZScore)/5.0),
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"type":        "entry",
			"leg":         1,
			"direction":   direction,
			"z_score":     spreadStats.ZScore,
			"spread":      spreadStats.CurrentSpread,
			"hedge_ratio": spreadStats.HedgeRatio,
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
		Signal:     spreadStats.ZScore, // Opposite direction
		Confidence: math.Min(1.0, math.Abs(spreadStats.ZScore)/5.0),
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"type":        "entry",
			"leg":         2,
			"direction":   direction,
			"z_score":     spreadStats.ZScore,
			"spread":      spreadStats.CurrentSpread,
			"hedge_ratio": spreadStats.HedgeRatio,
		},
	}
	pas.BaseStrategy.AddSignal(signal2)

	log.Printf("[PairwiseArbStrategy:%s] Entering %s spread: z=%.2f, leg1=%v %d, leg2=%v %d",
		pas.ID, direction, spreadStats.ZScore, signal1Side, qty, signal2Side, hedgeQty)

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

	// Get current z-score
	zScore := pas.spreadAnalyzer.GetZScore()

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
			"z_score": zScore,
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
			"z_score": zScore,
		},
	}
	pas.BaseStrategy.AddSignal(signal2)

	log.Printf("[PairwiseArbStrategy:%s] Exiting spread: z=%.2f, leg1=%v %d, leg2=%v %d",
		pas.ID, zScore, signal1Side, qty1, signal2Side, qty2)

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
	stats := pas.spreadAnalyzer.GetStats()
	if now.Unix()%30 == 0 && stats.Std > 0 {
		log.Printf("[PairwiseArbStrategy:%s] Spread=%.2f (mean=%.2f, std=%.2f), Z=%.2f, Pos=[%d,%d]",
			pas.ID, stats.CurrentSpread, stats.Mean, stats.Std,
			stats.ZScore, pas.leg1Position, pas.leg2Position)
	}
}

// Start starts the strategy
func (pas *PairwiseArbStrategy) Start() error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	// 设置运行状态为 Active
	pas.ControlState.RunState = StrategyRunStateActive
	pas.Activate()
	log.Printf("[PairwiseArbStrategy:%s] Started", pas.ID)
	return nil
}

// Stop stops the strategy
func (pas *PairwiseArbStrategy) Stop() error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	pas.ControlState.RunState = StrategyRunStateStopped
	pas.Deactivate()
	log.Printf("[PairwiseArbStrategy:%s] Stopped", pas.ID)
	return nil
}

// GetSpreadStatus returns current spread status
func (pas *PairwiseArbStrategy) GetSpreadStatus() map[string]interface{} {
	pas.mu.RLock()
	defer pas.mu.RUnlock()

	stats := pas.spreadAnalyzer.GetStats()
	return map[string]interface{}{
		"symbol1":        pas.symbol1,
		"symbol2":        pas.symbol2,
		"price1":         pas.price1,
		"price2":         pas.price2,
		"spread":         stats.CurrentSpread,
		"spread_mean":    stats.Mean,
		"spread_std":     stats.Std,
		"z_score":        stats.ZScore,
		"hedge_ratio":    stats.HedgeRatio,
		"leg1_position":  pas.leg1Position,
		"leg2_position":  pas.leg2Position,
	}
}

// GetBaseStrategy returns the underlying BaseStrategy (for engine integration)
func (pas *PairwiseArbStrategy) GetBaseStrategy() *BaseStrategy {
	return pas.BaseStrategy
}
