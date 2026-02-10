package trader

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/websocket"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
)

// APIServer provides HTTP REST API for trader control
// 对应 tbsrc 信号控制的现代化替代方案
type APIServer struct {
	trader    *Trader
	server    *http.Server
	wsHub     *WebSocketHub // WebSocket hub for real-time data push
	mu        sync.RWMutex
	commandMu sync.Mutex // 命令互斥锁，防止并发激活/停止
	running   bool
}

// APIResponse is the standard API response format
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// StrategyStatusResponse contains strategy status information
type StrategyStatusResponse struct {
	StrategyID string                 `json:"strategy_id"`
	Running    bool                   `json:"running"`
	Active     bool                   `json:"active"`
	Mode       string                 `json:"mode"`
	Symbols    []string               `json:"symbols"`
	Position   interface{}            `json:"estimated_position"` // Estimated position from order fills
	PNL        interface{}            `json:"pnl"`
	Risk       interface{}            `json:"risk"`
	Uptime     string                 `json:"uptime"`
	Details    map[string]interface{} `json:"details"`

	// Trading condition state (new fields)
	ConditionsMet   bool                   `json:"conditions_met"`    // Are market conditions satisfied?
	Eligible        bool                   `json:"eligible"`          // Ready to activate? (conditions met but not active)
	EligibleReason  string                 `json:"eligible_reason"`   // Why eligible/not eligible
	SignalStrength  float64                `json:"signal_strength"`   // Current signal strength (e.g., z-score)
	LastSignalTime  string                 `json:"last_signal_time"`  // When last signal was generated
	Indicators      map[string]float64     `json:"indicators"`        // All indicator values for display

	// Legs information for pair trading strategies (optional)
	Legs []LegInfo `json:"legs,omitempty"` // Detailed info for each leg
}

// LegInfo contains information for one leg of a pair trading strategy
type LegInfo struct {
	Symbol   string  `json:"symbol"`    // Symbol name (e.g., "ag2502")
	Price    float64 `json:"price"`     // Current price
	Position int64   `json:"position"`  // Current position
	Side     string  `json:"side"`      // "long" or "short" or "flat"
}

// loggingMiddleware logs all incoming requests
func (a *APIServer) loggingMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("[API] Incoming request: %s %s", r.Method, r.URL.Path)
		next(w, r)
	}
}

// NewAPIServer creates a new API server
func NewAPIServer(trader *Trader, port int) *APIServer {
	api := &APIServer{
		trader:  trader,
		running: false,
	}

	// Create WebSocket hub
	api.wsHub = NewWebSocketHub(trader)

	mux := http.NewServeMux()

	// Register endpoints with CORS
	mux.HandleFunc("/api/v1/strategy/activate", api.corsMiddleware(api.handleActivate))
	mux.HandleFunc("/api/v1/strategy/deactivate", api.corsMiddleware(api.handleDeactivate))
	mux.HandleFunc("/api/v1/strategy/status", api.corsMiddleware(api.handleStatus))
	mux.HandleFunc("/api/v1/trader/status", api.corsMiddleware(api.handleTraderStatus))
	mux.HandleFunc("/api/v1/health", api.corsMiddleware(api.handleHealth))
	mux.HandleFunc("/api/v1/test-ping", api.loggingMiddleware(api.handleTestPing))
	mux.HandleFunc("/api/v1/test-market-data", api.loggingMiddleware(api.handleTestMarketData))

	// Model hot reload endpoints
	mux.HandleFunc("/api/v1/model/reload", api.corsMiddleware(api.handleModelReload))
	mux.HandleFunc("/api/v1/model/status", api.corsMiddleware(api.handleModelStatus))
	mux.HandleFunc("/api/v1/model/history", api.corsMiddleware(api.handleModelHistory))

	// Position query endpoints
	mux.HandleFunc("/api/v1/positions", api.corsMiddleware(api.handlePositions))
	mux.HandleFunc("/api/v1/positions/summary", api.corsMiddleware(api.handlePositionsSummary))

	// Multi-strategy management endpoints (P2-12.2)
	mux.HandleFunc("/api/v1/dashboard/overview", api.corsMiddleware(api.handleDashboardOverview))
	mux.HandleFunc("/api/v1/strategies", api.corsMiddleware(api.handleStrategies))
	mux.HandleFunc("/api/v1/strategies/", api.corsMiddleware(api.handleStrategyByID))
	mux.HandleFunc("/api/v1/indicators/realtime", api.corsMiddleware(api.handleRealtimeIndicators))

	// WebSocket endpoint for real-time dashboard
	mux.Handle("/api/v1/ws/dashboard", websocket.Handler(api.wsHub.HandleWebSocket))

	// Serve dashboard HTML (static file)
	mux.HandleFunc("/dashboard", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "golang/web/dashboard.html")
	})

	api.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", port),
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	return api
}

// Start starts the API server
func (a *APIServer) Start() error {
	a.mu.Lock()
	if a.running {
		a.mu.Unlock()
		return fmt.Errorf("API server already running")
	}
	a.running = true
	a.mu.Unlock()

	// Start WebSocket hub
	a.wsHub.Start()

	log.Printf("[API] Starting HTTP API server on %s", a.server.Addr)
	log.Printf("[API] DEBUG: Test endpoints registered: /api/v1/test/ping and /api/v1/test/market-data")
	log.Printf("[API] WebSocket endpoint: ws://%s/api/v1/ws/dashboard", a.server.Addr)

	go func() {
		if err := a.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[API] Error starting server: %v", err)
		}
	}()

	log.Println("[API] ✓ HTTP API server started")
	return nil
}

// Stop stops the API server
func (a *APIServer) Stop() error {
	a.mu.Lock()
	if !a.running {
		a.mu.Unlock()
		return nil
	}
	a.running = false
	a.mu.Unlock()

	log.Println("[API] Stopping HTTP API server...")

	// Stop WebSocket hub first
	a.wsHub.Stop()

	if err := a.server.Close(); err != nil {
		return fmt.Errorf("failed to stop server: %w", err)
	}

	log.Println("[API] ✓ HTTP API server stopped")
	return nil
}

// IsRunning returns whether the API server is running
func (a *APIServer) IsRunning() bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.running
}

// handleActivate handles POST /api/v1/strategy/activate
// 对应 Unix 信号 SIGUSR1 / startTrade.sh
func (a *APIServer) handleActivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 防止并发激活（多人/多次点击）
	a.commandMu.Lock()
	defer a.commandMu.Unlock()

	log.Println("[API] ════════════════════════════════════════════════════════════")
	log.Println("[API] Received HTTP request: Activating strategy")
	log.Println("[API] ════════════════════════════════════════════════════════════")

	// Get BaseStrategy through type assertion
	baseStrat := a.getBaseStrategy()
	if baseStrat == nil {
		a.sendError(w, http.StatusInternalServerError, "Failed to access strategy control state")
		return
	}

	// Reset control state (same as SIGUSR1 handler)
	baseStrat.ControlState.ExitRequested = false
	baseStrat.ControlState.CancelPending = false
	baseStrat.ControlState.FlattenMode = false
	// 重置 RunState 以便可以重新 Start
	if baseStrat.ControlState.RunState == strategy.StrategyRunStateStopped ||
		baseStrat.ControlState.RunState == strategy.StrategyRunStateFlattening {
		baseStrat.ControlState.RunState = strategy.StrategyRunStateActive
	}
	baseStrat.ControlState.Activate()

	// Start strategy if not running
	if !a.trader.Strategy.IsRunning() {
		if err := a.trader.Strategy.Start(); err != nil {
			log.Printf("[API] Error starting strategy: %v", err)
			a.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start strategy: %v", err))
			return
		}
		log.Println("[API] ✓ Strategy activated and trading")
	} else {
		log.Println("[API] ✓ Strategy already running, re-activated")
	}

	a.sendSuccess(w, "Strategy activated successfully", map[string]interface{}{
		"strategy_id": a.trader.Config.System.StrategyID,
		"active":      true,
		"running":     true,
	})
}

// handleDeactivate handles POST /api/v1/strategy/deactivate
// 对应 Unix 信号 SIGUSR2 / stopTrade.sh
func (a *APIServer) handleDeactivate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 防止并发停止（多人/多次点击）
	a.commandMu.Lock()
	defer a.commandMu.Unlock()

	log.Println("[API] ════════════════════════════════════════════════════════════")
	log.Println("[API] Received HTTP request: Deactivating strategy (squareoff)")
	log.Println("[API] ════════════════════════════════════════════════════════════")

	// Get BaseStrategy through type assertion
	baseStrat := a.getBaseStrategy()
	if baseStrat == nil {
		a.sendError(w, http.StatusInternalServerError, "Failed to access strategy control state")
		return
	}

	// Trigger flatten mode (same as SIGUSR2 handler)
	baseStrat.TriggerFlatten(strategy.FlattenReasonManual, false)
	baseStrat.ControlState.Deactivate()

	log.Println("[API] ✓ Strategy deactivated, positions being closed")
	log.Println("[API] Strategy will stop trading but process continues running")

	a.sendSuccess(w, "Strategy deactivated successfully (squareoff initiated)", map[string]interface{}{
		"strategy_id": a.trader.Config.System.StrategyID,
		"active":      false,
		"flatten":     true,
	})
}

// handleStatus handles GET /api/v1/strategy/status
// Returns detailed strategy status
func (a *APIServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get BaseStrategy through type assertion
	baseStrat := a.getBaseStrategy()
	if baseStrat == nil {
		a.sendError(w, http.StatusInternalServerError, "Failed to access strategy control state")
		return
	}

	// Format last signal time
	lastSignalTime := ""
	if !baseStrat.ControlState.LastSignalTime.IsZero() {
		lastSignalTime = baseStrat.ControlState.LastSignalTime.Format("15:04:05")
	}

	status := &StrategyStatusResponse{
		StrategyID: a.trader.Config.System.StrategyID,
		Running:    a.trader.Strategy.IsRunning(),
		Active:     baseStrat.ControlState.IsActive(),
		Mode:       a.trader.Config.System.Mode,
		Symbols:    a.trader.Config.Strategy.Symbols,
		Position:   a.trader.Strategy.GetEstimatedPosition(),
		PNL:        a.trader.Strategy.GetPNL(),
		Risk:       a.trader.Strategy.GetRiskMetrics(),
		Details: map[string]interface{}{
			"flatten_mode":    baseStrat.ControlState.FlattenMode,
			"exit_requested":  baseStrat.ControlState.ExitRequested,
			"cancel_pending":  baseStrat.ControlState.CancelPending,
			"strategy_type":   a.trader.Config.Strategy.Type,
			"max_position":    a.trader.Config.Strategy.MaxPositionSize,
			"max_exposure":    a.trader.Config.Strategy.MaxExposure,
		},
		// Condition state (new)
		ConditionsMet:   baseStrat.ControlState.ConditionsMet,
		Eligible:        baseStrat.ControlState.Eligible,
		EligibleReason:  baseStrat.ControlState.EligibleReason,
		SignalStrength:  baseStrat.ControlState.SignalStrength,
		LastSignalTime:  lastSignalTime,
		Indicators:      baseStrat.ControlState.Indicators,
	}

	// Get legs info for pair trading strategies
	if legsProvider, ok := a.trader.Strategy.(interface {
		GetLegsInfo() []map[string]interface{}
	}); ok {
		legsData := legsProvider.GetLegsInfo()
		legs := make([]LegInfo, 0, len(legsData))
		for _, legData := range legsData {
			leg := LegInfo{}
			if sym, ok := legData["symbol"].(string); ok {
				leg.Symbol = sym
			}
			if price, ok := legData["price"].(float64); ok {
				leg.Price = price
			}
			if pos, ok := legData["position"].(int64); ok {
				leg.Position = pos
			}
			if side, ok := legData["side"].(string); ok {
				leg.Side = side
			}
			legs = append(legs, leg)
		}
		status.Legs = legs
	}

	// Set uptime if available (could be calculated from Status field in future)
	if a.trader.Strategy.IsRunning() {
		status.Uptime = "running"
	}

	a.sendSuccess(w, "Strategy status retrieved", status)
}

// handleTraderStatus handles GET /api/v1/trader/status
// Returns overall trader status
func (a *APIServer) handleTraderStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	status := a.trader.GetStatus()
	a.sendSuccess(w, "Trader status retrieved", status)
}

// handleHealth handles GET /api/v1/health
// Returns health check
func (a *APIServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	health := map[string]interface{}{
		"status":       "ok",
		"trader":       a.trader.IsRunning(),
		"api_server":   a.IsRunning(),
		"strategy_id":  a.trader.Config.System.StrategyID,
		"mode":         a.trader.Config.System.Mode,
		"test_routes_registered": true,  // DEBUG: confirm new binary
	}

	a.sendSuccess(w, "Healthy", health)
}

// sendSuccess sends a success response
func (a *APIServer) sendSuccess(w http.ResponseWriter, message string, data interface{}) {
	response := APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	}
	a.sendJSON(w, http.StatusOK, response)
}

// sendError sends an error response
func (a *APIServer) sendError(w http.ResponseWriter, statusCode int, errorMsg string) {
	response := APIResponse{
		Success: false,
		Error:   errorMsg,
	}
	a.sendJSON(w, statusCode, response)
}

// sendJSON sends a JSON response
func (a *APIServer) sendJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("[API] Error encoding response: %v", err)
	}
}

// getBaseStrategy is a helper to get the BaseStrategy
func (a *APIServer) getBaseStrategy() *strategy.BaseStrategy {
	if a.trader.Strategy == nil {
		log.Printf("[API] Error: Strategy is nil")
		return nil
	}
	return a.trader.Strategy.GetBaseStrategy()
}

// handleTestPing handles GET /api/v1/test/ping
// Simple test endpoint
func (a *APIServer) handleTestPing(w http.ResponseWriter, r *http.Request) {
	a.sendSuccess(w, "Pong", map[string]string{"status": "ok"})
}

// handleTestMarketData handles POST /api/v1/test/market-data
// 用于测试环境模拟行情数据
// ⚠️ SAFETY: Only available in simulation/backtest modes, disabled in live mode
func (a *APIServer) handleTestMarketData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// SAFETY CHECK: Disable test data injection in live mode
	if a.trader.Config.System.Mode == "live" {
		a.sendError(w, http.StatusForbidden, "Test market data endpoint is disabled in live mode")
		log.Printf("[API] WARNING: Attempted to inject test data in LIVE mode - request blocked")
		return
	}

	// 解析请求体
	var req struct {
		Symbol    string    `json:"symbol"`
		Exchange  string    `json:"exchange"`
		BidPrice  []float64 `json:"bid_price"`
		AskPrice  []float64 `json:"ask_price"`
		BidQty    []uint32  `json:"bid_qty,omitempty"`
		AskQty    []uint32  `json:"ask_qty,omitempty"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		a.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid request body: %v", err))
		return
	}

	// 验证必需字段
	if req.Symbol == "" || len(req.BidPrice) == 0 || len(req.AskPrice) == 0 {
		a.sendError(w, http.StatusBadRequest, "Missing required fields: symbol, bid_price, ask_price")
		return
	}

	// 设置默认值
	if req.Exchange == "" {
		req.Exchange = "SHFE" // 默认上期所
	}
	if len(req.BidQty) == 0 {
		req.BidQty = make([]uint32, len(req.BidPrice))
		for i := range req.BidQty {
			req.BidQty[i] = 100 // 默认量
		}
	}
	if len(req.AskQty) == 0 {
		req.AskQty = make([]uint32, len(req.AskPrice))
		for i := range req.AskQty {
			req.AskQty[i] = 100 // 默认量
		}
	}

	// 创建 MarketDataUpdate protobuf 消息
	md := &mdpb.MarketDataUpdate{
		Symbol:    req.Symbol,
		Exchange:  req.Exchange,
		Timestamp: uint64(time.Now().UnixNano()),
		BidPrice:  req.BidPrice,
		BidQty:    req.BidQty,
		AskPrice:  req.AskPrice,
		AskQty:    req.AskQty,
		LastPrice: (req.BidPrice[0] + req.AskPrice[0]) / 2, // 用中间价作为最新价
	}

	// 发送给策略
	a.trader.Strategy.OnMarketData(md)

	log.Printf("[API] Test market data sent: %s bid=%.2f ask=%.2f",
		req.Symbol, req.BidPrice[0], req.AskPrice[0])

	a.sendSuccess(w, "Market data sent to strategy", map[string]interface{}{
		"symbol": req.Symbol,
		"bid":    req.BidPrice[0],
		"ask":    req.AskPrice[0],
	})
}

// corsMiddleware adds CORS headers to allow browser access from file://
func (a *APIServer) corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 允许所有来源（开发环境）
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// 处理 OPTIONS 预检请求
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		// 调用实际处理函数
		next(w, r)
	}
}

// handleModelReload handles manual model reload trigger
func (a *APIServer) handleModelReload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		a.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	log.Println("[API] Model reload requested")

	if err := a.trader.ReloadModel(); err != nil {
		log.Printf("[API] Model reload failed: %v", err)
		a.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to reload model: %v", err))
		return
	}

	log.Println("[API] ✓ Model reloaded successfully")
	a.sendSuccess(w, "Model reloaded successfully", map[string]interface{}{
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

// handleModelStatus handles model status query
func (a *APIServer) handleModelStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	status := a.trader.GetModelStatus()
	a.sendSuccess(w, "Model status retrieved", status)
}

// handleModelHistory handles model reload history query
func (a *APIServer) handleModelHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	history := a.trader.GetModelReloadHistory()
	a.sendSuccess(w, "Model reload history retrieved", map[string]interface{}{
		"history": history,
		"count":   len(history),
	})
}

// handlePositions handles GET /api/v1/positions
// 返回所有持仓（按交易所分组）
func (a *APIServer) handlePositions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 可选：支持查询参数过滤
	exchange := r.URL.Query().Get("exchange") // 例如: ?exchange=SHFE
	symbol := r.URL.Query().Get("symbol")     // 例如: ?symbol=ag2603

	// 获取持仓数据
	a.trader.positionsMu.RLock()
	positions := a.trader.positionsByExchange
	a.trader.positionsMu.RUnlock()

	// 应用过滤逻辑
	filtered := make(map[string][]interface{})
	for exch, posList := range positions {
		// 过滤交易所
		if exchange != "" && exch != exchange {
			continue
		}

		var filteredPositions []interface{}
		for _, pos := range posList {
			// 过滤品种
			if symbol != "" && pos.Symbol != symbol {
				continue
			}

			filteredPositions = append(filteredPositions, map[string]interface{}{
				"symbol":           pos.Symbol,
				"exchange":         pos.Exchange,
				"direction":        pos.Direction,
				"volume":           pos.Volume,
				"today_volume":     pos.TodayVolume,
				"yesterday_volume": pos.YesterdayVolume,
				"avg_price":        pos.AvgPrice,
				"position_profit":  pos.PositionProfit,
				"margin":           pos.Margin,
			})
		}

		if len(filteredPositions) > 0 {
			filtered[exch] = filteredPositions
		}
	}

	a.sendSuccess(w, "Positions retrieved", filtered)
}

// handlePositionsSummary handles GET /api/v1/positions/summary
// 返回持仓摘要统计
func (a *APIServer) handlePositionsSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 获取持仓数据
	a.trader.positionsMu.RLock()
	positions := a.trader.positionsByExchange
	a.trader.positionsMu.RUnlock()

	// 计算统计数据
	var totalPositions int
	var totalVolume int64
	var totalProfit float64
	var totalMargin float64
	exchangeStats := make(map[string]map[string]interface{})

	for exch, posList := range positions {
		var exchVolume int64
		var exchProfit float64
		var exchMargin float64
		exchPositionCount := len(posList)

		for _, pos := range posList {
			totalVolume += pos.Volume
			totalProfit += pos.PositionProfit
			totalMargin += pos.Margin

			exchVolume += pos.Volume
			exchProfit += pos.PositionProfit
			exchMargin += pos.Margin
		}

		totalPositions += exchPositionCount

		exchangeStats[exch] = map[string]interface{}{
			"position_count": exchPositionCount,
			"total_volume":   exchVolume,
			"total_profit":   exchProfit,
			"total_margin":   exchMargin,
		}
	}

	summary := map[string]interface{}{
		"total_positions":  totalPositions,
		"exchange_count":   len(positions),
		"total_volume":     totalVolume,
		"total_profit":     totalProfit,
		"total_margin":     totalMargin,
		"by_exchange":      exchangeStats,
	}

	a.sendSuccess(w, "Position summary retrieved", summary)
}

// ==================== Multi-Strategy API Endpoints (P2-12.2) ====================

// DashboardOverview represents the dashboard overview response
type DashboardOverview struct {
	MultiStrategy     bool                   `json:"multi_strategy"`
	Mode              string                 `json:"mode"`
	TotalStrategies   int                    `json:"total_strategies"`
	ActiveStrategies  int                    `json:"active_strategies"`
	RunningStrategies int                    `json:"running_strategies"`
	TotalRealizedPnL  float64                `json:"total_realized_pnl"`
	TotalUnrealizedPnL float64               `json:"total_unrealized_pnl"`
	TotalPnL          float64                `json:"total_pnl"`
	Strategies        []StrategyOverviewItem `json:"strategies"`
	Timestamp         string                 `json:"timestamp"`
}

// StrategyOverviewItem represents a strategy item in the overview
type StrategyOverviewItem struct {
	ID            string   `json:"id"`
	Type          string   `json:"type"`
	Symbols       []string `json:"symbols"`
	Running       bool     `json:"running"`
	Active        bool     `json:"active"`
	ConditionsMet bool     `json:"conditions_met"`
	Eligible      bool     `json:"eligible"`
	Allocation    float64  `json:"allocation"`
	RealizedPnL   float64  `json:"realized_pnl"`
	UnrealizedPnL float64  `json:"unrealized_pnl"`
}

// handleDashboardOverview handles GET /api/v1/dashboard/overview
// 返回仪表板总览数据（支持多策略）
func (a *APIServer) handleDashboardOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	overview := &DashboardOverview{
		MultiStrategy: a.trader.IsMultiStrategy(),
		Mode:          a.trader.Config.System.Mode,
		Timestamp:     time.Now().Format(time.RFC3339),
		Strategies:    make([]StrategyOverviewItem, 0),
	}

	if a.trader.IsMultiStrategy() && a.trader.GetStrategyManager() != nil {
		// Multi-strategy mode
		mgr := a.trader.GetStrategyManager()
		status := mgr.GetStatus()

		overview.TotalStrategies = status.TotalStrategies
		overview.ActiveStrategies = status.ActiveStrategies
		overview.RunningStrategies = status.RunningStrategies

		// Get aggregated PNL
		aggPNL := mgr.GetAggregatedPNL()
		overview.TotalRealizedPnL = aggPNL.TotalRealizedPnL
		overview.TotalUnrealizedPnL = aggPNL.TotalUnrealizedPnL
		overview.TotalPnL = aggPNL.TotalPnL

		// Build strategy list
		for id, info := range status.StrategyStatuses {
			item := StrategyOverviewItem{
				ID:            id,
				Type:          info.Type,
				Symbols:       info.Symbols,
				Running:       info.Running,
				Active:        info.Active,
				ConditionsMet: info.ConditionsMet,
				Eligible:      info.Eligible,
				Allocation:    info.Allocation,
			}
			if info.PNL != nil {
				item.RealizedPnL = info.PNL.RealizedPnL
				item.UnrealizedPnL = info.PNL.UnrealizedPnL
			}
			overview.Strategies = append(overview.Strategies, item)
		}
	} else {
		// Single-strategy mode
		overview.TotalStrategies = 1

		baseStrat := a.getBaseStrategy()
		isActive := false
		conditionsMet := false
		eligible := false
		if baseStrat != nil {
			isActive = baseStrat.ControlState.IsActive()
			conditionsMet = baseStrat.ControlState.ConditionsMet
			eligible = baseStrat.ControlState.Eligible
		}

		isRunning := a.trader.Strategy.IsRunning()
		if isRunning {
			overview.RunningStrategies = 1
		}
		if isActive {
			overview.ActiveStrategies = 1
		}

		pnl := a.trader.Strategy.GetPNL()
		if pnl != nil {
			overview.TotalRealizedPnL = pnl.RealizedPnL
			overview.TotalUnrealizedPnL = pnl.UnrealizedPnL
			overview.TotalPnL = pnl.RealizedPnL + pnl.UnrealizedPnL
		}

		overview.Strategies = append(overview.Strategies, StrategyOverviewItem{
			ID:            a.trader.Config.System.StrategyID,
			Type:          a.trader.Config.Strategy.Type,
			Symbols:       a.trader.Config.Strategy.Symbols,
			Running:       isRunning,
			Active:        isActive,
			ConditionsMet: conditionsMet,
			Eligible:      eligible,
			Allocation:    1.0,
			RealizedPnL:   overview.TotalRealizedPnL,
			UnrealizedPnL: overview.TotalUnrealizedPnL,
		})
	}

	a.sendSuccess(w, "Dashboard overview retrieved", overview)
}

// StrategyListResponse represents the strategies list response
type StrategyListResponse struct {
	MultiStrategy bool                    `json:"multi_strategy"`
	Count         int                     `json:"count"`
	Strategies    []StrategyDetailItem    `json:"strategies"`
}

// StrategyDetailItem represents detailed strategy information
type StrategyDetailItem struct {
	ID             string                 `json:"id"`
	Type           string                 `json:"type"`
	Symbols        []string               `json:"symbols"`
	Running        bool                   `json:"running"`
	Active         bool                   `json:"active"`
	ConditionsMet  bool                   `json:"conditions_met"`
	Eligible       bool                   `json:"eligible"`
	EligibleReason string                 `json:"eligible_reason"`
	SignalStrength float64                `json:"signal_strength"`
	Allocation     float64                `json:"allocation"`
	Indicators     map[string]float64     `json:"indicators"`
	Position       interface{}            `json:"estimated_position"` // Estimated position from order fills
	PNL            interface{}            `json:"pnl"`
}

// handleStrategies handles GET /api/v1/strategies
// 返回所有策略列表
func (a *APIServer) handleStrategies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	response := &StrategyListResponse{
		MultiStrategy: a.trader.IsMultiStrategy(),
		Strategies:    make([]StrategyDetailItem, 0),
	}

	if a.trader.IsMultiStrategy() && a.trader.GetStrategyManager() != nil {
		// Multi-strategy mode
		mgr := a.trader.GetStrategyManager()
		status := mgr.GetStatus()

		response.Count = status.TotalStrategies

		for id, info := range status.StrategyStatuses {
			item := StrategyDetailItem{
				ID:            id,
				Type:          info.Type,
				Symbols:       info.Symbols,
				Running:       info.Running,
				Active:        info.Active,
				ConditionsMet: info.ConditionsMet,
				Eligible:      info.Eligible,
				Allocation:    info.Allocation,
				Indicators:    info.Indicators,
				Position:      info.EstimatedPosition, // Estimated position from order fills
				PNL:           info.PNL,
			}
			response.Strategies = append(response.Strategies, item)
		}
	} else {
		// Single-strategy mode
		response.Count = 1

		baseStrat := a.getBaseStrategy()
		item := StrategyDetailItem{
			ID:       a.trader.Config.System.StrategyID,
			Type:     a.trader.Config.Strategy.Type,
			Symbols:  a.trader.Config.Strategy.Symbols,
			Running:  a.trader.Strategy.IsRunning(),
			Position: a.trader.Strategy.GetEstimatedPosition(),
			PNL:      a.trader.Strategy.GetPNL(),
			Allocation: 1.0,
		}

		if baseStrat != nil {
			item.Active = baseStrat.ControlState.IsActive()
			item.ConditionsMet = baseStrat.ControlState.ConditionsMet
			item.Eligible = baseStrat.ControlState.Eligible
			item.EligibleReason = baseStrat.ControlState.EligibleReason
			item.SignalStrength = baseStrat.ControlState.SignalStrength
			item.Indicators = baseStrat.ControlState.Indicators
		}

		response.Strategies = append(response.Strategies, item)
	}

	a.sendSuccess(w, "Strategies list retrieved", response)
}

// handleStrategyByID handles requests to /api/v1/strategies/{id}
// Supports:
//   - GET /api/v1/strategies/{id} - Get strategy details
//   - POST /api/v1/strategies/{id}/activate - Activate strategy
//   - POST /api/v1/strategies/{id}/deactivate - Deactivate strategy
//   - POST /api/v1/strategies/{id}/model/reload - Hot reload model
//   - GET /api/v1/strategies/{id}/model/status - Get model status
//   - GET /api/v1/strategies/{id}/model/history - Get model reload history
func (a *APIServer) handleStrategyByID(w http.ResponseWriter, r *http.Request) {
	// Parse strategy ID and action from URL path
	// Expected paths:
	//   /api/v1/strategies/{id}
	//   /api/v1/strategies/{id}/activate
	//   /api/v1/strategies/{id}/deactivate
	//   /api/v1/strategies/{id}/model/reload
	//   /api/v1/strategies/{id}/model/status
	//   /api/v1/strategies/{id}/model/history
	path := r.URL.Path
	prefix := "/api/v1/strategies/"
	if len(path) <= len(prefix) {
		a.sendError(w, http.StatusBadRequest, "Strategy ID required")
		return
	}

	// Extract ID and action
	remainder := path[len(prefix):]
	parts := strings.Split(remainder, "/")
	strategyID := parts[0]

	// Route based on method and action
	switch {
	case r.Method == http.MethodGet && len(parts) == 1:
		// GET /api/v1/strategies/{id}
		a.handleGetStrategy(w, r, strategyID)
	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "activate":
		// POST /api/v1/strategies/{id}/activate
		a.handleActivateStrategy(w, r, strategyID)
	case r.Method == http.MethodPost && len(parts) == 2 && parts[1] == "deactivate":
		// POST /api/v1/strategies/{id}/deactivate
		a.handleDeactivateStrategy(w, r, strategyID)
	case r.Method == http.MethodPost && len(parts) == 3 && parts[1] == "model" && parts[2] == "reload":
		// POST /api/v1/strategies/{id}/model/reload
		a.handleStrategyModelReload(w, r, strategyID)
	case r.Method == http.MethodGet && len(parts) == 3 && parts[1] == "model" && parts[2] == "status":
		// GET /api/v1/strategies/{id}/model/status
		a.handleStrategyModelStatus(w, r, strategyID)
	case r.Method == http.MethodGet && len(parts) == 3 && parts[1] == "model" && parts[2] == "history":
		// GET /api/v1/strategies/{id}/model/history
		a.handleStrategyModelHistory(w, r, strategyID)
	default:
		a.sendError(w, http.StatusMethodNotAllowed, "Method not allowed or invalid action")
	}
}

// handleGetStrategy handles GET /api/v1/strategies/{id}
func (a *APIServer) handleGetStrategy(w http.ResponseWriter, r *http.Request, strategyID string) {
	if a.trader.IsMultiStrategy() && a.trader.GetStrategyManager() != nil {
		// Multi-strategy mode
		mgr := a.trader.GetStrategyManager()
		info, err := mgr.GetStrategyStatus(strategyID)
		if err != nil {
			a.sendError(w, http.StatusNotFound, fmt.Sprintf("Strategy not found: %s", strategyID))
			return
		}

		item := StrategyDetailItem{
			ID:            info.ID,
			Type:          info.Type,
			Symbols:       info.Symbols,
			Running:       info.Running,
			Active:        info.Active,
			ConditionsMet: info.ConditionsMet,
			Eligible:      info.Eligible,
			Allocation:    info.Allocation,
			Indicators:    info.Indicators,
			Position:      info.EstimatedPosition, // Estimated position
			PNL:           info.PNL,
		}
		a.sendSuccess(w, "Strategy details retrieved", item)
	} else {
		// Single-strategy mode - check if ID matches
		if strategyID != a.trader.Config.System.StrategyID {
			a.sendError(w, http.StatusNotFound, fmt.Sprintf("Strategy not found: %s", strategyID))
			return
		}

		baseStrat := a.getBaseStrategy()
		item := StrategyDetailItem{
			ID:         a.trader.Config.System.StrategyID,
			Type:       a.trader.Config.Strategy.Type,
			Symbols:    a.trader.Config.Strategy.Symbols,
			Running:    a.trader.Strategy.IsRunning(),
			Position:   a.trader.Strategy.GetEstimatedPosition(),
			PNL:        a.trader.Strategy.GetPNL(),
			Allocation: 1.0,
		}

		if baseStrat != nil {
			item.Active = baseStrat.ControlState.IsActive()
			item.ConditionsMet = baseStrat.ControlState.ConditionsMet
			item.Eligible = baseStrat.ControlState.Eligible
			item.EligibleReason = baseStrat.ControlState.EligibleReason
			item.SignalStrength = baseStrat.ControlState.SignalStrength
			item.Indicators = baseStrat.ControlState.Indicators
		}

		a.sendSuccess(w, "Strategy details retrieved", item)
	}
}

// handleActivateStrategy handles POST /api/v1/strategies/{id}/activate
func (a *APIServer) handleActivateStrategy(w http.ResponseWriter, r *http.Request, strategyID string) {
	a.commandMu.Lock()
	defer a.commandMu.Unlock()

	log.Printf("[API] Activating strategy: %s", strategyID)

	if a.trader.IsMultiStrategy() && a.trader.GetStrategyManager() != nil {
		// Multi-strategy mode
		mgr := a.trader.GetStrategyManager()
		if err := mgr.ActivateStrategy(strategyID); err != nil {
			a.sendError(w, http.StatusBadRequest, fmt.Sprintf("Failed to activate strategy: %v", err))
			return
		}

		log.Printf("[API] ✓ Strategy %s activated", strategyID)
		a.sendSuccess(w, "Strategy activated successfully", map[string]interface{}{
			"strategy_id": strategyID,
			"active":      true,
		})
	} else {
		// Single-strategy mode - check if ID matches
		if strategyID != a.trader.Config.System.StrategyID {
			a.sendError(w, http.StatusNotFound, fmt.Sprintf("Strategy not found: %s", strategyID))
			return
		}

		// Use existing activate logic
		baseStrat := a.getBaseStrategy()
		if baseStrat == nil {
			a.sendError(w, http.StatusInternalServerError, "Failed to access strategy control state")
			return
		}

		baseStrat.ControlState.ExitRequested = false
		baseStrat.ControlState.CancelPending = false
		baseStrat.ControlState.FlattenMode = false
		if baseStrat.ControlState.RunState == strategy.StrategyRunStateStopped ||
			baseStrat.ControlState.RunState == strategy.StrategyRunStateFlattening {
			baseStrat.ControlState.RunState = strategy.StrategyRunStateActive
		}
		baseStrat.ControlState.Activate()

		if !a.trader.Strategy.IsRunning() {
			if err := a.trader.Strategy.Start(); err != nil {
				a.sendError(w, http.StatusInternalServerError, fmt.Sprintf("Failed to start strategy: %v", err))
				return
			}
		}

		log.Printf("[API] ✓ Strategy %s activated", strategyID)
		a.sendSuccess(w, "Strategy activated successfully", map[string]interface{}{
			"strategy_id": strategyID,
			"active":      true,
		})
	}
}

// handleDeactivateStrategy handles POST /api/v1/strategies/{id}/deactivate
func (a *APIServer) handleDeactivateStrategy(w http.ResponseWriter, r *http.Request, strategyID string) {
	a.commandMu.Lock()
	defer a.commandMu.Unlock()

	log.Printf("[API] Deactivating strategy: %s", strategyID)

	if a.trader.IsMultiStrategy() && a.trader.GetStrategyManager() != nil {
		// Multi-strategy mode
		mgr := a.trader.GetStrategyManager()
		if err := mgr.DeactivateStrategy(strategyID); err != nil {
			a.sendError(w, http.StatusBadRequest, fmt.Sprintf("Failed to deactivate strategy: %v", err))
			return
		}

		log.Printf("[API] ✓ Strategy %s deactivated", strategyID)
		a.sendSuccess(w, "Strategy deactivated successfully", map[string]interface{}{
			"strategy_id": strategyID,
			"active":      false,
		})
	} else {
		// Single-strategy mode - check if ID matches
		if strategyID != a.trader.Config.System.StrategyID {
			a.sendError(w, http.StatusNotFound, fmt.Sprintf("Strategy not found: %s", strategyID))
			return
		}

		baseStrat := a.getBaseStrategy()
		if baseStrat == nil {
			a.sendError(w, http.StatusInternalServerError, "Failed to access strategy control state")
			return
		}

		baseStrat.TriggerFlatten(strategy.FlattenReasonManual, false)
		baseStrat.ControlState.Deactivate()

		log.Printf("[API] ✓ Strategy %s deactivated", strategyID)
		a.sendSuccess(w, "Strategy deactivated successfully", map[string]interface{}{
			"strategy_id": strategyID,
			"active":      false,
		})
	}
}

// handleStrategyModelReload handles POST /api/v1/strategies/{id}/model/reload
func (a *APIServer) handleStrategyModelReload(w http.ResponseWriter, r *http.Request, strategyID string) {
	log.Printf("[API] Model reload requested for strategy: %s", strategyID)

	if !a.trader.IsMultiStrategy() || a.trader.GetStrategyManager() == nil {
		a.sendError(w, http.StatusBadRequest, "Model hot reload only supported in multi-strategy mode")
		return
	}

	mgr := a.trader.GetStrategyManager()
	if err := mgr.ReloadStrategyModel(strategyID); err != nil {
		log.Printf("[API] Failed to reload model for strategy %s: %v", strategyID, err)
		a.sendError(w, http.StatusBadRequest, fmt.Sprintf("Failed to reload model: %v", err))
		return
	}

	log.Printf("[API] ✓ Model reloaded successfully for strategy: %s", strategyID)
	a.sendSuccess(w, "Model reloaded successfully", map[string]interface{}{
		"strategy_id": strategyID,
		"timestamp":   time.Now().Format(time.RFC3339),
	})
}

// handleStrategyModelStatus handles GET /api/v1/strategies/{id}/model/status
func (a *APIServer) handleStrategyModelStatus(w http.ResponseWriter, r *http.Request, strategyID string) {
	if !a.trader.IsMultiStrategy() || a.trader.GetStrategyManager() == nil {
		a.sendError(w, http.StatusBadRequest, "Model hot reload only supported in multi-strategy mode")
		return
	}

	mgr := a.trader.GetStrategyManager()
	status, err := mgr.GetStrategyModelStatus(strategyID)
	if err != nil {
		a.sendError(w, http.StatusNotFound, fmt.Sprintf("Strategy not found: %s", strategyID))
		return
	}

	a.sendSuccess(w, "Model status retrieved", status)
}

// handleStrategyModelHistory handles GET /api/v1/strategies/{id}/model/history
func (a *APIServer) handleStrategyModelHistory(w http.ResponseWriter, r *http.Request, strategyID string) {
	if !a.trader.IsMultiStrategy() || a.trader.GetStrategyManager() == nil {
		a.sendError(w, http.StatusBadRequest, "Model hot reload only supported in multi-strategy mode")
		return
	}

	// TODO: 实现model重载历史追踪
	// 目前返回空历史
	a.sendSuccess(w, "Model reload history retrieved", map[string]interface{}{
		"strategy_id": strategyID,
		"history":     []interface{}{},
		"count":       0,
	})
}

// RealtimeIndicatorsResponse represents realtime indicators response
type RealtimeIndicatorsResponse struct {
	Timestamp  string                          `json:"timestamp"`
	Strategies map[string]StrategyIndicators   `json:"strategies"`
}

// StrategyIndicators represents indicators for a single strategy
type StrategyIndicators struct {
	StrategyID    string             `json:"strategy_id"`
	StrategyType  string             `json:"strategy_type"`
	Symbols       []string           `json:"symbols"`
	Active        bool               `json:"active"`
	ConditionsMet bool               `json:"conditions_met"`
	Eligible      bool               `json:"eligible"`
	SignalStrength float64           `json:"signal_strength"`
	Indicators    map[string]float64 `json:"indicators"`
	MarketData    map[string]MarketDataSnapshot `json:"market_data"`
}

// MarketDataSnapshot represents a snapshot of market data for a symbol
type MarketDataSnapshot struct {
	Symbol    string  `json:"symbol"`
	BidPrice  float64 `json:"bid_price"`
	AskPrice  float64 `json:"ask_price"`
	LastPrice float64 `json:"last_price"`
	Spread    float64 `json:"spread"`
}

// handleRealtimeIndicators handles GET /api/v1/indicators/realtime
// 返回所有策略的实时指标数据
func (a *APIServer) handleRealtimeIndicators(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		a.sendError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	response := &RealtimeIndicatorsResponse{
		Timestamp:  time.Now().Format(time.RFC3339),
		Strategies: make(map[string]StrategyIndicators),
	}

	if a.trader.IsMultiStrategy() && a.trader.GetStrategyManager() != nil {
		// Multi-strategy mode
		mgr := a.trader.GetStrategyManager()
		mgr.ForEach(func(id string, strat strategy.Strategy) {
			indicators := StrategyIndicators{
				StrategyID:   id,
				StrategyType: strat.GetType(),
				Indicators:   make(map[string]float64),
				MarketData:   make(map[string]MarketDataSnapshot),
			}

			// Get config for symbols
			if cfg, ok := mgr.GetConfig(id); ok {
				indicators.Symbols = cfg.Symbols
			}

			// Get control state info
			baseStrat := strat.GetBaseStrategy()
			if baseStrat != nil {
				indicators.Active = baseStrat.ControlState.IsActive()
				indicators.ConditionsMet = baseStrat.ControlState.ConditionsMet
				indicators.Eligible = baseStrat.ControlState.Eligible
				indicators.SignalStrength = baseStrat.ControlState.SignalStrength
				indicators.Indicators = baseStrat.ControlState.Indicators
			}

			response.Strategies[id] = indicators
		})
	} else {
		// Single-strategy mode
		strategyID := a.trader.Config.System.StrategyID
		indicators := StrategyIndicators{
			StrategyID:   strategyID,
			StrategyType: a.trader.Config.Strategy.Type,
			Symbols:      a.trader.Config.Strategy.Symbols,
			Indicators:   make(map[string]float64),
			MarketData:   make(map[string]MarketDataSnapshot),
		}

		baseStrat := a.getBaseStrategy()
		if baseStrat != nil {
			indicators.Active = baseStrat.ControlState.IsActive()
			indicators.ConditionsMet = baseStrat.ControlState.ConditionsMet
			indicators.Eligible = baseStrat.ControlState.Eligible
			indicators.SignalStrength = baseStrat.ControlState.SignalStrength
			indicators.Indicators = baseStrat.ControlState.Indicators
		}

		response.Strategies[strategyID] = indicators
	}

	a.sendSuccess(w, "Realtime indicators retrieved", response)
}
