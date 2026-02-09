package trader

import (
	"log"
	"sync"
	"time"

	"golang.org/x/net/websocket"

	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
)

// WebSocketMessage represents a message sent to websocket clients
type WebSocketMessage struct {
	Type      string      `json:"type"`      // "dashboard_update", "ping"
	Timestamp string      `json:"timestamp"` // ISO 8601 format
	Data      interface{} `json:"data"`
}

// DashboardWSUpdate contains all data for dashboard real-time update via WebSocket
type DashboardWSUpdate struct {
	Overview   *DashboardOverview               `json:"overview"`
	Strategies map[string]*StrategyRealtimeData `json:"strategies"`
	MarketData map[string]*MarketDataDetail     `json:"market_data"`
	Positions  []*PositionDetail                `json:"positions"`
	Orders     []*OrderDetail                   `json:"orders"`
}

// StrategyRealtimeData contains real-time strategy data including thresholds
type StrategyRealtimeData struct {
	ID         string             `json:"id"`
	Type       string             `json:"type"`
	Running    bool               `json:"running"`
	Active     bool               `json:"active"`
	Symbols    []string           `json:"symbols"`
	Indicators map[string]float64 `json:"indicators"` // Current indicator values
	Thresholds map[string]float64 `json:"thresholds"` // Strategy thresholds configuration

	// Condition status
	ConditionsMet bool `json:"conditions_met"`
	Eligible      bool `json:"eligible"`

	// P&L
	RealizedPnL   float64 `json:"realized_pnl"`
	UnrealizedPnL float64 `json:"unrealized_pnl"`
	Allocation    float64 `json:"allocation"`
}

// MarketDataDetail contains detailed latest market data for a symbol
type MarketDataDetail struct {
	Symbol       string  `json:"symbol"`
	Exchange     string  `json:"exchange"`
	LastPrice    float64 `json:"last_price"`
	BidPrice     float64 `json:"bid_price"`
	AskPrice     float64 `json:"ask_price"`
	BidVolume    int64   `json:"bid_volume"`
	AskVolume    int64   `json:"ask_volume"`
	Volume       int64   `json:"volume"`
	Turnover     float64 `json:"turnover"`
	OpenInterest int64   `json:"open_interest"`
	UpdateTime   string  `json:"update_time"`
}

// PositionDetail contains detailed position information for dashboard
type PositionDetail struct {
	StrategyID    string  `json:"strategy_id"`    // Which strategy holds this position
	Symbol        string  `json:"symbol"`         // Symbol
	Exchange      string  `json:"exchange"`       // Exchange
	Direction     string  `json:"direction"`      // LONG or SHORT
	Volume        int64   `json:"volume"`         // Position quantity
	AvgPrice      float64 `json:"avg_price"`      // Average open price
	CurrentPrice  float64 `json:"current_price"`  // Current market price
	UnrealizedPnL float64 `json:"unrealized_pnl"` // Unrealized P&L
	LegIndex      int     `json:"leg_index"`      // For pairwise strategies: 1 or 2, 0 for single-leg
}

// OrderDetail contains detailed order information for dashboard
type OrderDetail struct {
	StrategyID   string  `json:"strategy_id"`   // Which strategy created this order
	OrderID      string  `json:"order_id"`      // Order ID
	Symbol       string  `json:"symbol"`        // Symbol
	Exchange     string  `json:"exchange"`      // Exchange
	Side         string  `json:"side"`          // BUY or SELL
	OrderType    string  `json:"order_type"`    // LIMIT, MARKET, etc.
	Status       string  `json:"status"`        // NEW, FILLED, CANCELED, REJECTED
	Price        float64 `json:"price"`         // Order price
	Quantity     int64   `json:"quantity"`      // Order quantity
	FilledQty    int64   `json:"filled_qty"`    // Filled quantity
	AvgPrice     float64 `json:"avg_price"`     // Average fill price
	CreateTime   string  `json:"create_time"`   // Order creation time
	UpdateTime   string  `json:"update_time"`   // Last update time
	RejectReason string  `json:"reject_reason"` // Rejection reason if rejected
}

// WebSocketHub manages all websocket connections
type WebSocketHub struct {
	trader     *Trader
	clients    map[*websocket.Conn]bool
	broadcast  chan *WebSocketMessage
	register   chan *websocket.Conn
	unregister chan *websocket.Conn
	mu         sync.RWMutex
	running    bool
	stopCh     chan struct{}
}

// NewWebSocketHub creates a new WebSocket hub
func NewWebSocketHub(trader *Trader) *WebSocketHub {
	return &WebSocketHub{
		trader:     trader,
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan *WebSocketMessage, 100),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
		stopCh:     make(chan struct{}),
	}
}

// Start starts the WebSocket hub
func (h *WebSocketHub) Start() {
	h.mu.Lock()
	if h.running {
		h.mu.Unlock()
		return
	}
	h.running = true
	h.mu.Unlock()

	// Start connection manager
	go h.run()

	// Start periodic data broadcaster
	go h.periodicBroadcast()

	log.Printf("[WebSocket] Hub started")
}

// Stop stops the WebSocket hub
func (h *WebSocketHub) Stop() {
	h.mu.Lock()
	defer h.mu.Unlock()

	if !h.running {
		return
	}

	h.running = false
	close(h.stopCh)

	// Close all client connections
	for client := range h.clients {
		client.Close()
	}

	log.Printf("[WebSocket] Hub stopped")
}

// run manages client connections
func (h *WebSocketHub) run() {
	for {
		select {
		case <-h.stopCh:
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("[WebSocket] Client connected, total: %d", len(h.clients))

			// Send initial data immediately
			go h.sendInitialData(client)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()
			log.Printf("[WebSocket] Client disconnected, total: %d", len(h.clients))

		case message := <-h.broadcast:
			h.mu.RLock()
			for client := range h.clients {
				go func(c *websocket.Conn, msg *WebSocketMessage) {
					if err := websocket.JSON.Send(c, msg); err != nil {
						log.Printf("[WebSocket] Send error: %v", err)
						h.unregister <- c
					}
				}(client, message)
			}
			h.mu.RUnlock()
		}
	}
}

// periodicBroadcast sends data to all clients periodically
func (h *WebSocketHub) periodicBroadcast() {
	log.Printf("[WebSocket] periodicBroadcast() goroutine started")
	ticker := time.NewTicker(1 * time.Second) // 1秒推送一次
	defer ticker.Stop()

	for {
		select {
		case <-h.stopCh:
			return

		case <-ticker.C:
			h.mu.RLock()
			clientCount := len(h.clients)
			h.mu.RUnlock()

			log.Printf("[WebSocket] Periodic broadcast tick, clients: %d", clientCount)

			if clientCount == 0 {
				continue // No clients, skip data collection
			}

			log.Printf("[WebSocket] Calling collectDashboardData()...")
			data := h.collectDashboardData()
			if data != nil {
				h.broadcast <- &WebSocketMessage{
					Type:      "dashboard_update",
					Timestamp: time.Now().Format(time.RFC3339),
					Data:      data,
				}
			}
		}
	}
}

// sendInitialData sends initial dashboard data to a new client
func (h *WebSocketHub) sendInitialData(client *websocket.Conn) {
	data := h.collectDashboardData()
	if data != nil {
		msg := &WebSocketMessage{
			Type:      "dashboard_update",
			Timestamp: time.Now().Format(time.RFC3339),
			Data:      data,
		}
		if err := websocket.JSON.Send(client, msg); err != nil {
			log.Printf("[WebSocket] Failed to send initial data: %v", err)
			h.unregister <- client
		}
	}
}

// collectDashboardData collects all dashboard data
func (h *WebSocketHub) collectDashboardData() *DashboardWSUpdate {
	update := &DashboardWSUpdate{
		Strategies: make(map[string]*StrategyRealtimeData),
		MarketData: make(map[string]*MarketDataDetail),
		Positions:  make([]*PositionDetail, 0),
	}

	// Collect overview data
	update.Overview = h.collectOverviewData()

	// Collect strategy data with thresholds
	if h.trader.IsMultiStrategy() && h.trader.GetStrategyManager() != nil {
		mgr := h.trader.GetStrategyManager()
		mgr.ForEach(func(id string, strat strategy.Strategy) {
			stratData := h.collectStrategyData(id, strat)
			if stratData != nil {
				update.Strategies[id] = stratData
			}
		})
	}

	// Collect market data for subscribed symbols
	log.Printf("[WebSocket] About to call collectMarketData()")
	update.MarketData = h.collectMarketData()
	log.Printf("[WebSocket] collectMarketData() returned %d symbols", len(update.MarketData))

	// Collect positions
	update.Positions = h.collectPositions()

	// Collect orders
	update.Orders = h.collectOrders()

	return update
}

// collectOverviewData collects dashboard overview data
func (h *WebSocketHub) collectOverviewData() *DashboardOverview {
	overview := &DashboardOverview{
		MultiStrategy: h.trader.IsMultiStrategy(),
		Mode:          h.trader.Config.System.Mode,
		Timestamp:     time.Now().Format(time.RFC3339),
		Strategies:    make([]StrategyOverviewItem, 0),
	}

	if h.trader.IsMultiStrategy() && h.trader.GetStrategyManager() != nil {
		mgr := h.trader.GetStrategyManager()
		status := mgr.GetStatus()

		overview.TotalStrategies = status.TotalStrategies
		overview.ActiveStrategies = status.ActiveStrategies
		overview.RunningStrategies = status.RunningStrategies

		// Calculate totals from strategies
		var totalRealized, totalUnrealized float64

		// Collect individual strategy status
		mgr.ForEach(func(id string, strat strategy.Strategy) {
			if accessor, ok := strat.(strategy.BaseStrategyAccessor); ok {
				if base := accessor.GetBaseStrategy(); base != nil {
					totalRealized += base.PNL.RealizedPnL
					totalUnrealized += base.PNL.UnrealizedPnL

					overview.Strategies = append(overview.Strategies, StrategyOverviewItem{
						ID:            id,
						Type:          strat.GetType(),
						Running:       base.IsRunning(),
						Active:        base.ControlState.IsActive(),
						ConditionsMet: base.ControlState.ConditionsMet,
						Eligible:      base.ControlState.Eligible,
						RealizedPnL:   base.PNL.RealizedPnL,
						UnrealizedPnL: base.PNL.UnrealizedPnL,
						Allocation:    base.Config.Allocation,
						Symbols:       base.Config.Symbols,
					})
				}
			}
		})

		overview.TotalRealizedPnL = totalRealized
		overview.TotalUnrealizedPnL = totalUnrealized
		overview.TotalPnL = totalRealized + totalUnrealized
	}

	return overview
}

// collectStrategyData collects strategy data including thresholds
func (h *WebSocketHub) collectStrategyData(id string, strat strategy.Strategy) *StrategyRealtimeData {
	accessor, ok := strat.(strategy.BaseStrategyAccessor)
	if !ok {
		return nil
	}

	base := accessor.GetBaseStrategy()
	if base == nil {
		return nil
	}

	data := &StrategyRealtimeData{
		ID:            id,
		Type:          strat.GetType(),
		Running:       base.IsRunning(),
		Active:        base.ControlState.IsActive(),
		Symbols:       base.Config.Symbols,
		Indicators:    make(map[string]float64),
		Thresholds:    make(map[string]float64),
		ConditionsMet: base.ControlState.ConditionsMet,
		Eligible:      base.ControlState.Eligible,
		RealizedPnL:   base.PNL.RealizedPnL,
		UnrealizedPnL: base.PNL.UnrealizedPnL,
		Allocation:    base.Config.Allocation,
	}

	// Collect indicator values from multiple sources
	// 1. SharedIndicators (共享指标池)
	if base.SharedIndicators != nil {
		for key, value := range base.SharedIndicators.GetAllValues() {
			data.Indicators[key] = value
		}
	}
	// 2. PrivateIndicators (私有指标)
	if base.PrivateIndicators != nil {
		for key, value := range base.PrivateIndicators.GetAllValues() {
			data.Indicators[key] = value
		}
	}
	// 3. ControlState.Indicators (策略计算的实时指标)
	if base.ControlState != nil && base.ControlState.Indicators != nil {
		for key, value := range base.ControlState.Indicators {
			data.Indicators[key] = value
		}
	}

	// Collect thresholds from strategy configuration
	data.Thresholds = h.extractThresholds(base)

	return data
}

// extractThresholds extracts threshold values from strategy configuration
func (h *WebSocketHub) extractThresholds(base *strategy.BaseStrategy) map[string]float64 {
	thresholds := make(map[string]float64)
	if base.Config == nil {
		return thresholds
	}

	// Extract thresholds from parameters
	params := base.Config.Parameters

	// PairwiseArbStrategy thresholds
	if entry, ok := params["entry_zscore"].(float64); ok {
		thresholds["entry_zscore"] = entry
	}
	if exit, ok := params["exit_zscore"].(float64); ok {
		thresholds["exit_zscore"] = exit
	}
	if minCorr, ok := params["min_correlation"].(float64); ok {
		thresholds["min_correlation"] = minCorr
	}

	// PassiveStrategy thresholds
	if minSpread, ok := params["min_spread"].(float64); ok {
		thresholds["min_spread"] = minSpread
	}
	if spreadMult, ok := params["spread_multiplier"].(float64); ok {
		thresholds["spread_multiplier"] = spreadMult
	}

	// Generic thresholds
	if maxPos, ok := params["max_position_size"].(float64); ok {
		thresholds["max_position_size"] = maxPos
	}

	return thresholds
}

// collectMarketData collects latest market data for subscribed symbols
func (h *WebSocketHub) collectMarketData() map[string]*MarketDataDetail {
	marketData := make(map[string]*MarketDataDetail)

	// Get all subscribed symbols and their latest market data
	if h.trader.IsMultiStrategy() && h.trader.GetStrategyManager() != nil {
		mgr := h.trader.GetStrategyManager()
		strategyCount := 0
		withMarketData := 0

		mgr.ForEach(func(id string, strat strategy.Strategy) {
			strategyCount++
			if accessor, ok := strat.(strategy.BaseStrategyAccessor); ok {
				base := accessor.GetBaseStrategy()
				if base != nil {
					base.MarketDataMu.RLock()
					hasData := base.LastMarketData != nil && len(base.LastMarketData) > 0
					base.MarketDataMu.RUnlock()

					if hasData {
						withMarketData++
						base.MarketDataMu.RLock()
						// Iterate through all market data in the map
						for symbol, md := range base.LastMarketData {
							// Only create snapshot if we haven't already for this symbol
							if _, exists := marketData[symbol]; !exists {
								snapshot := &MarketDataDetail{
									Symbol:       symbol,
									Exchange:     md.Exchange,
									LastPrice:    md.LastPrice,
									Volume:       int64(md.TotalVolume),
									Turnover:     md.Turnover,
									OpenInterest: 0, // Not available in protobuf
									UpdateTime:   time.Unix(0, int64(md.Timestamp)).Format(time.RFC3339),
								}
								// Set bid/ask if available
								if len(md.BidPrice) > 0 {
									snapshot.BidPrice = md.BidPrice[0]
								}
								if len(md.AskPrice) > 0 {
									snapshot.AskPrice = md.AskPrice[0]
								}
								if len(md.BidQty) > 0 {
									snapshot.BidVolume = int64(md.BidQty[0])
								}
								if len(md.AskQty) > 0 {
									snapshot.AskVolume = int64(md.AskQty[0])
								}
								marketData[symbol] = snapshot
								log.Printf("[WebSocket] Collected market data for %s: LastPrice=%.2f", symbol, md.LastPrice)
							}
						}
						base.MarketDataMu.RUnlock()
					} else {
						log.Printf("[WebSocket] Strategy %s has empty LastMarketData", id)
					}
				} else {
					log.Printf("[WebSocket] Strategy %s has nil BaseStrategy", id)
				}
			}
		})

		log.Printf("[WebSocket] collectMarketData: checked %d strategies, %d have market data, collected %d symbols",
			strategyCount, withMarketData, len(marketData))
	}

	return marketData
}

// collectPositions collects current positions from strategies
func (h *WebSocketHub) collectPositions() []*PositionDetail {
	positions := make([]*PositionDetail, 0)

	// Collect positions from strategies
	if h.trader.IsMultiStrategy() && h.trader.GetStrategyManager() != nil {
		mgr := h.trader.GetStrategyManager()
		mgr.ForEach(func(id string, strat strategy.Strategy) {
			stratType := strat.GetType()

			// Handle PairwiseArbStrategy (has two legs)
			if stratType == "pairwise_arb" {
				positions = append(positions, h.collectPairwisePositions(id, strat)...)
			} else {
				// Handle single-leg strategies
				positions = append(positions, h.collectSingleLegPositions(id, strat)...)
			}
		})
	}

	return positions
}

// collectPairwisePositions collects positions for pairwise arbitrage strategy
func (h *WebSocketHub) collectPairwisePositions(strategyID string, strat strategy.Strategy) []*PositionDetail {
	positions := make([]*PositionDetail, 0, 2)

	accessor, ok := strat.(strategy.BaseStrategyAccessor)
	if !ok {
		return positions
	}

	base := accessor.GetBaseStrategy()
	if base == nil || base.Config == nil || len(base.Config.Symbols) < 2 {
		return positions
	}

	// Get exchange
	exchange := "SHFE"
	if len(base.Config.Exchanges) > 0 {
		exchange = base.Config.Exchanges[0]
	}

	// Get leg positions from indicators (stored by PairwiseArb)
	leg1Pos := int64(0)
	leg2Pos := int64(0)
	if base.ControlState != nil && base.ControlState.Indicators != nil {
		if val, ok := base.ControlState.Indicators["leg1_position"]; ok {
			leg1Pos = int64(val)
		}
		if val, ok := base.ControlState.Indicators["leg2_position"]; ok {
			leg2Pos = int64(val)
		}
	}

	// Get current prices from LastMarketData
	price1 := 0.0
	price2 := 0.0
	base.MarketDataMu.RLock()
	if base.LastMarketData != nil {
		if md1, ok := base.LastMarketData[base.Config.Symbols[0]]; ok {
			price1 = md1.LastPrice
		}
		if md2, ok := base.LastMarketData[base.Config.Symbols[1]]; ok {
			price2 = md2.LastPrice
		}
	}
	base.MarketDataMu.RUnlock()

	// Leg 1
	if leg1Pos != 0 {
		direction := "LONG"
		volume := leg1Pos
		if leg1Pos < 0 {
			direction = "SHORT"
			volume = -leg1Pos
		}

		avgPrice := 0.0
		if base.ControlState != nil && base.ControlState.Indicators != nil {
			if val, ok := base.ControlState.Indicators["leg1_price"]; ok {
				avgPrice = val
			}
		}

		unrealizedPnL := 0.0
		if avgPrice > 0 && price1 > 0 {
			if leg1Pos > 0 {
				unrealizedPnL = float64(leg1Pos) * (price1 - avgPrice)
			} else {
				unrealizedPnL = float64(-leg1Pos) * (avgPrice - price1)
			}
		}

		positions = append(positions, &PositionDetail{
			StrategyID:    strategyID,
			Symbol:        base.Config.Symbols[0],
			Exchange:      exchange,
			Direction:     direction,
			Volume:        volume,
			AvgPrice:      avgPrice,
			CurrentPrice:  price1,
			UnrealizedPnL: unrealizedPnL,
			LegIndex:      1,
		})
	}

	// Leg 2
	if leg2Pos != 0 {
		direction := "LONG"
		volume := leg2Pos
		if leg2Pos < 0 {
			direction = "SHORT"
			volume = -leg2Pos
		}

		avgPrice := 0.0
		if base.ControlState != nil && base.ControlState.Indicators != nil {
			if val, ok := base.ControlState.Indicators["leg2_price"]; ok {
				avgPrice = val
			}
		}

		unrealizedPnL := 0.0
		if avgPrice > 0 && price2 > 0 {
			if leg2Pos > 0 {
				unrealizedPnL = float64(leg2Pos) * (price2 - avgPrice)
			} else {
				unrealizedPnL = float64(-leg2Pos) * (avgPrice - price2)
			}
		}

		positions = append(positions, &PositionDetail{
			StrategyID:    strategyID,
			Symbol:        base.Config.Symbols[1],
			Exchange:      exchange,
			Direction:     direction,
			Volume:        volume,
			AvgPrice:      avgPrice,
			CurrentPrice:  price2,
			UnrealizedPnL: unrealizedPnL,
			LegIndex:      2,
		})
	}

	return positions
}

// collectSingleLegPositions collects positions for single-leg strategies
func (h *WebSocketHub) collectSingleLegPositions(strategyID string, strat strategy.Strategy) []*PositionDetail {
	positions := make([]*PositionDetail, 0)

	accessor, ok := strat.(strategy.BaseStrategyAccessor)
	if !ok {
		return positions
	}

	base := accessor.GetBaseStrategy()
	if base == nil || base.EstimatedPosition == nil || base.EstimatedPosition.IsFlat() {
		return positions
	}

	// Get exchange
	exchange := "SHFE"
	if len(base.Config.Exchanges) > 0 {
		exchange = base.Config.Exchanges[0]
	}

	symbol := ""
	if len(base.Config.Symbols) > 0 {
		symbol = base.Config.Symbols[0]
	}

	// Get current price
	currentPrice := 0.0
	base.MarketDataMu.RLock()
	if base.LastMarketData != nil {
		if md, ok := base.LastMarketData[symbol]; ok {
			currentPrice = md.LastPrice
		}
	}
	base.MarketDataMu.RUnlock()

	// Determine direction and collect position
	if base.EstimatedPosition.IsLong() {
		positions = append(positions, &PositionDetail{
			StrategyID:    strategyID,
			Symbol:        symbol,
			Exchange:      exchange,
			Direction:     "LONG",
			Volume:        base.EstimatedPosition.LongQty,
			AvgPrice:      base.EstimatedPosition.AvgLongPrice,
			CurrentPrice:  currentPrice,
			UnrealizedPnL: base.PNL.UnrealizedPnL,
			LegIndex:      0,
		})
	}

	if base.EstimatedPosition.IsShort() {
		positions = append(positions, &PositionDetail{
			StrategyID:    strategyID,
			Symbol:        symbol,
			Exchange:      exchange,
			Direction:     "SHORT",
			Volume:        base.EstimatedPosition.ShortQty,
			AvgPrice:      base.EstimatedPosition.AvgShortPrice,
			CurrentPrice:  currentPrice,
			UnrealizedPnL: base.PNL.UnrealizedPnL,
			LegIndex:      0,
		})
	}

	return positions
}

// collectOrders collects all orders from all strategies
func (h *WebSocketHub) collectOrders() []*OrderDetail {
	orders := make([]*OrderDetail, 0)

	if !h.trader.IsMultiStrategy() || h.trader.GetStrategyManager() == nil {
		log.Printf("[WebSocket] collectOrders: not multi-strategy or no manager")
		return orders
	}

	mgr := h.trader.GetStrategyManager()
	orderCount := 0
	mgr.ForEach(func(id string, strat strategy.Strategy) {
		// Get base strategy to access Orders map
		accessor, ok := strat.(strategy.BaseStrategyAccessor)
		if !ok {
			log.Printf("[WebSocket] Strategy %s does not implement BaseStrategyAccessor", id)
			return
		}

		base := accessor.GetBaseStrategy()
		if base == nil {
			log.Printf("[WebSocket] Strategy %s has nil base", id)
			return
		}

		if base.Orders == nil {
			log.Printf("[WebSocket] Strategy %s has nil Orders map", id)
			return
		}

		stratOrderCount := len(base.Orders)
		orderCount += stratOrderCount
		log.Printf("[WebSocket] Strategy %s has %d orders", id, stratOrderCount)

		// Iterate through all orders for this strategy
		orderIdx := 0
		for orderID, orderUpdate := range base.Orders {
			// Debug: log first 3 orders
			if orderIdx < 3 {
				log.Printf("[WebSocket] DEBUG Order %s: Status=%d, Side=%d, Symbol=%s", orderID, orderUpdate.Status, orderUpdate.Side, orderUpdate.Symbol)
			}
			orderIdx++

			// Map order side
			var side string
			switch orderUpdate.Side {
			case 1: // BUY
				side = "BUY"
			case 2: // SELL
				side = "SELL"
			default:
				side = "UNKNOWN"
			}

			// Map order status (from protobuf OrderStatus enum)
			var status string
			if orderIdx < 5 {
				log.Printf("[WebSocket] Order %s raw status value: %d", orderID, orderUpdate.Status)
			}
			switch orderUpdate.Status {
			case 0: // STATUS_UNKNOWN - 订单刚发送，等待确认
				status = "PENDING"
			case 1: // PENDING
				status = "PENDING"
			case 2: // SUBMITTED
				status = "SUBMITTED"
			case 3: // ACCEPTED
				status = "ACCEPTED"
			case 4: // PARTIALLY_FILLED
				status = "PARTIAL"
			case 5: // FILLED
				status = "FILLED"
			case 6: // CANCELING
				status = "CANCELING"
			case 7: // CANCELED
				status = "CANCELED"
			case 8: // REJECTED
				status = "REJECTED"
			case 9: // EXPIRED
				status = "EXPIRED"
			default:
				status = "PENDING" // 默认显示为 PENDING 而非 UNKNOWN
			}
			if orderIdx < 5 {
				log.Printf("[WebSocket] Order %s mapped to status: %s", orderID, status)
			}

			// Map exchange enum to string
			var exchange string
			switch orderUpdate.Exchange {
			case 1:
				exchange = "SHFE"
			case 2:
				exchange = "DCE"
			case 3:
				exchange = "CZCE"
			case 4:
				exchange = "CFFEX"
			case 5:
				exchange = "INE"
			default:
				exchange = "UNKNOWN"
			}

			// Convert timestamps to readable format
			createTime := time.Unix(0, int64(orderUpdate.Timestamp)).Format("15:04:05.000")
			updateTime := createTime // Use same timestamp for now

			orders = append(orders, &OrderDetail{
				StrategyID:   id,
				OrderID:      orderID,
				Symbol:       orderUpdate.Symbol,
				Exchange:     exchange,
				Side:         side,
				OrderType:    "LIMIT", // Default to LIMIT
				Status:       status,
				Price:        orderUpdate.Price,
				Quantity:     orderUpdate.Quantity,
				FilledQty:    orderUpdate.FilledQty,
				AvgPrice:     orderUpdate.AvgPrice,
				CreateTime:   createTime,
				UpdateTime:   updateTime,
				RejectReason: orderUpdate.ErrorMsg,
			})
		}
	})

	log.Printf("[WebSocket] collectOrders: collected %d orders from %d total", len(orders), orderCount)
	return orders
}

// HandleWebSocket handles websocket connections
func (h *WebSocketHub) HandleWebSocket(ws *websocket.Conn) {
	// Register client
	h.register <- ws

	// Send heartbeat
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-h.stopCh:
				return
			case <-ticker.C:
				if err := websocket.JSON.Send(ws, &WebSocketMessage{
					Type:      "ping",
					Timestamp: time.Now().Format(time.RFC3339),
				}); err != nil {
					h.unregister <- ws
					return
				}
			}
		}
	}()

	// Read messages (for potential client commands and pong)
	for {
		var msg map[string]interface{}
		if err := websocket.JSON.Receive(ws, &msg); err != nil {
			h.unregister <- ws
			break
		}

		// Handle client messages
		if msgType, ok := msg["type"].(string); ok {
			if msgType == "pong" {
				// Heartbeat response, do nothing
				continue
			}
		}
	}
}
