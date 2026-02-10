// Package main provides a test program for shared memory functionality
package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/shm"
)

const (
	TestTVarKey   = 99901 // 测试用 TVar 键
	TestTCacheKey = 99902 // 测试用 TCache 键
)

func main() {
	if len(os.Args) < 2 {
		printUsage()
		return
	}

	switch os.Args[1] {
	case "tvar-write":
		testTVarWrite()
	case "tvar-read":
		testTVarRead()
	case "tcache-write":
		testTCacheWrite()
	case "tcache-read":
		testTCacheRead()
	case "demo":
		runDemo()
	default:
		printUsage()
	}
}

func printUsage() {
	fmt.Println(`共享内存测试程序

用法:
  shm_test <command>

命令:
  tvar-write   - 写入 TVar（模拟外部程序写入 tValue）
  tvar-read    - 读取 TVar（模拟策略读取 tValue）
  tcache-write - 写入 TCache（模拟策略写入持仓）
  tcache-read  - 读取 TCache（模拟外部程序读取持仓）
  demo         - 运行完整演示

测试步骤:
  1. 终端1: ./shm_test tvar-write    # 持续写入 tValue
  2. 终端2: ./shm_test tvar-read     # 持续读取 tValue

  或运行完整演示:
  ./shm_test demo`)
}

// testTVarWrite 模拟外部程序（如 Python）写入 tValue
func testTVarWrite() {
	log.Printf("=== TVar 写入测试 (key=%d) ===", TestTVarKey)

	tvar, err := shm.NewTVar(TestTVarKey)
	if err != nil {
		log.Fatalf("创建 TVar 失败: %v", err)
	}
	defer tvar.Close()

	log.Printf("TVar 创建成功，开始写入...")
	log.Printf("按 Ctrl+C 停止")

	// 模拟外部程序持续写入不同的值
	values := []float64{0.001, 0.002, -0.001, 0.0015, -0.0005, 0}
	i := 0
	for {
		value := values[i%len(values)]
		tvar.Store(value)
		log.Printf("写入 tValue = %.6f", value)
		time.Sleep(2 * time.Second)
		i++
	}
}

// testTVarRead 模拟策略读取 tValue
func testTVarRead() {
	log.Printf("=== TVar 读取测试 (key=%d) ===", TestTVarKey)

	tvar, err := shm.NewTVar(TestTVarKey)
	if err != nil {
		log.Fatalf("创建 TVar 失败: %v", err)
	}
	defer tvar.Close()

	log.Printf("TVar 连接成功，开始读取...")
	log.Printf("按 Ctrl+C 停止")

	lastValue := 0.0
	for {
		value := tvar.Load()
		if value != lastValue {
			log.Printf("读取 tValue = %.6f (变化: %.6f)", value, value-lastValue)
			lastValue = value
		} else {
			log.Printf("读取 tValue = %.6f (无变化)", value)
		}
		time.Sleep(1 * time.Second)
	}
}

// testTCacheWrite 模拟策略写入持仓数据
func testTCacheWrite() {
	log.Printf("=== TCache 写入测试 (key=%d) ===", TestTCacheKey)

	tcache, err := shm.NewTCache(TestTCacheKey, 100)
	if err != nil {
		log.Fatalf("创建 TCache 失败: %v", err)
	}
	defer tcache.Close()

	log.Printf("TCache 创建成功，开始写入...")
	log.Printf("按 Ctrl+C 停止")

	leg1Pos := 0.0
	leg2Pos := 0.0
	for {
		// 模拟持仓变化
		leg1Pos += 10
		leg2Pos -= 10

		tcache.Store("strategy1_leg1_pos", leg1Pos)
		tcache.Store("strategy1_leg2_pos", leg2Pos)
		tcache.Store("strategy1_exposure", leg1Pos+leg2Pos)

		log.Printf("写入持仓: leg1=%.0f, leg2=%.0f, exposure=%.0f",
			leg1Pos, leg2Pos, leg1Pos+leg2Pos)

		time.Sleep(2 * time.Second)
	}
}

// testTCacheRead 模拟外部程序读取持仓数据
func testTCacheRead() {
	log.Printf("=== TCache 读取测试 (key=%d) ===", TestTCacheKey)

	tcache, err := shm.NewTCache(TestTCacheKey, 100)
	if err != nil {
		log.Fatalf("创建 TCache 失败: %v", err)
	}
	defer tcache.Close()

	log.Printf("TCache 连接成功，开始读取...")
	log.Printf("按 Ctrl+C 停止")

	for {
		leg1, err1 := tcache.Load("strategy1_leg1_pos")
		leg2, err2 := tcache.Load("strategy1_leg2_pos")
		exposure, err3 := tcache.Load("strategy1_exposure")

		if err1 == nil && err2 == nil && err3 == nil {
			log.Printf("读取持仓: leg1=%.0f, leg2=%.0f, exposure=%.0f",
				leg1, leg2, exposure)
		} else {
			log.Printf("等待数据... (leg1 err=%v)", err1)
		}

		// 打印完整缓存内容
		log.Printf("TCache 内容:\n%s", tcache.String())

		time.Sleep(1 * time.Second)
	}
}

// runDemo 运行完整演示
func runDemo() {
	log.Println("========================================")
	log.Println("   共享内存 (SHM) 功能演示")
	log.Println("========================================")
	log.Println()

	// 测试 TVar
	log.Println("--- 测试 1: TVar (单值共享) ---")
	testTVarDemo()

	log.Println()

	// 测试 TCache
	log.Println("--- 测试 2: TCache (键值对缓存) ---")
	testTCacheDemo()

	log.Println()
	log.Println("========================================")
	log.Println("   演示完成！")
	log.Println("========================================")
}

func testTVarDemo() {
	tvar, err := shm.NewTVar(TestTVarKey)
	if err != nil {
		log.Printf("创建 TVar 失败: %v", err)
		return
	}
	defer tvar.Close()

	log.Printf("TVar 创建成功 (key=%d)", TestTVarKey)

	// 写入测试
	testValues := []float64{0.0, 0.001, 0.002, -0.001, 0.0015}
	for _, val := range testValues {
		tvar.Store(val)
		readBack := tvar.Load()
		log.Printf("  写入: %.6f, 读取: %.6f, 匹配: %v",
			val, readBack, val == readBack)
		time.Sleep(100 * time.Millisecond)
	}

	log.Println("TVar 测试通过!")
}

func testTCacheDemo() {
	tcache, err := shm.NewTCache(TestTCacheKey, 100)
	if err != nil {
		log.Printf("创建 TCache 失败: %v", err)
		return
	}
	defer tcache.Close()

	log.Printf("TCache 创建成功 (key=%d)", TestTCacheKey)

	// 写入测试
	testData := map[string]float64{
		"leg1_pos":     10,
		"leg2_pos":     -10,
		"exposure":     0,
		"realized_pnl": 1500.50,
		"zscore":       1.85,
	}

	for key, val := range testData {
		if err := tcache.Store(key, val); err != nil {
			log.Printf("  写入 %s 失败: %v", key, err)
		} else {
			readBack, _ := tcache.Load(key)
			log.Printf("  写入: %s=%.2f, 读取: %.2f, 匹配: %v",
				key, val, readBack, val == readBack)
		}
	}

	// 打印完整内容
	log.Println()
	log.Println("TCache 完整内容:")
	log.Println(tcache.String())

	log.Println("TCache 测试通过!")
}
