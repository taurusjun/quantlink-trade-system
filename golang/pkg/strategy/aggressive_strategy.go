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

// AggressiveStrategy implements an aggressive trend-following strategy
// It takes positions in the direction of the trend and momentum
type AggressiveStrategy struct {
	*BaseStrategy

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
	lastPrice         float64
	entryPrice        float64
	lastSignalTime    time.Time
	minRefreshInterval time.Duration

	mu sync.RWMutex
}

// NewAggressiveStrategy creates a new aggressive strategy
func NewAggressiveStrategy(id string) *AggressiveStrategy {
	as := &AggressiveStrategy{
		BaseStrategy:       NewBaseStrategy(id, "aggressive"),
		trendPeriod:        50,
		momentumPeriod:     20,
		signalThreshold:    0.6,
		orderSize:          20,
		maxPositionSize:    100,
		stopLossPercent:    0.02,
		takeProfitPercent:  0.05,
		minVolatility:      0.0001,
		useVolatilityScale: true,
		minRefreshInterval: 2 * time.Second,
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
	as.BaseStrategy.UpdatePNL(bidPrice, askPrice)
	as.BaseStrategy.UpdateRiskMetrics(midPrice)

	// Check if we should generate new signals
	now := time.Now()
	if now.Sub(as.lastSignalTime) < as.minRefreshInterval {
		return
	}

	// Check stop loss and take profit
	if as.EstimatedPosition.NetQty != 0 && as.entryPrice > 0 {
		pnlPercent := (midPrice - as.entryPrice) / as.entryPrice
		if as.EstimatedPosition.NetQty > 0 {
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
		if as.EstimatedPosition.NetQty >= as.maxPositionSize {
			return
		}
		if as.EstimatedPosition.NetQty+positionSize > as.maxPositionSize {
			positionSize = as.maxPositionSize - as.EstimatedPosition.NetQty
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
		as.BaseStrategy.AddSignal(buySignal)
		if as.EstimatedPosition.NetQty == 0 {
			as.entryPrice = buySignal.Price
		}

	} else {
		// Bearish signal - sell
		if as.EstimatedPosition.NetQty <= -as.maxPositionSize {
			return
		}
		if as.EstimatedPosition.NetQty-positionSize < -as.maxPositionSize {
			positionSize = as.EstimatedPosition.NetQty + as.maxPositionSize
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
		as.BaseStrategy.AddSignal(sellSignal)
		if as.EstimatedPosition.NetQty == 0 {
			as.entryPrice = sellSignal.Price
		}
	}
}

// generateExitSignal generates an exit signal to close position
func (as *AggressiveStrategy) generateExitSignal(md *mdpb.MarketDataUpdate, reason string) {
	if as.EstimatedPosition.NetQty == 0 {
		return
	}

	var signal *TradingSignal
	if as.EstimatedPosition.NetQty > 0 {
		// Close long position - sell
		signal = &TradingSignal{
			StrategyID: as.ID,
			Symbol:     md.Symbol,
			Side:       OrderSideSell,
			Price:      md.BidPrice[0],
			Quantity:   as.EstimatedPosition.NetQty,
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
			Quantity:   -as.EstimatedPosition.NetQty,
			Signal:     1.0,
			Confidence: 1.0,
			Timestamp:  time.Now(),
			Metadata: map[string]interface{}{
				"type":   "exit",
				"reason": reason,
			},
		}
	}

	as.BaseStrategy.AddSignal(signal)
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
	as.Activate()
	log.Printf("[AggressiveStrategy:%s] Started", as.ID)
	return nil
}

// Stop stops the strategy
func (as *AggressiveStrategy) Stop() error {
	as.mu.Lock()
	defer as.mu.Unlock()

	as.ControlState.RunState = StrategyRunStateStopped
	as.Deactivate()
	log.Printf("[AggressiveStrategy:%s] Stopped", as.ID)
	return nil
}

// GetBaseStrategy returns the underlying BaseStrategy (for engine integration)
func (as *AggressiveStrategy) GetBaseStrategy() *BaseStrategy {
	return as.BaseStrategy
}
