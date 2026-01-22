package main

import (
	"log"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

// AuctionAwareStrategy demonstrates how to implement auction-specific logic
// This aligns with tbsrc's AuctionCallBack concept
type AuctionAwareStrategy struct {
	*strategy.BaseStrategy
	auctionMode bool
	auctionBids []float64
}

func NewAuctionAwareStrategy(id string, config *strategy.StrategyConfig) *AuctionAwareStrategy {
	return &AuctionAwareStrategy{
		BaseStrategy: strategy.NewBaseStrategy(id, "auction_aware"),
		auctionBids:  make([]float64, 0),
	}
}

// OnMarketData handles continuous trading period data
func (as *AuctionAwareStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
	log.Printf("[%s] Continuous trading: Symbol=%s, BidPrice=%.2f, AskPrice=%.2f",
		as.GetID(), md.Symbol, md.BidPrice[0], md.AskPrice[0])

	as.auctionMode = false

	// Normal continuous trading logic
	as.PrivateIndicators.UpdateAll(md)

	// Generate signals based on continuous market conditions
	if len(md.BidPrice) > 0 && len(md.AskPrice) > 0 {
		spread := md.AskPrice[0] - md.BidPrice[0]
		if spread < 0.05 { // Tight spread, trade aggressively
			signal := &strategy.TradingSignal{
				StrategyID: as.GetID(),
				Symbol:     md.Symbol,
				Side:       orspb.OrderSide_BUY,
				Qty:        10,
				Price:      md.BidPrice[0],
				Type:       orspb.OrderType_LIMIT,
				Timestamp:  time.Now(),
			}
			as.AddSignal(signal)
		}
	}
}

// OnAuctionData handles auction period data (like tbsrc AuctionCallBack)
func (as *AuctionAwareStrategy) OnAuctionData(md *mdpb.MarketDataUpdate) {
	log.Printf("[%s] *** AUCTION PERIOD ***: Symbol=%s, BidPrice=%.2f, AskPrice=%.2f",
		as.GetID(), md.Symbol, md.BidPrice[0], md.AskPrice[0])

	as.auctionMode = true

	// Special auction logic:
	// 1. Collect bid prices during auction
	if len(md.BidPrice) > 0 {
		as.auctionBids = append(as.auctionBids, md.BidPrice[0])
	}

	// 2. Calculate auction reference price (e.g., average of collected bids)
	if len(as.auctionBids) > 0 {
		var sum float64
		for _, bid := range as.auctionBids {
			sum += bid
		}
		refPrice := sum / float64(len(as.auctionBids))

		// 3. Submit auction order near reference price
		if len(md.BidPrice) > 0 && md.BidPrice[0] > 0 {
			signal := &strategy.TradingSignal{
				StrategyID: as.GetID(),
				Symbol:     md.Symbol,
				Side:       orspb.OrderSide_BUY,
				Qty:        20, // Larger size during auction
				Price:      refPrice,
				Type:       orspb.OrderType_LIMIT,
				Timestamp:  time.Now(),
			}
			as.AddSignal(signal)
			log.Printf("[%s] Auction order submitted: Price=%.2f (Ref=%.2f)",
				as.GetID(), md.BidPrice[0], refPrice)
		}
	}
}

// OnOrderUpdate handles order updates
func (as *AuctionAwareStrategy) OnOrderUpdate(update *orspb.OrderUpdate) {
	as.UpdatePosition(update)
	log.Printf("[%s] Order update: OrderID=%s, Status=%v, FilledQty=%d",
		as.GetID(), update.OrderId, update.Status, update.FilledQty)

	// Clear auction bids after order is filled
	if update.Status == orspb.OrderStatus_FILLED && as.auctionMode {
		as.auctionBids = as.auctionBids[:0]
		log.Printf("[%s] Auction order filled, cleared bid history", as.GetID())
	}
}

// Implement required Strategy interface methods
func (as *AuctionAwareStrategy) Initialize(config *strategy.StrategyConfig) error {
	as.Config = config
	return nil
}

func (as *AuctionAwareStrategy) Start() error {
	as.IsRunningFlag = true
	log.Printf("[%s] Started", as.GetID())
	return nil
}

func (as *AuctionAwareStrategy) Stop() error {
	as.IsRunningFlag = false
	log.Printf("[%s] Stopped", as.GetID())
	return nil
}

func (as *AuctionAwareStrategy) OnTimer(now time.Time) {
	// Periodic tasks (e.g., risk check)
}

func main() {
	log.Println("=== Auction-Aware Strategy Example (tbsrc AuctionCallBack aligned) ===")

	// Create engine with sync mode
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

	// Create auction-aware strategy
	auctionStrat := NewAuctionAwareStrategy("auction_1", &strategy.StrategyConfig{
		Symbol: "IF2501",
		Parameters: map[string]interface{}{
			"auction_size": 20,
		},
	})

	// Add strategy to engine
	engine.AddStrategy(auctionStrat)

	// Start engine
	if err := engine.Start(); err != nil {
		log.Fatalf("Failed to start engine: %v", err)
	}

	// Subscribe to market data
	if err := engine.SubscribeMarketData("IF2501"); err != nil {
		log.Fatalf("Failed to subscribe: %v", err)
	}

	log.Println("[Engine Started] Listening for market data...")
	log.Println("  - Continuous trading: OnMarketData() will be called")
	log.Println("  - Auction period: OnAuctionData() will be called")
	log.Println("Press Ctrl+C to stop")

	// Simulate market data (for demonstration)
	go func() {
		time.Sleep(2 * time.Second)

		// Simulate continuous trading data
		continuousMD := &mdpb.MarketDataUpdate{
			Symbol:     "IF2501",
			FeedType:   mdpb.FeedType_CONTINUOUS,
			BidPrice:   []float64{4500.0},
			AskPrice:   []float64{4500.5},
			Timestamp:  uint64(time.Now().UnixNano()),
		}
		log.Println("\n[Simulator] Sending CONTINUOUS market data...")
		auctionStrat.OnMarketData(continuousMD)

		time.Sleep(2 * time.Second)

		// Simulate auction period data
		auctionMD := &mdpb.MarketDataUpdate{
			Symbol:     "IF2501",
			FeedType:   mdpb.FeedType_AUCTION,
			BidPrice:   []float64{4502.0},
			AskPrice:   []float64{4503.0},
			Timestamp:  uint64(time.Now().UnixNano()),
		}
		log.Println("\n[Simulator] Sending AUCTION market data...")
		auctionStrat.OnAuctionData(auctionMD)

		time.Sleep(1 * time.Second)

		// Another auction update
		auctionMD2 := &mdpb.MarketDataUpdate{
			Symbol:     "IF2501",
			FeedType:   mdpb.FeedType_AUCTION,
			BidPrice:   []float64{4503.0},
			AskPrice:   []float64{4504.0},
			Timestamp:  uint64(time.Now().UnixNano()),
		}
		log.Println("\n[Simulator] Sending another AUCTION update...")
		auctionStrat.OnAuctionData(auctionMD2)
	}()

	// Keep running
	select {}
}
