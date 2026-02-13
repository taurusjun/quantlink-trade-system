package types

// ThresholdSet 对应 C++ struct ThresholdSet
// 参考: tbsrc/main/include/TradeBotUtils.h:237-504
type ThresholdSet struct {
	// 布尔标志
	UseNotional     bool // USE_NOTIONAL
	UsePercent      bool // USE_PERCENT
	UsePriceLimit   bool // USE_PRICE_LIMIT
	UseAheadPercent bool // USE_AHEAD_PERCENT
	UseCloseCross   bool // USE_CLOSE_CROSS
	UsePassiveThold bool // USE_PASSIVE_THOLD
	UseLinearThold  bool // USE_LINEAR_THOLD
	QuoteMaxQty     bool // QUOTE_MAX_QTY
	ClosePNL        bool // CLOSE_PNL
	CheckPNL        bool // CHECK_PNL
	NewsFlat        bool // NEWS_FLAT

	// 入场/出场阈值
	BeginPlace  float64 // BEGIN_PLACE
	BeginRemove float64 // BEGIN_REMOVE
	LongPlace   float64 // LONG_PLACE
	LongRemove  float64 // LONG_REMOVE
	ShortPlace  float64 // SHORT_PLACE
	ShortRemove float64 // SHORT_REMOVE
	LongInc     float64 // LONG_INC

	// 高波动变体
	BeginPlaceHigh float64 // BEGIN_PLACE_HIGH
	LongPlaceHigh  float64 // LONG_PLACE_HIGH

	// 仓位大小
	Size          int32 // SIZE
	TASize        int32 // TA_SIZE
	BeginSize     int32 // BEGIN_SIZE
	MaxSize       int32 // MAX_SIZE
	PercentSize   int32 // PERCENT_SIZE
	PercentLevel  int32 // PERCENT_LEVEL
	NotionalSize  int32 // NOTIONAL_SIZE
	NotionalMaxSz int32 // NOTIONAL_MAX_SIZE
	SMSRatio      int32 // SMS_RATIO
	MaxOSOrder    int32 // MAX_OS_ORDER
	BidSize       int32 // BID_SIZE
	BidMaxSize    int32 // BID_MAX_SIZE
	AskSize       int32 // ASK_SIZE
	AskMaxSize    int32 // ASK_MAX_SIZE

	// 激进单
	Cross        float64 // CROSS
	CloseCross   float64 // CLOSE_CROSS
	CloseImprove float64 // CLOSE_IMPROVE
	Improve      float64 // IMPROVE
	MaxCross     int32   // MAX_CROSS
	MaxLongCross int32   // MAX_LONG_CROSS
	MaxShortCross int32  // MAX_SHORT_CROSS
	CrossTarget  int32   // CROSS_TARGET
	CrossTicks   int32   // CROSS_TICKS
	AggCoolOff   int64   // AGG_COOL_OFF
	PlaceSpread  float64 // PLACE_SPREAD
	PILFactor    float64 // PIL_FACTOR

	// 风控
	StopLoss float64 // STOP_LOSS
	MaxLoss  float64 // MAX_LOSS
	UPNLLoss float64 // UPNL_LOSS
	PTProft  float64 // PT_PROFIT
	PTLoss   float64 // PT_LOSS
	MaxPrice float64 // MAX_PRICE
	MinPrice float64 // MIN_PRICE

	// 队列与大小
	OppQty        float64 // OPP_QTY
	SuppTolerance int     // SUPP_TOLERANCE
	AheadPercent  float64 // AHEAD_PERCENT
	AheadSize     float64 // AHEAD_SIZE
	SzAheadNoCxl  int     // SZAHEAD_NOCXL
	BookSzNoCxl   int     // BOOKSZ_NOCXL
	AggFlatBookSz int     // AGGFLAT_BOOKSIZE
	AggFlatBookFr float64 // AGGFLAT_BOOKFRAC

	// 套利
	Alpha        float64 // ALPHA
	SpreadEWA    float64 // SPREAD_EWA
	AvgSpreadAway int    // AVG_SPREAD_AWAY
	HedgeRatio   float64 // HEDGE_RATIO
	HedgeThres   float64 // HEDGE_THRES
	HedgeSzRatio float64 // HEDGE_SIZE_RATIO
	Const        float64 // CONST
	PriceRatio   float64 // PRICE_RATIO

	// Slop
	Slop int // SLOP

	// 时间
	Pause         int64 // PAUSE
	CancelReqPause int64 // CANCELREQ_PAUSE
	SqrOffTime    int64 // SQROFF_TIME
	SqrOffAgg     int   // SQROFF_AGG

	// Quote 相关
	QuoteSkew     float64 // QUOTE_SKEW
	MaxQuoteSpread int32  // MAX_QUOTE_SPREAD
	MaxQuoteLevel int     // MAX_QUOTE_LEVEL
	QuoteSignal   int     // QUOTE_SIGNAL

	// 统计
	StatDurationSmall int64   // STAT_DURATION_SMALL
	StatDurationLong  int64   // STAT_DURATION_LONG
	StatTradeThresh   float64 // STAT_TRADE_THRESH
	StatDecay         int     // STAT_DECAY

	// Delta
	DeltaHedge    float64 // DELTA_HEDGE
	TargetDelta   float64 // TARGET_DELTA
	MaxDeltaValue float64 // MAX_DELTA_VALUE
	MinDeltaValue float64 // MIN_DELTA_VALUE
	MaxDeltaChange float64 // MAX_DELTA_CHANGE

	// VWAP
	VWAPRatio float64 // VWAP_RATIO
	VWAPCount float64 // VWAP_COUNT
	VWAPDepth float64 // VWAP_DEPTH
	BidAskRatio float64 // BIDASK_RATIO

	// 支撑/拖尾
	SupportingOrders int32 // SUPPORTING_ORDERS
	TailingOrders    int32 // TAILING_ORDERS
	MaxOrders        int32 // MAX_ORDERS

	// PCA
	PCACoeff1 float64 // PCA_COEFF1
	PCACoeff2 float64 // PCA_COEFF2
	PCACoeff3 float64 // PCA_COEFF3

	// 杂项
	MinExtrInd   int     // MIN_EXTR_IND
	TargetStdDev float64 // TARGET_STD_DEV
	PriceCooloff int     // PRICE_COOLOFF

	// Sweep
	SweepPlace      int32 // SWEEP_PLACE
	SweepClose      int32 // SWEEP_CLOSE
	SweepPlaceLevel int32 // SWEEP_PLACE_LEVEL
	SweepCloseLevel int32 // SWEEP_CLOSE_LEVEL

	// tvar/tcache
	TVarKey   int32 // TVAR_KEY
	TCacheKey int32 // TCACHE_KEY
}

// NewThresholdSet 创建带 C++ 默认值的 ThresholdSet
// 参考: tbsrc/main/include/TradeBotUtils.h 构造函数 (lines 237-320)
func NewThresholdSet() *ThresholdSet {
	return &ThresholdSet{
		// 布尔默认值
		UseNotional:     false,
		UsePercent:      false,
		UsePriceLimit:   false,
		UseAheadPercent: false,
		UseCloseCross:   false,
		UsePassiveThold: false,
		UseLinearThold:  false,
		QuoteMaxQty:     false,
		ClosePNL:        true,
		CheckPNL:        true,
		NewsFlat:        false,

		// 仓位
		MaxOSOrder:   5,
		PercentLevel: 1,

		// 风控 — 极大值表示"不启用"
		UPNLLoss: 10000000000,
		StopLoss: 10000000000,
		MaxLoss:  100000000000,
		PTProft:  1000000,
		PTLoss:   1000000,
		MaxPrice: 1000000000000,
		MinPrice: -1000,

		// 队列
		OppQty:        1000000000,
		SuppTolerance: 1,
		AheadPercent:  100,
		AheadSize:     1000000000000,
		SzAheadNoCxl:  1000000,
		BookSzNoCxl:   1000000,

		// 激进单 — 极大值表示"不启用"
		Cross:         1000000000,
		CloseCross:    100000000000,
		Improve:       1000000000,
		MaxCross:      1000000000,
		MaxLongCross:  1000000000,
		MaxShortCross: 1000000000,
		CloseImprove:  -1,

		// 套利
		SpreadEWA:     0.6,
		AvgSpreadAway: 20,
		Slop:          20,

		// Quote
		MaxQuoteSpread: 1000000000,
		MaxQuoteLevel:  3,

		// VWAP
		VWAPRatio:   1,
		VWAPCount:   100,
		VWAPDepth:   10,
		BidAskRatio: 1,

		// 统计
		StatDurationLong: 1,
		StatDecay:        5,

		// Delta
		DeltaHedge:     100000,
		MaxDeltaValue:  1,
		MinDeltaValue:  -1,
		MaxDeltaChange: 2,

		// Sweep
		SweepPlaceLevel: 0,
		SweepCloseLevel: 0,

		// tvar/tcache
		TVarKey:   -1,
		TCacheKey: -1,

		// BID/ASK 独立大小
		BidSize:    0,
		BidMaxSize: 0,
		AskSize:    0,
		AskMaxSize: 0,
	}
}
