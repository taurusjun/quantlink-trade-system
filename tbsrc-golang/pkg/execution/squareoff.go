package execution

import (
	"log"
)

// CheckSquareoff 检查是否需要触发平仓
// 参考: tbsrc/Strategies/ExecutionStrategy.cpp:2150-2186
//
// C++ 逻辑（两阶段）:
//   阶段 1: 检查激进平仓时间 → m_aggFlat = true
//   阶段 2: 检查标准退出条件（时间/亏损/订单数/成交量）→ m_onExit = true
func (s *ExecutionState) CheckSquareoff(currentTimeNs uint64) {
	// C++: Phase 1 — aggressive flat time
	if s.EndTimeAggEpoch > 0 && currentTimeNs >= s.EndTimeAggEpoch && !s.AggFlat {
		s.AggFlat = true
		s.OnExit = true
		s.OnCancel = true
		s.OnFlat = true
		log.Printf("[Squareoff] aggressive flat triggered at time=%d", currentTimeNs)
	}

	// C++: Phase 2 — standard exit conditions
	if s.OnExit {
		return // 已经在退出中
	}

	triggered := false
	reason := ""

	// 检查时间
	if s.EndTimeEpoch > 0 && currentTimeNs >= s.EndTimeEpoch {
		triggered = true
		reason = "end_time"
	}

	// 检查最大亏损
	if s.NetPNL < -s.MaxLossThreshold() {
		triggered = true
		reason = "max_loss"
	}

	// 检查最大订单数
	if s.MaxOrderCount > 0 && uint64(s.OrderCount) >= s.MaxOrderCount {
		triggered = true
		reason = "max_orders"
	}

	// 检查最大成交量
	if s.MaxTradedQty > 0 && (s.BuyTotalQty >= s.MaxTradedQty || s.SellTotalQty >= s.MaxTradedQty) {
		triggered = true
		reason = "max_traded_qty"
	}

	if triggered {
		s.OnExit = true
		s.OnCancel = true
		s.OnFlat = true
		log.Printf("[Squareoff] exit triggered: reason=%s netPNL=%.2f orders=%d",
			reason, s.NetPNL, s.OrderCount)
	}
}

// MaxLossThreshold 返回最大亏损阈值
// 用于外部设置（从 ThresholdSet.MaxLoss 获取）
func (s *ExecutionState) MaxLossThreshold() float64 {
	// 默认极大值（不触发）
	return 100000000000
}
