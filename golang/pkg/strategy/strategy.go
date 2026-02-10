package strategy

import (
	"fmt"
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

// BaseStrategy provides common functionality for all strategies
type BaseStrategy struct {
	ID                 string
	Type               string
	Config             *StrategyConfig
	SharedIndicators   *indicators.IndicatorLibrary // Shared indicators (read-only, updated by engine)
	PrivateIndicators  *indicators.IndicatorLibrary // Private indicators (strategy-specific)
	EstimatedPosition  *EstimatedPosition           // Estimated position (NOT real CTP position!)
	PNL                *PNL
	RiskMetrics        *RiskMetrics
	Status             *StrategyStatus
	ControlState       *StrategyControlState         // State control (aligned with tbsrc)
	PendingSignals     []*TradingSignal
	Orders             map[string]*orspb.OrderUpdate              // order_id -> OrderUpdate
	LastMarketData     map[string]*mdpb.MarketDataUpdate          // symbol -> Last market data (for WebSocket push)
	MarketDataMu       sync.RWMutex                               // Protects LastMarketData map

	// Concrete strategy instance (for parameter updates)
	// This is set by concrete strategies in their constructors
	concreteStrategy interface{}
}

// NewBaseStrategy creates a new base strategy
func NewBaseStrategy(id string, strategyType string) *BaseStrategy {
	return &BaseStrategy{
		ID:                id,
		Type:              strategyType,
		PrivateIndicators: indicators.NewIndicatorLibrary(),
		EstimatedPosition: &EstimatedPosition{}, // Estimated position (NOT real CTP!)
		PNL:               &PNL{},
		RiskMetrics:       &RiskMetrics{},
		Status:            &StrategyStatus{StrategyID: id},
		ControlState:      NewStrategyControlState(true), // Auto-activate by default
		PendingSignals:    make([]*TradingSignal, 0),
		Orders:            make(map[string]*orspb.OrderUpdate),
	}
}

// SetSharedIndicators sets the shared indicator library for this strategy
// (Called by StrategyEngine during initialization)
func (bs *BaseStrategy) SetSharedIndicators(shared *indicators.IndicatorLibrary) {
	bs.SharedIndicators = shared
}

// GetIndicator gets an indicator (tries shared first, then private)
func (bs *BaseStrategy) GetIndicator(name string) (indicators.Indicator, bool) {
	// Try shared indicators first
	if bs.SharedIndicators != nil {
		if ind, ok := bs.SharedIndicators.Get(name); ok {
			return ind, true
		}
	}

	// Try private indicators
	if bs.PrivateIndicators != nil {
		if ind, ok := bs.PrivateIndicators.Get(name); ok {
			return ind, true
		}
	}

	return nil, false
}

// GetID returns strategy ID
func (bs *BaseStrategy) GetID() string {
	return bs.ID
}

// GetType returns strategy type
func (bs *BaseStrategy) GetType() string {
	return bs.Type
}

// IsRunning returns true if strategy is running
// 注意：running 表示"策略进程在运行"，不等于"已激活可交易"
// 对应 tbsrc：TradeBot 启动后就是 running=true，但 m_Active 可能是 false
func (bs *BaseStrategy) IsRunning() bool {
	return bs.ControlState.RunState != StrategyRunStateStopped
}

// GetEstimatedPosition returns current estimated position (NOT real CTP position!)
func (bs *BaseStrategy) GetEstimatedPosition() *EstimatedPosition {
	return bs.EstimatedPosition
}

// GetPosition returns current position (alias for GetEstimatedPosition)
func (bs *BaseStrategy) GetPosition() *EstimatedPosition {
	return bs.EstimatedPosition
}

// GetPNL returns current P&L
func (bs *BaseStrategy) GetPNL() *PNL {
	return bs.PNL
}

// GetRiskMetrics returns risk metrics
func (bs *BaseStrategy) GetRiskMetrics() *RiskMetrics {
	return bs.RiskMetrics
}

// GetStatus returns strategy status
func (bs *BaseStrategy) GetStatus() *StrategyStatus {
	bs.Status.IsRunning = bs.ControlState.IsActivated() && bs.ControlState.RunState != StrategyRunStateStopped
	bs.Status.EstimatedPosition = bs.EstimatedPosition // Estimated position (NOT real CTP!)
	bs.Status.PNL = bs.PNL
	bs.Status.RiskMetrics = bs.RiskMetrics

	// Debug log
	if bs.EstimatedPosition != nil {
		log.Printf("[BaseStrategy:%s] 📊 GetStatus: EstimatedPosition Long=%d, Short=%d, Net=%d (ptr=%p)",
			bs.ID, bs.EstimatedPosition.LongQty, bs.EstimatedPosition.ShortQty,
			bs.EstimatedPosition.NetQty, bs.EstimatedPosition)
	} else {
		log.Printf("[BaseStrategy:%s] ⚠️  GetStatus: EstimatedPosition is NIL!", bs.ID)
	}

	return bs.Status
}

// GetSignals returns pending signals and clears the queue
func (bs *BaseStrategy) GetSignals() []*TradingSignal {
	signals := bs.PendingSignals
	bs.PendingSignals = make([]*TradingSignal, 0)
	return signals
}

// AddSignal adds a new trading signal
func (bs *BaseStrategy) AddSignal(signal *TradingSignal) {
	bs.PendingSignals = append(bs.PendingSignals, signal)
	bs.Status.SignalCount++
	bs.Status.LastSignalTime = time.Now()
}

// UpdatePosition updates position based on order update
// 符合中国期货市场规则：净持仓模型，买入先平空再开多，卖出先平多再开空
// 参考 tbsrc ExecutionStrategy::TradeCallBack
func (bs *BaseStrategy) UpdatePosition(update *orspb.OrderUpdate) {
	log.Printf("[BaseStrategy:%s] 🔍 UpdatePosition called: OrderID=%s, Symbol=%s, Status=%v, Side=%v, FilledQty=%d",
		bs.ID, update.OrderId, update.Symbol, update.Status, update.Side, update.FilledQty)

	// Store order update
	bs.Orders[update.OrderId] = update

	// Update position only for filled orders
	if update.Status == orspb.OrderStatus_FILLED {
		bs.Status.FillCount++

		qty := update.FilledQty
		price := update.AvgPrice

		if update.Side == orspb.OrderSide_BUY {
			// 买入逻辑
			// 1. 更新累计买入
			bs.EstimatedPosition.BuyTotalQty += qty
			bs.EstimatedPosition.BuyTotalValue += float64(qty) * price

			// 2. 检查是否有空头持仓需要平仓
			if bs.EstimatedPosition.NetQty < 0 {
				// 当前是空头持仓，买入平空
				closedQty := qty
				if closedQty > bs.EstimatedPosition.SellQty {
					closedQty = bs.EstimatedPosition.SellQty
				}

				// 计算平空盈亏: (卖出均价 - 买入价) × 平仓数量
				realizedPnL := (bs.EstimatedPosition.SellAvgPrice - price) * float64(closedQty)
				bs.PNL.RealizedPnL += realizedPnL

				// 减少空头持仓
				bs.EstimatedPosition.SellQty -= closedQty
				bs.EstimatedPosition.NetQty += closedQty

				log.Printf("[BaseStrategy:%s] 💰 平空盈亏: %.2f (平仓 %d @ %.2f, 空头均价 %.2f)",
					bs.ID, realizedPnL, closedQty, price, bs.EstimatedPosition.SellAvgPrice)

				// 剩余数量用于开多
				qty -= closedQty

				// 如果全部平仓，重置空头相关数据
				if bs.EstimatedPosition.SellQty == 0 {
					bs.EstimatedPosition.SellAvgPrice = 0
				}
			}

			// 3. 如果还有剩余数量，开多仓
			if qty > 0 {
				// 更新多头持仓和平均价
				totalCost := bs.EstimatedPosition.BuyAvgPrice * float64(bs.EstimatedPosition.BuyQty)
				totalCost += price * float64(qty)
				bs.EstimatedPosition.BuyQty += qty
				bs.EstimatedPosition.NetQty += qty
				if bs.EstimatedPosition.BuyQty > 0 {
					bs.EstimatedPosition.BuyAvgPrice = totalCost / float64(bs.EstimatedPosition.BuyQty)
				}

				log.Printf("[BaseStrategy:%s] 📈 开多: %d @ %.2f, 多头均价 %.2f",
					bs.ID, qty, price, bs.EstimatedPosition.BuyAvgPrice)
			}

		} else {
			// 卖出逻辑
			// 1. 更新累计卖出
			bs.EstimatedPosition.SellTotalQty += qty
			bs.EstimatedPosition.SellTotalValue += float64(qty) * price

			// 2. 检查是否有多头持仓需要平仓
			if bs.EstimatedPosition.NetQty > 0 {
				// 当前是多头持仓，卖出平多
				closedQty := qty
				if closedQty > bs.EstimatedPosition.BuyQty {
					closedQty = bs.EstimatedPosition.BuyQty
				}

				// 计算平多盈亏: (卖出价 - 买入均价) × 平仓数量
				realizedPnL := (price - bs.EstimatedPosition.BuyAvgPrice) * float64(closedQty)
				bs.PNL.RealizedPnL += realizedPnL

				// 减少多头持仓
				bs.EstimatedPosition.BuyQty -= closedQty
				bs.EstimatedPosition.NetQty -= closedQty

				log.Printf("[BaseStrategy:%s] 💰 平多盈亏: %.2f (平仓 %d @ %.2f, 多头均价 %.2f)",
					bs.ID, realizedPnL, closedQty, price, bs.EstimatedPosition.BuyAvgPrice)

				// 剩余数量用于开空
				qty -= closedQty

				// 如果全部平仓，重置多头相关数据
				if bs.EstimatedPosition.BuyQty == 0 {
					bs.EstimatedPosition.BuyAvgPrice = 0
				}
			}

			// 3. 如果还有剩余数量，开空仓
			if qty > 0 {
				// 更新空头持仓和平均价
				totalCost := bs.EstimatedPosition.SellAvgPrice * float64(bs.EstimatedPosition.SellQty)
				totalCost += price * float64(qty)
				bs.EstimatedPosition.SellQty += qty
				bs.EstimatedPosition.NetQty -= qty
				if bs.EstimatedPosition.SellQty > 0 {
					bs.EstimatedPosition.SellAvgPrice = totalCost / float64(bs.EstimatedPosition.SellQty)
				}

				log.Printf("[BaseStrategy:%s] 📉 开空: %d @ %.2f, 空头均价 %.2f",
					bs.ID, qty, price, bs.EstimatedPosition.SellAvgPrice)
			}
		}

		// 4. 更新兼容字段（为了 API 兼容性）
		bs.EstimatedPosition.UpdateCompatibilityFields()
		bs.EstimatedPosition.LastUpdate = time.Now()

		// 5. 当净持仓归零时，计算总的已实现盈亏
		if bs.EstimatedPosition.NetQty == 0 {
			// tbsrc 逻辑: m_realisedPNL = m_sellTotalValue - m_buyTotalValue
			totalRealizedPnL := bs.EstimatedPosition.SellTotalValue - bs.EstimatedPosition.BuyTotalValue
			log.Printf("[BaseStrategy:%s] ✅ 持仓归零，总已实现盈亏: %.2f (买入总额 %.2f, 卖出总额 %.2f)",
				bs.ID, totalRealizedPnL, bs.EstimatedPosition.BuyTotalValue, bs.EstimatedPosition.SellTotalValue)
		}

		log.Printf("[BaseStrategy:%s] ✅ 持仓更新: NetQty=%d (Buy=%d, Sell=%d), BuyAvg=%.2f, SellAvg=%.2f, RealizedPnL=%.2f",
			bs.ID, bs.EstimatedPosition.NetQty, bs.EstimatedPosition.BuyQty, bs.EstimatedPosition.SellQty,
			bs.EstimatedPosition.BuyAvgPrice, bs.EstimatedPosition.SellAvgPrice, bs.PNL.RealizedPnL)

	} else if update.Status == orspb.OrderStatus_REJECTED {
		bs.Status.RejectCount++
	}

	// 清理已完成的订单（FILLED/CANCELED/REJECTED），避免在 UI 中一直显示
	// C++: 订单完成后从 ordMap 中移除
	if update.Status == orspb.OrderStatus_FILLED ||
		update.Status == orspb.OrderStatus_CANCELED ||
		update.Status == orspb.OrderStatus_REJECTED {
		delete(bs.Orders, update.OrderId)
		log.Printf("[BaseStrategy:%s] 🗑️ Removed completed order %s from Orders map (status=%v)",
			bs.ID, update.OrderId, update.Status)
	}
}

// UpdatePNL updates P&L based on current market price
// 符合中国期货规则和 tbsrc 逻辑
// 参考 tbsrc ExecutionStrategy::CalculatePNL
func (bs *BaseStrategy) UpdatePNL(bidPrice, askPrice float64) {
	var unrealizedPnL float64 = 0

	if bs.EstimatedPosition.NetQty > 0 {
		// 多头持仓：使用卖一价（对手价）计算浮动盈亏
		// tbsrc: m_unrealisedPNL = m_netpos * (m_instru->bidPx[0] - m_buyPrice) * multiplier
		unrealizedPnL = float64(bs.EstimatedPosition.NetQty) * (bidPrice - bs.EstimatedPosition.BuyAvgPrice)

		log.Printf("[BaseStrategy:%s] 📊 多头 P&L: %.2f (NetQty=%d, BidPrice=%.2f, AvgBuy=%.2f)",
			bs.ID, unrealizedPnL, bs.EstimatedPosition.NetQty, bidPrice, bs.EstimatedPosition.BuyAvgPrice)

	} else if bs.EstimatedPosition.NetQty < 0 {
		// 空头持仓：使用买一价（对手价）计算浮动盈亏
		// tbsrc: m_unrealisedPNL = -1 * m_netpos * (m_sellPrice - m_instru->askPx[0]) * multiplier
		unrealizedPnL = float64(-bs.EstimatedPosition.NetQty) * (bs.EstimatedPosition.SellAvgPrice - askPrice)

		log.Printf("[BaseStrategy:%s] 📊 空头 P&L: %.2f (NetQty=%d, AskPrice=%.2f, AvgSell=%.2f)",
			bs.ID, unrealizedPnL, bs.EstimatedPosition.NetQty, askPrice, bs.EstimatedPosition.SellAvgPrice)
	}

	bs.PNL.UnrealizedPnL = unrealizedPnL
	bs.PNL.TotalPnL = bs.PNL.RealizedPnL + bs.PNL.UnrealizedPnL
	bs.PNL.NetPnL = bs.PNL.TotalPnL - bs.PNL.TradingFees
	bs.PNL.Timestamp = time.Now()

	if bs.EstimatedPosition.NetQty != 0 {
		log.Printf("[BaseStrategy:%s] 💰 Total P&L: Realized=%.2f, Unrealized=%.2f, Total=%.2f",
			bs.ID, bs.PNL.RealizedPnL, bs.PNL.UnrealizedPnL, bs.PNL.TotalPnL)
	}
}

// UpdateRiskMetrics updates risk metrics
func (bs *BaseStrategy) UpdateRiskMetrics(currentPrice float64) {
	bs.RiskMetrics.PositionSize = abs(bs.EstimatedPosition.NetQty)
	bs.RiskMetrics.ExposureValue = float64(bs.RiskMetrics.PositionSize) * currentPrice
	bs.RiskMetrics.Timestamp = time.Now()

	// Update max drawdown
	if bs.PNL.TotalPnL < 0 && absFloat(bs.PNL.TotalPnL) > bs.RiskMetrics.MaxDrawdown {
		bs.RiskMetrics.MaxDrawdown = absFloat(bs.PNL.TotalPnL)
	}
}

// CheckRiskLimits checks if risk limits are exceeded
func (bs *BaseStrategy) CheckRiskLimits() bool {
	if bs.Config == nil {
		return true
	}

	// Check position size limit
	if bs.Config.MaxPositionSize > 0 {
		if absInt64(bs.EstimatedPosition.NetQty) > bs.Config.MaxPositionSize {
			return false
		}
	}

	// Check exposure limit
	if bs.Config.MaxExposure > 0 {
		if bs.RiskMetrics.ExposureValue > bs.Config.MaxExposure {
			return false
		}
	}

	// Check drawdown limit
	if maxDrawdown, ok := bs.Config.RiskLimits["max_drawdown"]; ok {
		if bs.RiskMetrics.MaxDrawdown > maxDrawdown {
			return false
		}
	}

	return true
}

// Reset resets the strategy
func (bs *BaseStrategy) Reset() {
	bs.EstimatedPosition = &EstimatedPosition{}
	bs.PNL = &PNL{}
	bs.RiskMetrics = &RiskMetrics{}
	bs.PendingSignals = make([]*TradingSignal, 0)
	bs.Orders = make(map[string]*orspb.OrderUpdate)
	bs.PrivateIndicators.ResetAll()
}

// OnAuctionData provides default implementation for auction period data
// Default behavior: Do nothing (strategies can override for auction-specific logic)
// This aligns with tbsrc AuctionCallBack concept
func (bs *BaseStrategy) OnAuctionData(md *mdpb.MarketDataUpdate) {
	// Default: no action during auction period
	// Strategies that need auction logic should override this method
}

// UpdateParameters updates strategy parameters (hot reload support in BaseStrategy)
// This is the unified entry point for all strategies
func (bs *BaseStrategy) UpdateParameters(params map[string]interface{}) error {
	// Check if concrete strategy supports parameter updates
	if updatable, ok := bs.concreteStrategy.(ParameterUpdatable); ok {
		return updatable.ApplyParameters(params)
	}

	return fmt.Errorf("strategy %s does not implement ParameterUpdatable interface", bs.Type)
}

// GetCurrentParameters returns current strategy parameters
func (bs *BaseStrategy) GetCurrentParameters() map[string]interface{} {
	// Check if concrete strategy supports parameter queries
	if updatable, ok := bs.concreteStrategy.(ParameterUpdatable); ok {
		return updatable.GetCurrentParameters()
	}

	// Return empty map if not supported
	return make(map[string]interface{})
}

// SetConcreteStrategy sets the concrete strategy instance
// This should be called by concrete strategies in their constructors
func (bs *BaseStrategy) SetConcreteStrategy(strategy interface{}) {
	bs.concreteStrategy = strategy
}

// === 撤单管理方法 (C++: ExecutionStrategy::SendCancelOrder, ProcessCancelReject) ===

// CancelRequest 撤单请求结构
// C++: 对应 ExecutionStrategy 中的撤单请求
type CancelRequest struct {
	OrderID  string // 订单 ID
	Symbol   string // 合约代码
	Exchange string // 交易所
}

// GetPendingCancelOrders 获取待撤销的订单列表
// C++: 遍历 m_ordMap 找到 m_cancel=true 的订单
func (bs *BaseStrategy) GetPendingCancelOrders() []*CancelRequest {
	requests := make([]*CancelRequest, 0)
	for orderID, order := range bs.Orders {
		// 查找状态为 ACCEPTED 或 SUBMITTED 且未完全成交的订单
		// 这些是可以撤销的活跃订单
		if order.Status == orspb.OrderStatus_ACCEPTED ||
			order.Status == orspb.OrderStatus_SUBMITTED ||
			order.Status == orspb.OrderStatus_PARTIALLY_FILLED {

			// 检查是否需要撤单（通过 ControlState 判断）
			if bs.ControlState != nil && (bs.ControlState.FlattenMode || bs.ControlState.ExitRequested) {
				requests = append(requests, &CancelRequest{
					OrderID:  orderID,
					Symbol:   order.Symbol,
					Exchange: order.Exchange.String(), // common.Exchange -> string
				})
			}
		}
	}
	return requests
}

// MarkCancelSent 标记撤单请求已发送
// C++: 设置 order->m_cancel = false（表示已发送，等待回报）
func (bs *BaseStrategy) MarkCancelSent(orderID string) {
	if order, exists := bs.Orders[orderID]; exists {
		log.Printf("[BaseStrategy:%s] Cancel request sent for orderID=%s, symbol=%s",
			bs.ID, orderID, order.Symbol)
	}
}

// ProcessCancelReject 处理撤单拒绝
// C++: ExecutionStrategy::ProcessCancelReject()
func (bs *BaseStrategy) ProcessCancelReject(orderID string) {
	if order, exists := bs.Orders[orderID]; exists {
		log.Printf("[BaseStrategy:%s] Cancel rejected for orderID=%s, symbol=%s, status=%v",
			bs.ID, orderID, order.Symbol, order.Status)
		bs.Status.RejectCount++
	}
}

// CancelAllActiveOrders 撤销所有活跃订单
// C++: 在平仓时调用，撤销所有未成交订单
func (bs *BaseStrategy) CancelAllActiveOrders() []*CancelRequest {
	requests := make([]*CancelRequest, 0)
	for orderID, order := range bs.Orders {
		if order.Status == orspb.OrderStatus_ACCEPTED ||
			order.Status == orspb.OrderStatus_SUBMITTED ||
			order.Status == orspb.OrderStatus_PARTIALLY_FILLED {
			requests = append(requests, &CancelRequest{
				OrderID:  orderID,
				Symbol:   order.Symbol,
				Exchange: order.Exchange.String(), // common.Exchange -> string
			})
		}
	}
	return requests
}

// ============================================================================
// StrategyDataProvider 接口实现
// 提供策略数据给外部系统（WebSocket、REST API等）
// ============================================================================

// GetIndicatorValues 获取所有指标值
// 合并 SharedIndicators + PrivateIndicators + ControlState.Indicators
func (bs *BaseStrategy) GetIndicatorValues() map[string]float64 {
	values := make(map[string]float64)

	// 1. SharedIndicators（共享指标池）
	if bs.SharedIndicators != nil {
		for key, value := range bs.SharedIndicators.GetAllValues() {
			values[key] = value
		}
	}

	// 2. PrivateIndicators（私有指标）
	if bs.PrivateIndicators != nil {
		for key, value := range bs.PrivateIndicators.GetAllValues() {
			values[key] = value
		}
	}

	// 3. ControlState.Indicators（策略计算的实时指标）
	if bs.ControlState != nil && bs.ControlState.Indicators != nil {
		for key, value := range bs.ControlState.Indicators {
			values[key] = value
		}
	}

	return values
}

// GetMarketDataSnapshot 获取最新行情快照
func (bs *BaseStrategy) GetMarketDataSnapshot() map[string]*mdpb.MarketDataUpdate {
	bs.MarketDataMu.RLock()
	defer bs.MarketDataMu.RUnlock()

	// 返回副本，避免并发问题
	snapshot := make(map[string]*mdpb.MarketDataUpdate, len(bs.LastMarketData))
	for symbol, md := range bs.LastMarketData {
		snapshot[symbol] = md
	}
	return snapshot
}

// GetOrdersSnapshot 获取订单快照
func (bs *BaseStrategy) GetOrdersSnapshot() map[string]*orspb.OrderUpdate {
	// 返回副本
	snapshot := make(map[string]*orspb.OrderUpdate, len(bs.Orders))
	for orderID, order := range bs.Orders {
		snapshot[orderID] = order
	}
	return snapshot
}

// GetThresholds 获取阈值配置（用于前端显示）
func (bs *BaseStrategy) GetThresholds() map[string]float64 {
	thresholds := make(map[string]float64)

	if bs.Config == nil {
		return thresholds
	}

	params := bs.Config.Parameters

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
