package strategy

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/config"
)

// StrategyManager 多策略管理器
// 负责管理多个策略实例的生命周期、状态和资源分配
type StrategyManager struct {
	engine      *StrategyEngine              // 策略引擎（行情分发、订单发送）
	strategies  map[string]Strategy          // 策略实例 map[strategyID]Strategy
	configs     map[string]*config.StrategyItemConfig // 策略配置
	allocations map[string]float64           // 资金分配比例

	mu      sync.RWMutex
	running bool

	// 回调函数
	onStrategyStatusChange func(strategyID string, status *StrategyStatus)
}

// NewStrategyManager 创建策略管理器
func NewStrategyManager(engine *StrategyEngine) *StrategyManager {
	return &StrategyManager{
		engine:      engine,
		strategies:  make(map[string]Strategy),
		configs:     make(map[string]*config.StrategyItemConfig),
		allocations: make(map[string]float64),
		running:     false,
	}
}

// LoadStrategies 从配置加载所有策略
func (sm *StrategyManager) LoadStrategies(configs []config.StrategyItemConfig) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	log.Printf("[StrategyManager] Loading %d strategies...", len(configs))

	for _, cfg := range configs {
		if !cfg.Enabled {
			log.Printf("[StrategyManager] Strategy %s is disabled, skipping", cfg.ID)
			continue
		}

		if err := sm.addStrategyLocked(cfg); err != nil {
			return fmt.Errorf("failed to add strategy %s: %w", cfg.ID, err)
		}
	}

	log.Printf("[StrategyManager] Loaded %d strategies successfully", len(sm.strategies))
	return nil
}

// addStrategyLocked 添加策略（内部方法，需要持有锁）
func (sm *StrategyManager) addStrategyLocked(cfg config.StrategyItemConfig) error {
	// 检查是否已存在
	if _, exists := sm.strategies[cfg.ID]; exists {
		return fmt.Errorf("strategy %s already exists", cfg.ID)
	}

	// 创建策略实例
	strategy, err := sm.createStrategy(cfg)
	if err != nil {
		return fmt.Errorf("failed to create strategy: %w", err)
	}

	// 初始化策略
	strategyConfig := sm.toStrategyConfig(cfg)
	if err := strategy.Initialize(strategyConfig); err != nil {
		return fmt.Errorf("failed to initialize strategy: %w", err)
	}

	// 添加到 Engine
	if sm.engine != nil {
		if err := sm.engine.AddStrategy(strategy); err != nil {
			return fmt.Errorf("failed to add strategy to engine: %w", err)
		}
	}

	// 保存到管理器
	sm.strategies[cfg.ID] = strategy
	cfgCopy := cfg
	sm.configs[cfg.ID] = &cfgCopy
	sm.allocations[cfg.ID] = cfg.Allocation

	log.Printf("[StrategyManager] Added strategy: %s (type=%s, symbols=%v, allocation=%.2f)",
		cfg.ID, cfg.Type, cfg.Symbols, cfg.Allocation)

	return nil
}

// createStrategy 策略工厂
func (sm *StrategyManager) createStrategy(cfg config.StrategyItemConfig) (Strategy, error) {
	switch cfg.Type {
	case "passive":
		return NewPassiveStrategy(cfg.ID), nil
	case "aggressive":
		return NewAggressiveStrategy(cfg.ID), nil
	case "hedging":
		return NewHedgingStrategy(cfg.ID), nil
	case "pairwise_arb":
		return NewPairwiseArbStrategy(cfg.ID), nil
	// 可扩展更多策略类型
	// case "trend_following":
	// 	return NewTrendFollowingStrategy(cfg.ID), nil
	// case "grid":
	// 	return NewGridStrategy(cfg.ID), nil
	default:
		return nil, fmt.Errorf("unknown strategy type: %s", cfg.Type)
	}
}

// toStrategyConfig 将 StrategyItemConfig 转换为 StrategyConfig
func (sm *StrategyManager) toStrategyConfig(cfg config.StrategyItemConfig) *StrategyConfig {
	return &StrategyConfig{
		StrategyID:      cfg.ID,
		StrategyType:    cfg.Type,
		Symbols:         cfg.Symbols,
		MaxPositionSize: cfg.MaxPositionSize,
		Parameters:      cfg.Parameters,
		Enabled:         cfg.Enabled,
	}
}

// Start 启动所有策略
func (sm *StrategyManager) Start() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if sm.running {
		return fmt.Errorf("strategy manager already running")
	}

	log.Printf("[StrategyManager] Starting %d strategies...", len(sm.strategies))

	for id, strategy := range sm.strategies {
		if err := strategy.Start(); err != nil {
			log.Printf("[StrategyManager] Failed to start strategy %s: %v", id, err)
			// 继续启动其他策略，不中断
		} else {
			log.Printf("[StrategyManager] Strategy %s started", id)
		}
	}

	sm.running = true
	log.Printf("[StrategyManager] All strategies started")
	return nil
}

// Stop 停止所有策略
func (sm *StrategyManager) Stop() error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if !sm.running {
		return nil
	}

	log.Printf("[StrategyManager] Stopping %d strategies...", len(sm.strategies))

	for id, strategy := range sm.strategies {
		if err := strategy.Stop(); err != nil {
			log.Printf("[StrategyManager] Failed to stop strategy %s: %v", id, err)
		} else {
			log.Printf("[StrategyManager] Strategy %s stopped", id)
		}
	}

	sm.running = false
	log.Printf("[StrategyManager] All strategies stopped")
	return nil
}

// IsRunning 是否正在运行
func (sm *StrategyManager) IsRunning() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.running
}

// ==================== 策略操作 ====================

// AddStrategy 动态添加策略
func (sm *StrategyManager) AddStrategy(cfg config.StrategyItemConfig) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if err := sm.addStrategyLocked(cfg); err != nil {
		return err
	}

	// 如果管理器已在运行，自动启动新策略
	if sm.running {
		if strategy, ok := sm.strategies[cfg.ID]; ok {
			if err := strategy.Start(); err != nil {
				log.Printf("[StrategyManager] Failed to start newly added strategy %s: %v", cfg.ID, err)
			}
		}
	}

	return nil
}

// RemoveStrategy 动态移除策略
func (sm *StrategyManager) RemoveStrategy(strategyID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	strategy, exists := sm.strategies[strategyID]
	if !exists {
		return fmt.Errorf("strategy %s not found", strategyID)
	}

	// 停止策略
	if strategy.IsRunning() {
		if err := strategy.Stop(); err != nil {
			log.Printf("[StrategyManager] Warning: failed to stop strategy %s: %v", strategyID, err)
		}
	}

	// 从 Engine 移除
	if sm.engine != nil {
		if err := sm.engine.RemoveStrategy(strategyID); err != nil {
			log.Printf("[StrategyManager] Warning: failed to remove strategy from engine: %v", err)
		}
	}

	// 从管理器移除
	delete(sm.strategies, strategyID)
	delete(sm.configs, strategyID)
	delete(sm.allocations, strategyID)

	log.Printf("[StrategyManager] Removed strategy: %s", strategyID)
	return nil
}

// GetStrategy 获取策略实例
func (sm *StrategyManager) GetStrategy(strategyID string) (Strategy, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	strategy, ok := sm.strategies[strategyID]
	return strategy, ok
}

// GetAllStrategies 获取所有策略
func (sm *StrategyManager) GetAllStrategies() map[string]Strategy {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]Strategy, len(sm.strategies))
	for id, strategy := range sm.strategies {
		result[id] = strategy
	}
	return result
}

// GetStrategyIDs 获取所有策略ID
func (sm *StrategyManager) GetStrategyIDs() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	ids := make([]string, 0, len(sm.strategies))
	for id := range sm.strategies {
		ids = append(ids, id)
	}
	return ids
}

// GetStrategyCount 获取策略数量
func (sm *StrategyManager) GetStrategyCount() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.strategies)
}

// ==================== 策略控制 ====================

// ActivateStrategy 激活单个策略
func (sm *StrategyManager) ActivateStrategy(strategyID string) error {
	sm.mu.RLock()
	strategy, exists := sm.strategies[strategyID]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("strategy %s not found", strategyID)
	}

	// 通过 BaseStrategyAccessor 获取控制状态
	if accessor, ok := strategy.(BaseStrategyAccessor); ok {
		baseStrategy := accessor.GetBaseStrategy()
		if baseStrategy != nil {
			baseStrategy.ControlState.ExitRequested = false
			baseStrategy.ControlState.CancelPending = false
			baseStrategy.ControlState.FlattenMode = false
			if baseStrategy.ControlState.RunState == StrategyRunStateStopped ||
				baseStrategy.ControlState.RunState == StrategyRunStateFlattening {
				baseStrategy.ControlState.RunState = StrategyRunStateActive
			}
			baseStrategy.ControlState.Activate()
		}
	}

	// 启动策略（如果未运行）
	if !strategy.IsRunning() {
		if err := strategy.Start(); err != nil {
			return fmt.Errorf("failed to start strategy: %w", err)
		}
	}

	log.Printf("[StrategyManager] Strategy %s activated", strategyID)
	return nil
}

// DeactivateStrategy 停用单个策略
func (sm *StrategyManager) DeactivateStrategy(strategyID string) error {
	sm.mu.RLock()
	strategy, exists := sm.strategies[strategyID]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("strategy %s not found", strategyID)
	}

	// 通过 BaseStrategyAccessor 获取控制状态
	if accessor, ok := strategy.(BaseStrategyAccessor); ok {
		baseStrategy := accessor.GetBaseStrategy()
		if baseStrategy != nil {
			baseStrategy.TriggerFlatten(FlattenReasonManual, false)
			baseStrategy.ControlState.Deactivate()
		}
	}

	log.Printf("[StrategyManager] Strategy %s deactivated", strategyID)
	return nil
}

// ActivateAll 激活所有策略
func (sm *StrategyManager) ActivateAll() error {
	sm.mu.RLock()
	ids := make([]string, 0, len(sm.strategies))
	for id := range sm.strategies {
		ids = append(ids, id)
	}
	sm.mu.RUnlock()

	var lastErr error
	for _, id := range ids {
		if err := sm.ActivateStrategy(id); err != nil {
			log.Printf("[StrategyManager] Failed to activate strategy %s: %v", id, err)
			lastErr = err
		}
	}

	return lastErr
}

// DeactivateAll 停用所有策略
func (sm *StrategyManager) DeactivateAll() error {
	sm.mu.RLock()
	ids := make([]string, 0, len(sm.strategies))
	for id := range sm.strategies {
		ids = append(ids, id)
	}
	sm.mu.RUnlock()

	var lastErr error
	for _, id := range ids {
		if err := sm.DeactivateStrategy(id); err != nil {
			log.Printf("[StrategyManager] Failed to deactivate strategy %s: %v", id, err)
			lastErr = err
		}
	}

	return lastErr
}

// ==================== 状态查询 ====================

// StrategyManagerStatus 管理器状态
type StrategyManagerStatus struct {
	TotalStrategies   int                           `json:"total_strategies"`
	ActiveStrategies  int                           `json:"active_strategies"`
	RunningStrategies int                           `json:"running_strategies"`
	Allocations       map[string]float64            `json:"allocations"`
	StrategyStatuses  map[string]*StrategyStatusInfo `json:"strategy_statuses"`
}

// StrategyStatusInfo 策略状态信息
type StrategyStatusInfo struct {
	ID            string             `json:"id"`
	Type          string             `json:"type"`
	Running       bool               `json:"running"`
	Active        bool               `json:"active"`
	Symbols       []string           `json:"symbols"`
	Allocation    float64            `json:"allocation"`
	ConditionsMet bool               `json:"conditions_met"`
	Eligible      bool               `json:"eligible"`
	Indicators    map[string]float64 `json:"indicators"`
	Position      *Position          `json:"position"`
	PNL           *PNL               `json:"pnl"`
}

// GetStatus 获取管理器状态
func (sm *StrategyManager) GetStatus() *StrategyManagerStatus {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	status := &StrategyManagerStatus{
		TotalStrategies:  len(sm.strategies),
		Allocations:      make(map[string]float64),
		StrategyStatuses: make(map[string]*StrategyStatusInfo),
	}

	for id, alloc := range sm.allocations {
		status.Allocations[id] = alloc
	}

	for id, strategy := range sm.strategies {
		info := &StrategyStatusInfo{
			ID:       id,
			Type:     strategy.GetType(),
			Running:  strategy.IsRunning(),
			Position: strategy.GetPosition(),
			PNL:      strategy.GetPNL(),
		}

		// 获取配置信息
		if cfg, ok := sm.configs[id]; ok {
			info.Symbols = cfg.Symbols
			info.Allocation = cfg.Allocation
		}

		// 获取控制状态
		if accessor, ok := strategy.(BaseStrategyAccessor); ok {
			baseStrategy := accessor.GetBaseStrategy()
			if baseStrategy != nil {
				info.Active = baseStrategy.ControlState.IsActive()
				info.ConditionsMet = baseStrategy.ControlState.ConditionsMet
				info.Eligible = baseStrategy.ControlState.Eligible
				info.Indicators = baseStrategy.ControlState.Indicators
			}
		}

		if info.Running {
			status.RunningStrategies++
		}
		if info.Active {
			status.ActiveStrategies++
		}

		status.StrategyStatuses[id] = info
	}

	return status
}

// GetStrategyStatus 获取单个策略状态
func (sm *StrategyManager) GetStrategyStatus(strategyID string) (*StrategyStatusInfo, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	strategy, exists := sm.strategies[strategyID]
	if !exists {
		return nil, fmt.Errorf("strategy %s not found", strategyID)
	}

	info := &StrategyStatusInfo{
		ID:       strategyID,
		Type:     strategy.GetType(),
		Running:  strategy.IsRunning(),
		Position: strategy.GetPosition(),
		PNL:      strategy.GetPNL(),
	}

	if cfg, ok := sm.configs[strategyID]; ok {
		info.Symbols = cfg.Symbols
		info.Allocation = cfg.Allocation
	}

	if accessor, ok := strategy.(BaseStrategyAccessor); ok {
		baseStrategy := accessor.GetBaseStrategy()
		if baseStrategy != nil {
			info.Active = baseStrategy.ControlState.IsActive()
			info.ConditionsMet = baseStrategy.ControlState.ConditionsMet
			info.Eligible = baseStrategy.ControlState.Eligible
			info.Indicators = baseStrategy.ControlState.Indicators
		}
	}

	return info, nil
}

// AggregatedPNL 汇总 PNL
type AggregatedPNL struct {
	TotalRealizedPnL   float64        `json:"total_realized_pnl"`
	TotalUnrealizedPnL float64        `json:"total_unrealized_pnl"`
	TotalPnL           float64        `json:"total_pnl"`
	ByStrategy         map[string]*PNL `json:"by_strategy"`
}

// GetAggregatedPNL 获取汇总 PNL
func (sm *StrategyManager) GetAggregatedPNL() *AggregatedPNL {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	agg := &AggregatedPNL{
		ByStrategy: make(map[string]*PNL),
	}

	for id, strategy := range sm.strategies {
		pnl := strategy.GetPNL()
		if pnl != nil {
			agg.TotalRealizedPnL += pnl.RealizedPnL
			agg.TotalUnrealizedPnL += pnl.UnrealizedPnL
			agg.ByStrategy[id] = pnl
		}
	}

	agg.TotalPnL = agg.TotalRealizedPnL + agg.TotalUnrealizedPnL
	return agg
}

// ==================== 资源分配 ====================

// SetAllocation 设置策略资金分配
func (sm *StrategyManager) SetAllocation(strategyID string, allocation float64) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	if _, exists := sm.strategies[strategyID]; !exists {
		return fmt.Errorf("strategy %s not found", strategyID)
	}

	if allocation < 0 || allocation > 1 {
		return fmt.Errorf("allocation must be between 0 and 1")
	}

	sm.allocations[strategyID] = allocation
	log.Printf("[StrategyManager] Strategy %s allocation set to %.2f", strategyID, allocation)
	return nil
}

// GetAllocations 获取当前分配情况
func (sm *StrategyManager) GetAllocations() map[string]float64 {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	result := make(map[string]float64, len(sm.allocations))
	for id, alloc := range sm.allocations {
		result[id] = alloc
	}
	return result
}

// ==================== 风险管理 ====================

// TriggerGlobalFlatten 触发全局平仓
func (sm *StrategyManager) TriggerGlobalFlatten(reason string) error {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	log.Printf("[StrategyManager] Triggering global flatten: %s", reason)

	for id, strategy := range sm.strategies {
		if accessor, ok := strategy.(BaseStrategyAccessor); ok {
			baseStrategy := accessor.GetBaseStrategy()
			if baseStrategy != nil {
				baseStrategy.TriggerFlatten(FlattenReasonMaxLoss, false)
				log.Printf("[StrategyManager] Strategy %s flatten triggered", id)
			}
		}
	}

	return nil
}

// SetOnStrategyStatusChange 设置策略状态变化回调
func (sm *StrategyManager) SetOnStrategyStatusChange(callback func(strategyID string, status *StrategyStatus)) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.onStrategyStatusChange = callback
}

// GetFirstStrategy 获取第一个策略（向后兼容）
func (sm *StrategyManager) GetFirstStrategy() Strategy {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for _, strategy := range sm.strategies {
		return strategy
	}
	return nil
}

// GetConfig 获取策略配置
func (sm *StrategyManager) GetConfig(strategyID string) (*config.StrategyItemConfig, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	cfg, ok := sm.configs[strategyID]
	return cfg, ok
}

// ForEach 遍历所有策略执行操作
func (sm *StrategyManager) ForEach(fn func(id string, strategy Strategy)) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	for id, strategy := range sm.strategies {
		fn(id, strategy)
	}
}

// LastUpdate 最后更新时间
func (sm *StrategyManager) GetLastUpdateTime() time.Time {
	return time.Now() // TODO: 实现真实的最后更新时间追踪
}

// ==================== Model 热加载 ====================

// ReloadStrategyModel 重载单个策略的Model参数
func (sm *StrategyManager) ReloadStrategyModel(strategyID string) error {
	sm.mu.RLock()
	strategy, exists := sm.strategies[strategyID]
	cfg, cfgExists := sm.configs[strategyID]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("strategy %s not found", strategyID)
	}

	if !cfgExists || cfg.ModelFile == "" {
		return fmt.Errorf("strategy %s has no model file configured", strategyID)
	}

	if !cfg.HotReload.Enabled {
		return fmt.Errorf("strategy %s hot reload is not enabled", strategyID)
	}

	log.Printf("[StrategyManager] Reloading model for strategy %s from %s", strategyID, cfg.ModelFile)

	// 解析model文件
	parser := &config.ModelFileParser{FilePath: cfg.ModelFile}
	newParams, err := parser.Parse()
	if err != nil {
		return fmt.Errorf("failed to parse model file: %w", err)
	}

	// 转换参数
	strategyParams := config.ConvertModelToStrategyParams(newParams)

	// 应用参数到策略
	if accessor, ok := strategy.(BaseStrategyAccessor); ok {
		baseStrategy := accessor.GetBaseStrategy()
		if baseStrategy != nil {
			if err := baseStrategy.UpdateParameters(strategyParams); err != nil {
				return fmt.Errorf("failed to update parameters: %w", err)
			}
		}
	} else {
		return fmt.Errorf("strategy does not support parameter updates")
	}

	log.Printf("[StrategyManager] ✓ Strategy %s model reloaded successfully", strategyID)
	return nil
}

// GetStrategyModelStatus 获取策略Model状态
func (sm *StrategyManager) GetStrategyModelStatus(strategyID string) (map[string]interface{}, error) {
	sm.mu.RLock()
	cfg, exists := sm.configs[strategyID]
	sm.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("strategy %s not found", strategyID)
	}

	if cfg.ModelFile == "" {
		return map[string]interface{}{
			"enabled": false,
			"message": "Model file not configured",
		}, nil
	}

	// 获取文件信息
	fileInfo, err := config.GetFileInfo(cfg.ModelFile)
	if err != nil {
		return map[string]interface{}{
			"enabled":    cfg.HotReload.Enabled,
			"model_file": cfg.ModelFile,
			"error":      fmt.Sprintf("Failed to stat file: %v", err),
		}, nil
	}

	return map[string]interface{}{
		"enabled":       cfg.HotReload.Enabled,
		"model_file":    cfg.ModelFile,
		"last_mod_time": fileInfo.ModTime(),
		"file_size":     fileInfo.Size(),
	}, nil
}
