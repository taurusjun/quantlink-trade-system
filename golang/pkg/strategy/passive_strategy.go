package strategy

import (
	"fmt"
	"log"
	"sync"
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

	mu sync.RWMutex
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

	// 设置具体策略实例，用于参数热加载
	ps.BaseStrategy.SetConcreteStrategy(ps)

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

	// Initialize private indicators (strategy-specific)
	// 初始化私有指标（策略特定，每个策略可能有不同参数）

	// EWMA for trend - PRIVATE (each strategy may use different period)
	ewmaConfig := map[string]interface{}{
		"period":      20.0,
		"max_history": 100.0,
	}
	_, err := ps.PrivateIndicators.Create("ewma_20", "ewma", ewmaConfig)
	if err != nil {
		return fmt.Errorf("failed to create EWMA indicator: %w", err)
	}

	// Note: Shared indicators (Spread, OrderImbalance, Volatility) MUST be
	// initialized by the StrategyEngine and attached via SetSharedIndicators().
	// In unit tests, they must be manually set up.
	// 注意：共享指标（Spread, OrderImbalance, Volatility）必须由 StrategyEngine 初始化
	// 并通过 SetSharedIndicators() 附加。在单元测试中，必须手动设置。

	ps.Status.StartTime = time.Now()
	return nil
}

// Start starts the strategy
func (ps *PassiveStrategy) Start() error {
	ps.ControlState.RunState = StrategyRunStateActive
	ps.Activate()
	log.Printf("[PassiveStrategy:%s] Started", ps.ID)
	return nil
}

// Stop stops the strategy
func (ps *PassiveStrategy) Stop() error {
	if !ps.IsRunning() {
		return fmt.Errorf("strategy not running")
	}
	ps.ControlState.RunState = StrategyRunStateStopped
	ps.Deactivate()
	return nil
}

// OnMarketData is called when new market data arrives
func (ps *PassiveStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
	if !ps.IsRunning() {
		return
	}

	// Update private indicators (shared indicators are already updated by engine)
	// 更新私有指标（共享指标已由engine更新）
	ps.PrivateIndicators.UpdateAll(md)

	// Update market state
	ps.currentMarketState = FromMarketDataUpdate(md)

	// Update P&L and risk metrics (使用 bid/ask)
	ps.UpdatePNL(ps.currentMarketState.BidPrice, ps.currentMarketState.AskPrice)
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
	// CRITICAL: 检查订单是否属于本策略
	if update.StrategyId != ps.ID {
		return
	}

	if !ps.IsRunning() {
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
		ps.UpdatePNL(ps.currentMarketState.BidPrice, ps.currentMarketState.AskPrice)
		ps.UpdateRiskMetrics(ps.currentMarketState.MidPrice)
	}
}

// OnTimer is called periodically
func (ps *PassiveStrategy) OnTimer(now time.Time) {
	if !ps.IsRunning() {
		return
	}

	// Periodic housekeeping
	// Could implement order timeout, position rebalancing, etc.
}

// GetBaseStrategy returns the underlying BaseStrategy (for engine integration)
func (ps *PassiveStrategy) GetBaseStrategy() *BaseStrategy {
	return ps.BaseStrategy
}

// generateSignals generates trading signals based on current market state
func (ps *PassiveStrategy) generateSignals() {
	if ps.currentMarketState == nil {
		return
	}

	// Get indicator values (tries shared first, then private, then old)
	// 获取指标值（先尝试共享，然后私有，最后旧的）
	spread, ok := ps.GetIndicator("spread")
	if !ok {
		return
	}
	currentSpread := spread.GetValue()

	// Check minimum spread
	if currentSpread < ps.minSpread {
		return // Spread too tight, don't trade
	}

	// Get order imbalance for skewing
	var imbalanceSkew float64
	if ps.useOrderImbalance {
		oi, ok := ps.GetIndicator("order_imbalance")
		if ok {
			imbalance := oi.GetValue()
			// Imbalance ranges from -1 (all asks) to 1 (all bids)
			// Positive imbalance = more bids = more buying pressure = skew quotes higher
			imbalanceSkew = imbalance * 0.5 // Scale down the effect
		}
	}

	// Calculate inventory skew
	// If we're long, we want to sell more aggressively (tighten ask, widen bid)
	// If we're short, we want to buy more aggressively (tighten bid, widen ask)
	inventoryRatio := float64(ps.EstimatedPosition.NetQty) / float64(ps.maxInventory)
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
		if ps.EstimatedPosition.IsLong() {
			// Close long position with sell order
			ps.AddSignal(&TradingSignal{
				StrategyID:  ps.ID,
				Symbol:      ps.currentMarketState.Symbol,
				Exchange:    ps.currentMarketState.Exchange,
				Side:        OrderSideSell,
				Price:       midPrice - askOffset,
				Quantity:    abs(ps.EstimatedPosition.NetQty),
				OrderType:   OrderTypeLimit,
				TimeInForce: TimeInForceGTC,
				Signal:      -0.8,
				Confidence:  0.9,
				Timestamp:   time.Now(),
			})
		} else if ps.EstimatedPosition.IsShort() {
			// Close short position with buy order
			ps.AddSignal(&TradingSignal{
				StrategyID:  ps.ID,
				Symbol:      ps.currentMarketState.Symbol,
				Exchange:    ps.currentMarketState.Exchange,
				Side:        OrderSideBuy,
				Price:       midPrice + bidOffset,
				Quantity:    abs(ps.EstimatedPosition.NetQty),
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
	if ps.EstimatedPosition.NetQty > -ps.maxInventory {
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
	if ps.EstimatedPosition.NetQty < ps.maxInventory {
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

// ApplyParameters 应用新参数（实现 ParameterUpdatable 接口）
func (ps *PassiveStrategy) ApplyParameters(params map[string]interface{}) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	log.Printf("[PassiveStrategy:%s] Applying new parameters...", ps.ID)

	// 保存旧参数（用于日志和回滚）
	oldSpreadMultiplier := ps.spreadMultiplier
	oldOrderSize := ps.orderSize
	oldMaxInventory := ps.maxInventory
	oldInventorySkew := ps.inventorySkew
	oldMinSpread := ps.minSpread
	oldOrderRefreshMs := ps.orderRefreshMs

	// 更新参数
	updated := false

	if val, ok := params["spread_multiplier"].(float64); ok {
		ps.spreadMultiplier = val
		updated = true
	}
	if val, ok := params["order_size"].(int); ok {
		ps.orderSize = int64(val)
		updated = true
	} else if val, ok := params["order_size"].(float64); ok {
		ps.orderSize = int64(val)
		updated = true
	}
	if val, ok := params["max_inventory"].(int); ok {
		ps.maxInventory = int64(val)
		updated = true
	} else if val, ok := params["max_inventory"].(float64); ok {
		ps.maxInventory = int64(val)
		updated = true
	}
	if val, ok := params["inventory_skew"].(float64); ok {
		ps.inventorySkew = val
		updated = true
	}
	if val, ok := params["min_spread"].(float64); ok {
		ps.minSpread = val
		updated = true
	}
	if val, ok := params["order_refresh_ms"].(int); ok {
		ps.orderRefreshMs = int64(val)
		updated = true
	} else if val, ok := params["order_refresh_ms"].(float64); ok {
		ps.orderRefreshMs = int64(val)
		updated = true
	}
	if val, ok := params["use_order_imbalance"].(bool); ok {
		ps.useOrderImbalance = val
		updated = true
	}

	if !updated {
		return fmt.Errorf("no valid parameters found to update")
	}

	// 参数验证
	if ps.spreadMultiplier <= 0 || ps.spreadMultiplier > 2.0 {
		ps.spreadMultiplier = oldSpreadMultiplier
		return fmt.Errorf("invalid spread_multiplier (%.2f), must be in (0, 2.0]", ps.spreadMultiplier)
	}

	if ps.orderSize <= 0 {
		ps.orderSize = oldOrderSize
		return fmt.Errorf("invalid order_size (%d), must be > 0", ps.orderSize)
	}

	if ps.maxInventory <= 0 || ps.orderSize > ps.maxInventory {
		ps.orderSize = oldOrderSize
		ps.maxInventory = oldMaxInventory
		return fmt.Errorf("invalid max_inventory (%d) or order_size (%d), order_size must be <= max_inventory",
			ps.maxInventory, ps.orderSize)
	}

	if ps.inventorySkew < 0 || ps.inventorySkew > 1.0 {
		ps.inventorySkew = oldInventorySkew
		return fmt.Errorf("invalid inventory_skew (%.2f), must be in [0, 1.0]", ps.inventorySkew)
	}

	if ps.minSpread < 0 {
		ps.minSpread = oldMinSpread
		return fmt.Errorf("invalid min_spread (%.2f), must be >= 0", ps.minSpread)
	}

	if ps.orderRefreshMs < 100 {
		ps.orderRefreshMs = oldOrderRefreshMs
		return fmt.Errorf("invalid order_refresh_ms (%d), must be >= 100ms", ps.orderRefreshMs)
	}

	// 输出变更日志
	log.Printf("[PassiveStrategy:%s] ✓ Parameters updated:", ps.ID)
	if oldSpreadMultiplier != ps.spreadMultiplier {
		log.Printf("[PassiveStrategy:%s]   spread_multiplier: %.2f -> %.2f",
			ps.ID, oldSpreadMultiplier, ps.spreadMultiplier)
	}
	if oldOrderSize != ps.orderSize {
		log.Printf("[PassiveStrategy:%s]   order_size: %d -> %d",
			ps.ID, oldOrderSize, ps.orderSize)
	}
	if oldMaxInventory != ps.maxInventory {
		log.Printf("[PassiveStrategy:%s]   max_inventory: %d -> %d",
			ps.ID, oldMaxInventory, ps.maxInventory)
	}
	if oldInventorySkew != ps.inventorySkew {
		log.Printf("[PassiveStrategy:%s]   inventory_skew: %.2f -> %.2f",
			ps.ID, oldInventorySkew, ps.inventorySkew)
	}
	if oldMinSpread != ps.minSpread {
		log.Printf("[PassiveStrategy:%s]   min_spread: %.2f -> %.2f",
			ps.ID, oldMinSpread, ps.minSpread)
	}
	if oldOrderRefreshMs != ps.orderRefreshMs {
		log.Printf("[PassiveStrategy:%s]   order_refresh_ms: %d -> %d",
			ps.ID, oldOrderRefreshMs, ps.orderRefreshMs)
	}

	return nil
}

// GetCurrentParameters 获取当前参数（用于API查询）
func (ps *PassiveStrategy) GetCurrentParameters() map[string]interface{} {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	return map[string]interface{}{
		"spread_multiplier":   ps.spreadMultiplier,
		"order_size":          ps.orderSize,
		"max_inventory":       ps.maxInventory,
		"inventory_skew":      ps.inventorySkew,
		"min_spread":          ps.minSpread,
		"order_refresh_ms":    ps.orderRefreshMs,
		"use_order_imbalance": ps.useOrderImbalance,
	}
}
