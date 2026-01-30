package trader

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/client"
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
	Strategy    strategy.Strategy              // 单策略模式（向后兼容）或第一个策略
	StrategyMgr *strategy.StrategyManager      // 多策略管理器（新增）
	Portfolio   *portfolio.PortfolioManager
	RiskManager *risk.RiskManager
	SessionMgr  *SessionManager
	APIServer   *APIServer

	// Model hot reload
	ModelWatcher *ModelWatcher

	// Positions (按交易所分组)
	positionsByExchange map[string][]client.PositionInfo
	positionsMu         sync.RWMutex

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
	if t.Config.System.MultiStrategy {
		log.Printf("[Trader] Initializing trader (Multi-Strategy Mode, Mode: %s)...",
			t.Config.System.Mode)
	} else {
		log.Printf("[Trader] Initializing trader (Strategy ID: %s, Mode: %s)...",
			t.Config.System.StrategyID, t.Config.System.Mode)
	}

	log.Println("[Trader] DEBUG: Starting Initialize()")

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
		NATSAddr:            t.Config.Engine.NATSAddr,
		CounterBridgeAddr:   t.Config.Engine.CounterBridgeAddr,
		OrderQueueSize:      t.Config.Engine.OrderQueueSize,
		TimerInterval:       t.Config.Engine.TimerInterval,
		MaxConcurrentOrders: t.Config.Engine.MaxConcurrentOrders,
	}

	// Select ORS Gateway address based on mode
	if t.Config.System.Mode == "backtest" {
		// Backtest mode: use BacktestOrderRouter address
		engineConfig.ORSGatewayAddr = "localhost:50052"
		log.Printf("[Trader] Using backtest ORS Gateway: %s", engineConfig.ORSGatewayAddr)
	} else {
		// Live/simulation mode: use real ORS Gateway
		engineConfig.ORSGatewayAddr = t.Config.Engine.ORSGatewayAddr
		log.Printf("[Trader] Using live ORS Gateway: %s", engineConfig.ORSGatewayAddr)
	}

	// Log Counter Bridge address if configured
	if engineConfig.CounterBridgeAddr != "" {
		log.Printf("[Trader] Using Counter Bridge for position query: %s", engineConfig.CounterBridgeAddr)
	}

	t.Engine = strategy.NewStrategyEngine(engineConfig)

	// Initialize engine (may fail if services not running)
	if err := t.Engine.Initialize(); err != nil {
		// 在测试环境下，即使是 live 模式也允许启动（不连接外部服务）
		log.Printf("[Trader] Warning: Engine initialization failed (Mode: %s): %v",
			t.Config.System.Mode, err)
		log.Println("[Trader] Continuing without external connections...")
		log.Println("[Trader] This is OK for testing/demo purposes")
	} else {
		log.Println("[Trader] ✓ Strategy Engine initialized")
	}

	// 4. Create strategy instance(s)
	if t.Config.System.MultiStrategy {
		// 多策略模式：使用 StrategyManager
		if err := t.initializeMultiStrategy(); err != nil {
			return fmt.Errorf("failed to initialize multi-strategy: %w", err)
		}
	} else {
		// 单策略模式（向后兼容）
		if err := t.initializeSingleStrategy(); err != nil {
			return fmt.Errorf("failed to initialize single strategy: %w", err)
		}
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

	// 7. Create Model Watcher (if configured)
	if t.Config.Strategy.ModelFile != "" && t.Config.Strategy.HotReload.Enabled {
		log.Println("[Trader] Creating Model Watcher...")

		watcherCfg := ModelWatcherConfig{
			ModelFilePath: t.Config.Strategy.ModelFile,
			OnReload: func(newParams map[string]interface{}) error {
				return t.onModelReload(newParams)
			},
		}

		var err error
		t.ModelWatcher, err = NewModelWatcher(watcherCfg)
		if err != nil {
			log.Printf("[Trader] Warning: Failed to create model watcher: %v", err)
			log.Println("[Trader] Continuing without model hot reload...")
		} else {
			log.Printf("[Trader] ✓ Model Watcher created (file: %s, mode: manual)",
				t.Config.Strategy.ModelFile)
		}
	}

	// 8. Query initial positions (查询初始持仓)
	if err := t.queryInitialPositions(); err != nil {
		log.Printf("[Trader] Warning: Failed to query initial positions: %v", err)
		// 不阻断启动，策略可以从持久化文件恢复
	} else {
		// 初始化策略持仓
		t.initializeStrategyPositions()
	}

	// 9. Start position verification (定期持仓校验)
	t.startPositionVerification()

	log.Println("[Trader] ✓ All components initialized successfully")
	return nil
}

// queryInitialPositions queries initial positions from broker
func (t *Trader) queryInitialPositions() error {
	log.Println("[Trader] Querying initial positions from broker...")

	// 检查Engine是否有ORS Client
	if t.Engine == nil || t.Engine.GetORSClient() == nil {
		log.Println("[Trader] Warning: ORS Client not available, skipping position query")
		return nil
	}

	orsClient := t.Engine.GetORSClient()

	// 添加重试机制，等待 Counter Bridge 完全启动
	var positions map[string][]client.PositionInfo
	var err error
	maxRetries := 5
	retryInterval := 2 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("[Trader] Position query attempt %d/%d...", attempt, maxRetries)

		// 调用Counter Bridge查询持仓
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		positions, err = orsClient.QueryPositions(ctx, "", "")
		cancel()

		if err == nil {
			// 查询成功
			log.Printf("[Trader] ✓ Position query succeeded on attempt %d", attempt)
			break
		}

		// 查询失败，记录错误
		log.Printf("[Trader] Position query attempt %d failed: %v", attempt, err)

		if attempt < maxRetries {
			log.Printf("[Trader] Retrying in %v...", retryInterval)
			time.Sleep(retryInterval)
		}
	}

	if err != nil {
		return fmt.Errorf("failed to query positions after %d attempts: %w", maxRetries, err)
	}

	// 存储持仓数据（按交易所分组）
	t.positionsMu.Lock()
	t.positionsByExchange = positions
	t.positionsMu.Unlock()

	// 统计持仓信息
	totalPositions := 0
	for _, posList := range positions {
		totalPositions += len(posList)
	}

	log.Printf("[Trader] ✓ Loaded %d positions from %d exchanges", totalPositions, len(positions))

	// 打印持仓摘要
	t.printPositionSummary()

	return nil
}

// initializeStrategyPositions 初始化策略持仓（从CTP查询结果）
func (t *Trader) initializeStrategyPositions() {
	t.positionsMu.RLock()
	positionsByExchange := t.positionsByExchange
	t.positionsMu.RUnlock()

	// 按品种聚合持仓（净持仓）
	posMap := make(map[string]int64)
	for _, posList := range positionsByExchange {
		for _, pos := range posList {
			qty := int64(pos.Volume)
			if pos.Direction == "SHORT" || pos.Direction == "short" {
				qty = -qty
			}
			posMap[pos.Symbol] += qty
		}
	}

	if len(posMap) == 0 {
		log.Println("[Trader] No positions to initialize in strategies")
		return
	}

	log.Printf("[Trader] Initializing strategy positions from CTP query (%d symbols)", len(posMap))

	// 传递给每个策略
	if t.Config.System.MultiStrategy && t.StrategyMgr != nil {
		for _, strategyID := range t.StrategyMgr.GetStrategyIDs() {
			strat, exists := t.StrategyMgr.GetStrategy(strategyID)
			if !exists || strat == nil {
				continue
			}

			if initializer, ok := strat.(strategy.PositionInitializer); ok {
				if err := initializer.InitializePositions(posMap); err != nil {
					log.Printf("[Trader] Warning: Failed to initialize positions for %s: %v", strategyID, err)
				}
			}
		}
	} else if t.Strategy != nil {
		if initializer, ok := t.Strategy.(strategy.PositionInitializer); ok {
			if err := initializer.InitializePositions(posMap); err != nil {
				log.Printf("[Trader] Warning: Failed to initialize strategy positions: %v", err)
			}
		}
	}
}

// startPositionVerification 启动定期持仓校验
func (t *Trader) startPositionVerification() {
	// 定期校验间隔：5分钟
	verifyInterval := 5 * time.Minute

	log.Printf("[Trader] Starting position verification (interval: %v)", verifyInterval)

	go func() {
		ticker := time.NewTicker(verifyInterval)
		defer ticker.Stop()

		for range ticker.C {
			if err := t.verifyPositions(); err != nil {
				log.Printf("[Trader] Position verification failed: %v", err)
			}
		}
	}()
}

// verifyPositions 校验策略持仓与CTP真实持仓
func (t *Trader) verifyPositions() error {
	log.Println("[Trader] Starting position verification...")

	// 1. 查询CTP真实持仓
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if t.Engine == nil || t.Engine.GetORSClient() == nil {
		return fmt.Errorf("ORS client not available")
	}

	positions, err := t.Engine.GetORSClient().QueryPositions(ctx, "", "")
	if err != nil {
		return fmt.Errorf("failed to query CTP positions: %w", err)
	}

	// 2. 聚合CTP持仓（按品种，净持仓）
	ctpPosMap := make(map[string]int64)
	for _, posList := range positions {
		for _, pos := range posList {
			qty := int64(pos.Volume)
			if pos.Direction == "SHORT" || pos.Direction == "short" {
				qty = -qty
			}
			ctpPosMap[pos.Symbol] += qty
		}
	}

	// 3. 聚合策略估算持仓
	strategyPosMap := t.aggregateStrategyPositions()

	// 4. 对比
	mismatches := []string{}
	allSymbols := make(map[string]bool)

	for symbol := range ctpPosMap {
		allSymbols[symbol] = true
	}
	for symbol := range strategyPosMap {
		allSymbols[symbol] = true
	}

	for symbol := range allSymbols {
		ctpQty := ctpPosMap[symbol]
		strategyQty := strategyPosMap[symbol]

		if ctpQty != strategyQty {
			diff := ctpQty - strategyQty
			mismatches = append(mismatches,
				fmt.Sprintf("%s: CTP=%d, Strategy=%d, Diff=%d",
					symbol, ctpQty, strategyQty, diff))
		}
	}

	if len(mismatches) > 0 {
		log.Println("[Trader] ⚠️  Position mismatch detected:")
		for _, msg := range mismatches {
			log.Printf("[Trader]     %s", msg)
		}

		// TODO: 可选的自动同步逻辑
		// if t.Config.Risk.EnableAutoPositionSync {
		// 	return t.syncStrategyPositions(ctpPosMap)
		// }

		return fmt.Errorf("position mismatch detected")
	}

	log.Println("[Trader] ✓ Position verification passed")
	return nil
}

// aggregateStrategyPositions 聚合所有策略的持仓
func (t *Trader) aggregateStrategyPositions() map[string]int64 {
	posMap := make(map[string]int64)

	if t.Config.System.MultiStrategy && t.StrategyMgr != nil {
		for _, strategyID := range t.StrategyMgr.GetStrategyIDs() {
			strat, exists := t.StrategyMgr.GetStrategy(strategyID)
			if !exists || strat == nil {
				continue
			}

			// 如果策略实现了PositionProvider接口
			if provider, ok := strat.(strategy.PositionProvider); ok {
				for symbol, qty := range provider.GetPositionsBySymbol() {
					posMap[symbol] += qty
				}
			}
		}
	} else if t.Strategy != nil {
		if provider, ok := t.Strategy.(strategy.PositionProvider); ok {
			for symbol, qty := range provider.GetPositionsBySymbol() {
				posMap[symbol] += qty
			}
		}
	}

	return posMap
}

// printPositionSummary prints position summary
func (t *Trader) printPositionSummary() {
	t.positionsMu.RLock()
	defer t.positionsMu.RUnlock()

	if len(t.positionsByExchange) == 0 {
		log.Println("[Trader] No positions found")
		return
	}

	log.Println("[Trader] ════════════════════════════════════════════════════════════")
	log.Println("[Trader] Position Summary:")
	log.Println("[Trader] ════════════════════════════════════════════════════════════")

	for exchange, positions := range t.positionsByExchange {
		log.Printf("[Trader] %s Exchange:", exchange)
		for _, pos := range positions {
			log.Printf("[Trader]   - %s %s: %d lots (today: %d, yesterday: %d)",
				pos.Symbol, pos.Direction, pos.Volume, pos.TodayVolume, pos.YesterdayVolume)
			log.Printf("[Trader]     Avg Price: %.2f, P&L: %.2f, Margin: %.2f",
				pos.AvgPrice, pos.PositionProfit, pos.Margin)
		}
	}

	log.Println("[Trader] ════════════════════════════════════════════════════════════")
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
		// 在测试环境下允许继续（不连接外部服务）
		log.Printf("[Trader] Warning: Engine start failed: %v", err)
		log.Println("[Trader] Running in offline mode (no external connections)")
	} else {
		log.Println("[Trader] ✓ Strategy Engine started")

		// Subscribe to market data for all configured symbols
		log.Println("[Trader] Subscribing to market data...")
		symbols := t.getAllSymbols()
		for _, symbol := range symbols {
			if err := t.Engine.SubscribeMarketData(symbol); err != nil {
				log.Printf("[Trader] Warning: Failed to subscribe to %s: %v", symbol, err)
			} else {
				log.Printf("[Trader] ✓ Subscribed to market data: %s", symbol)
			}
		}
	}

	// Decide whether to auto-activate based on config (对应 tbsrc 行为)
	autoActivate := t.Config.Session.AutoActivate

	if autoActivate {
		log.Printf("[Trader] Auto-activation enabled (mode: %s)", t.Config.System.Mode)
	} else {
		log.Println("[Trader] ════════════════════════════════════════════════════════════")
		log.Println("[Trader] Strategy initialized but NOT activated")
		log.Println("[Trader] Waiting for manual activation...")
		log.Println("[Trader] Activate via:")
		log.Println("[Trader]   - Web UI: POST /api/v1/strategy/activate")
		log.Println("[Trader]   - Web UI: POST /api/v1/strategies/{id}/activate (multi-strategy)")
		log.Printf("[Trader]   - Signal: kill -SIGUSR1 %d\n", os.Getpid())
		log.Println("[Trader] ════════════════════════════════════════════════════════════")
	}

	if autoActivate {
		if t.Config.System.MultiStrategy && t.StrategyMgr != nil {
			// Multi-strategy mode: start all strategies
			if err := t.StrategyMgr.Start(); err != nil {
				return fmt.Errorf("failed to start strategies: %w", err)
			}
			log.Printf("[Trader] ✓ %d strategies activated and trading", t.StrategyMgr.GetStrategyCount())
		} else {
			// Single-strategy mode
			if err := t.Strategy.Start(); err != nil {
				return fmt.Errorf("failed to start strategy: %w", err)
			}
			log.Println("[Trader] ✓ Strategy activated and trading")
		}
	}

	// Start API server (if enabled)
	if t.APIServer != nil {
		if err := t.APIServer.Start(); err != nil {
			return fmt.Errorf("failed to start API server: %w", err)
		}
	}

	// Start model watcher (if configured)
	if t.ModelWatcher != nil {
		if err := t.ModelWatcher.Start(); err != nil {
			log.Printf("[Trader] Warning: Failed to start model watcher: %v", err)
		} else {
			log.Println("[Trader] ✓ Model Watcher started")
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
	if t.Config.System.MultiStrategy && t.StrategyMgr != nil {
		log.Printf("[Trader] Mode: %s (Multi-Strategy)", t.Config.System.Mode)
		log.Printf("[Trader] Strategies: %d active", t.StrategyMgr.GetStrategyCount())
		for _, id := range t.StrategyMgr.GetStrategyIDs() {
			if cfg, ok := t.StrategyMgr.GetConfig(id); ok {
				log.Printf("[Trader]   - %s (%s): %v", id, cfg.Type, cfg.Symbols)
			}
		}
	} else {
		log.Printf("[Trader] Strategy: %s (%s)", t.Config.System.StrategyID, t.Config.Strategy.Type)
		log.Printf("[Trader] Mode: %s", t.Config.System.Mode)
		log.Printf("[Trader] Symbols: %v", t.Config.Strategy.Symbols)
	}
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

	// Stop model watcher
	if t.ModelWatcher != nil {
		if err := t.ModelWatcher.Stop(); err != nil {
			log.Printf("[Trader] Error stopping model watcher: %v", err)
		} else {
			log.Println("[Trader] ✓ Model Watcher stopped")
		}
	}

	// Stop strategies
	if t.Config.System.MultiStrategy && t.StrategyMgr != nil {
		// Multi-strategy mode
		if err := t.StrategyMgr.Stop(); err != nil {
			log.Printf("[Trader] Error stopping strategies: %v", err)
		} else {
			log.Println("[Trader] ✓ All strategies stopped")
		}
	} else if t.Strategy != nil {
		// Single-strategy mode
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
	status := map[string]interface{}{
		"running":        t.IsRunning(),
		"mode":           t.Config.System.Mode,
		"multi_strategy": t.Config.System.MultiStrategy,
	}

	if t.Config.System.MultiStrategy && t.StrategyMgr != nil {
		// Multi-strategy mode
		status["strategy_count"] = t.StrategyMgr.GetStrategyCount()
		status["manager_status"] = t.StrategyMgr.GetStatus()
		status["aggregated_pnl"] = t.StrategyMgr.GetAggregatedPNL()

		// For backward compatibility, also include first strategy info
		if t.Strategy != nil {
			status["strategy_id"] = t.Strategy.GetID()
			status["strategy"] = t.Strategy.GetStatus()
			status["position"] = t.Strategy.GetEstimatedPosition()
			status["pnl"] = t.Strategy.GetPNL()
			status["risk"] = t.Strategy.GetRiskMetrics()
		}
	} else {
		// Single-strategy mode
		status["strategy_id"] = t.Config.System.StrategyID
		status["strategy"] = t.Strategy.GetStatus()
		status["position"] = t.Strategy.GetEstimatedPosition()
		status["pnl"] = t.Strategy.GetPNL()
		status["risk"] = t.Strategy.GetRiskMetrics()
	}

	return status
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

// initializeSingleStrategy initializes in single-strategy mode (backward compatible)
func (t *Trader) initializeSingleStrategy() error {
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

	// Set initial activation state based on config
	t.setInitialActivationState(t.Strategy)

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

	return nil
}

// initializeMultiStrategy initializes in multi-strategy mode
func (t *Trader) initializeMultiStrategy() error {
	log.Printf("[Trader] Creating StrategyManager for %d strategies...",
		len(t.Config.GetEnabledStrategies()))

	// Create StrategyManager
	t.StrategyMgr = strategy.NewStrategyManager(t.Engine)

	// Load strategies from config
	strategyConfigs := t.Config.GetEnabledStrategies()
	if err := t.StrategyMgr.LoadStrategies(strategyConfigs); err != nil {
		return fmt.Errorf("failed to load strategies: %w", err)
	}

	// Set t.Strategy to first strategy for backward compatibility
	t.Strategy = t.StrategyMgr.GetFirstStrategy()
	if t.Strategy == nil {
		return fmt.Errorf("no strategies loaded")
	}

	// Set initial activation state for all strategies
	t.StrategyMgr.ForEach(func(id string, strat strategy.Strategy) {
		t.setInitialActivationState(strat)
	})

	// Add strategies to portfolio (if portfolio manager exists)
	if t.Portfolio != nil {
		t.StrategyMgr.ForEach(func(id string, strat strategy.Strategy) {
			allocation := t.StrategyMgr.GetAllocations()[id]
			if allocation == 0 {
				allocation = 1.0 / float64(t.StrategyMgr.GetStrategyCount())
			}
			if err := t.Portfolio.AddStrategy(strat, allocation); err != nil {
				log.Printf("[Trader] Warning: Failed to add strategy %s to portfolio: %v", id, err)
			}
		})
		log.Printf("[Trader] ✓ %d strategies added to portfolio", t.StrategyMgr.GetStrategyCount())
	}

	log.Printf("[Trader] ✓ StrategyManager initialized with %d strategies", t.StrategyMgr.GetStrategyCount())
	return nil
}

// setInitialActivationState sets initial activation state based on config
func (t *Trader) setInitialActivationState(strat strategy.Strategy) {
	if accessor, ok := strat.(strategy.BaseStrategyAccessor); ok {
		baseStrat := accessor.GetBaseStrategy()
		if baseStrat != nil {
			if t.Config.Session.AutoActivate {
				baseStrat.ControlState.Activate()
				log.Printf("[Trader] Strategy %s: Activated (auto_activate=true)", strat.GetID())
			} else {
				baseStrat.ControlState.Deactivate()
				log.Printf("[Trader] Strategy %s: NOT activated (auto_activate=false)", strat.GetID())
			}
		}
	}
}

// getAllSymbols returns all unique symbols across all strategies
func (t *Trader) getAllSymbols() []string {
	symbolSet := make(map[string]bool)

	if t.Config.System.MultiStrategy {
		for _, cfg := range t.Config.GetEnabledStrategies() {
			for _, symbol := range cfg.Symbols {
				symbolSet[symbol] = true
			}
		}
	} else {
		for _, symbol := range t.Config.Strategy.Symbols {
			symbolSet[symbol] = true
		}
	}

	symbols := make([]string, 0, len(symbolSet))
	for symbol := range symbolSet {
		symbols = append(symbols, symbol)
	}
	return symbols
}

// runSessionManager monitors trading sessions
func (t *Trader) runSessionManager() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for t.IsRunning() {
		<-ticker.C

		inSession := t.SessionMgr.IsInSession()

		if t.Config.System.MultiStrategy && t.StrategyMgr != nil {
			// Multi-strategy mode
			t.StrategyMgr.ForEach(func(id string, strat strategy.Strategy) {
				strategyRunning := strat.IsRunning()

				if inSession && !strategyRunning && t.Config.Session.AutoStart {
					log.Printf("[Trader] Trading session started - starting strategy %s", id)
					if err := strat.Start(); err != nil {
						log.Printf("[Trader] Error starting strategy %s: %v", id, err)
					}
				}

				if !inSession && strategyRunning && t.Config.Session.AutoStop {
					log.Printf("[Trader] Trading session ended - stopping strategy %s", id)
					if err := strat.Stop(); err != nil {
						log.Printf("[Trader] Error stopping strategy %s: %v", id, err)
					}
				}
			})
		} else {
			// Single-strategy mode
			strategyRunning := t.Strategy.IsRunning()

			if inSession && !strategyRunning && t.Config.Session.AutoStart {
				log.Println("[Trader] Trading session started - starting strategy")
				if err := t.Strategy.Start(); err != nil {
					log.Printf("[Trader] Error starting strategy: %v", err)
				}
			}

			if !inSession && strategyRunning && t.Config.Session.AutoStop {
				log.Println("[Trader] Trading session ended - stopping strategy")
				if err := t.Strategy.Stop(); err != nil {
					log.Printf("[Trader] Error stopping strategy: %v", err)
				}
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

		var strategies map[string]strategy.Strategy

		if t.Config.System.MultiStrategy && t.StrategyMgr != nil {
			// Multi-strategy mode: get all strategies
			strategies = t.StrategyMgr.GetAllStrategies()
		} else {
			// Single-strategy mode
			if !t.Strategy.IsRunning() {
				continue
			}
			strategies = map[string]strategy.Strategy{
				t.Config.System.StrategyID: t.Strategy,
			}
		}

		// Check each strategy's risk
		for id, strat := range strategies {
			if !strat.IsRunning() {
				continue
			}

			strategyAlerts := t.RiskManager.CheckStrategy(strat)
			for _, alert := range strategyAlerts {
				t.RiskManager.AddAlert(&alert)

				if alert.Action == "stop" {
					log.Printf("[Trader] RISK ALERT: Stopping strategy %s due to %s", id, alert.Message)
					if err := strat.Stop(); err != nil {
						log.Printf("[Trader] Error stopping strategy %s: %v", id, err)
					}
				}
			}
		}

		// Check global limits
		globalAlerts := t.RiskManager.CheckGlobal(strategies)
		for _, alert := range globalAlerts {
			t.RiskManager.AddAlert(&alert)

			if alert.Action == "emergency_stop" && !t.RiskManager.IsEmergencyStop() {
				log.Println("[Trader] EMERGENCY STOP triggered by global risk limits!")
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
			log.Println("[Trader] ════════════════════════════════════════════════════════════")
			log.Println("[Trader] Received SIGUSR1: Activating all strategies")
			log.Println("[Trader] ════════════════════════════════════════════════════════════")

			if t.Config.System.MultiStrategy && t.StrategyMgr != nil {
				// Multi-strategy mode: activate all
				if err := t.StrategyMgr.ActivateAll(); err != nil {
					log.Printf("[Trader] Error activating strategies: %v", err)
				} else {
					log.Printf("[Trader] ✓ %d strategies activated", t.StrategyMgr.GetStrategyCount())
				}
			} else {
				// Single-strategy mode
				baseStrat := t.getBaseStrategy()
				if baseStrat == nil {
					log.Println("[Trader] Error: Failed to access strategy control state")
					continue
				}

				// Reset control state
				baseStrat.ControlState.ExitRequested = false
				baseStrat.ControlState.CancelPending = false
				baseStrat.ControlState.FlattenMode = false
				if baseStrat.ControlState.RunState == strategy.StrategyRunStateStopped ||
					baseStrat.ControlState.RunState == strategy.StrategyRunStateFlattening {
					baseStrat.ControlState.RunState = strategy.StrategyRunStateActive
				}
				baseStrat.ControlState.Activate()

				if !t.Strategy.IsRunning() {
					if err := t.Strategy.Start(); err != nil {
						log.Printf("[Trader] Error starting strategy: %v", err)
					} else {
						log.Println("[Trader] ✓ Strategy activated and trading")
					}
				} else {
					log.Println("[Trader] ✓ Strategy already running, re-activated")
				}
			}

		case syscall.SIGUSR2:
			// Deactivate strategy and squareoff (对应 tbsrc SIGTSTP)
			log.Println("[Trader] ════════════════════════════════════════════════════════════")
			log.Println("[Trader] Received SIGUSR2: Deactivating all strategies (squareoff)")
			log.Println("[Trader] ════════════════════════════════════════════════════════════")

			if t.Config.System.MultiStrategy && t.StrategyMgr != nil {
				// Multi-strategy mode: deactivate all
				if err := t.StrategyMgr.DeactivateAll(); err != nil {
					log.Printf("[Trader] Error deactivating strategies: %v", err)
				} else {
					log.Printf("[Trader] ✓ %d strategies deactivated", t.StrategyMgr.GetStrategyCount())
				}
			} else {
				// Single-strategy mode
				baseStrat := t.getBaseStrategy()
				if baseStrat == nil {
					log.Println("[Trader] Error: Failed to access strategy control state")
					continue
				}

				baseStrat.TriggerFlatten(strategy.FlattenReasonManual, false)
				baseStrat.ControlState.Deactivate()
			}

			log.Println("[Trader] ✓ Strategies deactivated, positions being closed")
			log.Println("[Trader] Strategies will stop trading but process continues running")
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

// GetStrategyManager returns the strategy manager (for API access)
func (t *Trader) GetStrategyManager() *strategy.StrategyManager {
	return t.StrategyMgr
}

// IsMultiStrategy returns whether running in multi-strategy mode
func (t *Trader) IsMultiStrategy() bool {
	return t.Config.System.MultiStrategy && t.StrategyMgr != nil
}

// onModelReload handles model hot reload callback
func (t *Trader) onModelReload(newParams map[string]interface{}) error {
	log.Printf("[Trader] Processing model hot reload with %d parameters", len(newParams))

	if t.IsMultiStrategy() {
		// Multi-strategy mode: apply to all strategies
		log.Printf("[Trader] Applying new parameters to all %d strategies...", t.StrategyMgr.GetStrategyCount())
		var errs []error
		t.StrategyMgr.ForEach(func(id string, strat strategy.Strategy) {
			if err := strat.UpdateParameters(newParams); err != nil {
				errs = append(errs, fmt.Errorf("strategy %s: %w", id, err))
				log.Printf("[Trader] ✗ Failed to apply parameters to strategy %s: %v", id, err)
			} else {
				log.Printf("[Trader] ✓ Successfully applied parameters to strategy %s", id)
			}
		})

		if len(errs) > 0 {
			// In a real scenario, you might want to decide on a rollback strategy
			return fmt.Errorf("failed to apply parameters to some strategies: %v", errs)
		}

	} else {
		// Single-strategy mode (backward compatibility)
		log.Println("[Trader] Applying new parameters to single strategy...")
		baseStrat := t.getBaseStrategy()
		if baseStrat == nil {
			return fmt.Errorf("failed to access base strategy")
		}

		if err := baseStrat.UpdateParameters(newParams); err != nil {
			return fmt.Errorf("failed to apply new parameters: %w", err)
		}
	}

	log.Println("[Trader] ✓ Model parameters reloaded successfully for all applicable strategies")
	return nil
}

// ReloadModel manually triggers model reload
func (t *Trader) ReloadModel() error {
	if t.ModelWatcher == nil {
		return fmt.Errorf("model watcher not configured")
	}

	return t.ModelWatcher.Reload()
}

// GetModelStatus returns model watcher status
func (t *Trader) GetModelStatus() map[string]interface{} {
	if t.ModelWatcher == nil {
		return map[string]interface{}{
			"enabled": false,
			"message": "Model hot reload not configured",
		}
	}

	status := t.ModelWatcher.GetStatus()
	status["enabled"] = true

	// Add strategy current parameters (via BaseStrategy)
	baseStrat := t.getBaseStrategy()
	if baseStrat != nil {
		status["current_parameters"] = baseStrat.GetCurrentParameters()
	}

	return status
}

// GetModelReloadHistory returns model reload history
func (t *Trader) GetModelReloadHistory() []ModelReloadHistory {
	if t.ModelWatcher == nil {
		return []ModelReloadHistory{}
	}

	return t.ModelWatcher.GetHistory(10)
}
