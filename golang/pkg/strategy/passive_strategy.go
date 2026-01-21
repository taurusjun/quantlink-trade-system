package strategy

import (
	"fmt"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

// PassiveStrategy implements a passive market making strategy
// Places orders on both sides of the book to capture bid-ask spread
type PassiveStrategy struct {
	*BaseStrategy

	// Strategy parameters
	spreadMultiplier  float64 // Multiplier for spread placement
	orderSize         int64   // Size of each order
	maxInventory      int64   // Maximum inventory (position limit)
	inventorySkew     float64 // Inventory skew factor
	minSpread         float64 // Minimum spread to trade
	orderRefreshMs    int64   // Order refresh interval in ms
	useOrderImbalance bool    // Use order imbalance for skewing

	// Runtime state
	lastOrderTime     time.Time
	currentMarketState *MarketState
	bidOrderID        string
	askOrderID        string
}

// NewPassiveStrategy creates a new passive strategy
func NewPassiveStrategy(id string) *PassiveStrategy {
	ps := &PassiveStrategy{
		BaseStrategy: NewBaseStrategy(id, "passive"),
		// Default parameters
		spreadMultiplier:  0.5,
		orderSize:         10,
		maxInventory:      100,
		inventorySkew:     0.5,
		minSpread:         1.0,
		orderRefreshMs:    1000,
		useOrderImbalance: true,
	}
	return ps
}

// Initialize initializes the strategy
func (ps *PassiveStrategy) Initialize(config *StrategyConfig) error {
	ps.Config = config

	// Load parameters from config
	if v, ok := config.Parameters["spread_multiplier"].(float64); ok {
		ps.spreadMultiplier = v
	}
	if v, ok := config.Parameters["order_size"].(float64); ok {
		ps.orderSize = int64(v)
	}
	if v, ok := config.Parameters["max_inventory"].(float64); ok {
		ps.maxInventory = int64(v)
	}
	if v, ok := config.Parameters["inventory_skew"].(float64); ok {
		ps.inventorySkew = v
	}
	if v, ok := config.Parameters["min_spread"].(float64); ok {
		ps.minSpread = v
	}
	if v, ok := config.Parameters["order_refresh_ms"].(float64); ok {
		ps.orderRefreshMs = int64(v)
	}
	if v, ok := config.Parameters["use_order_imbalance"].(bool); ok {
		ps.useOrderImbalance = v
	}

	// Initialize indicators
	// EWMA for trend
	ewmaConfig := map[string]interface{}{
		"period":      20.0,
		"max_history": 100.0,
	}
	_, err := ps.Indicators.Create("ewma_20", "ewma", ewmaConfig)
	if err != nil {
		return fmt.Errorf("failed to create EWMA indicator: %w", err)
	}

	// Order Imbalance for skewing
	oiConfig := map[string]interface{}{
		"levels":        5.0,
		"volume_weight": true,
		"max_history":   100.0,
	}
	_, err = ps.Indicators.Create("order_imbalance", "order_imbalance", oiConfig)
	if err != nil {
		return fmt.Errorf("failed to create OrderImbalance indicator: %w", err)
	}

	// Spread indicator
	spreadConfig := map[string]interface{}{
		"absolute":    true,
		"max_history": 100.0,
	}
	_, err = ps.Indicators.Create("spread", "spread", spreadConfig)
	if err != nil {
		return fmt.Errorf("failed to create Spread indicator: %w", err)
	}

	// Volatility for risk management
	volConfig := map[string]interface{}{
		"window":          20.0,
		"use_log_returns": true,
		"max_history":     100.0,
	}
	_, err = ps.Indicators.Create("volatility", "volatility", volConfig)
	if err != nil {
		return fmt.Errorf("failed to create Volatility indicator: %w", err)
	}

	ps.Status.StartTime = time.Now()
	return nil
}

// Start starts the strategy
func (ps *PassiveStrategy) Start() error {
	if ps.IsRunningFlag {
		return fmt.Errorf("strategy already running")
	}
	ps.IsRunningFlag = true
	ps.Status.IsRunning = true
	return nil
}

// Stop stops the strategy
func (ps *PassiveStrategy) Stop() error {
	if !ps.IsRunningFlag {
		return fmt.Errorf("strategy not running")
	}
	ps.IsRunningFlag = false
	ps.Status.IsRunning = false
	return nil
}

// OnMarketData is called when new market data arrives
func (ps *PassiveStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
	if !ps.IsRunningFlag {
		return
	}

	// Update indicators
	ps.Indicators.UpdateAll(md)

	// Update market state
	ps.currentMarketState = FromMarketDataUpdate(md)

	// Update P&L and risk metrics
	ps.UpdatePNL(ps.currentMarketState.MidPrice)
	ps.UpdateRiskMetrics(ps.currentMarketState.MidPrice)

	// Check if we need to refresh orders
	now := time.Now()
	if now.Sub(ps.lastOrderTime).Milliseconds() >= ps.orderRefreshMs {
		ps.generateSignals()
		ps.lastOrderTime = now
	}
}

// OnOrderUpdate is called when order status changes
func (ps *PassiveStrategy) OnOrderUpdate(update *orspb.OrderUpdate) {
	if !ps.IsRunningFlag {
		return
	}

	// Update position
	ps.UpdatePosition(update)

	// Track our orders
	if update.OrderId == ps.bidOrderID {
		if update.Status == orspb.OrderStatus_FILLED {
			ps.bidOrderID = "" // Clear filled order
		}
	}
	if update.OrderId == ps.askOrderID {
		if update.Status == orspb.OrderStatus_FILLED {
			ps.askOrderID = "" // Clear filled order
		}
	}

	// Update P&L if we have market state
	if ps.currentMarketState != nil {
		ps.UpdatePNL(ps.currentMarketState.MidPrice)
		ps.UpdateRiskMetrics(ps.currentMarketState.MidPrice)
	}
}

// OnTimer is called periodically
func (ps *PassiveStrategy) OnTimer(now time.Time) {
	if !ps.IsRunningFlag {
		return
	}

	// Periodic housekeeping
	// Could implement order timeout, position rebalancing, etc.
}

// generateSignals generates trading signals based on current market state
func (ps *PassiveStrategy) generateSignals() {
	if ps.currentMarketState == nil {
		return
	}

	// Get indicator values
	spread, _ := ps.Indicators.Get("spread")
	currentSpread := spread.GetValue()

	// Check minimum spread
	if currentSpread < ps.minSpread {
		return // Spread too tight, don't trade
	}

	// Get order imbalance for skewing
	var imbalanceSkew float64
	if ps.useOrderImbalance {
		oi, _ := ps.Indicators.Get("order_imbalance")
		imbalance := oi.GetValue()
		// Imbalance ranges from -1 (all asks) to 1 (all bids)
		// Positive imbalance = more bids = more buying pressure = skew quotes higher
		imbalanceSkew = imbalance * 0.5 // Scale down the effect
	}

	// Calculate inventory skew
	// If we're long, we want to sell more aggressively (tighten ask, widen bid)
	// If we're short, we want to buy more aggressively (tighten bid, widen ask)
	inventoryRatio := float64(ps.Position.NetQty) / float64(ps.maxInventory)
	inventorySkewAmount := inventoryRatio * ps.inventorySkew

	// Calculate bid/ask offsets
	bidOffset := currentSpread * ps.spreadMultiplier
	askOffset := currentSpread * ps.spreadMultiplier

	// Apply skews
	totalSkew := imbalanceSkew + inventorySkewAmount

	// If skew is positive (buying pressure or long position), widen bid, tighten ask
	bidOffset += totalSkew * currentSpread * 0.3
	askOffset -= totalSkew * currentSpread * 0.3

	// Ensure minimum offset
	minOffset := ps.minSpread * 0.3
	if bidOffset < minOffset {
		bidOffset = minOffset
	}
	if askOffset < minOffset {
		askOffset = minOffset
	}

	midPrice := ps.currentMarketState.MidPrice

	// Check risk limits before generating signals
	if !ps.CheckRiskLimits() {
		// Risk limits exceeded, only generate closing signals
		if ps.Position.IsLong() {
			// Close long position with sell order
			ps.AddSignal(&TradingSignal{
				StrategyID:  ps.ID,
				Symbol:      ps.currentMarketState.Symbol,
				Exchange:    ps.currentMarketState.Exchange,
				Side:        OrderSideSell,
				Price:       midPrice - askOffset,
				Quantity:    abs(ps.Position.NetQty),
				OrderType:   OrderTypeLimit,
				TimeInForce: TimeInForceGTC,
				Signal:      -0.8,
				Confidence:  0.9,
				Timestamp:   time.Now(),
			})
		} else if ps.Position.IsShort() {
			// Close short position with buy order
			ps.AddSignal(&TradingSignal{
				StrategyID:  ps.ID,
				Symbol:      ps.currentMarketState.Symbol,
				Exchange:    ps.currentMarketState.Exchange,
				Side:        OrderSideBuy,
				Price:       midPrice + bidOffset,
				Quantity:    abs(ps.Position.NetQty),
				OrderType:   OrderTypeLimit,
				TimeInForce: TimeInForceGTC,
				Signal:      0.8,
				Confidence:  0.9,
				Timestamp:   time.Now(),
			})
		}
		return
	}

	// Generate bid signal (only if not at max short)
	if ps.Position.NetQty > -ps.maxInventory {
		bidPrice := midPrice - bidOffset
		ps.AddSignal(&TradingSignal{
			StrategyID:  ps.ID,
			Symbol:      ps.currentMarketState.Symbol,
			Exchange:    ps.currentMarketState.Exchange,
			Side:        OrderSideBuy,
			Price:       bidPrice,
			Quantity:    ps.orderSize,
			OrderType:   OrderTypeLimit,
			TimeInForce: TimeInForceGTC,
			Signal:      0.5,
			Confidence:  0.7,
			Timestamp:   time.Now(),
			Metadata: map[string]interface{}{
				"bid_offset":      bidOffset,
				"imbalance_skew":  imbalanceSkew,
				"inventory_skew":  inventorySkewAmount,
			},
		})
	}

	// Generate ask signal (only if not at max long)
	if ps.Position.NetQty < ps.maxInventory {
		askPrice := midPrice + askOffset
		ps.AddSignal(&TradingSignal{
			StrategyID:  ps.ID,
			Symbol:      ps.currentMarketState.Symbol,
			Exchange:    ps.currentMarketState.Exchange,
			Side:        OrderSideSell,
			Price:       askPrice,
			Quantity:    ps.orderSize,
			OrderType:   OrderTypeLimit,
			TimeInForce: TimeInForceGTC,
			Signal:      -0.5,
			Confidence:  0.7,
			Timestamp:   time.Now(),
			Metadata: map[string]interface{}{
				"ask_offset":      askOffset,
				"imbalance_skew":  imbalanceSkew,
				"inventory_skew":  inventorySkewAmount,
			},
		})
	}
}

// GetStrategyInfo returns strategy description
func (ps *PassiveStrategy) GetStrategyInfo() string {
	return fmt.Sprintf(`PassiveStrategy: %s
  - Spread Multiplier: %.2f
  - Order Size: %d
  - Max Inventory: %d
  - Inventory Skew: %.2f
  - Min Spread: %.2f
  - Order Refresh: %dms
  - Use Order Imbalance: %v`,
		ps.ID,
		ps.spreadMultiplier,
		ps.orderSize,
		ps.maxInventory,
		ps.inventorySkew,
		ps.minSpread,
		ps.orderRefreshMs,
		ps.useOrderImbalance,
	)
}
