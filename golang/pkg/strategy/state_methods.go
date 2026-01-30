package strategy

import (
	"log"
	"time"
)

// State Control Methods for BaseStrategy
// Aligned with tbsrc's ExecutionStrategy state management

// =============================================================================
// Activation Control (对应 tbsrc: m_Active)
// =============================================================================

// Activate activates the strategy
// 对应 tbsrc: m_Active = true (manual activation in live mode)
func (bs *BaseStrategy) Activate() {
	bs.ControlState.Activate()
	log.Printf("[%s] Strategy activated", bs.ID)
}

// Deactivate deactivates the strategy
// 对应 tbsrc: m_Active = false
func (bs *BaseStrategy) Deactivate() {
	bs.ControlState.Deactivate()
	log.Printf("[%s] Strategy deactivated", bs.ID)
}

// IsActivated returns true if strategy is activated
func (bs *BaseStrategy) IsActivated() bool {
	return bs.ControlState.IsActivated()
}

// =============================================================================
// Flatten Control (对应 tbsrc: m_onFlat, m_onCancel, m_aggFlat)
// =============================================================================

// TriggerFlatten triggers flatten mode (stop sending new orders and close positions)
// 对应 tbsrc: m_onFlat = true, m_onCancel = true
func (bs *BaseStrategy) TriggerFlatten(reason FlattenReason, aggressive bool) {
	if bs.ControlState.ExitRequested {
		return // Already exiting, don't change state
	}

	bs.ControlState.FlattenMode = true
	bs.ControlState.CancelPending = true
	bs.ControlState.AggressiveFlat = aggressive
	bs.ControlState.FlattenReason = reason
	bs.ControlState.FlattenTime = time.Now()
	bs.ControlState.RunState = StrategyRunStateFlattening

	// Set recovery time based on reason
	if reason.CanRecover() {
		bs.ControlState.CanRecoverAt = time.Now().Add(reason.RecoveryCooldown())
		log.Printf("[%s] Flatten triggered: %s (aggressive=%v, can recover at %s)",
			bs.ID, reason, aggressive, bs.ControlState.CanRecoverAt.Format(time.RFC3339))
	} else {
		bs.ControlState.CanRecoverAt = time.Time{} // Cannot recover
		log.Printf("[%s] Flatten triggered: %s (aggressive=%v, no recovery)",
			bs.ID, reason, aggressive)
	}
}

// TryRecover attempts to recover from flatten mode
// Returns true if recovery was successful
// 对应 tbsrc: 自动恢复逻辑 (e.g., price back to normal range)
func (bs *BaseStrategy) TryRecover() bool {
	if !bs.ControlState.CanAttemptRecovery() {
		return false
	}

	// Check if position is flat (required for recovery)
	if !bs.EstimatedPosition.IsFlat() {
		log.Printf("[%s] Cannot recover: position not flat (net=%d)", bs.ID, bs.EstimatedPosition.NetQty)
		return false
	}

	// Recover
	bs.ControlState.FlattenMode = false
	bs.ControlState.CancelPending = false
	bs.ControlState.AggressiveFlat = false
	bs.ControlState.RunState = StrategyRunStateActive
	bs.ControlState.FlattenReason = FlattenReasonNone

	log.Printf("[%s] Strategy recovered from flatten mode", bs.ID)
	return true
}

// =============================================================================
// Exit Control (对应 tbsrc: m_onExit)
// =============================================================================

// TriggerExit triggers strategy exit (cannot recover)
// 对应 tbsrc: m_onExit = true, m_onCancel = true, m_onFlat = true
func (bs *BaseStrategy) TriggerExit(reason string) {
	bs.ControlState.ExitRequested = true
	bs.ControlState.FlattenMode = true
	bs.ControlState.CancelPending = true
	bs.ControlState.RunState = StrategyRunStateExiting
	bs.ControlState.ExitReason = reason

	log.Printf("[%s] Strategy exit requested: %s", bs.ID, reason)
}

// CompleteExit completes the exit process and stops the strategy
// Called when all positions are closed and all orders are canceled
// 对应 tbsrc: m_Active = false (after positions are flat)
func (bs *BaseStrategy) CompleteExit() {
	if !bs.ControlState.ExitRequested {
		return
	}

	// Check if we can exit
	if !bs.EstimatedPosition.IsFlat() {
		log.Printf("[%s] Cannot complete exit: position not flat (net=%d)", bs.ID, bs.EstimatedPosition.NetQty)
		return
	}

	if len(bs.PendingSignals) > 0 {
		log.Printf("[%s] Cannot complete exit: %d pending signals", bs.ID, len(bs.PendingSignals))
		return
	}

	// Complete exit
	bs.ControlState.RunState = StrategyRunStateStopped
	bs.ControlState.Active = false

	log.Printf("[%s] Strategy fully stopped", bs.ID)
}

// =============================================================================
// Flatten Execution (对应 tbsrc: HandleSquareoff)
// =============================================================================

// HandleFlatten handles the flatten process
// Generates orders to close positions based on current state
// 对应 tbsrc: HandleSquareoff()
func (bs *BaseStrategy) HandleFlatten(currentPrice float64) {
	if !bs.ControlState.FlattenMode {
		return
	}

	// Step 1: Cancel pending orders if needed
	if bs.ControlState.CancelPending {
		// TODO: Engine should handle order cancellation
		// For now, just mark as processed
		bs.ControlState.CancelPending = false
		log.Printf("[%s] Order cancellation requested", bs.ID)
	}

	// Step 2: Close positions if any
	if !bs.EstimatedPosition.IsFlat() {
		bs.generateFlattenOrders(currentPrice)
	}

	// Step 3: Check if exit can be completed
	if bs.ControlState.ExitRequested {
		bs.CompleteExit()
	}
}

// generateFlattenOrders generates orders to close positions
func (bs *BaseStrategy) generateFlattenOrders(currentPrice float64) {
	if bs.Config == nil || len(bs.Config.Symbols) == 0 {
		return
	}

	tickSize := 0.01 // Default tick size
	if bs.Config.Parameters != nil {
		if ts, ok := bs.Config.Parameters["tick_size"].(float64); ok {
			tickSize = ts
		}
	}

	var signal *TradingSignal
	symbol := bs.Config.Symbols[0] // Use first symbol

	if bs.EstimatedPosition.IsLong() {
		// Close long position: sell
		price := currentPrice
		if bs.ControlState.AggressiveFlat {
			// Aggressive mode: cross the spread (sell at bid or lower)
			// 对应 tbsrc: m_aggFlat ? bidPx[0] - tickSize : askPx[0]
			price = currentPrice - tickSize
			log.Printf("[%s] Aggressive flatten: SELL at %.2f (cross spread)", bs.ID, price)
		}

		signal = &TradingSignal{
			StrategyID: bs.ID,
			Symbol:     symbol,
			Side:       OrderSideSell,
			Quantity:   bs.EstimatedPosition.LongQty,
			Price:      price,
			OrderType:  OrderTypeLimit,
			Timestamp:  time.Now(),
		}
	} else if bs.EstimatedPosition.IsShort() {
		// Close short position: buy
		price := currentPrice
		if bs.ControlState.AggressiveFlat {
			// Aggressive mode: cross the spread (buy at ask or higher)
			// 对应 tbsrc: m_aggFlat ? askPx[0] + tickSize : bidPx[0]
			price = currentPrice + tickSize
			log.Printf("[%s] Aggressive flatten: BUY at %.2f (cross spread)", bs.ID, price)
		}

		signal = &TradingSignal{
			StrategyID: bs.ID,
			Symbol:     symbol,
			Side:       OrderSideBuy,
			Quantity:   bs.EstimatedPosition.ShortQty,
			Price:      price,
			OrderType:  OrderTypeLimit,
			Timestamp:  time.Now(),
		}
	}

	if signal != nil {
		bs.AddSignal(signal)
		sideStr := "BUY"
		if signal.Side == OrderSideSell {
			sideStr = "SELL"
		}
		log.Printf("[%s] Flatten order: %s %d @ %.2f", bs.ID, sideStr, signal.Quantity, signal.Price)
	}
}

// =============================================================================
// Order Sending Control (对应 tbsrc: !m_onFlat && m_Active)
// =============================================================================

// CanSendOrder returns true if the strategy can send new orders
// 对应 tbsrc: !m_onFlat && m_Active (used in SetTargetValue)
func (bs *BaseStrategy) CanSendOrder() bool {
	return bs.ControlState.CanSendNewOrders()
}

// =============================================================================
// Risk Check Integration
// =============================================================================

// CheckAndHandleRiskLimits checks risk limits and triggers appropriate actions
// 对应 tbsrc: CheckSquareoff() logic
func (bs *BaseStrategy) CheckAndHandleRiskLimits() {
	if bs.ControlState.ExitRequested {
		return // Already exiting
	}

	// Check stop loss (unrealized + net PNL)
	// 对应 tbsrc: m_unrealisedPNL < UPNL_LOSS * -1 || m_netPNL < STOP_LOSS * -1
	if bs.Config != nil && bs.Config.RiskLimits != nil {
		if stopLoss, ok := bs.Config.RiskLimits["stop_loss"]; ok {
			if bs.PNL.UnrealizedPnL < stopLoss*-1 || bs.PNL.NetPnL < stopLoss*-1 {
				if !bs.ControlState.FlattenMode {
					bs.TriggerFlatten(FlattenReasonStopLoss, false)
				}
			}
		}

		// Check max loss
		if maxLoss, ok := bs.Config.RiskLimits["max_loss"]; ok {
			if bs.PNL.NetPnL < maxLoss*-1 {
				if !bs.ControlState.ExitRequested {
					bs.TriggerExit("Maximum loss limit reached")
				}
			}
		}
	}

	// Check reject limit
	// 对应 tbsrc: m_rejectCount > REJECT_LIMIT
	const REJECT_LIMIT = 10
	if bs.Status.RejectCount > REJECT_LIMIT {
		if !bs.ControlState.ExitRequested {
			bs.TriggerExit("Too many order rejections")
		}
	}

	// Try recovery if applicable
	if bs.ControlState.FlattenMode && !bs.ControlState.ExitRequested {
		bs.TryRecover()
	}
}

// =============================================================================
// Status Reporting
// =============================================================================

// GetControlStateString returns a human-readable control state string
func (bs *BaseStrategy) GetControlStateString() string {
	return bs.ControlState.String()
}

// UpdateControlStateInStatus updates the strategy status with control state info
func (bs *BaseStrategy) UpdateControlStateInStatus() {
	if bs.Status != nil {
		bs.Status.IsRunning = bs.ControlState.IsActivated() &&
			bs.ControlState.RunState != StrategyRunStateStopped
	}
}
