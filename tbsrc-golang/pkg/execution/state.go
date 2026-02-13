package execution

// ExecutionState 持仓、PNL、计数器状态
// 对应 C++ ExecutionStrategy 类的成员变量
// 参考: tbsrc/Strategies/include/ExecutionStrategy.h:85-308
type ExecutionState struct {
	// 持仓 — C++: m_netpos, m_netpos_pass, m_netpos_pass_ytd, m_netpos_agg
	Netpos       int32 // 净持仓 = buyTotalQty - sellTotalQty
	NetposPass   int32 // 被动成交净持仓 (STANDARD)
	NetposPassYtd int32 // 昨日被动成交净持仓
	NetposAgg    int32 // 主动成交净持仓 (CROSS/MATCH)

	// 累计数量（session 级别，永不重置直到 Reset）
	BuyTotalQty  float64 // m_buyTotalQty
	SellTotalQty float64 // m_sellTotalQty
	BuyOpenQty   float64 // m_buyOpenQty — 未成交买单数量
	SellOpenQty  float64 // m_sellOpenQty — 未成交卖单数量

	// 当前腿数量（netpos 归零时重置）
	BuyQty  float64 // m_buyQty
	SellQty float64 // m_sellQty

	// 累计价值
	BuyTotalValue  float64 // m_buyTotalValue
	SellTotalValue float64 // m_sellTotalValue
	TransTotalValue float64 // m_transTotalValue — 累计手续费

	// 当前腿价值（netpos 归零时重置）
	BuyValue   float64 // m_buyValue
	SellValue  float64 // m_sellValue
	TransValue float64 // m_transValue — 当前腿手续费

	// 均价
	BuyAvgPrice  float64 // m_buyAvgPrice — session 级别
	SellAvgPrice float64 // m_sellAvgPrice — session 级别
	BuyPrice     float64 // m_buyPrice — 当前腿
	SellPrice    float64 // m_sellPrice — 当前腿

	// PNL
	RealisedPNL   float64 // m_realisedPNL
	UnrealisedPNL float64 // m_unrealisedPNL
	NetPNL        float64 // m_netPNL = grossPNL - transTotalValue
	GrossPNL      float64 // m_grossPNL = realisedPNL + unrealisedPNL
	MaxPNL        float64 // m_maxPNL — 高水位
	Drawdown      float64 // m_drawdown = netPNL - maxPNL

	// 订单计数器
	BuyOpenOrders  int32 // m_buyOpenOrders
	SellOpenOrders int32 // m_sellOpenOrders
	ImproveCount   int32 // m_improveCount
	CrossCount     int32 // m_crossCount
	TradeCount     int32 // m_tradeCount
	RejectCount    int32 // m_rejectCount
	OrderCount     int32 // m_orderCount
	CancelCount    int32 // m_cancelCount
	ConfirmCount   int32 // m_confirmCount
	CancelConfirmCnt int32 // m_cancelconfirmCount

	// 状态标志
	Active    bool // m_Active
	OnExit    bool // m_onExit
	OnCancel  bool // m_onCancel
	OnFlat    bool // m_onFlat
	AggFlat   bool // m_aggFlat
	OnStopLoss bool // m_onStopLoss
	OnMaxPx   bool // m_onMaxPx
	OnNewsFlat bool // m_onNewsFlat
	OnTimeSqOff bool // m_onTimeSqOff

	// 阈值派生字段（由 SetThresholds 计算）
	TholdBidPlace  float64 // m_tholdBidPlace
	TholdBidRemove float64 // m_tholdBidRemove
	TholdAskPlace  float64 // m_tholdAskPlace
	TholdAskRemove float64 // m_tholdAskRemove
	TholdMaxPos    int32   // m_tholdMaxPos
	TholdBeginPos  int32   // m_tholdBeginPos
	TholdInc       int32   // m_tholdInc
	TholdSize      int32   // m_tholdSize
	TholdBidSize   int32   // m_tholdBidSize
	TholdBidMaxPos int32   // m_tholdBidMaxPos
	TholdAskSize   int32   // m_tholdAskSize
	TholdAskMaxPos int32   // m_tholdAskMaxPos
	SMSRatio       int32   // m_smsRatio

	// 最新成交价
	LTP         float64 // m_ltp
	LastTradePx float64 // m_lastTradePx

	// 费率（从配置加载）
	BuyExchTx         float64 // m_buyExchTx — 买入交易所手续费率
	SellExchTx        float64 // m_sellExchTx — 卖出交易所手续费率
	BuyExchContractTx float64 // m_buyExchContractTx — 买入每手手续费
	SellExchContractTx float64 // m_sellExchContractTx — 卖出每手手续费

	// 时间戳
	ExchTS     uint64 // m_exchTS
	LocalTS    uint64 // m_localTS
	LastFlatTS uint64 // m_lastFlatTS

	// 上次 BBO（用于判断是否需要重算 PNL）
	BestBidLastPNL float64 // m_bestbid_lastpnl
	BestAskLastPNL float64 // m_bestask_lastpnl

	// 上次成交信息
	LastTradeSide bool   // m_lastTradeSide (true=buy, false=sell)
	LastTrade     bool   // m_lastTrade
	LastTradeTime uint64 // m_lastTradeTime

	// 平均数量（percent sizing 用）
	AvgQty   float64 // m_avgQty
	QtyCount int32   // m_qtyCount

	// SET_HIGH 标志
	SetHigh int // SET_HIGH

	// RMS 数量上限
	RmsQty int32 // m_rmsQty

	// 最大订单/仓位限制
	MaxOrderCount uint64 // m_maxOrderCount
	MaxTradedQty  float64 // m_maxTradedQty
	MaxPosSize    uint64 // m_maxPosSize

	// 结束时间
	EndTimeEpoch    uint64 // m_endTimeEpoch
	EndTimeAggEpoch uint64 // m_endTimeAggEpoch
	SqrOffTimeEpoch uint64 // 渐进式平仓时间 epoch（纳秒）

	// 止损时间戳（用于 auto-resume 冷却）
	StopLossTS uint64 // 触发 stop loss 时的纳秒时间戳
}

// Reset 将所有状态字段归零
// 参考: tbsrc/Strategies/ExecutionStrategy.cpp:276-396
func (s *ExecutionState) Reset() {
	// 计数器
	s.CancelConfirmCnt = 0
	s.ConfirmCount = 0
	s.ImproveCount = 0
	s.CrossCount = 0
	s.TradeCount = 0
	s.RejectCount = 0
	s.OrderCount = 0
	s.CancelCount = 0

	// 持仓
	s.Netpos = 0
	s.NetposPass = 0
	s.NetposPassYtd = 0
	s.NetposAgg = 0

	// 数量
	s.BuyQty = 0
	s.SellQty = 0
	s.BuyTotalQty = 0
	s.SellTotalQty = 0
	s.BuyOpenQty = 0
	s.SellOpenQty = 0

	// 价值
	s.BuyValue = 0
	s.SellValue = 0
	s.TransValue = 0
	s.BuyTotalValue = 0
	s.SellTotalValue = 0
	s.TransTotalValue = 0

	// 均价
	s.BuyPrice = 0
	s.SellPrice = 0
	s.BuyAvgPrice = 0
	s.SellAvgPrice = 0

	// PNL
	s.RealisedPNL = 0
	s.UnrealisedPNL = 0
	s.NetPNL = 0
	s.GrossPNL = 0
	s.MaxPNL = 0
	s.Drawdown = 0

	// 价格
	s.LTP = 0
	s.LastTradePx = 0

	// 状态标志
	s.OnExit = false
	s.OnCancel = false
	s.OnFlat = false
	s.AggFlat = false
	s.OnStopLoss = false
	s.OnNewsFlat = false
	s.Active = false

	// 订单
	s.BuyOpenOrders = 0
	s.SellOpenOrders = 0

	// 时间戳
	s.LastFlatTS = 0
	s.ExchTS = 0
	s.LocalTS = 0

	// BBO
	s.BestBidLastPNL = 0
	s.BestAskLastPNL = 0

	// 成交
	s.LastTradeSide = false
	s.LastTradeTime = 0
	s.LastTrade = false

	// 平均数量
	s.AvgQty = 0
	s.QtyCount = 0

	// SET_HIGH
	s.SetHigh = 0

	// RMS
	s.RmsQty = 0

	// 阈值派生
	s.TholdBidSize = 0
	s.TholdBidMaxPos = 0
	s.TholdAskSize = 0
	s.TholdAskMaxPos = 0

	// 止损时间戳
	s.StopLossTS = 0
}
