package main

import (
	"log"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

// StateControlExampleStrategy demonstrates state control features
// Aligned with tbsrc's m_Active, m_onFlat, m_onCancel, m_onExit, m_aggFlat
type StateControlExampleStrategy struct {
	*strategy.BaseStrategy
	tradeCount int
}

func NewStateControlExampleStrategy(id string, config *strategy.StrategyConfig) *StateControlExampleStrategy {
	return &StateControlExampleStrategy{
		BaseStrategy: strategy.NewBaseStrategy(id, "state_control_example"),
		tradeCount:   0,
	}
}

// OnMarketData handles market data
func (sces *StateControlExampleStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
	sces.PrivateIndicators.UpdateAll(md)

	// Check if we can send orders (aligned with tbsrc: !m_onFlat && m_Active)
	if !sces.CanSendOrder() {
		log.Printf("[%s] Cannot send order: %s", sces.ID, sces.ControlState.String())
		return
	}

	// Simple trading logic: buy on every tick
	if len(md.BidPrice) > 0 {
		signal := &strategy.TradingSignal{
			StrategyID: sces.ID,
			Symbol:     md.Symbol,
			Side:       orspb.OrderSide_BUY,
			Qty:        10,
			Price:      md.BidPrice[0],
			Type:       orspb.OrderType_LIMIT,
			Timestamp:  time.Now(),
		}
		sces.AddSignal(signal)
		sces.tradeCount++
		log.Printf("[%s] Trade #%d: BUY 10 @ %.2f", sces.ID, sces.tradeCount, signal.Price)
	}
}

// OnAuctionData handles auction period
func (sces *StateControlExampleStrategy) OnAuctionData(md *mdpb.MarketDataUpdate) {
	log.Printf("[%s] Auction period: no trading", sces.ID)
}

// OnOrderUpdate handles order updates
func (sces *StateControlExampleStrategy) OnOrderUpdate(update *orspb.OrderUpdate) {
	sces.UpdatePosition(update)

	if update.Status == orspb.OrderStatus_FILLED {
		log.Printf("[%s] Order filled: %s %d @ %.2f, NetPos=%d",
			sces.ID, update.Side, update.FilledQty, update.AvgPrice, sces.Position.NetQty)

		// Update P&L
		if len(update.Symbol) > 0 {
			// Use last price as current price (simplified)
			sces.UpdatePNL(update.AvgPrice)
		}
	}
}

// OnTimer is called periodically
func (sces *StateControlExampleStrategy) OnTimer(now time.Time) {
	// Periodic logging
	if now.Second()%10 == 0 {
		log.Printf("[%s] State=%s, Pos=%d, PNL=%.2f, Trades=%d",
			sces.ID, sces.ControlState.RunState, sces.Position.NetQty,
			sces.PNL.NetPnL, sces.tradeCount)
	}
}

// Implement required Strategy interface methods
func (sces *StateControlExampleStrategy) Initialize(config *strategy.StrategyConfig) error {
	sces.Config = config
	return nil
}

func (sces *StateControlExampleStrategy) Start() error {
	sces.IsRunningFlag = true
	log.Printf("[%s] Started", sces.ID)
	return nil
}

func (sces *StateControlExampleStrategy) Stop() error {
	sces.IsRunningFlag = false
	log.Printf("[%s] Stopped", sces.ID)
	return nil
}

func (sces *StateControlExampleStrategy) Reset() {
	sces.BaseStrategy.Reset()
	sces.tradeCount = 0
}

func main() {
	log.Println("=== State Control Example (tbsrc m_Active/m_onFlat/m_onExit aligned) ===")

	// Create strategy
	strat := NewStateControlExampleStrategy("state_ctrl_1", &strategy.StrategyConfig{
		Symbol: "IF2501",
		RiskLimits: map[string]float64{
			"stop_loss": 1000.0,  // Stop loss at -1000
			"max_loss":  5000.0,  // Exit at -5000
		},
	})

	// Initialize and start
	strat.Initialize(strat.Config)
	strat.Start()

	log.Println("\n=== Scenario 1: Normal Trading (m_Active=true, m_onFlat=false) ===")
	log.Printf("Initial state: %s\n", strat.ControlState.String())

	// Simulate market data
	md1 := &mdpb.MarketDataUpdate{
		Symbol:    "IF2501",
		FeedType:  mdpb.FeedType_CONTINUOUS,
		BidPrice:  []float64{4500.0},
		AskPrice:  []float64{4500.5},
		Timestamp: uint64(time.Now().UnixNano()),
	}
	strat.OnMarketData(md1)

	log.Println("\n=== Scenario 2: Manual Deactivation (m_Active=false) ===")
	strat.Deactivate()
	log.Printf("After deactivation: %s\n", strat.ControlState.String())

	// Try to trade (should be blocked)
	strat.OnMarketData(md1)

	log.Println("\n=== Scenario 3: Reactivation (m_Active=true) ===")
	strat.Activate()
	log.Printf("After activation: %s\n", strat.ControlState.String())
	strat.OnMarketData(md1)

	log.Println("\n=== Scenario 4: Stop Loss Trigger (m_onFlat=true, m_onStopLoss=true) ===")
	// Simulate loss
	strat.PNL.UnrealizedPnL = -1500.0 // Exceeds stop_loss limit
	strat.PNL.NetPnL = -1500.0

	// Manually trigger flatten (in real scenario, CheckAndHandleRiskLimits() does this)
	strat.TriggerFlatten(strategy.FlattenReasonStopLoss, false)
	log.Printf("After stop loss: %s\n", strat.ControlState.String())

	// Try to trade (should be blocked)
	strat.OnMarketData(md1)

	// Handle flatten process
	strat.HandleFlatten(4500.0)
	log.Printf("Flatten orders generated\n")

	log.Println("\n=== Scenario 5: Auto Recovery (m_onFlat=false after cooldown) ===")
	// Simulate position flat
	strat.Position.LongQty = 0
	strat.Position.ShortQty = 0
	strat.Position.NetQty = 0

	// Simulate P&L recovery
	strat.PNL.UnrealizedPnL = -500.0
	strat.PNL.NetPnL = -500.0

	// Set recovery time to now (skip cooldown for demo)
	strat.ControlState.CanRecoverAt = time.Now()

	// Try recovery
	if strat.TryRecover() {
		log.Printf("Recovery successful: %s\n", strat.ControlState.String())
		strat.OnMarketData(md1) // Can trade again
	} else {
		log.Printf("Recovery failed\n")
	}

	log.Println("\n=== Scenario 6: Max Loss Exit (m_onExit=true) ===")
	// Simulate max loss
	strat.PNL.NetPnL = -6000.0 // Exceeds max_loss limit

	// Trigger exit (in real scenario, CheckAndHandleRiskLimits() does this)
	strat.TriggerExit("Maximum loss limit reached")
	log.Printf("After exit trigger: %s\n", strat.ControlState.String())

	// Try to trade (should be blocked)
	strat.OnMarketData(md1)

	// Handle exit process
	strat.HandleFlatten(4500.0)

	// Complete exit (requires flat position)
	strat.Position.LongQty = 0
	strat.Position.ShortQty = 0
	strat.Position.NetQty = 0
	strat.CompleteExit()
	log.Printf("Exit completed: %s\n", strat.ControlState.String())

	log.Println("\n=== Scenario 7: Try Recovery After Exit (should fail) ===")
	if strat.TryRecover() {
		log.Printf("Recovery successful (unexpected!)\n")
	} else {
		log.Printf("Recovery failed: Cannot recover after exit\n")
	}

	log.Println("\n=== Scenario 8: Aggressive Flatten Mode (m_aggFlat=true) ===")
	// Create new strategy
	strat2 := NewStateControlExampleStrategy("state_ctrl_2", &strategy.StrategyConfig{
		Symbol: "IC2501",
		Parameters: map[string]interface{}{
			"tick_size": 0.2,
		},
	})
	strat2.Initialize(strat2.Config)
	strat2.Start()

	// Simulate position
	strat2.Position.LongQty = 100
	strat2.Position.NetQty = 100
	strat2.Position.AvgLongPrice = 7000.0

	// Trigger aggressive flatten
	strat2.TriggerFlatten(strategy.FlattenReasonManual, true) // aggressive=true
	log.Printf("Aggressive flatten triggered: %s\n", strat2.ControlState.String())

	// Generate flatten orders (will cross the spread)
	strat2.HandleFlatten(7000.0)
	signals := strat2.GetSignals()
	if len(signals) > 0 {
		log.Printf("Aggressive flatten order: %s %d @ %.2f (crosses spread)",
			signals[0].Side, signals[0].Qty, signals[0].Price)
	}

	log.Println("\n=== State Control Features Summary ===")
	log.Println("│")
	log.Println("├─ Activation Control (m_Active):")
	log.Println("│  ├─ Activate() / Deactivate()")
	log.Println("│  └─ Required for order sending (like tbsrc live mode)")
	log.Println("│")
	log.Println("├─ Flatten Mode (m_onFlat):")
	log.Println("│  ├─ TriggerFlatten(reason, aggressive)")
	log.Println("│  ├─ TryRecover() - Auto recovery after cooldown")
	log.Println("│  └─ Reasons: StopLoss, PriceLimit, DeltaLimit, etc.")
	log.Println("│")
	log.Println("├─ Exit Mode (m_onExit):")
	log.Println("│  ├─ TriggerExit(reason) - Cannot recover")
	log.Println("│  └─ CompleteExit() - Final shutdown")
	log.Println("│")
	log.Println("├─ Aggressive Flatten (m_aggFlat):")
	log.Println("│  └─ Crosses spread for quick position close")
	log.Println("│")
	log.Println("└─ State Checks:")
	log.Println("   ├─ CanSendOrder() - Pre-send validation")
	log.Println("   ├─ CheckAndHandleRiskLimits() - Auto risk mgmt")
	log.Println("   └─ HandleFlatten() - Auto position close")
	log.Println("\nAll features aligned with tbsrc ExecutionStrategy! ✅")
}
