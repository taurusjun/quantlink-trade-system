package strategy

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"google.golang.org/grpc"

	"github.com/yourusername/hft-poc/pkg/client"
	mdpb "github.com/yourusername/hft-poc/pkg/proto/md"
	orspb "github.com/yourusername/hft-poc/pkg/proto/ors"
)

// StrategyEngine manages multiple trading strategies
type StrategyEngine struct {
	strategies      map[string]Strategy // strategy_id -> Strategy
	orsClient       *client.ORSClient
	natsConn        *nats.Conn
	mdSubscriptions map[string]*nats.Subscription // symbol -> subscription

	ctx             context.Context
	cancel          context.CancelFunc
	wg              sync.WaitGroup
	mu              sync.RWMutex

	isRunning       bool
	orderQueue      chan *TradingSignal
	config          *EngineConfig
}

// EngineConfig represents strategy engine configuration
type EngineConfig struct {
	ORSGatewayAddr  string        // ORS Gateway address
	NATSAddr        string        // NATS server address
	OrderQueueSize  int           // Order queue buffer size
	TimerInterval   time.Duration // Timer interval for strategies
	MaxConcurrentOrders int       // Max concurrent orders
}

// NewStrategyEngine creates a new strategy engine
func NewStrategyEngine(config *EngineConfig) *StrategyEngine {
	ctx, cancel := context.WithCancel(context.Background())

	return &StrategyEngine{
		strategies:      make(map[string]Strategy),
		mdSubscriptions: make(map[string]*nats.Subscription),
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

	// Connect to ORS Gateway
	_, err = grpc.Dial(se.config.ORSGatewayAddr, grpc.WithInsecure())
	if err != nil {
		return fmt.Errorf("failed to connect to ORS Gateway: %w", err)
	}

	// Initialize ORS client (simplified for now)
	se.orsClient = &client.ORSClient{}
	log.Printf("[StrategyEngine] Connected to ORS Gateway: %s", se.config.ORSGatewayAddr)

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

	// Start order processing goroutine
	se.wg.Add(1)
	go se.processOrders()

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

	subject := fmt.Sprintf("md.%s", symbol)
	sub, err := se.natsConn.Subscribe(subject, func(msg *nats.Msg) {
		// Parse market data update
		var md mdpb.MarketDataUpdate
		// Note: Actual unmarshal implementation depends on protobuf version
		// For now, skip parsing (will be implemented when connecting to real MD Gateway)
		_ = msg.Data

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
					log.Printf("[StrategyEngine] Panic in strategy %s OnMarketData: %v", s.GetID(), r)
				}
			}()

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

			s.OnOrderUpdate(update)
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

// sendOrder sends an order (placeholder - actual implementation uses orsClient)
func (se *StrategyEngine) sendOrder(ctx context.Context, req *orspb.OrderRequest) (*orspb.OrderResponse, error) {
	// This is a placeholder
	// Actual implementation would use se.orsClient.SendOrder(ctx, req)
	return &orspb.OrderResponse{
		OrderId:   fmt.Sprintf("ORD_%d", time.Now().UnixNano()),
		ErrorCode: orspb.ErrorCode_SUCCESS,
	}, nil
}

// timerLoop calls OnTimer for all strategies periodically
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
				if strategy.IsRunning() {
					go func(s Strategy, t time.Time) {
						defer func() {
							if r := recover(); r != nil {
								log.Printf("[StrategyEngine] Panic in strategy %s OnTimer: %v", s.GetID(), r)
							}
						}()
						s.OnTimer(t)
					}(strategy, now)
				}
			}
			se.mu.RUnlock()

		case <-se.ctx.Done():
			log.Println("[StrategyEngine] Timer loop stopped")
			return
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
