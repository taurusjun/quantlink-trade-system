package main

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
)

func main() {
	fmt.Println("==========================================")
	fmt.Println("VWAP 执行策略演示程序")
	fmt.Println("VWAP Execution Strategy Demo")
	fmt.Println("==========================================")
	fmt.Println()

	// Scenario 1: Time-weighted VWAP execution
	fmt.Println("========== 场景 1: 时间加权 VWAP 执行 ==========")
	fmt.Println()
	runTimeWeightedVWAP()
	fmt.Println()

	time.Sleep(500 * time.Millisecond)

	// Scenario 2: Volume-weighted VWAP execution
	fmt.Println("========== 场景 2: 成交量加权 VWAP 执行 ==========")
	fmt.Println()
	runVolumeWeightedVWAP()
	fmt.Println()

	time.Sleep(500 * time.Millisecond)

	// Scenario 3: VWAP with target tracking
	fmt.Println("========== 场景 3: 目标 VWAP 偏离跟踪 ==========")
	fmt.Println()
	runVWAPWithTargetTracking()
	fmt.Println()

	fmt.Println("==========================================")
	fmt.Println("演示完成")
	fmt.Println("==========================================")
}

// runTimeWeightedVWAP demonstrates time-weighted VWAP execution
func runTimeWeightedVWAP() {
	symbol := "AG2502"
	totalQuantity := int64(10000)
	side := "buy"
	numSlices := 10

	// Execute over 5 seconds (use past time for immediate execution)
	startTime := time.Now().Add(-1 * time.Second)
	endTime := startTime.Add(5 * time.Second)

	fmt.Printf("交易品种: %s\n", symbol)
	fmt.Printf("总数量: %d\n", totalQuantity)
	fmt.Printf("方向: %s\n", side)
	fmt.Printf("分片数量: %d\n", numSlices)
	fmt.Printf("执行时长: %.0f 秒\n", endTime.Sub(startTime).Seconds())
	fmt.Println()

	// Create strategy
	vwapStrategy := strategy.NewVWAPStrategy(symbol, totalQuantity, side, startTime, endTime)
	vwapStrategy.SetNumSlices(numSlices)
	vwapStrategy.SetCheckInterval(100 * time.Millisecond)

	// Initialize
	err := vwapStrategy.Initialize()
	if err != nil {
		log.Fatalf("初始化策略失败: %v", err)
	}

	// Set execution callback
	executionCount := 0
	vwapStrategy.SetSliceExecutionCallback(func(slice *strategy.OrderSlice, price float64) error {
		executionCount++

		// Simulate market execution with random price movement
		basePrice := 6800.0
		priceVariation := (rand.Float64() - 0.5) * 10.0
		executionPrice := basePrice + priceVariation

		// Record the trade
		vwapStrategy.RecordTrade(executionPrice, slice.Quantity, time.Now())

		fmt.Printf("  [%d/%d] 执行切片 #%d: 数量=%d, 价格=%.2f, 时间=%s\n",
			executionCount, numSlices,
			slice.SliceID,
			slice.Quantity,
			executionPrice,
			slice.ScheduledTime.Format("15:04:05"))

		return nil
	})

	// Start strategy
	err = vwapStrategy.Start()
	if err != nil {
		log.Fatalf("启动策略失败: %v", err)
	}

	// Monitor progress
	fmt.Println("开始执行...")
	fmt.Println()

	// Wait for completion
	for {
		time.Sleep(200 * time.Millisecond)

		status := vwapStrategy.GetStatus()
		if status == strategy.VWAPStatusCompleted || status == strategy.VWAPStatusCanceled {
			break
		}

		progress := vwapStrategy.GetProgress()
		if progress > 0 && progress < 1.0 {
			fmt.Printf("  进度: %.0f%%\n", progress*100)
		}
	}

	// Display results
	stats := vwapStrategy.GetStatistics()
	fmt.Println()
	fmt.Println("执行结果:")
	fmt.Printf("  执行数量: %d / %d\n", stats.ExecutedQuantity, stats.TotalQuantity)
	fmt.Printf("  执行 VWAP: %.2f\n", stats.ExecutedVWAP)
	fmt.Printf("  执行切片: %d / %d\n", stats.ExecutedSlices, stats.TotalSlices)
	fmt.Printf("  失败切片: %d\n", stats.FailedSlices)
	fmt.Printf("  平均切片大小: %.0f\n", stats.AverageSliceSize)
	fmt.Printf("  执行进度: %.0f%%\n", stats.ExecutionProgress*100)
	fmt.Printf("  用时: %.2f 秒\n", stats.ElapsedTime.Seconds())
}

// runVolumeWeightedVWAP demonstrates volume-weighted VWAP execution
func runVolumeWeightedVWAP() {
	symbol := "AG2504"
	totalQuantity := int64(20000)
	side := "sell"

	// Execute over 4 seconds
	startTime := time.Now().Add(-1 * time.Second)
	endTime := startTime.Add(4 * time.Second)

	// Volume profile: higher volume at market open and close
	volumeProfile := []float64{0.3, 0.2, 0.2, 0.3} // Sums to 1.0

	fmt.Printf("交易品种: %s\n", symbol)
	fmt.Printf("总数量: %d\n", totalQuantity)
	fmt.Printf("方向: %s\n", side)
	fmt.Printf("成交量分布: %v\n", volumeProfile)
	fmt.Printf("执行时长: %.0f 秒\n", endTime.Sub(startTime).Seconds())
	fmt.Println()

	// Create strategy
	vwapStrategy := strategy.NewVWAPStrategy(symbol, totalQuantity, side, startTime, endTime)
	err := vwapStrategy.SetVolumeProfile(volumeProfile)
	if err != nil {
		log.Fatalf("设置成交量分布失败: %v", err)
	}
	vwapStrategy.SetCheckInterval(100 * time.Millisecond)

	// Initialize
	err = vwapStrategy.Initialize()
	if err != nil {
		log.Fatalf("初始化策略失败: %v", err)
	}

	// Set execution callback
	executionCount := 0
	vwapStrategy.SetSliceExecutionCallback(func(slice *strategy.OrderSlice, price float64) error {
		executionCount++

		// Simulate market execution
		basePrice := 6850.0
		priceVariation := (rand.Float64() - 0.5) * 15.0
		executionPrice := basePrice + priceVariation

		// Record the trade
		vwapStrategy.RecordTrade(executionPrice, slice.Quantity, time.Now())

		fmt.Printf("  [%d/%d] 执行切片 #%d: 数量=%d (%.0f%%), 价格=%.2f\n",
			executionCount, len(volumeProfile),
			slice.SliceID,
			slice.Quantity,
			float64(slice.Quantity)/float64(totalQuantity)*100,
			executionPrice)

		return nil
	})

	// Start strategy
	err = vwapStrategy.Start()
	if err != nil {
		log.Fatalf("启动策略失败: %v", err)
	}

	fmt.Println("开始执行...")
	fmt.Println()

	// Wait for completion
	for {
		time.Sleep(200 * time.Millisecond)

		status := vwapStrategy.GetStatus()
		if status == strategy.VWAPStatusCompleted || status == strategy.VWAPStatusCanceled {
			break
		}
	}

	// Display results
	stats := vwapStrategy.GetStatistics()
	fmt.Println()
	fmt.Println("执行结果:")
	fmt.Printf("  执行数量: %d / %d\n", stats.ExecutedQuantity, stats.TotalQuantity)
	fmt.Printf("  执行 VWAP: %.2f\n", stats.ExecutedVWAP)
	fmt.Printf("  执行切片: %d / %d\n", stats.ExecutedSlices, stats.TotalSlices)
	fmt.Printf("  用时: %.2f 秒\n", stats.ElapsedTime.Seconds())
}

// runVWAPWithTargetTracking demonstrates VWAP execution with target tracking
func runVWAPWithTargetTracking() {
	symbol := "AG2506"
	totalQuantity := int64(15000)
	side := "buy"
	numSlices := 8
	targetVWAP := 6800.0

	// Execute over 4 seconds
	startTime := time.Now().Add(-1 * time.Second)
	endTime := startTime.Add(4 * time.Second)

	fmt.Printf("交易品种: %s\n", symbol)
	fmt.Printf("总数量: %d\n", totalQuantity)
	fmt.Printf("方向: %s\n", side)
	fmt.Printf("分片数量: %d\n", numSlices)
	fmt.Printf("目标 VWAP: %.2f\n", targetVWAP)
	fmt.Printf("执行时长: %.0f 秒\n", endTime.Sub(startTime).Seconds())
	fmt.Println()

	// Create strategy
	vwapStrategy := strategy.NewVWAPStrategy(symbol, totalQuantity, side, startTime, endTime)
	vwapStrategy.SetNumSlices(numSlices)
	vwapStrategy.SetTargetVWAP(targetVWAP)
	vwapStrategy.SetCheckInterval(100 * time.Millisecond)

	// Initialize
	err := vwapStrategy.Initialize()
	if err != nil {
		log.Fatalf("初始化策略失败: %v", err)
	}

	// Set execution callback with price drift simulation
	executionCount := 0
	vwapStrategy.SetSliceExecutionCallback(func(slice *strategy.OrderSlice, price float64) error {
		executionCount++

		// Simulate upward price drift (unfavorable for buying)
		basePrice := 6795.0 + float64(executionCount)*1.5 // Price increases over time
		priceVariation := (rand.Float64() - 0.5) * 5.0
		executionPrice := basePrice + priceVariation

		// Record the trade
		vwapStrategy.RecordTrade(executionPrice, slice.Quantity, time.Now())

		// Get current statistics
		currentStats := vwapStrategy.GetStatistics()

		fmt.Printf("  [%d/%d] 执行切片 #%d: 数量=%d, 价格=%.2f\n",
			executionCount, numSlices,
			slice.SliceID,
			slice.Quantity,
			executionPrice)

		if currentStats.ExecutedQuantity > 0 {
			fmt.Printf("         当前 VWAP=%.2f, 偏离=%.2f (%.2f%%)\n",
				currentStats.ExecutedVWAP,
				currentStats.VWAPDeviation,
				currentStats.VWAPDeviationPct)
		}

		return nil
	})

	// Start strategy
	err = vwapStrategy.Start()
	if err != nil {
		log.Fatalf("启动策略失败: %v", err)
	}

	fmt.Println("开始执行...")
	fmt.Println()

	// Wait for completion
	for {
		time.Sleep(200 * time.Millisecond)

		status := vwapStrategy.GetStatus()
		if status == strategy.VWAPStatusCompleted || status == strategy.VWAPStatusCanceled {
			break
		}
	}

	// Display results
	stats := vwapStrategy.GetStatistics()
	fmt.Println()
	fmt.Println("执行结果:")
	fmt.Printf("  执行数量: %d / %d\n", stats.ExecutedQuantity, stats.TotalQuantity)
	fmt.Printf("  目标 VWAP: %.2f\n", stats.TargetVWAP)
	fmt.Printf("  执行 VWAP: %.2f\n", stats.ExecutedVWAP)
	fmt.Printf("  绝对偏离: %.2f\n", stats.VWAPDeviation)
	fmt.Printf("  百分比偏离: %.4f%%\n", stats.VWAPDeviationPct)

	if stats.VWAPDeviation > 0 {
		fmt.Printf("  ⚠️ 执行价格高于目标 (买入成本增加)\n")
	} else {
		fmt.Printf("  ✓ 执行价格低于目标 (买入成本降低)\n")
	}

	fmt.Printf("  执行切片: %d / %d\n", stats.ExecutedSlices, stats.TotalSlices)
	fmt.Printf("  用时: %.2f 秒\n", stats.ElapsedTime.Seconds())
}
