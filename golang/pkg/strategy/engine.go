package strategy

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	"github.com/yourusername/quantlink-trade-system/pkg/client"
	"github.com/yourusername/quantlink-trade-system/pkg/indicators"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

// StrategyEngine manages multiple trading strategies
type StrategyEngine struct {
	strategies      map[string]Strategy // strategy_id -> Strategy
	orsClient       *client.ORSClient
	natsConn        *nats.Conn
	mdSubscriptions map[string]*nats.Subscription // symbol -> subscription
	sharedIndPool   *indicators.SharedIndicatorPool // Shared indicator pool (like tbsrc Instrument-level indicators)

	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	mu              sync.RWMutex

	isRunning       bool
	orderQueue      chan *TradingSignal
	config          *EngineConfig
}

// OrderMode defines how orders are sent
type OrderMode int

const (
	// OrderModeAsync - Asynchronous order sending via queue (high throughput)
	OrderModeAsync OrderMode = iota
	// OrderModeSync - Synchronous order sending (low latency, like tbsrc)
	OrderModeSync
)

// EngineConfig represents strategy engine configuration
type EngineConfig struct {
	ORSGatewayAddr      string        // ORS Gateway address
	NATSAddr            string        // NATS server address
	OrderQueueSize      int           // Order queue buffer size
	TimerInterval       time.Duration // Timer interval for strategies
	MaxConcurrentOrders int           // Max concurrent orders
	OrderMode           OrderMode     // Order sending mode (Sync or Async)
	OrderTimeout        time.Duration // Timeout for synchronous order sending
}

// NewStrategyEngine creates a new strategy engine
func NewStrategyEngine(config *EngineConfig) *StrategyEngine {
	ctx, cancel := context.WithCancel(context.Background())

	// Set default values
	if config.OrderTimeout == 0 {
		config.OrderTimeout = 50 * time.Millisecond // Default 50ms timeout
	}
	if config.OrderQueueSize == 0 {
		config.OrderQueueSize = 1000 // Default queue size
	}

	return &StrategyEngine{
		strategies:      make(map[string]Strategy),
		mdSubscriptions: make(map[string]*nats.Subscription),
		sharedIndPool:   indicators.NewSharedIndicatorPool(),
		ctx:             ctx,
		cancel:          cancel,
		orderQueue:      make(chan *TradingSignal, config.OrderQueueSize),
		config:          config,
	}
}

// Initialize initializes the strategy engine
func (se *StrategyEngine) Initialize() error {
	// Connect to NATS
	var err error
	se.natsConn, err = nats.Connect(se.config.NATSAddr)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS: %w", err)
	}
	log.Printf("[StrategyEngine] Connected to NATS: %s", se.config.NATSAddr)

	// Initialize ORS client
	se.orsClient, err = client.NewORSClient(client.ORSClientConfig{
		GatewayAddr:       se.config.ORSGatewayAddr,
		NATSAddr:          se.config.NATSAddr,
		CounterBridgeAddr: "localhost:8080", // Counter Bridge地址（用于持仓查询）
		StrategyID:        "strategy_engine", // 使用通用ID
	})
	if err != nil {
		return fmt.Errorf("failed to initialize ORS client: %w", err)
	}
	log.Printf("[StrategyEngine] ORS Client initialized: %s", se.config.ORSGatewayAddr)

	return nil
}

// AddStrategy adds a strategy to the engine
func (se *StrategyEngine) AddStrategy(strategy Strategy) error {
	se.mu.Lock()
	defer se.mu.Unlock()

	id := strategy.GetID()
	if _, exists := se.strategies[id]; exists {
		return fmt.Errorf("strategy %s already exists", id)
	}

	se.strategies[id] = strategy
	log.Printf("[StrategyEngine] Added strategy: %s (type: %s)", id, strategy.GetType())
	return nil
}

// RemoveStrategy removes a strategy from the engine
func (se *StrategyEngine) RemoveStrategy(strategyID string) error {
	se.mu.Lock()
	defer se.mu.Unlock()

	strategy, exists := se.strategies[strategyID]
	if !exists {
		return fmt.Errorf("strategy %s not found", strategyID)
	}

	// Stop strategy if running
	if strategy.IsRunning() {
		strategy.Stop()
	}

	delete(se.strategies, strategyID)
	log.Printf("[StrategyEngine] Removed strategy: %s", strategyID)
	return nil
}

// GetStrategy returns a strategy by ID
func (se *StrategyEngine) GetStrategy(strategyID string) (Strategy, bool) {
	se.mu.RLock()
	defer se.mu.RUnlock()
	strategy, exists := se.strategies[strategyID]
	return strategy, exists
}

// Start starts the strategy engine
func (se *StrategyEngine) Start() error {
	se.mu.Lock()
	if se.isRunning {
		se.mu.Unlock()
		return fmt.Errorf("strategy engine already running")
	}
	se.isRunning = true
	se.mu.Unlock()

	log.Println("[StrategyEngine] Starting...")

	// Start order processing goroutine (only for async mode)
	if se.config.OrderMode == OrderModeAsync {
		se.wg.Add(1)
		go se.processOrders()
		log.Println("[StrategyEngine] Order mode: Async (queue-based)")
	} else {
		log.Println("[StrategyEngine] Order mode: Sync (direct send, low-latency)")
	}

	// Start timer goroutine
	se.wg.Add(1)
	go se.timerLoop()

	// Subscribe to order updates for all strategies
	se.subscribeOrderUpdates()

	log.Println("[StrategyEngine] Started successfully")
	return nil
}

// Stop stops the strategy engine
func (se *StrategyEngine) Stop() error {
	se.mu.Lock()
	if !se.isRunning {
		se.mu.Unlock()
		return fmt.Errorf("strategy engine not running")
	}
	se.isRunning = false
	se.mu.Unlock()

	log.Println("[StrategyEngine] Stopping...")

	// Stop all strategies
	se.mu.RLock()
	for _, strategy := range se.strategies {
		if strategy.IsRunning() {
			strategy.Stop()
		}
	}
	se.mu.RUnlock()

	// Cancel context
	se.cancel()

	// Unsubscribe from all market data
	for symbol, sub := range se.mdSubscriptions {
		sub.Unsubscribe()
		log.Printf("[StrategyEngine] Unsubscribed from market data: %s", symbol)
	}

	// Close connections
	if se.natsConn != nil {
		se.natsConn.Close()
	}

	// Wait for goroutines to finish
	se.wg.Wait()

	log.Println("[StrategyEngine] Stopped")
	return nil
}

// SubscribeMarketData subscribes to market data for a symbol
func (se *StrategyEngine) SubscribeMarketData(symbol string) error {
	se.mu.Lock()
	defer se.mu.Unlock()

	if _, exists := se.mdSubscriptions[symbol]; exists {
		return nil // Already subscribed
	}

	// Subscribe to md.*.symbol to match MD Gateway's md.exchange.symbol format
	subject := fmt.Sprintf("md.*.%s", symbol)
	sub, err := se.natsConn.Subscribe(subject, func(msg *nats.Msg) {
		// Parse market data update
		var md mdpb.MarketDataUpdate
		if err := proto.Unmarshal(msg.Data, &md); err != nil {
			log.Printf("[StrategyEngine] Failed to unmarshal market data: %v", err)
			return
		}

		log.Printf("[StrategyEngine] Received market data: %s (bid: %.2f, ask: %.2f)",
			md.Symbol, md.BidPrice[0], md.AskPrice[0])

		// Dispatch to all strategies
		se.dispatchMarketData(&md)
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to %s: %w", subject, err)
	}

	se.mdSubscriptions[symbol] = sub
	log.Printf("[StrategyEngine] Subscribed to market data: %s", symbol)
	return nil
}

// dispatchMarketData dispatches market data to all strategies
func (se *StrategyEngine) dispatchMarketData(md *mdpb.MarketDataUpdate) {
	if se.config.OrderMode == OrderModeSync {
		se.dispatchMarketDataSync(md)
	} else {
		se.dispatchMarketDataAsync(md)
	}
}

// dispatchMarketDataSync - Synchronous mode (low latency, like tbsrc)
func (se *StrategyEngine) dispatchMarketDataSync(md *mdpb.MarketDataUpdate) {
	// Step 1: Update shared indicators first (only once for all strategies)
	// 步骤1：先更新共享指标（所有策略只计算一次）
	se.sharedIndPool.UpdateAll(md.Symbol, md)

	// Step 2: Notify strategies about indicator update (optional interface)
	// 步骤2：通知策略指标已更新（可选接口，类似tbsrc INDCallBack）
	se.mu.RLock()
	sharedInds, _ := se.sharedIndPool.Get(md.Symbol)
	for _, strategy := range se.strategies {
		if !strategy.IsRunning() {
			continue
		}
		// Check if strategy implements IndicatorAwareStrategy interface
		if indStrategy, ok := strategy.(IndicatorAwareStrategy); ok {
			indStrategy.OnIndicatorUpdate(md.Symbol, sharedInds)
		}
	}
	se.mu.RUnlock()

	// Step 3: Process each strategy (distinguish auction vs continuous)
	// 步骤3：处理每个策略（区分竞价期/连续交易期，类似tbsrc AuctionCallBack）
	se.mu.RLock()
	defer se.mu.RUnlock()

	for _, strategy := range se.strategies {
		if !strategy.IsRunning() {
			continue
		}

		// Synchronous processing - no goroutine overhead
		func(s Strategy) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[StrategyEngine] Panic in strategy %s: %v", s.GetID(), r)
				}
			}()

			// 1. Update LastMarketData for WebSocket push
			if accessor, ok := s.(BaseStrategyAccessor); ok {
				if baseStrat := accessor.GetBaseStrategy(); baseStrat != nil {
					baseStrat.LastMarketData = md
				}
			}

			// 2. Call market data callback
			// TODO: Add FeedType to MarketDataUpdate protobuf to distinguish auction vs continuous
			// For now, always call OnMarketData
			s.OnMarketData(md)

			// 3. Immediately collect signals
			signals := s.GetSignals()

			// 3. Send orders immediately (synchronous) - but check state first
			// 对应 tbsrc: !m_onFlat && m_Active check before SendOrder()
			for _, signal := range signals {
				// Check if strategy can send orders (aligned with tbsrc)
				if accessor, ok := s.(BaseStrategyAccessor); ok {
					baseStrat := accessor.GetBaseStrategy()
					if baseStrat != nil && baseStrat.ControlState != nil {
						if !baseStrat.CanSendOrder() {
							log.Printf("[%s] Skipping order: strategy not in active state (%s)",
								s.GetID(), baseStrat.ControlState.String())
							continue
						}
					}
				}

				se.sendOrderSync(signal)
			}
		}(strategy)
	}
}

// dispatchMarketDataAsync - Asynchronous mode (high throughput, original behavior)
func (se *StrategyEngine) dispatchMarketDataAsync(md *mdpb.MarketDataUpdate) {
	// Step 1: Update shared indicators first (only once for all strategies)
	// 步骤1：先更新共享指标（所有策略只计算一次）
	se.sharedIndPool.UpdateAll(md.Symbol, md)

	// Step 2: Notify strategies about indicator update (optional interface)
	// 步骤2：通知策略指标已更新（可选接口）
	se.mu.RLock()
	sharedInds, _ := se.sharedIndPool.Get(md.Symbol)
	for _, strategy := range se.strategies {
		if !strategy.IsRunning() {
			continue
		}
		if indStrategy, ok := strategy.(IndicatorAwareStrategy); ok {
			indStrategy.OnIndicatorUpdate(md.Symbol, sharedInds)
		}
	}
	se.mu.RUnlock()

	// Step 3: Process each strategy in goroutine
	// 步骤3：在goroutine中处理每个策略
	se.mu.RLock()
	defer se.mu.RUnlock()

	for _, strategy := range se.strategies {
		if !strategy.IsRunning() {
			continue
		}

		// Dispatch in goroutine to avoid blocking
		go func(s Strategy) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[StrategyEngine] Panic in strategy %s: %v", s.GetID(), r)
				}
			}()

			// Update LastMarketData for WebSocket push
			if accessor, ok := s.(BaseStrategyAccessor); ok {
				if baseStrat := accessor.GetBaseStrategy(); baseStrat != nil {
					baseStrat.LastMarketData = md
				}
			}

			// Call market data callback
			// TODO: Add FeedType to MarketDataUpdate protobuf to distinguish auction vs continuous
			s.OnMarketData(md)

			// Collect signals
			signals := s.GetSignals()
			for _, signal := range signals {
				select {
				case se.orderQueue <- signal:
				case <-se.ctx.Done():
					return
				}
			}
		}(strategy)
	}
}

// sendOrderSync sends an order synchronously (for low-latency mode)
func (se *StrategyEngine) sendOrderSync(signal *TradingSignal) {
	// Convert signal to order request
	req := signal.ToOrderRequest()

	// Send order with timeout
	ctx, cancel := context.WithTimeout(se.ctx, se.config.OrderTimeout)
	defer cancel()

	resp, err := se.sendOrder(ctx, req)
	if err != nil {
		log.Printf("[StrategyEngine] Order failed for %s: %v", signal.StrategyID, err)
		return
	}

	log.Printf("[StrategyEngine] Order sent: %s, OrderID: %s, Status: %v",
		signal.StrategyID, resp.OrderId, resp.ErrorCode)
}

// subscribeOrderUpdates subscribes to order updates
func (se *StrategyEngine) subscribeOrderUpdates() error {
	// Subscribe to all strategy order updates
	subject := "order.>"
	_, err := se.natsConn.Subscribe(subject, func(msg *nats.Msg) {
		// Parse order update
		var update orspb.OrderUpdate
		// Note: Actual unmarshal implementation depends on protobuf version
		_ = msg.Data

		// Dispatch to strategies
		se.dispatchOrderUpdate(&update)
	})

	if err != nil {
		return fmt.Errorf("failed to subscribe to order updates: %w", err)
	}

	log.Println("[StrategyEngine] Subscribed to order updates")
	return nil
}

// dispatchOrderUpdate dispatches order update to strategies
func (se *StrategyEngine) dispatchOrderUpdate(update *orspb.OrderUpdate) {
	se.mu.RLock()
	defer se.mu.RUnlock()

	// Dispatch to all strategies (they will filter based on their orders)
	for _, strategy := range se.strategies {
		if !strategy.IsRunning() {
			continue
		}

		go func(s Strategy) {
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[StrategyEngine] Panic in strategy %s OnOrderUpdate: %v", s.GetID(), r)
				}
			}()

			// Call general OnOrderUpdate first
			s.OnOrderUpdate(update)

			// If strategy implements DetailedOrderStrategy, call fine-grained callbacks
			if detailedStrategy, ok := s.(DetailedOrderStrategy); ok {
				switch update.Status {
				case orspb.OrderStatus_ACCEPTED, orspb.OrderStatus_SUBMITTED:
					detailedStrategy.OnOrderNew(update)
				case orspb.OrderStatus_FILLED, orspb.OrderStatus_PARTIALLY_FILLED:
					detailedStrategy.OnOrderFilled(update)
				case orspb.OrderStatus_CANCELED:
					detailedStrategy.OnOrderCanceled(update)
				case orspb.OrderStatus_REJECTED:
					detailedStrategy.OnOrderRejected(update)
				}
			}
		}(strategy)
	}
}

// processOrders processes trading signals and sends orders
func (se *StrategyEngine) processOrders() {
	defer se.wg.Done()

	log.Println("[StrategyEngine] Order processor started")

	for {
		select {
		case signal := <-se.orderQueue:
			// Convert signal to order request
			req := signal.ToOrderRequest()

			// Send order via ORS client
			ctx, cancel := context.WithTimeout(se.ctx, 5*time.Second)
			resp, err := se.sendOrder(ctx, req)
			cancel()

			if err != nil {
				log.Printf("[StrategyEngine] Failed to send order for %s: %v", signal.StrategyID, err)
			} else {
				log.Printf("[StrategyEngine] Order sent: %s, OrderID: %s, Status: %v",
					signal.StrategyID, resp.OrderId, resp.ErrorCode)
			}

		case <-se.ctx.Done():
			log.Println("[StrategyEngine] Order processor stopped")
			return
		}
	}
}

// sendOrder sends an order via ORS client
func (se *StrategyEngine) sendOrder(ctx context.Context, req *orspb.OrderRequest) (*orspb.OrderResponse, error) {
	// Send order via ORS client (gRPC)
	if se.orsClient != nil {
		return se.orsClient.SendOrder(ctx, req)
	}

	// Fallback for testing/simulation
	return &orspb.OrderResponse{
		OrderId:   fmt.Sprintf("ORD_%d", time.Now().UnixNano()),
		ErrorCode: orspb.ErrorCode_SUCCESS,
	}, nil
}

// timerLoop calls OnTimer for all strategies periodically
// Also performs risk checks and state recovery (aligned with tbsrc CheckSquareoff)
func (se *StrategyEngine) timerLoop() {
	defer se.wg.Done()

	ticker := time.NewTicker(se.config.TimerInterval)
	defer ticker.Stop()

	log.Printf("[StrategyEngine] Timer loop started (interval: %v)", se.config.TimerInterval)

	for {
		select {
		case now := <-ticker.C:
			se.mu.RLock()
			for _, strategy := range se.strategies {
				if !strategy.IsRunning() {
					continue
				}

				// Perform risk checks and state management (aligned with tbsrc)
				se.performStateCheck(strategy)

				// Call strategy's timer callback
				go func(s Strategy, t time.Time) {
					defer func() {
						if r := recover(); r != nil {
							log.Printf("[StrategyEngine] Panic in strategy %s OnTimer: %v", s.GetID(), r)
						}
					}()
					s.OnTimer(t)
				}(strategy, now)
			}
			se.mu.RUnlock()

		case <-se.ctx.Done():
			log.Println("[StrategyEngine] Timer loop stopped")
			return
		}
	}
}

// performStateCheck performs risk checks and state management for a strategy
// Aligned with tbsrc's CheckSquareoff() logic
func (se *StrategyEngine) performStateCheck(strategy Strategy) {
	// Try to access BaseStrategy for state control
	accessor, ok := strategy.(BaseStrategyAccessor)
	if !ok {
		return // Strategy doesn't expose BaseStrategy
	}

	baseStrat := accessor.GetBaseStrategy()
	if baseStrat == nil || baseStrat.ControlState == nil {
		return
	}

	// Check and handle risk limits
	// This will trigger flatten/exit if limits are exceeded
	baseStrat.CheckAndHandleRiskLimits()

	// Handle flatten process if in flatten mode
	if baseStrat.ControlState.FlattenMode || baseStrat.ControlState.ExitRequested {
		// Get current price for flatten orders
		// TODO: Use actual market price from latest market data
		currentPrice := 0.0
		if baseStrat.Position.AvgLongPrice > 0 {
			currentPrice = baseStrat.Position.AvgLongPrice
		} else if baseStrat.Position.AvgShortPrice > 0 {
			currentPrice = baseStrat.Position.AvgShortPrice
		}

		if currentPrice > 0 {
			baseStrat.HandleFlatten(currentPrice)
		}
	}
}

// GetAllStatuses returns status of all strategies
func (se *StrategyEngine) GetAllStatuses() map[string]*StrategyStatus {
	se.mu.RLock()
	defer se.mu.RUnlock()

	statuses := make(map[string]*StrategyStatus)
	for id, strategy := range se.strategies {
		statuses[id] = strategy.GetStatus()
	}
	return statuses
}

// PrintStatistics prints statistics for all strategies
func (se *StrategyEngine) PrintStatistics() {
	se.mu.RLock()
	defer se.mu.RUnlock()

	fmt.Println("\n════════════════════════════════════════════════════════════")
	fmt.Println("Strategy Engine Statistics")
	fmt.Println("════════════════════════════════════════════════════════════")

	for _, strategy := range se.strategies {
		status := strategy.GetStatus()
		position := strategy.GetPosition()
		pnl := strategy.GetPNL()

		fmt.Printf("\nStrategy: %s (Type: %s)\n", status.StrategyID, strategy.GetType())
		fmt.Printf("  Running: %v\n", status.IsRunning)
		fmt.Printf("  Position: %d (Long: %d, Short: %d)\n",
			position.NetQty, position.LongQty, position.ShortQty)
		fmt.Printf("  P&L: %.2f (Realized: %.2f, Unrealized: %.2f)\n",
			pnl.TotalPnL, pnl.RealizedPnL, pnl.UnrealizedPnL)
		fmt.Printf("  Orders: Signals=%d, Sent=%d, Fills=%d, Rejects=%d\n",
			status.SignalCount, status.OrderCount, status.FillCount, status.RejectCount)
	}

	fmt.Println("════════════════════════════════════════════════════════════")
}

// GetORSClient returns the ORS client instance
func (se *StrategyEngine) GetORSClient() *client.ORSClient {
	return se.orsClient
}
