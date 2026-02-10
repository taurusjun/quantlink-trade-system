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
type Strategy interface {
	// GetID returns the unique strategy ID
	GetID() string

	// GetType returns the strategy type name
	GetType() string

	// GetBaseStrategy returns the underlying BaseStrategy
	// C++: 对应访问 ExecutionStrategy 基类成员
	GetBaseStrategy() *BaseStrategy

	// Initialize initializes the strategy with configuration
	Initialize(config *StrategyConfig) error

	// Start starts the strategy
	Start() error

	// Stop stops the strategy
	Stop() error

	// IsRunning returns true if strategy is running
	IsRunning() bool

	// OnMarketData is called when new market data arrives (continuous trading)
	OnMarketData(md *mdpb.MarketDataUpdate)

	// OnAuctionData is called when auction period market data arrives
	// This allows strategies to implement special logic for auction periods
	// (e.g., opening/closing auction, like tbsrc AuctionCallBack)
	OnAuctionData(md *mdpb.MarketDataUpdate)

	// OnOrderUpdate is called when order status changes
	OnOrderUpdate(update *orspb.OrderUpdate)

	// OnTimer is called periodically
	OnTimer(now time.Time)

	// GetSignals returns pending trading signals
	GetSignals() []*TradingSignal

	// GetEstimatedPosition returns current estimated position (NOT real CTP position!)
	GetEstimatedPosition() *EstimatedPosition

	// GetPosition returns current position (alias for GetEstimatedPosition for compatibility)
	GetPosition() *EstimatedPosition

	// GetPNL returns current P&L
	GetPNL() *PNL

	// GetRiskMetrics returns current risk metrics
	GetRiskMetrics() *RiskMetrics

	// GetStatus returns strategy status
	GetStatus() *StrategyStatus

	// Reset resets the strategy to initial state
	Reset()

	// UpdateParameters updates strategy parameters (for hot reload)
	UpdateParameters(params map[string]interface{}) error

	// GetCurrentParameters returns current strategy parameters
	GetCurrentParameters() map[string]interface{}
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
