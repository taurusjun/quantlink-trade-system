// Package strategy provides trading strategy implementations
package strategy

import (
	"fmt"
	"log"
	"sync"
	"time"

	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
	"github.com/yourusername/quantlink-trade-system/pkg/shm"
)

// Instrument represents a trading instrument
// C++: Instrument class from hftbase
type Instrument struct {
	Symbol     string  // 合约代码
	Exchange   string  // 交易所
	TickSize   float64 // 最小变动单位
	LotSize    int32   // 最小交易单位
	Multiplier float64 // 合约乘数
	// 行情数据字段
	BidPx  float64 // 最优买价
	AskPx  float64 // 最优卖价
	BidQty float64 // 买量
	AskQty float64 // 卖量
}

// MarketUpdateType 行情更新类型
// C++: MDUPDTYPE_*
type MarketUpdateType int32

const (
	MarketUpdateTypeTrade  MarketUpdateType = iota // MDUPDTYPE_TRADE - 成交
	MarketUpdateTypeAdd                            // MDUPDTYPE_ADD - 新增挂单
	MarketUpdateTypeDelete                         // MDUPDTYPE_DELETE - 删除挂单
	MarketUpdateTypeModify                         // MDUPDTYPE_MODIFY - 修改挂单
)

// ExecutionStrategy 执行策略基类
// C++: ExecutionStrategy.h (tbsrc/Strategies/include/ExecutionStrategy.h)
// 所有执行类策略的基类，包含持仓、订单、PNL、阈值等核心字段
//
// Go 设计：替代 BaseStrategy，包含所有必要字段
type ExecutionStrategy struct {
	mu sync.RWMutex

	// === 基本信息 (C++: ExecutionStrategy.h:248-256) ===
	StrategyID int32         // m_strategyID - 策略ID
	Instru     *Instrument   // m_instru - 主合约信息
	InstruSec  *Instrument   // m_instru_sec - 第二合约（可选）
	Thold      *ThresholdSet // m_thold - 阈值配置

	// === 状态标志 (C++: ExecutionStrategy.h:90-101) ===
	OnMaxPx       bool // m_onMaxPx - 达到最大价格
	OnNewsFlat    bool // m_onNewsFlat - 新闻触发平仓
	OnStopLoss    bool // m_onStopLoss - 触发止损
	OnExit        bool // m_onExit - 正在退出
	OnCancel      bool // m_onCancel - 正在撤单
	OnTimeSqOff   bool // m_onTimeSqOff - 时间触发平仓
	OnFlat        bool // m_onFlat - 正在平仓
	AggFlat       bool // m_aggFlat - 主动平仓
	SendMail      bool // m_sendMail - 是否发送邮件
	Active        bool // m_Active - 策略活跃
	CallSquareOff bool // callSquareOff - 触发平仓调用

	// === 时间戳 (C++: ExecutionStrategy.h:85-108) ===
	LastHBTS                uint64 // m_lastHBTS - 最后心跳时间戳
	LastOrdTS               uint64 // m_lastOrdTS - 最后订单时间戳
	LastDetailTS            uint64 // m_lastDetailTS - 最后详情时间戳
	LastPosTS               uint64 // m_lastPosTS - 最后持仓时间戳
	LastStsTS               uint64 // m_lastStsTS - 最后状态时间戳
	LastFlatTS              uint64 // m_lastFlatTS - 最后平仓时间戳
	LastPxTS                uint64 // m_lastPxTS - 最后价格时间戳
	LastDeltaTS             uint64 // m_lastDeltaTS - 最后Delta时间戳
	LastLossTS              uint64 // m_lastLossTS - 最后亏损时间戳
	LastQtyTS               uint64 // m_lastQtyTS - 最后数量时间戳
	ExchTS                  uint64 // m_exchTS - 交易所时间戳
	LocalTS                 uint64 // m_localTS - 本地时间戳
	LastTradeTime           uint64 // m_lastTradeTime - 最后成交时间
	LastOrderTime           uint64 // m_lastOrderTime - 最后下单时间
	LastSweepTradeTime      uint64 // m_lastSweepTradeTime - 最后扫单成交时间
	LastCancelRejectTime    uint64 // m_lastCancelRejectTime - 最后撤单拒绝时间
	LastCancelRejectOrderID uint32 // m_lastCancelRejectOrderID - 最后撤单拒绝订单ID

	// === 风控限制 (C++: ExecutionStrategy.h:109-110, 115) ===
	MaxOrderCount uint64 // m_maxOrderCount - 最大订单数
	MaxPosSize    uint64 // m_maxPosSize - 最大持仓
	RmsQty        int32  // m_rmsQty - RMS 数量限制

	// === 持仓字段 (C++: ExecutionStrategy.h:111-114) ===
	NetPos        int32 // m_netpos - 总净仓
	NetPosPass    int32 // m_netpos_pass - 被动成交净仓
	NetPosPassYtd int32 // m_netpos_pass_ytd - 昨仓
	NetPosAgg     int32 // m_netpos_agg - 主动成交净仓

	// === 时间控制 (C++: ExecutionStrategy.h:116-122) ===
	EndTimeH        int32  // m_endTimeH - 结束时
	EndTimeM        int32  // m_endTimeM - 结束分
	EndTimeExch     int32  // m_endTimeExch - 交易所结束时间
	EndTime         int64  // m_endTime - 结束时间
	EndTimeEpoch    uint64 // m_endTimeEpoch - 结束时间戳
	EndTimeAgg      int64  // m_endTimeAgg - 主动平仓时间
	EndTimeAggEpoch uint64 // m_endTimeAggEpoch - 主动平仓时间戳

	// === 订单统计 (C++: ExecutionStrategy.h:123-137) ===
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
	DeltaCount         int32 // m_deltaCount - Delta变动次数
	LossCount          int32 // m_lossCount - 亏损次数
	QtyCount           int32 // m_qtyCount - 数量变动次数

	// === 成交量统计 (C++: ExecutionStrategy.h:138-159) ===
	MaxTradedQty    float64 // m_maxTradedQty - 最大成交量
	BuyQty          float64 // m_buyQty - 买入数量（当前持仓）
	SellQty         float64 // m_sellQty - 卖出数量（当前持仓）
	BuyTotalQty     float64 // m_buyTotalQty - 买入总量
	SellTotalQty    float64 // m_sellTotalQty - 卖出总量
	BuyOpenQty      float64 // m_buyOpenQty - 买单未成交量
	SellOpenQty     float64 // m_sellOpenQty - 卖单未成交量
	BuyExchTx       float64 // m_buyExchTx - 买入交易所手续费
	SellExchTx      float64 // m_sellExchTx - 卖出交易所手续费
	BuyTotalValue   float64 // m_buyTotalValue - 买入总金额
	SellTotalValue  float64 // m_sellTotalValue - 卖出总金额
	BuyValue        float64 // m_buyValue - 买入价值（当前持仓）
	SellValue       float64 // m_sellValue - 卖出价值（当前持仓）
	TransTotalValue float64 // m_transTotalValue - 交易总价值
	TransValue      float64 // m_transValue - 交易价值
	BuyAvgPrice     float64 // m_buyAvgPrice - 买入均价
	SellAvgPrice    float64 // m_sellAvgPrice - 卖出均价
	BuyPrice        float64 // m_buyPrice - 买入价格
	SellPrice       float64 // m_sellPrice - 卖出价格
	AvgQty          float64 // m_avgQty - 平均数量

	// === PNL (C++: ExecutionStrategy.h:160-165) ===
	RealisedPNL   float64 // m_realisedPNL - 已实现盈亏
	UnrealisedPNL float64 // m_unrealisedPNL - 未实现盈亏
	NetPNL        float64 // m_netPNL - 净盈亏
	GrossPNL      float64 // m_grossPNL - 毛盈亏
	MaxPNL        float64 // m_maxPNL - 最大盈亏
	Drawdown      float64 // m_drawdown - 回撤

	// === 价格跟踪 (C++: ExecutionStrategy.h:166-174) ===
	LTP          float64 // m_ltp - 最新成交价
	CurrAvgPrice float64 // m_currAvgPrice - 当前均价
	CurrPrice    float64 // m_currPrice - 当前价格
	TargetPrice  float64 // m_targetPrice - 目标价格
	CurrAvgDelta float64 // m_currAvgDelta - 当前平均Delta
	CurrDelta    float64 // m_currDelta - 当前Delta
	CurrAvgLoss  float64 // m_currAvgLoss - 当前平均亏损
	CurrLoss     float64 // m_currLoss - 当前亏损
	IndValue     float64 // m_indvalue - 指标值

	// === 最后交易信息 (C++: ExecutionStrategy.h:175-177) ===
	LastTradeSide          bool // m_lastTradeSide - 最后交易方向
	LastTrade              bool // m_lastTrade - 最后交易标志
	LastCancelReqRejectSet int  // m_lastCancelReqRejectSet - 撤单拒绝设置

	// === 阈值字段 (C++: ExecutionStrategy.h:186-199) ===
	TholdBidPlace  float64 // m_tholdBidPlace - 买单入场阈值
	TholdBidRemove float64 // m_tholdBidRemove - 买单移除阈值
	TholdAskPlace  float64 // m_tholdAskPlace - 卖单入场阈值
	TholdAskRemove float64 // m_tholdAskRemove - 卖单移除阈值
	SmsRatio       int32   // m_smsRatio - SMS比率
	TholdMaxPos    int32   // m_tholdMaxPos - 最大持仓阈值
	TholdBeginPos  int32   // m_tholdBeginPos - 开始持仓阈值
	TholdInc       int32   // m_tholdInc - 增量阈值
	TholdSize      int32   // m_tholdSize - 单笔数量阈值
	TholdBidSize   int32   // m_tholdBidSize - 买单数量阈值
	TholdBidMaxPos int32   // m_tholdBidMaxPos - 买单最大持仓
	TholdAskSize   int32   // m_tholdAskSize - 卖单数量阈值
	TholdAskMaxPos int32   // m_tholdAskMaxPos - 卖单最大持仓

	// === Delta/Vega 调整 (C++: ExecutionStrategy.h:200-214) ===
	DeltaBias      float64 // m_deltaBias - Delta偏差
	VegaBias       float64 // m_vegaBias - Vega偏差
	DeltaAdj       float64 // m_deltaAdj - Delta调整
	VegaAdj        float64 // m_vegaAdj - Vega调整
	PosAdj         float64 // m_posAdj - 持仓调整
	PositionBias   float64 // m_positionBias - 持仓偏差
	ExcessPosition float64 // m_excessPosition - 超额持仓
	TotalBiasAdj   float64 // totalBiasAdj - 总偏差调整
	HedgeBid       float64 // hedgeBid - 对冲买价
	HedgeAsk       float64 // hedgeAsk - 对冲卖价
	IocBias        float64 // iocBias - IOC偏差
	IocPrice       float64 // iocPrice - IOC价格
	HedgeMid       float64 // hedgeMid - 对冲中间价
	HedgeScore     float64 // hedgeScore - 对冲得分
	IocScore       float64 // iocScore - IOC得分

	// === 理论价格 (C++: ExecutionStrategy.h:215-224) ===
	LastTheoBid       float64 // m_lastTheoBid - 上次理论买价
	LastTheoAsk       float64 // m_lastTheoAsk - 上次理论卖价
	LastBid           float64 // m_lastBid - 上次买价
	LastAsk           float64 // m_lastAsk - 上次卖价
	LastTradePx       float64 // m_lastTradePx - 上次成交价
	TmpAvgTargetPrice float64 // tmpAvgTargetPrice - 临时平均目标价
	TmpAvgDelta       float64 // tmpAvgDelta - 临时平均Delta
	TmpAvgLoss        float64 // tmpAvgLoss - 临时平均亏损
	TheoBid           float64 // m_theoBid - 理论买价
	TheoAsk           float64 // m_theoAsk - 理论卖价

	// === 订单穿越标志 (C++: ExecutionStrategy.h:225-231) ===
	QuoteChanged       bool    // quoteChanged - 报价改变
	IsBidOrderCrossing bool    // isBidOrderCrossing - 买单穿越
	IsAskOrderCrossing bool    // isAskOrderCrossing - 卖单穿越
	BidOrderQty        float64 // bidOrderQty - 买单数量
	BidOrderPx         float64 // bidOrderPx - 买单价格
	AskOrderQty        float64 // askOrderQty - 卖单数量
	AskOrderPx         float64 // askOrderPx - 卖单价格

	// === 对冲状态 (C++: ExecutionStrategy.h:232-236) ===
	IsHedging                bool    // isHedging - 正在对冲
	HedgingSide              bool    // hedgingSide - 对冲方向
	UnderlyingPredictedPrice float64 // m_underlyingPredictedPrice - 标的预测价格
	PendingBidCancel         bool    // m_pendingBidCancel - 待撤买单
	PendingAskCancel         bool    // m_pendingAskCancel - 待撤卖单

	// === 订单类型 (C++: ExecutionStrategy.h:241-242) ===
	OrdType OrderHitType // m_ordType - 订单类型
	Level   int32        // m_level - 价格层级

	// === 订单映射 (C++: ExecutionStrategy.h:257-264) ===
	OrdMap         map[uint32]*OrderStats  // m_ordMap: OrderID -> OrderStats
	SweepOrdMap    map[uint32]*OrderStats  // m_sweepordMap: 扫单映射
	BidMap         map[float64]*OrderStats // m_bidMap: Price -> OrderStats (买)
	AskMap         map[float64]*OrderStats // m_askMap: Price -> OrderStats (卖)
	BidMapCache    map[float64]*OrderStats // m_bidMapCache: 买单缓存
	AskMapCache    map[float64]*OrderStats // m_askMapCache: 卖单缓存
	BidMapCacheDel map[float64]*OrderStats // m_bidMapCacheDel: 买单删除缓存
	AskMapCacheDel map[float64]*OrderStats // m_askMapCacheDel: 卖单删除缓存

	// === 账户信息 (C++: ExecutionStrategy.h:265-267) ===
	Account     string // m_account - 账户
	InstruType  string // m_instruType - 合约类型
	Description string // m_description - 描述

	// === PNL缓存 (C++: ExecutionStrategy.h:269-270) ===
	BestAskLastPNL float64 // m_bestask_lastpnl
	BestBidLastPNL float64 // m_bestbid_lastpnl

	// === 统计队列相关 (C++: ExecutionStrategy.h:273-287) ===
	PrevTradeQty    float64 // prev_tradeQty - 上次成交量
	RunningMu       float64 // running_mu - 运行均值
	RunningVar      float64 // running_var - 运行方差
	Iter            int64   // iter - 迭代次数
	RunningMuSmall  float64 // running_mu_small - 小周期均值
	RunningVarSmall float64 // running_var_small - 小周期方差
	IterSmall       int64   // iter_small - 小周期迭代次数
	VolumeEwa       float64 // volume_ewa - 成交量EWA
	SetHigh         int     // SET_HIGH - 高位设置

	// === 追单控制 (C++: ExecutionStrategy.h:289-294) ===
	BuyAggCount  float64         // buyAggCount - 买单追单计数
	SellAggCount float64         // sellAggCount - 卖单追单计数
	BuyAggOrder  float64         // buyAggOrder - 买单追单数
	SellAggOrder float64         // sellAggOrder - 卖单追单数
	LastAggTime  uint64          // last_agg_time - 最后追单时间
	LastAggSide  TransactionType // last_agg_side - 最后追单方向

	// === 撤单数量检查 (C++: ExecutionStrategy.h:296) ===
	CheckCancelQuantity bool // m_checkCancelQuantity - 检查撤单数量

	// === 订单统计栈 (C++: ExecutionStrategy.h:299) ===
	OrderStatsStack []*OrderStats // m_orderStatsStack - 订单统计栈

	// === 合约统计 (C++: ExecutionStrategy.h:301-303) ===
	InstruRet         float64 // instruRet - 合约收益
	InstruAvgTradeQty float64 // instruAvgTradeQty - 合约平均成交量
	MachineName       string  // machineName - 机器名

	// === 产品标识 (C++: ExecutionStrategy.h:306) ===
	Product string // m_product - 产品标识

	// === 共享内存变量 (C++: ExecutionStrategy.h:308-311) ===
	// 用于外部程序（如 Python 模型）与策略间的数据共享
	TVar   *shm.TVar   // m_tvar - 用于读取外部 tValue 调整（价差均值）
	TCache *shm.TCache // m_tcache - 用于向外部共享持仓数据（SendTCacheLeg1Pos）
}

// NewExecutionStrategy 创建新的 ExecutionStrategy
// C++: ExecutionStrategy::ExecutionStrategy(CommonClient*, SimConfig*)
// 只包含 C++ 对应的字段，Go 特有字段在 StrategyDataContext 中
func NewExecutionStrategy(strategyID int32, instru *Instrument) *ExecutionStrategy {
	es := &ExecutionStrategy{
		StrategyID:      strategyID,
		Instru:          instru,
		Thold:           NewThresholdSet(),
		Active:          true,
		OrdMap:          make(map[uint32]*OrderStats),
		SweepOrdMap:     make(map[uint32]*OrderStats),
		BidMap:          make(map[float64]*OrderStats),
		AskMap:          make(map[float64]*OrderStats),
		BidMapCache:     make(map[float64]*OrderStats),
		AskMapCache:     make(map[float64]*OrderStats),
		BidMapCacheDel:  make(map[float64]*OrderStats),
		AskMapCacheDel:  make(map[float64]*OrderStats),
		OrderStatsStack: make([]*OrderStats, 0),
	}
	return es
}

// ============================================================================
// 订单映射管理方法
// ============================================================================

// AddToOrderMap 添加订单到映射
// C++: 通过 SendNewOrder 中的逻辑实现
func (es *ExecutionStrategy) AddToOrderMap(order *OrderStats) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if order == nil {
		return
	}

	es.OrdMap[order.OrderID] = order

	// 添加到价格映射
	if order.Side == TransactionTypeBuy {
		es.BidMap[order.Price] = order
		es.BuyOpenOrders++
	} else {
		es.AskMap[order.Price] = order
		es.SellOpenOrders++
	}
}

// RemoveFromOrderMap 从映射中移除订单
// C++: ExecutionStrategy::RemoveOrder(OrderMapIter&)
func (es *ExecutionStrategy) RemoveFromOrderMap(orderID uint32) *OrderStats {
	es.mu.Lock()
	defer es.mu.Unlock()

	order, exists := es.OrdMap[orderID]
	if !exists {
		return nil
	}

	// 从价格映射中移除
	if order.Side == TransactionTypeBuy {
		delete(es.BidMap, order.Price)
		es.BuyOpenOrders--
	} else {
		delete(es.AskMap, order.Price)
		es.SellOpenOrders--
	}

	delete(es.OrdMap, orderID)
	return order
}

// GetOrderByID 根据订单ID获取订单
func (es *ExecutionStrategy) GetOrderByID(orderID uint32) *OrderStats {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.OrdMap[orderID]
}

// GetOrderByPrice 根据价格和方向获取订单
func (es *ExecutionStrategy) GetOrderByPrice(price float64, side TransactionType) *OrderStats {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if side == TransactionTypeBuy {
		return es.BidMap[price]
	}
	return es.AskMap[price]
}

// HasOrderAtPrice 检查指定价格是否有订单
func (es *ExecutionStrategy) HasOrderAtPrice(price float64, side TransactionType) bool {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if side == TransactionTypeBuy {
		_, exists := es.BidMap[price]
		return exists
	}
	_, exists := es.AskMap[price]
	return exists
}

// GetAllOrders 获取所有订单
func (es *ExecutionStrategy) GetAllOrders() []*OrderStats {
	es.mu.RLock()
	defer es.mu.RUnlock()

	orders := make([]*OrderStats, 0, len(es.OrdMap))
	for _, order := range es.OrdMap {
		orders = append(orders, order)
	}
	return orders
}

// GetPendingOrders 获取所有待成交订单
func (es *ExecutionStrategy) GetPendingOrders() []*OrderStats {
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

// ============================================================================
// 持仓管理方法
// ============================================================================

// ProcessTrade 处理成交
// C++: ExecutionStrategy::ProcessTrade(ResponseMsg*, OrderMapIter)
func (es *ExecutionStrategy) ProcessTrade(orderID uint32, filledQty int32, price float64, side TransactionType) {
	es.mu.Lock()
	defer es.mu.Unlock()

	// 获取订单类型用于区分被动单和主动单
	var ordType OrderHitType = OrderHitTypeStandard
	if orderStats, exists := es.OrdMap[orderID]; exists {
		orderStats.UpdateOnFill(filledQty, price)
		es.TradeCount++
		ordType = orderStats.OrdType
	}

	originalFilledQty := filledQty

	// 更新持仓
	if side == TransactionTypeBuy {
		es.BuyTotalQty += float64(filledQty)
		es.BuyTotalValue += float64(filledQty) * price

		// 平空仓
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

		// 开多仓
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

		// 平多仓
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

		// 开空仓
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

	// 根据订单类型更新被动/主动持仓
	if ordType == OrderHitTypeCross || ordType == OrderHitTypeMatch {
		if side == TransactionTypeBuy {
			es.NetPosAgg += originalFilledQty
		} else {
			es.NetPosAgg -= originalFilledQty
		}
	} else {
		if side == TransactionTypeBuy {
			es.NetPosPass += originalFilledQty
		} else {
			es.NetPosPass -= originalFilledQty
		}
	}

	ordTypeStr := "STANDARD"
	if ordType == OrderHitTypeCross {
		ordTypeStr = "CROSS"
	}
	log.Printf("[ExecutionStrategy:%d] Trade processed: side=%v, qty=%d@%.2f, netPos=%d (pass=%d, agg=%d), ordType=%s",
		es.StrategyID, side, originalFilledQty, price, es.NetPos, es.NetPosPass, es.NetPosAgg, ordTypeStr)
}

// ProcessNewConfirm 处理新订单确认
// C++: ExecutionStrategy 中的订单确认处理
func (es *ExecutionStrategy) ProcessNewConfirm(orderID uint32) {
	es.mu.Lock()
	defer es.mu.Unlock()

	order, exists := es.OrdMap[orderID]
	if !exists {
		return
	}

	order.Status = OrderStatusNewConfirm
	order.Active = true
	order.New = false
	es.ConfirmCount++

	if order.Side == TransactionTypeBuy {
		es.BuyOpenOrders++
		es.BuyOpenQty += float64(order.OpenQty)
	} else {
		es.SellOpenOrders++
		es.SellOpenQty += float64(order.OpenQty)
	}

	log.Printf("[ExecutionStrategy:%d] New order confirmed: orderID=%d", es.StrategyID, orderID)
}

// ProcessCancelConfirm 处理撤单确认
// C++: ExecutionStrategy::ProcessCancelConfirm(ResponseMsg*, OrderMapIter)
func (es *ExecutionStrategy) ProcessCancelConfirm(orderID uint32) {
	es.mu.Lock()
	defer es.mu.Unlock()

	order, exists := es.OrdMap[orderID]
	if !exists {
		return
	}

	prevOpenQty := order.OpenQty
	order.UpdateOnCancel()
	es.CancelConfirmCount++

	if order.Side == TransactionTypeBuy {
		es.BuyOpenOrders--
		es.BuyOpenQty -= float64(prevOpenQty)
		delete(es.BidMap, order.Price)
	} else {
		es.SellOpenOrders--
		es.SellOpenQty -= float64(prevOpenQty)
		delete(es.AskMap, order.Price)
	}

	delete(es.OrdMap, orderID)

	log.Printf("[ExecutionStrategy:%d] Cancel confirmed: orderID=%d", es.StrategyID, orderID)
}

// ProcessModifyConfirm 处理改单确认
// C++: ExecutionStrategy::ProcessModifyConfirm(ResponseMsg*, OrderMapIter)
func (es *ExecutionStrategy) ProcessModifyConfirm(orderID uint32, newPrice float64, newQty int32) {
	es.mu.Lock()
	defer es.mu.Unlock()

	order, exists := es.OrdMap[orderID]
	if !exists {
		return
	}

	oldPrice := order.Price

	// 更新价格映射
	if order.Side == TransactionTypeBuy {
		delete(es.BidMap, oldPrice)
		es.BidMap[newPrice] = order
	} else {
		delete(es.AskMap, oldPrice)
		es.AskMap[newPrice] = order
	}

	order.OldPrice = oldPrice
	order.Price = newPrice
	order.Qty = newQty
	order.OpenQty = newQty
	order.Status = OrderStatusModifyConfirm
	order.ModifyWait = false

	log.Printf("[ExecutionStrategy:%d] Modify confirmed: orderID=%d, newPrice=%.2f, newQty=%d",
		es.StrategyID, orderID, newPrice, newQty)
}

// ProcessNewReject 处理新订单拒绝
// C++: ExecutionStrategy::ProcessNewReject(ResponseMsg*, OrderMapIter)
func (es *ExecutionStrategy) ProcessNewReject(orderID uint32) {
	es.mu.Lock()
	defer es.mu.Unlock()

	order, exists := es.OrdMap[orderID]
	if !exists {
		return
	}

	order.Status = OrderStatusNewReject
	order.Active = false
	es.RejectCount++

	// 从映射中移除
	delete(es.OrdMap, orderID)
	if order.Side == TransactionTypeBuy {
		delete(es.BidMap, order.Price)
	} else {
		delete(es.AskMap, order.Price)
	}

	log.Printf("[ExecutionStrategy:%d] New order rejected: orderID=%d", es.StrategyID, orderID)
}

// ProcessCancelReject 处理撤单拒绝
// C++: ExecutionStrategy::ProcessCancelReject(ResponseMsg*, OrderMapIter)
func (es *ExecutionStrategy) ProcessCancelReject(orderID uint32) {
	es.mu.Lock()
	defer es.mu.Unlock()

	order, exists := es.OrdMap[orderID]
	if !exists {
		return
	}

	order.Cancel = false
	order.Status = OrderStatusCancelReject
	es.RejectCount++
	es.LastCancelRejectOrderID = orderID
	es.LastCancelRejectTime = uint64(time.Now().UnixNano())

	log.Printf("[ExecutionStrategy:%d] Cancel rejected for orderID=%d", es.StrategyID, orderID)
}

// ProcessModifyReject 处理改单拒绝
// C++: ExecutionStrategy::ProcessModifyReject(ResponseMsg*, OrderMapIter)
func (es *ExecutionStrategy) ProcessModifyReject(orderID uint32) {
	es.mu.Lock()
	defer es.mu.Unlock()

	order, exists := es.OrdMap[orderID]
	if !exists {
		return
	}

	order.Status = OrderStatusModifyReject
	order.ModifyWait = false
	order.NewPrice = order.Price
	es.RejectCount++

	log.Printf("[ExecutionStrategy:%d] Modify rejected: orderID=%d", es.StrategyID, orderID)
}

// ProcessSelfTrade 处理自成交
// C++: ExecutionStrategy::ProcessSelfTrade(ResponseMsg*)
func (es *ExecutionStrategy) ProcessSelfTrade(orderID uint32) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if order, exists := es.OrdMap[orderID]; exists {
		order.Active = false
		log.Printf("[ExecutionStrategy:%d] Self-trade detected, orderID=%d", es.StrategyID, orderID)
	}
}

// ============================================================================
// 平仓处理方法
// ============================================================================

// HandleSquareoff 处理平仓
// C++: ExecutionStrategy::HandleSquareoff()
func (es *ExecutionStrategy) HandleSquareoff() {
	es.mu.Lock()
	defer es.mu.Unlock()

	es.OnFlat = true

	// 标记所有活跃订单为待撤销
	for orderID, order := range es.OrdMap {
		if order.Active && !order.Cancel {
			order.Cancel = true
			order.Status = OrderStatusCancelOrder
			log.Printf("[ExecutionStrategy:%d] HandleSquareoff: canceling order %d", es.StrategyID, orderID)
		}
	}
}

// HandleSquareON 恢复开仓能力
// C++: ExecutionStrategy::HandleSquareON()
func (es *ExecutionStrategy) HandleSquareON() {
	es.mu.Lock()
	defer es.mu.Unlock()

	es.OnExit = false
	es.OnCancel = false
	es.OnFlat = false

	log.Printf("[ExecutionStrategy:%d] HandleSquareON: squareoff mode OFF", es.StrategyID)
}

// HandleTimeLimitSquareoff 时间限制平仓
// C++: ExecutionStrategy::HandleTimeLimitSquareoff()
func (es *ExecutionStrategy) HandleTimeLimitSquareoff() {
	es.mu.Lock()
	es.OnTimeSqOff = true
	es.mu.Unlock()

	es.HandleSquareoff()
}

// CheckSquareoff 检查是否需要平仓
// C++: virtual void CheckSquareoff(MarketUpdateNew*)
func (es *ExecutionStrategy) CheckSquareoff() {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.OnStopLoss {
		es.CallSquareOff = true
		return
	}
	if es.Thold != nil && es.NetPNL < -es.Thold.MaxLoss {
		es.OnStopLoss = true
		es.CallSquareOff = true
		return
	}
}

// NeedSquareoff 返回是否需要平仓（辅助方法）
func (es *ExecutionStrategy) NeedSquareoff() bool {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.CallSquareOff || es.OnStopLoss
}

// SendOrder 发送订单（纯虚函数，子类必须实现）
// C++: virtual void SendOrder() = 0
func (es *ExecutionStrategy) SendOrder() {
	// 基类默认实现为空，子类需要覆盖
}

// OnTradeUpdate 成交更新回调
// C++: virtual void OnTradeUpdate() {}
func (es *ExecutionStrategy) OnTradeUpdate() {
	// 基类默认实现为空，子类可覆盖
}

// ============================================================================
// 队列位置计算方法
// ============================================================================

// SetQuantAhead 设置队列位置
// C++: ExecutionStrategy::SetQuantAhead(MarketUpdateNew*)
func (es *ExecutionStrategy) SetQuantAhead(updateType MarketUpdateType, price float64, newQuant, oldQuant int32, side TransactionType) {
	es.mu.Lock()
	defer es.mu.Unlock()

	// 根据方向找到对应的订单
	var orderStats *OrderStats
	if side == TransactionTypeBuy {
		orderStats = es.BidMap[price]
	} else {
		orderStats = es.AskMap[price]
	}

	if orderStats == nil {
		return
	}

	switch updateType {
	case MarketUpdateTypeTrade:
		// C++: ordStats->m_quantAhead -= update->m_newQuant;
		orderStats.QuantAhead -= float64(newQuant)
		if orderStats.QuantAhead < 0 {
			orderStats.QuantAhead = 0
		}

	case MarketUpdateTypeDelete, MarketUpdateTypeModify:
		var diffQty int32
		if updateType == MarketUpdateTypeDelete {
			diffQty = newQuant
		} else {
			diffQty = oldQuant - newQuant
		}

		if diffQty > 0 {
			ahead := orderStats.QuantAhead
			behind := orderStats.QuantBehind
			total := ahead + behind

			if total > 0 {
				if float64(diffQty) <= ahead && float64(diffQty) > behind {
					orderStats.QuantAhead -= float64(diffQty)
				} else if float64(diffQty) > ahead && float64(diffQty) <= behind {
					orderStats.QuantBehind -= float64(diffQty)
				} else {
					behindQty := (behind / total) * float64(diffQty)
					orderStats.QuantBehind -= behindQty
					orderStats.QuantAhead -= float64(diffQty) - behindQty
				}
			}

			if orderStats.QuantAhead < 0 {
				orderStats.QuantAhead = 0
			}
			if orderStats.QuantBehind < 0 {
				orderStats.QuantBehind = 0
			}
		} else if diffQty < 0 {
			orderStats.QuantBehind += float64(-diffQty)
		}

	case MarketUpdateTypeAdd:
		orderStats.QuantBehind += float64(newQuant)
	}
}

// InitQuantAhead 初始化订单的队列位置
func (es *ExecutionStrategy) InitQuantAhead(orderID uint32, totalQtyAtPrice int32, estimatedPosition float64) {
	es.mu.Lock()
	defer es.mu.Unlock()

	orderStats, exists := es.OrdMap[orderID]
	if !exists {
		return
	}

	totalQty := float64(totalQtyAtPrice) - float64(orderStats.OpenQty)
	if totalQty < 0 {
		totalQty = 0
	}

	orderStats.QuantAhead = totalQty * estimatedPosition
	orderStats.QuantBehind = totalQty * (1 - estimatedPosition)
}

// GetQuantAhead 获取订单前方排队量
func (es *ExecutionStrategy) GetQuantAhead(orderID uint32) float64 {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if orderStats, exists := es.OrdMap[orderID]; exists {
		return orderStats.QuantAhead
	}
	return 0
}

// GetQuantBehind 获取订单后方排队量
func (es *ExecutionStrategy) GetQuantBehind(orderID uint32) float64 {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if orderStats, exists := es.OrdMap[orderID]; exists {
		return orderStats.QuantBehind
	}
	return 0
}

// ============================================================================
// 阈值管理方法
// ============================================================================

// SetThresholds 设置动态阈值
// C++: ExecutionStrategy::SetThresholds()
func (es *ExecutionStrategy) SetThresholds() {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.Thold == nil {
		return
	}

	es.TholdBidPlace, es.TholdAskPlace = es.Thold.CalculateDynamicThreshold(
		es.NetPos,
		es.Thold.MaxSize,
	)

	es.TholdBidRemove = es.Thold.BeginRemove
	es.TholdAskRemove = es.Thold.BeginRemove
}

// SetLinearThresholds 设置线性阈值
// C++: ExecutionStrategy::SetLinearThresholds()
func (es *ExecutionStrategy) SetLinearThresholds() {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.Thold == nil || es.Thold.UseLinearThold == 0 {
		return
	}
	// 线性阈值计算逻辑
}

// ============================================================================
// 敞口计算方法
// ============================================================================

// CalcPendingNetposAgg 计算待成交主动单净仓
func (es *ExecutionStrategy) CalcPendingNetposAgg() int32 {
	es.mu.RLock()
	defer es.mu.RUnlock()

	var pending int32
	for _, order := range es.OrdMap {
		if order.OrdType == OrderHitTypeCross || order.OrdType == OrderHitTypeMatch {
			if order.OpenQty > 0 {
				if order.Side == TransactionTypeBuy {
					pending += order.OpenQty
				} else {
					pending -= order.OpenQty
				}
			}
		}
	}
	return pending
}

// GetTodayNetPos 返回今日净仓
func (es *ExecutionStrategy) GetTodayNetPos() int32 {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.NetPosPass - es.NetPosPassYtd
}

// ============================================================================
// PNL 计算方法
// ============================================================================

// CalculatePNL 计算盈亏
// C++: ExecutionStrategy::CalculatePNL()
func (es *ExecutionStrategy) CalculatePNL(bidPrice, askPrice float64) {
	es.mu.Lock()
	defer es.mu.Unlock()

	if es.NetPos > 0 {
		es.UnrealisedPNL = float64(es.NetPos) * (bidPrice - es.BuyAvgPrice)
	} else if es.NetPos < 0 {
		es.UnrealisedPNL = float64(-es.NetPos) * (es.SellAvgPrice - askPrice)
	} else {
		es.UnrealisedPNL = 0
	}

	es.NetPNL = es.RealisedPNL + es.UnrealisedPNL

	if es.NetPNL > es.MaxPNL {
		es.MaxPNL = es.NetPNL
	}
	es.Drawdown = es.MaxPNL - es.NetPNL
}

// ============================================================================
// 价格获取方法
// ============================================================================

// GetBidPrice 获取买入价格
// C++: ExecutionStrategy::GetBidPrice(double&, OrderHitType&, int32_t&)
func (es *ExecutionStrategy) GetBidPrice(level int32) (float64, OrderHitType, int32) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.Instru == nil {
		return 0, OrderHitTypeStandard, level
	}

	return es.Instru.BidPx, OrderHitTypeStandard, level
}

// GetAskPrice 获取卖出价格
// C++: ExecutionStrategy::GetAskPrice(double&, OrderHitType&, int32_t&)
func (es *ExecutionStrategy) GetAskPrice(level int32) (float64, OrderHitType, int32) {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.Instru == nil {
		return 0, OrderHitTypeStandard, level
	}

	return es.Instru.AskPx, OrderHitTypeStandard, level
}

// ============================================================================
// 订单发送方法
// ============================================================================

// SendNewOrder 发送新订单
// C++: ExecutionStrategy::SendNewOrder()
func (es *ExecutionStrategy) SendNewOrder(side TransactionType, price float64, qty int32, level int32, typeOfOrder TypeOfOrder, ordType OrderHitType) *OrderStats {
	es.mu.Lock()
	defer es.mu.Unlock()

	orderStats := NewOrderStats()
	orderStats.Side = side
	orderStats.Price = price
	orderStats.Qty = qty
	orderStats.OpenQty = qty
	orderStats.TypeOfOrder = typeOfOrder
	orderStats.OrdType = ordType
	orderStats.Status = OrderStatusNewOrder
	orderStats.New = true
	orderStats.Active = false // 等待确认后激活

	es.OrderCount++

	return orderStats
}

// SendCancelOrder 发送撤单请求
// C++: ExecutionStrategy::SendCancelOrder(uint32_t orderID)
func (es *ExecutionStrategy) SendCancelOrder(orderID uint32) bool {
	es.mu.Lock()
	defer es.mu.Unlock()

	orderStats, exists := es.OrdMap[orderID]
	if !exists || !orderStats.Active {
		return false
	}

	orderStats.Status = OrderStatusCancelOrder
	orderStats.Cancel = true
	es.CancelCount++

	return true
}

// SendCancelOrderByPrice 按价格撤单
// C++: ExecutionStrategy::SendCancelOrder(double price, TransactionType side)
func (es *ExecutionStrategy) SendCancelOrderByPrice(price float64, side TransactionType) bool {
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
	es.CancelCount++

	return true
}

// SendModifyOrder 发送改单请求
// C++: ExecutionStrategy::SendModifyOrder()
func (es *ExecutionStrategy) SendModifyOrder(orderID uint32, newPrice float64, newQty int32) (*TradingSignal, bool) {
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
		Metadata: map[string]any{
			"order_id":   fmt.Sprintf("%d", orderID),
			"order_type": "modify",
		},
	}, true
}

// SendSweepOrder 发送扫单
// C++: ExecutionStrategy::SendSweepOrder()
func (es *ExecutionStrategy) SendSweepOrder(price float64, qty int32, side TransactionType) (*TradingSignal, bool) {
	es.mu.Lock()
	defer es.mu.Unlock()

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
		Category: SignalCategoryAggressive,
	}

	es.OrderCount++
	return signal, true
}

// ============================================================================
// 订单回调处理
// ============================================================================

// HandleOrderUpdate 处理订单更新回调
// C++: ExecutionStrategy::ORSCallBack()
func (es *ExecutionStrategy) HandleOrderUpdate(update *orspb.OrderUpdate, orderID uint32) {
	switch update.Status {
	case orspb.OrderStatus_ACCEPTED:
		es.ProcessNewConfirm(orderID)

	case orspb.OrderStatus_PARTIALLY_FILLED:
		side := TransactionTypeBuy
		if update.Side == orspb.OrderSide_SELL {
			side = TransactionTypeSell
		}
		es.ProcessTrade(orderID, int32(update.FilledQty), update.AvgPrice, side)

	case orspb.OrderStatus_FILLED:
		side := TransactionTypeBuy
		if update.Side == orspb.OrderSide_SELL {
			side = TransactionTypeSell
		}
		es.ProcessTrade(orderID, int32(update.FilledQty), update.AvgPrice, side)
		es.RemoveFromOrderMap(orderID)

	case orspb.OrderStatus_CANCELED:
		es.ProcessCancelConfirm(orderID)

	case orspb.OrderStatus_REJECTED:
		es.ProcessNewReject(orderID)
	}

	es.SetThresholds()
}

// ============================================================================
// 统计和状态方法
// ============================================================================

// GetPositionStats 获取持仓统计
func (es *ExecutionStrategy) GetPositionStats() map[string]any {
	es.mu.RLock()
	defer es.mu.RUnlock()

	return map[string]any{
		"net_pos":          es.NetPos,
		"net_pos_pass":     es.NetPosPass,
		"net_pos_pass_ytd": es.NetPosPassYtd,
		"net_pos_agg":      es.NetPosAgg,
		"buy_qty":          es.BuyQty,
		"sell_qty":         es.SellQty,
		"buy_avg_price":    es.BuyAvgPrice,
		"sell_avg_price":   es.SellAvgPrice,
		"buy_open_orders":  es.BuyOpenOrders,
		"sell_open_orders": es.SellOpenOrders,
		"today_net_pos":    es.NetPosPass - es.NetPosPassYtd,
	}
}

// GetOrderStats 获取订单统计
func (es *ExecutionStrategy) GetOrderStats() map[string]any {
	es.mu.RLock()
	defer es.mu.RUnlock()

	return map[string]any{
		"order_count":    es.OrderCount,
		"trade_count":    es.TradeCount,
		"cancel_count":   es.CancelCount,
		"reject_count":   es.RejectCount,
		"confirm_count":  es.ConfirmCount,
		"improve_count":  es.ImproveCount,
		"cross_count":    es.CrossCount,
		"pending_orders": len(es.OrdMap),
		"buy_agg_order":  es.BuyAggOrder,
		"sell_agg_order": es.SellAggOrder,
	}
}

// Reset 重置策略状态
// C++: ExecutionStrategy::Reset()
func (es *ExecutionStrategy) Reset() {
	es.mu.Lock()
	defer es.mu.Unlock()

	// 保存昨仓
	es.NetPosPassYtd = es.NetPosPass

	// 清空日计数器
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

	// 清空订单映射
	es.OrdMap = make(map[uint32]*OrderStats)
	es.BidMap = make(map[float64]*OrderStats)
	es.AskMap = make(map[float64]*OrderStats)

	// 重置 PNL
	es.RealisedPNL = 0
	es.UnrealisedPNL = 0
	es.NetPNL = 0
	es.GrossPNL = 0
	es.MaxPNL = 0
	es.Drawdown = 0
}

// ============================================================================
// 共享内存方法
// ============================================================================

// InitSharedMemory 初始化共享内存变量
// C++: ExecutionStrategy.cpp:99-113
//
//	int tvarKey = simConfig->m_tholdSet.TVAR_KEY;
//	if (tvarKey > 0) {
//	    m_tvar = make_shared<hftlib::tvar<double>>();
//	    m_tvar->init(tvarKey, 0666);
//	}
//	int tcacheKey = simConfig->m_tholdSet.TCACHE_KEY;
//	if (tcacheKey > 0) {
//	    m_tcache = make_shared<hftlib::tcache<double>>();
//	    m_tcache->init(tcacheKey);
//	}
func (es *ExecutionStrategy) InitSharedMemory() error {
	if es.Thold == nil {
		return nil
	}

	// 初始化 TVar（用于读取外部 tValue）
	if es.Thold.TVarKey > 0 {
		tvar, err := shm.NewTVar(es.Thold.TVarKey)
		if err != nil {
			log.Printf("[ExecutionStrategy:%d] Warning: Failed to init TVar(key=%d): %v",
				es.StrategyID, es.Thold.TVarKey, err)
		} else {
			es.TVar = tvar
			log.Printf("[ExecutionStrategy:%d] TVar initialized with key=%d",
				es.StrategyID, es.Thold.TVarKey)
		}
	}

	// 初始化 TCache（用于向外部共享持仓）
	if es.Thold.TCacheKey > 0 {
		tcache, err := shm.NewTCache(es.Thold.TCacheKey, 100)
		if err != nil {
			log.Printf("[ExecutionStrategy:%d] Warning: Failed to init TCache(key=%d): %v",
				es.StrategyID, es.Thold.TCacheKey, err)
		} else {
			es.TCache = tcache
			log.Printf("[ExecutionStrategy:%d] TCache initialized with key=%d",
				es.StrategyID, es.Thold.TCacheKey)
		}
	}

	return nil
}

// LoadTValue 从共享内存读取 tValue
// C++: PairwiseArbStrategy.cpp:482-485
//
//	if (m_tvar) {
//	    tValue = m_tvar->load();
//	    TBLOG << "get tvar:" << fixed << tValue << endl;
//	}
func (es *ExecutionStrategy) LoadTValue() float64 {
	if es.TVar == nil {
		return 0
	}
	return es.TVar.Load()
}

// SendTCacheLeg1Pos 向共享内存写入 Leg1 持仓
// C++: PairwiseArbStrategy.cpp:SendTCacheLeg1Pos()
//
//	if (m_tcache) {
//	    m_tcache->store("leg1_pos", m_firstStrat->m_netpos_pass);
//	}
func (es *ExecutionStrategy) SendTCacheLeg1Pos(key string, value float64) error {
	if es.TCache == nil {
		return nil
	}
	return es.TCache.Store(key, value)
}

// CloseSharedMemory 关闭共享内存
func (es *ExecutionStrategy) CloseSharedMemory() {
	if es.TVar != nil {
		es.TVar.Close()
		es.TVar = nil
	}
	if es.TCache != nil {
		es.TCache.Close()
		es.TCache = nil
	}
}
