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
	bid1              float64  // 品种1买一价
	ask1              float64  // 品种1卖一价
	bid2              float64  // 品种2买一价
	ask2              float64  // 品种2卖一价
	lastTradeTime     time.Time
	minTradeInterval  time.Duration
	slippageTicks     int     // 滑点(tick数)
	useAggressivePrice bool   // 是否使用主动成交价格

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

	// 设置具体策略实例，用于参数热加载
	pas.BaseStrategy.SetConcreteStrategy(pas)

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
	// 滑点参数（支持int或float64类型）
	if val, ok := config.Parameters["slippage_ticks"].(float64); ok {
		pas.slippageTicks = int(val)
	} else if val, ok := config.Parameters["slippage_ticks"].(int); ok {
		pas.slippageTicks = val
	}
	// 是否使用主动成交价格
	if val, ok := config.Parameters["use_market_price"].(bool); ok {
		pas.useAggressivePrice = val
	}

	log.Printf("[PairwiseArbStrategy:%s] Initialized %s/%s, entry_z=%.2f, exit_z=%.2f, lookback=%d, min_corr=%.2f, slippage=%d ticks",
		pas.ID, pas.symbol1, pas.symbol2, pas.entryZScore, pas.exitZScore, pas.lookbackPeriod, pas.minCorrelation, pas.slippageTicks)

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
		pas.bid1 = md.BidPrice[0]
		pas.ask1 = md.AskPrice[0]
		pas.spreadAnalyzer.UpdatePrice1(midPrice, int64(md.Timestamp))
	} else if md.Symbol == pas.symbol2 {
		pas.price2 = midPrice
		pas.bid2 = md.BidPrice[0]
		pas.ask2 = md.AskPrice[0]
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
		// Leg 1 details
		"leg1_price":    pas.price1,
		"leg1_position": float64(pas.leg1Position),
		// Leg 2 details
		"leg2_price":    pas.price2,
		"leg2_position": float64(pas.leg2Position),
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

	// Debug logging periodically (every 5 seconds)
	if time.Since(pas.lastTradeTime) > 5*time.Second {
		log.Printf("[PairwiseArb:%s] Stats: zscore=%.2f (need ±%.2f), corr=%.3f (need %.3f), std=%.4f, ready=%v, condMet=%v",
			pas.ID, spreadStats.ZScore, pas.entryZScore, spreadStats.Correlation, pas.minCorrelation,
			spreadStats.Std, pas.spreadAnalyzer.IsReady(pas.lookbackPeriod), conditionsMet)
	}

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

	// 计算leg1的订单价格（使用bid/ask和滑点）
	orderPrice1 := GetOrderPrice(signal1Side, pas.bid1, pas.ask1, pas.symbol1,
		pas.slippageTicks, pas.useAggressivePrice)

	// Generate signal for leg 1
	signal1 := &TradingSignal{
		StrategyID: pas.ID,
		Symbol:     pas.symbol1,
		Side:       signal1Side,
		Price:      orderPrice1,  // 使用计算后的价格
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

	// 计算leg2的订单价格
	orderPrice2 := GetOrderPrice(signal2Side, pas.bid2, pas.ask2, pas.symbol2,
		pas.slippageTicks, pas.useAggressivePrice)

	// Generate signal for leg 2
	signal2 := &TradingSignal{
		StrategyID: pas.ID,
		Symbol:     pas.symbol2,
		Side:       signal2Side,
		Price:      orderPrice2,  // 使用计算后的价格
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

	// 计算平仓价格
	exitPrice1 := GetOrderPrice(signal1Side, pas.bid1, pas.ask1, pas.symbol1,
		pas.slippageTicks, pas.useAggressivePrice)

	signal1 := &TradingSignal{
		StrategyID: pas.ID,
		Symbol:     pas.symbol1,
		Side:       signal1Side,
		Price:      exitPrice1,  // 使用计算后的价格
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

	exitPrice2 := GetOrderPrice(signal2Side, pas.bid2, pas.ask2, pas.symbol2,
		pas.slippageTicks, pas.useAggressivePrice)

	signal2 := &TradingSignal{
		StrategyID: pas.ID,
		Symbol:     pas.symbol2,
		Side:       signal2Side,
		Price:      exitPrice2,  // 使用计算后的价格
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

	// Update base strategy position (for overall PNL tracking)
	pas.UpdatePosition(update)

	// Update leg-specific positions for pairwise arbitrage
	if update.Status == orspb.OrderStatus_FILLED && update.FilledQty > 0 {
		symbol := update.Symbol
		qty := int64(update.FilledQty)

		// Determine which leg this order belongs to
		if symbol == pas.symbol1 {
			// Update leg1 position
			if update.Side == orspb.OrderSide_BUY {
				pas.leg1Position += qty
			} else if update.Side == orspb.OrderSide_SELL {
				pas.leg1Position -= qty
			}
			log.Printf("[PairwiseArb:%s] Leg1 position updated: %s %s %d -> total: %d",
				pas.ID, symbol, update.Side, qty, pas.leg1Position)
		} else if symbol == pas.symbol2 {
			// Update leg2 position
			if update.Side == orspb.OrderSide_BUY {
				pas.leg2Position += qty
			} else if update.Side == orspb.OrderSide_SELL {
				pas.leg2Position -= qty
			}
			log.Printf("[PairwiseArb:%s] Leg2 position updated: %s %s %d -> total: %d",
				pas.ID, symbol, update.Side, qty, pas.leg2Position)
		}
	}
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

	// 尝试从持久化文件恢复持仓
	if snapshot, err := LoadPositionSnapshot(pas.ID); err == nil && snapshot != nil {
		log.Printf("[PairwiseArbStrategy:%s] Restoring position from snapshot (saved at %s)",
			pas.ID, snapshot.Timestamp.Format("2006-01-02 15:04:05"))

		// 恢复leg持仓
		if qty, exists := snapshot.SymbolsPos[pas.symbol1]; exists {
			pas.leg1Position = qty
			log.Printf("[PairwiseArbStrategy:%s] Restored leg1 position: %s = %d",
				pas.ID, pas.symbol1, qty)
		}
		if qty, exists := snapshot.SymbolsPos[pas.symbol2]; exists {
			pas.leg2Position = qty
			log.Printf("[PairwiseArbStrategy:%s] Restored leg2 position: %s = %d",
				pas.ID, pas.symbol2, qty)
		}

		// 恢复BaseStrategy持仓
		pas.Position.LongQty = snapshot.TotalLongQty
		pas.Position.ShortQty = snapshot.TotalShortQty
		pas.Position.NetQty = snapshot.TotalNetQty
		pas.Position.AvgLongPrice = snapshot.AvgLongPrice
		pas.Position.AvgShortPrice = snapshot.AvgShortPrice
		pas.PNL.RealizedPnL = snapshot.RealizedPnL

		log.Printf("[PairwiseArbStrategy:%s] Position restored: Long=%d, Short=%d, Net=%d",
			pas.ID, snapshot.TotalLongQty, snapshot.TotalShortQty, snapshot.TotalNetQty)
	} else if err != nil {
		log.Printf("[PairwiseArbStrategy:%s] Warning: Failed to load position snapshot: %v", pas.ID, err)
	}

	// 设置运行状态为 Active
	pas.ControlState.RunState = StrategyRunStateActive
	pas.Activate()
	log.Printf("[PairwiseArbStrategy:%s] Started", pas.ID)
	return nil
}

// ApplyParameters 应用新参数（实现 ParameterUpdatable 接口）
func (pas *PairwiseArbStrategy) ApplyParameters(params map[string]interface{}) error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	log.Printf("[PairwiseArbStrategy:%s] Applying new parameters...", pas.ID)

	// 保存旧参数（用于日志）
	oldEntryZ := pas.entryZScore
	oldExitZ := pas.exitZScore
	oldOrderSize := pas.orderSize
	oldMaxPos := pas.maxPositionSize

	// 更新参数
	updated := false

	if val, ok := params["entry_zscore"].(float64); ok {
		pas.entryZScore = val
		updated = true
	}
	if val, ok := params["exit_zscore"].(float64); ok {
		pas.exitZScore = val
		updated = true
	}
	if val, ok := params["order_size"].(int); ok {
		pas.orderSize = int64(val)
		updated = true
	} else if val, ok := params["order_size"].(float64); ok {
		pas.orderSize = int64(val)
		updated = true
	}
	if val, ok := params["max_position_size"].(int); ok {
		pas.maxPositionSize = int64(val)
		updated = true
	} else if val, ok := params["max_position_size"].(float64); ok {
		pas.maxPositionSize = int64(val)
		updated = true
	}
	if val, ok := params["lookback_period"].(int); ok {
		pas.lookbackPeriod = val
		updated = true
	} else if val, ok := params["lookback_period"].(float64); ok {
		pas.lookbackPeriod = int(val)
		updated = true
	}
	if val, ok := params["min_correlation"].(float64); ok {
		pas.minCorrelation = val
		updated = true
	}

	if !updated {
		return fmt.Errorf("no valid parameters found to update")
	}

	// 参数验证
	if pas.entryZScore <= pas.exitZScore {
		// 回滚
		pas.entryZScore = oldEntryZ
		pas.exitZScore = oldExitZ
		return fmt.Errorf("entry_zscore (%.2f) must be greater than exit_zscore (%.2f)",
			pas.entryZScore, pas.exitZScore)
	}

	if pas.orderSize <= 0 || pas.orderSize > pas.maxPositionSize {
		pas.orderSize = oldOrderSize
		pas.maxPositionSize = oldMaxPos
		return fmt.Errorf("invalid order_size (%d) or max_position_size (%d)",
			pas.orderSize, pas.maxPositionSize)
	}

	// 输出变更日志
	log.Printf("[PairwiseArbStrategy:%s] ✓ Parameters updated:", pas.ID)
	if oldEntryZ != pas.entryZScore {
		log.Printf("[PairwiseArbStrategy:%s]   entry_zscore: %.2f -> %.2f",
			pas.ID, oldEntryZ, pas.entryZScore)
	}
	if oldExitZ != pas.exitZScore {
		log.Printf("[PairwiseArbStrategy:%s]   exit_zscore: %.2f -> %.2f",
			pas.ID, oldExitZ, pas.exitZScore)
	}
	if oldOrderSize != pas.orderSize {
		log.Printf("[PairwiseArbStrategy:%s]   order_size: %d -> %d",
			pas.ID, oldOrderSize, pas.orderSize)
	}
	if oldMaxPos != pas.maxPositionSize {
		log.Printf("[PairwiseArbStrategy:%s]   max_position_size: %d -> %d",
			pas.ID, oldMaxPos, pas.maxPositionSize)
	}

	return nil
}

// GetCurrentParameters 获取当前参数（用于API查询）
func (pas *PairwiseArbStrategy) GetCurrentParameters() map[string]interface{} {
	pas.mu.RLock()
	defer pas.mu.RUnlock()

	return map[string]interface{}{
		"entry_zscore":       pas.entryZScore,
		"exit_zscore":        pas.exitZScore,
		"order_size":         pas.orderSize,
		"max_position_size":  pas.maxPositionSize,
		"lookback_period":    pas.lookbackPeriod,
		"min_correlation":    pas.minCorrelation,
		"hedge_ratio":        pas.hedgeRatio,
		"spread_type":        pas.spreadType,
		"use_cointegration":  pas.useCointegration,
	}
}

// Stop stops the strategy
func (pas *PairwiseArbStrategy) Stop() error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	// 保存当前持仓到文件
	snapshot := PositionSnapshot{
		StrategyID:    pas.ID,
		Timestamp:     time.Now(),
		TotalLongQty:  pas.Position.LongQty,
		TotalShortQty: pas.Position.ShortQty,
		TotalNetQty:   pas.Position.NetQty,
		AvgLongPrice:  pas.Position.AvgLongPrice,
		AvgShortPrice: pas.Position.AvgShortPrice,
		RealizedPnL:   pas.PNL.RealizedPnL,
		SymbolsPos: map[string]int64{
			pas.symbol1: pas.leg1Position,
			pas.symbol2: pas.leg2Position,
		},
	}

	if err := SavePositionSnapshot(snapshot); err != nil {
		log.Printf("[PairwiseArbStrategy:%s] Warning: Failed to save position snapshot: %v", pas.ID, err)
		// 不阻断停止流程
	} else {
		log.Printf("[PairwiseArbStrategy:%s] Position snapshot saved: Long=%d, Short=%d, Net=%d",
			pas.ID, snapshot.TotalLongQty, snapshot.TotalShortQty, snapshot.TotalNetQty)
	}

	pas.ControlState.RunState = StrategyRunStateStopped
	pas.Deactivate()
	log.Printf("[PairwiseArbStrategy:%s] Stopped", pas.ID)
	return nil
}

// InitializePositions 实现PositionInitializer接口：从外部初始化持仓
func (pas *PairwiseArbStrategy) InitializePositions(positions map[string]int64) error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	log.Printf("[PairwiseArbStrategy:%s] Initializing positions from external source", pas.ID)

	// 初始化leg持仓
	if qty, exists := positions[pas.symbol1]; exists {
		pas.leg1Position = qty
		log.Printf("[PairwiseArbStrategy:%s] Initialized leg1 position: %s = %d",
			pas.ID, pas.symbol1, qty)
	}

	if qty, exists := positions[pas.symbol2]; exists {
		pas.leg2Position = qty
		log.Printf("[PairwiseArbStrategy:%s] Initialized leg2 position: %s = %d",
			pas.ID, pas.symbol2, qty)
	}

	// 更新BaseStrategy的Position（简化处理）
	totalQty := pas.leg1Position + pas.leg2Position
	if totalQty > 0 {
		pas.Position.LongQty = totalQty
		pas.Position.NetQty = totalQty
	} else if totalQty < 0 {
		pas.Position.ShortQty = -totalQty
		pas.Position.NetQty = totalQty
	}

	log.Printf("[PairwiseArbStrategy:%s] Positions initialized: leg1=%d, leg2=%d, net=%d",
		pas.ID, pas.leg1Position, pas.leg2Position, pas.Position.NetQty)

	return nil
}

// GetPositionsBySymbol 实现PositionProvider接口：返回按品种的持仓
func (pas *PairwiseArbStrategy) GetPositionsBySymbol() map[string]int64 {
	pas.mu.RLock()
	defer pas.mu.RUnlock()

	return map[string]int64{
		pas.symbol1: pas.leg1Position,
		pas.symbol2: pas.leg2Position,
	}
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

// GetLegsInfo returns detailed information for each leg (for UI display)
func (pas *PairwiseArbStrategy) GetLegsInfo() []map[string]interface{} {
	pas.mu.RLock()
	defer pas.mu.RUnlock()

	// Determine side for each leg
	leg1Side := "flat"
	if pas.leg1Position > 0 {
		leg1Side = "long"
	} else if pas.leg1Position < 0 {
		leg1Side = "short"
	}

	leg2Side := "flat"
	if pas.leg2Position > 0 {
		leg2Side = "long"
	} else if pas.leg2Position < 0 {
		leg2Side = "short"
	}

	return []map[string]interface{}{
		{
			"symbol":   pas.symbol1,
			"price":    pas.price1,
			"position": pas.leg1Position,
			"side":     leg1Side,
		},
		{
			"symbol":   pas.symbol2,
			"price":    pas.price2,
			"position": pas.leg2Position,
			"side":     leg2Side,
		},
	}
}

// GetBaseStrategy returns the underlying BaseStrategy (for engine integration)
func (pas *PairwiseArbStrategy) GetBaseStrategy() *BaseStrategy {
	return pas.BaseStrategy
}
