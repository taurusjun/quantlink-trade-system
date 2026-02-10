// Package strategy provides trading strategy implementations
package strategy

// ThresholdSet represents a collection of threshold configuration parameters
// C++: TradeBotUtils.h:237-420
type ThresholdSet struct {
	// === 入场/出场阈值 (Entry/Exit Thresholds) ===
	BeginPlace  float64 // BEGIN_PLACE - 空仓时入场阈值
	BeginRemove float64 // BEGIN_REMOVE - 空仓时移除阈值
	LongPlace   float64 // LONG_PLACE - 满仓多头时入场阈值
	LongRemove  float64 // LONG_REMOVE - 满仓多头时移除阈值
	ShortPlace  float64 // SHORT_PLACE - 满仓空头时入场阈值
	ShortRemove float64 // SHORT_REMOVE - 满仓空头时移除阈值
	LongInc     float64 // LONG_INC - 多头增仓阈值

	// === 订单数量控制 (Order Size Control) ===
	Size             int32 // SIZE - 单笔下单量
	BeginSize        int32 // BEGIN_SIZE - 初始下单量
	MaxSize          int32 // MAX_SIZE - 最大持仓
	BidSize          int32 // BID_SIZE - 买单数量
	AskSize          int32 // ASK_SIZE - 卖单数量
	BidMaxSize       int32 // BID_MAX_SIZE - 买单最大持仓
	AskMaxSize       int32 // ASK_MAX_SIZE - 卖单最大持仓
	SupportingOrders int32 // SUPPORTING_ORDERS - 追单数量限制
	MaxQuoteLevel    int32 // MAX_QUOTE_LEVEL - 最大挂单层数
	MaxOSOrder       int32 // MAX_OS_ORDER - 最大未成交订单数

	// === 主动单参数 (Aggressive Order Parameters) ===
	Cross        float64 // CROSS - 吃单阈值
	CloseCross   float64 // CLOSE_CROSS - 平仓吃单阈值
	Improve      float64 // IMPROVE - 改价阈值
	CloseImprove float64 // CLOSE_IMPROVE - 平仓改价阈值
	AggCoolOff   int64   // AGG_COOL_OFF - 主动单冷却时间(ms)

	// === 风控参数 (Risk Control Parameters) ===
	StopLoss float64 // STOP_LOSS - 止损阈值
	MaxLoss  float64 // MAX_LOSS - 最大亏损
	UPnlLoss float64 // UPNL_LOSS - 未实现亏损阈值
	PTLoss   float64 // PT_LOSS - 点数亏损
	PTProfit float64 // PT_PROFIT - 点数盈利
	MaxPrice float64 // MAX_PRICE - 最大价格限制
	MinPrice float64 // MIN_PRICE - 最小价格限制
	Slop     float64 // SLOP - 追单滑点(tick数)

	// === 均值回归参数 (Mean Reversion Parameters) ===
	Alpha         float64 // ALPHA - 均值回归学习率
	AvgSpreadAway float64 // AVG_SPREAD_AWAY - 价差偏离阈值

	// === 对冲参数 (Hedging Parameters) ===
	HedgeThres     float64 // HEDGE_THRES - 对冲触发阈值
	HedgeSizeRatio float64 // HEDGE_SIZE_RATIO - 对冲比例

	// === 额外风控参数 (Additional Risk Parameters) ===
	PilFactor  float64 // PIL_FACTOR - 盈亏因子
	OppQty     int32   // OPP_QTY - 对手方数量阈值
	PriceRatio float64 // PRICE_RATIO - 价格比例

	// === 时间参数 (Timing Parameters) ===
	Pause         int64 // PAUSE - 订单间隔(ms)
	CancelReqPause int64 // CANCELREQ_PAUSE - 撤单间隔(ms)
	PriceCooloff  int64 // PRICE_COOLOFF - 价格变化冷却(ms)
	SqrOffTime    int64 // SQROFF_TIME - 平仓时间(秒)
	SqrOffAgg     int64 // SQROFF_AGG - 主动平仓时间

	// === 统计参数 (Statistical Parameters) ===
	VWAPRatio      float64 // VWAP_RATIO - VWAP比率
	VWAPCount      int32   // VWAP_COUNT - VWAP计数
	VWAPDepth      int32   // VWAP_DEPTH - VWAP深度
	BidAskRatio    float64 // BIDASK_RATIO - 买卖比率
	SpreadEWA      float64 // SPREAD_EWA - 价差EWA系数
	MaxDeltaValue  float64 // MAX_DELTA_VALUE - 最大Delta值
	MinDeltaValue  float64 // MIN_DELTA_VALUE - 最小Delta值
	MaxDeltaChange float64 // MAX_DELTA_CHANGE - 最大Delta变化

	// === 布尔标志 (Boolean Flags) ===
	UseNotional     bool // USE_NOTIONAL - 使用名义价值
	UsePercent      bool // USE_PERCENT - 使用百分比
	UsePriceLimit   bool // USE_PRICE_LIMIT - 使用价格限制
	UseCloseCross   bool // USE_CLOSE_CROSS - 使用平仓吃单
	UsePassiveThold bool // USE_PASSIVE_THOLD - 使用被动阈值
	UseLinearThold  int32 // USE_LINEAR_THOLD - 使用线性阈值模式
	QuoteMaxQty     bool // QUOTE_MAX_QTY - 限制挂单数量
	ClosePNL        bool // CLOSE_PNL - 基于PNL平仓
	CheckPNL        bool // CHECK_PNL - 检查PNL
	NewsFlat        bool // NEWS_FLAT - 新闻时平仓

	// === 共享内存配置 (Shared Memory Configuration) ===
	// C++: ExecutionStrategy.cpp:99-113
	TVarKey   int // TVAR_KEY - tValue 共享内存键 (用于外部调整价差均值)
	TCacheKey int // TCACHE_KEY - tcache 共享内存键 (用于向外部共享持仓)
}

// NewThresholdSet creates a new ThresholdSet with default values
// C++: TradeBotUtils.h default values
func NewThresholdSet() *ThresholdSet {
	return &ThresholdSet{
		// 阈值默认值（需要从配置加载）
		BeginPlace:  0,
		BeginRemove: 0,
		LongPlace:   0,
		LongRemove:  0,
		ShortPlace:  0,
		ShortRemove: 0,
		LongInc:     0,

		// 数量默认值
		Size:             1,
		BeginSize:        1,
		MaxSize:          100,
		BidSize:          0,
		AskSize:          0,
		BidMaxSize:       0,
		AskMaxSize:       0,
		SupportingOrders: 3,
		MaxQuoteLevel:    1,
		MaxOSOrder:       5,

		// 主动单默认值
		Cross:        1e9,  // 默认不触发
		CloseCross:   1e11, // 默认不触发
		Improve:      1e9,  // 默认不触发
		CloseImprove: -1,
		AggCoolOff:   0,

		// 风控默认值
		StopLoss: 1e10,
		MaxLoss:  1e11,
		UPnlLoss: 1e10,
		PTLoss:   1e6,
		PTProfit: 1e6,
		MaxPrice: 1e12,
		MinPrice: -1000,
		Slop:     20,

		// 均值回归默认值
		Alpha:         0.0,   // 默认不启用
		AvgSpreadAway: 0.0,   // 默认不启用

		// 对冲默认值
		HedgeThres:     0.0, // 默认不启用
		HedgeSizeRatio: 1.0, // 默认1:1对冲

		// 额外风控默认值
		PilFactor:  1.0, // 默认1.0
		OppQty:     0,   // 默认不限制
		PriceRatio: 1.0, // 默认1.0

		// 时间默认值
		Pause:         0,
		CancelReqPause: 0,
		PriceCooloff:  0,
		SqrOffTime:    0,
		SqrOffAgg:     0,

		// 统计默认值
		VWAPRatio:      1,
		VWAPCount:      100,
		VWAPDepth:      10,
		BidAskRatio:    1,
		SpreadEWA:      0.6,
		MaxDeltaValue:  1,
		MinDeltaValue:  -1,
		MaxDeltaChange: 2,

		// 布尔标志默认值
		UseNotional:     false,
		UsePercent:      false,
		UsePriceLimit:   false,
		UseCloseCross:   false,
		UsePassiveThold: false,
		UseLinearThold:  0,
		QuoteMaxQty:     false,
		ClosePNL:        true,
		CheckPNL:        true,
		NewsFlat:        false,

		// 共享内存默认值（0 表示不启用）
		TVarKey:   0,
		TCacheKey: 0,
	}
}

// Clone creates a deep copy of ThresholdSet
func (ts *ThresholdSet) Clone() *ThresholdSet {
	if ts == nil {
		return nil
	}
	clone := *ts
	return &clone
}

// LoadFromMap loads threshold values from a configuration map
// This maps config parameter names to ThresholdSet fields
func (ts *ThresholdSet) LoadFromMap(params map[string]interface{}) {
	if ts == nil || params == nil {
		return
	}

	// 入场/出场阈值
	if val, ok := params["begin_place"].(float64); ok {
		ts.BeginPlace = val
	}
	if val, ok := params["begin_remove"].(float64); ok {
		ts.BeginRemove = val
	}
	if val, ok := params["long_place"].(float64); ok {
		ts.LongPlace = val
	}
	if val, ok := params["long_remove"].(float64); ok {
		ts.LongRemove = val
	}
	if val, ok := params["short_place"].(float64); ok {
		ts.ShortPlace = val
	}
	if val, ok := params["short_remove"].(float64); ok {
		ts.ShortRemove = val
	}
	if val, ok := params["long_inc"].(float64); ok {
		ts.LongInc = val
	}

	// 订单数量
	if val, ok := params["size"].(float64); ok {
		ts.Size = int32(val)
	}
	if val, ok := params["begin_size"].(float64); ok {
		ts.BeginSize = int32(val)
	}
	if val, ok := params["max_size"].(float64); ok {
		ts.MaxSize = int32(val)
	}
	if val, ok := params["bid_size"].(float64); ok {
		ts.BidSize = int32(val)
	}
	if val, ok := params["ask_size"].(float64); ok {
		ts.AskSize = int32(val)
	}
	if val, ok := params["bid_max_size"].(float64); ok {
		ts.BidMaxSize = int32(val)
	}
	if val, ok := params["ask_max_size"].(float64); ok {
		ts.AskMaxSize = int32(val)
	}
	if val, ok := params["supporting_orders"].(float64); ok {
		ts.SupportingOrders = int32(val)
	}
	if val, ok := params["max_quote_level"].(float64); ok {
		ts.MaxQuoteLevel = int32(val)
	}
	if val, ok := params["max_os_order"].(float64); ok {
		ts.MaxOSOrder = int32(val)
	}

	// 主动单参数
	if val, ok := params["cross"].(float64); ok {
		ts.Cross = val
	}
	if val, ok := params["close_cross"].(float64); ok {
		ts.CloseCross = val
	}
	if val, ok := params["improve"].(float64); ok {
		ts.Improve = val
	}
	if val, ok := params["close_improve"].(float64); ok {
		ts.CloseImprove = val
	}
	if val, ok := params["agg_cool_off"].(float64); ok {
		ts.AggCoolOff = int64(val)
	}

	// 风控参数
	if val, ok := params["stop_loss"].(float64); ok {
		ts.StopLoss = val
	}
	if val, ok := params["max_loss"].(float64); ok {
		ts.MaxLoss = val
	}
	if val, ok := params["upnl_loss"].(float64); ok {
		ts.UPnlLoss = val
	}
	if val, ok := params["pt_loss"].(float64); ok {
		ts.PTLoss = val
	}
	if val, ok := params["pt_profit"].(float64); ok {
		ts.PTProfit = val
	}
	if val, ok := params["max_price"].(float64); ok {
		ts.MaxPrice = val
	}
	if val, ok := params["min_price"].(float64); ok {
		ts.MinPrice = val
	}
	if val, ok := params["slop"].(float64); ok {
		ts.Slop = val
	}

	// 均值回归参数
	if val, ok := params["alpha"].(float64); ok {
		ts.Alpha = val
	}
	if val, ok := params["avg_spread_away"].(float64); ok {
		ts.AvgSpreadAway = val
	}

	// 对冲参数
	if val, ok := params["hedge_thres"].(float64); ok {
		ts.HedgeThres = val
	}
	if val, ok := params["hedge_size_ratio"].(float64); ok {
		ts.HedgeSizeRatio = val
	}

	// 额外风控参数
	if val, ok := params["pil_factor"].(float64); ok {
		ts.PilFactor = val
	}
	if val, ok := params["opp_qty"].(float64); ok {
		ts.OppQty = int32(val)
	}
	if val, ok := params["price_ratio"].(float64); ok {
		ts.PriceRatio = val
	}

	// 时间参数
	if val, ok := params["pause"].(float64); ok {
		ts.Pause = int64(val)
	}
	if val, ok := params["cancel_req_pause"].(float64); ok {
		ts.CancelReqPause = int64(val)
	}
	if val, ok := params["price_cooloff"].(float64); ok {
		ts.PriceCooloff = int64(val)
	}
	if val, ok := params["sqroff_time"].(float64); ok {
		ts.SqrOffTime = int64(val)
	}
	if val, ok := params["sqroff_agg"].(float64); ok {
		ts.SqrOffAgg = int64(val)
	}

	// 统计参数
	if val, ok := params["vwap_ratio"].(float64); ok {
		ts.VWAPRatio = val
	}
	if val, ok := params["vwap_count"].(float64); ok {
		ts.VWAPCount = int32(val)
	}
	if val, ok := params["vwap_depth"].(float64); ok {
		ts.VWAPDepth = int32(val)
	}
	if val, ok := params["bidask_ratio"].(float64); ok {
		ts.BidAskRatio = val
	}
	if val, ok := params["spread_ewa"].(float64); ok {
		ts.SpreadEWA = val
	}
	if val, ok := params["max_delta_value"].(float64); ok {
		ts.MaxDeltaValue = val
	}
	if val, ok := params["min_delta_value"].(float64); ok {
		ts.MinDeltaValue = val
	}
	if val, ok := params["max_delta_change"].(float64); ok {
		ts.MaxDeltaChange = val
	}

	// 布尔标志
	if val, ok := params["use_notional"].(bool); ok {
		ts.UseNotional = val
	}
	if val, ok := params["use_percent"].(bool); ok {
		ts.UsePercent = val
	}
	if val, ok := params["use_price_limit"].(bool); ok {
		ts.UsePriceLimit = val
	}
	if val, ok := params["use_close_cross"].(bool); ok {
		ts.UseCloseCross = val
	}
	if val, ok := params["use_passive_thold"].(bool); ok {
		ts.UsePassiveThold = val
	}
	if val, ok := params["use_linear_thold"].(float64); ok {
		ts.UseLinearThold = int32(val)
	}
	if val, ok := params["quote_max_qty"].(bool); ok {
		ts.QuoteMaxQty = val
	}
	if val, ok := params["close_pnl"].(bool); ok {
		ts.ClosePNL = val
	}
	if val, ok := params["check_pnl"].(bool); ok {
		ts.CheckPNL = val
	}
	if val, ok := params["news_flat"].(bool); ok {
		ts.NewsFlat = val
	}

	// 共享内存配置
	if val, ok := params["tvar_key"].(float64); ok {
		ts.TVarKey = int(val)
	}
	if val, ok := params["tcache_key"].(float64); ok {
		ts.TCacheKey = int(val)
	}
}

// GetLongPlaceDiff returns LONG_PLACE - BEGIN_PLACE
// C++: auto long_place_diff_thold = m_thold_first->LONG_PLACE - m_thold_first->BEGIN_PLACE;
func (ts *ThresholdSet) GetLongPlaceDiff() float64 {
	return ts.LongPlace - ts.BeginPlace
}

// GetShortPlaceDiff returns BEGIN_PLACE - SHORT_PLACE
// C++: auto short_place_diff_thold = m_thold_first->BEGIN_PLACE - m_thold_first->SHORT_PLACE;
func (ts *ThresholdSet) GetShortPlaceDiff() float64 {
	return ts.BeginPlace - ts.ShortPlace
}

// CalculateDynamicThreshold calculates dynamic threshold based on position
// C++: SetThresholds() logic
// Returns (bidThreshold, askThreshold)
func (ts *ThresholdSet) CalculateDynamicThreshold(netPos int32, maxPos int32) (float64, float64) {
	if maxPos == 0 {
		return ts.BeginPlace, ts.BeginPlace
	}

	longPlaceDiff := ts.GetLongPlaceDiff()
	shortPlaceDiff := ts.GetShortPlaceDiff()
	posRatio := float64(netPos) / float64(maxPos)

	var bidThreshold, askThreshold float64

	if netPos == 0 {
		// C++: 无持仓时使用初始阈值
		bidThreshold = ts.BeginPlace
		askThreshold = ts.BeginPlace
	} else if netPos > 0 {
		// C++: 多头持仓
		// tholdBidPlace = BEGIN_PLACE + long_place_diff_thold * netpos / maxPos
		bidThreshold = ts.BeginPlace + longPlaceDiff*posRatio
		// tholdAskPlace = BEGIN_PLACE - short_place_diff_thold * netpos / maxPos
		askThreshold = ts.BeginPlace - shortPlaceDiff*posRatio
	} else {
		// C++: 空头持仓 (netpos < 0)
		// tholdBidPlace = BEGIN_PLACE + short_place_diff_thold * netpos / maxPos
		bidThreshold = ts.BeginPlace + shortPlaceDiff*posRatio
		// tholdAskPlace = BEGIN_PLACE - long_place_diff_thold * netpos / maxPos
		askThreshold = ts.BeginPlace - longPlaceDiff*posRatio
	}

	return bidThreshold, askThreshold
}
