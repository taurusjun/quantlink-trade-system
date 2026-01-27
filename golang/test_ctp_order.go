package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

func main() {
	// 连接到ORS Gateway
	conn, err := grpc.Dial("localhost:50052", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}
	defer conn.Close()

	client := orspb.NewOrderServiceClient(conn)

	fmt.Println("╔═══════════════════════════════════════════════════════╗")
	fmt.Println("║           CTP订单测试 - 直接发送测试订单                ║")
	fmt.Println("╚═══════════════════════════════════════════════════════╝")
	fmt.Println("")

	// 创建测试订单（买入ag2603 1手）
	req := &orspb.OrderRequest{
		StrategyId: "test_manual",
		Symbol:     "ag2603",
		Exchange:   "SHFE",
		Side:       orspb.OrderSide_BUY,
		OrderType:  orspb.OrderType_LIMIT,
		Price:      28000.0, // 测试价格
		Volume:     1,       // 1手
		OffsetFlag: orspb.OffsetFlag_OPEN,
	}

	fmt.Printf("发送测试订单：\n")
	fmt.Printf("  品种: %s.%s\n", req.Exchange, req.Symbol)
	fmt.Printf("  方向: 买入\n")
	fmt.Printf("  价格: %.2f\n", req.Price)
	fmt.Printf("  数量: %d手\n", req.Volume)
	fmt.Println("")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	resp, err := client.SendOrder(ctx, req)
	if err != nil {
		log.Fatalf("❌ 发送订单失败: %v", err)
	}

	fmt.Println("╔═══════════════════════════════════════════════════════╗")
	fmt.Println("║                 订单响应                              ║")
	fmt.Println("╚═══════════════════════════════════════════════════════╝")
	fmt.Printf("  订单ID: %s\n", resp.OrderId)
	fmt.Printf("  状态: %s\n", resp.ErrorCode.String())
	fmt.Printf("  消息: %s\n", resp.ErrorMessage)
	fmt.Println("")

	if resp.ErrorCode == orspb.ErrorCode_SUCCESS {
		fmt.Println("✅ 订单发送成功！")
		fmt.Println("")
		fmt.Println("请检查：")
		fmt.Println("  1. ORS Gateway日志: tail -f log/ors_gateway.log")
		fmt.Println("  2. Counter Bridge日志: tail -f log/counter_bridge.log")
		fmt.Println("  3. CTP交易日志: tail -f log/ctp_td.log")
	} else {
		fmt.Println("❌ 订单发送失败")
	}
}
