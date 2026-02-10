package spread

import (
	"math"
	"sync"

	"github.com/yourusername/quantlink-trade-system/pkg/stats"
)

// SpreadAnalyzer 分析两个资产之间的 spread
// 用于配对交易、跨期套利等策略
type SpreadAnalyzer struct {
	symbol1    string
	symbol2    string
	spreadType SpreadType
	hedgeRatio float64

	// Time series
	price1Series *stats.TimeSeries
	price2Series *stats.TimeSeries
	spreadSeries *stats.TimeSeries

	// Current state
	price1        float64
	price2        float64
	currentSpread float64
	spreadMean    float64
	spreadStd     float64
	currentZScore float64
	correlation   float64

	mu sync.RWMutex
}

// NewSpreadAnalyzer 创建 spread 分析器
func NewSpreadAnalyzer(symbol1, symbol2 string, spreadType SpreadType, maxHistory int) *SpreadAnalyzer {
	sm := stats.NewSeriesManager()

	return &SpreadAnalyzer{
		symbol1:      symbol1,
		symbol2:      symbol2,
		spreadType:   spreadType,
		hedgeRatio:   1.0, // 默认 1:1
		price1Series: sm.AddSeries("price1", maxHistory),
		price2Series: sm.AddSeries("price2", maxHistory),
		spreadSeries: sm.AddSeries("spread", maxHistory),
	}
}

// UpdatePrice1 更新第一个资产价格
func (sa *SpreadAnalyzer) UpdatePrice1(price float64, timestamp int64) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	sa.price1 = price
	sa.price1Series.Append(price, timestamp)
}

// UpdatePrice2 更新第二个资产价格
func (sa *SpreadAnalyzer) UpdatePrice2(price float64, timestamp int64) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	sa.price2 = price
	sa.price2Series.Append(price, timestamp)
}

// UpdatePrices 同时更新两个资产价格
func (sa *SpreadAnalyzer) UpdatePrices(price1, price2 float64, timestamp int64) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	sa.price1 = price1
	sa.price2 = price2
	sa.price1Series.Append(price1, timestamp)
	sa.price2Series.Append(price2, timestamp)

	// 自动计算 spread
	sa.calculateSpreadLocked()
}

// UpdatePricesNow 使用当前时间戳更新价格
func (sa *SpreadAnalyzer) UpdatePricesNow(price1, price2 float64) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	sa.price1 = price1
	sa.price2 = price2
	sa.price1Series.AppendNow(price1)
	sa.price2Series.AppendNow(price2)

	// 自动计算 spread
	sa.calculateSpreadLocked()
}

// CalculateSpread 计算当前 spread（需要先调用 UpdatePrices）
func (sa *SpreadAnalyzer) CalculateSpread() float64 {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	return sa.calculateSpreadLocked()
}

// calculateSpreadLocked 计算 spread（内部使用，已持有锁）
func (sa *SpreadAnalyzer) calculateSpreadLocked() float64 {
	if sa.price1 == 0 || sa.price2 == 0 {
		return 0
	}

	switch sa.spreadType {
	case SpreadTypeRatio:
		sa.currentSpread = sa.price1 / sa.price2
	case SpreadTypeLog:
		sa.currentSpread = math.Log(sa.price1) - math.Log(sa.price2)
	case SpreadTypeDifference:
		fallthrough
	default:
		sa.currentSpread = sa.price1 - sa.hedgeRatio*sa.price2
	}

	// 添加到历史
	sa.spreadSeries.AppendNow(sa.currentSpread)

	return sa.currentSpread
}

// UpdateStatistics 更新统计指标（均值、标准差、z-score）
func (sa *SpreadAnalyzer) UpdateStatistics(lookbackPeriod int) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	if sa.spreadSeries.Len() < lookbackPeriod {
		return
	}

	// 计算 spread 统计
	spreadStats := sa.spreadSeries.Stats(lookbackPeriod)
	sa.spreadMean = spreadStats.Mean
	sa.spreadStd = spreadStats.Std

	// 计算 z-score
	sa.currentZScore = stats.ZScore(sa.currentSpread, sa.spreadMean, sa.spreadStd)
}

// UpdateHedgeRatio 更新对冲比率（使用线性回归）
func (sa *SpreadAnalyzer) UpdateHedgeRatio(lookbackPeriod int) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	if sa.price1Series.Len() < lookbackPeriod || sa.price2Series.Len() < lookbackPeriod {
		return
	}

	price1 := sa.price1Series.GetLast(lookbackPeriod)
	price2 := sa.price2Series.GetLast(lookbackPeriod)

	// 计算 Beta (price1 对 price2 的回归系数)
	sa.hedgeRatio = stats.Beta(price1, price2)
}

// UpdateCorrelation 更新相关系数
func (sa *SpreadAnalyzer) UpdateCorrelation(lookbackPeriod int) float64 {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	if sa.price1Series.Len() < lookbackPeriod || sa.price2Series.Len() < lookbackPeriod {
		return 0
	}

	price1 := sa.price1Series.GetLast(lookbackPeriod)
	price2 := sa.price2Series.GetLast(lookbackPeriod)

	sa.correlation = stats.Correlation(price1, price2)
	return sa.correlation
}

// UpdateAll 更新所有统计指标（统计、对冲比率、相关系数）
func (sa *SpreadAnalyzer) UpdateAll(lookbackPeriod int) {
	sa.UpdateStatistics(lookbackPeriod)
	sa.UpdateHedgeRatio(lookbackPeriod)
	sa.UpdateCorrelation(lookbackPeriod)
}

// GetStats 获取当前统计信息
func (sa *SpreadAnalyzer) GetStats() SpreadStats {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	return SpreadStats{
		CurrentSpread: sa.currentSpread,
		Mean:          sa.spreadMean,
		Std:           sa.spreadStd,
		ZScore:        sa.currentZScore,
		Correlation:   sa.correlation,
		HedgeRatio:    sa.hedgeRatio,
	}
}

// GetZScore 获取当前 z-score
func (sa *SpreadAnalyzer) GetZScore() float64 {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	return sa.currentZScore
}

// GetCorrelation 获取相关系数
func (sa *SpreadAnalyzer) GetCorrelation() float64 {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	return sa.correlation
}

// GetHedgeRatio 获取对冲比率
func (sa *SpreadAnalyzer) GetHedgeRatio() float64 {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	return sa.hedgeRatio
}

// GetCurrentSpread 获取当前 spread 值
func (sa *SpreadAnalyzer) GetCurrentSpread() float64 {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	return sa.currentSpread
}

// GetSpreadMean 获取 spread 均值
func (sa *SpreadAnalyzer) GetSpreadMean() float64 {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	return sa.spreadMean
}

// SetSpreadMean 设置 spread 均值（用于从 daily_init 恢复）
// C++: avgSpreadRatio_ori = std::stod(row["avgPx"]);
func (sa *SpreadAnalyzer) SetSpreadMean(mean float64) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	sa.spreadMean = mean
}

// GetSpreadStd 获取 spread 标准差
func (sa *SpreadAnalyzer) GetSpreadStd() float64 {
	sa.mu.RLock()
	defer sa.mu.RUnlock()
	return sa.spreadStd
}

// SetHedgeRatio 手动设置对冲比率
func (sa *SpreadAnalyzer) SetHedgeRatio(ratio float64) {
	sa.mu.Lock()
	defer sa.mu.Unlock()
	sa.hedgeRatio = ratio
}

// IsReady 检查是否有足够的历史数据
func (sa *SpreadAnalyzer) IsReady(lookbackPeriod int) bool {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	return sa.spreadSeries.Len() >= lookbackPeriod &&
		sa.price1Series.Len() >= lookbackPeriod &&
		sa.price2Series.Len() >= lookbackPeriod
}

// Reset 重置分析器
func (sa *SpreadAnalyzer) Reset() {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	sa.price1 = 0
	sa.price2 = 0
	sa.currentSpread = 0
	sa.spreadMean = 0
	sa.spreadStd = 0
	sa.currentZScore = 0
	sa.correlation = 0

	sa.price1Series.Clear()
	sa.price2Series.Clear()
	sa.spreadSeries.Clear()
}
