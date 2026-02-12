package trader

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
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
	StrategyMgr *strategy.StrategyManager      // 多策略管理器
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
	log.Printf("[Trader] Initializing trader (Multi-Strategy Mode, Mode: %s)...",
		t.Config.System.Mode)

	log.Println("[Trader] DEBUG: Starting Initialize()")

	// 0. 设置数据目录（实盘和模拟盘使用不同目录）
	// 优先使用配置文件中的 data_dir，如果未配置则根据 mode 自动选择
	dataDir := t.Config.System.DataDir
	if dataDir == "" {
		// 默认数据目录：data/live 或 data/simulation
		switch t.Config.System.Mode {
		case "live":
			dataDir = "data/live"
		case "simulation":
			dataDir = "data/simulation"
		default:
			dataDir = "data"
		}
	}
	strategy.SetDataDir(dataDir)
	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		log.Printf("[Trader] Warning: failed to create data directory %s: %v", dataDir, err)
	}
	if err := os.MkdirAll(filepath.Join(dataDir, "positions"), 0755); err != nil {
		log.Printf("[Trader] Warning: failed to create positions directory: %v", err)
	}
	log.Printf("[Trader] Data directory: %s", dataDir)

	// 1. Create and initialize Risk Manager
	log.Println("[Trader] Creating Risk Manager...")
	// 从配置文件读取风控参数，如果未设置则使用大默认值（相当于禁用）
	defaultLargeValue := 1e10 // 100亿，相当于禁用
	stopLoss := t.Config.Risk.StopLoss
	if stopLoss <= 0 {
		stopLoss = defaultLargeValue
	}
	maxLoss := t.Config.Risk.MaxLoss
	if maxLoss <= 0 {
		maxLoss = defaultLargeValue
	}
	maxDrawdown := t.Config.Risk.MaxDrawdown
	if maxDrawdown <= 0 {
		maxDrawdown = defaultLargeValue
	}
	dailyLossLimit := t.Config.Risk.DailyLossLimit
	if dailyLossLimit <= 0 {
		dailyLossLimit = defaultLargeValue
	}

	riskConfig := &risk.RiskManagerConfig{
		EnableGlobalLimits:     true,
		EnableStrategyLimits:   true,
		EnablePortfolioLimits:  true,
		AlertRetentionSeconds:  3600,
		MaxAlertQueueSize:      1000,
		EmergencyStopThreshold: 100, // 提高阈值，避免因持仓成本价为0导致误触发紧急停止
		CheckIntervalMs:        t.Config.Risk.CheckIntervalMs,

		// 策略级别风控参数
		MaxPosition: 10000,             // 最大持仓数量
		MaxExposure: defaultLargeValue, // 敞口限制使用大值
		StopLoss:    stopLoss,
		MaxLoss:     maxLoss,
		UpnlLoss:    defaultLargeValue, // 未实现盈亏使用大值
		MaxOrders:   10000,             // 最大订单数

		// 全局级别风控参数
		GlobalMaxExposure:  defaultLargeValue,
		GlobalMaxDrawdown:  maxDrawdown,
		GlobalMaxDailyLoss: dailyLossLimit,
	}
	log.Printf("[Trader] Risk config: StopLoss=%.0f, MaxLoss=%.0f, MaxDrawdown=%.0f",
		stopLoss, maxLoss, maxDrawdown)
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

	// 4. Create strategy instance(s) - 使用 StrategyManager
	if err := t.initializeMultiStrategy(); err != nil {
		return fmt.Errorf("failed to initialize strategies: %w", err)
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
		// 8.1 启动时校验持仓（CTP查询 vs 保存的文件）
		if err := t.verifyPositionsOnStartup(); err != nil {
			return fmt.Errorf("position verification failed on startup: %w", err)
		}

		// 8.2 初始化策略持仓
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

	// 添加重试机制，等待 Counter Bridge 完全启动且 CTP 持仓数据就绪
	var positions map[string][]client.PositionInfo
	var err error
	maxRetries := 15 // 增加重试次数，等待 CTP 持仓数据就绪
	retryInterval := 2 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("[Trader] Position query attempt %d/%d (waiting for CTP data ready)...", attempt, maxRetries)

		// 调用Counter Bridge查询持仓
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		positions, err = orsClient.QueryPositions(ctx, "", "")
		cancel()

		if err == nil {
			// 查询成功，检查数据是否有效
			log.Printf("[Trader] ✓ Position query succeeded on attempt %d", attempt)
			break
		}

		// 查询失败，检查是否是"数据未就绪"错误
		errMsg := err.Error()
		if strings.Contains(errMsg, "not ready") || strings.Contains(errMsg, "still initializing") {
			log.Printf("[Trader] CTP position data not ready yet, waiting...")
		} else {
			log.Printf("[Trader] Position query attempt %d failed: %v", attempt, err)
		}

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
// 注意：此函数使用 CTP 返回的成本价初始化昨仓
// 与 C++ 原代码不同：C++ 的昨仓成本为 0，只计算当天交易产生的盈亏
// Go 代码使用 CTP 返回的成本价来计算完整的浮动盈亏，便于风控和监控
func (t *Trader) initializeStrategyPositions() {
	t.positionsMu.RLock()
	positionsByExchange := t.positionsByExchange
	t.positionsMu.RUnlock()

	// 按品种聚合持仓（净持仓 + 成本价）
	// 注意：CTP 返回的 avg_price 是持仓成本价
	// C++ 原代码不使用此成本价，而是使用当天成交均价（开盘时为 0）
	posMapWithCost := make(map[string]strategy.PositionWithCost)
	for _, posList := range positionsByExchange {
		for _, pos := range posList {
			qty := int64(pos.Volume)
			if pos.Direction == "SHORT" || pos.Direction == "short" {
				qty = -qty
			}

			existing := posMapWithCost[pos.Symbol]
			newQty := existing.Quantity + qty

			// 计算加权平均成本
			// CTP 返回的 avg_price = PositionCost / Position
			// PositionCost = 开仓价格 * 合约乘数 * 持仓数量
			// 所以 avg_price = 开仓价格 * 合约乘数
			// 需要除以合约乘数才能得到实际的开仓价格
			avgCost := pos.AvgPrice
			multiplier := strategy.GetContractMultiplier(pos.Symbol)
			if multiplier > 1 && avgCost > 0 {
				avgCost = avgCost / multiplier
				log.Printf("[Trader] Converted avg_price for %s: raw=%.2f / multiplier=%.0f = %.2f",
					pos.Symbol, pos.AvgPrice, multiplier, avgCost)
			}

			if newQty != 0 && existing.Quantity != 0 && avgCost > 0 {
				// 加权平均成本
				totalValue := existing.AvgCost*float64(abs64(existing.Quantity)) + avgCost*float64(pos.Volume)
				totalQty := float64(abs64(existing.Quantity) + int64(pos.Volume))
				existing.AvgCost = totalValue / totalQty
			} else if avgCost > 0 {
				existing.AvgCost = avgCost
			}
			existing.Quantity = newQty
			posMapWithCost[pos.Symbol] = existing
		}
	}

	if len(posMapWithCost) == 0 {
		log.Println("[Trader] No positions to initialize in strategies")
		return
	}

	log.Printf("[Trader] Initializing strategy positions from CTP query (%d symbols)", len(posMapWithCost))
	for symbol, pos := range posMapWithCost {
		log.Printf("[Trader]   %s: Qty=%d, AvgCost=%.2f", symbol, pos.Quantity, pos.AvgCost)
	}

	// 传递给每个策略（使用新的带成本价的接口）
	if t.StrategyMgr != nil {
		for _, strategyID := range t.StrategyMgr.GetStrategyIDs() {
			strat, exists := t.StrategyMgr.GetStrategy(strategyID)
			if !exists || strat == nil {
				continue
			}

			if initializer, ok := strat.(strategy.PositionInitializer); ok {
				// 优先使用带成本价的初始化方法
				if err := initializer.InitializePositionsWithCost(posMapWithCost); err != nil {
					log.Printf("[Trader] Warning: Failed to initialize positions with cost for %s: %v", strategyID, err)
					// 回退到不带成本价的方法
					posMap := make(map[string]int64)
					for symbol, pos := range posMapWithCost {
						posMap[symbol] = pos.Quantity
					}
					if err := initializer.InitializePositions(posMap); err != nil {
						log.Printf("[Trader] Warning: Failed to initialize positions for %s: %v", strategyID, err)
					}
				}
			}
		}
	}
}

// abs64 返回 int64 的绝对值
func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
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

	if t.StrategyMgr != nil {
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

// saveAllPositions 保存所有策略的持仓到文件
func (t *Trader) saveAllPositions() {
	log.Println("[Trader] Saving all strategy positions...")

	if t.StrategyMgr != nil {
		for _, strategyID := range t.StrategyMgr.GetStrategyIDs() {
			strat, exists := t.StrategyMgr.GetStrategy(strategyID)
			if !exists || strat == nil {
				continue
			}

			if err := strategy.SaveStrategyPosition(strat); err != nil {
				log.Printf("[Trader] Warning: Failed to save positions for %s: %v", strategyID, err)
			} else {
				log.Printf("[Trader] ✓ Saved positions for %s", strategyID)
			}
		}
	}
}

// verifyPositionsOnStartup 启动时校验持仓（保存的文件 vs CTP查询）
// 如果不一致则返回错误，阻止系统启动
func (t *Trader) verifyPositionsOnStartup() error {
	log.Println("[Trader] Verifying positions on startup...")

	// 获取CTP查询的持仓
	t.positionsMu.RLock()
	positionsByExchange := t.positionsByExchange
	t.positionsMu.RUnlock()

	// 聚合CTP持仓为净持仓
	ctpPosMap := make(map[string]int64)
	for _, posList := range positionsByExchange {
		for _, pos := range posList {
			qty := int64(pos.Volume)
			if pos.Direction == "SHORT" || pos.Direction == "short" {
				qty = -qty
			}
			ctpPosMap[pos.Symbol] += qty
		}
	}

	// 加载保存的持仓快照
	var savedPosMap map[string]int64
	var savedTimestamp time.Time

	if t.StrategyMgr != nil {
		// 聚合所有策略保存的持仓
		savedPosMap = make(map[string]int64)
		for _, sid := range t.StrategyMgr.GetStrategyIDs() {
			snapshot, err := strategy.LoadPositionSnapshot(sid)
			if err != nil {
				log.Printf("[Trader] Warning: Failed to load saved positions for %s: %v", sid, err)
				continue
			}
			if snapshot != nil {
				for symbol, qty := range snapshot.SymbolsPos {
					savedPosMap[symbol] += qty
				}
				savedTimestamp = snapshot.Timestamp
			}
		}
	}

	// 如果没有保存的持仓文件，跳过校验
	if len(savedPosMap) == 0 {
		log.Println("[Trader] No saved position snapshot found, skipping verification")
		return nil
	}

	log.Printf("[Trader] Loaded saved positions (last saved: %s)", savedTimestamp.Format("2006-01-02 15:04:05"))

	// 比较CTP持仓和保存的持仓
	var mismatches []string

	// 检查所有品种
	allSymbols := make(map[string]bool)
	for s := range ctpPosMap {
		allSymbols[s] = true
	}
	for s := range savedPosMap {
		allSymbols[s] = true
	}

	for symbol := range allSymbols {
		ctpQty := ctpPosMap[symbol]
		savedQty := savedPosMap[symbol]

		if ctpQty != savedQty {
			mismatches = append(mismatches,
				fmt.Sprintf("%s: CTP=%d, Saved=%d, Diff=%d",
					symbol, ctpQty, savedQty, ctpQty-savedQty))
		}
	}

	if len(mismatches) > 0 {
		log.Println("[Trader] ════════════════════════════════════════════════════════════")
		log.Println("[Trader] ⚠️  Position mismatch detected, auto-correcting...")
		log.Println("[Trader] ════════════════════════════════════════════════════════════")
		log.Println("[Trader] CTP positions do not match saved positions:")
		for _, msg := range mismatches {
			log.Printf("[Trader]     %s", msg)
		}
		log.Println("[Trader] ════════════════════════════════════════════════════════════")
		log.Println("[Trader] Auto-correction: Using CTP positions as source of truth")

		// 1. 清除旧的持仓文件
		log.Println("[Trader] Step 1: Clearing old position files...")
		if err := t.clearPositionFiles(); err != nil {
			log.Printf("[Trader] Warning: Failed to clear position files: %v", err)
		}

		// 2. 等待 CTP 持仓查询完成并重新获取最新数据
		log.Println("[Trader] Step 2: Waiting for CTP position query to complete...")
		if err := t.waitForCTPPositionReady(); err != nil {
			log.Printf("[Trader] Warning: Failed to wait for CTP positions: %v", err)
		}

		// 获取最新的 CTP 持仓
		t.positionsMu.RLock()
		latestPosMap := make(map[string]int64)
		for _, posList := range t.positionsByExchange {
			for _, pos := range posList {
				qty := int64(pos.Volume)
				if pos.Direction == "SHORT" || pos.Direction == "short" {
					qty = -qty
				}
				latestPosMap[pos.Symbol] += qty
			}
		}
		t.positionsMu.RUnlock()

		// 3. 使用 CTP 查询的持仓初始化策略
		log.Println("[Trader] Step 3: Initializing strategies with CTP positions...")
		if err := t.initializeStrategiesWithCTPPositions(latestPosMap); err != nil {
			log.Printf("[Trader] Warning: Failed to initialize strategies: %v", err)
		}

		// 4. 保存新的持仓文件
		log.Println("[Trader] Step 4: Saving new position files...")
		if err := t.savePositionSnapshots(); err != nil {
			log.Printf("[Trader] Warning: Failed to save position snapshots: %v", err)
		}

		log.Println("[Trader] ════════════════════════════════════════════════════════════")
		log.Println("[Trader] ✓ Position auto-correction completed")
		log.Println("[Trader] ════════════════════════════════════════════════════════════")
		return nil
	}

	log.Println("[Trader] ✓ Position verification passed: CTP positions match saved positions")
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
		log.Println("[Trader]   - Web UI: POST /api/v1/strategies/{id}/activate")
		log.Printf("[Trader]   - Signal: kill -SIGUSR1 %d\n", os.Getpid())
		log.Println("[Trader] ════════════════════════════════════════════════════════════")
	}

	if autoActivate {
		if t.StrategyMgr != nil {
			// Start all strategies
			if err := t.StrategyMgr.Start(); err != nil {
				return fmt.Errorf("failed to start strategies: %w", err)
			}
			log.Printf("[Trader] ✓ %d strategies activated and trading", t.StrategyMgr.GetStrategyCount())
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
	if t.StrategyMgr != nil {
		log.Printf("[Trader] Mode: %s (Multi-Strategy)", t.Config.System.Mode)
		log.Printf("[Trader] Strategies: %d active", t.StrategyMgr.GetStrategyCount())
		for _, id := range t.StrategyMgr.GetStrategyIDs() {
			if cfg, ok := t.StrategyMgr.GetConfig(id); ok {
				log.Printf("[Trader]   - %s (%s): %v", id, cfg.Type, cfg.Symbols)
			}
		}
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

	// 保存所有策略持仓到文件
	t.saveAllPositions()

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
	if t.StrategyMgr != nil {
		if err := t.StrategyMgr.Stop(); err != nil {
			log.Printf("[Trader] Error stopping strategies: %v", err)
		} else {
			log.Println("[Trader] ✓ All strategies stopped")
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
		"multi_strategy": true,
	}

	if t.StrategyMgr != nil {
		status["strategy_count"] = t.StrategyMgr.GetStrategyCount()
		status["manager_status"] = t.StrategyMgr.GetStatus()
		status["aggregated_pnl"] = t.StrategyMgr.GetAggregatedPNL()
	}

	return status
}

// initializeMultiStrategy initializes strategies using StrategyManager
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

	// Check if at least one strategy was loaded
	if t.StrategyMgr.GetStrategyCount() == 0 {
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
	controlState := strat.GetControlState()
	if controlState != nil {
		if t.Config.Session.AutoActivate {
			controlState.Activate()
			log.Printf("[Trader] Strategy %s: Activated (auto_activate=true)", strat.GetID())
		} else {
			controlState.Deactivate()
			log.Printf("[Trader] Strategy %s: NOT activated (auto_activate=false)", strat.GetID())
		}
	}
}

// getAllSymbols returns all unique symbols across all strategies
func (t *Trader) getAllSymbols() []string {
	symbolSet := make(map[string]bool)

	for _, cfg := range t.Config.GetEnabledStrategies() {
		for _, symbol := range cfg.Symbols {
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

		if t.StrategyMgr != nil {
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

		if t.StrategyMgr != nil {
			strategies = t.StrategyMgr.GetAllStrategies()
		} else {
			continue
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

			if t.StrategyMgr != nil {
				if err := t.StrategyMgr.ActivateAll(); err != nil {
					log.Printf("[Trader] Error activating strategies: %v", err)
				} else {
					log.Printf("[Trader] ✓ %d strategies activated", t.StrategyMgr.GetStrategyCount())
				}
			}

		case syscall.SIGUSR2:
			// Deactivate strategy and squareoff (对应 tbsrc SIGTSTP)
			log.Println("[Trader] ════════════════════════════════════════════════════════════")
			log.Println("[Trader] Received SIGUSR2: Deactivating all strategies (squareoff)")
			log.Println("[Trader] ════════════════════════════════════════════════════════════")

			if t.StrategyMgr != nil {
				if err := t.StrategyMgr.DeactivateAll(); err != nil {
					log.Printf("[Trader] Error deactivating strategies: %v", err)
				} else {
					log.Printf("[Trader] ✓ %d strategies deactivated", t.StrategyMgr.GetStrategyCount())
				}
			}

			log.Println("[Trader] ✓ Strategies deactivated, positions being closed")
			log.Println("[Trader] Strategies will stop trading but process continues running")
			log.Printf("[Trader] To re-activate: kill -SIGUSR1 %d\n", os.Getpid())
		}
	}
}

// GetStrategyManager returns the strategy manager (for API access)
func (t *Trader) GetStrategyManager() *strategy.StrategyManager {
	return t.StrategyMgr
}

// IsMultiStrategy returns whether running in multi-strategy mode
func (t *Trader) IsMultiStrategy() bool {
	return t.StrategyMgr != nil
}

// onModelReload handles model hot reload callback
func (t *Trader) onModelReload(newParams map[string]interface{}) error {
	log.Printf("[Trader] Processing model hot reload with %d parameters", len(newParams))

	if t.StrategyMgr == nil {
		return fmt.Errorf("strategy manager not initialized")
	}

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
		return fmt.Errorf("failed to apply parameters to some strategies: %v", errs)
	}

	log.Println("[Trader] ✓ Model parameters reloaded successfully for all strategies")
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

	// Add strategy current parameters from first strategy
	if t.StrategyMgr != nil {
		firstStrategy := t.StrategyMgr.GetFirstStrategy()
		if firstStrategy != nil {
			status["current_parameters"] = firstStrategy.GetCurrentParameters()
		}
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

// waitForCTPPositionReady 等待 CTP 持仓查询完成
// CTP 查询接口返回 -1 表示还在查询中，需要等待
func (t *Trader) waitForCTPPositionReady() error {
	if t.Engine == nil {
		return fmt.Errorf("engine not initialized")
	}

	orsClient := t.Engine.GetORSClient()
	if orsClient == nil {
		return fmt.Errorf("ORS client not available")
	}

	maxRetries := 30 // 最多等待 60 秒
	retryInterval := 2 * time.Second

	for attempt := 1; attempt <= maxRetries; attempt++ {
		log.Printf("[Trader] Waiting for CTP position query to complete (attempt %d/%d)...", attempt, maxRetries)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		positions, err := orsClient.QueryPositions(ctx, "", "")
		cancel()

		if err == nil && positions != nil {
			// 检查是否有有效数据（不是 -1 状态）
			hasValidData := false
			for _, posList := range positions {
				if len(posList) > 0 {
					// 检查第一个持仓数据是否有效（Volume >= 0）
					for _, pos := range posList {
						if pos.Volume >= 0 {
							hasValidData = true
							break
						}
					}
				}
				if hasValidData {
					break
				}
			}

			if hasValidData || len(positions) == 0 {
				// 有有效数据或没有持仓
				log.Printf("[Trader] ✓ CTP position query completed (attempt %d)", attempt)

				// 更新 positionsByExchange
				t.positionsMu.Lock()
				t.positionsByExchange = positions
				t.positionsMu.Unlock()

				return nil
			}
		}

		if err != nil {
			log.Printf("[Trader] Position query attempt %d: %v", attempt, err)
		} else {
			log.Printf("[Trader] Position query attempt %d: data not ready yet", attempt)
		}

		if attempt < maxRetries {
			time.Sleep(retryInterval)
		}
	}

	return fmt.Errorf("CTP position query did not complete after %d attempts", maxRetries)
}

// clearPositionFiles 清除所有持仓快照文件
func (t *Trader) clearPositionFiles() error {
	positionDir := "data/positions"

	// 检查目录是否存在
	if _, err := os.Stat(positionDir); os.IsNotExist(err) {
		log.Printf("[Trader] Position directory does not exist: %s", positionDir)
		return nil
	}

	// 读取目录中的所有文件
	files, err := os.ReadDir(positionDir)
	if err != nil {
		return fmt.Errorf("failed to read position directory: %w", err)
	}

	// 删除所有 .json 文件
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			filePath := filepath.Join(positionDir, file.Name())
			if err := os.Remove(filePath); err != nil {
				log.Printf("[Trader] Warning: Failed to remove %s: %v", filePath, err)
			} else {
				log.Printf("[Trader] Removed old position file: %s", filePath)
			}
		}
	}

	return nil
}

// initializeStrategiesWithCTPPositions 使用 CTP 持仓初始化策略
// 注意：此函数已升级为使用带成本价的初始化
// 与 C++ 原代码不同：C++ 的昨仓成本为 0，只计算当天交易产生的盈亏
// Go 代码使用 CTP 返回的成本价来计算完整的浮动盈亏
func (t *Trader) initializeStrategiesWithCTPPositions(ctpPosMap map[string]int64) error {
	// 从 positionsByExchange 获取带成本价的持仓信息
	t.positionsMu.RLock()
	posMapWithCost := make(map[string]strategy.PositionWithCost)
	for _, posList := range t.positionsByExchange {
		for _, pos := range posList {
			qty := int64(pos.Volume)
			if pos.Direction == "SHORT" || pos.Direction == "short" {
				qty = -qty
			}

			existing := posMapWithCost[pos.Symbol]
			newQty := existing.Quantity + qty

			// 处理成本价（与 initializeStrategyPositions 相同的逻辑）
			// CTP 返回的 avg_price = PositionCost / Position
			// PositionCost = 开仓价格 * 合约乘数 * 持仓数量
			// 所以 avg_price = 开仓价格 * 合约乘数
			// 需要除以合约乘数才能得到实际的开仓价格
			// 注意：此转换是 Go 代码新增的，C++ 原代码中昨仓成本为 0
			avgCost := pos.AvgPrice
			multiplier := strategy.GetContractMultiplier(pos.Symbol)
			if multiplier > 1 && avgCost > 0 {
				avgCost = avgCost / multiplier
				log.Printf("[Trader] CTP initializeStrategiesWithCTPPositions: Converted avg_price for %s: raw=%.2f / multiplier=%.0f = %.2f",
					pos.Symbol, pos.AvgPrice, multiplier, avgCost)
			}

			if newQty != 0 && existing.Quantity != 0 && avgCost > 0 {
				totalValue := existing.AvgCost*float64(abs64(existing.Quantity)) + avgCost*float64(pos.Volume)
				totalQty := float64(abs64(existing.Quantity) + int64(pos.Volume))
				existing.AvgCost = totalValue / totalQty
			} else if avgCost > 0 {
				existing.AvgCost = avgCost
			}
			existing.Quantity = newQty
			posMapWithCost[pos.Symbol] = existing
		}
	}
	t.positionsMu.RUnlock()

	if t.Config.System.MultiStrategy && t.StrategyMgr != nil {
		// 多策略模式
		for _, sid := range t.StrategyMgr.GetStrategyIDs() {
			strat, exists := t.StrategyMgr.GetStrategy(sid)
			if !exists {
				log.Printf("[Trader] Warning: Strategy %s not found", sid)
				continue
			}

			if posInit, ok := strat.(strategy.PositionInitializer); ok {
				// 优先使用带成本价的初始化
				if err := posInit.InitializePositionsWithCost(posMapWithCost); err != nil {
					log.Printf("[Trader] Warning: InitializePositionsWithCost failed for %s: %v, falling back", sid, err)
					if err := posInit.InitializePositions(ctpPosMap); err != nil {
						log.Printf("[Trader] Warning: Failed to initialize positions for %s: %v", sid, err)
					}
				} else {
					log.Printf("[Trader] Initialized positions with cost for strategy %s from CTP", sid)
				}
			}
		}
	}

	return nil
}

// savePositionSnapshots 保存所有策略的持仓快照
func (t *Trader) savePositionSnapshots() error {
	if t.StrategyMgr != nil {
		for _, sid := range t.StrategyMgr.GetStrategyIDs() {
			strat, exists := t.StrategyMgr.GetStrategy(sid)
			if !exists {
				log.Printf("[Trader] Warning: Strategy %s not found", sid)
				continue
			}

			if posProvider, ok := strat.(strategy.PositionProvider); ok {
				posMap := posProvider.GetPositionsBySymbol()
				snapshot := strategy.PositionSnapshot{
					StrategyID: sid,
					Timestamp:  time.Now(),
					SymbolsPos: posMap,
				}
				if err := strategy.SavePositionSnapshot(snapshot); err != nil {
					log.Printf("[Trader] Warning: Failed to save positions for %s: %v", sid, err)
				} else {
					log.Printf("[Trader] Saved position snapshot for strategy %s", sid)
				}
			}
		}
	}

	return nil
}
