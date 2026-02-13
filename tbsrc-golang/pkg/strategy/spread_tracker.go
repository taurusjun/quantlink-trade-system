package strategy

import (
	"math"
)

// SpreadTracker 跟踪配对套利的价差 EWA
// 对应 C++ PairwiseArbStrategy 中的 avgSpreadRatio_ori, avgSpreadRatio, currSpreadRatio
// 参考: tbsrc/Strategies/PairwiseArbStrategy.cpp:496-523
type SpreadTracker struct {
	AvgSpreadOri  float64 // C++: avgSpreadRatio_ori — EWA of spread (persisted to daily_init)
	AvgSpread     float64 // C++: avgSpreadRatio = avgSpreadRatio_ori + tValue
	CurrSpread    float64 // C++: currSpreadRatio = mid1 - mid2
	TValue        float64 // C++: tValue — external adjustment from tvar SHM
	Alpha         float64 // C++: m_thold_first->ALPHA — EWA decay factor
	TickSize      float64 // for AVG_SPREAD_AWAY check
	AvgSpreadAway int32   // C++: m_thold_first->AVG_SPREAD_AWAY (default 20)
	IsValid       bool    // C++: is_valid_mkdata — false if spread deviates too far
	Initialized   bool    // false until first Update call
}

// NewSpreadTracker 创建 SpreadTracker
func NewSpreadTracker(alpha float64, tickSize float64, avgSpreadAway int32) *SpreadTracker {
	if avgSpreadAway <= 0 {
		avgSpreadAway = 20 // C++ default
	}
	return &SpreadTracker{
		Alpha:         alpha,
		TickSize:      tickSize,
		AvgSpreadAway: avgSpreadAway,
		IsValid:       true,
	}
}

// Seed 从 daily_init 文件初始化 EWA 种子值
// 参考: PairwiseArbStrategy.cpp:31 — avgSpreadRatio_ori 从文件加载
func (st *SpreadTracker) Seed(avgSpreadOri float64) {
	st.AvgSpreadOri = avgSpreadOri
	st.AvgSpread = avgSpreadOri + st.TValue
	st.Initialized = true
}

// SetTValue 更新外部调整值
// C++: tValue = m_tvar->load(); avgSpreadRatio = avgSpreadRatio_ori + tValue
// 参考: PairwiseArbStrategy.cpp:482-486
func (st *SpreadTracker) SetTValue(v float64) {
	st.TValue = v
	st.AvgSpread = st.AvgSpreadOri + st.TValue
}

// Update 更新价差，返回 true 如果价差有效
// 参考: PairwiseArbStrategy.cpp:496-523
//
// C++ 逻辑:
//  1. currSpreadRatio = (bid1+ask1)/2 - (bid2+ask2)/2
//  2. if |curr - avg| > tickSize * AVG_SPREAD_AWAY: invalid
//  3. avgSpreadRatio_ori = (1-ALPHA)*avgSpreadRatio_ori + ALPHA*currSpreadRatio
//  4. avgSpreadRatio = avgSpreadRatio_ori + tValue
//
// isLeg1Update: 仅在 leg1 行情更新时才刷新 EWA（C++ 行为）
func (st *SpreadTracker) Update(mid1, mid2 float64, isLeg1Update bool) bool {
	st.CurrSpread = mid1 - mid2

	// 首次更新时用当前价差初始化 EWA
	if !st.Initialized {
		st.AvgSpreadOri = st.CurrSpread
		st.AvgSpread = st.AvgSpreadOri + st.TValue
		st.Initialized = true
	}

	// C++: AVG_SPREAD_AWAY 安全检查
	// 参考: PairwiseArbStrategy.cpp:506-517
	deviation := math.Abs(st.CurrSpread - st.AvgSpread)
	maxDeviation := st.TickSize * float64(st.AvgSpreadAway)
	if maxDeviation > 0 && deviation > maxDeviation {
		st.IsValid = false
		return false
	}
	st.IsValid = true

	// C++: EWA 仅在 leg1 行情更新时刷新
	// 参考: PairwiseArbStrategy.cpp:519-523
	if isLeg1Update && st.Alpha > 0 {
		st.AvgSpreadOri = (1-st.Alpha)*st.AvgSpreadOri + st.Alpha*st.CurrSpread
		st.AvgSpread = st.AvgSpreadOri + st.TValue
	}

	return true
}

// Deviation 返回当前价差相对于 EWA 的偏差
func (st *SpreadTracker) Deviation() float64 {
	return st.CurrSpread - st.AvgSpread
}
