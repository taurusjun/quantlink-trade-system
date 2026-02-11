package strategy

import (
	"log"
	"sync"
	"time"

	"github.com/yourusername/quantlink-trade-system/pkg/indicators"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

// Strategy is the interface that all trading strategies must implement
// C++: 对应 ExecutionStrategy 基类的虚函数
//
// ExecutionStrategy.h 虚函数映射:
//   virtual void Reset()                    -> Reset()
//   virtual void ORSCallBack(ResponseMsg*)  -> OnOrderUpdate()
//   virtual void MDCallBack(MarketUpdateNew*) -> OnMarketData()
//   virtual void AuctionCallBack(MarketUpdateNew*) -> OnAuctionData()
//   virtual void SendOrder() = 0            -> SendOrder() [纯虚函数]
//   virtual void OnTradeUpdate()            -> OnTradeUpdate()
//   virtual void CheckSquareoff()           -> CheckSquareoff()
//   virtual void HandleSquareON()           -> HandleSquareON()
//   virtual void HandleSquareoff()          -> HandleSquareoff()
//   virtual void SetThresholds()            -> SetThresholds()
//   virtual void SetTargetValue()           -> SetTargetValue()
type Strategy interface {
	// === C++ 虚函数对应 ===

	// Reset resets the strategy to initial state
	// C++: virtual void Reset()
	Reset()

	// OnOrderUpdate is called when order status changes
	// C++: virtual void ORSCallBack(ResponseMsg*)
	OnOrderUpdate(update *orspb.OrderUpdate)

	// OnMarketData is called when new market data arrives (continuous trading)
	// C++: virtual void MDCallBack(MarketUpdateNew*)
	OnMarketData(md *mdpb.MarketDataUpdate)

	// OnAuctionData is called when auction period market data arrives
	// C++: virtual void AuctionCallBack(MarketUpdateNew*)
	OnAuctionData(md *mdpb.MarketDataUpdate)

	// SendOrder generates and sends orders based on current state
	// C++: virtual void SendOrder() = 0 [纯虚函数，子类必须实现]
	SendOrder()

	// OnTradeUpdate is called after a trade is processed
	// C++: virtual void OnTradeUpdate() {}
	OnTradeUpdate()

	// CheckSquareoff checks if position needs to be squared off
	// C++: virtual void CheckSquareoff(MarketUpdateNew*)
	CheckSquareoff()

	// HandleSquareON handles square off initiation
	// C++: virtual void HandleSquareON()
	HandleSquareON()

	// HandleSquareoff executes the square off logic
	// C++: virtual void HandleSquareoff()
	HandleSquareoff()

	// SetThresholds sets dynamic thresholds based on position
	// C++: virtual void SetThresholds()
	SetThresholds()

	// === Go 特有方法（策略管理和状态查询）===

	// GetID returns the unique strategy ID
	GetID() string

	// GetType returns the strategy type name
	GetType() string

	// Initialize initializes the strategy with configuration
	Initialize(config *StrategyConfig) error

	// Start starts the strategy
	Start() error

	// Stop stops the strategy
	Stop() error

	// IsRunning returns true if strategy is running
	IsRunning() bool

	// OnTimer is called periodically
	OnTimer(now time.Time)

	// GetSignals returns pending trading signals
	GetSignals() []*TradingSignal

	// GetEstimatedPosition returns current estimated position
	GetEstimatedPosition() *EstimatedPosition

	// GetPosition is an alias for GetEstimatedPosition (for compatibility)
	GetPosition() *EstimatedPosition

	// GetPNL returns current P&L
	GetPNL() *PNL

	// GetRiskMetrics returns current risk metrics
	GetRiskMetrics() *RiskMetrics

	// GetStatus returns strategy status
	GetStatus() *StrategyStatus

	// GetControlState returns the strategy control state
	GetControlState() *StrategyControlState

	// GetConfig returns the strategy configuration
	GetConfig() *StrategyConfig

	// UpdateParameters updates strategy parameters (for hot reload)
	UpdateParameters(params map[string]interface{}) error

	// GetCurrentParameters returns current strategy parameters
	GetCurrentParameters() map[string]interface{}

	// === Engine/Manager 需要的方法 ===

	// CanSendOrder returns true if strategy can send orders
	// C++: 对应 !m_onFlat && m_Active 检查
	CanSendOrder() bool

	// SetLastMarketData stores the last market data for a symbol (for WebSocket push)
	SetLastMarketData(symbol string, md *mdpb.MarketDataUpdate)

	// GetLastMarketData returns the last market data for a symbol
	GetLastMarketData(symbol string) *mdpb.MarketDataUpdate

	// TriggerFlatten triggers position flattening
	TriggerFlatten(reason FlattenReason, aggressive bool)

	// GetPendingCancels returns orders pending cancellation
	GetPendingCancels() []*orspb.OrderUpdate
}

// ParameterUpdatable is an interface for strategies that support hot parameter reload
type ParameterUpdatable interface {
	// ApplyParameters applies new parameters to the strategy
	// Each strategy implements this to map generic parameters to strategy-specific fields
	ApplyParameters(params map[string]interface{}) error

	// GetCurrentParameters returns current strategy parameters
	GetCurrentParameters() map[string]interface{}
}

// IndicatorAwareStrategy is an optional interface for strategies that need
// to be notified when shared indicators are updated (like tbsrc INDCallBack).
// This allows strategies to insert custom logic between indicator calculation
// and signal generation.
type IndicatorAwareStrategy interface {
	// OnIndicatorUpdate is called after shared indicators are updated for a symbol
	OnIndicatorUpdate(symbol string, indicators *indicators.IndicatorLibrary)
}

// DetailedOrderStrategy is an optional interface for strategies that need
// fine-grained order event callbacks (more granular than OnOrderUpdate).
type DetailedOrderStrategy interface {
	// OnOrderNew is called when order is confirmed by exchange
	OnOrderNew(update *orspb.OrderUpdate)

	// OnOrderFilled is called when order is filled (partially or fully)
	OnOrderFilled(update *orspb.OrderUpdate)

	// OnOrderCanceled is called when order is canceled
	OnOrderCanceled(update *orspb.OrderUpdate)

	// OnOrderRejected is called when order is rejected
	OnOrderRejected(update *orspb.OrderUpdate)
}

// StrategyDataProvider 提供策略数据给外部系统（WebSocket、REST API等）
// 与核心 Strategy 接口分离，职责单一
// 由 StrategyDataContext 实现，具体策略通过嵌入自动获得
type StrategyDataProvider interface {
	// GetIndicatorValues 获取所有指标值（合并 Shared + Private + ControlState.Indicators）
	GetIndicatorValues() map[string]float64

	// GetMarketDataSnapshot 获取最新行情快照
	GetMarketDataSnapshot() map[string]*mdpb.MarketDataUpdate

	// GetOrdersSnapshot 获取订单快照
	GetOrdersSnapshot() map[string]*orspb.OrderUpdate

	// GetThresholds 获取阈值配置（用于前端显示）
	GetThresholds() map[string]float64
}

// StrategyDataContext 提供 Go 特有的策略数据字段
// 与 C++ ExecutionStrategy 分离，专门用于外部数据访问（WebSocket、API等）
// 实现 StrategyDataProvider 接口
type StrategyDataContext struct {
	ID                string
	Type              string
	Config            *StrategyConfig
	SharedIndicators  *indicators.IndicatorLibrary  // Shared indicators (read-only, updated by engine)
	PrivateIndicators *indicators.IndicatorLibrary  // Private indicators (strategy-specific)
	ControlState      *StrategyControlState         // State control (aligned with tbsrc)
	Status            *StrategyStatus
	PendingSignals    []*TradingSignal
	Orders            map[string]*orspb.OrderUpdate // order_id -> OrderUpdate (for WebSocket display)
	LastMarketData    map[string]*mdpb.MarketDataUpdate // symbol -> Last market data (for WebSocket push)
	MarketDataMu      sync.RWMutex                  // Protects LastMarketData map

	// Concrete strategy instance (for parameter updates)
	concreteStrategy interface{}
}

// NewStrategyDataContext creates a new StrategyDataContext
func NewStrategyDataContext(id string, strategyType string) *StrategyDataContext {
	return &StrategyDataContext{
		ID:                id,
		Type:              strategyType,
		PrivateIndicators: indicators.NewIndicatorLibrary(),
		ControlState:      NewStrategyControlState(true),
		Status:            &StrategyStatus{StrategyID: id},
		PendingSignals:    make([]*TradingSignal, 0),
		Orders:            make(map[string]*orspb.OrderUpdate),
		LastMarketData:    make(map[string]*mdpb.MarketDataUpdate),
	}
}

// SetSharedIndicators sets the shared indicator library
func (ctx *StrategyDataContext) SetSharedIndicators(shared *indicators.IndicatorLibrary) {
	ctx.SharedIndicators = shared
}

// GetIndicator gets an indicator (tries shared first, then private)
func (ctx *StrategyDataContext) GetIndicator(name string) (indicators.Indicator, bool) {
	if ctx.SharedIndicators != nil {
		if ind, ok := ctx.SharedIndicators.Get(name); ok {
			return ind, true
		}
	}
	if ctx.PrivateIndicators != nil {
		if ind, ok := ctx.PrivateIndicators.Get(name); ok {
			return ind, true
		}
	}
	return nil, false
}

// GetID returns strategy ID
func (ctx *StrategyDataContext) GetID() string {
	return ctx.ID
}

// GetType returns strategy type
func (ctx *StrategyDataContext) GetType() string {
	return ctx.Type
}

// IsRunning returns true if strategy is running
func (ctx *StrategyDataContext) IsRunning() bool {
	return ctx.ControlState.RunState != StrategyRunStateStopped
}

// Activate activates the strategy
func (ctx *StrategyDataContext) Activate() {
	ctx.ControlState.Activate()
	log.Printf("[%s] Strategy activated", ctx.ID)
}

// Deactivate deactivates the strategy
func (ctx *StrategyDataContext) Deactivate() {
	ctx.ControlState.Deactivate()
	log.Printf("[%s] Strategy deactivated", ctx.ID)
}

// GetSignals returns pending signals and clears the queue
func (ctx *StrategyDataContext) GetSignals() []*TradingSignal {
	signals := ctx.PendingSignals
	ctx.PendingSignals = make([]*TradingSignal, 0)
	return signals
}

// AddSignal adds a new trading signal
func (ctx *StrategyDataContext) AddSignal(signal *TradingSignal) {
	ctx.PendingSignals = append(ctx.PendingSignals, signal)
	ctx.Status.SignalCount++
	ctx.Status.LastSignalTime = time.Now()
}

// SetConcreteStrategy sets the concrete strategy instance
func (ctx *StrategyDataContext) SetConcreteStrategy(strategy interface{}) {
	ctx.concreteStrategy = strategy
}

// === StrategyDataProvider 接口实现 ===

// GetIndicatorValues 获取所有指标值
func (ctx *StrategyDataContext) GetIndicatorValues() map[string]float64 {
	values := make(map[string]float64)

	if ctx.SharedIndicators != nil {
		for key, value := range ctx.SharedIndicators.GetAllValues() {
			values[key] = value
		}
	}
	if ctx.PrivateIndicators != nil {
		for key, value := range ctx.PrivateIndicators.GetAllValues() {
			values[key] = value
		}
	}
	if ctx.ControlState != nil && ctx.ControlState.Indicators != nil {
		for key, value := range ctx.ControlState.Indicators {
			values[key] = value
		}
	}
	return values
}

// GetMarketDataSnapshot 获取最新行情快照
func (ctx *StrategyDataContext) GetMarketDataSnapshot() map[string]*mdpb.MarketDataUpdate {
	ctx.MarketDataMu.RLock()
	defer ctx.MarketDataMu.RUnlock()

	snapshot := make(map[string]*mdpb.MarketDataUpdate, len(ctx.LastMarketData))
	for symbol, md := range ctx.LastMarketData {
		snapshot[symbol] = md
	}
	return snapshot
}

// GetOrdersSnapshot 获取订单快照
func (ctx *StrategyDataContext) GetOrdersSnapshot() map[string]*orspb.OrderUpdate {
	snapshot := make(map[string]*orspb.OrderUpdate, len(ctx.Orders))
	for orderID, order := range ctx.Orders {
		snapshot[orderID] = order
	}
	return snapshot
}

// GetThresholds 获取阈值配置
func (ctx *StrategyDataContext) GetThresholds() map[string]float64 {
	thresholds := make(map[string]float64)
	if ctx.Config == nil {
		return thresholds
	}

	params := ctx.Config.Parameters

	if entry, ok := params["entry_zscore"].(float64); ok {
		thresholds["entry_zscore"] = entry
	}
	if exit, ok := params["exit_zscore"].(float64); ok {
		thresholds["exit_zscore"] = exit
	}
	if minCorr, ok := params["min_correlation"].(float64); ok {
		thresholds["min_correlation"] = minCorr
	}
	if minSpread, ok := params["min_spread"].(float64); ok {
		thresholds["min_spread"] = minSpread
	}
	if spreadMult, ok := params["spread_multiplier"].(float64); ok {
		thresholds["spread_multiplier"] = spreadMult
	}
	if maxPos, ok := params["max_position_size"].(float64); ok {
		thresholds["max_position_size"] = maxPos
	}
	return thresholds
}

// SetLastMarketData stores the last market data for a symbol
func (ctx *StrategyDataContext) SetLastMarketData(symbol string, md *mdpb.MarketDataUpdate) {
	ctx.MarketDataMu.Lock()
	defer ctx.MarketDataMu.Unlock()
	ctx.LastMarketData[symbol] = md
}

// GetLastMarketData returns the last market data for a symbol
func (ctx *StrategyDataContext) GetLastMarketData(symbol string) *mdpb.MarketDataUpdate {
	ctx.MarketDataMu.RLock()
	defer ctx.MarketDataMu.RUnlock()
	return ctx.LastMarketData[symbol]
}

// CancelRequest 撤单请求结构
// C++: 对应 ExecutionStrategy 中的撤单请求
type CancelRequest struct {
	OrderID  string // 订单 ID
	Symbol   string // 合约代码
	Exchange string // 交易所
}

// Helper functions
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func absInt64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}

func absFloat(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
