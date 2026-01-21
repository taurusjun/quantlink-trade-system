package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/hft-poc/pkg/strategy"
	mdpb "github.com/yourusername/hft-poc/pkg/proto/md"
)

func main() {
	fmt.Println("╔════════════════════════════════════════════════════════════╗")
	fmt.Println("║          HFT All Strategies Demo                          ║")
	fmt.Println("║    PassivePassive | Aggressive | Hedging | Pairs          ║")
	fmt.Println("╚════════════════════════════════════════════════════════════╝")
	fmt.Println()

	// Create all 4 strategy types
	strategies := make(map[string]strategy.Strategy)

	// 1. Passive Strategy
	fmt.Println("[Main] Creating Passive Strategy...")
	passive := strategy.NewPassiveStrategy("passive_ag")
	passiveConfig := &strategy.StrategyConfig{
		StrategyID:      "passive_ag",
		StrategyType:    "passive",
		Symbols:         []string{"ag2412"},
		Exchanges:       []string{"SHFE"},
		MaxPositionSize: 100,
		MaxExposure:     500000.0,
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
	if err := passive.Initialize(passiveConfig); err != nil {
		log.Fatalf("Failed to initialize passive strategy: %v", err)
	}
	passive.Start()
	strategies["passive"] = passive
	fmt.Println("[Main] ✓ Passive Strategy created")

	// 2. Aggressive Strategy
	fmt.Println("\n[Main] Creating Aggressive Strategy...")
	aggressive := strategy.NewAggressiveStrategy("aggressive_au")
	aggressiveConfig := &strategy.StrategyConfig{
		StrategyID:      "aggressive_au",
		StrategyType:    "aggressive",
		Symbols:         []string{"au2412"},
		Exchanges:       []string{"SHFE"},
		MaxPositionSize: 100,
		MaxExposure:     500000.0,
		Parameters: map[string]interface{}{
			"trend_period":         50.0,
			"momentum_period":      20.0,
			"signal_threshold":     0.6,
			"order_size":           20.0,
			"stop_loss_percent":    0.02,
			"take_profit_percent":  0.05,
			"min_volatility":       0.0001,
			"use_volatility_scale": true,
			"signal_refresh_ms":    2000.0,
		},
		Enabled: true,
	}
	if err := aggressive.Initialize(aggressiveConfig); err != nil {
		log.Fatalf("Failed to initialize aggressive strategy: %v", err)
	}
	aggressive.Start()
	strategies["aggressive"] = aggressive
	fmt.Println("[Main] ✓ Aggressive Strategy created")

	// 3. Hedging Strategy
	fmt.Println("\n[Main] Creating Hedging Strategy...")
	hedging := strategy.NewHedgingStrategy("hedging_ag")
	hedgingConfig := &strategy.StrategyConfig{
		StrategyID:      "hedging_ag",
		StrategyType:    "hedging",
		Symbols:         []string{"ag2412", "ag2501"}, // 2 symbols for hedge
		Exchanges:       []string{"SHFE", "SHFE"},
		MaxPositionSize: 100,
		MaxExposure:     500000.0,
		Parameters: map[string]interface{}{
			"hedge_ratio":           1.0,
			"rebalance_threshold":   0.1,
			"order_size":            10.0,
			"dynamic_hedge_ratio":   true,
			"correlation_period":    100.0,
			"target_delta":          0.0,
			"rebalance_interval_ms": 5000.0,
		},
		Enabled: true,
	}
	if err := hedging.Initialize(hedgingConfig); err != nil {
		log.Fatalf("Failed to initialize hedging strategy: %v", err)
	}
	hedging.Start()
	strategies["hedging"] = hedging
	fmt.Println("[Main] ✓ Hedging Strategy created")

	// 4. Pairwise Arbitrage Strategy
	fmt.Println("\n[Main] Creating Pairwise Arbitrage Strategy...")
	pairs := strategy.NewPairwiseArbStrategy("pairs_au")
	pairsConfig := &strategy.StrategyConfig{
		StrategyID:      "pairs_au",
		StrategyType:    "pairwise_arb",
		Symbols:         []string{"au2412", "au2501"}, // 2 symbols for pairs
		Exchanges:       []string{"SHFE", "SHFE"},
		MaxPositionSize: 50,
		MaxExposure:     300000.0,
		Parameters: map[string]interface{}{
			"lookback_period":   100.0,
			"entry_zscore":      2.0,
			"exit_zscore":       0.5,
			"order_size":        10.0,
			"min_correlation":   0.7,
			"spread_type":       "difference",
			"use_cointegration": false,
			"trade_interval_ms": 3000.0,
		},
		Enabled: true,
	}
	if err := pairs.Initialize(pairsConfig); err != nil {
		log.Fatalf("Failed to initialize pairs strategy: %v", err)
	}
	pairs.Start()
	strategies["pairs"] = pairs
	fmt.Println("[Main] ✓ Pairwise Arbitrage Strategy created")

	// Print summary
	fmt.Println("\n════════════════════════════════════════════════════════════")
	fmt.Println("All 4 Strategies Running")
	fmt.Println("════════════════════════════════════════════════════════════")
	fmt.Printf("1. Passive Strategy:      %s (%s)\n", passive.GetID(), passive.GetType())
	fmt.Printf("2. Aggressive Strategy:   %s (%s)\n", aggressive.GetID(), aggressive.GetType())
	fmt.Printf("3. Hedging Strategy:      %s (%s)\n", hedging.GetID(), hedging.GetType())
	fmt.Printf("4. Pairwise Arb Strategy: %s (%s)\n", pairs.GetID(), pairs.GetType())
	fmt.Println("════════════════════════════════════════════════════════════")

	// Simulate market data for all symbols
	go simulateMarketData(strategies["passive"], "ag2412", 7950.0)
	go simulateMarketData(strategies["aggressive"], "au2412", 550.0)
	go simulateMarketData(strategies["hedging"], "ag2412", 7950.0)
	go simulateMarketData(strategies["hedging"], "ag2501", 7980.0)
	go simulateMarketData(strategies["pairs"], "au2412", 550.0)
	go simulateMarketData(strategies["pairs"], "au2501", 552.0)

	// Periodic status printing
	statusTicker := time.NewTicker(5 * time.Second)
	defer statusTicker.Stop()

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("Press Ctrl+C to stop...")
	fmt.Println("────────────────────────────────────────────────────────────")

	running := true
	for running {
		select {
		case <-statusTicker.C:
			printAllStatus(strategies)
		case <-sigChan:
			running = false
		}
	}

	// Shutdown
	fmt.Println("\n\n[Main] Shutting down all strategies...")
	for name, s := range strategies {
		s.Stop()
		fmt.Printf("[Main] ✓ %s stopped\n", name)
	}

	// Final report
	fmt.Println("\n════════════════════════════════════════════════════════════")
	fmt.Println("Final Report")
	fmt.Println("════════════════════════════════════════════════════════════")
	printAllStatus(strategies)

	fmt.Println("\n✓ All strategies demo completed!")
}

// simulateMarketData simulates market data for a strategy
func simulateMarketData(s strategy.Strategy, symbol string, basePrice float64) {
	tickCount := 0

	for {
		if !s.IsRunning() {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		tickCount++

		// Simulate price movement
		priceMove := float64(tickCount%40-20) * (basePrice * 0.0002)
		bidPrice := basePrice + priceMove - (basePrice * 0.0001)
		askPrice := basePrice + priceMove + (basePrice * 0.0001)

		// Create synthetic market data
		md := &mdpb.MarketDataUpdate{
			Symbol:      symbol,
			Exchange:    "SHFE",
			Timestamp:   uint64(time.Now().UnixNano()),
			BidPrice:    []float64{bidPrice, bidPrice - 1, bidPrice - 2, bidPrice - 3, bidPrice - 4},
			BidQty:      []uint32{100, 80, 60, 50, 40},
			AskPrice:    []float64{askPrice, askPrice + 1, askPrice + 2, askPrice + 3, askPrice + 4},
			AskQty:      []uint32{95, 75, 55, 45, 35},
			LastPrice:   bidPrice + (basePrice * 0.0001),
			TotalVolume: uint64(1000 + tickCount*10),
			Turnover:    (bidPrice + (basePrice * 0.0001)) * float64(1000+tickCount*10),
		}

		// Send to strategy
		s.OnMarketData(md)

		// Sleep to simulate market data rate
		time.Sleep(100 * time.Millisecond)
	}
}

// printAllStatus prints status of all strategies
func printAllStatus(strategies map[string]strategy.Strategy) {
	fmt.Println("\n" + time.Now().Format("15:04:05") + " - Strategy Status Update")
	fmt.Println("────────────────────────────────────────────────────────────")

	for name, s := range strategies {
		status := s.GetStatus()
		position := s.GetPosition()
		pnl := s.GetPNL()
		signals := s.GetSignals()

		fmt.Printf("\n[%s] %s (%s)\n", name, s.GetID(), s.GetType())
		fmt.Printf("  Running:     %v\n", s.IsRunning())
		fmt.Printf("  Position:    %d (Long: %d, Short: %d)\n",
			position.NetQty, position.LongQty, position.ShortQty)
		fmt.Printf("  P&L:         %.2f (Realized: %.2f, Unrealized: %.2f)\n",
			pnl.TotalPnL, pnl.RealizedPnL, pnl.UnrealizedPnL)
		fmt.Printf("  Signals:     %d pending\n", len(signals))
		fmt.Printf("  Orders:      %d total, %d fills\n",
			status.OrderCount, status.FillCount)
	}

	fmt.Println()
}
