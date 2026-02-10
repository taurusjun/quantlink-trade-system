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

// HedgingStrategy implements a delta-neutral hedging strategy
// It maintains a hedge position to offset risk from a primary position
//
// C++: class HedgingStrategy : public ExecutionStrategy
// Go:  type HedgingStrategy struct { *ExecutionStrategy, *StrategyDataContext }
type HedgingStrategy struct {
	*ExecutionStrategy   // C++: public ExecutionStrategy（C++ 字段）
	*StrategyDataContext // Go 特有字段（指标、配置、状态等）

	// Strategy parameters
	primarySymbol        string  // Primary symbol to hedge (e.g., "ag2412")
	hedgeSymbol          string  // Hedge symbol (e.g., "ag2501")
	hedgeRatio           float64 // Hedge ratio (default: 1.0)
	rebalanceThreshold   float64 // Rebalance when delta exceeds threshold (default: 0.1)
	orderSize            int64   // Size per rebalancing order (default: 10)
	maxPositionSize      int64   // Maximum position size (default: 100)
	minSpread            float64 // Minimum spread to hedge (default: 1.0)
	dynamicHedgeRatio    bool    // Calculate hedge ratio dynamically (default: true)
	correlationPeriod    int     // Period for correlation calculation (default: 100)

	// State
	primaryPrice         float64
	hedgePrice           float64
	targetDelta          float64
	currentDelta         float64
	lastRebalanceTime    time.Time
	minRebalanceInterval time.Duration

	// Price history for correlation
	primaryHistory []float64
	hedgeHistory   []float64
	maxHistoryLen  int

	// 持仓和PNL（这些字段用于策略内部计算）
	estimatedPosition *EstimatedPosition
	pnl               *PNL
	riskMetrics       *RiskMetrics

	mu sync.RWMutex
}

// NewHedgingStrategy creates a new hedging strategy
// C++: HedgingStrategy::HedgingStrategy(CommonClient*, SimConfig*)
func NewHedgingStrategy(id string) *HedgingStrategy {
	strategyID := int32(hashStringToInt(id))
	baseExecStrategy := NewExecutionStrategy(strategyID, &Instrument{Symbol: "", TickSize: 1.0})

	hs := &HedgingStrategy{
		ExecutionStrategy:    baseExecStrategy,
		StrategyDataContext:  NewStrategyDataContext(id, "hedging"),
		hedgeRatio:           1.0,
		rebalanceThreshold:   0.1,
		orderSize:            10,
		maxPositionSize:      100,
		minSpread:            1.0,
		dynamicHedgeRatio:    true,
		correlationPeriod:    100,
		targetDelta:          0.0, // Delta-neutral target
		minRebalanceInterval: 5 * time.Second,
		maxHistoryLen:        200,
		primaryHistory:       make([]float64, 0, 200),
		hedgeHistory:         make([]float64, 0, 200),
		estimatedPosition:    &EstimatedPosition{},
		pnl:                  &PNL{},
		riskMetrics:          &RiskMetrics{},
	}

	return hs
}

// Initialize initializes the strategy
func (hs *HedgingStrategy) Initialize(config *StrategyConfig) error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	hs.Config = config

	// Validate we have at least 2 symbols
	if len(config.Symbols) < 2 {
		return fmt.Errorf("hedging strategy requires at least 2 symbols (primary and hedge)")
	}

	hs.primarySymbol = config.Symbols[0]
	hs.hedgeSymbol = config.Symbols[1]

	// Load strategy-specific parameters
	if val, ok := config.Parameters["hedge_ratio"].(float64); ok {
		hs.hedgeRatio = val
	}
	if val, ok := config.Parameters["rebalance_threshold"].(float64); ok {
		hs.rebalanceThreshold = val
	}
	if val, ok := config.Parameters["order_size"].(float64); ok {
		hs.orderSize = int64(val)
	}
	if val, ok := config.Parameters["max_position_size"].(float64); ok {
		hs.maxPositionSize = int64(val)
	}
	if val, ok := config.Parameters["min_spread"].(float64); ok {
		hs.minSpread = val
	}
	if val, ok := config.Parameters["dynamic_hedge_ratio"].(bool); ok {
		hs.dynamicHedgeRatio = val
	}
	if val, ok := config.Parameters["correlation_period"].(float64); ok {
		hs.correlationPeriod = int(val)
	}
	if val, ok := config.Parameters["target_delta"].(float64); ok {
		hs.targetDelta = val
	}
	if val, ok := config.Parameters["rebalance_interval_ms"].(float64); ok {
		hs.minRebalanceInterval = time.Duration(val) * time.Millisecond
	}

	// Create spread indicator for hedging pair
	spreadConfig := map[string]interface{}{
		"max_history": 200.0,
	}
	_, err := hs.PrivateIndicators.Create("hedge_spread", "spread", spreadConfig)
	if err != nil {
		return fmt.Errorf("failed to create spread indicator: %w", err)
	}

	log.Printf("[HedgingStrategy:%s] Initialized primary=%s, hedge=%s, ratio=%.2f, threshold=%.2f",
		hs.ID, hs.primarySymbol, hs.hedgeSymbol, hs.hedgeRatio, hs.rebalanceThreshold)

	return nil
}

// OnMarketData handles market data updates
func (hs *HedgingStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if !hs.IsRunning() {
		return
	}

	// Update indicators
	hs.PrivateIndicators.UpdateAll(md)

	// Track prices for both symbols
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return
	}
	midPrice := (md.BidPrice[0] + md.AskPrice[0]) / 2.0

	if md.Symbol == hs.primarySymbol {
		hs.primaryPrice = midPrice
		hs.primaryHistory = append(hs.primaryHistory, midPrice)
		if len(hs.primaryHistory) > hs.maxHistoryLen {
			hs.primaryHistory = hs.primaryHistory[1:]
		}
	} else if md.Symbol == hs.hedgeSymbol {
		hs.hedgePrice = midPrice
		hs.hedgeHistory = append(hs.hedgeHistory, midPrice)
		if len(hs.hedgeHistory) > hs.maxHistoryLen {
			hs.hedgeHistory = hs.hedgeHistory[1:]
		}
	}

	// Update PNL (简化：假设使用 midPrice 作为 bid/ask)
	avgPrice := (hs.primaryPrice + hs.hedgePrice) / 2.0
	if avgPrice > 0 {
		hs.updatePNL(avgPrice, avgPrice) // 简化处理，使用相同价格
		hs.updateRiskMetrics(avgPrice)
	}

	// Check if we should rebalance
	now := time.Now()
	if now.Sub(hs.lastRebalanceTime) < hs.minRebalanceInterval {
		return
	}

	// Calculate dynamic hedge ratio if enabled
	if hs.dynamicHedgeRatio && len(hs.primaryHistory) >= hs.correlationPeriod {
		hs.updateHedgeRatio()
	}

	// Calculate current delta
	hs.calculateDelta()

	// Check if rebalancing is needed
	deltaDeviation := math.Abs(hs.currentDelta - hs.targetDelta)
	if deltaDeviation > hs.rebalanceThreshold {
		hs.rebalance(md)
		hs.lastRebalanceTime = now
	}
}

// updateHedgeRatio calculates optimal hedge ratio using regression
func (hs *HedgingStrategy) updateHedgeRatio() {
	n := len(hs.primaryHistory)
	if n < hs.correlationPeriod || len(hs.hedgeHistory) < hs.correlationPeriod {
		return
	}

	// Use last correlationPeriod points
	primary := hs.primaryHistory[n-hs.correlationPeriod:]
	hedge := hs.hedgeHistory[len(hs.hedgeHistory)-hs.correlationPeriod:]

	// Calculate returns
	primaryReturns := make([]float64, len(primary)-1)
	hedgeReturns := make([]float64, len(hedge)-1)
	for i := 1; i < len(primary); i++ {
		if primary[i-1] != 0 {
			primaryReturns[i-1] = (primary[i] - primary[i-1]) / primary[i-1]
		}
		if hedge[i-1] != 0 {
			hedgeReturns[i-1] = (hedge[i] - hedge[i-1]) / hedge[i-1]
		}
	}

	// Calculate means
	var primaryMean, hedgeMean float64
	for i := range primaryReturns {
		primaryMean += primaryReturns[i]
		hedgeMean += hedgeReturns[i]
	}
	primaryMean /= float64(len(primaryReturns))
	hedgeMean /= float64(len(hedgeReturns))

	// Calculate covariance and variance
	var covariance, variance float64
	for i := range primaryReturns {
		primaryDiff := primaryReturns[i] - primaryMean
		hedgeDiff := hedgeReturns[i] - hedgeMean
		covariance += primaryDiff * hedgeDiff
		variance += hedgeDiff * hedgeDiff
	}

	if variance > 1e-10 {
		// Beta = Cov(primary, hedge) / Var(hedge)
		beta := covariance / variance
		// Hedge ratio is beta (how much hedge per unit primary)
		hs.hedgeRatio = math.Abs(beta)
		// Clamp to reasonable range
		hs.hedgeRatio = math.Max(0.5, math.Min(2.0, hs.hedgeRatio))
	}
}

// calculateDelta calculates current portfolio delta
func (hs *HedgingStrategy) calculateDelta() {
	// Delta = primary_position + hedge_ratio * hedge_position
	// For delta-neutral: Delta = 0
	// We track positions separately but for simplicity, use net position
	primaryDelta := float64(hs.estimatedPosition.NetQty)
	hedgeDelta := 0.0 // Would need separate tracking per symbol
	hs.currentDelta = primaryDelta + hs.hedgeRatio*hedgeDelta
}

// rebalance generates signals to rebalance the hedge
func (hs *HedgingStrategy) rebalance(md *mdpb.MarketDataUpdate) {
	// Calculate required hedge adjustment
	deltaError := hs.currentDelta - hs.targetDelta
	hedgeAdjustment := -deltaError / hs.hedgeRatio

	// Round to order size
	hedgeQty := int64(math.Round(hedgeAdjustment/float64(hs.orderSize))) * hs.orderSize
	if hedgeQty == 0 {
		return
	}

	// Check position limits
	if math.Abs(float64(hs.estimatedPosition.NetQty+hedgeQty)) > float64(hs.maxPositionSize) {
		return
	}

	// Check spread is reasonable
	spreadInd, ok := hs.GetIndicator("hedge_spread")
	if ok && spreadInd.IsReady() {
		spread := spreadInd.GetValue()
		if spread < hs.minSpread {
			log.Printf("[HedgingStrategy:%s] Spread %.2f below minimum %.2f, skipping rebalance",
				hs.ID, spread, hs.minSpread)
			return
		}
	}

	// Generate hedge signal
	var side OrderSide
	var price float64
	if hedgeQty > 0 {
		side = OrderSideBuy
		price = md.AskPrice[0] // Take the ask
	} else {
		side = OrderSideSell
		price = md.BidPrice[0] // Hit the bid
		hedgeQty = -hedgeQty
	}

	signal := &TradingSignal{
		StrategyID: hs.ID,
		Symbol:     md.Symbol,
		Side:       side,
		Price:      price,
		Quantity:   hedgeQty,
		Signal:     0.0, // Hedging is neutral
		Confidence: 0.8,
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"type":         "rebalance",
			"delta_before": hs.currentDelta,
			"delta_target": hs.targetDelta,
			"hedge_ratio":  hs.hedgeRatio,
		},
	}

	hs.AddSignal(signal)
	log.Printf("[HedgingStrategy:%s] Rebalancing: delta=%.2f, target=%.2f, hedge_qty=%d, ratio=%.2f",
		hs.ID, hs.currentDelta, hs.targetDelta, signal.Quantity, hs.hedgeRatio)
}

// OnOrderUpdate handles order updates
func (hs *HedgingStrategy) OnOrderUpdate(update *orspb.OrderUpdate) {
	// CRITICAL: 检查订单是否属于本策略
	if update.StrategyId != hs.ID {
		return
	}

	hs.mu.Lock()
	defer hs.mu.Unlock()

	if !hs.IsRunning() {
		return
	}

	// Update position based on order status
	hs.updatePosition(update)
}

// OnTimer handles timer events
func (hs *HedgingStrategy) OnTimer(now time.Time) {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	// Periodic housekeeping
	if !hs.IsRunning() {
		return
	}

	// Log hedge status
	if now.Unix()%30 == 0 {
		log.Printf("[HedgingStrategy:%s] Delta=%.2f (target=%.2f), HedgeRatio=%.2f, Position=%d",
			hs.ID, hs.currentDelta, hs.targetDelta, hs.hedgeRatio, hs.estimatedPosition.NetQty)
	}
}

// Start starts the strategy
func (hs *HedgingStrategy) Start() error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	hs.ControlState.RunState = StrategyRunStateActive
	hs.Activate()
	log.Printf("[HedgingStrategy:%s] Started", hs.ID)
	return nil
}

// Stop stops the strategy
func (hs *HedgingStrategy) Stop() error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	hs.ControlState.RunState = StrategyRunStateStopped
	hs.Deactivate()
	log.Printf("[HedgingStrategy:%s] Stopped", hs.ID)
	return nil
}

// GetHedgeStatus returns current hedge status
func (hs *HedgingStrategy) GetHedgeStatus() map[string]interface{} {
	hs.mu.RLock()
	defer hs.mu.RUnlock()

	return map[string]interface{}{
		"primary_symbol":  hs.primarySymbol,
		"hedge_symbol":    hs.hedgeSymbol,
		"primary_price":   hs.primaryPrice,
		"hedge_price":     hs.hedgePrice,
		"hedge_ratio":     hs.hedgeRatio,
		"current_delta":   hs.currentDelta,
		"target_delta":    hs.targetDelta,
		"delta_deviation": math.Abs(hs.currentDelta - hs.targetDelta),
	}
}

// === C++ 虚函数对应 ===

// Reset resets the strategy to initial state
func (hs *HedgingStrategy) Reset() {
	hs.ExecutionStrategy.Reset()
	hs.estimatedPosition = &EstimatedPosition{}
	hs.pnl = &PNL{}
	hs.riskMetrics = &RiskMetrics{}
}

// SendOrder generates and sends orders based on current state
func (hs *HedgingStrategy) SendOrder() {
	// HedgingStrategy generates signals in rebalance() called from OnMarketData
}

// OnTradeUpdate is called after a trade is processed
func (hs *HedgingStrategy) OnTradeUpdate() {
	// Default: no action
}

// CheckSquareoff checks if position needs to be squared off
func (hs *HedgingStrategy) CheckSquareoff() {
	// Check risk limits
}

// HandleSquareON handles square off initiation
func (hs *HedgingStrategy) HandleSquareON() {
	hs.ControlState.FlattenMode = true
}

// HandleSquareoff executes the square off logic
func (hs *HedgingStrategy) HandleSquareoff() {
	hs.ExecutionStrategy.HandleSquareoff()
}

// SetThresholds sets dynamic thresholds based on position
func (hs *HedgingStrategy) SetThresholds() {
	// Hedging strategy uses fixed thresholds
}

// OnAuctionData handles auction period market data
func (hs *HedgingStrategy) OnAuctionData(md *mdpb.MarketDataUpdate) {
	// Hedging strategy ignores auction data
}

// === Engine/Manager 需要的方法 ===

// GetControlState returns the strategy control state
func (hs *HedgingStrategy) GetControlState() *StrategyControlState {
	return hs.ControlState
}

// GetConfig returns the strategy configuration
func (hs *HedgingStrategy) GetConfig() *StrategyConfig {
	return hs.Config
}

// CanSendOrder returns true if strategy can send orders
func (hs *HedgingStrategy) CanSendOrder() bool {
	return hs.IsRunning() && hs.ControlState.IsActivated() && !hs.ControlState.FlattenMode
}

// GetEstimatedPosition returns current estimated position
func (hs *HedgingStrategy) GetEstimatedPosition() *EstimatedPosition {
	return hs.estimatedPosition
}

// GetPosition returns current position (alias)
func (hs *HedgingStrategy) GetPosition() *EstimatedPosition {
	return hs.estimatedPosition
}

// GetPNL returns current P&L
func (hs *HedgingStrategy) GetPNL() *PNL {
	return hs.pnl
}

// GetRiskMetrics returns risk metrics
func (hs *HedgingStrategy) GetRiskMetrics() *RiskMetrics {
	return hs.riskMetrics
}

// GetStatus returns strategy status
func (hs *HedgingStrategy) GetStatus() *StrategyStatus {
	hs.Status.IsRunning = hs.ControlState.IsActivated() && hs.ControlState.RunState != StrategyRunStateStopped
	hs.Status.EstimatedPosition = hs.estimatedPosition
	hs.Status.PNL = hs.pnl
	hs.Status.RiskMetrics = hs.riskMetrics
	return hs.Status
}

// TriggerFlatten triggers position flattening
func (hs *HedgingStrategy) TriggerFlatten(reason FlattenReason, aggressive bool) {
	hs.ControlState.FlattenMode = true
}

// GetPendingCancels returns orders pending cancellation
func (hs *HedgingStrategy) GetPendingCancels() []*orspb.OrderUpdate {
	return nil // HedgingStrategy doesn't track pending cancels
}

// UpdateParameters updates strategy parameters
func (hs *HedgingStrategy) UpdateParameters(params map[string]interface{}) error {
	hs.mu.Lock()
	defer hs.mu.Unlock()

	if v, ok := params["hedge_ratio"].(float64); ok {
		hs.hedgeRatio = v
	}
	if v, ok := params["rebalance_threshold"].(float64); ok {
		hs.rebalanceThreshold = v
	}
	if v, ok := params["order_size"].(float64); ok {
		hs.orderSize = int64(v)
	}
	if v, ok := params["max_position_size"].(float64); ok {
		hs.maxPositionSize = int64(v)
	}
	return nil
}

// GetCurrentParameters returns current strategy parameters
func (hs *HedgingStrategy) GetCurrentParameters() map[string]interface{} {
	hs.mu.RLock()
	defer hs.mu.RUnlock()
	return map[string]interface{}{
		"hedge_ratio":         hs.hedgeRatio,
		"rebalance_threshold": hs.rebalanceThreshold,
		"order_size":          hs.orderSize,
		"max_position_size":   hs.maxPositionSize,
		"target_delta":        hs.targetDelta,
		"dynamic_hedge_ratio": hs.dynamicHedgeRatio,
	}
}

// === 内部辅助方法 ===

// updatePNL updates P&L based on current market price
func (hs *HedgingStrategy) updatePNL(bidPrice, askPrice float64) {
	var unrealizedPnL float64 = 0

	if hs.estimatedPosition.NetQty > 0 {
		unrealizedPnL = float64(hs.estimatedPosition.NetQty) * (bidPrice - hs.estimatedPosition.BuyAvgPrice)
	} else if hs.estimatedPosition.NetQty < 0 {
		unrealizedPnL = float64(-hs.estimatedPosition.NetQty) * (hs.estimatedPosition.SellAvgPrice - askPrice)
	}

	hs.pnl.UnrealizedPnL = unrealizedPnL
	hs.pnl.TotalPnL = hs.pnl.RealizedPnL + hs.pnl.UnrealizedPnL
	hs.pnl.NetPnL = hs.pnl.TotalPnL - hs.pnl.TradingFees
	hs.pnl.Timestamp = time.Now()
}

// updateRiskMetrics updates risk metrics
func (hs *HedgingStrategy) updateRiskMetrics(currentPrice float64) {
	hs.riskMetrics.PositionSize = abs(hs.estimatedPosition.NetQty)
	hs.riskMetrics.MaxPositionSize = hs.maxPositionSize
	hs.riskMetrics.ExposureValue = float64(hs.riskMetrics.PositionSize) * currentPrice
	hs.riskMetrics.Timestamp = time.Now()
}

// updatePosition updates position based on order update
func (hs *HedgingStrategy) updatePosition(update *orspb.OrderUpdate) {
	hs.Orders[update.OrderId] = update

	if update.Status != orspb.OrderStatus_FILLED {
		return
	}

	qty := update.FilledQty
	price := update.AvgPrice

	if update.Side == orspb.OrderSide_BUY {
		hs.estimatedPosition.BuyTotalQty += qty
		hs.estimatedPosition.BuyTotalValue += float64(qty) * price

		if hs.estimatedPosition.NetQty < 0 {
			closedQty := qty
			if closedQty > hs.estimatedPosition.SellQty {
				closedQty = hs.estimatedPosition.SellQty
			}
			realizedPnL := (hs.estimatedPosition.SellAvgPrice - price) * float64(closedQty)
			hs.pnl.RealizedPnL += realizedPnL
			hs.estimatedPosition.SellQty -= closedQty
			hs.estimatedPosition.NetQty += closedQty
			qty -= closedQty
			if hs.estimatedPosition.SellQty == 0 {
				hs.estimatedPosition.SellAvgPrice = 0
			}
		}

		if qty > 0 {
			totalCost := hs.estimatedPosition.BuyAvgPrice * float64(hs.estimatedPosition.BuyQty)
			totalCost += price * float64(qty)
			hs.estimatedPosition.BuyQty += qty
			hs.estimatedPosition.NetQty += qty
			if hs.estimatedPosition.BuyQty > 0 {
				hs.estimatedPosition.BuyAvgPrice = totalCost / float64(hs.estimatedPosition.BuyQty)
			}
		}
	} else {
		hs.estimatedPosition.SellTotalQty += qty
		hs.estimatedPosition.SellTotalValue += float64(qty) * price

		if hs.estimatedPosition.NetQty > 0 {
			closedQty := qty
			if closedQty > hs.estimatedPosition.BuyQty {
				closedQty = hs.estimatedPosition.BuyQty
			}
			realizedPnL := (price - hs.estimatedPosition.BuyAvgPrice) * float64(closedQty)
			hs.pnl.RealizedPnL += realizedPnL
			hs.estimatedPosition.BuyQty -= closedQty
			hs.estimatedPosition.NetQty -= closedQty
			qty -= closedQty
			if hs.estimatedPosition.BuyQty == 0 {
				hs.estimatedPosition.BuyAvgPrice = 0
			}
		}

		if qty > 0 {
			totalCost := hs.estimatedPosition.SellAvgPrice * float64(hs.estimatedPosition.SellQty)
			totalCost += price * float64(qty)
			hs.estimatedPosition.SellQty += qty
			hs.estimatedPosition.NetQty -= qty
			if hs.estimatedPosition.SellQty > 0 {
				hs.estimatedPosition.SellAvgPrice = totalCost / float64(hs.estimatedPosition.SellQty)
			}
		}
	}

	hs.estimatedPosition.UpdateCompatibilityFields()
	hs.estimatedPosition.LastUpdate = time.Now()

	// Remove completed orders
	if update.Status == orspb.OrderStatus_FILLED ||
		update.Status == orspb.OrderStatus_CANCELED ||
		update.Status == orspb.OrderStatus_REJECTED {
		delete(hs.Orders, update.OrderId)
	}
}
