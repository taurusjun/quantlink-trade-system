package main

import (
	"log"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
)

func main() {
	// Example 1: Low-latency mode (like tbsrc)
	// 低延迟模式（类似tbsrc）- 行情到达后立即同步发单
	syncConfig := &strategy.EngineConfig{
		ORSGatewayAddr: "localhost:50052",
		NATSAddr:       "nats://localhost:4222",
		OrderQueueSize: 1000,
		TimerInterval:  100 * time.Millisecond,
		OrderMode:      strategy.OrderModeSync,  // ← 同步模式
		OrderTimeout:   50 * time.Millisecond,   // 50ms超时
	}

	syncEngine := strategy.NewStrategyEngine(syncConfig)
	log.Println("Created low-latency engine (Sync mode)")

	// Example 2: High-throughput mode (original behavior)
	// 高吞吐模式（原始行为）- 通过队列异步发单
	asyncConfig := &strategy.EngineConfig{
		ORSGatewayAddr:      "localhost:50052",
		NATSAddr:            "nats://localhost:4222",
		OrderQueueSize:      10000,             // 更大的队列
		TimerInterval:       100 * time.Millisecond,
		OrderMode:           strategy.OrderModeAsync, // ← 异步模式
		MaxConcurrentOrders: 100,
	}

	asyncEngine := strategy.NewStrategyEngine(asyncConfig)
	log.Println("Created high-throughput engine (Async mode)")

	// Performance comparison:
	// 性能对比：
	//
	// Sync Mode (OrderModeSync):
	//   - Latency: ~10-50μs (行情到发单)
	//   - Throughput: Medium (受限于同步处理)
	//   - Use case: 超低延迟策略（套利、做市）
	//
	// Async Mode (OrderModeAsync):
	//   - Latency: ~50-200μs (行情到发单)
	//   - Throughput: High (队列+goroutine并发)
	//   - Use case: 高频策略、多策略并行

	_ = syncEngine
	_ = asyncEngine
}
