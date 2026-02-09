// Package strategy provides trading strategy implementations
package strategy

import (
	"log"
	"sync"

	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
)

// Instrument represents a trading instrument
// C++: Instrument class from hftbase
type Instrument struct {
	Symbol    string  // 合约代码
	Exchange  string  // 交易所
	TickSize  float64 // 最小变动单位
	LotSize   int32   // 最小交易单位
	Multiplier float64 // 合约乘数
}

// ExtraStrategy represents a single leg strategy for position and order management
// C++: ExtraStrategy.h + ExecutionStrategy.h
// This encapsulates all per-leg state including position, orders, and thresholds
type ExtraStrategy struct {
	mu sync.RWMutex

	// === 基本信息 ===
	StrategyID int32       // m_strategyID - 策略ID
	Instru     *Instrument // m_instru - 合约信息
	Thold      *ThresholdSet // m_thold - 阈值配置

	// === 持仓字段 (C++: ExecutionStrategy.h:111-114) ===
	NetPos       int32 // m_netpos - 总净仓
	NetPosPass   int32 // m_netpos_pass - 被动成交净仓
	NetPosPassYtd int32 // m_netpos_pass_ytd - 昨仓
	NetPosAgg    int32 // m_netpos_agg - 主动成交净仓

	// === 订单统计 (C++: ExecutionStrategy.h:123-137) ===
	BuyOpenOrders  int32 // m_buyOpenOrders - 买单未成交数
	SellOpenOrders int32 // m_sellOpenOrders - 卖单未成交数
	ImproveCount   int32 // m_improveCount - 改价次数
	CrossCount     int32 // m_crossCount - 吃单次数
	TradeCount     int32 // m_tradeCount - 成交次数
	RejectCount    int32 // m_rejectCount - 拒绝次数
	OrderCount     int32 // m_orderCount - 订单总数
	CancelCount    int32 // m_cancelCount - 撤单次数
	ConfirmCount   int32 // m_confirmCount - 确认次数

	// === 成交量统计 (C++: ExecutionStrategy.h:139-153) ===
	BuyQty         float64 // m_buyQty - 买入数量（当前持仓）
	SellQty        float64 // m_sellQty - 卖出数量（当前持仓）
	BuyTotalQty    float64 // m_buyTotalQty - 买入总量
	SellTotalQty   float64 // m_sellTotalQty - 卖出总量
	BuyOpenQty     float64 // m_buyOpenQty - 买单未成交量
	SellOpenQty    float64 // m_sellOpenQty - 卖单未成交量
	BuyTotalValue  float64 // m_buyTotalValue - 买入总金额
	SellTotalValue float64 // m_sellTotalValue - 卖出总金额
	BuyAvgPrice    float64 // m_buyAvgPrice - 买入均价
	SellAvgPrice   float64 // m_sellAvgPrice - 卖出均价

	// === PNL (C++: ExecutionStrategy.h:160-165) ===
	RealisedPNL   float64 // m_realisedPNL - 已实现盈亏
	UnrealisedPNL float64 // m_unrealisedPNL - 未实现盈亏
	NetPNL        float64 // m_netPNL - 净盈亏
	GrossPNL      float64 // m_grossPNL - 毛盈亏
	MaxPNL        float64 // m_maxPNL - 最大盈亏
	Drawdown      float64 // m_drawdown - 回撤

	// === 追单控制 (C++: ExecutionStrategy.h:289-294) ===
	BuyAggCount  float64         // buyAggCount - 买单追单计数
	SellAggCount float64         // sellAggCount - 卖单追单计数
	BuyAggOrder  float64         // buyAggOrder - 买单追单数
	SellAggOrder float64         // sellAggOrder - 卖单追单数
	LastAggTime  uint64          // last_agg_time - 最后追单时间
	LastAggSide  TransactionType // last_agg_side - 最后追单方向

	// === 订单映射 (C++: ExecutionStrategy.h:257-264) ===
	OrdMap map[uint32]*OrderStats  // m_ordMap: OrderID → OrderStats
	BidMap map[float64]*OrderStats // m_bidMap: Price → OrderStats
	AskMap map[float64]*OrderStats // m_askMap: Price → OrderStats

	// === 阈值 (动态计算) ===
	TholdBidPlace  float64 // m_tholdBidPlace - 买单入场阈值
	TholdBidRemove float64 // m_tholdBidRemove - 买单移除阈值
	TholdAskPlace  float64 // m_tholdAskPlace - 卖单入场阈值
	TholdAskRemove float64 // m_tholdAskRemove - 卖单移除阈值

	// === 时间戳 ===
	LastTradeTime uint64 // m_lastTradeTime - 最后成交时间
	LastOrderTime uint64 // m_lastOrderTime - 最后下单时间

	// === 状态标志 (C++: ExecutionStrategy.h:97-105) ===
	OnExit     bool // m_onExit - 正在退出
	OnCancel   bool // m_onCancel - 正在撤单
	OnFlat     bool // m_onFlat - 正在平仓
	Active     bool // m_Active - 策略活跃
	OnStopLoss bool // m_onStopLoss - 触发止损
	AggFlat    bool // m_aggFlat - 主动平仓
}

// NewExtraStrategy creates a new ExtraStrategy with initialized maps
func NewExtraStrategy(strategyID int32, instru *Instrument) *ExtraStrategy {
	return &ExtraStrategy{
		StrategyID: strategyID,
		Instru:     instru,
		Thold:      NewThresholdSet(),
		OrdMap:     make(map[uint32]*OrderStats),
		BidMap:     make(map[float64]*OrderStats),
		AskMap:     make(map[float64]*OrderStats),
		Active:     true,
	}
}

// === 订单映射管理方法 ===

// AddToOrderMap adds an order to the order maps
// C++: ExecutionStrategy::AddToOrderMap()
func (es *ExtraStrategy) AddToOrderMap(orderStats *OrderStats) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if orderStats == nil {
		return
	}

	// Add to OrdMap by OrderID
	es.OrdMap[orderStats.OrderID] = orderStats

	// Add to BidMap or AskMap by price
	if orderStats.Side == TransactionTypeBuy {
		es.BidMap[orderStats.Price] = orderStats
	} else {
		es.AskMap[orderStats.Price] = orderStats
	}
}

// RemoveFromOrderMap removes an order from the order maps
// C++: ExecutionStrategy::RemoveFromOrderMap()
func (es *ExtraStrategy) RemoveFromOrderMap(orderID uint32) *OrderStats {
	es.mu.Lock()
	defer es.mu.Unlock()

	orderStats, exists := es.OrdMap[orderID]
	if !exists {
		return nil
	}

	// Remove from OrdMap
	delete(es.OrdMap, orderID)

	// Remove from price map
	if orderStats.Side == TransactionTypeBuy {
		delete(es.BidMap, orderStats.Price)
	} else {
		delete(es.AskMap, orderStats.Price)
	}

	return orderStats
}

// GetOrderByID returns an order by its ID
func (es *ExtraStrategy) GetOrderByID(orderID uint32) *OrderStats {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.OrdMap[orderID]
}

// GetOrderByPrice returns an order by price and side
func (es *ExtraStrategy) GetOrderByPrice(price float64, side TransactionType) *OrderStats {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if side == TransactionTypeBuy {
		return es.BidMap[price]
	}
	return es.AskMap[price]
}

// HasOrderAtPrice checks if there's an order at the given price
func (es *ExtraStrategy) HasOrderAtPrice(price float64, side TransactionType) bool {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if side == TransactionTypeBuy {
		_, exists := es.BidMap[price]
		return exists
	}
	_, exists := es.AskMap[price]
	return exists
}

// GetAllOrders returns all orders in the order map
func (es *ExtraStrategy) GetAllOrders() []*OrderStats {
	es.mu.RLock()
	defer es.mu.RUnlock()

	orders := make([]*OrderStats, 0, len(es.OrdMap))
	for _, order := range es.OrdMap {
		orders = append(orders, order)
	}
	return orders
}

// GetPendingOrders returns all pending (active) orders
func (es *ExtraStrategy) GetPendingOrders() []*OrderStats {
	es.mu.RLock()
	defer es.mu.RUnlock()

	orders := make([]*OrderStats, 0)
	for _, order := range es.OrdMap {
		if order.Active && order.OpenQty > 0 {
			orders = append(orders, order)
		}
	}
	return orders
}

// === 持仓管理方法 ===

// ProcessTrade processes a trade fill and updates position
// C++: ExecutionStrategy::ProcessTrade()
func (es *ExtraStrategy) ProcessTrade(orderID uint32, filledQty int32, price float64, side TransactionType) {
	es.mu.Lock()
	defer es.mu.Unlock()

	// Update order stats
	if orderStats, exists := es.OrdMap[orderID]; exists {
		orderStats.UpdateOnFill(filledQty, price)
		es.TradeCount++
	}

	// Update position based on side
	if side == TransactionTypeBuy {
		es.BuyTotalQty += float64(filledQty)
		es.BuyTotalValue += float64(filledQty) * price

		// Check if closing short position
		if es.NetPos < 0 {
			closedQty := filledQty
			if closedQty > int32(es.SellQty) {
				closedQty = int32(es.SellQty)
			}
			es.SellQty -= float64(closedQty)
			es.NetPos += closedQty
			filledQty -= closedQty

			if es.SellQty == 0 {
				es.SellAvgPrice = 0
			}
		}

		// Open long
		if filledQty > 0 {
			totalCost := es.BuyAvgPrice * es.BuyQty
			totalCost += price * float64(filledQty)
			es.BuyQty += float64(filledQty)
			es.NetPos += filledQty
			if es.BuyQty > 0 {
				es.BuyAvgPrice = totalCost / es.BuyQty
			}
		}
	} else {
		es.SellTotalQty += float64(filledQty)
		es.SellTotalValue += float64(filledQty) * price

		// Check if closing long position
		if es.NetPos > 0 {
			closedQty := filledQty
			if closedQty > int32(es.BuyQty) {
				closedQty = int32(es.BuyQty)
			}
			es.BuyQty -= float64(closedQty)
			es.NetPos -= closedQty
			filledQty -= closedQty

			if es.BuyQty == 0 {
				es.BuyAvgPrice = 0
			}
		}

		// Open short
		if filledQty > 0 {
			totalCost := es.SellAvgPrice * es.SellQty
			totalCost += price * float64(filledQty)
			es.SellQty += float64(filledQty)
			es.NetPos -= filledQty
			if es.SellQty > 0 {
				es.SellAvgPrice = totalCost / es.SellQty
			}
		}
	}

	// Update NetPosPass or NetPosAgg based on order type
	// This is simplified - in full implementation, check order type
	es.NetPosPass = es.NetPos

	log.Printf("[ExtraStrategy:%d] Trade processed: side=%v, qty=%d@%.2f, netPos=%d",
		es.StrategyID, side, filledQty, price, es.NetPos)
}

// ProcessNewConfirm processes a new order confirmation
// C++: ExecutionStrategy::ProcessNewConfirm()
func (es *ExtraStrategy) ProcessNewConfirm(orderID uint32) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if orderStats, exists := es.OrdMap[orderID]; exists {
		orderStats.Status = OrderStatusNewConfirm
		orderStats.Active = true
		orderStats.New = false
		es.ConfirmCount++

		if orderStats.Side == TransactionTypeBuy {
			es.BuyOpenOrders++
			es.BuyOpenQty += float64(orderStats.OpenQty)
		} else {
			es.SellOpenOrders++
			es.SellOpenQty += float64(orderStats.OpenQty)
		}
	}
}

// ProcessCancelConfirm processes a cancel confirmation
// C++: ExecutionStrategy::ProcessCancelConfirm()
func (es *ExtraStrategy) ProcessCancelConfirm(orderID uint32) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if orderStats, exists := es.OrdMap[orderID]; exists {
		prevOpenQty := orderStats.OpenQty
		orderStats.UpdateOnCancel()
		es.CancelCount++

		if orderStats.Side == TransactionTypeBuy {
			es.BuyOpenOrders--
			es.BuyOpenQty -= float64(prevOpenQty)
		} else {
			es.SellOpenOrders--
			es.SellOpenQty -= float64(prevOpenQty)
		}

		// Remove from order maps
		delete(es.OrdMap, orderID)
		if orderStats.Side == TransactionTypeBuy {
			delete(es.BidMap, orderStats.Price)
		} else {
			delete(es.AskMap, orderStats.Price)
		}
	}
}

// ProcessNewReject processes a new order rejection
// C++: ExecutionStrategy::ProcessNewReject()
func (es *ExtraStrategy) ProcessNewReject(orderID uint32) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if orderStats, exists := es.OrdMap[orderID]; exists {
		orderStats.Status = OrderStatusNewReject
		orderStats.Active = false
		es.RejectCount++

		// Remove from order maps
		delete(es.OrdMap, orderID)
		if orderStats.Side == TransactionTypeBuy {
			delete(es.BidMap, orderStats.Price)
		} else {
			delete(es.AskMap, orderStats.Price)
		}
	}
}

// === 阈值管理方法 ===

// SetThresholds updates dynamic thresholds based on position
// C++: ExecutionStrategy::SetThresholds()
func (es *ExtraStrategy) SetThresholds() {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.Thold == nil {
		return
	}

	es.TholdBidPlace, es.TholdAskPlace = es.Thold.CalculateDynamicThreshold(
		es.NetPos,
		es.Thold.MaxSize,
	)

	// Calculate remove thresholds (similar logic)
	// Simplified - in full implementation, add remove threshold calculation
	es.TholdBidRemove = es.Thold.BeginRemove
	es.TholdAskRemove = es.Thold.BeginRemove
}

// === 敞口计算方法 ===

// CalcPendingNetposAgg calculates pending aggressive order net position
// C++: CalcPendingNetposAgg() in PairwiseArbStrategy.cpp:688-699
func (es *ExtraStrategy) CalcPendingNetposAgg() int32 {
	es.mu.RLock()
	defer es.mu.RUnlock()

	var netposPending int32

	for _, order := range es.OrdMap {
		// Only count CROSS and MATCH type orders
		if order.OrdType == OrderHitTypeCross || order.OrdType == OrderHitTypeMatch {
			if order.Side == TransactionTypeBuy {
				netposPending += order.OpenQty
			} else {
				netposPending -= order.OpenQty
			}
		}
	}

	return netposPending
}

// GetTodayNetPos returns today's net position
// C++: m_netpos_pass - m_netpos_pass_ytd
func (es *ExtraStrategy) GetTodayNetPos() int32 {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.NetPosPass - es.NetPosPassYtd
}

// === 订单发送方法 (信号生成) ===

// SendBidOrder2 generates a buy order signal
// C++: ExtraStrategy::SendBidOrder2()
// Returns true if order should be sent
func (es *ExtraStrategy) SendBidOrder2(price float64, qty int32, ordType OrderHitType, level int32) (*TradingSignal, bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

	// Check if already have order at this price
	if _, exists := es.BidMap[price]; exists {
		return nil, false
	}

	// Check position limits
	if es.Thold != nil && es.Thold.BidMaxSize > 0 {
		if es.NetPos+qty > es.Thold.BidMaxSize {
			return nil, false
		}
	}

	// Create order stats
	orderStats := NewOrderStats()
	orderStats.Side = TransactionTypeBuy
	orderStats.Price = price
	orderStats.Qty = qty
	orderStats.OpenQty = qty
	orderStats.OrdType = ordType
	orderStats.Status = OrderStatusNewOrder
	// OrderID will be assigned when order is confirmed

	// Determine signal category
	category := SignalCategoryPassive
	if ordType == OrderHitTypeCross || ordType == OrderHitTypeMatch {
		category = SignalCategoryAggressive
	}

	signal := &TradingSignal{
		Side:       OrderSideBuy,
		Price:      price,
		Quantity:   int64(qty),
		Category:   category,
		QuoteLevel: int(level),
	}

	es.OrderCount++
	return signal, true
}

// SendAskOrder2 generates a sell order signal
// C++: ExtraStrategy::SendAskOrder2()
// Returns true if order should be sent
func (es *ExtraStrategy) SendAskOrder2(price float64, qty int32, ordType OrderHitType, level int32) (*TradingSignal, bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

	// Check if already have order at this price
	if _, exists := es.AskMap[price]; exists {
		return nil, false
	}

	// Check position limits
	if es.Thold != nil && es.Thold.AskMaxSize > 0 {
		if es.NetPos-qty < -es.Thold.AskMaxSize {
			return nil, false
		}
	}

	// Create order stats
	orderStats := NewOrderStats()
	orderStats.Side = TransactionTypeSell
	orderStats.Price = price
	orderStats.Qty = qty
	orderStats.OpenQty = qty
	orderStats.OrdType = ordType
	orderStats.Status = OrderStatusNewOrder
	// OrderID will be assigned when order is confirmed

	// Determine signal category
	category := SignalCategoryPassive
	if ordType == OrderHitTypeCross || ordType == OrderHitTypeMatch {
		category = SignalCategoryAggressive
	}

	signal := &TradingSignal{
		Side:       OrderSideSell,
		Price:      price,
		Quantity:   int64(qty),
		Category:   category,
		QuoteLevel: int(level),
	}

	es.OrderCount++
	return signal, true
}

// SendCancelOrder generates a cancel order request
// C++: ExtraStrategy::SendCancelOrder()
func (es *ExtraStrategy) SendCancelOrder(orderID uint32) bool {
	es.mu.Lock()
	defer es.mu.Unlock()

	orderStats, exists := es.OrdMap[orderID]
	if !exists || !orderStats.Active {
		return false
	}

	orderStats.Status = OrderStatusCancelOrder
	return true
}

// SendCancelOrderByPrice generates a cancel for order at specific price
// C++: ExtraStrategy::SendCancelOrder(price, side)
func (es *ExtraStrategy) SendCancelOrderByPrice(price float64, side TransactionType) bool {
	es.mu.Lock()
	defer es.mu.Unlock()

	var orderStats *OrderStats
	if side == TransactionTypeBuy {
		orderStats = es.BidMap[price]
	} else {
		orderStats = es.AskMap[price]
	}

	if orderStats == nil || !orderStats.Active {
		return false
	}

	orderStats.Status = OrderStatusCancelOrder
	return true
}

// === 从 orspb.OrderUpdate 处理回调 ===

// HandleOrderUpdate processes an order update from ORS
// This is the main callback handler for order events
func (es *ExtraStrategy) HandleOrderUpdate(update *orspb.OrderUpdate, orderID uint32) {
	switch update.Status {
	case orspb.OrderStatus_ACCEPTED:
		es.ProcessNewConfirm(orderID)

	case orspb.OrderStatus_PARTIALLY_FILLED:
		// Partial fill - update order stats
		side := TransactionTypeBuy
		if update.Side == orspb.OrderSide_SELL {
			side = TransactionTypeSell
		}
		es.ProcessTrade(orderID, int32(update.FilledQty), update.AvgPrice, side)

	case orspb.OrderStatus_FILLED:
		// Full fill
		side := TransactionTypeBuy
		if update.Side == orspb.OrderSide_SELL {
			side = TransactionTypeSell
		}
		es.ProcessTrade(orderID, int32(update.FilledQty), update.AvgPrice, side)
		// Remove from maps after full fill
		es.RemoveFromOrderMap(orderID)

	case orspb.OrderStatus_CANCELED:
		es.ProcessCancelConfirm(orderID)

	case orspb.OrderStatus_REJECTED:
		es.ProcessNewReject(orderID)
	}

	// Update thresholds after any order event
	es.SetThresholds()
}

// === 统计和状态方法 ===

// GetPositionStats returns current position statistics
func (es *ExtraStrategy) GetPositionStats() map[string]interface{} {
	es.mu.RLock()
	defer es.mu.RUnlock()

	return map[string]interface{}{
		"net_pos":         es.NetPos,
		"net_pos_pass":    es.NetPosPass,
		"net_pos_pass_ytd": es.NetPosPassYtd,
		"net_pos_agg":     es.NetPosAgg,
		"buy_qty":         es.BuyQty,
		"sell_qty":        es.SellQty,
		"buy_avg_price":   es.BuyAvgPrice,
		"sell_avg_price":  es.SellAvgPrice,
		"buy_open_orders": es.BuyOpenOrders,
		"sell_open_orders": es.SellOpenOrders,
		"today_net_pos":   es.NetPosPass - es.NetPosPassYtd,
	}
}

// GetOrderStats returns order statistics
func (es *ExtraStrategy) GetOrderStats() map[string]interface{} {
	es.mu.RLock()
	defer es.mu.RUnlock()

	return map[string]interface{}{
		"order_count":   es.OrderCount,
		"trade_count":   es.TradeCount,
		"cancel_count":  es.CancelCount,
		"reject_count":  es.RejectCount,
		"confirm_count": es.ConfirmCount,
		"improve_count": es.ImproveCount,
		"cross_count":   es.CrossCount,
		"pending_orders": len(es.OrdMap),
		"buy_agg_order": es.BuyAggOrder,
		"sell_agg_order": es.SellAggOrder,
	}
}

// Reset resets all counters and state (for new trading day)
func (es *ExtraStrategy) Reset() {
	es.mu.Lock()
	defer es.mu.Unlock()

	// Save yesterday position
	es.NetPosPassYtd = es.NetPosPass

	// Clear daily counters
	es.BuyTotalQty = 0
	es.SellTotalQty = 0
	es.BuyTotalValue = 0
	es.SellTotalValue = 0
	es.TradeCount = 0
	es.OrderCount = 0
	es.CancelCount = 0
	es.RejectCount = 0
	es.ConfirmCount = 0
	es.ImproveCount = 0
	es.CrossCount = 0
	es.BuyAggOrder = 0
	es.SellAggOrder = 0
	es.BuyAggCount = 0
	es.SellAggCount = 0

	// Clear order maps
	es.OrdMap = make(map[uint32]*OrderStats)
	es.BidMap = make(map[float64]*OrderStats)
	es.AskMap = make(map[float64]*OrderStats)

	// Reset PNL
	es.RealisedPNL = 0
	es.UnrealisedPNL = 0
	es.NetPNL = 0
	es.GrossPNL = 0
	es.MaxPNL = 0
	es.Drawdown = 0
}
