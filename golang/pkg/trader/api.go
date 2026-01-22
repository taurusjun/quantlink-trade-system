package trader

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
)

// APIServer provides HTTP REST API for trader control
// 对应 tbsrc 信号控制的现代化替代方案
type APIServer struct {
	trader    *Trader
	server    *http.Server
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
	Position   interface{}            `json:"position"`
	PNL        interface{}            `json:"pnl"`
	Risk       interface{}            `json:"risk"`
	Uptime     string                 `json:"uptime"`
	Details    map[string]interface{} `json:"details"`
}

// NewAPIServer creates a new API server
func NewAPIServer(trader *Trader, port int) *APIServer {
	api := &APIServer{
		trader:  trader,
		running: false,
	}

	mux := http.NewServeMux()

	// Register endpoints with CORS
	mux.HandleFunc("/api/v1/strategy/activate", api.corsMiddleware(api.handleActivate))
	mux.HandleFunc("/api/v1/strategy/deactivate", api.corsMiddleware(api.handleDeactivate))
	mux.HandleFunc("/api/v1/strategy/status", api.corsMiddleware(api.handleStatus))
	mux.HandleFunc("/api/v1/trader/status", api.corsMiddleware(api.handleTraderStatus))
	mux.HandleFunc("/api/v1/health", api.corsMiddleware(api.handleHealth))

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

	log.Printf("[API] Starting HTTP API server on %s", a.server.Addr)

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

	status := &StrategyStatusResponse{
		StrategyID: a.trader.Config.System.StrategyID,
		Running:    a.trader.Strategy.IsRunning(),
		Active:     baseStrat.ControlState.IsActive(),
		Mode:       a.trader.Config.System.Mode,
		Symbols:    a.trader.Config.Strategy.Symbols,
		Position:   a.trader.Strategy.GetPosition(),
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

// getBaseStrategy is a helper to get the BaseStrategy through type assertion
func (a *APIServer) getBaseStrategy() *strategy.BaseStrategy {
	if accessor, ok := a.trader.Strategy.(strategy.BaseStrategyAccessor); ok {
		return accessor.GetBaseStrategy()
	}
	log.Printf("[API] Error: Strategy does not implement BaseStrategyAccessor")
	return nil
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
