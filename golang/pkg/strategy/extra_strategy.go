// Package strategy provides trading strategy implementations
package strategy

import (
	"log"
)

// ExtraStrategy 扩展执行策略
// C++: ExtraStrategy.h (tbsrc/Strategies/include/ExtraStrategy.h)
// ExtraStrategy 继承自 ExecutionStrategy，扩展支持多 Instrument 操作
//
// C++ 继承关系:
//   class ExtraStrategy : public ExecutionStrategy
//
// Go 通过嵌入实现"继承":
//   type ExtraStrategy struct {
//       *ExecutionStrategy  // 嵌入基类
//   }
type ExtraStrategy struct {
	*ExecutionStrategy // 嵌入 ExecutionStrategy 基类

	// ExtraStrategy 不需要额外字段，所有字段都在 ExecutionStrategy 中
	// C++ ExtraStrategy.h 中只定义了方法，没有额外字段
}

// NewExtraStrategy 创建新的 ExtraStrategy
// C++: ExtraStrategy::ExtraStrategy(CommonClient*, SimConfig*)
func NewExtraStrategy(strategyID int32, instru *Instrument) *ExtraStrategy {
	return &ExtraStrategy{
		ExecutionStrategy: NewExecutionStrategy(strategyID, instru),
	}
}

// ============================================================================
// ExtraStrategy 扩展方法
// 以下方法是 ExtraStrategy 特有的，支持多 Instrument 操作
// C++: ExtraStrategy.h:14-22
// ============================================================================

// SendBidOrder2 发送买单（支持指定 Instrument）
// C++: ExtraStrategy::SendBidOrder2(Instrument*, RequestType, int32_t level, double price, OrderHitType, int32_t qty, uint32_t ordID, double oldPx)
// 区分 ask/bid 的单笔报单量和最大仓位
func (es *ExtraStrategy) SendBidOrder2(instru *Instrument, price float64, qty int32, ordType OrderHitType, level int32) (*TradingSignal, bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

	// 检查是否已有该价格的订单
	if _, exists := es.BidMap[price]; exists {
		return nil, false
	}

	// 检查买单方向的持仓限制
	// C++: 使用 m_tholdBidMaxPos 和 m_tholdBidSize
	if es.Thold != nil {
		if es.Thold.BidMaxSize > 0 && es.NetPos+qty > es.Thold.BidMaxSize {
			return nil, false
		}
		if es.TholdBidMaxPos > 0 && es.NetPos+qty > es.TholdBidMaxPos {
			return nil, false
		}
	}

	// 创建订单统计
	orderStats := NewOrderStats()
	orderStats.Side = TransactionTypeBuy
	orderStats.Price = price
	orderStats.Qty = qty
	orderStats.OpenQty = qty
	orderStats.OrdType = ordType
	orderStats.Status = OrderStatusNewOrder
	orderStats.New = true

	// 确定信号类别
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

// SendAskOrder2 发送卖单（支持指定 Instrument）
// C++: ExtraStrategy::SendAskOrder2(Instrument*, RequestType, int32_t level, double price, OrderHitType, int32_t qty, uint32_t ordID, double oldPx)
// 区分 ask/bid 的单笔报单量和最大仓位
func (es *ExtraStrategy) SendAskOrder2(instru *Instrument, price float64, qty int32, ordType OrderHitType, level int32) (*TradingSignal, bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

	// 检查是否已有该价格的订单
	if _, exists := es.AskMap[price]; exists {
		return nil, false
	}

	// 检查卖单方向的持仓限制
	// C++: 使用 m_tholdAskMaxPos 和 m_tholdAskSize
	if es.Thold != nil {
		if es.Thold.AskMaxSize > 0 && es.NetPos-qty < -es.Thold.AskMaxSize {
			return nil, false
		}
		if es.TholdAskMaxPos > 0 && es.NetPos-qty < -es.TholdAskMaxPos {
			return nil, false
		}
	}

	// 创建订单统计
	orderStats := NewOrderStats()
	orderStats.Side = TransactionTypeSell
	orderStats.Price = price
	orderStats.Qty = qty
	orderStats.OpenQty = qty
	orderStats.OrdType = ordType
	orderStats.Status = OrderStatusNewOrder
	orderStats.New = true

	// 确定信号类别
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

// SendCancelOrderWithInstru 发送撤单请求（支持指定 Instrument）
// C++: ExtraStrategy::SendCancelOrder(Instrument*, uint32_t orderID)
func (es *ExtraStrategy) SendCancelOrderWithInstru(instru *Instrument, orderID uint32) bool {
	return es.ExecutionStrategy.SendCancelOrder(orderID)
}

// SendCancelOrderByPriceWithInstru 按价格撤单（支持指定 Instrument）
// C++: ExtraStrategy::SendCancelOrder(Instrument*, double price, TransactionType side)
func (es *ExtraStrategy) SendCancelOrderByPriceWithInstru(instru *Instrument, price float64, side TransactionType) bool {
	return es.ExecutionStrategy.SendCancelOrderByPrice(price, side)
}

// SendNewOrderWithInstru 发送新订单（支持指定 Instrument）
// C++: ExtraStrategy::SendNewOrder(TransactionType, double, int32_t, int32_t, Instrument*, TypeOfOrder, OrderHitType)
func (es *ExtraStrategy) SendNewOrderWithInstru(side TransactionType, price float64, qty int32, level int32, instru *Instrument, typeOfOrder TypeOfOrder, ordType OrderHitType) *OrderStats {
	return es.ExecutionStrategy.SendNewOrder(side, price, qty, level, typeOfOrder, ordType)
}

// SendModifyOrderWithInstru 发送改单请求（支持指定 Instrument）
// C++: ExtraStrategy::SendModifyOrder(Instrument*, uint32_t, double, double, int32_t, int32_t, TypeOfOrder, OrderHitType)
func (es *ExtraStrategy) SendModifyOrderWithInstru(instru *Instrument, orderID uint32, newPrice, oldPrice float64, qty, level int32, typeOfOrder TypeOfOrder, ordType OrderHitType) (*TradingSignal, bool) {
	return es.ExecutionStrategy.SendModifyOrder(orderID, newPrice, qty)
}

// HandleSquareoffWithInstru 处理平仓（支持指定 Instrument）
// C++: ExtraStrategy::HandleSquareoff(Instrument*)
func (es *ExtraStrategy) HandleSquareoffWithInstru(instru *Instrument) {
	es.ExecutionStrategy.HandleSquareoff()
}

// HandleSquareONWithInstru 恢复开仓能力（支持指定 Instrument）
// C++: ExtraStrategy::HandleSquareON(Instrument*)
func (es *ExtraStrategy) HandleSquareONWithInstru(instru *Instrument) {
	es.ExecutionStrategy.HandleSquareON()
}

// InitMonitorStratDatas 初始化监控策略数据
// C++: ExtraStrategy::InitMonitorStratDatas()
func (es *ExtraStrategy) InitMonitorStratDatas() {
	// C++: 初始化监控相关数据
	// 在 Go 版本中，使用日志替代监控上报
	log.Printf("[ExtraStrategy:%d] InitMonitorStratDatas called", es.StrategyID)
}

// MDCallBack 行情回调
// C++: ExtraStrategy::MDCallBack(MarketUpdateNew*)
func (es *ExtraStrategy) MDCallBack(bidPrice, askPrice float64, tradePrice float64, tradeQty int32) {
	es.mu.Lock()
	defer es.mu.Unlock()

	// 更新价格
	es.LastBid = es.TheoBid
	es.LastAsk = es.TheoAsk
	es.TheoBid = bidPrice
	es.TheoAsk = askPrice
	es.LTP = tradePrice

	// 更新 PNL
	if es.NetPos > 0 {
		es.UnrealisedPNL = float64(es.NetPos) * (bidPrice - es.BuyAvgPrice)
	} else if es.NetPos < 0 {
		es.UnrealisedPNL = float64(-es.NetPos) * (es.SellAvgPrice - askPrice)
	} else {
		es.UnrealisedPNL = 0
	}
	es.NetPNL = es.RealisedPNL + es.UnrealisedPNL
}

// SendOrder 发送订单（纯虚函数实现）
// C++: ExtraStrategy::SendOrder()
func (es *ExtraStrategy) SendOrder() {
	// C++: 纯虚函数，由派生类实现
	// 在 Go 版本中，订单发送通过 SendBidOrder2/SendAskOrder2 完成
}

// AddtoCache 添加到缓存
// C++: ExtraStrategy::AddtoCache(OrderMapIter&, double&)
func (es *ExtraStrategy) AddtoCache(order *OrderStats, price float64) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if order == nil {
		return
	}

	if order.Side == TransactionTypeBuy {
		es.BidMapCache[price] = order
	} else {
		es.AskMapCache[price] = order
	}
}
