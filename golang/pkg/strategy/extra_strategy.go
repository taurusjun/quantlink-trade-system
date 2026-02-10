// Package strategy provides trading strategy implementations
package strategy

import (
	"fmt"
	"log"
	"sync"
	"time"

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
// C++: ExtraStrategy.h (tbsrc/Strategies/include/ExtraStrategy.h)
// This encapsulates all per-leg state including position, orders, and thresholds
type ExtraStrategy struct {
	mu sync.RWMutex

	// === 基本信息 ===
	StrategyID int32       // m_strategyID - 策略ID
	Instru     *Instrument // m_instru - 合约信息
	Thold      *ThresholdSet // m_thold - 阈值配置

	// === 持仓字段 (C++: ExtraStrategy.h:111-114) ===
	NetPos       int32 // m_netpos - 总净仓
	NetPosPass   int32 // m_netpos_pass - 被动成交净仓
	NetPosPassYtd int32 // m_netpos_pass_ytd - 昨仓
	NetPosAgg    int32 // m_netpos_agg - 主动成交净仓

	// === 订单统计 (C++: ExtraStrategy.h:123-137) ===
	BuyOpenOrders      int32 // m_buyOpenOrders - 买单未成交数
	SellOpenOrders     int32 // m_sellOpenOrders - 卖单未成交数
	ImproveCount       int32 // m_improveCount - 改价次数
	CrossCount         int32 // m_crossCount - 吃单次数
	TradeCount         int32 // m_tradeCount - 成交次数
	RejectCount        int32 // m_rejectCount - 拒绝次数
	OrderCount         int32 // m_orderCount - 订单总数
	CancelCount        int32 // m_cancelCount - 撤单次数
	ConfirmCount       int32 // m_confirmCount - 确认次数
	CancelConfirmCount int32 // m_cancelconfirmCount - 撤单确认次数
	PriceCount         int32 // m_priceCount - 价格变动次数
	DeltaCount         int32 // m_deltaCount - Delta 变动次数
	LossCount          int32 // m_lossCount - 亏损次数
	QtyCount           int32 // m_qtyCount - 数量变动次数

	// === 成交量统计 (C++: ExtraStrategy.h:139-153) ===
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
	BuyExchTx      float64 // m_buyExchTx - 买入交易所手续费
	SellExchTx     float64 // m_sellExchTx - 卖出交易所手续费
	BuyValue       float64 // m_buyValue - 买入价值（当前持仓）
	SellValue      float64 // m_sellValue - 卖出价值（当前持仓）

	// === PNL (C++: ExtraStrategy.h:160-165) ===
	RealisedPNL   float64 // m_realisedPNL - 已实现盈亏
	UnrealisedPNL float64 // m_unrealisedPNL - 未实现盈亏
	NetPNL        float64 // m_netPNL - 净盈亏
	GrossPNL      float64 // m_grossPNL - 毛盈亏
	MaxPNL        float64 // m_maxPNL - 最大盈亏
	Drawdown      float64 // m_drawdown - 回撤

	// === 追单控制 (C++: ExtraStrategy.h:289-294) ===
	BuyAggCount  float64         // buyAggCount - 买单追单计数
	SellAggCount float64         // sellAggCount - 卖单追单计数
	BuyAggOrder  float64         // buyAggOrder - 买单追单数
	SellAggOrder float64         // sellAggOrder - 卖单追单数
	LastAggTime  uint64          // last_agg_time - 最后追单时间
	LastAggSide  TransactionType // last_agg_side - 最后追单方向

	// === 订单映射 (C++: ExtraStrategy.h:257-264) ===
	OrdMap      map[uint32]*OrderStats  // m_ordMap: OrderID → OrderStats
	BidMap      map[float64]*OrderStats // m_bidMap: Price → OrderStats
	AskMap      map[float64]*OrderStats // m_askMap: Price → OrderStats
	SweepOrdMap map[uint32]*OrderStats  // m_sweepordMap - 扫单映射
	BidMapCache map[float64]*OrderStats // m_bidMapCache - 买单缓存
	AskMapCache map[float64]*OrderStats // m_askMapCache - 卖单缓存

	// === 阈值 (动态计算) ===
	TholdBidPlace  float64 // m_tholdBidPlace - 买单入场阈值
	TholdBidRemove float64 // m_tholdBidRemove - 买单移除阈值
	TholdAskPlace  float64 // m_tholdAskPlace - 卖单入场阈值
	TholdAskRemove float64 // m_tholdAskRemove - 卖单移除阈值

	// === 时间戳 ===
	LastTradeTime  uint64 // m_lastTradeTime - 最后成交时间
	LastOrderTime  uint64 // m_lastOrderTime - 最后下单时间
	LastHBTS       uint64 // m_lastHBTS - 最后心跳时间
	LastOrdTS      uint64 // m_lastOrdTS - 最后订单时间戳
	LastDetailTS   uint64 // m_lastDetailTS - 最后详情时间戳

	// === 撤单拒绝相关 (C++: ExtraStrategy.h) ===
	LastCancelRejectTime    uint64 // m_lastCancelRejectTime - 最后撤单拒绝时间
	LastCancelRejectOrderID uint32 // m_lastCancelRejectOrderID - 最后撤单拒绝订单ID

	// === 状态标志 (C++: ExtraStrategy.h:90-101) ===
	OnExit      bool // m_onExit - 正在退出
	OnCancel    bool // m_onCancel - 正在撤单
	OnFlat      bool // m_onFlat - 正在平仓
	Active      bool // m_Active - 策略活跃
	OnStopLoss  bool // m_onStopLoss - 触发止损
	AggFlat     bool // m_aggFlat - 主动平仓
	OnMaxPx     bool // m_onMaxPx - 达到最大价格
	OnNewsFlat  bool // m_onNewsFlat - 新闻触发平仓
	OnTimeSqOff bool // m_onTimeSqOff - 时间触发平仓
	SendMail    bool // m_sendMail - 是否发送邮件
	CallSquareOff bool // callSquareOff - 触发平仓调用

	// === 风控限制 (C++: ExtraStrategy.h:109-110, 115) ===
	RmsQty        int32  // m_rmsQty - RMS 数量限制
	MaxOrderCount uint64 // m_maxOrderCount - 最大订单数
	MaxPosSize    uint64 // m_maxPosSize - 最大持仓

	// === 时间控制 (C++: ExtraStrategy.h:116-122) ===
	EndTimeH        int32  // m_endTimeH - 结束时
	EndTimeM        int32  // m_endTimeM - 结束分
	EndTimeExch     int32  // m_endTimeExch - 交易所结束时间
	EndTime         int64  // m_endTime - 结束时间
	EndTimeEpoch    uint64 // m_endTimeEpoch - 结束时间戳
	EndTimeAgg      int64  // m_endTimeAgg - 主动平仓时间
	EndTimeAggEpoch uint64 // m_endTimeAggEpoch - 主动平仓时间戳

	// === 阈值控制 (C++: ExtraStrategy.h:191-199) ===
	TholdMaxPos    int32 // m_tholdMaxPos - 最大持仓阈值
	TholdBeginPos  int32 // m_tholdBeginPos - 开始持仓阈值
	TholdInc       int32 // m_tholdInc - 增量阈值
	TholdSize      int32 // m_tholdSize - 单笔数量阈值
	TholdBidSize   int32 // m_tholdBidSize - 买单数量阈值
	TholdBidMaxPos int32 // m_tholdBidMaxPos - 买单最大持仓
	TholdAskSize   int32 // m_tholdAskSize - 卖单数量阈值
	TholdAskMaxPos int32 // m_tholdAskMaxPos - 卖单最大持仓

	// === 价格跟踪 (C++: ExtraStrategy.h:166-174) ===
	Ltp          float64 // m_ltp - 最新成交价
	CurrAvgPrice float64 // m_currAvgPrice - 当前均价
	CurrPrice    float64 // m_currPrice - 当前价格
	TargetPrice  float64 // m_targetPrice - 目标价格
	TheoBid      float64 // m_theoBid - 理论买价
	TheoAsk      float64 // m_theoAsk - 理论卖价
	LastTheoBid  float64 // m_lastTheoBid - 上次理论买价
	LastTheoAsk  float64 // m_lastTheoAsk - 上次理论卖价
	LastBid      float64 // m_lastBid - 上次买价
	LastAsk      float64 // m_lastAsk - 上次卖价
	LastTradePx  float64 // m_lastTradePx - 上次成交价

	// === 额外时间戳 (C++: ExtraStrategy.h:85-108) ===
	LastPosTS          uint64 // m_lastPosTS - 最后持仓时间戳
	LastStsTS          uint64 // m_lastStsTS - 最后状态时间戳
	LastFlatTS         uint64 // m_lastFlatTS - 最后平仓时间戳
	LastPxTS           uint64 // m_lastPxTS - 最后价格时间戳
	LastDeltaTS        uint64 // m_lastDeltaTS - 最后Delta时间戳
	LastLossTS         uint64 // m_lastLossTS - 最后亏损时间戳
	LastQtyTS          uint64 // m_lastQtyTS - 最后数量时间戳
	ExchTS             uint64 // m_exchTS - 交易所时间戳
	LocalTS            uint64 // m_localTS - 本地时间戳
	LastSweepTradeTime uint64 // m_lastSweepTradeTime - 最后扫单成交时间

	// === 撤单状态 (C++: ExtraStrategy.h:177, 235-236) ===
	LastCancelReqRejectSet int32 // m_lastCancelReqRejectSet - 撤单拒绝设置
	PendingBidCancel       bool  // m_pendingBidCancel - 待撤买单
	PendingAskCancel       bool  // m_pendingAskCancel - 待撤卖单
	CheckCancelQuantity    bool  // m_checkCancelQuantity - 检查撤单数量

	// === 订单状态 (C++: ExtraStrategy.h:225-231) ===
	QuoteChanged       bool    // quoteChanged - 报价变化
	IsBidOrderCrossing bool    // isBidOrderCrossing - 买单穿价
	IsAskOrderCrossing bool    // isAskOrderCrossing - 卖单穿价
	BidOrderQty        float64 // bidOrderQty - 买单数量
	BidOrderPx         float64 // bidOrderPx - 买单价格
	AskOrderQty        float64 // askOrderQty - 卖单数量
	AskOrderPx         float64 // askOrderPx - 卖单价格

	// === 成交统计补充 (C++: ExtraStrategy.h:147-148, 153-154) ===
	BuyExchContractTx  float64 // m_buyExchContractTx - 买入合约交易所费用
	SellExchContractTx float64 // m_sellExchContractTx - 卖出合约交易所费用
	TransTotalValue    float64 // m_transTotalValue - 交易总价值
	TransValue         float64 // m_transValue - 交易价值

	// === 最后成交状态 (C++: ExtraStrategy.h:175-176) ===
	LastTradeSide bool // m_lastTradeSide - 最后成交方向
	LastTrade     bool // m_lastTrade - 是否有最后成交
}

// NewExtraStrategy creates a new ExtraStrategy with initialized maps
func NewExtraStrategy(strategyID int32, instru *Instrument) *ExtraStrategy {
	return &ExtraStrategy{
		StrategyID:  strategyID,
		Instru:      instru,
		Thold:       NewThresholdSet(),
		OrdMap:      make(map[uint32]*OrderStats),
		BidMap:      make(map[float64]*OrderStats),
		AskMap:      make(map[float64]*OrderStats),
		SweepOrdMap: make(map[uint32]*OrderStats),
		BidMapCache: make(map[float64]*OrderStats),
		AskMapCache: make(map[float64]*OrderStats),
		Active:      true,
	}
}

// === 订单映射管理方法 ===

// AddToOrderMap adds an order to the order maps
// C++: ExtraStrategy::AddToOrderMap()
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
// C++: ExtraStrategy::RemoveFromOrderMap()
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
// C++: ExtraStrategy::ProcessTrade()
func (es *ExtraStrategy) ProcessTrade(orderID uint32, filledQty int32, price float64, side TransactionType) {
	es.mu.Lock()
	defer es.mu.Unlock()

	// 获取订单类型用于区分被动单和主动单
	// C++: 根据 order->m_ordType 判断是 STANDARD（被动）还是 CROSS/MATCH（主动）
	var ordType OrderHitType = OrderHitTypeStandard // 默认为被动单
	if orderStats, exists := es.OrdMap[orderID]; exists {
		orderStats.UpdateOnFill(filledQty, price)
		es.TradeCount++
		ordType = orderStats.OrdType
	}

	// 保存原始 filledQty 用于后续 NetPosPass/NetPosAgg 更新
	originalFilledQty := filledQty

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
	// C++: if (order->m_ordType == CROSS || order->m_ordType == MATCH) -> NetPosAgg
	//      else -> NetPosPass
	if ordType == OrderHitTypeCross || ordType == OrderHitTypeMatch {
		// 主动单更新 NetPosAgg
		if side == TransactionTypeBuy {
			es.NetPosAgg += originalFilledQty
		} else {
			es.NetPosAgg -= originalFilledQty
		}
	} else {
		// 被动单更新 NetPosPass
		if side == TransactionTypeBuy {
			es.NetPosPass += originalFilledQty
		} else {
			es.NetPosPass -= originalFilledQty
		}
	}

	log.Printf("[ExtraStrategy:%d] Trade processed: side=%v, qty=%d@%.2f, netPos=%d (pass=%d, agg=%d), ordType=%v",
		es.StrategyID, side, originalFilledQty, price, es.NetPos, es.NetPosPass, es.NetPosAgg, ordType)
}

// ProcessNewConfirm processes a new order confirmation
// C++: ExtraStrategy::ProcessNewConfirm()
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
// C++: ExtraStrategy::ProcessCancelConfirm()
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
// C++: ExtraStrategy::ProcessNewReject()
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

// ProcessCancelReject processes a cancel order rejection
// C++: ExtraStrategy::ProcessCancelReject()
func (es *ExtraStrategy) ProcessCancelReject(orderID uint32) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if orderStats, exists := es.OrdMap[orderID]; exists {
		orderStats.Status = OrderStatusCancelReject
		orderStats.Cancel = false
		es.RejectCount++
		es.LastCancelRejectTime = uint64(time.Now().UnixNano())
		es.LastCancelRejectOrderID = orderID

		log.Printf("[ExtraStrategy:%d] Cancel rejected for orderID=%d", es.StrategyID, orderID)
	}
}

// === 阈值管理方法 ===

// SetThresholds updates dynamic thresholds based on position
// C++: ExtraStrategy::SetThresholds()
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
	orderStats.Cancel = true
	return true
}

// === 平仓处理方法 ===

// HandleSquareoff 处理平仓
// C++: ExtraStrategy::HandleSquareoff()
func (es *ExtraStrategy) HandleSquareoff() {
	es.mu.Lock()
	defer es.mu.Unlock()

	es.OnFlat = true
	// 撤销所有活跃订单
	for orderID, order := range es.OrdMap {
		if order.Active {
			order.Status = OrderStatusCancelOrder
			order.Cancel = true
			log.Printf("[ExtraStrategy:%d] HandleSquareoff: canceling order %d", es.StrategyID, orderID)
		}
	}
}

// HandleTimeLimitSquareoff 时间限制平仓
// C++: ExtraStrategy::HandleTimeLimitSquareoff()
func (es *ExtraStrategy) HandleTimeLimitSquareoff() {
	es.mu.Lock()
	es.OnTimeSqOff = true
	es.mu.Unlock()

	es.HandleSquareoff()
}

// CheckSquareoff 检查是否需要平仓
// C++: ExtraStrategy::CheckSquareoff()
func (es *ExtraStrategy) CheckSquareoff() bool {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.OnStopLoss {
		return true
	}
	if es.Thold != nil && es.NetPNL < -es.Thold.MaxLoss {
		return true
	}
	return false
}

// === 改单处理方法 ===

// SendModifyOrder 发送改单请求
// C++: ExtraStrategy::SendModifyOrder()
func (es *ExtraStrategy) SendModifyOrder(orderID uint32, newPrice float64, newQty int32) (*TradingSignal, bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

	orderStats, exists := es.OrdMap[orderID]
	if !exists || !orderStats.Active || orderStats.ModifyWait || orderStats.Cancel {
		return nil, false
	}

	orderStats.OldPrice = orderStats.Price
	orderStats.OldQty = orderStats.Qty
	orderStats.NewPrice = newPrice
	orderStats.NewQty = newQty
	orderStats.Status = OrderStatusModifyOrder
	orderStats.ModifyWait = true

	var side OrderSide
	if orderStats.Side == TransactionTypeBuy {
		side = OrderSideBuy
	} else {
		side = OrderSideSell
	}

	return &TradingSignal{
		Side:     side,
		Price:    newPrice,
		Quantity: int64(newQty),
		Metadata: map[string]interface{}{
			"order_id":   fmt.Sprintf("%d", orderID),
			"order_type": "modify",
		},
	}, true
}

// ProcessModifyConfirm 处理改单确认
// C++: ExtraStrategy::ProcessModifyConfirm()
func (es *ExtraStrategy) ProcessModifyConfirm(orderID uint32, newPrice float64, newQty int32) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if orderStats, exists := es.OrdMap[orderID]; exists {
		// 更新价格映射
		if orderStats.Side == TransactionTypeBuy {
			delete(es.BidMap, orderStats.Price)
			es.BidMap[newPrice] = orderStats
		} else {
			delete(es.AskMap, orderStats.Price)
			es.AskMap[newPrice] = orderStats
		}

		orderStats.OldPrice = orderStats.Price
		orderStats.Price = newPrice
		orderStats.Qty = newQty
		orderStats.OpenQty = newQty
		orderStats.Status = OrderStatusModifyConfirm
		orderStats.ModifyWait = false

		log.Printf("[ExtraStrategy:%d] Modify confirmed: orderID=%d, newPrice=%.2f, newQty=%d",
			es.StrategyID, orderID, newPrice, newQty)
	}
}

// ProcessModifyReject 处理改单拒绝
// C++: ExtraStrategy::ProcessModifyReject()
func (es *ExtraStrategy) ProcessModifyReject(orderID uint32) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if orderStats, exists := es.OrdMap[orderID]; exists {
		orderStats.Status = OrderStatusModifyReject
		orderStats.ModifyWait = false
		orderStats.NewPrice = orderStats.Price
		es.RejectCount++

		log.Printf("[ExtraStrategy:%d] Modify rejected: orderID=%d", es.StrategyID, orderID)
	}
}

// === 自成交处理 ===

// ProcessSelfTrade 处理自成交
// C++: ExtraStrategy::ProcessSelfTrade()
func (es *ExtraStrategy) ProcessSelfTrade(orderID uint32) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if orderStats, exists := es.OrdMap[orderID]; exists {
		orderStats.Active = false
		log.Printf("[ExtraStrategy:%d] Self-trade detected, orderID=%d", es.StrategyID, orderID)
	}
}

// === 阈值设置方法 ===

// SetLinearThresholds 设置线性阈值
// C++: ExtraStrategy::SetLinearThresholds()
func (es *ExtraStrategy) SetLinearThresholds() {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.Thold == nil || es.Thold.UseLinearThold == 0 {
		return
	}

	// 线性阈值计算：根据持仓线性调整入场/出场阈值
	// C++: 具体逻辑根据 USE_LINEAR_THOLD 模式而定
}

// === 扫单方法 ===

// SendSweepOrder 发送扫单
// C++: ExtraStrategy::SendSweepOrder()
func (es *ExtraStrategy) SendSweepOrder(price float64, qty int32, side TransactionType) (*TradingSignal, bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

	// 扫单：以激进价格快速成交
	var orderSide OrderSide
	if side == TransactionTypeBuy {
		orderSide = OrderSideBuy
	} else {
		orderSide = OrderSideSell
	}

	signal := &TradingSignal{
		Side:     orderSide,
		Price:    price,
		Quantity: int64(qty),
		Category: SignalCategoryAggressive, // 扫单是主动单
	}

	es.OrderCount++
	return signal, true
}

// === 价格获取方法 ===

// GetBidPrice 获取买单挂单价格
// C++: ExtraStrategy::GetBidPrice()
func (es *ExtraStrategy) GetBidPrice(level int32) (float64, OrderHitType, int32) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	// 基础实现：从 Instru 获取价格
	// 完整实现需要行情数据
	return 0, OrderHitTypeStandard, level
}

// GetAskPrice 获取卖单挂单价格
// C++: ExtraStrategy::GetAskPrice()
func (es *ExtraStrategy) GetAskPrice(level int32) (float64, OrderHitType, int32) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	// 基础实现：从 Instru 获取价格
	// 完整实现需要行情数据
	return 0, OrderHitTypeStandard, level
}

// === PNL 计算方法 ===

// CalculatePNL 计算盈亏
// C++: ExtraStrategy::CalculatePNL()
func (es *ExtraStrategy) CalculatePNL(bidPrice, askPrice float64) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.NetPos > 0 {
		// 多头：用卖价计算（平仓时卖出）
		es.UnrealisedPNL = float64(es.NetPos) * (bidPrice - es.BuyAvgPrice)
	} else if es.NetPos < 0 {
		// 空头：用买价计算（平仓时买入）
		es.UnrealisedPNL = float64(-es.NetPos) * (es.SellAvgPrice - askPrice)
	} else {
		es.UnrealisedPNL = 0
	}

	es.NetPNL = es.RealisedPNL + es.UnrealisedPNL

	// 更新最大盈亏和回撤
	if es.NetPNL > es.MaxPNL {
		es.MaxPNL = es.NetPNL
	}
	es.Drawdown = es.MaxPNL - es.NetPNL
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
		// 注意：撤单拒绝通过 CancelResponse.error_code 返回，不是 OrderUpdate
		// 如需处理撤单拒绝，应在订单管理层调用 ProcessCancelReject(orderID)
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
