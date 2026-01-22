package main

import (
	"log"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

// DetailedOrderStrategy demonstrates fine-grained order event callbacks
// This provides more granular control than the generic OnOrderUpdate
type DetailedOrderCallbackStrategy struct {
	*strategy.BaseStrategy
	newOrderCount      int
	filledOrderCount   int
	canceledOrderCount int
	rejectedOrderCount int
}

func NewDetailedOrderCallbackStrategy(id string, config *strategy.StrategyConfig) *DetailedOrderCallbackStrategy {
	return &DetailedOrderCallbackStrategy{
		BaseStrategy: strategy.NewBaseStrategy(id, "detailed_order"),
	}
}

// OnMarketData generates trading signals
func (docs *DetailedOrderCallbackStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
	docs.PrivateIndicators.UpdateAll(md)

	// Simple signal generation
	if len(md.BidPrice) > 0 {
		signal := &strategy.TradingSignal{
			StrategyID: docs.GetID(),
			Symbol:     md.Symbol,
			Side:       orspb.OrderSide_BUY,
			Qty:        10,
			Price:      md.BidPrice[0],
			Type:       orspb.OrderType_LIMIT,
			Timestamp:  time.Now(),
		}
		docs.AddSignal(signal)
	}
}

// OnAuctionData handles auction period
func (docs *DetailedOrderCallbackStrategy) OnAuctionData(md *mdpb.MarketDataUpdate) {
	// No auction trading
}

// OnOrderUpdate is the generic callback (still called)
func (docs *DetailedOrderCallbackStrategy) OnOrderUpdate(update *orspb.OrderUpdate) {
	docs.UpdatePosition(update)
	log.Printf("[%s] Generic order update: OrderID=%s, Status=%v",
		docs.GetID(), update.OrderId, update.Status)
}

// === Fine-grained order callbacks (optional, more specific) ===

// OnOrderNew is called when order is confirmed by exchange
func (docs *DetailedOrderCallbackStrategy) OnOrderNew(update *orspb.OrderUpdate) {
	docs.newOrderCount++
	log.Printf("[%s] ✅ ORDER NEW: OrderID=%s, Symbol=%s, Side=%v, Qty=%d, Price=%.2f (Total NEW: %d)",
		docs.GetID(), update.OrderId, update.Symbol, update.Side, update.Qty, update.Price, docs.newOrderCount)

	// Strategy-specific logic for new order confirmation
	// e.g., Start monitoring for fill, adjust risk limits
}

// OnOrderFilled is called when order is filled (partially or fully)
func (docs *DetailedOrderCallbackStrategy) OnOrderFilled(update *orspb.OrderUpdate) {
	docs.filledOrderCount++
	fillType := "FULLY"
	if update.Status == orspb.OrderStatus_PARTIALLY_FILLED {
		fillType = "PARTIALLY"
	}

	log.Printf("[%s] 💰 ORDER FILLED (%s): OrderID=%s, FilledQty=%d/%d, AvgPrice=%.2f (Total FILLED: %d)",
		docs.GetID(), fillType, update.OrderId, update.FilledQty, update.Qty, update.AvgPrice, docs.filledOrderCount)

	// Strategy-specific logic for fill
	// e.g., Trigger hedging, update P&L, send notification
	if update.Status == orspb.OrderStatus_FILLED {
		log.Printf("[%s]   → Position updated: NetQty=%d, UnrealizedPnL=%.2f",
			docs.GetID(), docs.Position.NetQty, docs.PNL.UnrealizedPnL)
	}
}

// OnOrderCanceled is called when order is canceled
func (docs *DetailedOrderCallbackStrategy) OnOrderCanceled(update *orspb.OrderUpdate) {
	docs.canceledOrderCount++
	log.Printf("[%s] ❌ ORDER CANCELED: OrderID=%s, CanceledQty=%d (Total CANCELED: %d)",
		docs.GetID(), update.OrderId, update.Qty-update.FilledQty, docs.canceledOrderCount)

	// Strategy-specific logic for cancel
	// e.g., Resubmit order, log for analysis
}

// OnOrderRejected is called when order is rejected
func (docs *DetailedOrderCallbackStrategy) OnOrderRejected(update *orspb.OrderUpdate) {
	docs.rejectedOrderCount++
	log.Printf("[%s] ⛔ ORDER REJECTED: OrderID=%s, Reason=%s (Total REJECTED: %d)",
		docs.GetID(), update.OrderId, update.ErrorMessage, docs.rejectedOrderCount)

	// Strategy-specific logic for reject
	// e.g., Check risk limits, adjust order size, pause trading
	if docs.rejectedOrderCount > 5 {
		log.Printf("[%s]   ⚠️  TOO MANY REJECTIONS! Consider pausing strategy", docs.GetID())
	}
}

// Implement required Strategy interface methods
func (docs *DetailedOrderCallbackStrategy) Initialize(config *strategy.StrategyConfig) error {
	docs.Config = config
	return nil
}

func (docs *DetailedOrderCallbackStrategy) Start() error {
	docs.IsRunningFlag = true
	log.Printf("[%s] Started", docs.GetID())
	return nil
}

func (docs *DetailedOrderCallbackStrategy) Stop() error {
	docs.IsRunningFlag = false
	log.Printf("[%s] Stopped", docs.GetID())
	return nil
}

func (docs *DetailedOrderCallbackStrategy) OnTimer(now time.Time) {
	// Print statistics periodically
	if now.Second()%10 == 0 {
		log.Printf("[%s] Order Statistics: NEW=%d, FILLED=%d, CANCELED=%d, REJECTED=%d",
			docs.GetID(), docs.newOrderCount, docs.filledOrderCount,
			docs.canceledOrderCount, docs.rejectedOrderCount)
	}
}

func main() {
	log.Println("=== Detailed Order Callback Example ===")

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

	// Create strategy with detailed order callbacks
	detailStrat := NewDetailedOrderCallbackStrategy("detail_order_1", &strategy.StrategyConfig{
		Symbol: "IF2501",
	})

	// Add strategy
	engine.AddStrategy(detailStrat)

	// Start engine
	if err := engine.Start(); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}

	// Subscribe to market data
	if err := engine.SubscribeMarketData("IF2501"); err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	log.Println("\n=== Order Event Flow ===")
	log.Println("1. Order sent to exchange")
	log.Println("2. Engine receives order update")
	log.Println("3. Engine calls OnOrderUpdate() (generic)")
	log.Println("4. Engine calls specific callback based on status:")
	log.Println("   - OnOrderNew() for NEW")
	log.Println("   - OnOrderFilled() for FILLED/PARTIALLY_FILLED")
	log.Println("   - OnOrderCanceled() for CANCELED")
	log.Println("   - OnOrderRejected() for REJECTED\n")

	log.Println("[Engine Started] Press Ctrl+C to stop")

	// Simulate order updates for demonstration
	go func() {
		time.Sleep(2 * time.Second)

		// Simulate NEW order
		newUpdate := &orspb.OrderUpdate{
			OrderId: "ORDER001",
			Symbol:  "IF2501",
			Side:    orspb.OrderSide_BUY,
			Qty:     10,
			Price:   4500.0,
			Status:  orspb.OrderStatus_NEW,
		}
		log.Println("\n[Simulator] Sending NEW order update...")
		detailStrat.OnOrderUpdate(newUpdate)
		detailStrat.OnOrderNew(newUpdate)

		time.Sleep(1 * time.Second)

		// Simulate FILLED order
		filledUpdate := &orspb.OrderUpdate{
			OrderId:  "ORDER001",
			Symbol:   "IF2501",
			Side:     orspb.OrderSide_BUY,
			Qty:      10,
			FilledQty: 10,
			Price:    4500.0,
			AvgPrice: 4500.5,
			Status:   orspb.OrderStatus_FILLED,
		}
		log.Println("\n[Simulator] Sending FILLED order update...")
		detailStrat.OnOrderUpdate(filledUpdate)
		detailStrat.OnOrderFilled(filledUpdate)

		time.Sleep(1 * time.Second)

		// Simulate REJECTED order
		rejectedUpdate := &orspb.OrderUpdate{
			OrderId:      "ORDER002",
			Symbol:       "IF2501",
			Side:         orspb.OrderSide_SELL,
			Qty:          10,
			Price:        4501.0,
			Status:       orspb.OrderStatus_REJECTED,
			ErrorMessage: "Insufficient margin",
		}
		log.Println("\n[Simulator] Sending REJECTED order update...")
		detailStrat.OnOrderUpdate(rejectedUpdate)
		detailStrat.OnOrderRejected(rejectedUpdate)
	}()

	// Keep running
	select {}
}
