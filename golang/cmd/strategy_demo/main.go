package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func main() {
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║         HFT Strategy Engine Demo                         ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Create strategy engine configuration
	engineConfig := &strategy.EngineConfig{
		ORSGatewayAddr:      "localhost:50052",
		NATSAddr:            "nats://localhost:4222",
		OrderQueueSize:      100,
		TimerInterval:       5 * time.Second,
		MaxConcurrentOrders: 10,
	}

	// Create strategy engine
	engine := strategy.NewStrategyEngine(engineConfig)

	// Initialize engine (will try to connect to services)
	fmt.Println("[Main] Initializing strategy engine...")
	if err := engine.Initialize(); err != nil {
		log.Printf("[Main] Warning: Failed to initialize engine (services may not be running): %v", err)
		log.Println("[Main] Continuing in demo mode without real connections...")
	} else {
		fmt.Println("[Main] ✓ Strategy engine initialized")
	}

	// Create passive strategy
	fmt.Println("\n[Main] Creating passive market making strategy...")
	passive := strategy.NewPassiveStrategy("passive_1")

	// Configure the passive strategy
	strategyConfig := &strategy.StrategyConfig{
		StrategyID:      "passive_1",
		StrategyType:    "passive",
		Symbols:         []string{"ag2412"},
		Exchanges:       []string{"SHFE"},
		MaxPositionSize: 100,
		MaxExposure:     1000000.0,
		RiskLimits: map[string]float64{
			"max_drawdown": 10000.0,
		},
		Parameters: map[string]interface{}{
			"spread_multiplier":   0.5,
			"order_size":          10.0,
			"max_inventory":       100.0,
			"inventory_skew":      0.5,
			"min_spread":          1.0,
			"order_refresh_ms":    1000.0,
			"use_order_imbalance": true,
		},
		Enabled: true,
	}

	// Initialize strategy
	if err := passive.Initialize(strategyConfig); err != nil {
		log.Fatalf("[Main] Failed to initialize strategy: %v", err)
	}
	fmt.Println("[Main] ✓ Strategy initialized")

	// Add strategy to engine
	if err := engine.AddStrategy(passive); err != nil {
		log.Fatalf("[Main] Failed to add strategy to engine: %v", err)
	}
	fmt.Println("[Main] ✓ Strategy added to engine")

	// Print strategy info
	fmt.Println("\n" + passive.GetStrategyInfo())

	// Start the strategy
	fmt.Println("\n[Main] Starting strategy...")
	if err := passive.Start(); err != nil {
		log.Fatalf("[Main] Failed to start strategy: %v", err)
	}
	fmt.Println("[Main] ✓ Strategy started")

	// Start the engine (this will fail if services aren't running, but that's OK for demo)
	fmt.Println("\n[Main] Starting strategy engine...")
	if err := engine.Start(); err != nil {
		log.Printf("[Main] Warning: Failed to start engine: %v", err)
	}

	fmt.Println("\n════════════════════════════════════════════════════════════")
	fmt.Println("Strategy Engine Running - Simulating Market Data")
	fmt.Println("════════════════════════════════════════════════════════════")

	// Simulate market data updates
	go simulateMarketData(passive)

	// Print statistics periodically
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	go func() {
		for range ticker.C {
			printStatistics(passive)
		}
	}()

	// Wait for interrupt signal
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	fmt.Println("\n\n[Main] Shutting down...")

	// Stop strategy
	passive.Stop()

	// Stop engine
	if err := engine.Stop(); err != nil {
		log.Printf("[Main] Error stopping engine: %v", err)
	}

	// Print final statistics
	fmt.Println("\n════════════════════════════════════════════════════════════")
	fmt.Println("Final Statistics")
	fmt.Println("════════════════════════════════════════════════════════════")
	printStatistics(passive)

	fmt.Println("\n✓ Strategy engine demo completed!")
}

// simulateMarketData simulates market data updates
func simulateMarketData(strategy strategy.Strategy) {
	basePrice := 7950.0
	tickCount := 0

	for {
		if !strategy.IsRunning() {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		tickCount++

		// Simulate price movement
		priceMove := float64(tickCount%20-10) * 2.0
		bidPrice := basePrice + priceMove - 1.0
		askPrice := basePrice + priceMove + 1.0

		// Create synthetic market data
		md := &mdpb.MarketDataUpdate{
			Symbol:      "ag2412",
			Exchange:    "SHFE",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{bidPrice, bidPrice - 2, bidPrice - 4, bidPrice - 6, bidPrice - 8},
			BidQty:      []uint32{100, 80, 60, 50, 40},
			AskPrice:    []float64{askPrice, askPrice + 2, askPrice + 4, askPrice + 6, askPrice + 8},
			AskQty:      []uint32{95, 75, 55, 45, 35},
			LastPrice:   bidPrice + 1.0,
			TotalVolume: uint64(1000 + tickCount*10),
			Turnover:    (bidPrice + 1.0) * float64(1000+tickCount*10),
		}

		// Send market data to strategy
		strategy.OnMarketData(md)

		// Check for signals
		signals := strategy.GetSignals()
		if len(signals) > 0 {
			fmt.Printf("\n[Tick %d] Generated %d signals:\n", tickCount, len(signals))
			for _, signal := range signals {
				sideStr := "BUY"
				if signal.Side == 2 { // OrderSideSell
					sideStr = "SELL"
				}
				fmt.Printf("  %s %s @ %.2f, qty=%d, signal=%.2f, confidence=%.2f\n",
					sideStr, signal.Symbol, signal.Price, signal.Quantity, signal.Signal, signal.Confidence)
			}
		}

		// Print progress every 10 ticks
		if tickCount%10 == 0 {
			position := strategy.GetPosition()
			pnl := strategy.GetPNL()
			fmt.Printf("[Tick %d] Price: %.2f, Position: %d, PnL: %.2f\n",
				tickCount, bidPrice+1.0, position.NetQty, pnl.TotalPnL)
		}

		// Sleep to simulate market data rate
		time.Sleep(100 * time.Millisecond)
	}
}

// printStatistics prints strategy statistics
func printStatistics(strategy strategy.Strategy) {
	status := strategy.GetStatus()
	position := strategy.GetPosition()
	pnl := strategy.GetPNL()
	risk := strategy.GetRiskMetrics()

	fmt.Println("\n┌────────────────────────────────────────────────────────────┐")
	fmt.Printf("│ Strategy: %-49s│\n", status.StrategyID)
	fmt.Println("├────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ Running:          %-41v│\n", status.IsRunning)
	fmt.Printf("│ Position:         %-41d│\n", position.NetQty)
	fmt.Printf("│   Long:           %-41d│\n", position.LongQty)
	fmt.Printf("│   Short:          %-41d│\n", position.ShortQty)
	fmt.Printf("│   Avg Long:       %-41.2f│\n", position.AvgLongPrice)
	fmt.Printf("│   Avg Short:      %-41.2f│\n", position.AvgShortPrice)
	fmt.Println("├────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ P&L Total:        %-41.2f│\n", pnl.TotalPnL)
	fmt.Printf("│   Realized:       %-41.2f│\n", pnl.RealizedPnL)
	fmt.Printf("│   Unrealized:     %-41.2f│\n", pnl.UnrealizedPnL)
	fmt.Printf("│   Trading Fees:   %-41.2f│\n", pnl.TradingFees)
	fmt.Printf("│   Net P&L:        %-41.2f│\n", pnl.NetPnL)
	fmt.Println("├────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ Signals:          %-41d│\n", status.SignalCount)
	fmt.Printf("│ Orders:           %-41d│\n", status.OrderCount)
	fmt.Printf("│ Fills:            %-41d│\n", status.FillCount)
	fmt.Printf("│ Rejects:          %-41d│\n", status.RejectCount)
	fmt.Println("├────────────────────────────────────────────────────────────┤")
	fmt.Printf("│ Max Drawdown:     %-41.2f│\n", risk.MaxDrawdown)
	fmt.Printf("│ Exposure:         %-41.2f│\n", risk.ExposureValue)
	fmt.Println("└────────────────────────────────────────────────────────────┘")
}
