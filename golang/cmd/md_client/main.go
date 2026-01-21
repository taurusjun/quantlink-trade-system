package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/client"
)

var (
	gatewayAddr = flag.String("gateway", "localhost:50051", "MD Gateway address")
	symbols     = flag.String("symbols", "ag2412,cu2412", "Comma-separated symbol list")
	useNATS     = flag.Bool("nats", false, "Use NATS instead of gRPC")
	natsURL     = flag.String("nats-url", "nats://localhost:4222", "NATS server URL")
)

func main() {
	flag.Parse()

	fmt.Println(`
╔═══════════════════════════════════════════════════════╗
║       HFT Market Data Client - POC v0.1               ║
╚═══════════════════════════════════════════════════════╝
	`)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 信号处理
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	if *useNATS {
		runNATSClient(ctx, cancel)
	} else {
		runGRPCClient(ctx, cancel)
	}

	// 等待退出信号
	<-sigChan
	fmt.Println("\n[Client] Shutting down...")
	cancel()
	time.Sleep(time.Second)
}

func runGRPCClient(ctx context.Context, cancel context.CancelFunc) {
	// 创建gRPC客户端
	mdClient, err := client.NewMDClient(*gatewayAddr)
	if err != nil {
		log.Fatalf("[Client] Failed to create MD client: %v", err)
	}
	defer mdClient.Close()

	fmt.Printf("[Client] Connected to gateway: %s\n", *gatewayAddr)

	// 订阅行情
	symbolList := parseSymbols(*symbols)
	stream, err := mdClient.Subscribe(ctx, symbolList, "SHFE", true)
	if err != nil {
		log.Fatalf("[Client] Failed to subscribe: %v", err)
	}

	fmt.Printf("[Client] Subscribed to symbols: %v\n", symbolList)

	// 统计
	startTime := time.Now()
	count := 0
	var totalLatency time.Duration

	// 接收行情
	for {
		md, err := stream.Recv()
		if err != nil {
			log.Printf("[Client] Stream error: %v", err)
			cancel()
			return
		}

		count++
		now := time.Now()
		mdTime := time.Unix(0, int64(md.Timestamp))
		latency := now.Sub(mdTime)
		totalLatency += latency

		// 前10条每条都打印，之后每10个打印一次
		if count <= 10 || count%10 == 0 {
			elapsed := time.Since(startTime)
			avgLatency := totalLatency / time.Duration(count)
			throughput := float64(count) / elapsed.Seconds()

			fmt.Printf("[Client] Count: %d, Avg Latency: %v, Throughput: %.0f msg/s\n",
				count, avgLatency, throughput)
		}

		// 每1000个详细打印
		if count%1000 == 0 {
			printMarketData(md)
		}
	}
}

func runNATSClient(ctx context.Context, cancel context.CancelFunc) {
	// 创建NATS客户端
	natsClient, err := client.NewNATSClient(*natsURL)
	if err != nil {
		log.Fatalf("[Client] Failed to create NATS client: %v", err)
	}
	defer natsClient.Close()

	fmt.Printf("[Client] Connected to NATS: %s\n", *natsURL)

	// 订阅行情主题
	symbolList := parseSymbols(*symbols)
	for _, symbol := range symbolList {
		subject := fmt.Sprintf("md.SHFE.%s", symbol)
		fmt.Printf("[Client] Subscribing to NATS subject: %s\n", subject)

		err := natsClient.Subscribe(subject, func(md *client.MarketData) {
			// 处理行情
			now := time.Now()
			mdTime := time.Unix(0, int64(md.Timestamp))
			latency := now.Sub(mdTime)

			fmt.Printf("[Client] Received %s: BidPx=%.1f, AskPx=%.1f, Latency=%v\n",
				md.Symbol, md.BidPrice[0], md.AskPrice[0], latency)
		})

		if err != nil {
			log.Fatalf("[Client] Failed to subscribe to %s: %v", subject, err)
		}
	}

	fmt.Println("[Client] Listening for market data...")

	// 等待取消
	<-ctx.Done()
}

func parseSymbols(s string) []string {
	var result []string
	for i := 0; i < len(s); {
		j := i
		for j < len(s) && s[j] != ',' {
			j++
		}
		if i < j {
			result = append(result, s[i:j])
		}
		i = j + 1
	}
	return result
}

func printMarketData(md *client.MarketData) {
	fmt.Printf(`
─────────────────────────────────────
Symbol:    %s
Exchange:  %s
Timestamp: %v
─────────────────────────────────────
Bid5: %.1f × %d  |  Ask5: %.1f × %d
Bid4: %.1f × %d  |  Ask4: %.1f × %d
Bid3: %.1f × %d  |  Ask3: %.1f × %d
Bid2: %.1f × %d  |  Ask2: %.1f × %d
Bid1: %.1f × %d  |  Ask1: %.1f × %d
─────────────────────────────────────
Last: %.1f × %d, Volume: %d
─────────────────────────────────────
`,
		md.Symbol, md.Exchange,
		time.Unix(0, int64(md.Timestamp)),
		md.BidPrice[4], md.BidQty[4], md.AskPrice[4], md.AskQty[4],
		md.BidPrice[3], md.BidQty[3], md.AskPrice[3], md.AskQty[3],
		md.BidPrice[2], md.BidQty[2], md.AskPrice[2], md.AskQty[2],
		md.BidPrice[1], md.BidQty[1], md.AskPrice[1], md.AskQty[1],
		md.BidPrice[0], md.BidQty[0], md.AskPrice[0], md.AskQty[0],
		md.LastPrice, md.LastQty, md.TotalVolume,
	)
}
