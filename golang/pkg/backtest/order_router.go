package backtest

import (
	"context"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
	"google.golang.org/grpc"
)

// BacktestOrderRouter handles order routing and matching in backtest mode
type BacktestOrderRouter struct {
	config       *BacktestConfig
	matchEngine  *SimpleMatchEngine
	orderHistory []*Order
	fillHistory  []*Fill
	mu           sync.RWMutex

	// gRPC server
	grpcServer *grpc.Server
	port       int

	// Order callback
	onOrderUpdate func(*orspb.OrderUpdate)
}

// SimpleMatchEngine provides simple order matching logic
type SimpleMatchEngine struct {
	currentMarketData map[string]*mdpb.MarketDataUpdate
	openOrders        map[string]*Order
	fillDelay         time.Duration
	slippageBps       float64
	commissionRate    float64
	mu                sync.RWMutex
}

// NewBacktestOrderRouter creates a new order router
func NewBacktestOrderRouter(config *BacktestConfig, port int) (*BacktestOrderRouter, error) {
	router := &BacktestOrderRouter{
		config:       config,
		orderHistory: make([]*Order, 0, 1000),
		fillHistory:  make([]*Fill, 0, 1000),
		port:         port,
	}

	// Create match engine
	router.matchEngine = &SimpleMatchEngine{
		currentMarketData: make(map[string]*mdpb.MarketDataUpdate),
		openOrders:        make(map[string]*Order),
		fillDelay:         config.GetFillDelay(),
		slippageBps:       config.GetSlippage(),
		commissionRate:    config.GetCommissionRate(),
	}

	return router, nil
}

// SetOrderUpdateCallback sets the callback for order updates
func (r *BacktestOrderRouter) SetOrderUpdateCallback(callback func(*orspb.OrderUpdate)) {
	r.onOrderUpdate = callback
}

// Start starts the order router and gRPC server
func (r *BacktestOrderRouter) Start() error {
	// Start gRPC server for ORS Gateway interface
	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", r.port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %w", r.port, err)
	}

	r.grpcServer = grpc.NewServer()
	orspb.RegisterORSGatewayServer(r.grpcServer, &BacktestORSService{
		router: r,
	})

	// Start gRPC server in background
	go func() {
		log.Printf("[OrderRouter] gRPC server listening on port %d", r.port)
		if err := r.grpcServer.Serve(lis); err != nil {
			log.Printf("[OrderRouter] gRPC server error: %v", err)
		}
	}()

	// Wait a bit for server to start
	time.Sleep(100 * time.Millisecond)

	log.Printf("[OrderRouter] Order router started (backtest mode)")
	return nil
}

// Stop stops the order router and gRPC server
func (r *BacktestOrderRouter) Stop() error {
	if r.grpcServer != nil {
		log.Println("[OrderRouter] Stopping gRPC server...")
		r.grpcServer.GracefulStop()
	}
	log.Println("[OrderRouter] Order router stopped")
	return nil
}

// UpdateMarketData updates the current market data for matching
func (r *BacktestOrderRouter) UpdateMarketData(md *mdpb.MarketDataUpdate) {
	r.matchEngine.mu.Lock()
	r.matchEngine.currentMarketData[md.Symbol] = md
	r.matchEngine.mu.Unlock()

	// Try to match open orders
	r.tryMatchOpenOrders(md.Symbol)
}

// SubmitOrder submits an order for matching
func (r *BacktestOrderRouter) SubmitOrder(req *orspb.OrderRequest) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate order ID if not provided
	orderID := req.ClientOrderId
	if orderID == "" {
		orderID = fmt.Sprintf("ORD_%d", time.Now().UnixNano())
	}

	// Create order
	order := &Order{
		OrderID:   orderID,
		Symbol:    req.Symbol,
		Side:      req.Side,
		Price:     req.Price,
		Volume:    int32(req.Quantity),
		Filled:    0,
		Status:    orspb.OrderStatus_PENDING,
		Timestamp: time.Now(),
	}

	// Add to history
	r.orderHistory = append(r.orderHistory, order)

	// Send order acknowledgment
	r.sendOrderUpdate(&orspb.OrderUpdate{
		OrderId:       orderID,
		ClientOrderId: req.ClientOrderId,
		StrategyId:    req.StrategyId,
		Symbol:        order.Symbol,
		Side:          order.Side,
		Status:        orspb.OrderStatus_ACCEPTED,
		Timestamp:     uint64(time.Now().UnixNano()),
		ErrorCode:     orspb.ErrorCode_SUCCESS,
	})

	// Try immediate matching
	fill := r.matchEngine.TryMatch(order)
	if fill != nil {
		// Order filled
		order.Filled = fill.Volume
		order.Status = orspb.OrderStatus_FILLED

		// Add to fill history
		r.fillHistory = append(r.fillHistory, fill)

		// Send fill update
		r.sendOrderUpdate(&orspb.OrderUpdate{
			OrderId:       orderID,
			ClientOrderId: req.ClientOrderId,
			StrategyId:    req.StrategyId,
			Symbol:        order.Symbol,
			Side:          order.Side,
			Status:        orspb.OrderStatus_FILLED,
			Price:         order.Price,
			Quantity:      int64(order.Volume),
			FilledQty:     int64(fill.Volume),
			RemainingQty:  0,
			AvgPrice:      fill.Price,
			LastFillPrice: fill.Price,
			LastFillQty:   int64(fill.Volume),
			Timestamp:     uint64(fill.Timestamp.UnixNano()),
			ErrorCode:     orspb.ErrorCode_SUCCESS,
		})

		log.Printf("[OrderRouter] Order filled: %s %s %d@%.2f",
			orderID, order.Symbol, fill.Volume, fill.Price)
	} else {
		// Order pending (cannot fill immediately)
		order.Status = orspb.OrderStatus_PENDING
		r.matchEngine.mu.Lock()
		r.matchEngine.openOrders[orderID] = order
		r.matchEngine.mu.Unlock()

		log.Printf("[OrderRouter] Order pending: %s %s %s %d@%.2f",
			orderID, order.Symbol, order.Side, order.Volume, order.Price)
	}

	return nil
}

// tryMatchOpenOrders tries to match all open orders for a symbol
func (r *BacktestOrderRouter) tryMatchOpenOrders(symbol string) {
	r.matchEngine.mu.Lock()
	defer r.matchEngine.mu.Unlock()

	for orderID, order := range r.matchEngine.openOrders {
		if order.Symbol != symbol {
			continue
		}

		fill := r.matchEngine.TryMatchUnsafe(order)
		if fill != nil {
			// Order filled
			order.Filled = fill.Volume
			order.Status = orspb.OrderStatus_FILLED

			// Add to fill history
			r.mu.Lock()
			r.fillHistory = append(r.fillHistory, fill)
			r.mu.Unlock()

			// Send fill update
			r.sendOrderUpdate(&orspb.OrderUpdate{
				OrderId:       orderID,
				Symbol:        order.Symbol,
				Side:          order.Side,
				Status:        orspb.OrderStatus_FILLED,
				Price:         order.Price,
				Quantity:      int64(order.Volume),
				FilledQty:     int64(fill.Volume),
				RemainingQty:  0,
				AvgPrice:      fill.Price,
				LastFillPrice: fill.Price,
				LastFillQty:   int64(fill.Volume),
				Timestamp:     uint64(fill.Timestamp.UnixNano()),
				ErrorCode:     orspb.ErrorCode_SUCCESS,
			})

			// Remove from open orders
			delete(r.matchEngine.openOrders, orderID)

			log.Printf("[OrderRouter] Order filled: %s %s %d@%.2f",
				orderID, order.Symbol, fill.Volume, fill.Price)
		}
	}
}

// sendOrderUpdate sends order update to callback
func (r *BacktestOrderRouter) sendOrderUpdate(update *orspb.OrderUpdate) {
	if r.onOrderUpdate != nil {
		r.onOrderUpdate(update)
	}
}

// TryMatch tries to match an order (thread-safe)
func (e *SimpleMatchEngine) TryMatch(order *Order) *Fill {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.TryMatchUnsafe(order)
}

// TryMatchUnsafe tries to match an order (not thread-safe, caller must hold lock)
func (e *SimpleMatchEngine) TryMatchUnsafe(order *Order) *Fill {
	md, exists := e.currentMarketData[order.Symbol]
	if !exists || md == nil {
		return nil
	}

	// Check if we have valid market data
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return nil
	}

	var fillPrice float64
	canFill := false

	switch order.Side {
	case orspb.OrderSide_BUY:
		// Buy order: check if price >= ask price
		askPrice := md.AskPrice[0]
		askQty := md.AskQty[0]

		if askQty == 0 {
			return nil
		}

		if order.Price >= askPrice {
			// Can fill at ask price
			fillPrice = askPrice
			canFill = true

			// Apply slippage
			if e.slippageBps > 0 {
				fillPrice = fillPrice * (1 + e.slippageBps/10000.0)
			}
		}

	case orspb.OrderSide_SELL:
		// Sell order: check if price <= bid price
		bidPrice := md.BidPrice[0]
		bidQty := md.BidQty[0]

		if bidQty == 0 {
			return nil
		}

		if order.Price <= bidPrice {
			// Can fill at bid price
			fillPrice = bidPrice
			canFill = true

			// Apply slippage
			if e.slippageBps > 0 {
				fillPrice = fillPrice * (1 - e.slippageBps/10000.0)
			}
		}
	}

	if !canFill {
		return nil
	}

	// Create fill
	fill := &Fill{
		OrderID:   order.OrderID,
		Price:     fillPrice,
		Volume:    order.Volume,
		Timestamp: time.Now().Add(e.fillDelay),
	}

	return fill
}

// GetOrderHistory returns all order history
func (r *BacktestOrderRouter) GetOrderHistory() []*Order {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]*Order{}, r.orderHistory...)
}

// GetFillHistory returns all fill history
func (r *BacktestOrderRouter) GetFillHistory() []*Fill {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return append([]*Fill{}, r.fillHistory...)
}

// GetOpenOrders returns all open orders
func (r *BacktestOrderRouter) GetOpenOrders() []*Order {
	r.matchEngine.mu.RLock()
	defer r.matchEngine.mu.RUnlock()

	orders := make([]*Order, 0, len(r.matchEngine.openOrders))
	for _, order := range r.matchEngine.openOrders {
		orders = append(orders, order)
	}
	return orders
}

// CancelOrder cancels an open order
func (r *BacktestOrderRouter) CancelOrder(orderID string) error {
	r.matchEngine.mu.Lock()
	defer r.matchEngine.mu.Unlock()

	order, exists := r.matchEngine.openOrders[orderID]
	if !exists {
		return fmt.Errorf("order not found: %s", orderID)
	}

	// Remove from open orders
	delete(r.matchEngine.openOrders, orderID)

	// Update status
	order.Status = orspb.OrderStatus_CANCELED

	// Send cancel update
	r.sendOrderUpdate(&orspb.OrderUpdate{
		OrderId:   orderID,
		Symbol:    order.Symbol,
		Side:      order.Side,
		Status:    orspb.OrderStatus_CANCELED,
		Timestamp: uint64(time.Now().UnixNano()),
		ErrorCode: orspb.ErrorCode_SUCCESS,
	})

	log.Printf("[OrderRouter] Order cancelled: %s", orderID)
	return nil
}

// CancelAllOrders cancels all open orders
func (r *BacktestOrderRouter) CancelAllOrders() {
	r.matchEngine.mu.Lock()
	orderIDs := make([]string, 0, len(r.matchEngine.openOrders))
	for orderID := range r.matchEngine.openOrders {
		orderIDs = append(orderIDs, orderID)
	}
	r.matchEngine.mu.Unlock()

	for _, orderID := range orderIDs {
		r.CancelOrder(orderID)
	}
}

// GetStats returns order routing statistics
func (r *BacktestOrderRouter) GetStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	r.matchEngine.mu.RLock()
	openOrderCount := len(r.matchEngine.openOrders)
	r.matchEngine.mu.RUnlock()

	return map[string]interface{}{
		"total_orders":  len(r.orderHistory),
		"total_fills":   len(r.fillHistory),
		"open_orders":   openOrderCount,
		"filled_orders": len(r.fillHistory),
	}
}

// Implement ORSGateway gRPC service interface (for compatibility)
type BacktestORSService struct {
	orspb.UnimplementedORSGatewayServer
	router *BacktestOrderRouter
}

func (s *BacktestORSService) SendOrder(ctx context.Context, req *orspb.OrderRequest) (*orspb.OrderResponse, error) {
	err := s.router.SubmitOrder(req)
	if err != nil {
		return &orspb.OrderResponse{
			ErrorCode: orspb.ErrorCode_INTERNAL_ERROR,
			ErrorMsg:  err.Error(),
		}, nil
	}

	orderID := req.ClientOrderId
	if orderID == "" {
		orderID = fmt.Sprintf("ORD_%d", time.Now().UnixNano())
	}

	return &orspb.OrderResponse{
		ErrorCode:     orspb.ErrorCode_SUCCESS,
		OrderId:       orderID,
		ClientOrderId: req.ClientOrderId,
	}, nil
}

func (s *BacktestORSService) CancelOrder(ctx context.Context, req *orspb.CancelRequest) (*orspb.CancelResponse, error) {
	err := s.router.CancelOrder(req.OrderId)
	if err != nil {
		return &orspb.CancelResponse{
			ErrorCode: orspb.ErrorCode_ORDER_NOT_FOUND,
			ErrorMsg:  err.Error(),
		}, nil
	}

	return &orspb.CancelResponse{
		ErrorCode: orspb.ErrorCode_SUCCESS,
		OrderId:   req.OrderId,
	}, nil
}

func (s *BacktestORSService) QueryOrders(req *orspb.OrderQuery, stream orspb.ORSGateway_QueryOrdersServer) error {
	// Not implemented for backtest
	return nil
}

func (s *BacktestORSService) QueryPosition(req *orspb.PositionQuery, stream orspb.ORSGateway_QueryPositionServer) error {
	// Not implemented for backtest
	return nil
}

func (s *BacktestORSService) SendBatchOrders(stream orspb.ORSGateway_SendBatchOrdersServer) error {
	// Not implemented for backtest
	return nil
}
