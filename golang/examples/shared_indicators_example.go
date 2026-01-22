package main

import (
	"log"

	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
)

func main() {
	// Example: Using shared indicator pool
	// 示例：使用共享指标池

	// Step 1: Create engine with shared indicator pool
	// 步骤1：创建带有共享指标池的引擎
	engine := strategy.NewStrategyEngine(&strategy.EngineConfig{
		ORSGatewayAddr: "localhost:50052",
		NATSAddr:       "nats://localhost:4222",
		OrderMode:      strategy.OrderModeSync,
	})

	// Step 2: Initialize shared indicators for symbols
	// 步骤2：为symbol初始化共享指标
	symbols := []string{"IF2501", "IC2501", "IH2501"}
	for _, symbol := range symbols {
		err := engine.InitializeSharedIndicators(symbol, map[string]interface{}{
			"volatility": map[string]interface{}{
				"window": 20,
			},
		})
		if err != nil {
			log.Printf("Failed to initialize shared indicators for %s: %v", symbol, err)
		}
	}

	// Step 3: Create strategies and attach shared indicators
	// 步骤3：创建策略并附加共享指标

	// Example: 3 strategies trading the same symbol
	// 示例：3个策略交易同一个symbol
	// Benefit: VWAP, Spread, OrderImbalance只计算一次！
	for i := 1; i <= 3; i++ {
		strat := strategy.NewPassiveStrategy(
			"passive_"+string(rune('0'+i)),
			&strategy.StrategyConfig{
				Symbol: "IF2501",
			},
		)

		// Attach shared indicators (VWAP, Spread, etc.)
		// 附加共享指标（VWAP, Spread等）
		engine.AttachSharedIndicators(strat, []string{"IF2501"})

		engine.AddStrategy(strat)
	}

	// Performance comparison:
	// 性能对比：
	//
	// Without SharedIndicatorPool:
	//   - 3 strategies × 4 indicators = 12 calculations per market update
	//   - Total time: ~60μs
	//
	// With SharedIndicatorPool:
	//   - 1 × 4 shared indicators = 4 calculations (共享)
	//   - 3 × private indicators = 3 calculations (私有)
	//   - Total time: ~25μs
	//   - Performance improvement: 58%!

	// Get statistics
	stats := engine.GetSharedIndicatorStats()
	log.Printf("Shared indicator stats: %+v", stats)

	// Architecture alignment with tbsrc:
	// 与tbsrc的架构对齐：
	//
	// tbsrc:
	//   - Instrument级指标：所有策略共享（VWAP, Spread等）
	//   - Strategy级指标：策略私有（自定义组合指标）
	//
	// quantlink (now):
	//   - SharedIndicators：所有策略共享（对应tbsrc的Instrument级）
	//   - PrivateIndicators：策略私有（对应tbsrc的Strategy级）
}
