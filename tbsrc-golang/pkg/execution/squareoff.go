package execution

import (
	"log"
	"math"

	"tbsrc-golang/pkg/instrument"
)

// CheckSquareoff 检查是否需要触发平仓
// 参考: tbsrc/Strategies/ExecutionStrategy.cpp:2150-2186, 2279-2339
//
// C++ 逻辑:
//   阶段 1: 检查激进平仓时间 → m_aggFlat = true
//   阶段 2: 检查标准退出条件（时间/亏损/订单数/成交量）→ m_onExit = true
//   阶段 3: UPNL_LOSS / STOP_LOSS 检查
//   阶段 4: Auto-resume（15 分钟冷却后恢复）
func (s *ExecutionState) CheckSquareoff(currentTimeNs uint64, maxLoss, upnlLoss, stopLoss float64) {
	// C++: Phase 1 — aggressive flat time
	if s.EndTimeAggEpoch > 0 && currentTimeNs >= s.EndTimeAggEpoch && !s.AggFlat {
		s.AggFlat = true
		s.OnExit = true
		s.OnCancel = true
		s.OnFlat = true
		log.Printf("[Squareoff] aggressive flat triggered at time=%d", currentTimeNs)
	}

	// C++: Phase 4 — auto-resume after 15 minutes of stop loss
	// 参考: ExecutionStrategy.cpp:2313-2320
	if s.OnStopLoss && s.StopLossTS > 0 && currentTimeNs-s.StopLossTS >= 900_000_000_000 {
		s.OnStopLoss = false
		s.OnExit = false
		s.OnCancel = false
		s.OnFlat = false
		log.Printf("[Squareoff] auto-resume after stop loss cool-off (15 min)")
		return
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
	if maxLoss > 0 && s.NetPNL < -maxLoss {
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
		return
	}

	// C++: Phase 3 — UPNL_LOSS / STOP_LOSS per-leg
	// 参考: ExecutionStrategy.cpp:2279-2301
	s.checkStopLoss(currentTimeNs, upnlLoss, stopLoss)
}

// checkStopLoss 检查未实现亏损和回撤止损
// 参考: ExecutionStrategy.cpp:2279-2301
//
// C++ 逻辑:
//   1. unrealisedPNL < -UPNL_LOSS → trigger flat + onStopLoss
//   2. drawdown < -STOP_LOSS → trigger flat + onStopLoss
//   3. onStopLoss 时 thresholds doubled（允许更宽的价差范围平仓）
func (s *ExecutionState) checkStopLoss(currentTimeNs uint64, upnlLoss, stopLoss float64) {
	if s.Netpos == 0 {
		return // 无持仓，无需检查
	}

	triggered := false
	reason := ""

	// C++: UPNL_LOSS — 未实现亏损限制
	if upnlLoss > 0 && s.UnrealisedPNL < -upnlLoss {
		triggered = true
		reason = "upnl_loss"
	}

	// C++: STOP_LOSS — 回撤限制
	if stopLoss > 0 && s.Drawdown < -stopLoss {
		triggered = true
		reason = "stop_loss"
	}

	if triggered {
		s.OnFlat = true
		s.OnCancel = true
		s.OnStopLoss = true
		s.StopLossTS = currentTimeNs
		log.Printf("[Squareoff] stop loss triggered: reason=%s upnl=%.2f drawdown=%.2f",
			reason, s.UnrealisedPNL, s.Drawdown)
	}
}

// CheckRejectLimit 检查拒绝次数是否超限
// 参考: ExecutionStrategy.cpp:432-481
const RejectLimit = 200

func (s *ExecutionState) CheckRejectLimit() bool {
	return s.RejectCount >= RejectLimit
}

// HandleRMSReject 处理 RMS 拒绝，触发激进平仓并减半订单量
// 参考: ExecutionStrategy.cpp:1069-1080
func (s *ExecutionState) HandleRMSReject(inst *instrument.Instrument) {
	s.AggFlat = true
	if s.RmsQty > 0 {
		s.RmsQty /= 2
	}
	if s.RmsQty <= 0 {
		s.RmsQty = int32(math.Max(1, inst.LotSize))
	}
	log.Printf("[Squareoff] RMS reject: aggFlat=true rmsQty=%d", s.RmsQty)
}
