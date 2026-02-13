package strategy

import (
	"log"
	"time"

	"tbsrc-golang/pkg/shm"
	"tbsrc-golang/pkg/types"
)

// SendAggressiveOrder 发送主动对冲单（leg2）
// 在每次 ORS 回调后调用，补齐未对冲的敞口
// 参考: PairwiseArbStrategy.cpp:701-800
//
// C++ 逻辑:
//  1. 计算 net exposure = netpos_pass + netpos_agg + pendingNetposAgg
//  2. exposure > 0: 需要在 leg2 卖出
//  3. exposure < 0: 需要在 leg2 买入
//  4. 激进重试阶梯:
//     - 首次或 >500ms: 以 bidPx[0] / askPx[0] 下单
//     - retry 1-2: 价格偏移 tickSize * repeat
//     - retry 3: 价格偏移 tickSize * SLOP (大跳)
//     - retry >3: HandleSquareoff() 平仓
func (pas *PairwiseArbStrategy) SendAggressiveOrder() {
	inst2 := pas.Inst2
	thold2 := pas.Thold2

	pendingAgg := pas.CalcPendingNetposAgg()
	netExposure := pas.Leg1.State.NetposPass + pas.Leg2.State.NetposAgg + pendingAgg

	// C++: 获取当前时间（毫秒）
	nowMS := uint64(time.Now().UnixMilli())

	if netExposure > 0 && pas.SellAggOrder <= thold2.SupportingOrders {
		// C++: NET LONG — 需要在 leg2 卖出
		qty := netExposure

		if pas.LastAggSide != types.Sell ||
			(pas.LastAggSide == types.Sell && nowMS-pas.LastAggTS > 500) {
			// C++: 首次或 >500ms — 以 bidPx[0] 卖出（吃单）
			pas.Leg2.SendAskOrder2(shm.NEWORDER, 0, inst2.BidPx[0], types.HitCross, qty, 0, 0)
			pas.SellAggOrder++
			pas.LastAggTS = nowMS
			pas.LastAggSide = types.Sell
		} else {
			// C++: 重试阶梯
			if pas.AggRepeat > 3 {
				// C++: 超过最大重试次数 → 平仓
				log.Printf("[PairwiseArb] aggressive retry exceeded (repeat=%d), squareoff", pas.AggRepeat)
				pas.handleSquareoffLocked()
			} else {
				// C++: 计算激进价格
				var aggPrice float64
				tickSize := inst2.TickSize
				if pas.AggRepeat < 3 {
					// C++: bidPx[0] - tickSize * repeat
					aggPrice = inst2.BidPx[0] - tickSize*float64(pas.AggRepeat)
				} else {
					// C++: bidPx[0] - tickSize * SLOP
					aggPrice = inst2.BidPx[0] - tickSize*float64(thold2.Slop)
				}
				ok := pas.Leg2.SendAskOrder2(shm.NEWORDER, 0, aggPrice, types.HitCross, qty, 0, 0)
				if ok {
					pas.AggRepeat++
					pas.SellAggOrder++
					pas.LastAggTS = nowMS
					pas.LastAggSide = types.Sell
				}
			}
		}
	} else if netExposure < 0 && pas.BuyAggOrder <= thold2.SupportingOrders {
		// C++: NET SHORT — 需要在 leg2 买入
		qty := -netExposure

		if pas.LastAggSide != types.Buy ||
			(pas.LastAggSide == types.Buy && nowMS-pas.LastAggTS > 500) {
			// C++: 首次或 >500ms — 以 askPx[0] 买入（吃单）
			pas.Leg2.SendBidOrder2(shm.NEWORDER, 0, inst2.AskPx[0], types.HitCross, qty, 0, 0)
			pas.BuyAggOrder++
			pas.LastAggTS = nowMS
			pas.LastAggSide = types.Buy
		} else {
			// C++: 重试阶梯
			if pas.AggRepeat > 3 {
				log.Printf("[PairwiseArb] aggressive retry exceeded (repeat=%d), squareoff", pas.AggRepeat)
				pas.handleSquareoffLocked()
			} else {
				var aggPrice float64
				tickSize := inst2.TickSize
				if pas.AggRepeat < 3 {
					// C++: askPx[0] + tickSize * repeat
					aggPrice = inst2.AskPx[0] + tickSize*float64(pas.AggRepeat)
				} else {
					// C++: askPx[0] + tickSize * SLOP
					aggPrice = inst2.AskPx[0] + tickSize*float64(thold2.Slop)
				}
				ok := pas.Leg2.SendBidOrder2(shm.NEWORDER, 0, aggPrice, types.HitCross, qty, 0, 0)
				if ok {
					pas.AggRepeat++
					pas.BuyAggOrder++
					pas.LastAggTS = nowMS
					pas.LastAggSide = types.Buy
				}
			}
		}
	}
}

// CalcPendingNetposAgg 计算 leg2 待成交的净持仓
// 参考: PairwiseArbStrategy.cpp:688-699
//
// C++ 逻辑:
//
//	遍历 m_ordMap2，仅统计 CROSS/MATCH 类型订单
//	BUY → +openQty, SELL → -openQty
func (pas *PairwiseArbStrategy) CalcPendingNetposAgg() int32 {
	var pending int32
	for _, ord := range pas.Leg2.Orders.OrdMap {
		if ord.OrdType == types.HitCross || ord.OrdType == types.HitMatch {
			if ord.Side == types.Buy {
				pending += ord.OpenQty
			} else {
				pending -= ord.OpenQty
			}
		}
	}
	return pending
}
