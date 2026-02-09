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
type PairwiseArbStrategy struct {
	*BaseStrategy

	// Strategy parameters
	symbol1           string  // First symbol (e.g., "ag2412")
	symbol2           string  // Second symbol (e.g., "ag2501")
	lookbackPeriod    int     // Period for mean/std calculation (default: 100)
	entryZScore       float64 // Z-score threshold to enter (default: 2.0)
	exitZScore        float64 // Z-score threshold to exit (default: 0.5)
	orderSize         int64   // Size per leg (default: 10)
	maxPositionSize   int64   // Maximum position per leg (default: 50)
	minCorrelation    float64 // Minimum correlation to trade (default: 0.7)
	hedgeRatio        float64 // Current hedge ratio (calculated dynamically)
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

	// C++: SUPPORTING_ORDERS - 限制追单数量
	// 防止在一个方向上发送过多追单
	supportingOrders int   // 最大追单数量（配置参数）
	sellAggOrder     int   // 当前卖单追单计数（C++: sellAggOrder）
	buyAggOrder      int   // 当前买单追单计数（C++: buyAggOrder）

	// Spread analyzer (encapsulates spread calculation and statistics)
	spreadAnalyzer    *spread.SpreadAnalyzer

	// Position tracking (separate for each leg)
	// 参考 tbsrc：每条腿有独立的 ExecutionStrategy，因此需要独立的持仓统计
	leg1Position      int64
	leg2Position      int64

	// Leg1 独立持仓统计（类似 tbsrc m_firstStrat）
	leg1BuyQty        int64
	leg1SellQty       int64
	leg1BuyAvgPrice   float64
	leg1SellAvgPrice  float64
	leg1BuyTotalQty   int64
	leg1SellTotalQty  int64
	leg1BuyTotalValue float64
	leg1SellTotalValue float64
	// 昨仓净值（C++: m_netpos_pass_ytd）
	// 含义：昨日收盘时的净持仓（正=多头，负=空头）
	// 今仓净值 = leg1Position - leg1YtdPosition
	leg1YtdPosition   int64 // Leg1 昨仓净值（C++: m_netpos_pass_ytd）

	// Leg2 独立持仓统计（类似 tbsrc m_secondStrat）
	leg2BuyQty        int64
	leg2SellQty       int64
	leg2BuyAvgPrice   float64
	leg2SellAvgPrice  float64
	leg2BuyTotalQty   int64
	leg2SellTotalQty  int64
	leg2BuyTotalValue float64
	leg2SellTotalValue float64
	// 昨仓净值（C++: m_netpos_agg 作为昨仓初值）
	leg2YtdPosition   int64 // Leg2 昨仓净值

	// 多层挂单参数（C++: MAX_QUOTE_LEVEL）
	maxQuoteLevel    int     // 最大挂单层数 (默认: 1, 仅一档)
	quoteLevelSizes  []int64 // 每层下单量 (默认: [orderSize])
	enableMultiLevel bool    // 是否启用多层挂单

	// 订单簿深度（5档价格）
	bidPrices1 []float64 // Leg1 买盘 5 档价格
	askPrices1 []float64 // Leg1 卖盘 5 档价格
	bidPrices2 []float64 // Leg2 买盘 5 档价格
	askPrices2 []float64 // Leg2 卖盘 5 档价格

	// 挂单映射（C++: m_bidMap/m_askMap）
	leg1OrderMap *OrderPriceMap // Leg1 订单映射
	leg2OrderMap *OrderPriceMap // Leg2 订单映射

	// 价格优化参数（C++: GetBidPrice_first 隐性订单簿检测）
	enablePriceOptimize bool    // 是否启用价格优化
	priceOptimizeGap    int     // 触发优化的 tick 跳跃数
	tickSize1           float64 // Leg1 最小变动单位
	tickSize2           float64 // Leg2 最小变动单位

	// 外部 tValue 调整参数（C++: avgSpreadRatio = avgSpreadRatio_ori + tValue）
	// tValue 允许外部信号调整价差均值，使策略更容易入场或出场
	tValue float64 // 外部调整值（正值提高均值，负值降低均值）

	mu sync.RWMutex
}

// NewPairwiseArbStrategy creates a new pairs trading strategy
func NewPairwiseArbStrategy(id string) *PairwiseArbStrategy {
	maxHistoryLen := 200

	pas := &PairwiseArbStrategy{
		BaseStrategy:     NewBaseStrategy(id, "pairwise_arb"),
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
		spreadAnalyzer:   nil,
		// 多层挂单默认值
		maxQuoteLevel:    1,
		quoteLevelSizes:  []int64{10},
		enableMultiLevel: false,
		// 订单簿深度
		bidPrices1: make([]float64, 5),
		askPrices1: make([]float64, 5),
		bidPrices2: make([]float64, 5),
		askPrices2: make([]float64, 5),
		// 订单映射
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

	// 设置具体策略实例，用于参数热加载
	pas.BaseStrategy.SetConcreteStrategy(pas)

	return pas
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
	if val, ok := config.Parameters["max_position_size"].(float64); ok {
		pas.maxPositionSize = int64(val)
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
	if val, ok := config.Parameters["supporting_orders"].(float64); ok {
		pas.supportingOrders = int(val)
	} else {
		pas.supportingOrders = 3 // 默认限制 3 个追单
	}
	// 初始化追单状态
	pas.aggRepeat = 1
	pas.aggDirection = 0
	pas.sellAggOrder = 0
	pas.buyAggOrder = 0

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

	return nil
}

// OnMarketData handles market data updates
func (pas *PairwiseArbStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	if !pas.IsRunning() {
		return
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

	// Update PNL (配对策略专用计算：分别计算两腿)
	pas.updatePairwisePNL()

	// Update risk metrics (use average price for exposure calculation)
	avgPrice := (pas.price1 + pas.price2) / 2.0
	pas.BaseStrategy.UpdateRiskMetrics(avgPrice)

	// 动态调整入场阈值（根据持仓）
	pas.setDynamicThresholds()

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

	// 应用外部 tValue 调整
	// C++: avgSpreadRatio = avgSpreadRatio_ori + tValue
	// 调整后的 Z-Score = (spread - (mean + tValue)) / std = zscore_ori - tValue/std
	adjustedZScore := spreadStats.ZScore
	if pas.tValue != 0 && spreadStats.Std > 1e-10 {
		// tValue > 0: zscore 降低（均值升高，当前spread相对更低）
		// tValue < 0: zscore 升高（均值降低，当前spread相对更高）
		adjustedZScore = (spreadStats.CurrentSpread - (spreadStats.Mean + pas.tValue)) / spreadStats.Std
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

	// Calculate hedge quantity using current hedge ratio
	hedgeQty := int64(math.Round(float64(qty) * spreadStats.HedgeRatio))

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
	pas.BaseStrategy.AddSignal(signal1)

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
	pas.BaseStrategy.AddSignal(signal2)

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
	pas.BaseStrategy.AddSignal(signal1)

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
	pas.BaseStrategy.AddSignal(signal2)

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

	// Calculate hedge quantity using current hedge ratio
	hedgeQty := int64(math.Round(float64(qty) * stats.HedgeRatio))

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
	pas.BaseStrategy.AddSignal(signal1)

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
	pas.BaseStrategy.AddSignal(signal2)

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
		// 未启用动态阈值，使用静态 entryZScore
		pas.entryZScoreBid = pas.entryZScore
		pas.entryZScoreAsk = pas.entryZScore
		return
	}

	// C++: long_place_diff_thold = LONG_PLACE - BEGIN_PLACE
	longPlaceDiff := pas.longZScore - pas.beginZScore
	// C++: short_place_diff_thold = BEGIN_PLACE - SHORT_PLACE
	shortPlaceDiff := pas.beginZScore - pas.shortZScore

	// 计算持仓比例：netpos / maxPos
	posRatio := float64(pas.leg1Position) / float64(pas.maxPositionSize)

	if pas.leg1Position == 0 {
		// C++: 无持仓时使用初始阈值
		pas.entryZScoreBid = pas.beginZScore
		pas.entryZScoreAsk = pas.beginZScore
	} else if pas.leg1Position > 0 {
		// C++: 多头持仓
		// tholdBidPlace = BEGIN_PLACE + long_place_diff_thold * netpos / maxPos
		pas.entryZScoreBid = pas.beginZScore + longPlaceDiff*posRatio
		// tholdAskPlace = BEGIN_PLACE - short_place_diff_thold * netpos / maxPos
		pas.entryZScoreAsk = pas.beginZScore - shortPlaceDiff*posRatio
	} else {
		// C++: 空头持仓 (netpos < 0)
		// tholdBidPlace = BEGIN_PLACE + short_place_diff_thold * netpos / maxPos
		pas.entryZScoreBid = pas.beginZScore + shortPlaceDiff*posRatio
		// tholdAskPlace = BEGIN_PLACE - long_place_diff_thold * netpos / maxPos
		pas.entryZScoreAsk = pas.beginZScore - longPlaceDiff*posRatio
	}
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
		pas.sellAggOrder = 0
		pas.buyAggOrder = 0
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
	if pas.supportingOrders > 0 {
		if targetSide == OrderSideSell && pas.sellAggOrder > pas.supportingOrders {
			log.Printf("[PairwiseArb:%s] ⛔ Sell aggressive order limit reached: %d > %d",
				pas.ID, pas.sellAggOrder, pas.supportingOrders)
			return
		}
		if targetSide == OrderSideBuy && pas.buyAggOrder > pas.supportingOrders {
			log.Printf("[PairwiseArb:%s] ⛔ Buy aggressive order limit reached: %d > %d",
				pas.ID, pas.buyAggOrder, pas.supportingOrders)
			return
		}
	}

	// 4. 方向变化检查：如果方向变化，重置计数
	if pas.aggDirection != newDirection {
		pas.aggRepeat = 1
		pas.aggDirection = newDirection
		// 方向变化时也重置对应方向的追单计数
		if newDirection == -1 {
			pas.sellAggOrder = 0
		} else {
			pas.buyAggOrder = 0
		}
	}

	// 5. 时间间隔检查
	// C++: if (last_agg_side != side || now - last_agg_time > 500ms)
	if time.Since(pas.aggLastTime) < pas.aggressiveInterval {
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
			"sell_agg_order": pas.sellAggOrder,
			"buy_agg_order":  pas.buyAggOrder,
		},
	}
	pas.BaseStrategy.AddSignal(signal)

	log.Printf("[PairwiseArb:%s] 🏃 Aggressive order #%d: %v %s %d @ %.2f (exposure=%d, pending=%d, sellAgg=%d, buyAgg=%d)",
		pas.ID, pas.aggRepeat, targetSide, targetSymbol, targetQty, orderPrice, exposure, pendingNetpos, pas.sellAggOrder, pas.buyAggOrder)

	// 9. 更新追单状态
	// C++: sellAggOrder++ / buyAggOrder++
	if targetSide == OrderSideSell {
		pas.sellAggOrder++
	} else {
		pas.buyAggOrder++
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
func (pas *PairwiseArbStrategy) OnOrderUpdate(update *orspb.OrderUpdate) {
	// CRITICAL: 检查订单是否属于本策略
	// 修复 Bug: 防止策略接收到其他策略的订单回调
	if update.StrategyId != pas.ID {
		// 不是本策略的订单，直接忽略
		return
	}

	pas.mu.Lock()
	defer pas.mu.Unlock()

	log.Printf("[PairwiseArb:%s] 🚨 OnOrderUpdate ENTRY: OrderID=%s, Status=%v, Symbol=%s, Side=%v, FilledQty=%d",
		pas.ID, update.OrderId, update.Status, update.Symbol, update.Side, update.FilledQty)

	if !pas.IsRunning() {
		log.Printf("[PairwiseArb:%s] ⚠️  Strategy not running, ignoring update", pas.ID)
		return
	}

	// Update base strategy position (for overall PNL tracking)
	log.Printf("[PairwiseArb:%s] 🚨 BEFORE UpdatePosition call, EstimatedPosition ptr=%p", pas.ID, pas.EstimatedPosition)
	pas.UpdatePosition(update)
	log.Printf("[PairwiseArb:%s] 🚨 AFTER UpdatePosition call, EstimatedPosition=%+v", pas.ID, pas.EstimatedPosition)

	// 维护订单映射（多层挂单用）
	pas.updateOrderMaps(update)

	// Update leg-specific positions (similar to tbsrc: each leg has its own ExecutionStrategy)
	// 参考 tbsrc ExecutionStrategy::TradeCallBack
	if update.Status == orspb.OrderStatus_FILLED && update.FilledQty > 0 {
		symbol := update.Symbol
		qty := int64(update.FilledQty)
		price := update.AvgPrice

		// Determine which leg this order belongs to
		if symbol == pas.symbol1 {
			pas.updateLeg1Position(update.Side, qty, price)
		} else if symbol == pas.symbol2 {
			pas.updateLeg2Position(update.Side, qty, price)
		}

		// 成交后检查敞口，如果敞口为0则重置追单状态
		exposure := pas.calculateExposure()
		if exposure == 0 {
			if pas.aggRepeat > 1 || pas.sellAggOrder > 0 || pas.buyAggOrder > 0 {
				log.Printf("[PairwiseArb:%s] ✅ Exposure cleared, resetting aggressive order state (retry=%d, sellAgg=%d, buyAgg=%d)",
					pas.ID, pas.aggRepeat-1, pas.sellAggOrder, pas.buyAggOrder)
			}
			pas.aggRepeat = 1
			pas.aggDirection = 0
			pas.aggFailCount = 0   // 成功清除敞口，重置失败计数
			pas.sellAggOrder = 0   // 重置卖追单计数
			pas.buyAggOrder = 0    // 重置买追单计数
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

// updateLeg1Position updates leg1 position statistics
// 与 C++ ExecutionStrategy::TradeCallBack() 完全一致
// 参考: tbsrc/Strategies/ExecutionStrategy.cpp
//
// 中国期货净持仓模型:
//   - 买入: 先平空(m_sellQty)，再开多(m_buyQty)
//   - 卖出: 先平多(m_buyQty)，再开空(m_sellQty)
//   - 净持仓: m_netpos = m_buyQty - m_sellQty
//
// C++ 原代码中 m_netpos_pass_ytd 仅用于日志打印，实际的今/昨仓处理在 CTP Plugin 层
// 今仓净值 = leg1Position - leg1YtdPosition (C++: m_netpos_pass - m_netpos_pass_ytd)
func (pas *PairwiseArbStrategy) updateLeg1Position(side orspb.OrderSide, qty int64, price float64) {
	if side == orspb.OrderSide_BUY {
		// 买入逻辑
		pas.leg1BuyTotalQty += qty
		pas.leg1BuyTotalValue += float64(qty) * price

		// 检查是否有空头需要平仓
		if pas.leg1Position < 0 {
			// 平空
			closedQty := qty
			if closedQty > pas.leg1SellQty {
				closedQty = pas.leg1SellQty
			}
			pas.leg1SellQty -= closedQty
			pas.leg1Position += closedQty
			qty -= closedQty

			if pas.leg1SellQty == 0 {
				pas.leg1SellAvgPrice = 0
			}
		}

		// 开多
		if qty > 0 {
			totalCost := pas.leg1BuyAvgPrice * float64(pas.leg1BuyQty)
			totalCost += price * float64(qty)
			pas.leg1BuyQty += qty
			pas.leg1Position += qty
			if pas.leg1BuyQty > 0 {
				pas.leg1BuyAvgPrice = totalCost / float64(pas.leg1BuyQty)
			}
			// C++: TBLOG << " m_netpos_pass_ytd:" << m_firstStrat->m_netpos_pass_ytd
			todayNet := pas.leg1Position - pas.leg1YtdPosition
			log.Printf("[PairwiseArb:%s] Leg1 开多: %d @ %.2f, 多头均价 %.2f, 净持仓 %d (ytd=%d, 2day=%d)",
				pas.ID, qty, price, pas.leg1BuyAvgPrice, pas.leg1Position, pas.leg1YtdPosition, todayNet)
		}
	} else {
		// 卖出逻辑
		pas.leg1SellTotalQty += qty
		pas.leg1SellTotalValue += float64(qty) * price

		// 检查是否有多头需要平仓
		if pas.leg1Position > 0 {
			// 平多
			closedQty := qty
			if closedQty > pas.leg1BuyQty {
				closedQty = pas.leg1BuyQty
			}
			pas.leg1BuyQty -= closedQty
			pas.leg1Position -= closedQty
			qty -= closedQty

			if pas.leg1BuyQty == 0 {
				pas.leg1BuyAvgPrice = 0
			}
		}

		// 开空
		if qty > 0 {
			totalCost := pas.leg1SellAvgPrice * float64(pas.leg1SellQty)
			totalCost += price * float64(qty)
			pas.leg1SellQty += qty
			pas.leg1Position -= qty
			if pas.leg1SellQty > 0 {
				pas.leg1SellAvgPrice = totalCost / float64(pas.leg1SellQty)
			}
			// C++: TBLOG << " m_netpos_pass_ytd:" << m_firstStrat->m_netpos_pass_ytd
			todayNet := pas.leg1Position - pas.leg1YtdPosition
			log.Printf("[PairwiseArb:%s] Leg1 开空: %d @ %.2f, 空头均价 %.2f, 净持仓 %d (ytd=%d, 2day=%d)",
				pas.ID, qty, price, pas.leg1SellAvgPrice, pas.leg1Position, pas.leg1YtdPosition, todayNet)
		}
	}

	todayNet := pas.leg1Position - pas.leg1YtdPosition
	// C++: TBLOG << " m_netpos_pass:" << m_firstStrat->m_netpos_pass << " m_netpos_pass_ytd:" << m_firstStrat->m_netpos_pass_ytd
	log.Printf("[PairwiseArb:%s] Leg1(%s) 持仓更新: NetPos=%d (Buy=%d@%.2f, Sell=%d@%.2f) [ytd=%d, 2day=%d]",
		pas.ID, pas.symbol1, pas.leg1Position, pas.leg1BuyQty, pas.leg1BuyAvgPrice,
		pas.leg1SellQty, pas.leg1SellAvgPrice, pas.leg1YtdPosition, todayNet)
}

// updateLeg2Position updates leg2 position statistics
// 与 C++ ExecutionStrategy::TradeCallBack() 完全一致
// 参考: tbsrc/Strategies/ExecutionStrategy.cpp
//
// C++ 原代码中 Leg2 使用 m_netpos_agg，今仓净值同样是差值计算
func (pas *PairwiseArbStrategy) updateLeg2Position(side orspb.OrderSide, qty int64, price float64) {
	if side == orspb.OrderSide_BUY {
		// 买入逻辑
		pas.leg2BuyTotalQty += qty
		pas.leg2BuyTotalValue += float64(qty) * price

		// 检查是否有空头需要平仓
		if pas.leg2Position < 0 {
			// 平空
			closedQty := qty
			if closedQty > pas.leg2SellQty {
				closedQty = pas.leg2SellQty
			}
			pas.leg2SellQty -= closedQty
			pas.leg2Position += closedQty
			qty -= closedQty

			if pas.leg2SellQty == 0 {
				pas.leg2SellAvgPrice = 0
			}
		}

		// 开多
		if qty > 0 {
			totalCost := pas.leg2BuyAvgPrice * float64(pas.leg2BuyQty)
			totalCost += price * float64(qty)
			pas.leg2BuyQty += qty
			pas.leg2Position += qty
			if pas.leg2BuyQty > 0 {
				pas.leg2BuyAvgPrice = totalCost / float64(pas.leg2BuyQty)
			}
			todayNet := pas.leg2Position - pas.leg2YtdPosition
			log.Printf("[PairwiseArb:%s] Leg2 开多: %d @ %.2f, 多头均价 %.2f, 净持仓 %d (ytd=%d, 2day=%d)",
				pas.ID, qty, price, pas.leg2BuyAvgPrice, pas.leg2Position, pas.leg2YtdPosition, todayNet)
		}
	} else {
		// 卖出逻辑
		pas.leg2SellTotalQty += qty
		pas.leg2SellTotalValue += float64(qty) * price

		// 检查是否有多头需要平仓
		if pas.leg2Position > 0 {
			// 平多
			closedQty := qty
			if closedQty > pas.leg2BuyQty {
				closedQty = pas.leg2BuyQty
			}
			pas.leg2BuyQty -= closedQty
			pas.leg2Position -= closedQty
			qty -= closedQty

			if pas.leg2BuyQty == 0 {
				pas.leg2BuyAvgPrice = 0
			}
		}

		// 开空
		if qty > 0 {
			totalCost := pas.leg2SellAvgPrice * float64(pas.leg2SellQty)
			totalCost += price * float64(qty)
			pas.leg2SellQty += qty
			pas.leg2Position -= qty
			if pas.leg2SellQty > 0 {
				pas.leg2SellAvgPrice = totalCost / float64(pas.leg2SellQty)
			}
			todayNet := pas.leg2Position - pas.leg2YtdPosition
			log.Printf("[PairwiseArb:%s] Leg2 开空: %d @ %.2f, 空头均价 %.2f, 净持仓 %d (ytd=%d, 2day=%d)",
				pas.ID, qty, price, pas.leg2SellAvgPrice, pas.leg2Position, pas.leg2YtdPosition, todayNet)
		}
	}

	todayNet := pas.leg2Position - pas.leg2YtdPosition
	log.Printf("[PairwiseArb:%s] Leg2(%s) 持仓更新: NetPos=%d (Buy=%d@%.2f, Sell=%d@%.2f) [ytd=%d, 2day=%d]",
		pas.ID, pas.symbol2, pas.leg2Position, pas.leg2BuyQty, pas.leg2BuyAvgPrice,
		pas.leg2SellQty, pas.leg2SellAvgPrice, pas.leg2YtdPosition, todayNet)
}

// OnTimer handles timer events
func (pas *PairwiseArbStrategy) OnTimer(now time.Time) {
	pas.mu.RLock()
	defer pas.mu.RUnlock()

	// Periodic housekeeping
	if !pas.IsRunning() {
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

		// 恢复BaseStrategy持仓（符合新的持仓模型）
		pas.EstimatedPosition.NetQty = snapshot.TotalNetQty
		if snapshot.TotalNetQty > 0 {
			pas.EstimatedPosition.BuyQty = snapshot.TotalLongQty
			pas.EstimatedPosition.BuyAvgPrice = snapshot.AvgLongPrice
		} else if snapshot.TotalNetQty < 0 {
			pas.EstimatedPosition.SellQty = snapshot.TotalShortQty
			pas.EstimatedPosition.SellAvgPrice = snapshot.AvgShortPrice
		}
		// 更新兼容字段
		pas.EstimatedPosition.UpdateCompatibilityFields()
		pas.PNL.RealizedPnL = snapshot.RealizedPnL

		log.Printf("[PairwiseArbStrategy:%s] Position restored: Long=%d, Short=%d, Net=%d",
			pas.ID, snapshot.TotalLongQty, snapshot.TotalShortQty, snapshot.TotalNetQty)
	} else if err != nil {
		log.Printf("[PairwiseArbStrategy:%s] Warning: Failed to load position snapshot: %v", pas.ID, err)
	}

	// 设置运行状态为 Active
	pas.ControlState.RunState = StrategyRunStateActive
	pas.Activate()
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

// Stop stops the strategy
func (pas *PairwiseArbStrategy) Stop() error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	// 保存当前持仓到文件（包括昨/今仓区分）
	snapshot := PositionSnapshot{
		StrategyID:    pas.ID,
		Timestamp:     time.Now(),
		TotalLongQty:  pas.EstimatedPosition.LongQty,
		TotalShortQty: pas.EstimatedPosition.ShortQty,
		TotalNetQty:   pas.EstimatedPosition.NetQty,
		AvgLongPrice:  pas.EstimatedPosition.AvgLongPrice,
		AvgShortPrice: pas.EstimatedPosition.AvgShortPrice,
		RealizedPnL:   pas.PNL.RealizedPnL,
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
	pas.Deactivate()
	log.Printf("[PairwiseArbStrategy:%s] Stopped", pas.ID)
	return nil
}

// InitializePositions 实现PositionInitializer接口：从外部初始化持仓
func (pas *PairwiseArbStrategy) InitializePositions(positions map[string]int64) error {
	pas.mu.Lock()
	defer pas.mu.Unlock()

	log.Printf("[PairwiseArbStrategy:%s] Initializing positions from external source", pas.ID)

	// 初始化leg持仓
	if qty, exists := positions[pas.symbol1]; exists {
		pas.leg1Position = qty
		log.Printf("[PairwiseArbStrategy:%s] Initialized leg1 position: %s = %d",
			pas.ID, pas.symbol1, qty)
	}

	if qty, exists := positions[pas.symbol2]; exists {
		pas.leg2Position = qty
		log.Printf("[PairwiseArbStrategy:%s] Initialized leg2 position: %s = %d",
			pas.ID, pas.symbol2, qty)
	}

	// 更新BaseStrategy的Position（简化处理）
	totalQty := pas.leg1Position + pas.leg2Position
	if totalQty > 0 {
		pas.EstimatedPosition.LongQty = totalQty
		pas.EstimatedPosition.NetQty = totalQty
	} else if totalQty < 0 {
		pas.EstimatedPosition.ShortQty = -totalQty
		pas.EstimatedPosition.NetQty = totalQty
	}

	log.Printf("[PairwiseArbStrategy:%s] Positions initialized: leg1=%d, leg2=%d, net=%d",
		pas.ID, pas.leg1Position, pas.leg2Position, pas.EstimatedPosition.NetQty)

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
// 参考 tbsrc PairwiseArbStrategy: 每条腿有独立的 ExecutionStrategy，因此有独立的平均价格
// arbi_unrealisedPNL = m_firstStrat->m_unrealisedPNL + m_secondStrat->m_unrealisedPNL
func (pas *PairwiseArbStrategy) updatePairwisePNL() {
	var unrealizedPnL float64 = 0

	// Leg1 浮动盈亏（使用对手价和 Leg1 独立的平均价格）
	// 参考 tbsrc ExecutionStrategy::CalculatePNL
	if pas.leg1Position != 0 {
		var leg1PnL float64
		var avgCost float64
		var counterPrice float64

		if pas.leg1Position > 0 {
			// Leg1 多头: 使用卖一价（bid），因为平仓时要卖出
			// tbsrc: m_unrealisedPNL = m_netpos * (m_instru->bidPx[0] - m_buyPrice)
			avgCost = pas.leg1BuyAvgPrice  // ✅ 使用 Leg1 独立的买入均价
			counterPrice = pas.bid1
			leg1PnL = (counterPrice - avgCost) * float64(pas.leg1Position)
		} else {
			// Leg1 空头: 使用买一价（ask），因为平仓时要买入
			// tbsrc: m_unrealisedPNL = -1 * m_netpos * (m_sellPrice - m_instru->askPx[0])
			avgCost = pas.leg1SellAvgPrice  // ✅ 使用 Leg1 独立的卖出均价
			counterPrice = pas.ask1
			leg1PnL = (avgCost - counterPrice) * float64(-pas.leg1Position)
		}
		unrealizedPnL += leg1PnL

		log.Printf("[PairwiseArb:%s] 📊 Leg1(%s) P&L: %.2f (Pos=%d, AvgCost=%.2f, Counter=%.2f)",
			pas.ID, pas.symbol1, leg1PnL, pas.leg1Position, avgCost, counterPrice)
	}

	// Leg2 浮动盈亏（使用对手价和 Leg2 独立的平均价格）
	// 参考 tbsrc ExecutionStrategy::CalculatePNL
	if pas.leg2Position != 0 {
		var leg2PnL float64
		var avgCost float64
		var counterPrice float64

		if pas.leg2Position > 0 {
			// Leg2 多头: 使用卖一价（bid）
			avgCost = pas.leg2BuyAvgPrice  // ✅ 使用 Leg2 独立的买入均价
			counterPrice = pas.bid2
			leg2PnL = (counterPrice - avgCost) * float64(pas.leg2Position)
		} else {
			// Leg2 空头: 使用买一价（ask）
			avgCost = pas.leg2SellAvgPrice  // ✅ 使用 Leg2 独立的卖出均价
			counterPrice = pas.ask2
			leg2PnL = (avgCost - counterPrice) * float64(-pas.leg2Position)
		}
		unrealizedPnL += leg2PnL

		log.Printf("[PairwiseArb:%s] 📊 Leg2(%s) P&L: %.2f (Pos=%d, AvgCost=%.2f, Counter=%.2f)",
			pas.ID, pas.symbol2, leg2PnL, pas.leg2Position, avgCost, counterPrice)
	}

	// 更新 BaseStrategy 的 PNL
	// tbsrc: 配对策略的总 P&L = 两条腿的 P&L 相加
	pas.PNL.UnrealizedPnL = unrealizedPnL
	pas.PNL.TotalPnL = pas.PNL.RealizedPnL + pas.PNL.UnrealizedPnL
	pas.PNL.NetPnL = pas.PNL.TotalPnL - pas.PNL.TradingFees
	pas.PNL.Timestamp = time.Now()

	if pas.leg1Position != 0 || pas.leg2Position != 0 {
		log.Printf("[PairwiseArb:%s] 💰 Total P&L: Realized=%.2f, Unrealized=%.2f, Total=%.2f",
			pas.ID, pas.PNL.RealizedPnL, pas.PNL.UnrealizedPnL, pas.PNL.TotalPnL)
	}
}

// GetBaseStrategy returns the underlying BaseStrategy (for engine integration)
func (pas *PairwiseArbStrategy) GetBaseStrategy() *BaseStrategy {
	return pas.BaseStrategy
}
