package trader

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/config"
	"github.com/yourusername/quantlink-trade-system/pkg/portfolio"
	"github.com/yourusername/quantlink-trade-system/pkg/risk"
	"github.com/yourusername/quantlink-trade-system/pkg/strategy"
)

// Trader encapsulates the complete trading system
// 对应 tbsrc 的 TradeBot 主程序
type Trader struct {
	Config *config.TraderConfig

	// Core components
	Engine      *strategy.StrategyEngine
	Strategy    strategy.Strategy
	Portfolio   *portfolio.PortfolioManager
	RiskManager *risk.RiskManager
	SessionMgr  *SessionManager
	APIServer   *APIServer

	// State
	mu             sync.RWMutex
	running        bool
	controlSignals chan os.Signal
}

// NewTrader creates a new trader instance
func NewTrader(cfg *config.TraderConfig) (*Trader, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	t := &Trader{
		Config:  cfg,
		running: false,
	}

	return t, nil
}

// Initialize initializes all components
func (t *Trader) Initialize() error {
	log.Printf("[Trader] Initializing trader (Strategy ID: %s, Mode: %s)...",
		t.Config.System.StrategyID, t.Config.System.Mode)

	// 1. Create and initialize Risk Manager
	log.Println("[Trader] Creating Risk Manager...")
	riskConfig := &risk.RiskManagerConfig{
		EnableGlobalLimits:     true,
		EnableStrategyLimits:   true,
		EnablePortfolioLimits:  true,
		AlertRetentionSeconds:  3600,
		MaxAlertQueueSize:      1000,
		EmergencyStopThreshold: 3,
		CheckIntervalMs:        t.Config.Risk.CheckIntervalMs,
	}
	t.RiskManager = risk.NewRiskManager(riskConfig)
	if err := t.RiskManager.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize risk manager: %w", err)
	}

	// Update risk limits (using UpdateLimit method)
	if t.Config.Risk.MaxLoss > 0 {
		t.RiskManager.UpdateLimit("global_max_loss", t.Config.Risk.MaxLoss, true)
	}
	if t.Config.Risk.DailyLossLimit > 0 {
		t.RiskManager.UpdateLimit("global_daily_loss", t.Config.Risk.DailyLossLimit, true)
	}
	log.Println("[Trader] ✓ Risk Manager initialized")

	// 2. Create and initialize Portfolio Manager (if configured)
	if t.Config.Portfolio.TotalCapital > 0 {
		log.Println("[Trader] Creating Portfolio Manager...")
		portfolioConfig := &portfolio.PortfolioConfig{
			TotalCapital:          t.Config.Portfolio.TotalCapital,
			StrategyAllocation:    t.Config.Portfolio.StrategyAllocation,
			RebalanceIntervalSec:  t.Config.Portfolio.RebalanceIntervalSec,
			MinAllocation:         t.Config.Portfolio.MinAllocation,
			MaxAllocation:         t.Config.Portfolio.MaxAllocation,
			EnableAutoRebalance:   t.Config.Portfolio.EnableAutoRebalance,
			EnableCorrelationCalc: t.Config.Portfolio.EnableCorrelation,
		}
		t.Portfolio = portfolio.NewPortfolioManager(portfolioConfig)
		if err := t.Portfolio.Initialize(); err != nil {
			return fmt.Errorf("failed to initialize portfolio manager: %w", err)
		}
		log.Println("[Trader] ✓ Portfolio Manager initialized")
	}

	// 3. Create and initialize Strategy Engine
	log.Println("[Trader] Creating Strategy Engine...")
	engineConfig := &strategy.EngineConfig{
		ORSGatewayAddr:      t.Config.Engine.ORSGatewayAddr,
		NATSAddr:            t.Config.Engine.NATSAddr,
		OrderQueueSize:      t.Config.Engine.OrderQueueSize,
		TimerInterval:       t.Config.Engine.TimerInterval,
		MaxConcurrentOrders: t.Config.Engine.MaxConcurrentOrders,
	}
	t.Engine = strategy.NewStrategyEngine(engineConfig)

	// Initialize engine (may fail if services not running)
	if err := t.Engine.Initialize(); err != nil {
		if t.Config.System.Mode == "live" {
			return fmt.Errorf("failed to initialize engine in live mode: %w", err)
		}
		log.Printf("[Trader] Warning: Engine initialization failed (Mode: %s): %v",
			t.Config.System.Mode, err)
		log.Println("[Trader] Continuing without external connections...")
	} else {
		log.Println("[Trader] ✓ Strategy Engine initialized")
	}

	// 4. Create strategy instance
	log.Printf("[Trader] Creating %s strategy...", t.Config.Strategy.Type)
	var err error
	t.Strategy, err = t.createStrategy()
	if err != nil {
		return fmt.Errorf("failed to create strategy: %w", err)
	}

	// Initialize strategy
	if err := t.Strategy.Initialize(t.toStrategyConfig()); err != nil {
		return fmt.Errorf("failed to initialize strategy: %w", err)
	}
	log.Println("[Trader] ✓ Strategy initialized")

	// Add strategy to engine
	if err := t.Engine.AddStrategy(t.Strategy); err != nil {
		return fmt.Errorf("failed to add strategy to engine: %w", err)
	}

	// Add strategy to portfolio (if portfolio manager exists)
	if t.Portfolio != nil {
		allocation := 1.0 // default 100% for single strategy
		if alloc, ok := t.Config.Portfolio.StrategyAllocation[t.Config.System.StrategyID]; ok {
			allocation = alloc
		}
		if err := t.Portfolio.AddStrategy(t.Strategy, allocation); err != nil {
			return fmt.Errorf("failed to add strategy to portfolio: %w", err)
		}
		log.Printf("[Trader] ✓ Strategy added to portfolio (allocation: %.2f%%)", allocation*100)
	}

	// 5. Create Session Manager
	log.Println("[Trader] Creating Session Manager...")
	t.SessionMgr = NewSessionManager(&t.Config.Session)
	log.Println("[Trader] ✓ Session Manager created")

	// 6. Create API Server (if enabled)
	if t.Config.API.Enabled {
		log.Printf("[Trader] Creating API Server (port: %d)...", t.Config.API.Port)
		t.APIServer = NewAPIServer(t, t.Config.API.Port)
		log.Println("[Trader] ✓ API Server created")
	}

	log.Println("[Trader] ✓ All components initialized successfully")
	return nil
}

// Start starts the trader
func (t *Trader) Start() error {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return fmt.Errorf("trader already running")
	}
	t.running = true
	t.mu.Unlock()

	log.Println("[Trader] Starting trader...")

	// Start risk manager
	if err := t.RiskManager.Start(); err != nil {
		return fmt.Errorf("failed to start risk manager: %w", err)
	}
	log.Println("[Trader] ✓ Risk Manager started")

	// Start portfolio manager (if exists)
	if t.Portfolio != nil {
		if err := t.Portfolio.Start(); err != nil {
			return fmt.Errorf("failed to start portfolio manager: %w", err)
		}
		log.Println("[Trader] ✓ Portfolio Manager started")
	}

	// Start strategy engine
	if err := t.Engine.Start(); err != nil {
		if t.Config.System.Mode == "live" {
			return fmt.Errorf("failed to start engine in live mode: %w", err)
		}
		log.Printf("[Trader] Warning: Engine start failed: %v", err)
	} else {
		log.Println("[Trader] ✓ Strategy Engine started")
	}

	// Decide whether to auto-activate based on mode (对应 tbsrc 行为)
	autoActivate := false
	if t.Config.System.Mode == "simulation" || t.Config.System.Mode == "backtest" {
		// Simulation/Backtest 模式：自动激活
		autoActivate = true
		log.Println("[Trader] Simulation/Backtest mode: auto-activating strategy")
	} else if t.Config.System.Mode == "live" {
		// Live 模式：等待手动激活（对应 tbsrc m_Active = false）
		autoActivate = false
		log.Println("[Trader] ════════════════════════════════════════════════════════════")
		log.Println("[Trader] Live mode: Strategy initialized but NOT activated")
		log.Println("[Trader] Waiting for manual activation signal...")
		log.Printf("[Trader] To activate: kill -SIGUSR1 %d\n", os.Getpid())
		log.Printf("[Trader] To deactivate: kill -SIGUSR2 %d\n", os.Getpid())
		log.Println("[Trader] ════════════════════════════════════════════════════════════")
	}

	if autoActivate {
		if err := t.Strategy.Start(); err != nil {
			return fmt.Errorf("failed to start strategy: %w", err)
		}
		log.Println("[Trader] ✓ Strategy activated and trading")
	}

	// Start API server (if enabled)
	if t.APIServer != nil {
		if err := t.APIServer.Start(); err != nil {
			return fmt.Errorf("failed to start API server: %w", err)
		}
	}

	// Start session manager
	go t.runSessionManager()

	// Start risk monitoring
	go t.runRiskMonitoring()

	// Start signal handlers (对应 tbsrc 信号处理)
	t.setupSignalHandlers()

	log.Println("[Trader] ✓ Trader started successfully")
	log.Println("[Trader] ════════════════════════════════════════════════════════════")
	log.Printf("[Trader] Strategy: %s (%s)", t.Config.System.StrategyID, t.Config.Strategy.Type)
	log.Printf("[Trader] Mode: %s", t.Config.System.Mode)
	log.Printf("[Trader] Symbols: %v", t.Config.Strategy.Symbols)
	log.Println("[Trader] ════════════════════════════════════════════════════════════")

	return nil
}

// Stop stops the trader
func (t *Trader) Stop() error {
	t.mu.Lock()
	if !t.running {
		t.mu.Unlock()
		return nil
	}
	t.running = false
	t.mu.Unlock()

	log.Println("[Trader] Stopping trader...")

	// Stop API server
	if t.APIServer != nil {
		if err := t.APIServer.Stop(); err != nil {
			log.Printf("[Trader] Error stopping API server: %v", err)
		} else {
			log.Println("[Trader] ✓ API Server stopped")
		}
	}

	// Stop strategy
	if t.Strategy != nil {
		if err := t.Strategy.Stop(); err != nil {
			log.Printf("[Trader] Error stopping strategy: %v", err)
		} else {
			log.Println("[Trader] ✓ Strategy stopped")
		}
	}

	// Stop engine
	if t.Engine != nil {
		if err := t.Engine.Stop(); err != nil {
			log.Printf("[Trader] Error stopping engine: %v", err)
		} else {
			log.Println("[Trader] ✓ Engine stopped")
		}
	}

	// Stop portfolio manager
	if t.Portfolio != nil {
		if err := t.Portfolio.Stop(); err != nil {
			log.Printf("[Trader] Error stopping portfolio: %v", err)
		} else {
			log.Println("[Trader] ✓ Portfolio Manager stopped")
		}
	}

	// Stop risk manager
	if t.RiskManager != nil {
		if err := t.RiskManager.Stop(); err != nil {
			log.Printf("[Trader] Error stopping risk manager: %v", err)
		} else {
			log.Println("[Trader] ✓ Risk Manager stopped")
		}
	}

	log.Println("[Trader] ✓ Trader stopped successfully")
	return nil
}

// IsRunning returns whether the trader is running
func (t *Trader) IsRunning() bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.running
}

// GetStatus returns the trader status
func (t *Trader) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"running":     t.IsRunning(),
		"strategy_id": t.Config.System.StrategyID,
		"mode":        t.Config.System.Mode,
		"strategy":    t.Strategy.GetStatus(),
		"position":    t.Strategy.GetPosition(),
		"pnl":         t.Strategy.GetPNL(),
		"risk":        t.Strategy.GetRiskMetrics(),
	}
}

// createStrategy creates a strategy instance based on type
func (t *Trader) createStrategy() (strategy.Strategy, error) {
	strategyID := t.Config.System.StrategyID
	strategyType := t.Config.Strategy.Type

	var s strategy.Strategy

	switch strategyType {
	case "passive":
		s = strategy.NewPassiveStrategy(strategyID)
	case "aggressive":
		s = strategy.NewAggressiveStrategy(strategyID)
	case "hedging":
		s = strategy.NewHedgingStrategy(strategyID)
	case "pairwise_arb":
		s = strategy.NewPairwiseArbStrategy(strategyID)
	default:
		return nil, fmt.Errorf("unknown strategy type: %s", strategyType)
	}

	return s, nil
}

// toStrategyConfig converts trader config to strategy config
func (t *Trader) toStrategyConfig() *strategy.StrategyConfig {
	return &strategy.StrategyConfig{
		StrategyID:      t.Config.System.StrategyID,
		StrategyType:    t.Config.Strategy.Type,
		Symbols:         t.Config.Strategy.Symbols,
		Exchanges:       t.Config.Strategy.Exchanges,
		MaxPositionSize: t.Config.Strategy.MaxPositionSize,
		MaxExposure:     t.Config.Strategy.MaxExposure,
		RiskLimits: map[string]float64{
			"max_drawdown":    t.Config.Risk.MaxDrawdown,
			"stop_loss":       t.Config.Risk.StopLoss,
			"max_loss":        t.Config.Risk.MaxLoss,
			"daily_loss":      t.Config.Risk.DailyLossLimit,
			"max_reject":      float64(t.Config.Risk.MaxRejectCount),
		},
		Parameters: t.Config.Strategy.Parameters,
		Enabled:    true,
	}
}

// runSessionManager monitors trading sessions
func (t *Trader) runSessionManager() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for t.IsRunning() {
		<-ticker.C

		inSession := t.SessionMgr.IsInSession()
		strategyRunning := t.Strategy.IsRunning()

		// Auto-start strategy when session begins
		if inSession && !strategyRunning && t.Config.Session.AutoStart {
			log.Println("[Trader] Trading session started - starting strategy")
			if err := t.Strategy.Start(); err != nil {
				log.Printf("[Trader] Error starting strategy: %v", err)
			}
		}

		// Auto-stop strategy when session ends
		if !inSession && strategyRunning && t.Config.Session.AutoStop {
			log.Println("[Trader] Trading session ended - stopping strategy")
			if err := t.Strategy.Stop(); err != nil {
				log.Printf("[Trader] Error stopping strategy: %v", err)
			}
		}
	}
}

// runRiskMonitoring monitors risk continuously
func (t *Trader) runRiskMonitoring() {
	ticker := time.NewTicker(time.Duration(t.Config.Risk.CheckIntervalMs) * time.Millisecond)
	defer ticker.Stop()

	for t.IsRunning() {
		<-ticker.C

		if !t.Strategy.IsRunning() {
			continue
		}

		// Check strategy risk
		strategyAlerts := t.RiskManager.CheckStrategy(t.Strategy)
		for _, alert := range strategyAlerts {
			t.RiskManager.AddAlert(&alert)

			// Take action based on alert
			if alert.Action == "stop" {
				log.Printf("[Trader] RISK ALERT: Stopping strategy due to %s", alert.Message)
				if err := t.Strategy.Stop(); err != nil {
					log.Printf("[Trader] Error stopping strategy: %v", err)
				}
			}
		}

		// Check global limits
		strategies := map[string]strategy.Strategy{
			t.Config.System.StrategyID: t.Strategy,
		}
		globalAlerts := t.RiskManager.CheckGlobal(strategies)
		for _, alert := range globalAlerts {
			t.RiskManager.AddAlert(&alert)

			if alert.Action == "emergency_stop" && !t.RiskManager.IsEmergencyStop() {
				log.Println("[Trader] EMERGENCY STOP triggered by global risk limits!")
				// Stop all strategies
				if err := t.Stop(); err != nil {
					log.Printf("[Trader] Error during emergency stop: %v", err)
				}
			}
		}
	}
}

// setupSignalHandlers sets up Unix signal handlers for strategy control
// 对应 tbsrc 的信号处理机制
func (t *Trader) setupSignalHandlers() {
	t.controlSignals = make(chan os.Signal, 1)

	// Listen for control signals (SIGUSR1, SIGUSR2)
	// SIGUSR1: Activate strategy (对应 tbsrc SIGUSR1)
	// SIGUSR2: Deactivate strategy / Squareoff (对应 tbsrc SIGTSTP)
	signal.Notify(t.controlSignals, syscall.SIGUSR1, syscall.SIGUSR2)

	go t.handleControlSignals()

	log.Println("[Trader] ✓ Signal handlers installed (SIGUSR1, SIGUSR2)")
}

// handleControlSignals handles incoming control signals
// 对应 tbsrc main.cpp:132-149 的信号处理
func (t *Trader) handleControlSignals() {
	for t.IsRunning() {
		sig := <-t.controlSignals

		switch sig {
		case syscall.SIGUSR1:
			// Activate strategy (对应 tbsrc SIGUSR1)
			// tbsrc: Strategy->m_Active = true
			log.Println("[Trader] ════════════════════════════════════════════════════════════")
			log.Println("[Trader] Received SIGUSR1: Activating strategy")
			log.Println("[Trader] ════════════════════════════════════════════════════════════")

			// Get BaseStrategy through type assertion
			baseStrat := t.getBaseStrategy()
			if baseStrat == nil {
				log.Println("[Trader] Error: Failed to access strategy control state")
				continue
			}

			// Reset control state (对应 tbsrc)
			baseStrat.ControlState.ExitRequested = false
			baseStrat.ControlState.CancelPending = false
			baseStrat.ControlState.FlattenMode = false
			baseStrat.ControlState.Activate()

			// Start strategy if not running
			if !t.Strategy.IsRunning() {
				if err := t.Strategy.Start(); err != nil {
					log.Printf("[Trader] Error starting strategy: %v", err)
				} else {
					log.Println("[Trader] ✓ Strategy activated and trading")
				}
			} else {
				log.Println("[Trader] ✓ Strategy already running, re-activated")
			}

		case syscall.SIGUSR2:
			// Deactivate strategy and squareoff (对应 tbsrc SIGTSTP)
			// tbsrc: Strategy->m_onExit = true, m_onCancel = true, m_onFlat = true, m_Active = false
			log.Println("[Trader] ════════════════════════════════════════════════════════════")
			log.Println("[Trader] Received SIGUSR2: Deactivating strategy (squareoff)")
			log.Println("[Trader] ════════════════════════════════════════════════════════════")

			// Get BaseStrategy through type assertion
			baseStrat := t.getBaseStrategy()
			if baseStrat == nil {
				log.Println("[Trader] Error: Failed to access strategy control state")
				continue
			}

			// Trigger flatten mode (对应 tbsrc HandleSquareoff)
			baseStrat.TriggerFlatten(strategy.FlattenReasonManual, false)
			baseStrat.ControlState.Deactivate()

			log.Println("[Trader] ✓ Strategy deactivated, positions being closed")
			log.Println("[Trader] Strategy will stop trading but process continues running")
			log.Printf("[Trader] To re-activate: kill -SIGUSR1 %d\n", os.Getpid())
		}
	}
}

// getBaseStrategy is a helper to get the BaseStrategy through type assertion
func (t *Trader) getBaseStrategy() *strategy.BaseStrategy {
	if accessor, ok := t.Strategy.(strategy.BaseStrategyAccessor); ok {
		return accessor.GetBaseStrategy()
	}
	log.Printf("[Trader] Error: Strategy does not implement BaseStrategyAccessor")
	return nil
}
