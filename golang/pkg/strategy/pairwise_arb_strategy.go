// Package strategy provides trading strategy implementations
package strategy

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
	orspb "github.com/yourusername/quantlink-trade-system/pkg/proto/ors"
	"github.com/yourusername/quantlink-trade-system/pkg/strategy/spread"
)

// PairwiseArbStrategy implements a statistical arbitrage / pairs trading strategy
// It identifies and trades mean-reverting spread between two correlated instruments
//
// C++: class PairwiseArbStrategy : public ExecutionStrategy
// Go:  type PairwiseArbStrategy struct { *ExecutionStrategy, *StrategyDataContext }
//
// 架构完全与 C++ 一致：
// - 继承 ExecutionStrategy（Go 使用嵌入）
// - m_firstStrat, m_secondStrat 是 ExtraStrategy* 指针
// - StrategyDataContext 提供 Go 特有的策略管理字段（与其他策略保持一致）
type PairwiseArbStrategy struct {
	*ExecutionStrategy   // C++: public ExecutionStrategy
	*StrategyDataContext // Go 特有字段（指标、配置、状态等）- 与其他策略保持一致

	// === 腿策略对象（C++: m_firstStrat, m_secondStrat） ===
	// 使用 ExtraStrategy 封装每条腿的持仓、订单和阈值管理
	firstStrat  *ExtraStrategy // 第一条腿（原 leg1*）
	secondStrat *ExtraStrategy // 第二条腿（原 leg2*）

	// === 阈值配置（C++: m_thold_first, m_thold_second） ===
	tholdFirst  *ThresholdSet // 第一条腿阈值配置（用于动态阈值计算）
	tholdSecond *ThresholdSet // 第二条腿阈值配置（C++: m_thold_second）

	// Strategy parameters
	symbol1           string  // First symbol (e.g., "ag2412")
	symbol2           string  // Second symbol (e.g., "ag2501")
	lookbackPeriod    int     // Period for mean/std calculation (default: 100)
	entryZScore       float64 // Z-score threshold to enter (default: 2.0)
	exitZScore        float64 // Z-score threshold to exit (default: 0.5)
	orderSize         int64   // Size per leg (default: 10)
	maxPositionSize   int64   // Maximum position per leg (default: 50)
	minCorrelation    float64 // Minimum correlation to trade (default: 0.7)
	hedgeRatio        float64 // 对冲比率，当前固定为 1.0（同品种跨期套利）
	spreadType        string  // "ratio" or "difference" (default: "difference")
	useCointegration  bool    // Use cointegration instead of correlation (default: false)

	// State
	price1            float64
	price2            float64
	bid1              float64  // 品种1买一价
	ask1              float64  // 品种1卖一价
	bid2              float64  // 品种2买一价
	ask2              float64  // 品种2卖一价
	lastTradeTime     time.Time
	minTradeInterval  time.Duration
	slippageTicks     int     // 滑点(tick数)
	useAggressivePrice bool   // 是否使用主动成交价格

	// 动态阈值参数（参考旧系统 SetThresholds）
	beginZScore         float64       // 空仓时入场阈值
	longZScore          float64       // 满仓多头时做多阈值
	shortZScore         float64       // 满仓空头时做空阈值
	useDynamicThreshold bool          // 是否启用动态阈值
	entryZScoreBid      float64       // 运行时：做多入场阈值
	entryZScoreAsk      float64       // 运行时：做空入场阈值

	// 撤单阈值（C++: m_tholdBidRemove/m_tholdAskRemove）
	// 用于在价差回归均值时撤销偏离的挂单
	exitZScoreBid       float64       // 运行时：做多撤单阈值 (tholdBidRemove)
	exitZScoreAsk       float64       // 运行时：做空撤单阈值 (tholdAskRemove)
	longExitZScore      float64       // 满仓多头时撤单阈值 (LONG_REMOVE)
	shortExitZScore     float64       // 满仓空头时撤单阈值 (SHORT_REMOVE)

	// 主动追单参数（参考旧系统 SendAggressiveOrder）
	aggressiveEnabled       bool          // 是否启用追单
	aggressiveInterval      time.Duration // 追单间隔
	aggressiveMaxRetry      int           // 最大追单次数
	aggressiveSlopTicks     int           // 跳跃tick数
	aggressiveFailThreshold int           // 失败阈值

	// 追单运行时状态
	aggRepeat     int       // 当前追单次数
	aggDirection  int       // 追单方向（1=买，-1=卖，0=无）
	aggLastTime   time.Time // 上次追单时间
	aggFailCount  int       // 连续失败次数

	// Spread analyzer (encapsulates spread calculation and statistics)
	spreadAnalyzer    *spread.SpreadAnalyzer

	// === 兼容字段（逐步迁移到 ExtraStrategy） ===
	// 这些字段保留用于兼容现有代码，新代码应使用 firstStrat/secondStrat
	leg1Position      int64 // 兼容：使用 firstStrat.NetPos
	leg2Position      int64 // 兼容：使用 secondStrat.NetPos
	leg1YtdPosition   int64 // 兼容：使用 firstStrat.NetPosPassYtd
	leg2YtdPosition   int64 // 兼容：使用 secondStrat.NetPosPassYtd

	// 多层挂单参数（C++: MAX_QUOTE_LEVEL）
	maxQuoteLevel    int     // 最大挂单层数 (默认: 1, 仅一档)
	quoteLevelSizes  []int64 // 每层下单量 (默认: [orderSize])
	enableMultiLevel bool    // 是否启用多层挂单

	// 订单簿深度（5档价格）
	bidPrices1 []float64 // Leg1 买盘 5 档价格
	askPrices1 []float64 // Leg1 卖盘 5 档价格
	bidPrices2 []float64 // Leg2 买盘 5 档价格
	askPrices2 []float64 // Leg2 卖盘 5 档价格

	// === 已废弃：订单映射已迁移到 ExtraStrategy ===
	// leg1OrderMap 和 leg2OrderMap 现在使用 firstStrat.OrdMap/BidMap/AskMap
	// 保留引用用于兼容
	leg1OrderMap *OrderPriceMap // 兼容：使用 firstStrat 的 maps
	leg2OrderMap *OrderPriceMap // 兼容：使用 secondStrat 的 maps

	// 价格优化参数（C++: GetBidPrice_first 隐性订单簿检测）
	enablePriceOptimize bool    // 是否启用价格优化
	priceOptimizeGap    int     // 触发优化的 tick 跳跃数
	tickSize1           float64 // Leg1 最小变动单位
	tickSize2           float64 // Leg2 最小变动单位

	// 外部 tValue 调整参数（C++: avgSpreadRatio = avgSpreadRatio_ori + tValue）
	// tValue 允许外部信号调整价差均值，使策略更容易入场或出场
	avgSpreadRatio_ori float64 // C++: avgSpreadRatio_ori - 原始价差均值（从 daily_init 加载）
	tValue             float64 // 外部调整值（正值提高均值，负值降低均值）

	// === 风控字段 (C++: PairwiseArbStrategy.h) ===
	maxLossLimit  float64 // m_maxloss_limit - 最大亏损限制
	isValidMkdata bool    // is_valid_mkdata - 行情数据是否有效

	// === 价差辅助字段 (C++: PairwiseArbStrategy.cpp) ===
	currSpreadRatioPrev float64 // currSpreadRatio_prev - 前一价差比率
	expectedRatio       float64 // expectedRatio - 期望比率
	iu                  float64 // iu - 内部变量
	count               float64 // count - 计数器

	// === 追单辅助字段 ===
	secondOrdIDStart uint32 // second_ordIDstart - 第二腿订单ID起始

	// === 矩阵数据 (C++: mx_daily_init) ===
	mxDailyInit map[string]map[string]float64 // 每日初始化矩阵

	// === PairwiseArb 特有的额外字段 ===
	// 注意：基础字段已通过 StrategyDataContext 提供
	estimatedPosition *EstimatedPosition // 估计持仓（用于 UI）- 配对策略的特殊持仓
	pnl               *PNL               // 盈亏统计
	riskMetrics       *RiskMetrics       // 风险指标
	running           bool               // 运行状态（需要单独维护，因为配对策略有特殊逻辑）

	mu sync.RWMutex
}

// NewPairwiseArbStrategy creates a new pairs trading strategy
// C++: PairwiseArbStrategy::PairwiseArbStrategy(CommonClient*, SimConfig*)
func NewPairwiseArbStrategy(id string) *PairwiseArbStrategy {
	maxHistoryLen := 200

	// 创建 ExecutionStrategy 基类（C++: ExecutionStrategy 构造函数）
	// 使用字符串 ID 的哈希作为 int32 StrategyID
	strategyID := int32(hashStringToInt(id))
	baseExecStrategy := NewExecutionStrategy(strategyID, &Instrument{Symbol: "", TickSize: 1.0})

	// 创建 StrategyDataContext（Go 特有，与其他策略保持一致）
	// 不自动激活，需要手动调用 Start()
	dataContext := NewStrategyDataContext(id, "pairwise_arb")
	dataContext.ControlState = NewStrategyControlState(false) // 默认不激活

	// 创建 ExtraStrategy 实例（C++: m_firstStrat, m_secondStrat）
	// 注意：Instrument 将在 Initialize 中设置正确的值
	firstStrat := NewExtraStrategy(1, &Instrument{Symbol: "", TickSize: 1.0})
	secondStrat := NewExtraStrategy(2, &Instrument{Symbol: "", TickSize: 1.0})

	// 创建阈值配置（C++: m_thold_first, m_thold_second）
	tholdFirst := NewThresholdSet()
	tholdSecond := NewThresholdSet()

	pas := &PairwiseArbStrategy{
		ExecutionStrategy:   baseExecStrategy,
		StrategyDataContext: dataContext,
		// ExtraStrategy 实例
		firstStrat:  firstStrat,
		secondStrat: secondStrat,
		tholdFirst:  tholdFirst,
		tholdSecond: tholdSecond,
		// === PairwiseArb 特有字段 ===
		running:           false,
		estimatedPosition: &EstimatedPosition{},
		pnl:               &PNL{},
		riskMetrics:       &RiskMetrics{},
		// 基本参数
		lookbackPeriod:   100,
		entryZScore:      2.0,
		exitZScore:       0.5,
		orderSize:        10,
		maxPositionSize:  50,
		minCorrelation:   0.7,
		hedgeRatio:       1.0,
		spreadType:       "difference",
		useCointegration: false,
		minTradeInterval: 3 * time.Second,
		// SpreadAnalyzer 将在 Initialize 中创建（需要知道 symbol 名称）
		spreadAnalyzer: nil,
		// 多层挂单默认值
		maxQuoteLevel:    1,
		quoteLevelSizes:  []int64{10},
		enableMultiLevel: false,
		// 订单簿深度
		bidPrices1: make([]float64, 5),
		askPrices1: make([]float64, 5),
		bidPrices2: make([]float64, 5),
		askPrices2: make([]float64, 5),
		// 订单映射（兼容）
		leg1OrderMap: NewOrderPriceMap(),
		leg2OrderMap: NewOrderPriceMap(),
		// 价格优化默认值
		enablePriceOptimize: false,
		priceOptimizeGap:    2,
		tickSize1:           1.0,
		tickSize2:           1.0,
	}

	// 预创建一个临时的 SpreadAnalyzer（将在 Initialize 时重新创建）
	pas.spreadAnalyzer = spread.NewSpreadAnalyzer("", "", spread.SpreadTypeDifference, maxHistoryLen)

	return pas
}

// hashStringToInt 将字符串转换为 int（用于生成 StrategyID）
func hashStringToInt(s string) int {
	h := 0
	for _, c := range s {
		h = 31*h + int(c)
	}
	if h < 0 {
		h = -h
	}
	return h
}

// Initialize initializes the strategy
func (pas *PairwiseArbStrategy) Initialize(config *StrategyConfig) error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	pas.Config = config

	// Validate we have exactly 2 symbols
	if len(config.Symbols) != 2 {
		return fmt.Errorf("pairwise arbitrage requires exactly 2 symbols")
	}

	pas.symbol1 = config.Symbols[0]
	pas.symbol2 = config.Symbols[1]

	// === 加载 StrategyID（C++: m_strategyID） ===
	// C++: m_strategyID 从配置文件读取，用于 daily_init 文件名
	// 例如：daily_init.92201 中的 92201 就是 m_strategyID
	if val, ok := config.Parameters["strategy_id"].(float64); ok {
		pas.ExecutionStrategy.StrategyID = int32(val)
		log.Printf("[PairwiseArbStrategy:%s] Loaded strategy_id from config: %d", pas.ID, pas.ExecutionStrategy.StrategyID)
	}
	// 如果配置中没有 strategy_id，保持构造函数中的哈希值

	// 初始化 ExtraStrategy 的 Instrument 信息（C++: m_firstStrat->m_instru, m_secondStrat->m_instru）
	pas.firstStrat.Instru = &Instrument{
		Symbol:   pas.symbol1,
		TickSize: pas.tickSize1,
	}
	pas.secondStrat.Instru = &Instrument{
		Symbol:   pas.symbol2,
		TickSize: pas.tickSize2,
	}

	// Load strategy-specific parameters (load spread_type first)
	if val, ok := config.Parameters["lookback_period"].(float64); ok {
		pas.lookbackPeriod = int(val)
	}
	if val, ok := config.Parameters["entry_zscore"].(float64); ok {
		pas.entryZScore = val
	}
	if val, ok := config.Parameters["exit_zscore"].(float64); ok {
		pas.exitZScore = val
	}
	if val, ok := config.Parameters["order_size"].(float64); ok {
		pas.orderSize = int64(val)
	}
	// 优先从 Parameters 读取 max_position_size，否则从顶层 MaxPositionSize 读取
	// 修复: 实盘配置通常在顶层设置 max_position_size，而不是在 parameters 中
	if val, ok := config.Parameters["max_position_size"].(float64); ok {
		pas.maxPositionSize = int64(val)
	} else if config.MaxPositionSize > 0 {
		pas.maxPositionSize = config.MaxPositionSize
	}
	if val, ok := config.Parameters["min_correlation"].(float64); ok {
		pas.minCorrelation = val
	}
	if val, ok := config.Parameters["spread_type"].(string); ok {
		pas.spreadType = val
	}

	// 初始化 SpreadAnalyzer（现在知道 symbol 和 spread_type 了）
	spreadType := spread.SpreadTypeDifference
	if pas.spreadType == "ratio" {
		spreadType = spread.SpreadTypeRatio
	}
	pas.spreadAnalyzer = spread.NewSpreadAnalyzer(pas.symbol1, pas.symbol2, spreadType, 200)
	if val, ok := config.Parameters["use_cointegration"].(bool); ok {
		pas.useCointegration = val
	}
	if val, ok := config.Parameters["trade_interval_ms"].(float64); ok {
		pas.minTradeInterval = time.Duration(val) * time.Millisecond
	}
	// 滑点参数（支持int或float64类型）
	if val, ok := config.Parameters["slippage_ticks"].(float64); ok {
		pas.slippageTicks = int(val)
	} else if val, ok := config.Parameters["slippage_ticks"].(int); ok {
		pas.slippageTicks = val
	}
	// 是否使用主动成交价格
	if val, ok := config.Parameters["use_market_price"].(bool); ok {
		pas.useAggressivePrice = val
	}

	// 动态阈值参数（与 C++ 配置一致）
	// C++ 对应: BEGIN_PLACE, LONG_PLACE, SHORT_PLACE
	if val, ok := config.Parameters["begin_zscore"].(float64); ok {
		pas.beginZScore = val
	} else {
		pas.beginZScore = pas.entryZScore // 默认使用 entry_zscore
	}
	if val, ok := config.Parameters["long_zscore"].(float64); ok {
		pas.longZScore = val
	}
	if val, ok := config.Parameters["short_zscore"].(float64); ok {
		pas.shortZScore = val
	}
	// 撤单阈值参数（C++: LONG_REMOVE, SHORT_REMOVE）
	if val, ok := config.Parameters["long_exit_zscore"].(float64); ok {
		pas.longExitZScore = val
	} else {
		// 默认使用 exitZScore * 1.5
		pas.longExitZScore = pas.exitZScore * 1.5
	}
	if val, ok := config.Parameters["short_exit_zscore"].(float64); ok {
		pas.shortExitZScore = val
	} else {
		// 默认使用 exitZScore * 0.5
		pas.shortExitZScore = pas.exitZScore * 0.5
	}
	// 是否启用动态阈值（需要配置 long_zscore 和 short_zscore）
	if val, ok := config.Parameters["use_dynamic_threshold"].(bool); ok {
		pas.useDynamicThreshold = val
	} else {
		// 如果配置了 long_zscore 和 short_zscore，则自动启用
		pas.useDynamicThreshold = pas.longZScore > 0 && pas.shortZScore > 0
	}
	// 初始化运行时阈值
	pas.entryZScoreBid = pas.beginZScore
	pas.entryZScoreAsk = pas.beginZScore
	pas.exitZScoreBid = pas.exitZScore
	pas.exitZScoreAsk = pas.exitZScore

	// 主动追单参数
	if val, ok := config.Parameters["aggressive_enabled"].(bool); ok {
		pas.aggressiveEnabled = val
	}
	if val, ok := config.Parameters["aggressive_interval_ms"].(float64); ok {
		pas.aggressiveInterval = time.Duration(val) * time.Millisecond
	} else {
		pas.aggressiveInterval = 500 * time.Millisecond // 默认 500ms
	}
	if val, ok := config.Parameters["aggressive_max_retry"].(float64); ok {
		pas.aggressiveMaxRetry = int(val)
	} else {
		pas.aggressiveMaxRetry = 4 // 默认 4 次
	}
	if val, ok := config.Parameters["aggressive_slop_ticks"].(float64); ok {
		pas.aggressiveSlopTicks = int(val)
	} else {
		pas.aggressiveSlopTicks = 20 // 默认 20 ticks
	}
	if val, ok := config.Parameters["aggressive_fail_threshold"].(float64); ok {
		pas.aggressiveFailThreshold = int(val)
	} else {
		pas.aggressiveFailThreshold = 3 // 默认 3 次
	}
	// C++: SUPPORTING_ORDERS - 限制追单数量，防止单方向发送过多追单
	// 关键参数！如果设置为 0 则不限制追单数量
	// 使用 tholdFirst.SupportingOrders 存储
	if val, ok := config.Parameters["supporting_orders"].(float64); ok {
		pas.tholdFirst.SupportingOrders = int32(val)
	} else {
		pas.tholdFirst.SupportingOrders = 3 // 默认限制 3 个追单
	}
	// 初始化追单状态
	pas.aggRepeat = 1
	pas.aggDirection = 0
	// 追单计数使用 secondStrat 的字段（C++: sellAggOrder, buyAggOrder）
	pas.secondStrat.SellAggOrder = 0
	pas.secondStrat.BuyAggOrder = 0

	// 多层挂单参数（C++: MAX_QUOTE_LEVEL）
	if val, ok := config.Parameters["enable_multi_level"].(bool); ok {
		pas.enableMultiLevel = val
	}
	// max_quote_level 支持 int 和 float64 类型
	if val, ok := config.Parameters["max_quote_level"].(float64); ok {
		pas.maxQuoteLevel = int(val)
	} else if val, ok := config.Parameters["max_quote_level"].(int); ok {
		pas.maxQuoteLevel = val
	}
	if pas.maxQuoteLevel < 1 {
		pas.maxQuoteLevel = 1
	}
	if pas.maxQuoteLevel > 5 {
		pas.maxQuoteLevel = 5 // 最多支持 5 档
	}

	// 每层下单量（支持数组配置，处理 []interface{} 中的 int/float64 元素）
	if val, ok := config.Parameters["quote_level_sizes"].([]interface{}); ok {
		pas.quoteLevelSizes = make([]int64, 0, len(val))
		for _, v := range val {
			switch size := v.(type) {
			case float64:
				pas.quoteLevelSizes = append(pas.quoteLevelSizes, int64(size))
			case int:
				pas.quoteLevelSizes = append(pas.quoteLevelSizes, int64(size))
			}
		}
	}
	// 如果未配置每层量，则使用默认的 orderSize
	if len(pas.quoteLevelSizes) == 0 {
		pas.quoteLevelSizes = make([]int64, pas.maxQuoteLevel)
		for i := range pas.quoteLevelSizes {
			pas.quoteLevelSizes[i] = pas.orderSize
		}
	}

	// 价格优化参数
	if val, ok := config.Parameters["enable_price_optimize"].(bool); ok {
		pas.enablePriceOptimize = val
	}
	if val, ok := config.Parameters["price_optimize_gap"].(float64); ok {
		pas.priceOptimizeGap = int(val)
	}
	// tick_size 参数
	if val, ok := config.Parameters["tick_size_1"].(float64); ok {
		pas.tickSize1 = val
	}
	if val, ok := config.Parameters["tick_size_2"].(float64); ok {
		pas.tickSize2 = val
	}

	log.Printf("[PairwiseArbStrategy:%s] Initialized %s/%s, entry_z=%.2f, exit_z=%.2f, lookback=%d, min_corr=%.2f, slippage=%d ticks",
		pas.ID, pas.symbol1, pas.symbol2, pas.entryZScore, pas.exitZScore, pas.lookbackPeriod, pas.minCorrelation, pas.slippageTicks)
	if pas.useDynamicThreshold {
		log.Printf("[PairwiseArbStrategy:%s] Dynamic threshold enabled: begin=%.2f, long=%.2f, short=%.2f",
			pas.ID, pas.beginZScore, pas.longZScore, pas.shortZScore)
	}
	if pas.aggressiveEnabled {
		log.Printf("[PairwiseArbStrategy:%s] Aggressive order enabled: interval=%v, max_retry=%d, slop_ticks=%d",
			pas.ID, pas.aggressiveInterval, pas.aggressiveMaxRetry, pas.aggressiveSlopTicks)
	}
	if pas.enableMultiLevel {
		log.Printf("[PairwiseArbStrategy:%s] Multi-level quoting enabled: max_level=%d, sizes=%v",
			pas.ID, pas.maxQuoteLevel, pas.quoteLevelSizes)
	}
	if pas.enablePriceOptimize {
		log.Printf("[PairwiseArbStrategy:%s] Price optimize enabled: gap=%d ticks, tick_size1=%.2f, tick_size2=%.2f",
			pas.ID, pas.priceOptimizeGap, pas.tickSize1, pas.tickSize2)
	}

	// === 加载新增参数（对齐 C++ TradeBot_China） ===
	// ALPHA - 均值回归学习率
	if val, ok := config.Parameters["alpha"].(float64); ok {
		pas.tholdFirst.Alpha = val
		log.Printf("[PairwiseArbStrategy:%s] Loaded alpha: %v", pas.ID, val)
	}
	// AVG_SPREAD_AWAY - 价差偏离阈值
	if val, ok := config.Parameters["avg_spread_away"].(float64); ok {
		pas.tholdFirst.AvgSpreadAway = val
		log.Printf("[PairwiseArbStrategy:%s] Loaded avg_spread_away: %v", pas.ID, val)
	}
	// HEDGE_THRES - 对冲触发阈值
	if val, ok := config.Parameters["hedge_thres"].(float64); ok {
		pas.tholdFirst.HedgeThres = val
	}
	// HEDGE_SIZE_RATIO - 对冲比例
	if val, ok := config.Parameters["hedge_size_ratio"].(float64); ok {
		pas.tholdFirst.HedgeSizeRatio = val
	}
	// PIL_FACTOR - 盈亏因子
	if val, ok := config.Parameters["pil_factor"].(float64); ok {
		pas.tholdFirst.PilFactor = val
	}
	// OPP_QTY - 对手方数量阈值
	if val, ok := config.Parameters["opp_qty"].(float64); ok {
		pas.tholdFirst.OppQty = int32(val)
	}
	// PRICE_RATIO - 价格比例
	if val, ok := config.Parameters["price_ratio"].(float64); ok {
		pas.tholdFirst.PriceRatio = val
	}

	// === 配置 ThresholdSet（C++: m_thold_first） ===
	// 将 Z-Score 阈值映射到 ThresholdSet 的 PLACE/REMOVE 字段
	pas.tholdFirst.BeginPlace = pas.beginZScore
	pas.tholdFirst.BeginRemove = pas.exitZScore
	pas.tholdFirst.LongPlace = pas.longZScore
	pas.tholdFirst.ShortPlace = pas.shortZScore
	pas.tholdFirst.MaxSize = int32(pas.maxPositionSize)
	pas.tholdFirst.Size = int32(pas.orderSize)
	pas.tholdFirst.Slop = float64(pas.aggressiveSlopTicks)

	// 将阈值配置关联到 firstStrat（C++: m_firstStrat->m_thold = m_thold_first）
	pas.firstStrat.Thold = pas.tholdFirst

	// 将阈值配置关联到 secondStrat（C++: m_secondStrat->m_thold = m_thold_second）
	// 注意：第二条腿可能有不同的阈值配置，这里默认复制第一条腿的配置
	pas.tholdSecond = pas.tholdFirst.Clone()
	pas.secondStrat.Thold = pas.tholdSecond

	// 更新 Instrument tick size（可能在参数加载后才确定）
	pas.firstStrat.Instru.TickSize = pas.tickSize1
	pas.secondStrat.Instru.TickSize = pas.tickSize2

	// === 加载共享内存配置（C++: TVAR_KEY, TCACHE_KEY） ===
	// 从配置中读取共享内存键值
	if val, ok := config.Parameters["tvar_key"].(float64); ok {
		pas.tholdFirst.TVarKey = int(val)
		pas.ExecutionStrategy.Thold.TVarKey = int(val)
	}
	if val, ok := config.Parameters["tcache_key"].(float64); ok {
		pas.tholdFirst.TCacheKey = int(val)
		pas.ExecutionStrategy.Thold.TCacheKey = int(val)
	}

	// 初始化共享内存（C++: ExecutionStrategy.cpp:99-113）
	// if (tvarKey > 0) { m_tvar = make_shared<hftlib::tvar<double>>(); m_tvar->init(tvarKey, 0666); }
	// if (tcacheKey > 0) { m_tcache = make_shared<hftlib::tcache<double>>(); m_tcache->init(tcacheKey); }
	if err := pas.ExecutionStrategy.InitSharedMemory(); err != nil {
		log.Printf("[PairwiseArbStrategy:%s] Warning: Failed to init shared memory: %v", pas.ID, err)
	}

	// === 加载 daily_init 文件（C++: PairwiseArbStrategy.cpp:18-62） ===
	// C++: auto mx_daily_init2 = LoadMatrix2(std::string("../data/daily_init.") + std::to_string(m_strategyID));
	// 使用 ExecutionStrategy.StrategyID 作为文件标识
	dailyInitPath := GetDailyInitPath(pas.ExecutionStrategy.StrategyID)
	mx_daily_init2, err := LoadMatrix2(dailyInitPath)
	if err != nil {
		log.Printf("[PairwiseArbStrategy:%s] LoadMatrix2: %v (will use default values)", pas.ID, err)
		// C++ 行为：找不到文件会 exit(-1)，这里我们继续但使用默认值
	} else {
		// C++: if (mx_daily_init2.find(m_strategyID) == mx_daily_init2.end()) { ... exit(-1); }
		row, exists := mx_daily_init2[pas.ExecutionStrategy.StrategyID]
		if !exists {
			log.Printf("[PairwiseArbStrategy:%s] daily_init ERROR! Missing m_strategyID %d",
				pas.ID, pas.ExecutionStrategy.StrategyID)
		} else {
			// C++: avgSpreadRatio_ori = std::stod(row["avgPx"]);
			// C++: avgSpreadRatio = avgSpreadRatio_ori;
			pas.avgSpreadRatio_ori = row.AvgPx
			pas.spreadAnalyzer.SetSpreadMean(pas.avgSpreadRatio_ori)
			log.Printf("[PairwiseArbStrategy:%s] Restored avgSpreadRatio_ori=%.6f from daily_init",
				pas.ID, pas.avgSpreadRatio_ori)

			// C++: int netpos_ytd1 = std::stoi(row["ytd1"]);     // 昨仓
			// C++: int netpos_2day1 = std::stoi(row["2day"]);    // 今仓（通常为 0）
			// C++: m_firstStrat->m_netpos_pass_ytd = netpos_ytd1;
			// C++: m_firstStrat->m_netpos = netpos_ytd1 + netpos_2day1;
			// C++: m_firstStrat->m_netpos_pass = netpos_ytd1 + netpos_2day1;
			netpos_ytd1 := row.Ytd1
			netpos_2day1 := row.TwoDay
			pas.firstStrat.NetPosPassYtd = netpos_ytd1
			pas.firstStrat.NetPos = netpos_ytd1 + netpos_2day1
			pas.firstStrat.NetPosPass = netpos_ytd1 + netpos_2day1
			// 更新兼容字段
			pas.leg1Position = int64(pas.firstStrat.NetPos)
			pas.leg1YtdPosition = int64(pas.firstStrat.NetPosPassYtd)

			// C++: int netpos_agg2 = std::stoi(row["ytd2"]);
			// C++: m_secondStrat->m_netpos = netpos_agg2;
			// C++: m_secondStrat->m_netpos_agg = netpos_agg2;
			netpos_agg2 := row.Ytd2
			pas.secondStrat.NetPos = netpos_agg2
			pas.secondStrat.NetPosAgg = netpos_agg2
			// 更新兼容字段
			pas.leg2Position = int64(pas.secondStrat.NetPos)

			log.Printf("[PairwiseArbStrategy:%s] Restored positions from daily_init: "+
				"firstStrat[netpos=%d, ytd=%d, 2day=%d], secondStrat[netpos_agg=%d]",
				pas.ID, pas.firstStrat.NetPos, netpos_ytd1, netpos_2day1, netpos_agg2)
		}
	}

	log.Printf("[PairwiseArbStrategy:%s] ExtraStrategy initialized: firstStrat(symbol=%s), secondStrat(symbol=%s)",
		pas.ID, pas.firstStrat.Instru.Symbol, pas.secondStrat.Instru.Symbol)

	return nil
}

// OnMarketData handles market data updates
func (pas *PairwiseArbStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	if !pas.running {
		return
	}

	// 从共享内存加载 tValue（C++: PairwiseArbStrategy.cpp:482-485）
	// if (m_tvar) {
	//     tValue = m_tvar->load();
	//     TBLOG << "get tvar:" << fixed << tValue << endl;
	// }
	if pas.ExecutionStrategy != nil && pas.ExecutionStrategy.TVar != nil {
		newTValue := pas.ExecutionStrategy.LoadTValue()
		if newTValue != pas.tValue {
			log.Printf("[PairwiseArbStrategy:%s] get tvar: %.6f (was %.6f)",
				pas.ID, newTValue, pas.tValue)
			pas.tValue = newTValue
		}
	}

	// Update indicators
	pas.PrivateIndicators.UpdateAll(md)

	// Track prices for both symbols
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return
	}
	midPrice := (md.BidPrice[0] + md.AskPrice[0]) / 2.0

	if md.Symbol == pas.symbol1 {
		pas.price1 = midPrice
		pas.bid1 = md.BidPrice[0]
		pas.ask1 = md.AskPrice[0]
		pas.spreadAnalyzer.UpdatePrice1(midPrice, int64(md.Timestamp))
		// 更新订单簿深度（多层挂单用）
		pas.updateOrderbookDepth(md.BidPrice, md.AskPrice, true)
	} else if md.Symbol == pas.symbol2 {
		pas.price2 = midPrice
		pas.bid2 = md.BidPrice[0]
		pas.ask2 = md.AskPrice[0]
		pas.spreadAnalyzer.UpdatePrice2(midPrice, int64(md.Timestamp))
		// 更新订单簿深度（多层挂单用）
		pas.updateOrderbookDepth(md.BidPrice, md.AskPrice, false)
	}

	// Need both prices to calculate spread
	if pas.price1 == 0 || pas.price2 == 0 {
		return
	}

	// Calculate spread and update statistics using SpreadAnalyzer
	pas.spreadAnalyzer.CalculateSpread()
	pas.spreadAnalyzer.UpdateAll(pas.lookbackPeriod)

	// === AVG_SPREAD_AWAY 保护机制（C++: PairwiseArbStrategy.cpp:506-517）===
	// 检查当前价差是否偏离均值过大，如果是则停止策略
	// C++: if (abs(currSpreadRatio - avgSpreadRatio) > m_tickSize * AVG_SPREAD_AWAY)
	if pas.tholdFirst.AvgSpreadAway > 0 && pas.isValidMkdata {
		currentSpread := pas.spreadAnalyzer.GetStats().CurrentSpread
		avgSpreadRatio := pas.getAvgSpreadRatio()
		spreadDeviation := math.Abs(currentSpread - avgSpreadRatio)
		maxDeviation := pas.tickSize1 * pas.tholdFirst.AvgSpreadAway

		if spreadDeviation > maxDeviation {
			pas.isValidMkdata = false
			log.Printf("[PairwiseArb:%s] ⚠️ AVG_SPREAD_AWAY triggered: deviation=%.4f > max=%.4f (curr=%.4f, avg=%.4f, tickSize=%.2f, away=%.2f)",
				pas.ID, spreadDeviation, maxDeviation, currentSpread, avgSpreadRatio, pas.tickSize1, pas.tholdFirst.AvgSpreadAway)

			// C++: if (m_Active) { HandleSquareoff(); }
			if pas.ControlState.RunState == StrategyRunStateActive {
				log.Printf("[PairwiseArb:%s] 🛑 Deactivating strategy due to AVG_SPREAD_AWAY", pas.ID)
				pas.ControlState.RunState = StrategyRunStateExiting
			}
			return
		}
		pas.isValidMkdata = true
	}

	// === EMA 更新 avgSpreadRatio_ori（C++: PairwiseArbStrategy.cpp:519-522）===
	// 收到第一腿行情时，使用 EMA 公式更新价差均值
	// C++: avgSpreadRatio_ori = (1 - ALPHA) * avgSpreadRatio_ori + ALPHA * currSpreadRatio;
	// C++: avgSpreadRatio = avgSpreadRatio_ori + tValue;
	if md.Symbol == pas.symbol1 && pas.tholdFirst.Alpha > 0 {
		currentSpread := pas.spreadAnalyzer.GetStats().CurrentSpread
		alpha := pas.tholdFirst.Alpha
		pas.avgSpreadRatio_ori = (1-alpha)*pas.avgSpreadRatio_ori + alpha*currentSpread
	}

	// Update PNL (配对策略专用计算：分别计算两腿)
	pas.updatePairwisePNL()

	// Update risk metrics (use average price for exposure calculation)
	avgPrice := (pas.price1 + pas.price2) / 2.0
	pas.updateRiskMetrics(avgPrice)

	// 动态调整入场阈值（根据持仓）
	pas.setDynamicThresholds()

	// C++: 检查并撤销偏离均值的挂单
	// 参考 PairwiseArbStrategy.cpp:205-228
	pas.cancelOutOfRangeOrders()

	// Get current statistics from SpreadAnalyzer
	spreadStats := pas.spreadAnalyzer.GetStats()

	// 计算敞口
	exposure := pas.calculateExposure()

	// Update condition state for UI display
	indicators := map[string]float64{
		"z_score":            spreadStats.ZScore,
		"entry_threshold":    pas.entryZScore,
		"entry_threshold_bid": pas.entryZScoreBid, // 动态做多阈值
		"entry_threshold_ask": pas.entryZScoreAsk, // 动态做空阈值
		"exit_threshold":     pas.exitZScore,
		"spread":             spreadStats.CurrentSpread,
		"spread_mean":        spreadStats.Mean,
		"spread_std":         spreadStats.Std,
		"correlation":        spreadStats.Correlation,
		"min_correlation":    pas.minCorrelation,
		"hedge_ratio":        spreadStats.HedgeRatio,
		// Leg 1 details
		"leg1_price":    pas.price1,
		"leg1_position": float64(pas.leg1Position),
		// Leg 2 details
		"leg2_price":    pas.price2,
		"leg2_position": float64(pas.leg2Position),
		// Exposure (敞口)
		"exposure":      float64(exposure),
	}

	// Conditions are met if:
	// 1. Z-score exceeds entry threshold (using dynamic thresholds)
	// 2. Correlation is above minimum
	// 3. Enough history data
	// 使用动态阈值判断：做多需要 -zscore >= entryZScoreBid，做空需要 zscore >= entryZScoreAsk
	conditionsMet := spreadStats.Std > 1e-10 &&
		(spreadStats.ZScore >= pas.entryZScoreAsk || -spreadStats.ZScore >= pas.entryZScoreBid) &&
		spreadStats.Correlation >= pas.minCorrelation &&
		pas.spreadAnalyzer.IsReady(pas.lookbackPeriod)

	// Update control state with current conditions
	pas.ControlState.UpdateConditions(conditionsMet, spreadStats.ZScore, indicators)

	// Check if we should trade
	now := time.Now()

	// Debug logging periodically (every 5 seconds)
	if time.Since(pas.lastTradeTime) > 5*time.Second {
		if pas.useDynamicThreshold {
			log.Printf("[PairwiseArb:%s] Stats: zscore=%.2f (bid>=%.2f, ask>=%.2f), corr=%.3f, pos=%d, exposure=%d",
				pas.ID, spreadStats.ZScore, pas.entryZScoreBid, pas.entryZScoreAsk,
				spreadStats.Correlation, pas.leg1Position, exposure)
		} else {
			log.Printf("[PairwiseArb:%s] Stats: zscore=%.2f (need ±%.2f), corr=%.3f (need %.3f), std=%.4f, ready=%v, condMet=%v",
				pas.ID, spreadStats.ZScore, pas.entryZScore, spreadStats.Correlation, pas.minCorrelation,
				spreadStats.Std, pas.spreadAnalyzer.IsReady(pas.lookbackPeriod), conditionsMet)
		}
	}

	// 主动追单检测（优先于正常交易逻辑）
	pas.sendAggressiveOrder()

	if now.Sub(pas.lastTradeTime) < pas.minTradeInterval {
		return
	}

	// Check correlation before trading
	if spreadStats.Correlation < pas.minCorrelation {
		return
	}

	// Generate signals based on z-score
	// 使用多层挂单或单层挂单
	if pas.enableMultiLevel {
		pas.generateMultiLevelSignals(md)
	} else {
		pas.generateSignals(md)
	}
	pas.lastTradeTime = now
}


// generateSignals generates trading signals based on z-score
// 使用动态阈值：
// - 做多（long spread）：-zscore >= entryZScoreBid
// - 做空（short spread）：zscore >= entryZScoreAsk
//
// tValue 调整（C++: avgSpreadRatio = avgSpreadRatio_ori + tValue）：
// - tValue > 0: 提高均值，使做空更容易触发（zscore更大）
// - tValue < 0: 降低均值，使做多更容易触发（zscore更小/更负）
func (pas *PairwiseArbStrategy) generateSignals(md *mdpb.MarketDataUpdate) {
	// Get current statistics from SpreadAnalyzer
	spreadStats := pas.spreadAnalyzer.GetStats()

	if spreadStats.Std < 1e-10 {
		return
	}

	// === 计算 Z-Score（C++: PairwiseArbStrategy.cpp:205-206）===
	// C++: currSpreadRatio = mid1 - mid2 * PRICE_RATIO;
	// C++: expectedRatio = (currSpreadRatio - avgSpreadRatio) / m_stdevSpreadRatio;
	// C++: avgSpreadRatio = avgSpreadRatio_ori + tValue
	//
	// 如果启用了 EMA（alpha > 0），使用 avgSpreadRatio_ori 作为均值
	// 否则回退到 SpreadAnalyzer 的 SMA 均值
	var adjustedZScore float64
	if pas.tholdFirst.Alpha > 0 && pas.avgSpreadRatio_ori != 0 {
		// 使用 EMA 均值（与 C++ 一致）
		avgSpreadRatio := pas.avgSpreadRatio_ori + pas.tValue
		adjustedZScore = (spreadStats.CurrentSpread - avgSpreadRatio) / spreadStats.Std
	} else {
		// 回退到 SMA 均值（兼容旧配置）
		adjustedZScore = spreadStats.ZScore
		if pas.tValue != 0 && spreadStats.Std > 1e-10 {
			adjustedZScore = (spreadStats.CurrentSpread - (spreadStats.Mean + pas.tValue)) / spreadStats.Std
		}
	}

	// Entry signals using dynamic thresholds
	// zscore > 0: spread 偏高，做空 spread（卖 symbol1，买 symbol2）
	// zscore < 0: spread 偏低，做多 spread（买 symbol1，卖 symbol2）
	if adjustedZScore >= pas.entryZScoreAsk {
		// Spread is too high - short spread (sell symbol1, buy symbol2)
		pas.generateSpreadSignals(md, "short", pas.orderSize)
		return
	} else if -adjustedZScore >= pas.entryZScoreBid {
		// Spread is too low - long spread (buy symbol1, sell symbol2)
		pas.generateSpreadSignals(md, "long", pas.orderSize)
		return
	}

	// Exit signals（使用调整后的Z-Score）
	if pas.leg1Position != 0 && math.Abs(adjustedZScore) <= pas.exitZScore {
		// Spread has reverted to mean - close positions
		pas.generateExitSignals(md)
	}
}

// updateOrderbookDepth 更新订单簿深度数据
// 用于多层挂单时获取各档价格
func (pas *PairwiseArbStrategy) updateOrderbookDepth(bidPrices, askPrices []float64, isLeg1 bool) {
	if isLeg1 {
		// 更新 Leg1 的订单簿深度
		for i := 0; i < len(pas.bidPrices1) && i < len(bidPrices); i++ {
			pas.bidPrices1[i] = bidPrices[i]
		}
		for i := 0; i < len(pas.askPrices1) && i < len(askPrices); i++ {
			pas.askPrices1[i] = askPrices[i]
		}
	} else {
		// 更新 Leg2 的订单簿深度
		for i := 0; i < len(pas.bidPrices2) && i < len(bidPrices); i++ {
			pas.bidPrices2[i] = bidPrices[i]
		}
		for i := 0; i < len(pas.askPrices2) && i < len(askPrices); i++ {
			pas.askPrices2[i] = askPrices[i]
		}
	}
}

// generateSpreadSignals generates signals to enter a spread trade
func (pas *PairwiseArbStrategy) generateSpreadSignals(md *mdpb.MarketDataUpdate, direction string, qty int64) {
	// Check position limits
	if math.Abs(float64(pas.leg1Position)) >= float64(pas.maxPositionSize) {
		return
	}

	// Get current statistics from SpreadAnalyzer
	spreadStats := pas.spreadAnalyzer.GetStats()

	var signal1Side, signal2Side OrderSide
	if direction == "long" {
		signal1Side = OrderSideBuy
		signal2Side = OrderSideSell
	} else {
		signal1Side = OrderSideSell
		signal2Side = OrderSideBuy
	}

	// C++: SendAggressiveOrder() 中对冲数量 = m_firstStrat->m_netpos_pass + m_secondStrat->m_netpos_agg + pending
	// C++ 原代码始终使用 1:1 对冲（同品种跨期套利，合约乘数相同、价格接近）
	// 不使用 SpreadAnalyzer 的动态 hedgeRatio（回归 beta），避免：
	//   1. 回归不稳定导致两腿数量漂移
	//   2. 整数取整累积误差
	//   3. 破坏市场中性（套利策略不应有方向性暴露）
	// 如需跨品种套利，可在配置中增加 hedge_ratio_mode: dynamic
	hedgeQty := qty

	// 计算leg1的订单价格（使用bid/ask和滑点）
	orderPrice1 := GetOrderPrice(signal1Side, pas.bid1, pas.ask1, pas.symbol1,
		pas.slippageTicks, pas.useAggressivePrice)

	// Generate signal for leg 1
	// 注意：不设置 OpenClose，Plugin 层会自动根据持仓判断
	signal1 := &TradingSignal{
		StrategyID: pas.ID,
		Symbol:     pas.symbol1,
		Side:       signal1Side,
		// OpenClose: 不设置，让 Plugin 自动判断
		Price:      orderPrice1, // 使用计算后的价格
		Quantity:   qty,
		Signal:     -spreadStats.ZScore, // Negative z-score means buy, positive means sell
		Confidence: math.Min(1.0, math.Abs(spreadStats.ZScore)/5.0),
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"type":        "entry",
			"leg":         1,
			"direction":   direction,
			"z_score":     spreadStats.ZScore,
			"spread":      spreadStats.CurrentSpread,
			"hedge_ratio": spreadStats.HedgeRatio,
		},
	}
	pas.AddSignal(signal1)

	// 计算leg2的订单价格
	orderPrice2 := GetOrderPrice(signal2Side, pas.bid2, pas.ask2, pas.symbol2,
		pas.slippageTicks, pas.useAggressivePrice)

	// Generate signal for leg 2
	// 注意：不设置 OpenClose，Plugin 层会自动根据持仓判断
	signal2 := &TradingSignal{
		StrategyID: pas.ID,
		Symbol:     pas.symbol2,
		Side:       signal2Side,
		// OpenClose: 不设置，让 Plugin 自动判断
		Price:      orderPrice2, // 使用计算后的价格
		Quantity:   hedgeQty,
		Signal:     spreadStats.ZScore, // Opposite direction
		Confidence: math.Min(1.0, math.Abs(spreadStats.ZScore)/5.0),
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"type":        "entry",
			"leg":         2,
			"direction":   direction,
			"z_score":     spreadStats.ZScore,
			"spread":      spreadStats.CurrentSpread,
			"hedge_ratio": spreadStats.HedgeRatio,
		},
	}
	pas.AddSignal(signal2)

	log.Printf("[PairwiseArbStrategy:%s] Entering %s spread: z=%.2f, leg1=%v %d, leg2=%v %d",
		pas.ID, direction, spreadStats.ZScore, signal1Side, qty, signal2Side, hedgeQty)

	// 注意：不在这里直接修改 leg1Position/leg2Position
	// 持仓应该从订单成交回报中计算（OnOrderUpdate）
}

// generateExitSignals generates signals to exit the spread trade
func (pas *PairwiseArbStrategy) generateExitSignals(md *mdpb.MarketDataUpdate) {
	if pas.leg1Position == 0 {
		return
	}

	// Get current z-score
	zScore := pas.spreadAnalyzer.GetZScore()

	// Close leg 1
	var signal1Side OrderSide
	qty1 := absInt64(pas.leg1Position)
	if pas.leg1Position > 0 {
		signal1Side = OrderSideSell
	} else {
		signal1Side = OrderSideBuy
	}

	// 计算平仓价格
	exitPrice1 := GetOrderPrice(signal1Side, pas.bid1, pas.ask1, pas.symbol1,
		pas.slippageTicks, pas.useAggressivePrice)

	// 注意：不设置 OpenClose，Plugin 层会自动判断
	// 退出信号时，Plugin 会根据持仓自动设置为 CLOSE
	signal1 := &TradingSignal{
		StrategyID: pas.ID,
		Symbol:     pas.symbol1,
		Side:       signal1Side,
		// OpenClose: 不设置，让 Plugin 自动判断（会是 CLOSE）
		Price:      exitPrice1, // 使用计算后的价格
		Quantity:   qty1,
		Signal:     0,
		Confidence: 0.9,
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"type":    "exit",
			"leg":     1,
			"z_score": zScore,
		},
	}
	pas.AddSignal(signal1)

	// Close leg 2
	var signal2Side OrderSide
	qty2 := absInt64(pas.leg2Position)
	if pas.leg2Position > 0 {
		signal2Side = OrderSideSell
	} else {
		signal2Side = OrderSideBuy
	}

	exitPrice2 := GetOrderPrice(signal2Side, pas.bid2, pas.ask2, pas.symbol2,
		pas.slippageTicks, pas.useAggressivePrice)

	// 注意：不设置 OpenClose，Plugin 层会自动判断
	signal2 := &TradingSignal{
		StrategyID: pas.ID,
		Symbol:     pas.symbol2,
		Side:       signal2Side,
		// OpenClose: 不设置，让 Plugin 自动判断（会是 CLOSE）
		Price:      exitPrice2, // 使用计算后的价格
		Quantity:   qty2,
		Signal:     0,
		Confidence: 0.9,
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"type":    "exit",
			"leg":     2,
			"z_score": zScore,
		},
	}
	pas.AddSignal(signal2)

	log.Printf("[PairwiseArbStrategy:%s] Exiting spread: z=%.2f, leg1=%v %d, leg2=%v %d",
		pas.ID, zScore, signal1Side, qty1, signal2Side, qty2)

	// 注意：不在这里直接重置持仓
	// 持仓应该从订单成交回报中计算（OnOrderUpdate）
}

// generateMultiLevelSignals 生成多层挂单信号
// C++: 对应 MAX_QUOTE_LEVEL 多层挂单逻辑，在多个价位同时挂单
//
// 多层挂单的好处：
// 1. 提高成交概率：如果一档没有成交，二档、三档仍有机会
// 2. 降低滑点：被动挂单而非主动吃单
// 3. 分散风险：不同价位的仓位分配
//
// tValue 调整同 generateSignals
func (pas *PairwiseArbStrategy) generateMultiLevelSignals(md *mdpb.MarketDataUpdate) {
	// Get current statistics from SpreadAnalyzer
	spreadStats := pas.spreadAnalyzer.GetStats()

	if spreadStats.Std < 1e-10 {
		return
	}

	// 应用外部 tValue 调整到均值
	adjustedMean := spreadStats.Mean + pas.tValue

	// 计算调整后的 Z-Score（用于出场判断）
	adjustedZScore := spreadStats.ZScore
	if pas.tValue != 0 && spreadStats.Std > 1e-10 {
		adjustedZScore = (spreadStats.CurrentSpread - adjustedMean) / spreadStats.Std
	}

	// Exit signals - 平仓信号不使用多层挂单
	if pas.leg1Position != 0 && math.Abs(adjustedZScore) <= pas.exitZScore {
		pas.generateExitSignals(md)
		return
	}

	// Check position limits
	if math.Abs(float64(pas.leg1Position)) >= float64(pas.maxPositionSize) {
		return
	}

	// 遍历每一层，生成挂单信号
	for level := 0; level < pas.maxQuoteLevel; level++ {
		// 确保有足够的订单簿深度数据
		if level >= len(pas.bidPrices1) || level >= len(pas.askPrices1) {
			break
		}

		// 获取该层的挂单价格
		bidPrice := pas.bidPrices1[level]
		askPrice := pas.askPrices1[level]

		// 跳过无效价格
		if bidPrice <= 0 || askPrice <= 0 {
			continue
		}

		// 检查该层是否已有挂单（避免重复挂单）
		if pas.leg1OrderMap.HasOrderAtPrice(bidPrice, OrderSideBuy) {
			continue
		}
		if pas.leg1OrderMap.HasOrderAtPrice(askPrice, OrderSideSell) {
			continue
		}

		// 获取该层的下单量
		qty := pas.orderSize
		if level < len(pas.quoteLevelSizes) {
			qty = pas.quoteLevelSizes[level]
		}

		// 计算该层的价差
		// 做多 spread：用 Leg1 买价 - Leg2 卖价
		// 做空 spread：用 Leg1 卖价 - Leg2 买价
		longSpread := bidPrice - pas.ask2
		shortSpread := askPrice - pas.bid2

		// 计算该层的等效 Z-Score（使用调整后的均值）
		longZScore := 0.0
		shortZScore := 0.0
		if spreadStats.Std > 1e-10 {
			longZScore = (longSpread - adjustedMean) / spreadStats.Std
			shortZScore = (shortSpread - adjustedMean) / spreadStats.Std
		}

		// 优化挂单价格（检测隐性订单簿）
		optimizedBidPrice := pas.optimizeOrderPrice(OrderSideBuy, level, bidPrice, pas.tickSize1)
		optimizedAskPrice := pas.optimizeOrderPrice(OrderSideSell, level, askPrice, pas.tickSize1)

		// 做多信号：-longZScore >= entryZScoreBid
		// 注意：longZScore 通常为负（因为买价 < 卖价），所以取负值比较
		if -longZScore >= pas.entryZScoreBid {
			pas.generateLevelSignal("long", level, optimizedBidPrice, qty, spreadStats)
		}

		// 做空信号：shortZScore >= entryZScoreAsk
		if shortZScore >= pas.entryZScoreAsk {
			pas.generateLevelSignal("short", level, optimizedAskPrice, qty, spreadStats)
		}
	}
}

// generateLevelSignal 生成指定层级的挂单信号
// C++: 对应每层独立的信号生成逻辑
func (pas *PairwiseArbStrategy) generateLevelSignal(direction string, level int, price float64, qty int64, stats spread.SpreadStats) {
	var signal1Side, signal2Side OrderSide
	if direction == "long" {
		signal1Side = OrderSideBuy
		signal2Side = OrderSideSell
	} else {
		signal1Side = OrderSideSell
		signal2Side = OrderSideBuy
	}

	// C++: 同品种跨期套利固定 1:1 对冲，不使用动态 hedgeRatio
	// 参考: tbsrc/Strategies/PairwiseArbStrategy.cpp SendAggressiveOrder()
	hedgeQty := qty

	// Generate signal for leg 1 (被动单)
	signal1 := &TradingSignal{
		StrategyID:  pas.ID,
		Symbol:      pas.symbol1,
		Side:        signal1Side,
		Price:       price,
		Quantity:    qty,
		OrderType:   OrderTypeLimit,
		TimeInForce: TimeInForceGTC,
		Signal:      -stats.ZScore,
		Confidence:  math.Min(1.0, math.Abs(stats.ZScore)/5.0),
		Timestamp:   time.Now(),
		Category:    SignalCategoryPassive, // 被动单
		QuoteLevel:  level,
		Metadata: map[string]interface{}{
			"type":        "entry",
			"leg":         1,
			"direction":   direction,
			"level":       level,
			"z_score":     stats.ZScore,
			"spread":      stats.CurrentSpread,
			"hedge_ratio": stats.HedgeRatio,
		},
	}
	pas.AddSignal(signal1)

	// 计算 Leg2 的挂单价格
	var price2 float64
	if direction == "long" {
		// 做多 spread：Leg2 卖出，使用 ask 价格
		if level < len(pas.askPrices2) && pas.askPrices2[level] > 0 {
			price2 = pas.optimizeOrderPrice(signal2Side, level, pas.askPrices2[level], pas.tickSize2)
		} else {
			price2 = pas.ask2
		}
	} else {
		// 做空 spread：Leg2 买入，使用 bid 价格
		if level < len(pas.bidPrices2) && pas.bidPrices2[level] > 0 {
			price2 = pas.optimizeOrderPrice(signal2Side, level, pas.bidPrices2[level], pas.tickSize2)
		} else {
			price2 = pas.bid2
		}
	}

	// Generate signal for leg 2 (被动单)
	signal2 := &TradingSignal{
		StrategyID:  pas.ID,
		Symbol:      pas.symbol2,
		Side:        signal2Side,
		Price:       price2,
		Quantity:    hedgeQty,
		OrderType:   OrderTypeLimit,
		TimeInForce: TimeInForceGTC,
		Signal:      stats.ZScore,
		Confidence:  math.Min(1.0, math.Abs(stats.ZScore)/5.0),
		Timestamp:   time.Now(),
		Category:    SignalCategoryPassive, // 被动单
		QuoteLevel:  level,
		Metadata: map[string]interface{}{
			"type":        "entry",
			"leg":         2,
			"direction":   direction,
			"level":       level,
			"z_score":     stats.ZScore,
			"spread":      stats.CurrentSpread,
			"hedge_ratio": stats.HedgeRatio,
		},
	}
	pas.AddSignal(signal2)

	log.Printf("[PairwiseArbStrategy:%s] Level %d %s spread: z=%.2f, leg1=%v@%.2f %d, leg2=%v@%.2f %d",
		pas.ID, level, direction, stats.ZScore, signal1Side, price, qty, signal2Side, price2, hedgeQty)
}

// optimizeOrderPrice 优化挂单价格
// C++: 对应 GetBidPrice_first() 等方法中的隐性订单簿检测
//
// 当检测到价格跳跃（隐性订单簿）时，可以适当优化挂单价格以提高成交概率
// 例如：如果二档和一档之间有较大的价格跳跃，可能存在隐性流动性
//
// 参数:
//   - side: 买卖方向
//   - level: 当前挂单层级
//   - basePrice: 基础挂单价格
//   - tickSize: 最小变动单位
//
// 返回优化后的价格
func (pas *PairwiseArbStrategy) optimizeOrderPrice(side OrderSide, level int, basePrice float64, tickSize float64) float64 {
	// 一档不优化，或者未启用价格优化
	if level == 0 || !pas.enablePriceOptimize {
		return basePrice
	}

	// 获取前一档价格
	var prevPrice float64
	if side == OrderSideBuy {
		if level-1 < len(pas.bidPrices1) {
			prevPrice = pas.bidPrices1[level-1]
		}
	} else {
		if level-1 < len(pas.askPrices1) {
			prevPrice = pas.askPrices1[level-1]
		}
	}

	if prevPrice <= 0 {
		return basePrice
	}

	// 计算价格跳跃（tick 数）
	var gap float64
	if side == OrderSideBuy {
		// 买单：前一档价格 > 当前档价格，gap = (prevPrice - basePrice) / tickSize
		gap = (prevPrice - basePrice) / tickSize
	} else {
		// 卖单：前一档价格 < 当前档价格，gap = (basePrice - prevPrice) / tickSize
		gap = (basePrice - prevPrice) / tickSize
	}

	// 检测是否存在价格跳跃
	if gap > float64(pas.priceOptimizeGap) {
		// 存在隐性订单簿，优化挂单价格
		var optimizedPrice float64
		if side == OrderSideBuy {
			// 买单：尝试提高价格一个 tick（更激进）
			optimizedPrice = basePrice + tickSize
		} else {
			// 卖单：尝试降低价格一个 tick（更激进）
			optimizedPrice = basePrice - tickSize
		}

		// 验证优化后的价格是否合理
		// 买单优化价格不能超过前一档价格
		// 卖单优化价格不能低于前一档价格
		if side == OrderSideBuy && optimizedPrice < prevPrice {
			log.Printf("[PairwiseArbStrategy:%s] Price optimize: level=%d, gap=%.0f ticks, %.2f -> %.2f",
				pas.ID, level, gap, basePrice, optimizedPrice)
			return optimizedPrice
		} else if side == OrderSideSell && optimizedPrice > prevPrice {
			log.Printf("[PairwiseArbStrategy:%s] Price optimize: level=%d, gap=%.0f ticks, %.2f -> %.2f",
				pas.ID, level, gap, basePrice, optimizedPrice)
			return optimizedPrice
		}
	}

	return basePrice
}

// setDynamicThresholds 根据持仓动态调整入场阈值
// 与 C++ SetThresholds() 完全一致
// 参考: docs/cpp_reference/SetThresholds.cpp
//
// C++ 代码:
//   auto long_place_diff_thold = m_thold_first->LONG_PLACE - m_thold_first->BEGIN_PLACE;
//   auto short_place_diff_thold = m_thold_first->BEGIN_PLACE - m_thold_first->SHORT_PLACE;
//
//   多头持仓 (netpos > 0):
//     tholdBidPlace = BEGIN_PLACE + long_place_diff_thold * netpos / maxPos
//     tholdAskPlace = BEGIN_PLACE - short_place_diff_thold * netpos / maxPos
//
//   空头持仓 (netpos < 0):
//     tholdBidPlace = BEGIN_PLACE + short_place_diff_thold * netpos / maxPos
//     tholdAskPlace = BEGIN_PLACE - long_place_diff_thold * netpos / maxPos
func (pas *PairwiseArbStrategy) setDynamicThresholds() {
	if !pas.useDynamicThreshold || pas.maxPositionSize == 0 {
		// 未启用动态阈值，使用静态 entryZScore 和 exitZScore
		pas.entryZScoreBid = pas.entryZScore
		pas.entryZScoreAsk = pas.entryZScore
		pas.exitZScoreBid = pas.exitZScore
		pas.exitZScoreAsk = pas.exitZScore
		return
	}

	// C++: long_place_diff_thold = LONG_PLACE - BEGIN_PLACE
	longPlaceDiff := pas.longZScore - pas.beginZScore
	// C++: short_place_diff_thold = BEGIN_PLACE - SHORT_PLACE
	shortPlaceDiff := pas.beginZScore - pas.shortZScore
	// C++: long_remove_diff_thold = LONG_REMOVE - BEGIN_REMOVE
	longRemoveDiff := pas.longExitZScore - pas.exitZScore
	// C++: short_remove_diff_thold = BEGIN_REMOVE - SHORT_REMOVE
	shortRemoveDiff := pas.exitZScore - pas.shortExitZScore

	// C++: 使用 m_firstStrat->m_netpos_pass (被动成交净持仓)
	// 不是用 m_netpos (总净持仓)
	var netPosPass int32 = 0
	if pas.firstStrat != nil {
		netPosPass = pas.firstStrat.NetPosPass
	}

	// 计算持仓比例：m_netpos_pass / m_tholdMaxPos
	posRatio := float64(netPosPass) / float64(pas.maxPositionSize)

	if netPosPass == 0 {
		// C++: 无持仓时使用初始阈值
		pas.entryZScoreBid = pas.beginZScore
		pas.entryZScoreAsk = pas.beginZScore
		pas.exitZScoreBid = pas.exitZScore
		pas.exitZScoreAsk = pas.exitZScore
	} else if netPosPass > 0 {
		// C++: 多头持仓
		// tholdBidPlace = BEGIN_PLACE + long_place_diff_thold * netpos / maxPos
		pas.entryZScoreBid = pas.beginZScore + longPlaceDiff*posRatio
		// tholdAskPlace = BEGIN_PLACE - short_place_diff_thold * netpos / maxPos
		pas.entryZScoreAsk = pas.beginZScore - shortPlaceDiff*posRatio
		// tholdBidRemove = BEGIN_REMOVE + long_remove_diff_thold * netpos / maxPos
		pas.exitZScoreBid = pas.exitZScore + longRemoveDiff*posRatio
		// tholdAskRemove = BEGIN_REMOVE - short_remove_diff_thold * netpos / maxPos
		pas.exitZScoreAsk = pas.exitZScore - shortRemoveDiff*posRatio
	} else {
		// C++: 空头持仓 (netpos < 0)
		// tholdBidPlace = BEGIN_PLACE + short_place_diff_thold * netpos / maxPos
		pas.entryZScoreBid = pas.beginZScore + shortPlaceDiff*posRatio
		// tholdAskPlace = BEGIN_PLACE - long_place_diff_thold * netpos / maxPos
		pas.entryZScoreAsk = pas.beginZScore - longPlaceDiff*posRatio
		// tholdBidRemove = BEGIN_REMOVE + short_remove_diff_thold * netpos / maxPos
		pas.exitZScoreBid = pas.exitZScore + shortRemoveDiff*posRatio
		// tholdAskRemove = BEGIN_REMOVE - long_remove_diff_thold * netpos / maxPos
		pas.exitZScoreAsk = pas.exitZScore - longRemoveDiff*posRatio
	}
}

// getAvgSpreadRatio 获取调整后的价差均值
// C++: avgSpreadRatio = avgSpreadRatio_ori + tValue
// tValue 允许外部信号调整价差均值，使策略更容易入场或出场
func (pas *PairwiseArbStrategy) getAvgSpreadRatio() float64 {
	return pas.spreadAnalyzer.GetStats().Mean + pas.tValue
}

// cancelOutOfRangeOrders 检查并撤销偏离均值的挂单
// C++: PairwiseArbStrategy.cpp:205-228
//
// 撤单逻辑：
// - 买单撤单: LongSpreadRatio1 > avgSpreadRatio - tholdBidRemove
//   即: 买价差 > 均值 - 撤单阈值（价差太高，撤销买单）
// - 卖单撤单: ShortSpreadRatio1 < avgSpreadRatio + tholdAskRemove
//   即: 卖价差 < 均值 + 撤单阈值（价差太低，撤销卖单）
func (pas *PairwiseArbStrategy) cancelOutOfRangeOrders() {
	// 检查撤单阈值是否有效
	if pas.exitZScoreBid == 0 && pas.exitZScoreAsk == 0 {
		return
	}

	// 检查价格数据有效性
	if pas.bid2 <= 0 || pas.ask2 <= 0 {
		return
	}

	// 获取当前价差均值（含 tValue 调整）
	avgSpreadRatio := pas.getAvgSpreadRatio()

	// 遍历 Leg1 的买单，检查是否需要撤单
	// C++: for (PriceMapIter iter = m_bidMap1.begin(); iter != m_bidMap1.end(); iter++)
	if pas.leg1OrderMap != nil {
		for _, order := range pas.leg1OrderMap.GetAllBidOrders() {
			// C++: LongSpreadRatio1 = iter->second->m_price - m_secondinstru->bidPx[0]
			longSpreadRatio := order.Price - pas.bid2

			// C++: if (LongSpreadRatio1 > avgSpreadRatio - m_firstStrat->m_tholdBidRemove)
			// 买价差太高，撤单
			if longSpreadRatio > avgSpreadRatio-pas.exitZScoreBid {
				// 只撤销已确认的订单
				// C++: m_status == NEW_CONFIRM || m_status == MODIFY_CONFIRM || m_status == MODIFY_REJECT
				log.Printf("[PairwiseArb:%s] Cancel bid order (spread too high): orderID=%s, price=%.2f, spreadRatio=%.4f > %.4f (avg=%.4f - remove=%.4f)",
					pas.ID, order.OrderID, order.Price, longSpreadRatio, avgSpreadRatio-pas.exitZScoreBid, avgSpreadRatio, pas.exitZScoreBid)

				// 发送撤单请求
				pas.sendCancelOrder(order.OrderID, pas.symbol1)
			}
		}
	}

	// 遍历 Leg1 的卖单，检查是否需要撤单
	// C++: for (PriceMapIter iter = m_askMap1.begin(); iter != m_askMap1.end(); iter++)
	if pas.leg1OrderMap != nil {
		for _, order := range pas.leg1OrderMap.GetAllAskOrders() {
			// C++: ShortSpreadRatio1 = iter->second->m_price - m_secondinstru->askPx[0]
			shortSpreadRatio := order.Price - pas.ask2

			// C++: if (ShortSpreadRatio1 < avgSpreadRatio + m_firstStrat->m_tholdAskRemove)
			// 卖价差太低，撤单
			if shortSpreadRatio < avgSpreadRatio+pas.exitZScoreAsk {
				log.Printf("[PairwiseArb:%s] Cancel ask order (spread too low): orderID=%s, price=%.2f, spreadRatio=%.4f < %.4f (avg=%.4f + remove=%.4f)",
					pas.ID, order.OrderID, order.Price, shortSpreadRatio, avgSpreadRatio+pas.exitZScoreAsk, avgSpreadRatio, pas.exitZScoreAsk)

				// 发送撤单请求
				pas.sendCancelOrder(order.OrderID, pas.symbol1)
			}
		}
	}
}

// sendCancelOrder 发送撤单请求
// 注意：撤单信号通过 Signal 的 Metadata 标记为撤单
func (pas *PairwiseArbStrategy) sendCancelOrder(orderID, symbol string) {
	// 通过 Signal 字段标记为撤单信号
	cancelSignal := &TradingSignal{
		StrategyID: pas.ID,
		Symbol:     symbol,
		Side:       OrderSideBuy, // 占位符，实际会根据 metadata 中的 action=cancel 处理
		Signal:     0,            // 撤单信号
		Timestamp:  time.Now(),
		Metadata: map[string]interface{}{
			"action":          "cancel",
			"cancel_order_id": orderID,
		},
	}

	// 通过策略的 signals channel 发送（如果有订阅者）
	// 注意：这个信号需要被 Trader 或 PluginProcessor 拦截并处理撤单
	log.Printf("[PairwiseArb:%s] Cancel signal for order: %s (need handler to process)", pas.ID, orderID)

	// 目前先记录日志，后续需要在 PluginProcessor 中添加撤单处理
	// TODO: 实现撤单处理逻辑
	_ = cancelSignal
}

// === 价格计算方法（C++: GetBidPrice_first 等）===

// GetBidPrice_first 获取第一条腿买单挂单价格
// C++: PairwiseArbStrategy::GetBidPrice_first()
// 实现隐性订单簿检测逻辑
//
// C++ 原代码 (PairwiseArbStrategy.cpp:802-820):
//   price = m_firstStrat->m_instru->bidPx[level];
//   if (m_configParams->m_bUseInvisibleBook && level != 0 && price < bidPx[level-1] - tickSize) {
//       double bidInv = bidPx[level] - secondStrat->bidPx[0] + tickSize;
//       if (bidInv <= avgSpreadRatio - BEGIN_PLACE) {
//           PriceMapIter iter = m_bidMap1.find(price);
//           if (iter != m_bidMap1.end() && iter->second->m_quantAhead > m_firstinstru->m_lotSize) {
//               price = bidPx[level] + tickSize;
//           }
//       }
//   }
func (pas *PairwiseArbStrategy) GetBidPrice_first(level int) (price float64, ordType OrderHitType) {
	if level >= len(pas.bidPrices1) || pas.bidPrices1[level] <= 0 {
		return 0, OrderHitTypeStandard
	}

	price = pas.bidPrices1[level]
	ordType = OrderHitTypeStandard

	// 隐性订单簿检测
	// C++: if (m_configParams->m_bUseInvisibleBook && level != 0 && price < bidPx[level-1] - tickSize)
	if pas.enablePriceOptimize && level > 0 {
		prevPrice := pas.bidPrices1[level-1]
		tickSize := pas.tickSize1
		if price < prevPrice-tickSize {
			// 检测到价格跳跃，计算隐性价差
			// C++: bidInv = bidPx[level] - secondStrat->bidPx[0] + tickSize
			bidInv := price - pas.bid2 + tickSize
			spreadMean := pas.getAvgSpreadRatio()

			// C++: if (bidInv <= avgSpreadRatio - BEGIN_PLACE)
			if bidInv <= spreadMean-pas.tholdFirst.BeginPlace {
				// 检查该价位是否已有订单，以及前方排队量是否足够
				// C++: if (iter != m_bidMap1.end() && iter->second->m_quantAhead > m_firstinstru->m_lotSize)
				orderStats := pas.firstStrat.GetOrderByPrice(price, TransactionTypeBuy)
				lotSize := float64(pas.firstStrat.Instru.LotSize)
				if lotSize == 0 {
					lotSize = 1 // 默认为1
				}
				if orderStats != nil && orderStats.QuantAhead > lotSize {
					// C++: price = bidPx[level] + tickSize
					price = price + tickSize
					log.Printf("[PairwiseArb:%s] GetBidPrice_first: invisible book detected at level=%d, quantAhead=%.0f, optimize %.2f -> %.2f",
						pas.ID, level, orderStats.QuantAhead, pas.bidPrices1[level], price)
				}
			}
		}
	}

	return price, ordType
}

// GetAskPrice_first 获取第一条腿卖单挂单价格
// C++: PairwiseArbStrategy::GetAskPrice_first()
//
// C++ 原代码 (PairwiseArbStrategy.cpp:822-840):
//   price = m_firstStrat->m_instru->askPx[level];
//   if (m_configParams->m_bUseInvisibleBook && level != 0 && price > askPx[level-1] + tickSize) {
//       double askInv = askPx[level] - secondStrat->askPx[0] - tickSize;
//       if (askInv >= avgSpreadRatio + BEGIN_PLACE) {
//           PriceMapIter iter = m_askMap1.find(price);
//           if (iter != m_askMap1.end() && iter->second->m_quantAhead > m_firstinstru->m_lotSize) {
//               price = askPx[level] - tickSize;
//           }
//       }
//   }
func (pas *PairwiseArbStrategy) GetAskPrice_first(level int) (price float64, ordType OrderHitType) {
	if level >= len(pas.askPrices1) || pas.askPrices1[level] <= 0 {
		return 0, OrderHitTypeStandard
	}

	price = pas.askPrices1[level]
	ordType = OrderHitTypeStandard

	// 隐性订单簿检测
	// C++: if (m_configParams->m_bUseInvisibleBook && level != 0 && price > askPx[level-1] + tickSize)
	if pas.enablePriceOptimize && level > 0 {
		prevPrice := pas.askPrices1[level-1]
		tickSize := pas.tickSize1
		if price > prevPrice+tickSize {
			// 检测到价格跳跃，计算隐性价差
			// C++: askInv = askPx[level] - secondStrat->askPx[0] - tickSize
			askInv := price - pas.ask2 - tickSize
			spreadMean := pas.getAvgSpreadRatio()

			// C++: if (askInv >= avgSpreadRatio + BEGIN_PLACE)
			if askInv >= spreadMean+pas.tholdFirst.BeginPlace {
				// 检查该价位是否已有订单，以及前方排队量是否足够
				// C++: if (iter != m_askMap1.end() && iter->second->m_quantAhead > m_firstinstru->m_lotSize)
				orderStats := pas.firstStrat.GetOrderByPrice(price, TransactionTypeSell)
				lotSize := float64(pas.firstStrat.Instru.LotSize)
				if lotSize == 0 {
					lotSize = 1 // 默认为1
				}
				if orderStats != nil && orderStats.QuantAhead > lotSize {
					// C++: price = askPx[level] - tickSize
					price = price - tickSize
					log.Printf("[PairwiseArb:%s] GetAskPrice_first: invisible book detected at level=%d, quantAhead=%.0f, optimize %.2f -> %.2f",
						pas.ID, level, orderStats.QuantAhead, pas.askPrices1[level], price)
				}
			}
		}
	}

	return price, ordType
}

// GetBidPrice_second 获取第二条腿买单挂单价格
// C++: PairwiseArbStrategy::GetBidPrice_second()
func (pas *PairwiseArbStrategy) GetBidPrice_second(level int) (price float64, ordType OrderHitType) {
	if level >= len(pas.bidPrices2) || pas.bidPrices2[level] <= 0 {
		return 0, OrderHitTypeStandard
	}

	price = pas.bidPrices2[level]
	ordType = OrderHitTypeStandard

	// 隐性订单簿检测（与 first 类似，但参照 firstStrat 的价格）
	if pas.enablePriceOptimize && level > 0 {
		prevPrice := pas.bidPrices2[level-1]
		tickSize := pas.tickSize2
		if price < prevPrice-tickSize {
			// 检测到价格跳跃
			bidInv := pas.bid1 - price + tickSize
			spreadMean := pas.getAvgSpreadRatio()

			if bidInv <= spreadMean-pas.tholdFirst.BeginPlace {
				if !pas.secondStrat.HasOrderAtPrice(price, TransactionTypeBuy) {
					price = price + tickSize
					log.Printf("[PairwiseArb:%s] GetBidPrice_second: invisible book detected at level=%d, optimize %.2f -> %.2f",
						pas.ID, level, pas.bidPrices2[level], price)
				}
			}
		}
	}

	return price, ordType
}

// GetAskPrice_second 获取第二条腿卖单挂单价格
// C++: PairwiseArbStrategy::GetAskPrice_second()
func (pas *PairwiseArbStrategy) GetAskPrice_second(level int) (price float64, ordType OrderHitType) {
	if level >= len(pas.askPrices2) || pas.askPrices2[level] <= 0 {
		return 0, OrderHitTypeStandard
	}

	price = pas.askPrices2[level]
	ordType = OrderHitTypeStandard

	// 隐性订单簿检测
	if pas.enablePriceOptimize && level > 0 {
		prevPrice := pas.askPrices2[level-1]
		tickSize := pas.tickSize2
		if price > prevPrice+tickSize {
			// 检测到价格跳跃
			askInv := pas.ask1 - price - tickSize
			spreadMean := pas.getAvgSpreadRatio()

			if askInv >= spreadMean+pas.tholdFirst.BeginPlace {
				if !pas.secondStrat.HasOrderAtPrice(price, TransactionTypeSell) {
					price = price - tickSize
					log.Printf("[PairwiseArb:%s] GetAskPrice_second: invisible book detected at level=%d, optimize %.2f -> %.2f",
						pas.ID, level, pas.askPrices2[level], price)
				}
			}
		}
	}

	return price, ordType
}

// calculateExposure 计算当前敞口
// C++ 对应: exposure = m_firstStrat->m_netpos_pass + m_secondStrat->m_netpos_agg
// 敞口 = leg1Position + leg2Position（理想情况下应为 0）
// 参考: docs/cpp_reference/SendAggressiveOrder.cpp
func (pas *PairwiseArbStrategy) calculateExposure() int64 {
	return pas.leg1Position + pas.leg2Position
}

// sendAggressiveOrder 主动追单机制
// 与 C++ SendAggressiveOrder() 完全一致
// 参考: tbsrc/Strategies/PairwiseArbStrategy.cpp:701-800
//
// C++ 逻辑:
//   1. exposure = m_netpos_pass + m_netpos_agg + pending_netpos_agg (敞口计算)
//   2. CRITICAL: sellAggOrder/buyAggOrder <= SUPPORTING_ORDERS (限制追单数量)
//   3. if (last_agg_side != side || now - last_agg_time > 500ms) 发送新追单
//   4. 价格递进:
//      - m_agg_repeat < 3: bid/ask ± tickSize * m_agg_repeat
//      - m_agg_repeat >= 3: bid/ask ± tickSize * SLOP
//   5. m_agg_repeat > 3: HandleSquareoff() (触发策略停止)
func (pas *PairwiseArbStrategy) sendAggressiveOrder() {
	if !pas.aggressiveEnabled {
		return
	}

	// 1. 计算敞口（包括待成交订单）
	// C++: exposure = m_firstStrat->m_netpos_pass + m_secondStrat->m_netpos_agg + pending_netpos_agg2
	exposure := pas.calculateExposure()
	pendingNetpos := pas.calculatePendingNetpos() // 新增：计算待成交订单净头寸
	totalExposure := exposure + pendingNetpos

	if totalExposure == 0 {
		// 无敞口，重置追单状态和计数
		pas.aggRepeat = 1
		pas.aggDirection = 0
		// 使用 secondStrat 的追单计数（C++: sellAggOrder, buyAggOrder）
		pas.secondStrat.SellAggOrder = 0
		pas.secondStrat.BuyAggOrder = 0
		return
	}

	// 2. 确定追单方向
	// exposure > 0: 多头敞口，需要卖出 leg2 来平衡
	// exposure < 0: 空头敞口，需要买入 leg2 来平衡
	var newDirection int
	var targetSide OrderSide
	var targetSymbol string
	var targetQty int64
	var bid, ask float64

	if totalExposure > 0 {
		// 多头敞口：需要卖出
		newDirection = -1
		targetSide = OrderSideSell
		targetSymbol = pas.symbol2
		targetQty = totalExposure
		bid = pas.bid2
		ask = pas.ask2
	} else {
		// 空头敞口：需要买入
		newDirection = 1
		targetSide = OrderSideBuy
		targetSymbol = pas.symbol2
		targetQty = -totalExposure
		bid = pas.bid2
		ask = pas.ask2
	}

	// 3. CRITICAL: 检查 SUPPORTING_ORDERS 限制
	// C++: if (exposure > 0 && sellAggOrder <= SUPPORTING_ORDERS)
	// C++: if (exposure < 0 && buyAggOrder <= SUPPORTING_ORDERS)
	supportingOrders := int(pas.tholdFirst.SupportingOrders)
	sellAggOrder := int(pas.secondStrat.SellAggOrder)
	buyAggOrder := int(pas.secondStrat.BuyAggOrder)
	if supportingOrders > 0 {
		if targetSide == OrderSideSell && sellAggOrder > supportingOrders {
			log.Printf("[PairwiseArb:%s] ⛔ Sell aggressive order limit reached: %d > %d",
				pas.ID, sellAggOrder, supportingOrders)
			return
		}
		if targetSide == OrderSideBuy && buyAggOrder > supportingOrders {
			log.Printf("[PairwiseArb:%s] ⛔ Buy aggressive order limit reached: %d > %d",
				pas.ID, buyAggOrder, supportingOrders)
			return
		}
	}

	// 4. 方向变化检查：如果方向变化，重置计数
	directionChanged := pas.aggDirection != newDirection
	if directionChanged {
		pas.aggRepeat = 1
		pas.aggDirection = newDirection
		// 方向变化时也重置对应方向的追单计数（使用 secondStrat）
		if newDirection == -1 {
			pas.secondStrat.SellAggOrder = 0
		} else {
			pas.secondStrat.BuyAggOrder = 0
		}
	}

	// 5. 时间间隔检查
	// C++: if (last_agg_side != side || now - last_agg_time > 500ms)
	// 方向变化时跳过间隔检查
	if !directionChanged && time.Since(pas.aggLastTime) < pas.aggressiveInterval {
		// 同方向追单，间隔不足
		return
	}

	// 6. 检查追单次数限制
	if pas.aggRepeat > pas.aggressiveMaxRetry {
		// 超过最大追单次数
		pas.aggFailCount++
		log.Printf("[PairwiseArb:%s] ⚠️  Aggressive order exceeded max retry (%d), fail count: %d",
			pas.ID, pas.aggressiveMaxRetry, pas.aggFailCount)

		if pas.aggFailCount >= pas.aggressiveFailThreshold {
			log.Printf("[PairwiseArb:%s] 🚨 Aggressive order fail threshold reached, exiting strategy!",
				pas.ID)
			// 触发策略退出
			pas.ControlState.RunState = StrategyRunStateExiting
		}
		return
	}

	// 7. 计算追单价格
	// C++: agg_price = m_agg_repeat < 3
	//        ? bidPx[0] - tickSize * m_agg_repeat
	//        : bidPx[0] - tickSize * SLOP
	tickSize := GetTickSize(targetSymbol)
	var priceAdjust float64
	if pas.aggRepeat <= 3 {
		// C++: m_agg_repeat < 3 -> tickSize * m_agg_repeat
		priceAdjust = float64(pas.aggRepeat) * tickSize
	} else {
		// C++: m_agg_repeat >= 3 -> tickSize * SLOP
		priceAdjust = float64(pas.aggressiveSlopTicks) * tickSize
	}

	var orderPrice float64
	if targetSide == OrderSideBuy {
		// C++: askPx[0] + tickSize * m_agg_repeat
		orderPrice = ask + priceAdjust
	} else {
		// C++: bidPx[0] - tickSize * m_agg_repeat
		orderPrice = bid - priceAdjust
	}
	orderPrice = RoundToTickSize(orderPrice, tickSize)

	// 8. 发送追单信号
	// C++: SendAskOrder2/SendBidOrder2 with CROSS type
	signal := &TradingSignal{
		StrategyID: pas.ID,
		Symbol:     targetSymbol,
		Side:       targetSide,
		Price:      orderPrice,
		Quantity:   targetQty,
		Signal:     0, // 追单信号
		Confidence: 0.8,
		Timestamp:  time.Now(),
		Category:   SignalCategoryAggressive, // 🔑 关键：标记为主动单（C++: CROSS）
		Metadata: map[string]interface{}{
			"type":           "aggressive",
			"exposure":       exposure,
			"pending_netpos": pendingNetpos,
			"total_exposure": totalExposure,
			"retry":          pas.aggRepeat,
			"price_adjust":   priceAdjust,
			"sell_agg_order": sellAggOrder,
			"buy_agg_order":  buyAggOrder,
		},
	}
	pas.AddSignal(signal)

	log.Printf("[PairwiseArb:%s] 🏃 Aggressive order #%d: %v %s %d @ %.2f (exposure=%d, pending=%d, sellAgg=%d, buyAgg=%d)",
		pas.ID, pas.aggRepeat, targetSide, targetSymbol, targetQty, orderPrice, exposure, pendingNetpos, sellAggOrder, buyAggOrder)

	// 9. 更新追单状态
	// C++: sellAggOrder++ / buyAggOrder++（使用 secondStrat）
	if targetSide == OrderSideSell {
		pas.secondStrat.SellAggOrder++
	} else {
		pas.secondStrat.BuyAggOrder++
	}
	pas.aggLastTime = time.Now()
	pas.aggRepeat++
}

// calculatePendingNetpos 计算待成交订单的净头寸
// C++ 对应: CalcPendingNetposAgg()
// 参考: tbsrc/Strategies/PairwiseArbStrategy.cpp:688-699
//
// C++ 原代码:
//   for (auto &it : *m_ordMap2) {
//       auto &order = it.second;
//       if (order->m_ordType == CROSS || order->m_ordType == MATCH)
//           order->m_side == BUY ? netpos_agg_pending += order->m_openQty
//                                : netpos_agg_pending -= order->m_openQty;
//   }
func (pas *PairwiseArbStrategy) calculatePendingNetpos() int64 {
	var netposPending int64

	// 遍历 leg2 订单映射，计算待成交的 CROSS/MATCH 类型订单净头寸
	if pas.leg2OrderMap != nil {
		pas.leg2OrderMap.mu.RLock()
		for _, order := range pas.leg2OrderMap.orderByID {
			// C++: if (order->m_ordType == CROSS || order->m_ordType == MATCH)
			// 只统计主动单（追单）的待成交量
			if order.Category == SignalCategoryAggressive {
				// C++: m_openQty = 待成交数量
				pendingQty := order.Quantity - order.FilledQty
				if order.Side == OrderSideBuy {
					netposPending += pendingQty
				} else {
					netposPending -= pendingQty
				}
			}
		}
		pas.leg2OrderMap.mu.RUnlock()
	}

	return netposPending
}

// OnOrderUpdate handles order updates
// C++: PairwiseArbStrategy::ORSCallBack
func (pas *PairwiseArbStrategy) OnOrderUpdate(update *orspb.OrderUpdate) {
	// CRITICAL: 检查订单是否属于本策略
	// 修复 Bug: 防止策略接收到其他策略的订单回调
	if update.StrategyId != pas.ID {
		// 不是本策略的订单，直接忽略
		return
	}

	pas.mu.Lock()
	defer pas.mu.Unlock()

	log.Printf("[PairwiseArb:%s] OnOrderUpdate: OrderID=%s, Status=%v, Symbol=%s, Side=%v, FilledQty=%d",
		pas.ID, update.OrderId, update.Status, update.Symbol, update.Side, update.FilledQty)

	if !pas.running {
		log.Printf("[PairwiseArb:%s] Strategy not running, ignoring update", pas.ID)
		return
	}

	// Update base strategy position (for overall PNL tracking)
	pas.UpdatePosition(update)

	// 维护订单映射（多层挂单用）
	pas.updateOrderMaps(update)

	symbol := update.Symbol

	// C++: 区分 Leg1 (被动单) 和 Leg2 (主动单) 处理逻辑
	if symbol == pas.symbol1 {
		// Leg1 订单处理
		// C++: m_firstStrat->ORSCallBack(response);
		if update.Status == orspb.OrderStatus_FILLED && update.FilledQty > 0 {
			qty := int64(update.FilledQty)
			price := update.AvgPrice
			pas.updateLeg1Position(update.Side, qty, price)
			// C++: m_agg_repeat = 1; (Leg1 成交时重置追单计数)
			pas.aggRepeat = 1
			log.Printf("[PairwiseArb:%s] Leg1 trade, reset aggRepeat=1", pas.ID)
		}
	} else if symbol == pas.symbol2 {
		// Leg2 订单处理
		// C++: HandleAggOrder(response, order, m_secondStrat);
		pas.handleAggOrder(update)
		// C++: m_secondStrat->ORSCallBack(response);
		if update.Status == orspb.OrderStatus_FILLED && update.FilledQty > 0 {
			qty := int64(update.FilledQty)
			price := update.AvgPrice
			pas.updateLeg2Position(update.Side, qty, price)
			// C++: m_agg_repeat = 1; (Leg2 成交时也重置)
			pas.aggRepeat = 1
			log.Printf("[PairwiseArb:%s] Leg2 trade, reset aggRepeat=1", pas.ID)
		}
	}

	// 成交后检查敞口，如果敞口为0则完全重置追单状态
	if update.Status == orspb.OrderStatus_FILLED {
		exposure := pas.calculateExposure()
		if exposure == 0 {
			sellAggOrder := int(pas.secondStrat.SellAggOrder)
			buyAggOrder := int(pas.secondStrat.BuyAggOrder)
			if sellAggOrder > 0 || buyAggOrder > 0 {
				log.Printf("[PairwiseArb:%s] Exposure cleared, resetting all aggressive state (sellAgg=%d, buyAgg=%d)",
					pas.ID, sellAggOrder, buyAggOrder)
			}
			pas.aggDirection = 0
			pas.aggFailCount = 0
			pas.secondStrat.SellAggOrder = 0
			pas.secondStrat.BuyAggOrder = 0
		}
	}

	// C++: if (m_Active) SendAggressiveOrder();
	// 在订单回调后立即检查是否需要追单
	if pas.ControlState.RunState == StrategyRunStateActive {
		pas.sendAggressiveOrder()
	}
}

// handleAggOrder 处理主动追单订单的状态更新
// C++: PairwiseArbStrategy::HandleAggOrder
//
// 在以下情况下递减追单计数 (sellAggOrder/buyAggOrder):
// - ORS_REJECT, BUSINESS_REJECT, SIM_REJECT, RMS_REJECT
// - ORDERS_PER_DAY_LIMIT_REJECT, ORDER_ERROR
// - CANCEL_ORDER_CONFIRM (撤单确认)
// - TRADE_CONFIRM 且 openQty == filledQty (完全成交)
func (pas *PairwiseArbStrategy) handleAggOrder(update *orspb.OrderUpdate) {
	// 判断是否需要递减追单计数
	shouldDecrement := false

	switch update.Status {
	case orspb.OrderStatus_REJECTED:
		// C++: ORS_REJECT, BUSINESS_REJECT, SIM_REJECT, RMS_REJECT, ORDER_ERROR
		shouldDecrement = true
		log.Printf("[PairwiseArb:%s] Leg2 order rejected: %s", pas.ID, update.OrderId)

	case orspb.OrderStatus_CANCELED:
		// C++: CANCEL_ORDER_CONFIRM
		shouldDecrement = true
		log.Printf("[PairwiseArb:%s] Leg2 order canceled: %s", pas.ID, update.OrderId)

	case orspb.OrderStatus_FILLED:
		// C++: TRADE_CONFIRM && openQty == Quantity (完全成交)
		// Go: FilledQty == Quantity 表示完全成交
		if update.FilledQty == update.Quantity {
			shouldDecrement = true
			log.Printf("[PairwiseArb:%s] Leg2 order fully filled: %s", pas.ID, update.OrderId)
		}
	}

	if shouldDecrement {
		// C++: order->m_side == BUY ? strat->buyAggOrder-- : strat->sellAggOrder--;
		if update.Side == orspb.OrderSide_BUY {
			if pas.secondStrat.BuyAggOrder > 0 {
				pas.secondStrat.BuyAggOrder--
				log.Printf("[PairwiseArb:%s] Decremented buyAggOrder to %.0f", pas.ID, pas.secondStrat.BuyAggOrder)
			}
		} else {
			if pas.secondStrat.SellAggOrder > 0 {
				pas.secondStrat.SellAggOrder--
				log.Printf("[PairwiseArb:%s] Decremented sellAggOrder to %.0f", pas.ID, pas.secondStrat.SellAggOrder)
			}
		}
	}
}

// updateOrderMaps 根据订单状态更新订单映射
// C++: 维护 m_bidMap/m_askMap 用于避免重复挂单和计算待成交净头寸
func (pas *PairwiseArbStrategy) updateOrderMaps(update *orspb.OrderUpdate) {
	symbol := update.Symbol
	orderID := update.OrderId
	var orderMap *OrderPriceMap

	// 确定是哪个 leg 的订单
	switch symbol {
	case pas.symbol1:
		orderMap = pas.leg1OrderMap
	case pas.symbol2:
		orderMap = pas.leg2OrderMap
	default:
		return // 不属于本策略的品种
	}

	switch update.Status {
	case orspb.OrderStatus_ACCEPTED, orspb.OrderStatus_PARTIALLY_FILLED:
		// 订单确认或部分成交，添加到映射
		var side OrderSide
		if update.Side == orspb.OrderSide_BUY {
			side = OrderSideBuy
		} else {
			side = OrderSideSell
		}

		// 从订单中获取 level（如果在 metadata 中）
		level := 0
		// 注意：实际实现中可能需要从订单的扩展字段获取 level

		// 确定订单类别（C++: STANDARD/CROSS/MATCH）
		// 从 metadata 中读取 order_category 字段
		// 如果 metadata["order_category"] == "aggressive"，则为主动单
		category := SignalCategoryPassive
		if update.Metadata != nil {
			if cat, ok := update.Metadata["order_category"]; ok && cat == "aggressive" {
				category = SignalCategoryAggressive
			}
		}

		order := &PriceOrder{
			Price:     update.Price,
			OrderID:   orderID,
			Symbol:    symbol,
			Side:      side,
			Quantity:  int64(update.Quantity),
			FilledQty: int64(update.FilledQty),
			Level:     level,
			Category:  category,
		}
		orderMap.AddOrder(order)
		log.Printf("[PairwiseArb:%s] Added order to map: %s@%.2f, side=%v, level=%d",
			pas.ID, orderID, update.Price, side, level)

	case orspb.OrderStatus_FILLED, orspb.OrderStatus_CANCELED, orspb.OrderStatus_REJECTED:
		// 订单完成或取消，从映射中移除
		removed := orderMap.RemoveOrder(orderID)
		if removed != nil {
			log.Printf("[PairwiseArb:%s] Removed order from map: %s@%.2f, status=%v",
				pas.ID, orderID, removed.Price, update.Status)
		}
	}
}

// updateLeg1Position updates leg1 position statistics using ExtraStrategy
// 与 C++ ExtraStrategy::TradeCallBack() 完全一致
// 参考: tbsrc/Strategies/ExtraStrategy.cpp
//
// 重构：使用 firstStrat.ProcessTrade() 处理持仓更新
func (pas *PairwiseArbStrategy) updateLeg1Position(side orspb.OrderSide, qty int64, price float64) {
	// 转换方向类型
	txnSide := TransactionTypeBuy
	if side == orspb.OrderSide_SELL {
		txnSide = TransactionTypeSell
	}

	// 使用 ExtraStrategy.ProcessTrade 处理
	// 注意：这里需要一个 dummy orderID，因为这是从外部更新调用
	pas.firstStrat.ProcessTrade(0, int32(qty), price, txnSide)

	// 同步兼容字段
	pas.leg1Position = int64(pas.firstStrat.NetPos)
	pas.leg1YtdPosition = int64(pas.firstStrat.NetPosPassYtd)

	// 向共享内存写入 Leg1 持仓
	// C++: PairwiseArbStrategy.cpp SendTCacheLeg1Pos()
	// if (m_tcache) {
	//     m_tcache->store("leg1_pos", m_firstStrat->m_netpos_pass);
	// }
	if pas.ExecutionStrategy != nil && pas.ExecutionStrategy.TCache != nil {
		key := fmt.Sprintf("%s_leg1_pos", pas.ID)
		if err := pas.ExecutionStrategy.SendTCacheLeg1Pos(key, float64(pas.firstStrat.NetPosPass)); err != nil {
			log.Printf("[PairwiseArb:%s] Warning: Failed to send TCache leg1 pos: %v", pas.ID, err)
		}
	}

	// 日志输出
	todayNet := pas.firstStrat.NetPos - pas.firstStrat.NetPosPassYtd
	log.Printf("[PairwiseArb:%s] Leg1(%s) 持仓更新: NetPos=%d (Buy=%.0f@%.2f, Sell=%.0f@%.2f) [ytd=%d, 2day=%d]",
		pas.ID, pas.symbol1, pas.firstStrat.NetPos,
		pas.firstStrat.BuyQty, pas.firstStrat.BuyAvgPrice,
		pas.firstStrat.SellQty, pas.firstStrat.SellAvgPrice,
		pas.firstStrat.NetPosPassYtd, todayNet)
}

// updateLeg2Position updates leg2 position statistics using ExtraStrategy
// 与 C++ ExtraStrategy::TradeCallBack() 完全一致
// 参考: tbsrc/Strategies/ExtraStrategy.cpp
//
// 重构：使用 secondStrat.ProcessTrade() 处理持仓更新
func (pas *PairwiseArbStrategy) updateLeg2Position(side orspb.OrderSide, qty int64, price float64) {
	// 转换方向类型
	txnSide := TransactionTypeBuy
	if side == orspb.OrderSide_SELL {
		txnSide = TransactionTypeSell
	}

	// 使用 ExtraStrategy.ProcessTrade 处理
	// 注意：这里需要一个 dummy orderID，因为这是从外部更新调用
	pas.secondStrat.ProcessTrade(0, int32(qty), price, txnSide)

	// 同步兼容字段
	pas.leg2Position = int64(pas.secondStrat.NetPos)
	pas.leg2YtdPosition = int64(pas.secondStrat.NetPosPassYtd)

	// 向共享内存写入 Leg2 持仓
	// C++: 类似 SendTCacheLeg1Pos，但是用于 leg2
	if pas.ExecutionStrategy != nil && pas.ExecutionStrategy.TCache != nil {
		key := fmt.Sprintf("%s_leg2_pos", pas.ID)
		if err := pas.ExecutionStrategy.SendTCacheLeg1Pos(key, float64(pas.secondStrat.NetPosPass)); err != nil {
			log.Printf("[PairwiseArb:%s] Warning: Failed to send TCache leg2 pos: %v", pas.ID, err)
		}
	}

	// 日志输出
	todayNet := pas.secondStrat.NetPos - pas.secondStrat.NetPosPassYtd
	log.Printf("[PairwiseArb:%s] Leg2(%s) 持仓更新: NetPos=%d (Buy=%.0f@%.2f, Sell=%.0f@%.2f) [ytd=%d, 2day=%d]",
		pas.ID, pas.symbol2, pas.secondStrat.NetPos,
		pas.secondStrat.BuyQty, pas.secondStrat.BuyAvgPrice,
		pas.secondStrat.SellQty, pas.secondStrat.SellAvgPrice,
		pas.secondStrat.NetPosPassYtd, todayNet)
}

// OnTimer handles timer events
func (pas *PairwiseArbStrategy) OnTimer(now time.Time) {
	pas.mu.RLock()
	defer pas.mu.RUnlock()

	// Periodic housekeeping
	if !pas.running {
		return
	}

	// Log spread status
	stats := pas.spreadAnalyzer.GetStats()
	if now.Unix()%30 == 0 && stats.Std > 0 {
		log.Printf("[PairwiseArbStrategy:%s] Spread=%.2f (mean=%.2f, std=%.2f), Z=%.2f, Pos=[%d,%d]",
			pas.ID, stats.CurrentSpread, stats.Mean, stats.Std,
			stats.ZScore, pas.leg1Position, pas.leg2Position)
	}
}

// Start starts the strategy
func (pas *PairwiseArbStrategy) Start() error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	// 尝试从持久化文件恢复持仓
	if snapshot, err := LoadPositionSnapshot(pas.ID); err == nil && snapshot != nil {
		log.Printf("[PairwiseArbStrategy:%s] Restoring position from snapshot (saved at %s)",
			pas.ID, snapshot.Timestamp.Format("2006-01-02 15:04:05"))

		// 恢复leg持仓
		if qty, exists := snapshot.SymbolsPos[pas.symbol1]; exists {
			pas.leg1Position = qty
			log.Printf("[PairwiseArbStrategy:%s] Restored leg1 position: %s = %d",
				pas.ID, pas.symbol1, qty)
		}
		if qty, exists := snapshot.SymbolsPos[pas.symbol2]; exists {
			pas.leg2Position = qty
			log.Printf("[PairwiseArbStrategy:%s] Restored leg2 position: %s = %d",
				pas.ID, pas.symbol2, qty)
		}

		// 恢复昨仓净值（C++: m_netpos_pass_ytd）
		// 注意：SymbolsYesterdayPos 存储的是昨仓净值，今仓 = 当前持仓 - 昨仓
		if snapshot.SymbolsYesterdayPos != nil {
			if ytdPos, exists := snapshot.SymbolsYesterdayPos[pas.symbol1]; exists {
				pas.leg1YtdPosition = ytdPos
			}
			if ytdPos, exists := snapshot.SymbolsYesterdayPos[pas.symbol2]; exists {
				pas.leg2YtdPosition = ytdPos
			}
		}
		leg1TodayNet := pas.leg1Position - pas.leg1YtdPosition
		leg2TodayNet := pas.leg2Position - pas.leg2YtdPosition
		log.Printf("[PairwiseArbStrategy:%s] Restored ytd positions: leg1=[ytd=%d, 2day=%d], leg2=[ytd=%d, 2day=%d]",
			pas.ID, pas.leg1YtdPosition, leg1TodayNet, pas.leg2YtdPosition, leg2TodayNet)

		// 恢复 estimatedPosition 持仓（符合新的持仓模型）
		pas.estimatedPosition.NetQty = snapshot.TotalNetQty
		if snapshot.TotalNetQty > 0 {
			pas.estimatedPosition.BuyQty = snapshot.TotalLongQty
			pas.estimatedPosition.BuyAvgPrice = snapshot.AvgLongPrice
		} else if snapshot.TotalNetQty < 0 {
			pas.estimatedPosition.SellQty = snapshot.TotalShortQty
			pas.estimatedPosition.SellAvgPrice = snapshot.AvgShortPrice
		}
		// 更新兼容字段
		pas.estimatedPosition.UpdateCompatibilityFields()
		pas.pnl.RealizedPnL = snapshot.RealizedPnL

		log.Printf("[PairwiseArbStrategy:%s] Position restored: Long=%d, Short=%d, Net=%d",
			pas.ID, snapshot.TotalLongQty, snapshot.TotalShortQty, snapshot.TotalNetQty)
	} else if err != nil {
		log.Printf("[PairwiseArbStrategy:%s] Warning: Failed to load position snapshot: %v", pas.ID, err)
	}

	// 设置运行状态为 Active (直接设置，避免死锁)
	pas.ControlState.RunState = StrategyRunStateActive
	pas.running = true
	if pas.ControlState != nil {
		pas.ControlState.Active = true
	}
	log.Printf("[%s] Strategy activated", pas.ID)
	log.Printf("[PairwiseArbStrategy:%s] Started", pas.ID)
	return nil
}

// ApplyParameters 应用新参数（实现 ParameterUpdatable 接口）
func (pas *PairwiseArbStrategy) ApplyParameters(params map[string]interface{}) error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	log.Printf("[PairwiseArbStrategy:%s] Applying new parameters...", pas.ID)

	// 保存旧参数（用于日志）
	oldEntryZ := pas.entryZScore
	oldExitZ := pas.exitZScore
	oldOrderSize := pas.orderSize
	oldMaxPos := pas.maxPositionSize

	// 更新参数
	updated := false

	if val, ok := params["entry_zscore"].(float64); ok {
		pas.entryZScore = val
		updated = true
	}
	if val, ok := params["exit_zscore"].(float64); ok {
		pas.exitZScore = val
		updated = true
	}
	if val, ok := params["order_size"].(int); ok {
		pas.orderSize = int64(val)
		updated = true
	} else if val, ok := params["order_size"].(float64); ok {
		pas.orderSize = int64(val)
		updated = true
	}
	if val, ok := params["max_position_size"].(int); ok {
		pas.maxPositionSize = int64(val)
		updated = true
	} else if val, ok := params["max_position_size"].(float64); ok {
		pas.maxPositionSize = int64(val)
		updated = true
	}
	if val, ok := params["lookback_period"].(int); ok {
		pas.lookbackPeriod = val
		updated = true
	} else if val, ok := params["lookback_period"].(float64); ok {
		pas.lookbackPeriod = int(val)
		updated = true
	}
	if val, ok := params["min_correlation"].(float64); ok {
		pas.minCorrelation = val
		updated = true
	}

	// 动态阈值参数
	if val, ok := params["begin_zscore"].(float64); ok {
		pas.beginZScore = val
		updated = true
	}
	if val, ok := params["long_zscore"].(float64); ok {
		pas.longZScore = val
		updated = true
	}
	if val, ok := params["short_zscore"].(float64); ok {
		pas.shortZScore = val
		updated = true
	}
	if val, ok := params["use_dynamic_threshold"].(bool); ok {
		pas.useDynamicThreshold = val
		updated = true
	}

	// 主动追单参数
	if val, ok := params["aggressive_enabled"].(bool); ok {
		pas.aggressiveEnabled = val
		updated = true
	}
	if val, ok := params["aggressive_interval_ms"].(float64); ok {
		pas.aggressiveInterval = time.Duration(val) * time.Millisecond
		updated = true
	}
	if val, ok := params["aggressive_max_retry"].(float64); ok {
		pas.aggressiveMaxRetry = int(val)
		updated = true
	}
	if val, ok := params["aggressive_slop_ticks"].(float64); ok {
		pas.aggressiveSlopTicks = int(val)
		updated = true
	}
	if val, ok := params["aggressive_fail_threshold"].(float64); ok {
		pas.aggressiveFailThreshold = int(val)
		updated = true
	}

	// 外部 tValue 调整（C++: avgSpreadRatio = avgSpreadRatio_ori + tValue）
	if val, ok := params["t_value"].(float64); ok {
		oldTValue := pas.tValue
		pas.tValue = val
		log.Printf("[PairwiseArbStrategy:%s] tValue updated via ApplyParameters: %.4f -> %.4f",
			pas.ID, oldTValue, val)
		updated = true
	}

	if !updated {
		return fmt.Errorf("no valid parameters found to update")
	}

	// 参数验证
	if pas.entryZScore <= pas.exitZScore {
		// 回滚
		pas.entryZScore = oldEntryZ
		pas.exitZScore = oldExitZ
		return fmt.Errorf("entry_zscore (%.2f) must be greater than exit_zscore (%.2f)",
			pas.entryZScore, pas.exitZScore)
	}

	if pas.orderSize <= 0 || pas.orderSize > pas.maxPositionSize {
		pas.orderSize = oldOrderSize
		pas.maxPositionSize = oldMaxPos
		return fmt.Errorf("invalid order_size (%d) or max_position_size (%d)",
			pas.orderSize, pas.maxPositionSize)
	}

	// 输出变更日志
	log.Printf("[PairwiseArbStrategy:%s] ✓ Parameters updated:", pas.ID)
	if oldEntryZ != pas.entryZScore {
		log.Printf("[PairwiseArbStrategy:%s]   entry_zscore: %.2f -> %.2f",
			pas.ID, oldEntryZ, pas.entryZScore)
	}
	if oldExitZ != pas.exitZScore {
		log.Printf("[PairwiseArbStrategy:%s]   exit_zscore: %.2f -> %.2f",
			pas.ID, oldExitZ, pas.exitZScore)
	}
	if oldOrderSize != pas.orderSize {
		log.Printf("[PairwiseArbStrategy:%s]   order_size: %d -> %d",
			pas.ID, oldOrderSize, pas.orderSize)
	}
	if oldMaxPos != pas.maxPositionSize {
		log.Printf("[PairwiseArbStrategy:%s]   max_position_size: %d -> %d",
			pas.ID, oldMaxPos, pas.maxPositionSize)
	}

	return nil
}

// GetCurrentParameters 获取当前参数（用于API查询）
func (pas *PairwiseArbStrategy) GetCurrentParameters() map[string]interface{} {
	pas.mu.RLock()
	defer pas.mu.RUnlock()

	return map[string]interface{}{
		"entry_zscore":             pas.entryZScore,
		"exit_zscore":              pas.exitZScore,
		"order_size":               pas.orderSize,
		"max_position_size":        pas.maxPositionSize,
		"lookback_period":          pas.lookbackPeriod,
		"min_correlation":          pas.minCorrelation,
		"hedge_ratio":              pas.hedgeRatio,
		"spread_type":              pas.spreadType,
		"use_cointegration":        pas.useCointegration,
		// 动态阈值参数
		"use_dynamic_threshold":    pas.useDynamicThreshold,
		"begin_zscore":             pas.beginZScore,
		"long_zscore":              pas.longZScore,
		"short_zscore":             pas.shortZScore,
		"entry_zscore_bid":         pas.entryZScoreBid, // 运行时值
		"entry_zscore_ask":         pas.entryZScoreAsk, // 运行时值
		// 主动追单参数
		"aggressive_enabled":       pas.aggressiveEnabled,
		"aggressive_interval_ms":   pas.aggressiveInterval.Milliseconds(),
		"aggressive_max_retry":     pas.aggressiveMaxRetry,
		"aggressive_slop_ticks":    pas.aggressiveSlopTicks,
		"aggressive_fail_threshold": pas.aggressiveFailThreshold,
		// 外部 tValue 调整
		"t_value":                  pas.tValue,
	}
}

// SetTValue 设置外部 tValue 调整值
// C++: avgSpreadRatio = avgSpreadRatio_ori + tValue
// tValue 允许外部信号调整价差均值，使策略更容易入场或出场
//
// 参数:
//   - value: 调整值（正值提高均值使做空更容易，负值降低均值使做多更容易）
func (pas *PairwiseArbStrategy) SetTValue(value float64) {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	oldValue := pas.tValue
	pas.tValue = value
	log.Printf("[PairwiseArbStrategy:%s] tValue updated: %.4f -> %.4f", pas.ID, oldValue, value)
}

// GetTValue 获取当前 tValue 值
func (pas *PairwiseArbStrategy) GetTValue() float64 {
	pas.mu.RLock()
	defer pas.mu.RUnlock()
	return pas.tValue
}

// HandleSquareoff 处理平仓
// C++: PairwiseArbStrategy::HandleSquareoff()
func (pas *PairwiseArbStrategy) HandleSquareoff() {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	log.Printf("[PairwiseArb:%s] HandleSquareoff triggered", pas.ID)

	// 两条腿都触发平仓
	pas.firstStrat.HandleSquareoff()
	pas.secondStrat.HandleSquareoff()

	// 生成退出信号
	pas.generateExitSignals(nil)
}

// HandleSquareON 恢复开仓能力（平仓完成后调用）
// C++: PairwiseArbStrategy::HandleSquareON()
// 注意：这不是"开启平仓"，而是"恢复开仓"（平仓状态 OFF）
//
// C++ 原代码:
//   ExecutionStrategy::HandleSquareON();  // 发送监控状态
//   m_agg_repeat = 1;
//   m_firstStrat->m_onExit = false;
//   m_firstStrat->m_onCancel = false;
//   m_firstStrat->m_onFlat = false;
//   m_secondStrat->m_onExit = false;
//   m_secondStrat->m_onCancel = false;
//   m_secondStrat->m_onFlat = false;
func (pas *PairwiseArbStrategy) HandleSquareON() {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	// C++: m_agg_repeat = 1
	pas.aggRepeat = 1

	// C++: 重置 firstStrat 的平仓标志
	pas.firstStrat.OnExit = false
	pas.firstStrat.OnCancel = false
	pas.firstStrat.OnFlat = false

	// C++: 重置 secondStrat 的平仓标志
	pas.secondStrat.OnExit = false
	pas.secondStrat.OnCancel = false
	pas.secondStrat.OnFlat = false

	// 重置控制状态中的平仓模式
	if pas.ControlState != nil {
		pas.ControlState.FlattenMode = false
	}

	log.Printf("[PairwiseArb:%s] HandleSquareON: Squareoff mode OFF, trading enabled", pas.ID)
}

// Stop stops the strategy
func (pas *PairwiseArbStrategy) Stop() error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	// === 保存 daily_init 文件（C++: PairwiseArbStrategy::SaveMatrix2） ===
	// C++: SaveMatrix2(std::string("../data/daily_init.") + std::to_string(m_strategyID));
	// 在 HandleSquareoff() 末尾调用，保存当前状态供下次启动恢复
	dailyInitPath := GetDailyInitPath(pas.ExecutionStrategy.StrategyID)

	// C++: avgSpreadRatio_ori 在停止时应保存运行时计算的均值
	// 这样下次启动时可以恢复到当前的价差均值状态
	// 如果运行时均值为 0（未初始化），则保留原始值
	avgPxToSave := pas.spreadAnalyzer.GetStats().Mean
	if avgPxToSave == 0 && pas.avgSpreadRatio_ori != 0 {
		avgPxToSave = pas.avgSpreadRatio_ori
	}

	err := SaveMatrix2(
		dailyInitPath,
		pas.ExecutionStrategy.StrategyID,
		avgPxToSave,                                      // avgSpreadRatio_ori（运行时均值）
		pas.firstStrat.Instru.Symbol,                     // m_origbaseName1
		pas.secondStrat.Instru.Symbol,                    // m_origbaseName2
		pas.firstStrat.NetPosPass,                        // m_netpos_pass (ytd1)
		pas.secondStrat.NetPosAgg,                        // m_netpos_agg (ytd2)
	)
	if err != nil {
		log.Printf("[PairwiseArbStrategy:%s] Warning: SaveMatrix2 failed: %v", pas.ID, err)
	} else {
		log.Printf("[PairwiseArbStrategy:%s] SaveMatrix2 saved: avgPx=%.6f (spreadMean), "+
			"origBaseName1=%s, origBaseName2=%s, netpos_pass=%d, netpos_agg=%d",
			pas.ID, avgPxToSave,
			pas.firstStrat.Instru.Symbol, pas.secondStrat.Instru.Symbol,
			pas.firstStrat.NetPosPass, pas.secondStrat.NetPosAgg)
	}

	// 保存当前持仓到文件（包括昨/今仓区分）- JSON 格式（Go 特有）
	snapshot := PositionSnapshot{
		StrategyID:    pas.ID,
		Timestamp:     time.Now(),
		TotalLongQty:  pas.estimatedPosition.LongQty,
		TotalShortQty: pas.estimatedPosition.ShortQty,
		TotalNetQty:   pas.estimatedPosition.NetQty,
		AvgLongPrice:  pas.estimatedPosition.AvgLongPrice,
		AvgShortPrice: pas.estimatedPosition.AvgShortPrice,
		RealizedPnL:   pas.pnl.RealizedPnL,
		SymbolsPos: map[string]int64{
			pas.symbol1: pas.leg1Position,
			pas.symbol2: pas.leg2Position,
		},
		// 昨仓净值（C++: m_netpos_pass_ytd）
		// 注意：收盘时当前持仓变成"昨仓"，所以保存当前持仓作为下一交易日的昨仓
		SymbolsYesterdayPos: map[string]int64{
			pas.symbol1: pas.leg1Position, // 收盘持仓 = 下一交易日的昨仓
			pas.symbol2: pas.leg2Position,
		},
	}

	if err := SavePositionSnapshot(snapshot); err != nil {
		log.Printf("[PairwiseArbStrategy:%s] Warning: Failed to save position snapshot: %v", pas.ID, err)
		// 不阻断停止流程
	} else {
		leg1TodayNet := pas.leg1Position - pas.leg1YtdPosition
		leg2TodayNet := pas.leg2Position - pas.leg2YtdPosition
		log.Printf("[PairwiseArbStrategy:%s] Position snapshot saved: Long=%d, Short=%d, Net=%d [leg1: ytd=%d, 2day=%d] [leg2: ytd=%d, 2day=%d]",
			pas.ID, snapshot.TotalLongQty, snapshot.TotalShortQty, snapshot.TotalNetQty,
			pas.leg1YtdPosition, leg1TodayNet, pas.leg2YtdPosition, leg2TodayNet)
	}

	pas.ControlState.RunState = StrategyRunStateStopped
	// 直接设置，避免死锁
	pas.running = false
	if pas.ControlState != nil {
		pas.ControlState.Active = false
	}

	// 关闭共享内存（C++: 析构函数中调用 shmdt）
	if pas.ExecutionStrategy != nil {
		pas.ExecutionStrategy.CloseSharedMemory()
	}

	log.Printf("[%s] Strategy deactivated", pas.ID)
	log.Printf("[PairwiseArbStrategy:%s] Stopped", pas.ID)
	return nil
}

// InitializePositions 实现PositionInitializer接口：从外部初始化持仓
// C++: 对应从 CTP 查询持仓后初始化 m_firstStrat/m_secondStrat 的 m_netpos_pass
func (pas *PairwiseArbStrategy) InitializePositions(positions map[string]int64) error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	log.Printf("[PairwiseArbStrategy:%s] Initializing positions from external source (CTP)", pas.ID)

	// 初始化 leg1 持仓 (firstStrat)
	if qty, exists := positions[pas.symbol1]; exists {
		pas.leg1Position = qty
		// 同步到 ExtraStrategy
		// C++: m_firstStrat->m_netpos_pass = qty
		if pas.firstStrat != nil {
			pas.firstStrat.NetPosPass = int32(qty)
			pas.firstStrat.NetPos = int32(qty)
			if qty > 0 {
				pas.firstStrat.BuyQty = float64(qty)
			} else if qty < 0 {
				pas.firstStrat.SellQty = float64(-qty)
			}
			log.Printf("[PairwiseArbStrategy:%s] Initialized firstStrat position: %s NetPosPass=%d",
				pas.ID, pas.symbol1, pas.firstStrat.NetPosPass)
		}
		log.Printf("[PairwiseArbStrategy:%s] Initialized leg1 position: %s = %d",
			pas.ID, pas.symbol1, qty)
	}

	// 初始化 leg2 持仓 (secondStrat)
	if qty, exists := positions[pas.symbol2]; exists {
		pas.leg2Position = qty
		// 同步到 ExtraStrategy
		// C++: m_secondStrat->m_netpos_agg = qty (secondStrat 是主动单腿，用 NetPosAgg)
		// C++: daily_init 中 ytd2 保存的是 m_secondStrat->m_netpos_agg
		if pas.secondStrat != nil {
			pas.secondStrat.NetPosPass = int32(qty)
			pas.secondStrat.NetPosAgg = int32(qty) // C++: m_netpos_agg
			pas.secondStrat.NetPos = int32(qty)
			if qty > 0 {
				pas.secondStrat.BuyQty = float64(qty)
			} else if qty < 0 {
				pas.secondStrat.SellQty = float64(-qty)
			}
			log.Printf("[PairwiseArbStrategy:%s] Initialized secondStrat position: %s NetPos=%d, NetPosAgg=%d",
				pas.ID, pas.symbol2, pas.secondStrat.NetPos, pas.secondStrat.NetPosAgg)
		}
		log.Printf("[PairwiseArbStrategy:%s] Initialized leg2 position: %s = %d",
			pas.ID, pas.symbol2, qty)
	}

	// 更新 estimatedPosition（简化处理）
	totalQty := pas.leg1Position + pas.leg2Position
	if totalQty > 0 {
		pas.estimatedPosition.LongQty = totalQty
		pas.estimatedPosition.NetQty = totalQty
	} else if totalQty < 0 {
		pas.estimatedPosition.ShortQty = -totalQty
		pas.estimatedPosition.NetQty = totalQty
	}

	log.Printf("[PairwiseArbStrategy:%s] Positions initialized: leg1=%d, leg2=%d, net=%d",
		pas.ID, pas.leg1Position, pas.leg2Position, pas.estimatedPosition.NetQty)

	return nil
}

// InitializePositionsWithCost 使用成本价初始化持仓
// 注意：此方法是 Go 代码新增的，C++ 原代码中没有对应实现
// C++ 原代码的 m_buyPrice/m_sellPrice 是当天成交均价，开盘时为 0
// C++ 的 P&L 只计算当天交易产生的盈亏，昨仓成本为 0
// Go 代码使用 CTP 返回的成本价来计算完整的浮动盈亏，便于风控和监控
func (pas *PairwiseArbStrategy) InitializePositionsWithCost(positions map[string]PositionWithCost) error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	log.Printf("[PairwiseArbStrategy:%s] Initializing positions with cost from CTP", pas.ID)

	// 初始化 leg1 持仓和成本 (firstStrat)
	if pos, exists := positions[pas.symbol1]; exists && pos.Quantity != 0 {
		pas.leg1Position = pos.Quantity
		// 同步到 ExtraStrategy
		if pas.firstStrat != nil {
			pas.firstStrat.NetPosPass = int32(pos.Quantity)
			pas.firstStrat.NetPos = int32(pos.Quantity)
			// 设置成本价（与 C++ 不同：C++ 开盘时成本为 0）
			if pos.Quantity > 0 {
				pas.firstStrat.BuyQty = float64(pos.Quantity)
				pas.firstStrat.BuyTotalQty = float64(pos.Quantity)
				pas.firstStrat.BuyAvgPrice = pos.AvgCost
				pas.firstStrat.BuyTotalValue = pos.AvgCost * float64(pos.Quantity)
			} else {
				pas.firstStrat.SellQty = float64(-pos.Quantity)
				pas.firstStrat.SellTotalQty = float64(-pos.Quantity)
				pas.firstStrat.SellAvgPrice = pos.AvgCost
				pas.firstStrat.SellTotalValue = pos.AvgCost * float64(-pos.Quantity)
			}
		}
		log.Printf("[PairwiseArbStrategy:%s] Initialized leg1: %s Qty=%d, AvgCost=%.2f",
			pas.ID, pas.symbol1, pos.Quantity, pos.AvgCost)
	}

	// 初始化 leg2 持仓和成本 (secondStrat)
	if pos, exists := positions[pas.symbol2]; exists && pos.Quantity != 0 {
		pas.leg2Position = pos.Quantity
		// 同步到 ExtraStrategy
		// C++: m_secondStrat->m_netpos_agg = qty (secondStrat 是主动单腿)
		if pas.secondStrat != nil {
			pas.secondStrat.NetPosPass = int32(pos.Quantity)
			pas.secondStrat.NetPosAgg = int32(pos.Quantity) // C++: m_netpos_agg
			pas.secondStrat.NetPos = int32(pos.Quantity)
			// 设置成本价（与 C++ 不同：C++ 开盘时成本为 0）
			if pos.Quantity > 0 {
				pas.secondStrat.BuyQty = float64(pos.Quantity)
				pas.secondStrat.BuyTotalQty = float64(pos.Quantity)
				pas.secondStrat.BuyAvgPrice = pos.AvgCost
				pas.secondStrat.BuyTotalValue = pos.AvgCost * float64(pos.Quantity)
			} else {
				pas.secondStrat.SellQty = float64(-pos.Quantity)
				pas.secondStrat.SellTotalQty = float64(-pos.Quantity)
				pas.secondStrat.SellAvgPrice = pos.AvgCost
				pas.secondStrat.SellTotalValue = pos.AvgCost * float64(-pos.Quantity)
			}
		}
		log.Printf("[PairwiseArbStrategy:%s] Initialized leg2: %s Qty=%d, AvgCost=%.2f, NetPosAgg=%d",
			pas.ID, pas.symbol2, pos.Quantity, pos.AvgCost, pas.secondStrat.NetPosAgg)
	}

	// 更新 estimatedPosition
	totalQty := pas.leg1Position + pas.leg2Position
	if totalQty > 0 {
		pas.estimatedPosition.LongQty = totalQty
		pas.estimatedPosition.NetQty = totalQty
	} else if totalQty < 0 {
		pas.estimatedPosition.ShortQty = -totalQty
		pas.estimatedPosition.NetQty = totalQty
	}

	log.Printf("[PairwiseArbStrategy:%s] Positions with cost initialized: leg1=%d, leg2=%d, net=%d",
		pas.ID, pas.leg1Position, pas.leg2Position, pas.estimatedPosition.NetQty)

	return nil
}

// GetPositionsBySymbol 实现PositionProvider接口：返回按品种的持仓
func (pas *PairwiseArbStrategy) GetPositionsBySymbol() map[string]int64 {
	pas.mu.RLock()
	defer pas.mu.RUnlock()

	return map[string]int64{
		pas.symbol1: pas.leg1Position,
		pas.symbol2: pas.leg2Position,
	}
}

// GetSpreadStatus returns current spread status
func (pas *PairwiseArbStrategy) GetSpreadStatus() map[string]interface{} {
	pas.mu.RLock()
	defer pas.mu.RUnlock()

	stats := pas.spreadAnalyzer.GetStats()
	return map[string]interface{}{
		"symbol1":        pas.symbol1,
		"symbol2":        pas.symbol2,
		"price1":         pas.price1,
		"price2":         pas.price2,
		"spread":         stats.CurrentSpread,
		"spread_mean":    stats.Mean,
		"spread_std":     stats.Std,
		"z_score":        stats.ZScore,
		"hedge_ratio":    stats.HedgeRatio,
		"leg1_position":  pas.leg1Position,
		"leg2_position":  pas.leg2Position,
	}
}

// GetLegsInfo returns detailed information for each leg (for UI display)
// 包括昨/今仓区分信息
func (pas *PairwiseArbStrategy) GetLegsInfo() []map[string]interface{} {
	pas.mu.RLock()
	defer pas.mu.RUnlock()

	// Determine side for each leg
	leg1Side := "flat"
	if pas.leg1Position > 0 {
		leg1Side = "long"
	} else if pas.leg1Position < 0 {
		leg1Side = "short"
	}

	leg2Side := "flat"
	if pas.leg2Position > 0 {
		leg2Side = "long"
	} else if pas.leg2Position < 0 {
		leg2Side = "short"
	}

	// C++: 2day = m_netpos_pass - m_netpos_pass_ytd
	leg1TodayNet := pas.leg1Position - pas.leg1YtdPosition
	leg2TodayNet := pas.leg2Position - pas.leg2YtdPosition

	return []map[string]interface{}{
		{
			"symbol":        pas.symbol1,
			"price":         pas.price1,
			"position":      pas.leg1Position,
			"side":          leg1Side,
			"ytd_position":  pas.leg1YtdPosition, // 昨仓净值 (C++: m_netpos_pass_ytd)
			"today_net":     leg1TodayNet,        // 今仓净值 (C++: m_netpos_pass - m_netpos_pass_ytd)
		},
		{
			"symbol":        pas.symbol2,
			"price":         pas.price2,
			"position":      pas.leg2Position,
			"side":          leg2Side,
			"ytd_position":  pas.leg2YtdPosition,
			"today_net":     leg2TodayNet,
		},
	}
}

// updatePairwisePNL 计算配对套利的专用P&L
// 配对策略有两个独立的品种，需要分别计算每一腿的盈亏
// 使用对手价（bid/ask）计算，符合 tbsrc 逻辑
// updatePairwisePNL calculates P&L for pairwise strategy
// 参考 tbsrc PairwiseArbStrategy: 每条腿有独立的 ExtraStrategy，因此有独立的平均价格
// arbi_unrealisedPNL = m_firstStrat->m_unrealisedPNL + m_secondStrat->m_unrealisedPNL
// 重构：使用 firstStrat/secondStrat 的 BuyAvgPrice/SellAvgPrice
func (pas *PairwiseArbStrategy) updatePairwisePNL() {
	var unrealizedPnL float64 = 0

	// Leg1 浮动盈亏（使用对手价和 firstStrat 的平均价格）
	// 参考 tbsrc ExtraStrategy::CalculatePNL
	// C++: m_unrealisedPNL = m_netpos * (counterPrice - costPrice) * m_instru->m_priceMultiplier
	if pas.leg1Position != 0 {
		var leg1PnL float64
		var avgCost float64
		var counterPrice float64
		multiplier1 := GetContractMultiplier(pas.symbol1) // C++: m_instru->m_priceMultiplier

		if pas.leg1Position > 0 {
			// Leg1 多头: 使用卖一价（bid），因为平仓时要卖出
			// tbsrc: m_unrealisedPNL = m_netpos * (m_instru->bidPx[0] - m_buyPrice) * m_priceMultiplier
			avgCost = pas.firstStrat.BuyAvgPrice  // 使用 firstStrat 的买入均价
			counterPrice = pas.bid1
			leg1PnL = (counterPrice - avgCost) * float64(pas.leg1Position) * multiplier1
		} else {
			// Leg1 空头: 使用买一价（ask），因为平仓时要买入
			// tbsrc: m_unrealisedPNL = -1 * m_netpos * (m_sellPrice - m_instru->askPx[0]) * m_priceMultiplier
			avgCost = pas.firstStrat.SellAvgPrice  // 使用 firstStrat 的卖出均价
			counterPrice = pas.ask1
			leg1PnL = (avgCost - counterPrice) * float64(-pas.leg1Position) * multiplier1
		}
		unrealizedPnL += leg1PnL

		log.Printf("[PairwiseArb:%s] 📊 Leg1(%s) P&L: %.2f (Pos=%d, AvgCost=%.2f, Counter=%.2f, Mult=%.0f)",
			pas.ID, pas.symbol1, leg1PnL, pas.leg1Position, avgCost, counterPrice, multiplier1)
	}

	// Leg2 浮动盈亏（使用对手价和 secondStrat 的平均价格）
	// 参考 tbsrc ExtraStrategy::CalculatePNL
	// C++: m_unrealisedPNL = m_netpos * (counterPrice - costPrice) * m_instru->m_priceMultiplier
	if pas.leg2Position != 0 {
		var leg2PnL float64
		var avgCost float64
		var counterPrice float64
		multiplier2 := GetContractMultiplier(pas.symbol2) // C++: m_instru->m_priceMultiplier

		if pas.leg2Position > 0 {
			// Leg2 多头: 使用卖一价（bid）
			// tbsrc: m_unrealisedPNL = m_netpos * (m_instru->bidPx[0] - m_buyPrice) * m_priceMultiplier
			avgCost = pas.secondStrat.BuyAvgPrice  // 使用 secondStrat 的买入均价
			counterPrice = pas.bid2
			leg2PnL = (counterPrice - avgCost) * float64(pas.leg2Position) * multiplier2
		} else {
			// Leg2 空头: 使用买一价（ask）
			// tbsrc: m_unrealisedPNL = -1 * m_netpos * (m_sellPrice - m_instru->askPx[0]) * m_priceMultiplier
			avgCost = pas.secondStrat.SellAvgPrice  // 使用 secondStrat 的卖出均价
			counterPrice = pas.ask2
			leg2PnL = (avgCost - counterPrice) * float64(-pas.leg2Position) * multiplier2
		}
		unrealizedPnL += leg2PnL

		log.Printf("[PairwiseArb:%s] 📊 Leg2(%s) P&L: %.2f (Pos=%d, AvgCost=%.2f, Counter=%.2f, Mult=%.0f)",
			pas.ID, pas.symbol2, leg2PnL, pas.leg2Position, avgCost, counterPrice, multiplier2)
	}

	// 更新 PNL
	// tbsrc: 配对策略的总 P&L = 两条腿的 P&L 相加
	pas.pnl.UnrealizedPnL = unrealizedPnL
	// C++: arbi_realisedPNL = (m_firstStrat->m_realisedPNL - m_firstStrat->m_transTotalValue)
	//                       + (m_secondStrat->m_realisedPNL - m_secondStrat->m_transTotalValue)
	// 参考: tbsrc/Strategies/PairwiseArbStrategy.cpp:180
	pas.pnl.RealizedPnL = (pas.firstStrat.RealisedPNL - pas.firstStrat.TransTotalValue) +
		(pas.secondStrat.RealisedPNL - pas.secondStrat.TransTotalValue)
	pas.pnl.TotalPnL = pas.pnl.RealizedPnL + pas.pnl.UnrealizedPnL
	pas.pnl.NetPnL = pas.pnl.TotalPnL - pas.pnl.TradingFees
	pas.pnl.Timestamp = time.Now()

	if pas.leg1Position != 0 || pas.leg2Position != 0 {
		log.Printf("[PairwiseArb:%s] 💰 Total P&L: Realized=%.2f, Unrealized=%.2f, Total=%.2f",
			pas.ID, pas.pnl.RealizedPnL, pas.pnl.UnrealizedPnL, pas.pnl.TotalPnL)
	}
}

// GetFirstLeg 返回第一条腿的 ExtraStrategy
// C++: 对应 m_firstStrat
func (pas *PairwiseArbStrategy) GetFirstLeg() *ExtraStrategy {
	return pas.firstStrat
}

// GetSecondLeg 返回第二条腿的 ExtraStrategy
// C++: 对应 m_secondStrat
func (pas *PairwiseArbStrategy) GetSecondLeg() *ExtraStrategy {
	return pas.secondStrat
}

// ============================================================================
// Strategy 接口实现
// 以下方法实现 Strategy 接口，使 PairwiseArbStrategy 可以被 StrategyEngine 管理
// ============================================================================

// GetID returns the strategy ID
func (pas *PairwiseArbStrategy) GetID() string {
	return pas.ID
}

// GetType returns the strategy type
func (pas *PairwiseArbStrategy) GetType() string {
	return pas.Type
}

// IsRunning returns true if strategy is running
func (pas *PairwiseArbStrategy) IsRunning() bool {
	pas.mu.RLock()
	defer pas.mu.RUnlock()
	return pas.running
}

// GetSignals returns pending signals and clears the queue
func (pas *PairwiseArbStrategy) GetSignals() []*TradingSignal {
	pas.mu.Lock()
	defer pas.mu.Unlock()
	signals := pas.PendingSignals
	pas.PendingSignals = make([]*TradingSignal, 0)
	return signals
}

// AddSignal adds a new trading signal
// Note: This method assumes the caller already holds the lock (pas.mu)
// because it's typically called from within OnMarketData which holds the lock
func (pas *PairwiseArbStrategy) AddSignal(signal *TradingSignal) {
	// 不获取锁 - 调用者已持有锁
	pas.PendingSignals = append(pas.PendingSignals, signal)
	pas.Status.SignalCount++
	pas.Status.LastSignalTime = time.Now()
}

// GetEstimatedPosition returns current estimated position
func (pas *PairwiseArbStrategy) GetEstimatedPosition() *EstimatedPosition {
	return pas.estimatedPosition
}

// GetPosition returns current position (alias for GetEstimatedPosition)
func (pas *PairwiseArbStrategy) GetPosition() *EstimatedPosition {
	return pas.estimatedPosition
}

// GetPNL returns current P&L
func (pas *PairwiseArbStrategy) GetPNL() *PNL {
	return pas.pnl
}

// GetRiskMetrics returns current risk metrics
func (pas *PairwiseArbStrategy) GetRiskMetrics() *RiskMetrics {
	return pas.riskMetrics
}

// GetStatus returns strategy status
func (pas *PairwiseArbStrategy) GetStatus() *StrategyStatus {
	pas.mu.RLock()
	defer pas.mu.RUnlock()
	pas.Status.IsRunning = pas.running
	pas.Status.EstimatedPosition = pas.estimatedPosition
	pas.Status.PNL = pas.pnl
	pas.Status.RiskMetrics = pas.riskMetrics
	return pas.Status
}

// Reset resets the strategy to initial state
func (pas *PairwiseArbStrategy) Reset() {
	pas.mu.Lock()
	defer pas.mu.Unlock()
	pas.PendingSignals = make([]*TradingSignal, 0)
	pas.Orders = make(map[string]*orspb.OrderUpdate)
	pas.estimatedPosition = &EstimatedPosition{}
	pas.pnl = &PNL{}
	pas.riskMetrics = &RiskMetrics{}
	pas.Status = &StrategyStatus{StrategyID: pas.ID}
}

// Activate activates the strategy
func (pas *PairwiseArbStrategy) Activate() {
	pas.mu.Lock()
	defer pas.mu.Unlock()
	pas.running = true
	if pas.ControlState != nil {
		pas.ControlState.Activate()
	}
	log.Printf("[%s] Strategy activated", pas.ID)
}

// Deactivate deactivates the strategy
func (pas *PairwiseArbStrategy) Deactivate() {
	pas.mu.Lock()
	defer pas.mu.Unlock()
	pas.running = false
	if pas.ControlState != nil {
		pas.ControlState.Deactivate()
	}
	log.Printf("[%s] Strategy deactivated", pas.ID)
}

// updateRiskMetrics updates risk metrics
func (pas *PairwiseArbStrategy) updateRiskMetrics(currentPrice float64) {
	pas.riskMetrics.PositionSize = abs(pas.estimatedPosition.NetQty)
	pas.riskMetrics.ExposureValue = float64(pas.riskMetrics.PositionSize) * currentPrice
	pas.riskMetrics.Timestamp = time.Now()

	// Update max drawdown
	if pas.pnl.TotalPnL < 0 && absFloat(pas.pnl.TotalPnL) > pas.riskMetrics.MaxDrawdown {
		pas.riskMetrics.MaxDrawdown = absFloat(pas.pnl.TotalPnL)
	}
}

// UpdatePosition updates position based on order update
// 符合中国期货市场规则：净持仓模型
func (pas *PairwiseArbStrategy) UpdatePosition(update *orspb.OrderUpdate) {
	// Store order update
	pas.Orders[update.OrderId] = update

	// Update position only for filled orders
	if update.Status == orspb.OrderStatus_FILLED {
		pas.Status.FillCount++

		qty := update.FilledQty
		price := update.AvgPrice

		if update.Side == orspb.OrderSide_BUY {
			// 买入逻辑
			pas.estimatedPosition.BuyTotalQty += qty
			pas.estimatedPosition.BuyTotalValue += float64(qty) * price

			if pas.estimatedPosition.NetQty < 0 {
				// 当前是空头持仓，买入平空
				closedQty := qty
				if closedQty > pas.estimatedPosition.SellQty {
					closedQty = pas.estimatedPosition.SellQty
				}
				// realized P&L 由按腿的 ExecutionStrategy.ProcessTrade() 计算，此处不做跨合约计算
				pas.estimatedPosition.SellQty -= closedQty
				pas.estimatedPosition.NetQty += closedQty
				qty -= closedQty
			}

			if qty > 0 {
				// 开多
				pas.estimatedPosition.BuyQty += qty
				if pas.estimatedPosition.BuyQty > 0 {
					pas.estimatedPosition.BuyAvgPrice = pas.estimatedPosition.BuyTotalValue / float64(pas.estimatedPosition.BuyTotalQty)
				}
				pas.estimatedPosition.NetQty += qty
			}
		} else {
			// 卖出逻辑
			pas.estimatedPosition.SellTotalQty += qty
			pas.estimatedPosition.SellTotalValue += float64(qty) * price

			if pas.estimatedPosition.NetQty > 0 {
				// 当前是多头持仓，卖出平多
				closedQty := qty
				if closedQty > pas.estimatedPosition.BuyQty {
					closedQty = pas.estimatedPosition.BuyQty
				}
				// realized P&L 由按腿的 ExecutionStrategy.ProcessTrade() 计算，此处不做跨合约计算
				pas.estimatedPosition.BuyQty -= closedQty
				pas.estimatedPosition.NetQty -= closedQty
				qty -= closedQty
			}

			if qty > 0 {
				// 开空
				pas.estimatedPosition.SellQty += qty
				if pas.estimatedPosition.SellQty > 0 {
					pas.estimatedPosition.SellAvgPrice = pas.estimatedPosition.SellTotalValue / float64(pas.estimatedPosition.SellTotalQty)
				}
				pas.estimatedPosition.NetQty -= qty
			}
		}

		// Update long/short quantities
		if pas.estimatedPosition.NetQty > 0 {
			pas.estimatedPosition.LongQty = pas.estimatedPosition.NetQty
			pas.estimatedPosition.ShortQty = 0
		} else {
			pas.estimatedPosition.LongQty = 0
			pas.estimatedPosition.ShortQty = -pas.estimatedPosition.NetQty
		}
	}
}

// UpdateParameters updates strategy parameters (for hot reload)
func (pas *PairwiseArbStrategy) UpdateParameters(params map[string]interface{}) error {
	return pas.ApplyParameters(params)
}

// OnAuctionData handles auction data (集合竞价行情)
func (pas *PairwiseArbStrategy) OnAuctionData(md *mdpb.MarketDataUpdate) {
	// 配对套利策略在集合竞价期间不操作
	log.Printf("[PairwiseArbStrategy:%s] Ignoring auction data for %s", pas.ID, md.Symbol)
}

// GetConfig returns the strategy configuration
func (pas *PairwiseArbStrategy) GetConfig() *StrategyConfig {
	pas.mu.RLock()
	defer pas.mu.RUnlock()
	return pas.Config
}

// GetControlState returns the strategy control state
func (pas *PairwiseArbStrategy) GetControlState() *StrategyControlState {
	pas.mu.RLock()
	defer pas.mu.RUnlock()
	return pas.ControlState
}

// SendOrder generates and sends orders based on current state
// C++: virtual void SendOrder() = 0
func (pas *PairwiseArbStrategy) SendOrder() {
	// PairwiseArbStrategy 通过 OnMarketData 中的 sendAggressiveOrder 发送订单
	// 此方法保留以满足接口要求
}

// OnTradeUpdate is called after a trade is processed
// C++: virtual void OnTradeUpdate() {}
func (pas *PairwiseArbStrategy) OnTradeUpdate() {
	// 成交后更新状态，可用于统计或日志
}

// CheckSquareoff checks if position needs to be squared off
// C++: virtual void CheckSquareoff(MarketUpdateNew*)
func (pas *PairwiseArbStrategy) CheckSquareoff() {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	// 检查止损
	if pas.pnl != nil && pas.Config != nil {
		maxLoss := pas.Config.Parameters["max_loss"]
		if maxLoss != nil {
			if ml, ok := maxLoss.(float64); ok && pas.pnl.TotalPnL < -ml {
				pas.ControlState.FlattenMode = true
			}
		}
	}
}

// SetThresholds sets dynamic thresholds based on position
// C++: virtual void SetThresholds()
func (pas *PairwiseArbStrategy) SetThresholds() {
	// 已在 setDynamicThresholds 中实现
	pas.setDynamicThresholds()
}

// === Engine/Manager 需要的方法 ===

// CanSendOrder returns true if strategy can send orders
func (pas *PairwiseArbStrategy) CanSendOrder() bool {
	pas.mu.RLock()
	defer pas.mu.RUnlock()
	return pas.running && pas.ControlState != nil && pas.ControlState.IsActive() && !pas.ControlState.FlattenMode
}

// SetLastMarketData stores the last market data for a symbol
func (pas *PairwiseArbStrategy) SetLastMarketData(symbol string, md *mdpb.MarketDataUpdate) {
	pas.mu.Lock()
	defer pas.mu.Unlock()
	if pas.LastMarketData == nil {
		pas.LastMarketData = make(map[string]*mdpb.MarketDataUpdate)
	}
	pas.LastMarketData[symbol] = md
}

// GetLastMarketData returns the last market data for a symbol
func (pas *PairwiseArbStrategy) GetLastMarketData(symbol string) *mdpb.MarketDataUpdate {
	pas.mu.RLock()
	defer pas.mu.RUnlock()
	if pas.LastMarketData == nil {
		return nil
	}
	return pas.LastMarketData[symbol]
}

// TriggerFlatten triggers position flattening
func (pas *PairwiseArbStrategy) TriggerFlatten(reason FlattenReason, aggressive bool) {
	pas.mu.Lock()
	defer pas.mu.Unlock()
	if pas.ControlState != nil {
		pas.ControlState.FlattenMode = true
		pas.ControlState.FlattenReason = reason
	}
}

// GetPendingCancels returns orders pending cancellation
func (pas *PairwiseArbStrategy) GetPendingCancels() []*orspb.OrderUpdate {
	pas.mu.RLock()
	defer pas.mu.RUnlock()
	// 返回所有撤单中的订单
	var cancels []*orspb.OrderUpdate
	for _, order := range pas.Orders {
		if order.Status == orspb.OrderStatus_CANCELING {
			cancels = append(cancels, order)
		}
	}
	return cancels
}
