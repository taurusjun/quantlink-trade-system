package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/portfolio"
	"github.com/yourusername/quantlink-trade-system/pkg/risk"
	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

func main() {
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║         HFT Integrated System Demo                       ║")
	fmt.Println("║    Strategy Engine + Portfolio + Risk Management         ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
	fmt.Println()

	// 1. Create Risk Manager
	fmt.Println("[Main] Creating Risk Manager...")
	riskConfig := &risk.RiskManagerConfig{
		EnableGlobalLimits:     true,
		EnableStrategyLimits:   true,
		EnablePortfolioLimits:  true,
		AlertRetentionSeconds:  3600,
		MaxAlertQueueSize:      1000,
		EmergencyStopThreshold: 3,
		CheckIntervalMs:        100,
	}
	riskMgr := risk.NewRiskManager(riskConfig)
	if err := riskMgr.Initialize(); err != nil {
		log.Fatalf("Failed to initialize risk manager: %v", err)
	}
	if err := riskMgr.Start(); err != nil {
		log.Fatalf("Failed to start risk manager: %v", err)
	}
	fmt.Println("[Main] ✓ Risk Manager started")

	// 2. Create Portfolio Manager
	fmt.Println("\n[Main] Creating Portfolio Manager...")
	portfolioConfig := &portfolio.PortfolioConfig{
		TotalCapital:          1000000.0, // 100万
		StrategyAllocation:    make(map[string]float64),
		RebalanceIntervalSec:  3600, // 1 hour
		MinAllocation:         0.05,
		MaxAllocation:         0.50,
		EnableAutoRebalance:   false, // Disable for demo
		EnableCorrelationCalc: false,
	}
	portfolioMgr := portfolio.NewPortfolioManager(portfolioConfig)
	if err := portfolioMgr.Initialize(); err != nil {
		log.Fatalf("Failed to initialize portfolio manager: %v", err)
	}
	if err := portfolioMgr.Start(); err != nil {
		log.Fatalf("Failed to start portfolio manager: %v", err)
	}
	fmt.Println("[Main] ✓ Portfolio Manager started")

	// 3. Create Strategy Engine
	fmt.Println("\n[Main] Creating Strategy Engine...")
	engineConfig := &strategy.EngineConfig{
		ORSGatewayAddr:      "localhost:50052",
		NATSAddr:            "nats://localhost:4222",
		OrderQueueSize:      100,
		TimerInterval:       5 * time.Second,
		MaxConcurrentOrders: 10,
	}
	engine := strategy.NewStrategyEngine(engineConfig)

	// Try to initialize (may fail if services not running, that's OK for demo)
	if err := engine.Initialize(); err != nil {
		log.Printf("[Main] Warning: %v", err)
		log.Println("[Main] Continuing in demo mode...")
	}

	// 4. Create multiple strategies
	fmt.Println("\n[Main] Creating strategies...")

	// Strategy 1: Passive Market Making
	passive1 := strategy.NewPassiveStrategy("passive_ag")
	passive1Config := &strategy.StrategyConfig{
		StrategyID:      "passive_ag",
		StrategyType:    "passive",
		Symbols:         []string{"ag2412"},
		Exchanges:       []string{"SHFE"},
		MaxPositionSize: 100,
		MaxExposure:     500000.0,
		RiskLimits: map[string]float64{
			"max_drawdown": 5000.0,
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
	if err := passive1.Initialize(passive1Config); err != nil {
		log.Fatalf("Failed to initialize passive_ag: %v", err)
	}

	// Strategy 2: Another Passive Strategy
	passive2 := strategy.NewPassiveStrategy("passive_au")
	passive2Config := &strategy.StrategyConfig{
		StrategyID:      "passive_au",
		StrategyType:    "passive",
		Symbols:         []string{"au2412"},
		Exchanges:       []string{"SHFE"},
		MaxPositionSize: 50,
		MaxExposure:     300000.0,
		RiskLimits: map[string]float64{
			"max_drawdown": 3000.0,
		},
		Parameters: map[string]interface{}{
			"spread_multiplier":   0.4,
			"order_size":          5.0,
			"max_inventory":       50.0,
			"inventory_skew":      0.4,
			"min_spread":          0.5,
			"order_refresh_ms":    800.0,
			"use_order_imbalance": true,
		},
		Enabled: true,
	}
	if err := passive2.Initialize(passive2Config); err != nil {
		log.Fatalf("Failed to initialize passive_au: %v", err)
	}

	// Add strategies to engine
	if err := engine.AddStrategy(passive1); err != nil {
		log.Fatalf("Failed to add passive_ag: %v", err)
	}
	if err := engine.AddStrategy(passive2); err != nil {
		log.Fatalf("Failed to add passive_au: %v", err)
	}

	fmt.Println("[Main] ✓ Created 2 strategies")

	// 5. Add strategies to portfolio
	fmt.Println("\n[Main] Adding strategies to portfolio...")
	if err := portfolioMgr.AddStrategy(passive1, 0.50); err != nil { // 50% allocation
		log.Fatalf("Failed to add passive_ag to portfolio: %v", err)
	}
	if err := portfolioMgr.AddStrategy(passive2, 0.30); err != nil { // 30% allocation
		log.Fatalf("Failed to add passive_au to portfolio: %v", err)
	}
	fmt.Println("[Main] ✓ Strategies added to portfolio")

	// 6. Start all strategies
	fmt.Println("\n[Main] Starting strategies...")
	passive1.Start()
	passive2.Start()
	fmt.Println("[Main] ✓ Strategies started")

	// Start engine
	if err := engine.Start(); err != nil {
		log.Printf("[Main] Warning: %v", err)
	}

	// 7. Print initial state
	fmt.Println("\n════════════════════════════════════════════════════════════")
	fmt.Println("System Started - Running Simulation")
	fmt.Println("════════════════════════════════════════════════════════════")

	portfolioMgr.PrintReport()

	// 8. Run simulation
	go simulateMarketData(passive1, "ag2412", 7950.0)
	go simulateMarketData(passive2, "au2412", 550.0)

	// 9. Periodic monitoring
	monitorTicker := time.NewTicker(10 * time.Second)
	defer monitorTicker.Stop()

	riskCheckTicker := time.NewTicker(1 * time.Second)
	defer riskCheckTicker.Stop()

	// Wait for interrupt
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	fmt.Println("\nPress Ctrl+C to stop...")
	fmt.Println("────────────────────────────────────────────────────────────")

	running := true
	for running {
		select {
		case <-monitorTicker.C:
			fmt.Println("\n" + time.Now().Format("15:04:05") + " - Periodic Update")
			fmt.Println("────────────────────────────────────────────────────────────")

			// Update portfolio
			portfolioMgr.UpdateAllocations()

			// Print portfolio report
			portfolioMgr.PrintReport()

			// Print risk alerts
			alerts := riskMgr.GetAlerts("", 5)
			if len(alerts) > 0 {
				fmt.Println("\n⚠️  Recent Risk Alerts:")
				for _, alert := range alerts {
					fmt.Printf("  [%s] %s: %s\n",
						alert.Level, alert.TargetID, alert.Message)
				}
			}

			// Print global risk stats
			stats := riskMgr.GetGlobalStats()
			fmt.Println("\n🛡️  Risk Statistics:")
			fmt.Printf("  Total Exposure: %.2f\n", stats["total_exposure"])
			fmt.Printf("  Total P&L: %.2f\n", stats["total_pnl"])
			fmt.Printf("  Emergency Stop: %v\n", stats["emergency_stop"])
			fmt.Printf("  Critical Alerts: %d\n", stats["critical_alerts"])

		case <-riskCheckTicker.C:
			// Perform risk checks
			strategies := map[string]strategy.Strategy{
				"passive_ag": passive1,
				"passive_au": passive2,
			}

			// Check individual strategies
			for id, s := range strategies {
				if !s.IsRunning() {
					continue
				}

				strategyAlerts := riskMgr.CheckStrategy(s)
				for _, alert := range strategyAlerts {
					riskMgr.AddAlert(&alert)

					// Take action based on alert
					if alert.Action == "stop" {
						log.Printf("[Main] Stopping strategy %s due to risk alert", id)
						s.Stop()
					}
				}
			}

			// Check global limits
			globalAlerts := riskMgr.CheckGlobal(strategies)
			for _, alert := range globalAlerts {
				riskMgr.AddAlert(&alert)

				if alert.Action == "emergency_stop" && !riskMgr.IsEmergencyStop() {
					log.Println("[Main] EMERGENCY STOP triggered by global risk limits!")
					// In production: stop all strategies, cancel all orders, flatten positions
				}
			}

		case <-sigChan:
			running = false
		}
	}

	// 10. Shutdown
	fmt.Println("\n\n[Main] Shutting down system...")

	passive1.Stop()
	passive2.Stop()
	engine.Stop()
	portfolioMgr.Stop()
	riskMgr.Stop()

	// Final report
	fmt.Println("\n════════════════════════════════════════════════════════════")
	fmt.Println("Final Report")
	fmt.Println("════════════════════════════════════════════════════════════")

	portfolioMgr.UpdateAllocations()
	portfolioMgr.PrintReport()

	fmt.Println("\n✓ Integrated system demo completed!")
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
		priceMove := float64(tickCount%20-10) * (basePrice * 0.0002)
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
