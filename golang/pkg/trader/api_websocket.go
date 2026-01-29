package trader

import (
	"log"
	"sync"
	"time"

	"golang.org/x/net/websocket"

	"github.com/yourusername/quantlink-trade-system/pkg/client"
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
	Overview   *DashboardOverview                     `json:"overview"`
	Strategies map[string]*StrategyRealtimeData       `json:"strategies"`
	MarketData map[string]*MarketDataDetail           `json:"market_data"`
	Positions  map[string][]client.PositionInfo       `json:"positions"`
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
		Positions:  make(map[string][]client.PositionInfo),
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

	// Collect indicator values
	if base.SharedIndicators != nil {
		for key, value := range base.SharedIndicators.GetAllValues() {
			data.Indicators[key] = value
		}
	}
	if base.PrivateIndicators != nil {
		for key, value := range base.PrivateIndicators.GetAllValues() {
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
					if base.LastMarketData != nil && len(base.LastMarketData) > 0 {
						withMarketData++
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
func (h *WebSocketHub) collectPositions() map[string][]client.PositionInfo {
	positions := make(map[string][]client.PositionInfo)

	// Collect positions from strategies
	if h.trader.IsMultiStrategy() && h.trader.GetStrategyManager() != nil {
		mgr := h.trader.GetStrategyManager()
		mgr.ForEach(func(id string, strat strategy.Strategy) {
			if accessor, ok := strat.(strategy.BaseStrategyAccessor); ok {
				base := accessor.GetBaseStrategy()
				if base != nil && base.Position != nil && !base.Position.IsFlat() {
					// Get exchange from config (use first exchange if available)
					exchange := "SHFE" // Default
					if len(base.Config.Exchanges) > 0 {
						exchange = base.Config.Exchanges[0]
					}

					// Convert to PositionInfo
					if _, exists := positions[exchange]; !exists {
						positions[exchange] = make([]client.PositionInfo, 0)
					}

					// Determine direction
					direction := "LONG"
					volume := base.Position.LongQty
					avgPrice := base.Position.AvgLongPrice
					if base.Position.IsShort() {
						direction = "SHORT"
						volume = base.Position.ShortQty
						avgPrice = base.Position.AvgShortPrice
					}

					symbol := ""
					if len(base.Config.Symbols) > 0 {
						symbol = base.Config.Symbols[0]
					}

					positions[exchange] = append(positions[exchange], client.PositionInfo{
						Symbol:         symbol,
						Exchange:       exchange,
						Direction:      direction,
						Volume:         volume,
						AvgPrice:       avgPrice,
						PositionProfit: base.PNL.UnrealizedPnL,
					})
				}
			}
		})
	}

	return positions
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
