// Package strategy provides trading strategy implementations
package strategy

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/indicators"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

// AggressiveStrategy implements an aggressive trend-following strategy
// It takes positions in the direction of the trend and momentum
//
// C++: class AggressiveStrategy : public ExecutionStrategy
// Go:  type AggressiveStrategy struct { *ExecutionStrategy, *StrategyDataContext }
type AggressiveStrategy struct {
	*ExecutionStrategy   // C++: public ExecutionStrategy（C++ 字段）
	*StrategyDataContext // Go 特有字段（指标、配置、状态等）

	// Strategy parameters
	trendPeriod        int     // Period for trend calculation (default: 50)
	momentumPeriod     int     // Period for momentum calculation (default: 20)
	signalThreshold    float64 // Signal strength threshold to trade (default: 0.6)
	orderSize          int64   // Size per order (default: 20)
	maxPositionSize    int64   // Maximum position size (default: 100)
	stopLossPercent    float64 // Stop loss percentage (default: 0.02 = 2%)
	takeProfitPercent  float64 // Take profit percentage (default: 0.05 = 5%)
	minVolatility      float64 // Minimum volatility to trade (default: 0.0001)
	useVolatilityScale bool    // Scale position by volatility (default: true)

	// State
	lastPrice          float64
	entryPrice         float64
	lastSignalTime     time.Time
	minRefreshInterval time.Duration

	// 持仓和PNL（这些字段用于策略内部计算）
	estimatedPosition *EstimatedPosition
	pnl               *PNL
	riskMetrics       *RiskMetrics

	mu sync.RWMutex
}

// NewAggressiveStrategy creates a new aggressive strategy
// C++: AggressiveStrategy::AggressiveStrategy(CommonClient*, SimConfig*)
func NewAggressiveStrategy(id string) *AggressiveStrategy {
	strategyID := int32(hashStringToInt(id))
	baseExecStrategy := NewExecutionStrategy(strategyID, &Instrument{Symbol: "", TickSize: 1.0})

	as := &AggressiveStrategy{
		ExecutionStrategy:    baseExecStrategy,
		StrategyDataContext:  NewStrategyDataContext(id, "aggressive"),
		trendPeriod:          50,
		momentumPeriod:       20,
		signalThreshold:      0.6,
		orderSize:            20,
		maxPositionSize:      100,
		stopLossPercent:      0.02,
		takeProfitPercent:    0.05,
		minVolatility:        0.0001,
		useVolatilityScale:   true,
		minRefreshInterval:   2 * time.Second,
		estimatedPosition:    &EstimatedPosition{},
		pnl:                  &PNL{},
		riskMetrics:          &RiskMetrics{},
	}

	return as
}

// Initialize initializes the strategy
func (as *AggressiveStrategy) Initialize(config *StrategyConfig) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	as.Config = config

	// Load strategy-specific parameters
	if val, ok := config.Parameters["trend_period"].(float64); ok {
		as.trendPeriod = int(val)
	}
	if val, ok := config.Parameters["momentum_period"].(float64); ok {
		as.momentumPeriod = int(val)
	}
	if val, ok := config.Parameters["signal_threshold"].(float64); ok {
		as.signalThreshold = val
	}
	if val, ok := config.Parameters["order_size"].(float64); ok {
		as.orderSize = int64(val)
	}
	if val, ok := config.Parameters["max_position_size"].(float64); ok {
		as.maxPositionSize = int64(val)
	}
	if val, ok := config.Parameters["stop_loss_percent"].(float64); ok {
		as.stopLossPercent = val
	}
	if val, ok := config.Parameters["take_profit_percent"].(float64); ok {
		as.takeProfitPercent = val
	}
	if val, ok := config.Parameters["min_volatility"].(float64); ok {
		as.minVolatility = val
	}
	if val, ok := config.Parameters["use_volatility_scale"].(bool); ok {
		as.useVolatilityScale = val
	}
	if val, ok := config.Parameters["signal_refresh_ms"].(float64); ok {
		as.minRefreshInterval = time.Duration(val) * time.Millisecond
	}

	// Create indicators
	// EWMA for trend
	ewmaTrendConfig := map[string]interface{}{
		"period":      float64(as.trendPeriod),
		"max_history": 200.0,
	}
	_, err := as.PrivateIndicators.Create(fmt.Sprintf("ewma_trend_%d", as.trendPeriod), "ewma", ewmaTrendConfig)
	if err != nil {
		return fmt.Errorf("failed to create trend EWMA: %w", err)
	}

	// EWMA for momentum
	ewmaMomentumConfig := map[string]interface{}{
		"period":      float64(as.momentumPeriod),
		"max_history": 200.0,
	}
	_, err = as.PrivateIndicators.Create(fmt.Sprintf("ewma_momentum_%d", as.momentumPeriod), "ewma", ewmaMomentumConfig)
	if err != nil {
		return fmt.Errorf("failed to create momentum EWMA: %w", err)
	}

	// Volatility indicator
	volConfig := map[string]interface{}{
		"period":      float64(as.momentumPeriod),
		"max_history": 200.0,
	}
	_, err = as.PrivateIndicators.Create("volatility", "volatility", volConfig)
	if err != nil {
		return fmt.Errorf("failed to create volatility indicator: %w", err)
	}

	log.Printf("[AggressiveStrategy:%s] Initialized with trend_period=%d, momentum_period=%d, threshold=%.2f",
		as.ID, as.trendPeriod, as.momentumPeriod, as.signalThreshold)

	return nil
}

// OnMarketData handles market data updates
func (as *AggressiveStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
	as.mu.Lock()
	defer as.mu.Unlock()

	if !as.IsRunning() {
		return
	}

	// Update all indicators
	as.PrivateIndicators.UpdateAll(md)

	// Calculate mid price
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return
	}
	bidPrice := md.BidPrice[0]
	askPrice := md.AskPrice[0]
	midPrice := (bidPrice + askPrice) / 2.0
	as.lastPrice = midPrice

	// Update PNL and risk metrics (使用对手价)
	as.updatePNL(bidPrice, askPrice)
	as.updateRiskMetrics(midPrice)

	// Check if we should generate new signals
	now := time.Now()
	if now.Sub(as.lastSignalTime) < as.minRefreshInterval {
		return
	}

	// Check stop loss and take profit
	if as.estimatedPosition.NetQty != 0 && as.entryPrice > 0 {
		pnlPercent := (midPrice - as.entryPrice) / as.entryPrice
		if as.estimatedPosition.NetQty > 0 {
			// Long position
			if pnlPercent <= -as.stopLossPercent {
				// Stop loss hit
				as.generateExitSignal(md, "stop_loss")
				as.lastSignalTime = now
				return
			}
			if pnlPercent >= as.takeProfitPercent {
				// Take profit hit
				as.generateExitSignal(md, "take_profit")
				as.lastSignalTime = now
				return
			}
		} else {
			// Short position
			if pnlPercent >= as.stopLossPercent {
				// Stop loss hit
				as.generateExitSignal(md, "stop_loss")
				as.lastSignalTime = now
				return
			}
			if pnlPercent <= -as.takeProfitPercent {
				// Take profit hit
				as.generateExitSignal(md, "take_profit")
				as.lastSignalTime = now
				return
			}
		}
	}

	// Generate trading signals based on trend and momentum
	as.generateSignals(md)
	as.lastSignalTime = now
}

// generateSignals generates trading signals based on trend and momentum
func (as *AggressiveStrategy) generateSignals(md *mdpb.MarketDataUpdate) {
	// Get indicators
	trendIndicator, ok := as.GetIndicator(fmt.Sprintf("ewma_trend_%d", as.trendPeriod))
	if !ok || !trendIndicator.IsReady() {
		return
	}

	momentumIndicator, ok := as.GetIndicator(fmt.Sprintf("ewma_momentum_%d", as.momentumPeriod))
	if !ok || !momentumIndicator.IsReady() {
		return
	}

	volIndicator, ok := as.GetIndicator("volatility")
	if !ok || !volIndicator.IsReady() {
		return
	}

	trend := trendIndicator.GetValue()
	momentum := momentumIndicator.GetValue()
	volatility := volIndicator.GetValue()

	// Check minimum volatility
	if volatility < as.minVolatility {
		return
	}

	// Calculate signal strength
	// Positive signal = bullish (price > trend and momentum)
	// Negative signal = bearish (price < trend and momentum)
	trendSignal := (as.lastPrice - trend) / trend
	momentumSignal := (as.lastPrice - momentum) / momentum

	// Combine signals with weights (trend 60%, momentum 40%)
	signal := 0.6*trendSignal + 0.4*momentumSignal

	// Normalize signal to [-1, 1]
	signal = math.Max(-1.0, math.Min(1.0, signal*100))

	// Calculate confidence based on alignment
	alignment := 1.0 - math.Abs(trendSignal-momentumSignal)/(math.Abs(trendSignal)+math.Abs(momentumSignal)+1e-10)
	confidence := alignment * (1.0 - volatility)

	// Check if signal is strong enough
	if math.Abs(signal) < as.signalThreshold {
		return
	}

	// Calculate position size
	positionSize := as.orderSize
	if as.useVolatilityScale {
		// Scale down position in high volatility
		volatilityScale := math.Max(0.5, 1.0-volatility*10)
		positionSize = int64(float64(as.orderSize) * volatilityScale)
	}

	// Check position limits
	if signal > 0 {
		// Bullish signal - buy
		if as.estimatedPosition.NetQty >= as.maxPositionSize {
			return
		}
		if as.estimatedPosition.NetQty+positionSize > as.maxPositionSize {
			positionSize = as.maxPositionSize - as.estimatedPosition.NetQty
		}

		// Generate buy signal
		buySignal := &TradingSignal{
			StrategyID: as.ID,
			Symbol:     md.Symbol,
			Side:       OrderSideBuy,
			Price:      md.AskPrice[0], // Take the ask (aggressive)
			Quantity:   positionSize,
			Signal:     signal,
			Confidence: confidence,
			Timestamp:  time.Now(),
			Metadata: map[string]interface{}{
				"trend":     trend,
				"momentum":  momentum,
				"volatility": volatility,
				"type":      "entry",
			},
		}
		as.addSignal(buySignal)
		if as.estimatedPosition.NetQty == 0 {
			as.entryPrice = buySignal.Price
		}

	} else {
		// Bearish signal - sell
		if as.estimatedPosition.NetQty <= -as.maxPositionSize {
			return
		}
		if as.estimatedPosition.NetQty-positionSize < -as.maxPositionSize {
			positionSize = as.estimatedPosition.NetQty + as.maxPositionSize
		}

		// Generate sell signal
		sellSignal := &TradingSignal{
			StrategyID: as.ID,
			Symbol:     md.Symbol,
			Side:       OrderSideSell,
			Price:      md.BidPrice[0], // Hit the bid (aggressive)
			Quantity:   positionSize,
			Signal:     signal,
			Confidence: confidence,
			Timestamp:  time.Now(),
			Metadata: map[string]interface{}{
				"trend":     trend,
				"momentum":  momentum,
				"volatility": volatility,
				"type":      "entry",
			},
		}
		as.addSignal(sellSignal)
		if as.estimatedPosition.NetQty == 0 {
			as.entryPrice = sellSignal.Price
		}
	}
}

// generateExitSignal generates an exit signal to close position
func (as *AggressiveStrategy) generateExitSignal(md *mdpb.MarketDataUpdate, reason string) {
	if as.estimatedPosition.NetQty == 0 {
		return
	}

	var signal *TradingSignal
	if as.estimatedPosition.NetQty > 0 {
		// Close long position - sell
		signal = &TradingSignal{
			StrategyID: as.ID,
			Symbol:     md.Symbol,
			Side:       OrderSideSell,
			Price:      md.BidPrice[0],
			Quantity:   as.estimatedPosition.NetQty,
			Signal:     -1.0,
			Confidence: 1.0,
			Timestamp:  time.Now(),
			Metadata: map[string]interface{}{
				"type":   "exit",
				"reason": reason,
			},
		}
	} else {
		// Close short position - buy
		signal = &TradingSignal{
			StrategyID: as.ID,
			Symbol:     md.Symbol,
			Side:       OrderSideBuy,
			Price:      md.AskPrice[0],
			Quantity:   -as.estimatedPosition.NetQty,
			Signal:     1.0,
			Confidence: 1.0,
			Timestamp:  time.Now(),
			Metadata: map[string]interface{}{
				"type":   "exit",
				"reason": reason,
			},
		}
	}

	as.addSignal(signal)
	as.entryPrice = 0
	log.Printf("[AggressiveStrategy:%s] Generated exit signal: reason=%s, qty=%d, price=%.2f",
		as.ID, reason, signal.Quantity, signal.Price)
}

// OnOrderUpdate handles order updates
func (as *AggressiveStrategy) OnOrderUpdate(update *orspb.OrderUpdate) {
	// CRITICAL: 检查订单是否属于本策略
	if update.StrategyId != as.ID {
		return
	}

	as.mu.Lock()
	defer as.mu.Unlock()

	if !as.IsRunning() {
		return
	}

	// Update position based on order status
	as.UpdatePosition(update)
}

// OnTimer handles timer events
func (as *AggressiveStrategy) OnTimer(now time.Time) {
	as.mu.RLock()
	defer as.mu.RUnlock()

	// Periodic housekeeping
	if !as.IsRunning() {
		return
	}
}

// Start starts the strategy
func (as *AggressiveStrategy) Start() error {
	as.mu.Lock()
	defer as.mu.Unlock()

	as.ControlState.RunState = StrategyRunStateActive
	as.ControlState.Active = true
	log.Printf("[AggressiveStrategy:%s] Started", as.ID)
	return nil
}

// Stop stops the strategy
func (as *AggressiveStrategy) Stop() error {
	as.mu.Lock()
	defer as.mu.Unlock()

	as.ControlState.RunState = StrategyRunStateStopped
	as.ControlState.Active = false
	log.Printf("[AggressiveStrategy:%s] Stopped", as.ID)
	return nil
}

// GetBaseStrategy returns the underlying BaseStrategy (for engine integration)
// OnAuctionData handles auction period market data
// AggressiveStrategy ignores auction data by default
func (as *AggressiveStrategy) OnAuctionData(md *mdpb.MarketDataUpdate) {
	// Aggressive strategy does not trade during auction periods
	log.Printf("[AggressiveStrategy:%s] Ignoring auction data for %s", as.ID, md.Symbol)
}

// === Strategy 接口方法 ===

// GetID returns the strategy ID
func (as *AggressiveStrategy) GetID() string {
	return as.ID
}

// GetType returns the strategy type
func (as *AggressiveStrategy) GetType() string {
	return as.Type
}

// IsRunning returns whether the strategy is running (lock-free for internal use)
// Delegates to StrategyDataContext.IsRunning() which checks ControlState.RunState
func (as *AggressiveStrategy) IsRunning() bool {
	return as.StrategyDataContext.IsRunning()
}

// GetConfig returns the strategy configuration
func (as *AggressiveStrategy) GetConfig() *StrategyConfig {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.Config
}

// GetSignals returns pending trading signals
func (as *AggressiveStrategy) GetSignals() []*TradingSignal {
	as.mu.Lock()
	defer as.mu.Unlock()
	signals := as.PendingSignals
	as.PendingSignals = make([]*TradingSignal, 0)
	return signals
}

// AddSignal adds a trading signal (thread-safe, acquires lock)
func (as *AggressiveStrategy) AddSignal(signal *TradingSignal) {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.PendingSignals = append(as.PendingSignals, signal)
}

// addSignal adds a trading signal (internal use, caller must hold lock)
func (as *AggressiveStrategy) addSignal(signal *TradingSignal) {
	as.PendingSignals = append(as.PendingSignals, signal)
}

// GetEstimatedPosition returns the estimated position
func (as *AggressiveStrategy) GetEstimatedPosition() *EstimatedPosition {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.estimatedPosition
}

// GetPNL returns the PNL
func (as *AggressiveStrategy) GetPNL() *PNL {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.pnl
}

// GetRiskMetrics returns the risk metrics
func (as *AggressiveStrategy) GetRiskMetrics() *RiskMetrics {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.riskMetrics
}

// GetStatus returns the strategy status
func (as *AggressiveStrategy) GetStatus() *StrategyStatus {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.Status
}

// GetControlState returns the control state
func (as *AggressiveStrategy) GetControlState() *StrategyControlState {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.ControlState
}

// GetCurrentParameters returns the current strategy parameters
func (as *AggressiveStrategy) GetCurrentParameters() map[string]interface{} {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return map[string]interface{}{
		"trend_period":        as.trendPeriod,
		"momentum_period":     as.momentumPeriod,
		"signal_threshold":    as.signalThreshold,
		"order_size":          as.orderSize,
		"max_position_size":   as.maxPositionSize,
		"stop_loss_percent":   as.stopLossPercent,
		"take_profit_percent": as.takeProfitPercent,
		"min_volatility":      as.minVolatility,
		"use_volatility_scale": as.useVolatilityScale,
	}
}

// GetIndicator returns an indicator from the private indicator library
func (as *AggressiveStrategy) GetIndicator(name string) (indicators.Indicator, bool) {
	return as.PrivateIndicators.Get(name)
}

// GetPosition returns current position (alias for GetEstimatedPosition for Strategy interface)
func (as *AggressiveStrategy) GetPosition() *EstimatedPosition {
	return as.estimatedPosition
}

// UpdatePosition updates position based on order update
func (as *AggressiveStrategy) UpdatePosition(update *orspb.OrderUpdate) {
	// Store order
	as.Orders[update.OrderId] = update

	// Update position on fill
	if update.Status == orspb.OrderStatus_FILLED || update.Status == orspb.OrderStatus_PARTIALLY_FILLED {
		filledQty := update.LastFillQty
		if update.Side == orspb.OrderSide_SELL {
			filledQty = -filledQty
		}
		as.estimatedPosition.NetQty += filledQty
	}
}

// updatePNL updates PNL based on current prices (internal use, caller must hold lock)
func (as *AggressiveStrategy) updatePNL(bidPrice, askPrice float64) {
	if as.estimatedPosition.NetQty == 0 {
		as.pnl.UnrealizedPnL = 0
		return
	}

	// Calculate unrealised PNL using exit price (bid for long, ask for short)
	var exitPrice float64
	if as.estimatedPosition.NetQty > 0 {
		exitPrice = bidPrice
	} else {
		exitPrice = askPrice
	}

	if as.entryPrice > 0 {
		as.pnl.UnrealizedPnL = (exitPrice - as.entryPrice) * float64(as.estimatedPosition.NetQty)
	}
	as.pnl.TotalPnL = as.pnl.RealizedPnL + as.pnl.UnrealizedPnL
}

// updateRiskMetrics updates risk metrics (internal use, caller must hold lock)
func (as *AggressiveStrategy) updateRiskMetrics(midPrice float64) {
	// Update max PNL and drawdown
	if as.pnl.TotalPnL > as.pnl.MaxDrawdown {
		// Track max PnL in MaxDrawdown field temporarily (will be recalculated)
	}
	if as.pnl.TotalPnL < as.riskMetrics.MaxDrawdown {
		as.riskMetrics.MaxDrawdown = as.pnl.TotalPnL
	}
}

// === Strategy 接口：C++ 虚函数对应 ===

// SendOrder generates and sends orders based on current state
// C++: virtual void SendOrder() = 0
func (as *AggressiveStrategy) SendOrder() {
	// AggressiveStrategy 通过 OnMarketData 中的 generateSignals 发送订单
}

// OnTradeUpdate is called after a trade is processed
// C++: virtual void OnTradeUpdate() {}
func (as *AggressiveStrategy) OnTradeUpdate() {
	// 成交后更新状态
}

// CheckSquareoff checks if position needs to be squared off
// C++: virtual void CheckSquareoff(MarketUpdateNew*)
func (as *AggressiveStrategy) CheckSquareoff() {
	as.mu.Lock()
	defer as.mu.Unlock()

	// 检查止损
	if as.pnl != nil && as.stopLossPercent > 0 && as.entryPrice > 0 {
		if as.estimatedPosition.NetQty != 0 {
			pnlPercent := (as.lastPrice - as.entryPrice) / as.entryPrice
			if as.estimatedPosition.NetQty > 0 && pnlPercent <= -as.stopLossPercent {
				as.ControlState.FlattenMode = true
			} else if as.estimatedPosition.NetQty < 0 && pnlPercent >= as.stopLossPercent {
				as.ControlState.FlattenMode = true
			}
		}
	}
}

// HandleSquareON handles square off initiation
// C++: virtual void HandleSquareON()
func (as *AggressiveStrategy) HandleSquareON() {
	as.mu.Lock()
	defer as.mu.Unlock()
	as.ControlState.FlattenMode = true
}

// HandleSquareoff executes the square off logic
// C++: virtual void HandleSquareoff()
func (as *AggressiveStrategy) HandleSquareoff() {
	as.mu.Lock()
	defer as.mu.Unlock()
	// 平仓逻辑：清空持仓
	as.estimatedPosition.NetQty = 0
	as.entryPrice = 0
	as.ControlState.FlattenMode = false
}

// SetThresholds sets dynamic thresholds based on position
// C++: virtual void SetThresholds()
func (as *AggressiveStrategy) SetThresholds() {
	// AggressiveStrategy 使用固定阈值，此处为空实现
}

// Reset resets the strategy to initial state
// C++: virtual void Reset()
func (as *AggressiveStrategy) Reset() {
	as.mu.Lock()
	defer as.mu.Unlock()

	as.lastPrice = 0
	as.entryPrice = 0
	as.lastSignalTime = time.Time{}
	as.estimatedPosition = &EstimatedPosition{}
	as.pnl = &PNL{}
	as.riskMetrics = &RiskMetrics{}
	as.PendingSignals = make([]*TradingSignal, 0)
	as.Orders = make(map[string]*orspb.OrderUpdate)
}

// UpdateParameters updates strategy parameters (for hot reload)
func (as *AggressiveStrategy) UpdateParameters(params map[string]interface{}) error {
	as.mu.Lock()
	defer as.mu.Unlock()

	if val, ok := params["trend_period"].(float64); ok {
		as.trendPeriod = int(val)
	}
	if val, ok := params["momentum_period"].(float64); ok {
		as.momentumPeriod = int(val)
	}
	if val, ok := params["signal_threshold"].(float64); ok {
		as.signalThreshold = val
	}
	if val, ok := params["order_size"].(float64); ok {
		as.orderSize = int64(val)
	}
	if val, ok := params["max_position_size"].(float64); ok {
		as.maxPositionSize = int64(val)
	}
	if val, ok := params["stop_loss_percent"].(float64); ok {
		as.stopLossPercent = val
	}
	if val, ok := params["take_profit_percent"].(float64); ok {
		as.takeProfitPercent = val
	}
	return nil
}

// === Engine/Manager 需要的方法 ===

// CanSendOrder returns true if strategy can send orders
func (as *AggressiveStrategy) CanSendOrder() bool {
	as.mu.RLock()
	defer as.mu.RUnlock()
	return as.IsRunning() && as.ControlState != nil && as.ControlState.IsActive() && !as.ControlState.FlattenMode
}

// SetLastMarketData stores the last market data for a symbol
func (as *AggressiveStrategy) SetLastMarketData(symbol string, md *mdpb.MarketDataUpdate) {
	as.mu.Lock()
	defer as.mu.Unlock()
	// AggressiveStrategy 使用 lastPrice 字段，不需要存储完整 MD
	if len(md.BidPrice) > 0 && len(md.AskPrice) > 0 {
		as.lastPrice = (md.BidPrice[0] + md.AskPrice[0]) / 2.0
	}
}

// GetLastMarketData returns the last market data for a symbol
func (as *AggressiveStrategy) GetLastMarketData(symbol string) *mdpb.MarketDataUpdate {
	// AggressiveStrategy 不存储完整的 MarketData
	return nil
}

// TriggerFlatten triggers position flattening
func (as *AggressiveStrategy) TriggerFlatten(reason FlattenReason, aggressive bool) {
	as.mu.Lock()
	defer as.mu.Unlock()
	if as.ControlState != nil {
		as.ControlState.FlattenMode = true
	}
}

// GetPendingCancels returns orders pending cancellation
func (as *AggressiveStrategy) GetPendingCancels() []*orspb.OrderUpdate {
	// AggressiveStrategy 目前不追踪待撤订单
	return nil
}
