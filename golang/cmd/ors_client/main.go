package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/client"
	commonpb "github.com/yourusername/quantlink-trade-system/pkg/proto/common"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

var (
	gatewayAddr = flag.String("gateway", "localhost:50052", "ORS Gateway address")
	natsAddr    = flag.String("nats", "nats://localhost:4222", "NATS server address")
	strategyID  = flag.String("strategy", "test_strategy_1", "Strategy ID")
	symbol      = flag.String("symbol", "ag2412", "Symbol to trade")
	testMode    = flag.String("mode", "single", "Test mode: single, batch, monitor")
)

func printBanner() {
	fmt.Println("╔═══════════════════════════════════════════════════════════╗")
	fmt.Println("║           HFT ORS Client - Order Testing Tool            ║")
	fmt.Println("╚═══════════════════════════════════════════════════════════╝")
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  -gateway string")
	fmt.Println("        ORS Gateway address (default: localhost:50052)")
	fmt.Println("  -nats string")
	fmt.Println("        NATS server address (default: nats://localhost:4222)")
	fmt.Println("  -strategy string")
	fmt.Println("        Strategy ID (default: test_strategy_1)")
	fmt.Println("  -symbol string")
	fmt.Println("        Symbol to trade (default: ag2412)")
	fmt.Println("  -mode string")
	fmt.Println("        Test mode: single, batch, monitor (default: single)")
	fmt.Println()
	fmt.Println("Test Modes:")
	fmt.Println("  single  - Send a single test order")
	fmt.Println("  batch   - Send 10 test orders")
	fmt.Println("  monitor - Subscribe to order updates only")
}

func main() {
	flag.Parse()

	printBanner()

	if *gatewayAddr == "" {
		printUsage()
		os.Exit(1)
	}

	// 创建ORS客户端
	config := client.ORSClientConfig{
		GatewayAddr: *gatewayAddr,
		NATSAddr:    *natsAddr,
		StrategyID:  *strategyID,
	}

	orsClient, err := client.NewORSClient(config)
	if err != nil {
		log.Fatalf("[Main] Failed to create ORS client: %v", err)
	}
	defer orsClient.Close()

	fmt.Printf("[Main] Connected to ORS Gateway: %s\n", *gatewayAddr)
	fmt.Printf("[Main] Strategy ID: %s\n", *strategyID)
	fmt.Printf("[Main] Symbol: %s\n", *symbol)
	fmt.Printf("[Main] Test Mode: %s\n\n", *testMode)

	// 订阅订单回报
	err = orsClient.SubscribeOrderUpdates(func(update *orspb.OrderUpdate) {
		fmt.Printf("[OrderUpdate] OrderID: %s, Status: %s, FilledQty: %d, AvgPrice: %.2f\n",
			update.OrderId,
			update.Status.String(),
			update.FilledQty,
			update.AvgPrice)
	})
	if err != nil {
		log.Printf("[Main] Warning: Failed to subscribe to order updates: %v", err)
	}

	// 根据测试模式执行
	switch *testMode {
	case "single":
		testSingleOrder(orsClient)
	case "batch":
		testBatchOrders(orsClient)
	case "monitor":
		monitorOrderUpdates()
	default:
		log.Fatalf("[Main] Unknown test mode: %s", *testMode)
	}

	// 打印统计
	stats := orsClient.GetStatistics()
	fmt.Println("\n" + strings.Repeat("═", 60))
	fmt.Println("Statistics:")
	fmt.Printf("  Orders Sent:     %d\n", stats["orders_sent"])
	fmt.Printf("  Orders Accepted: %d\n", stats["orders_accepted"])
	fmt.Printf("  Orders Rejected: %d\n", stats["orders_rejected"])
	fmt.Println(strings.Repeat("═", 60))
}

// testSingleOrder 测试单个订单
func testSingleOrder(orsClient *client.ORSClient) {
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("Test Mode: Single Order")
	fmt.Println(strings.Repeat("─", 60))

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// 构造订单请求
	req := &orspb.OrderRequest{
		StrategyId:  *strategyID,
		Symbol:      *symbol,
		Exchange:    commonpb.Exchange_SHFE,
		Side:        orspb.OrderSide_BUY,
		OrderType:   orspb.OrderType_LIMIT,
		Price:       7950.0,
		Quantity:    10,
		TimeInForce: orspb.TimeInForce_GTC,
		OpenClose:   orspb.OpenClose_OPEN,
		Account:     "test_account",
	}

	fmt.Println("\nSending Order:")
	fmt.Printf("  Strategy: %s\n", req.StrategyId)
	fmt.Printf("  Symbol:   %s\n", req.Symbol)
	fmt.Printf("  Side:     %s\n", req.Side.String())
	fmt.Printf("  Price:    %.2f\n", req.Price)
	fmt.Printf("  Quantity: %d\n", req.Quantity)
	fmt.Println()

	// 发送订单
	startTime := time.Now()
	resp, err := orsClient.SendOrder(ctx, req)
	latency := time.Since(startTime)

	if err != nil {
		log.Fatalf("[Main] Failed to send order: %v", err)
	}

	// 打印响应
	fmt.Println("Order Response:")
	fmt.Printf("  Order ID:    %s\n", resp.OrderId)
	fmt.Printf("  Client Token: %d\n", resp.ClientToken)
	fmt.Printf("  Error Code:  %s\n", resp.ErrorCode.String())
	if resp.ErrorMsg != "" {
		fmt.Printf("  Error Msg:   %s\n", resp.ErrorMsg)
	}
	fmt.Printf("  Latency:     %v\n", latency)
	fmt.Println()

	if resp.ErrorCode == orspb.ErrorCode_SUCCESS {
		fmt.Println("✓ Order sent successfully!")
	} else {
		fmt.Println("✗ Order rejected!")
	}

	// 等待一段时间接收回报
	fmt.Println("\nWaiting for order updates (5 seconds)...")
	time.Sleep(5 * time.Second)
}

// testBatchOrders 测试批量订单
func testBatchOrders(orsClient *client.ORSClient) {
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("Test Mode: Batch Orders (10 orders)")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println()

	const numOrders = 10

	for i := 0; i < numOrders; i++ {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

		// 交替买卖
		side := orspb.OrderSide_BUY
		price := 7950.0 + float64(i)
		if i%2 == 1 {
			side = orspb.OrderSide_SELL
			price = 7951.0 + float64(i)
		}

		req := &orspb.OrderRequest{
			StrategyId:  *strategyID,
			Symbol:      *symbol,
			Exchange:    commonpb.Exchange_SHFE,
			Side:        side,
			OrderType:   orspb.OrderType_LIMIT,
			Price:       price,
			Quantity:    int64(10 + i),
			TimeInForce: orspb.TimeInForce_GTC,
			OpenClose:   orspb.OpenClose_OPEN,
		}

		startTime := time.Now()
		resp, err := orsClient.SendOrder(ctx, req)
		latency := time.Since(startTime)

		cancel()

		if err != nil {
			log.Printf("[Main] Order %d failed: %v", i+1, err)
			continue
		}

		fmt.Printf("[%d/%d] OrderID: %s, Side: %s, Price: %.2f, Qty: %d, Latency: %v, Status: %s\n",
			i+1, numOrders,
			resp.OrderId,
			req.Side.String(),
			req.Price,
			req.Quantity,
			latency,
			resp.ErrorCode.String())

		// 短暂延迟
		time.Sleep(100 * time.Millisecond)
	}

	// 等待回报
	fmt.Println("\nWaiting for order updates (10 seconds)...")
	time.Sleep(10 * time.Second)
}

// monitorOrderUpdates 仅监听订单回报
func monitorOrderUpdates() {
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("Test Mode: Monitor Order Updates")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("\nListening for order updates... (Press Ctrl+C to exit)")

	// 等待信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan

	fmt.Println("\n\nExiting...")
}
