package main

import (
	"log"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/indicators"
	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

// IndicatorCallbackStrategy demonstrates how to use OnIndicatorUpdate callback
// This aligns with tbsrc's INDCallBack concept
type IndicatorCallbackStrategy struct {
	*strategy.BaseStrategy
	lastVWAP       float64
	lastVolatility float64
}

func NewIndicatorCallbackStrategy(id string, config *strategy.StrategyConfig) *IndicatorCallbackStrategy {
	return &IndicatorCallbackStrategy{
		BaseStrategy: strategy.NewBaseStrategy(id, "indicator_callback"),
	}
}

// OnIndicatorUpdate is called AFTER shared indicators are updated
// This is the explicit indicator callback (like tbsrc INDCallBack)
func (ics *IndicatorCallbackStrategy) OnIndicatorUpdate(symbol string, indicators *indicators.IndicatorLibrary) {
	log.Printf("[%s] *** INDICATOR UPDATE ***: Symbol=%s", ics.GetID(), symbol)

	// Access shared indicators that were just updated
	if vwap, ok := indicators.Get("vwap"); ok {
		ics.lastVWAP = vwap.Value()
		log.Printf("[%s]   Shared VWAP updated: %.2f", ics.GetID(), ics.lastVWAP)
	}

	if volatility, ok := indicators.Get("volatility"); ok {
		ics.lastVolatility = volatility.Value()
		log.Printf("[%s]   Shared Volatility updated: %.4f", ics.GetID(), ics.lastVolatility)
	}

	// Example: Pre-market data validation
	if ics.lastVolatility > 0.05 {
		log.Printf("[%s]   ⚠️  HIGH VOLATILITY DETECTED! Consider adjusting strategy parameters", ics.GetID())
	}

	// Example: Cross-strategy coordination point
	// At this point, all shared indicators are updated but strategies haven't
	// generated signals yet. This is a good place to:
	// - Validate market conditions
	// - Adjust strategy parameters based on market regime
	// - Coordinate between multiple strategies
}

// OnMarketData handles market data (called AFTER OnIndicatorUpdate)
func (ics *IndicatorCallbackStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
	log.Printf("[%s] Market data: Symbol=%s, BidPrice=%.2f, AskPrice=%.2f",
		ics.GetID(), md.Symbol, md.BidPrice[0], md.AskPrice[0])

	// Update private indicators
	ics.PrivateIndicators.UpdateAll(md)

	// Generate signals using shared indicators (already cached from OnIndicatorUpdate)
	if len(md.BidPrice) > 0 && ics.lastVWAP > 0 {
		currentPrice := (md.BidPrice[0] + md.AskPrice[0]) / 2

		// Trading logic: Buy when price < VWAP and volatility is low
		if currentPrice < ics.lastVWAP && ics.lastVolatility < 0.03 {
			signal := &strategy.TradingSignal{
				StrategyID: ics.GetID(),
				Symbol:     md.Symbol,
				Side:       orspb.OrderSide_BUY,
				Qty:        10,
				Price:      md.BidPrice[0],
				Type:       orspb.OrderType_LIMIT,
				Timestamp:  time.Now(),
			}
			ics.AddSignal(signal)
			log.Printf("[%s] BUY signal: Price=%.2f < VWAP=%.2f, Vol=%.4f",
				ics.GetID(), currentPrice, ics.lastVWAP, ics.lastVolatility)
		}
	}
}

// OnAuctionData handles auction period data
func (ics *IndicatorCallbackStrategy) OnAuctionData(md *mdpb.MarketDataUpdate) {
	// Default: no action during auction
	log.Printf("[%s] Auction period: No trading", ics.GetID())
}

// OnOrderUpdate handles order updates
func (ics *IndicatorCallbackStrategy) OnOrderUpdate(update *orspb.OrderUpdate) {
	ics.UpdatePosition(update)
	log.Printf("[%s] Order update: OrderID=%s, Status=%v",
		ics.GetID(), update.OrderId, update.Status)
}

// Implement required Strategy interface methods
func (ics *IndicatorCallbackStrategy) Initialize(config *strategy.StrategyConfig) error {
	ics.Config = config
	return nil
}

func (ics *IndicatorCallbackStrategy) Start() error {
	ics.IsRunningFlag = true
	log.Printf("[%s] Started", ics.GetID())
	return nil
}

func (ics *IndicatorCallbackStrategy) Stop() error {
	ics.IsRunningFlag = false
	log.Printf("[%s] Stopped", ics.GetID())
	return nil
}

func (ics *IndicatorCallbackStrategy) OnTimer(now time.Time) {
	// Periodic tasks
}

func main() {
	log.Println("=== Indicator Callback Example (tbsrc INDCallBack aligned) ===")

	// Create engine
	config := &strategy.EngineConfig{
		ORSGatewayAddr: "localhost:50052",
		NATSAddr:       "nats://localhost:4222",
		OrderQueueSize: 1000,
		TimerInterval:  100 * time.Millisecond,
		OrderMode:      strategy.OrderModeSync,
		OrderTimeout:   50 * time.Millisecond,
	}

	engine := strategy.NewStrategyEngine(config)
	if err := engine.Initialize(); err != nil {
		log.Fatalf("Failed to initialize engine: %v", err)
	}

	// Initialize shared indicators for symbol
	symbol := "IF2501"
	err := engine.InitializeSharedIndicators(symbol, map[string]interface{}{
		"vwap": map[string]interface{}{
			"window": 100,
		},
		"volatility": map[string]interface{}{
			"window": 20,
		},
	})
	if err != nil {
		log.Fatalf("Failed to init shared indicators: %v", err)
	}

	// Create strategy with indicator callback
	indStrat := NewIndicatorCallbackStrategy("ind_cb_1", &strategy.StrategyConfig{
		Symbol: symbol,
	})

	// Attach shared indicators
	engine.AttachSharedIndicators(indStrat, []string{symbol})

	// Add strategy
	engine.AddStrategy(indStrat)

	// Start engine
	if err := engine.Start(); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}

	// Subscribe to market data
	if err := engine.SubscribeMarketData(symbol); err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	log.Println("\n=== Event Flow (tbsrc-aligned) ===")
	log.Println("1. Market data arrives")
	log.Println("2. Engine updates shared indicators (ONCE for all strategies)")
	log.Println("3. Engine calls OnIndicatorUpdate() ← Explicit callback (like tbsrc INDCallBack)")
	log.Println("4. Engine calls OnMarketData() or OnAuctionData()")
	log.Println("5. Strategy generates signals")
	log.Println("6. Engine sends orders synchronously\n")

	log.Println("[Engine Started] Press Ctrl+C to stop")

	// Keep running
	select {}
}
