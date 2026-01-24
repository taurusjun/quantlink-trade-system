package backtest

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/yourusername/quantlink-trade-system/pkg/config"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
	"github.com/yourusername/quantlink-trade-system/pkg/trader"
	"google.golang.org/protobuf/proto"
)

// BacktestRunner coordinates all components and runs the backtest
type BacktestRunner struct {
	config      *BacktestConfig
	dataReader  *HistoricalDataReader
	orderRouter *BacktestOrderRouter
	statistics  *BacktestStatistics
	trader      TraderInterface // 策略引擎接口
	natsConn    *nats.Conn
	mdSub       *nats.Subscription

	ctx    context.Context
	cancel context.CancelFunc
}

// TraderInterface defines the interface for trader in backtest
type TraderInterface interface {
	Initialize() error
	Start() error
	Stop() error
	IsRunning() bool
}

// NewBacktestRunner creates a new backtest runner
func NewBacktestRunner(config *BacktestConfig) (*BacktestRunner, error) {
	ctx, cancel := context.WithCancel(context.Background())

	runner := &BacktestRunner{
		config: config,
		ctx:    ctx,
		cancel: cancel,
	}

	return runner, nil
}

// Run runs the complete backtest
func (r *BacktestRunner) Run() (*BacktestResult, error) {
	log.Println("[Backtest] ========================================")
	log.Println("[Backtest] Starting backtest...")
	log.Println("[Backtest] ========================================")

	// 1. Initialize components
	log.Println("[Backtest] [1/7] Initializing components...")
	if err := r.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	// 2. Load historical data
	log.Println("[Backtest] [2/7] Loading historical data...")
	if err := r.dataReader.LoadData(); err != nil {
		return nil, fmt.Errorf("failed to load data: %w", err)
	}
	log.Printf("[Backtest] Loaded %d ticks", r.dataReader.GetTickCount())

	// 3. Start order router (must start before Trader connects)
	log.Println("[Backtest] [3/8] Starting order router...")
	if err := r.orderRouter.Start(); err != nil {
		return nil, fmt.Errorf("failed to start order router: %w", err)
	}

	// 4. Start Trader (strategy engine)
	log.Println("[Backtest] [4/8] Starting trader (strategy engine)...")
	if err := r.trader.Start(); err != nil {
		return nil, fmt.Errorf("failed to start trader: %w", err)
	}
	log.Println("[Backtest] ✓ Trader started, strategy ready")

	// 5. Subscribe to market data updates (to feed order router and strategy)
	log.Println("[Backtest] [5/8] Subscribing to market data...")
	if err := r.subscribeMarketData(); err != nil {
		return nil, fmt.Errorf("failed to subscribe market data: %w", err)
	}

	// 6. Start data replay
	log.Println("[Backtest] [6/8] Starting data replay...")
	startTime := time.Now()
	if err := r.dataReader.Replay(); err != nil {
		return nil, fmt.Errorf("failed to replay data: %w", err)
	}
	replayDuration := time.Since(startTime)
	log.Printf("[Backtest] Data replay completed in %v", replayDuration)

	// 7. Wait a bit for final order processing
	log.Println("[Backtest] [7/8] Waiting for final order processing...")
	time.Sleep(100 * time.Millisecond)

	// 8. Generate statistics
	log.Println("[Backtest] [8/8] Generating statistics...")
	result := r.statistics.GenerateReport()

	// 9. Cleanup
	r.cleanup()

	log.Println("[Backtest] ========================================")
	log.Println("[Backtest] Backtest completed successfully!")
	log.Println("[Backtest] ========================================")

	// Print summary
	r.statistics.PrintSummary()

	return result, nil
}

// initialize initializes all components
func (r *BacktestRunner) initialize() error {
	// Connect to NATS
	nc, err := nats.Connect(r.config.Engine.NATSAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	r.natsConn = nc
	log.Printf("[Backtest] Connected to NATS at %s", r.config.Engine.NATSAddr)

	// Create data reader
	dataReader, err := NewHistoricalDataReader(r.config, r.natsConn)
	if err != nil {
		return fmt.Errorf("failed to create data reader: %w", err)
	}
	r.dataReader = dataReader

	// Create order router
	// Use port 0 to skip gRPC server (for optimization mode)
	// Use port 50052 for standalone backtest (with Trader integration)
	port := 0
	if r.config.Backtest.EnableTrader {
		port = 50052
	}
	orderRouter, err := NewBacktestOrderRouter(r.config, port)
	if err != nil {
		return fmt.Errorf("failed to create order router: %w", err)
	}
	r.orderRouter = orderRouter

	// Create statistics collector
	r.statistics = NewBacktestStatistics(r.config)

	// Set order update callback
	r.orderRouter.SetOrderUpdateCallback(r.onOrderUpdate)

	// Create Trader (strategy engine) if enabled
	if r.config.Backtest.EnableTrader {
		log.Println("[Backtest] Creating Trader (strategy engine)...")
		traderConfig := r.convertToTraderConfig()
		t, err := trader.NewTrader(traderConfig)
		if err != nil {
			return fmt.Errorf("failed to create trader: %w", err)
		}

		// Initialize Trader
		if err := t.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize trader: %w", err)
		}
		r.trader = t
		log.Println("[Backtest] ✓ Trader created and initialized")
	}

	return nil
}

// subscribeMarketData subscribes to market data to feed the order router
func (r *BacktestRunner) subscribeMarketData() error {
	// Subscribe to all symbols
	subject := "md.*.*" // md.{exchange}.{symbol}

	sub, err := r.natsConn.Subscribe(subject, func(msg *nats.Msg) {
		// Parse market data
		var md mdpb.MarketDataUpdate
		if err := proto.Unmarshal(msg.Data, &md); err != nil {
			log.Printf("[Backtest] Failed to unmarshal market data: %v", err)
			return
		}

		// Update statistics with latest price
		if len(md.BidPrice) > 0 && len(md.AskPrice) > 0 {
			midPrice := (md.BidPrice[0] + md.AskPrice[0]) / 2
			r.statistics.UpdatePrice(md.Symbol, midPrice)
		}

		// Feed to order router for matching
		r.orderRouter.UpdateMarketData(&md)
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to market data: %w", err)
	}

	r.mdSub = sub
	log.Printf("[Backtest] Subscribed to market data: %s", subject)
	return nil
}

// onOrderUpdate handles order update callbacks
func (r *BacktestRunner) onOrderUpdate(update *orspb.OrderUpdate) {
	// Record filled orders in statistics
	if update.Status == orspb.OrderStatus_FILLED {
		// Get fill details from order router
		fills := r.orderRouter.GetFillHistory()
		if len(fills) > 0 {
			lastFill := fills[len(fills)-1]

			// Calculate commission
			commission := float64(lastFill.Volume) * lastFill.Price * r.config.GetCommissionRate()

			// Record trade
			r.statistics.OnTrade(lastFill, update.Side, update.Symbol, commission)
		}
	}
}

// cleanup cleans up resources
func (r *BacktestRunner) cleanup() {
	log.Println("[Backtest] Cleaning up...")

	// Unsubscribe from market data
	if r.mdSub != nil {
		r.mdSub.Unsubscribe()
	}

	// Stop trader (strategy engine)
	if r.trader != nil {
		if err := r.trader.Stop(); err != nil {
			log.Printf("[Backtest] Error stopping trader: %v", err)
		}
	}

	// Stop order router
	if r.orderRouter != nil {
		r.orderRouter.Stop()
	}

	// Stop data reader
	if r.dataReader != nil {
		r.dataReader.Stop()
	}

	// Close NATS connection
	if r.natsConn != nil {
		r.natsConn.Close()
	}
}

// Stop stops the backtest
func (r *BacktestRunner) Stop() {
	r.cancel()
	r.cleanup()
}

// convertToTraderConfig converts backtest config to trader config
func (r *BacktestRunner) convertToTraderConfig() *config.TraderConfig {
	return &config.TraderConfig{
		System: config.SystemConfig{
			StrategyID: r.config.Backtest.Name,
			Mode:       "backtest", // Critical: mark as backtest mode
		},
		Strategy: config.StrategyConfig{
			Type:       r.config.Strategy.Type,
			Symbols:    r.config.Strategy.Symbols,
			Parameters: r.config.Strategy.Parameters,
		},
		Engine: config.EngineConfig{
			ORSGatewayAddr: "localhost:50052", // BacktestOrderRouter gRPC address
			NATSAddr:       r.config.Engine.NATSAddr,
			TimerInterval:  5 * time.Second, // Strategy timer interval
			// OrderQueueSize and MaxConcurrentOrders will use default values
		},
		Session: config.SessionConfig{
			AutoActivate: true, // Backtest mode: auto-activate strategy
			StartTime:    r.config.Backtest.StartTime,
			EndTime:      r.config.Backtest.EndTime,
		},
		Risk: config.RiskConfig{
			CheckIntervalMs: 100,
		},
		API: config.APIConfig{
			Enabled: false, // Disable API server in backtest
		},
	}
}

// RunBatch runs backtest for multiple dates
func RunBatch(config *BacktestConfig, dates []string) ([]*BacktestResult, error) {
	results := make([]*BacktestResult, 0, len(dates))

	for i, date := range dates {
		log.Printf("\n[BatchBacktest] ========================================")
		log.Printf("[BatchBacktest] Running backtest %d/%d for date: %s", i+1, len(dates), date)
		log.Printf("[BatchBacktest] ========================================\n")

		// Update config dates
		config.Backtest.StartDate = date
		config.Backtest.EndDate = date

		// Create runner
		runner, err := NewBacktestRunner(config)
		if err != nil {
			log.Printf("[BatchBacktest] Failed to create runner for %s: %v", date, err)
			continue
		}

		// Run backtest
		result, err := runner.Run()
		if err != nil {
			log.Printf("[BatchBacktest] Failed to run backtest for %s: %v", date, err)
			continue
		}

		results = append(results, result)
	}

	// Print batch summary
	printBatchSummary(results)

	return results, nil
}

// printBatchSummary prints summary for batch backtest
func printBatchSummary(results []*BacktestResult) {
	if len(results) == 0 {
		return
	}

	fmt.Println("\n" + "========================================")
	fmt.Println("BATCH BACKTEST SUMMARY")
	fmt.Println("========================================")

	var totalPNL float64
	var totalTrades int
	var totalWins int

	for _, result := range results {
		totalPNL += result.TotalPNL
		totalTrades += result.TotalTrades
		totalWins += result.WinTrades
	}

	avgPNL := totalPNL / float64(len(results))
	avgTrades := float64(totalTrades) / float64(len(results))
	overallWinRate := float64(totalWins) / float64(totalTrades)

	fmt.Printf("\nTotal Days:        %d\n", len(results))
	fmt.Printf("Total PNL:         %.2f\n", totalPNL)
	fmt.Printf("Average Daily PNL: %.2f\n", avgPNL)
	fmt.Printf("Total Trades:      %d\n", totalTrades)
	fmt.Printf("Avg Trades/Day:    %.1f\n", avgTrades)
	fmt.Printf("Overall Win Rate:  %.1f%%\n", overallWinRate*100)

	fmt.Println("========================================")
}
