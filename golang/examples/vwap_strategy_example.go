package main

import (
	"fmt"
	"log"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
)

// Example demonstrates how to use the VWAP execution strategy

func main() {
	fmt.Println("=== VWAP执行策略演示 ===\n")

	// 运行三个示例场景
	example1_TimeWeighted()
	fmt.Println()

	example2_VolumeWeighted()
	fmt.Println()

	example3_TargetVWAP()
}

// Example 1: 时间加权VWAP执行
func example1_TimeWeighted() {
	fmt.Println("【示例1】时间加权VWAP执行")
	fmt.Println("场景: 1小时内执行10000手ag2502多单，使用20个切片均匀分布")
	fmt.Println()

	// 1. 创建策略
	startTime := time.Now()
	endTime := startTime.Add(1 * time.Hour)

	vwapStrategy := strategy.NewVWAPStrategy(
		"ag2502",   // 合约
		10000,      // 总手数
		"buy",      // 买入
		startTime,  // 开始时间
		endTime,    // 结束时间
	)

	// 2. 配置参数
	vwapStrategy.SetNumSlices(20)                      // 20个切片
	vwapStrategy.SetTargetVWAP(6800.0)                 // 目标VWAP
	vwapStrategy.SetCheckInterval(100 * time.Millisecond)  // 检查间隔

	// 3. 设置执行回调（模拟下单）
	executedCount := 0
	vwapStrategy.SetSliceExecutionCallback(func(slice *strategy.OrderSlice, price float64) error {
		executedCount++

		// 模拟成交价格（随机波动）
		executionPrice := 6800.0 + float64(executedCount%5) - 2.0

		// 记录成交
		vwapStrategy.RecordTrade(executionPrice, slice.Quantity, time.Now())

		log.Printf("  [Slice %d] 执行 %d手 @ %.2f", executedCount, slice.Quantity, executionPrice)

		return nil
	})

	// 4. 初始化
	err := vwapStrategy.Initialize()
	if err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	fmt.Printf("✓ 策略初始化完成，共 %d 个切片\n", vwapStrategy.GetStatistics().TotalSlices)

	// 5. 启动执行（异步）
	err = vwapStrategy.Start()
	if err != nil {
		log.Fatalf("启动失败: %v", err)
	}

	fmt.Println("✓ 策略已启动\n")

	// 6. 模拟执行（实际场景中会等待真实执行完成）
	time.Sleep(3 * time.Second)

	// 7. 停止策略
	vwapStrategy.Stop()

	// 8. 获取最终统计
	stats := vwapStrategy.GetStatistics()
	fmt.Println("\n执行结果:")
	fmt.Printf("  总手数: %d\n", stats.TotalQuantity)
	fmt.Printf("  已执行: %d手\n", stats.ExecutedQuantity)
	fmt.Printf("  执行进度: %.2f%%\n", stats.ExecutionProgress*100)
	fmt.Printf("  执行VWAP: %.2f\n", stats.ExecutedVWAP)
	fmt.Printf("  目标VWAP: %.2f\n", stats.TargetVWAP)
	fmt.Printf("  偏离度: %.2f (%.4f%%)\n", stats.VWAPDeviation, stats.VWAPDeviationPct)
	fmt.Printf("  已执行切片: %d/%d\n", stats.ExecutedSlices, stats.TotalSlices)
	fmt.Printf("  失败切片: %d\n", stats.FailedSlices)
}

// Example 2: 成交量加权VWAP执行
func example2_VolumeWeighted() {
	fmt.Println("【示例2】成交量加权VWAP执行")
	fmt.Println("场景: 根据历史成交量分布执行订单")
	fmt.Println()

	// 历史成交量分布（模拟U型分布：开盘高、中间低、收盘高）
	volumeProfile := []float64{
		0.20, // 09:30-09:45: 开盘高峰 20%
		0.15, // 09:45-10:00: 15%
		0.10, // 10:00-10:15: 10%
		0.10, // 10:15-10:30: 10%
		0.10, // 10:30-10:45: 10%
		0.10, // 10:45-11:00: 10%
		0.25, // 11:00-11:15: 收盘高峰 25%
	}

	fmt.Println("成交量分布:")
	for i, pct := range volumeProfile {
		fmt.Printf("  时段%d: %.0f%%\n", i+1, pct*100)
	}
	fmt.Println()

	// 1. 创建策略
	startTime := time.Now()
	endTime := startTime.Add(1 * time.Hour)

	vwapStrategy := strategy.NewVWAPStrategy("ag2502", 10000, "buy", startTime, endTime)

	// 2. 设置成交量分布
	err := vwapStrategy.SetVolumeProfile(volumeProfile)
	if err != nil {
		log.Fatalf("设置成交量分布失败: %v", err)
	}

	// 3. 设置执行回调
	executedCount := 0
	vwapStrategy.SetSliceExecutionCallback(func(slice *strategy.OrderSlice, price float64) error {
		executedCount++
		executionPrice := 6805.0 + float64(executedCount%7) - 3.0
		vwapStrategy.RecordTrade(executionPrice, slice.Quantity, time.Now())
		log.Printf("  [Slice %d] 执行 %d手 @ %.2f", executedCount, slice.Quantity, executionPrice)
		return nil
	})

	// 4. 初始化并启动
	if err := vwapStrategy.Initialize(); err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	fmt.Printf("✓ 策略初始化完成，共 %d 个切片（按成交量分布）\n", vwapStrategy.GetStatistics().TotalSlices)

	if err := vwapStrategy.Start(); err != nil {
		log.Fatalf("启动失败: %v", err)
	}

	fmt.Println("✓ 策略已启动\n")

	// 模拟执行
	time.Sleep(2 * time.Second)
	vwapStrategy.Stop()

	// 获取结果
	stats := vwapStrategy.GetStatistics()
	fmt.Println("\n执行结果:")
	fmt.Printf("  已执行: %d/%d手 (%.2f%%)\n",
		stats.ExecutedQuantity, stats.TotalQuantity, stats.ExecutionProgress*100)
	fmt.Printf("  执行VWAP: %.2f\n", stats.ExecutedVWAP)
	fmt.Printf("  平均切片大小: %.0f手\n", stats.AverageSliceSize)
}

// Example 3: 监控目标VWAP偏离
func example3_TargetVWAP() {
	fmt.Println("【示例3】监控目标VWAP偏离")
	fmt.Println("场景: 设置目标VWAP为6800，实时监控偏离度")
	fmt.Println()

	// 1. 创建策略
	startTime := time.Now()
	endTime := startTime.Add(30 * time.Minute)  // 30分钟执行

	vwapStrategy := strategy.NewVWAPStrategy("ag2502", 5000, "buy", startTime, endTime)

	// 2. 配置
	vwapStrategy.SetNumSlices(10)
	vwapStrategy.SetTargetVWAP(6800.0)  // 设置目标

	// 3. 设置执行回调（模拟价格波动）
	executedCount := 0
	vwapStrategy.SetSliceExecutionCallback(func(slice *strategy.OrderSlice, price float64) error {
		executedCount++

		// 模拟价格逐渐偏离目标
		executionPrice := 6800.0 + float64(executedCount)*0.3

		vwapStrategy.RecordTrade(executionPrice, slice.Quantity, time.Now())

		// 获取当前统计
		stats := vwapStrategy.GetStatistics()

		log.Printf("  [Slice %d] 执行 %d手 @ %.2f | 当前VWAP: %.2f | 偏离: %.4f%%",
			executedCount, slice.Quantity, executionPrice,
			stats.ExecutedVWAP, stats.VWAPDeviationPct)

		// 检查偏离度警告
		if stats.VWAPDeviationPct > 0.05 {
			log.Printf("    ⚠️  偏离度超过0.05%%，当前: %.4f%%", stats.VWAPDeviationPct)
		}

		return nil
	})

	// 4. 初始化并启动
	if err := vwapStrategy.Initialize(); err != nil {
		log.Fatalf("初始化失败: %v", err)
	}

	fmt.Printf("✓ 策略初始化完成\n")
	fmt.Printf("  目标VWAP: %.2f\n", 6800.0)
	fmt.Printf("  切片数: %d\n\n", vwapStrategy.GetStatistics().TotalSlices)

	if err := vwapStrategy.Start(); err != nil {
		log.Fatalf("启动失败: %v", err)
	}

	// 监控执行
	monitorDone := make(chan bool)
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if vwapStrategy.GetStatus() != strategy.VWAPStatusRunning {
					monitorDone <- true
					return
				}

				stats := vwapStrategy.GetStatistics()
				if stats.ExecutionProgress > 0 {
					fmt.Printf("  [监控] 进度: %.0f%% | VWAP: %.2f | 偏离: %.4f%%\n",
						stats.ExecutionProgress*100,
						stats.ExecutedVWAP,
						stats.VWAPDeviationPct)
				}
			}
		}
	}()

	// 等待执行完成
	time.Sleep(3 * time.Second)
	vwapStrategy.Stop()
	<-monitorDone

	// 最终结果
	stats := vwapStrategy.GetStatistics()
	fmt.Println("\n最终结果:")
	fmt.Printf("  执行VWAP: %.2f\n", stats.ExecutedVWAP)
	fmt.Printf("  目标VWAP: %.2f\n", stats.TargetVWAP)
	fmt.Printf("  绝对偏离: %.2f\n", stats.VWAPDeviation)
	fmt.Printf("  百分比偏离: %.4f%%\n", stats.VWAPDeviationPct)

	if stats.VWAPDeviationPct < 0.1 {
		fmt.Println("  ✓ 偏离度在可接受范围内 (<0.1%)")
	} else {
		fmt.Println("  ⚠️ 偏离度较大，可能需要调整执行策略")
	}
}
