package strategy

import (
	"time"

	"tbsrc-golang/pkg/execution"
	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/shm"
	"tbsrc-golang/pkg/types"
)

// SendOrder 核心下单逻辑
// 每次行情更新时调用，执行被动报价 + 对冲
// 参考: PairwiseArbStrategy.cpp:146-385
//
// C++ 执行阶段:
//  1. SetThresholds (动态阈值)
//  2. 撤销所有 CROSS/MATCH 订单
//  3. 撤销偏离均值的 Leg1 订单
//  4. 零价格保护
//  5. 多档报价循环
//  6. Leg2 对冲
func (pas *PairwiseArbStrategy) SendOrder() {
	inst1 := pas.Inst1
	inst2 := pas.Inst2
	thold1 := pas.Thold1

	// ---- Phase 1: 动态阈值 ----
	// C++: PairwiseArbStrategy::SetThresholds()
	// 参考: PairwiseArbStrategy.cpp:902-947
	// 注意：PairwiseArb 有自己的 SetThresholds，使用 NetposPass（被动持仓）
	// 而非通用的 ExecutionStrategy::SetThresholds 使用 Netpos（总持仓）
	pas.setThresholds()

	state1 := pas.Leg1.State
	bidPlace := state1.TholdBidPlace
	bidRemove := state1.TholdBidRemove
	askPlace := state1.TholdAskPlace
	askRemove := state1.TholdAskRemove

	// C++: 所有四个阈值必须有效
	if bidPlace == -1 || bidRemove == -1 || askPlace == -1 || askRemove == -1 {
		return
	}

	avgSpread := pas.Spread.AvgSpread

	// ---- Phase 2: 撤销所有 CROSS/MATCH 订单 ----
	// C++: cancel all cross/match orders in both legs
	// 参考: PairwiseArbStrategy.cpp:188-203
	pas.cancelCrossOrders(pas.Leg1)
	pas.cancelCrossOrders(pas.Leg2)

	// ---- Phase 3: 撤销偏离均值的 Leg1 订单 ----
	// 参考: PairwiseArbStrategy.cpp:205-228
	pas.cancelOutOfRangeOrders(avgSpread, bidRemove, askRemove)

	// ---- Phase 4: 零价格保护 ----
	// C++: if any best bid/ask is zero, return
	// 参考: PairwiseArbStrategy.cpp:230-231
	if inst1.BidPx[0] == 0 || inst1.AskPx[0] == 0 ||
		inst2.BidPx[0] == 0 || inst2.AskPx[0] == 0 {
		return
	}

	// ---- Phase 5: 多档报价循环 ----
	// 参考: PairwiseArbStrategy.cpp:235-346
	for level := int32(0); level < pas.MaxQuoteLevel; level++ {
		if level >= int32(instrument.BookDepth) {
			break
		}
		if inst1.BidPx[level] == 0 || inst1.AskPx[level] == 0 {
			break
		}

		// C++: LongSpreadRatio1 = leg1.bidPx[level] - leg2.bidPx[0]
		longSpread := inst1.BidPx[level] - inst2.BidPx[0]
		// C++: ShortSpreadRatio1 = leg1.askPx[level] - leg2.askPx[0]
		shortSpread := inst1.AskPx[level] - inst2.AskPx[0]

		// ---- ASK (sell) placement ----
		// C++: if ShortSpreadRatio1 > avgSpreadRatio + m_tholdAskPlace
		// 参考: PairwiseArbStrategy.cpp:242-294
		if shortSpread > avgSpread+askPlace {
			askPrice := inst1.AskPx[level]
			ordType := types.HitStandard

			// C++: GetAskPrice_first(price, ordType, level)
			askPrice, ordType = pas.GetAskPrice(askPrice, ordType, level)

			// C++: 检查持仓限制
			// C++: if (m_netpos_pass * -1 < m_tholdAskMaxPos)
			netposPass := pas.Leg1.State.NetposPass
			tholdAskMaxPos := state1.TholdAskMaxPos
			if tholdAskMaxPos == 0 {
				tholdAskMaxPos = state1.TholdMaxPos
			}

			if tholdAskMaxPos == 0 || -netposPass < tholdAskMaxPos {
				// C++: if (sellOpenOrders > SUPPORTING_ORDERS || sellOpenQty + -1*netpos_pass >= tholdAskMaxPos)
				if state1.SellOpenOrders > thold1.SupportingOrders ||
					int32(state1.SellOpenQty)+(-netposPass) >= tholdAskMaxPos {
					// 找最差的 ask（价格最高），如果新价更好则撤最差的
					pas.cancelWorstAskIfBetter(askPrice)
				} else {
					pas.Leg1.SendAskOrder2(shm.NEWORDER, level, askPrice, ordType, 0, 0, 0)
				}
			} else {
				// C++: 持仓超限，撤所有 ask
				for _, ord := range pas.Leg1.Orders.AskMap {
					pas.Leg1.Orders.SendCancelOrderByID(pas.Inst1, ord.OrderID)
				}
			}
		}

		// ---- BID (buy) placement ----
		// C++: if LongSpreadRatio1 < avgSpreadRatio - m_tholdBidPlace
		// 参考: PairwiseArbStrategy.cpp:297-345
		if longSpread < avgSpread-bidPlace {
			bidPrice := inst1.BidPx[level]
			ordType := types.HitStandard

			// C++: GetBidPrice_first(price, ordType, level)
			bidPrice, ordType = pas.GetBidPrice(bidPrice, ordType, level)

			// C++: 检查持仓限制
			// C++: if (m_netpos_pass < m_tholdBidMaxPos)
			netposPass := pas.Leg1.State.NetposPass
			tholdBidMaxPos := state1.TholdBidMaxPos
			if tholdBidMaxPos == 0 {
				tholdBidMaxPos = state1.TholdMaxPos
			}

			if tholdBidMaxPos == 0 || netposPass < tholdBidMaxPos {
				// C++: if (buyOpenOrders > SUPPORTING_ORDERS || buyOpenQty + netpos_pass >= tholdBidMaxPos)
				if state1.BuyOpenOrders > thold1.SupportingOrders ||
					int32(state1.BuyOpenQty)+netposPass >= tholdBidMaxPos {
					// 找最差的 bid（价格最低），如果新价更好则撤最差的
					pas.cancelWorstBidIfBetter(bidPrice)
				} else {
					pas.Leg1.SendBidOrder2(shm.NEWORDER, level, bidPrice, ordType, 0, 0, 0)
				}
			} else {
				// C++: 持仓超限，撤所有 bid
				for _, ord := range pas.Leg1.Orders.BidMap {
					pas.Leg1.Orders.SendCancelOrderByID(pas.Inst1, ord.OrderID)
				}
			}
		}
	}

	// ---- Phase 6: Leg2 对冲 ----
	// 参考: PairwiseArbStrategy.cpp:348-375
	pas.hedgeLeg2()
}

// cancelCrossOrders 撤销一条腿上所有 CROSS/MATCH 订单
// 参考: PairwiseArbStrategy.cpp:188-203
// 使用 SendCancelOrderByIDForce 绕过 CROSS 保护（这里是主动撤销 CROSS 订单的场景）
func (pas *PairwiseArbStrategy) cancelCrossOrders(leg *execution.LegManager) {
	for _, ord := range leg.Orders.OrdMap {
		if ord.OrdType == types.HitCross || ord.OrdType == types.HitMatch {
			leg.Orders.SendCancelOrderByIDForce(leg.Inst, ord.OrderID)
		}
	}
}

// cancelOutOfRangeOrders 撤销偏离均值的 Leg1 bid/ask 订单
// 参考: PairwiseArbStrategy.cpp:205-228
//
// C++ 逻辑:
//
//	Bid: if (ourBidPx - leg2.bid[0]) > avgSpread - bidRemove → cancel
//	Ask: if (ourAskPx - leg2.ask[0]) < avgSpread + askRemove → cancel
func (pas *PairwiseArbStrategy) cancelOutOfRangeOrders(avgSpread, bidRemove, askRemove float64) {
	inst2 := pas.Inst2

	// C++: cancel bid orders where spread is too tight
	for _, ord := range pas.Leg1.Orders.BidMap {
		longSpread := ord.Price - inst2.BidPx[0]
		if longSpread > avgSpread-bidRemove {
			if ord.Status == types.StatusNewConfirm ||
				ord.Status == types.StatusModifyConfirm ||
				ord.Status == types.StatusModifyReject {
				pas.Leg1.Orders.SendCancelOrderByID(pas.Inst1, ord.OrderID)
			}
		}
	}

	// C++: cancel ask orders where spread is too tight
	for _, ord := range pas.Leg1.Orders.AskMap {
		shortSpread := ord.Price - inst2.AskPx[0]
		if shortSpread < avgSpread+askRemove {
			if ord.Status == types.StatusNewConfirm ||
				ord.Status == types.StatusModifyConfirm ||
				ord.Status == types.StatusModifyReject {
				pas.Leg1.Orders.SendCancelOrderByID(pas.Inst1, ord.OrderID)
			}
		}
	}
}

// hedgeLeg2 对冲 leg2
// 参考: PairwiseArbStrategy.cpp:348-385
//
// C++ 逻辑:
//
//	1. 计算 net exposure = netpos_pass + netpos_agg + pendingNetposAgg
//	2. 检查 agg order 数量限制 (SUPPORTING_ORDERS)
//	3. 检查 last_agg_side + 100ms 冷却时间
//	4. 发送 SendAskOrder2/SendBidOrder2 with CROSS ordType
func (pas *PairwiseArbStrategy) hedgeLeg2() {
	pendingAgg := pas.CalcPendingNetposAgg()
	exposure := pas.Leg1.State.NetposPass + pas.Leg2.State.NetposAgg + pendingAgg
	if exposure == 0 {
		return
	}

	inst2 := pas.Inst2
	thold1 := pas.Thold1

	// C++: 获取当前时间（毫秒）
	nowMS := uint64(time.Now().UnixMilli())

	if exposure > 0 {
		// C++: NET LONG — 需要在 leg2 卖出
		// C++: sellAggOrder <= SUPPORTING_ORDERS
		if pas.SellAggOrder > thold1.SupportingOrders {
			return
		}
		// C++: last_agg_side != SELL || (now - last_agg_time > 100ms)
		if pas.LastAggSide == types.Sell && nowMS-pas.LastAggTS <= 100 {
			return
		}
		// C++: price = bidPx[0] - tickSize
		sellPrice := inst2.BidPx[0] - inst2.TickSize
		pas.Leg2.SendAskOrder2(shm.NEWORDER, 0, sellPrice, types.HitCross, exposure, 0, 0)
		pas.SellAggOrder++
		pas.LastAggTS = nowMS
		pas.LastAggSide = types.Sell
	} else {
		// C++: NET SHORT — 需要在 leg2 买入
		if pas.BuyAggOrder > thold1.SupportingOrders {
			return
		}
		if pas.LastAggSide == types.Buy && nowMS-pas.LastAggTS <= 100 {
			return
		}
		// C++: price = askPx[0] + tickSize
		buyPrice := inst2.AskPx[0] + inst2.TickSize
		pas.Leg2.SendBidOrder2(shm.NEWORDER, 0, buyPrice, types.HitCross, -exposure, 0, 0)
		pas.BuyAggOrder++
		pas.LastAggTS = nowMS
		pas.LastAggSide = types.Buy
	}
}

// cancelWorstAskIfBetter 如果新价格比最差 ask 更好（更低），撤最差 ask
func (pas *PairwiseArbStrategy) cancelWorstAskIfBetter(newPrice float64) {
	var worstPrice float64
	var worstOrd *types.OrderStats
	for price, ord := range pas.Leg1.Orders.AskMap {
		if worstOrd == nil || price > worstPrice {
			worstPrice = price
			worstOrd = ord
		}
	}
	if worstOrd != nil && newPrice < worstPrice {
		// 已有更差的 ask，先撤它
		_, exists := pas.Leg1.Orders.AskMap[newPrice]
		if !exists {
			pas.Leg1.Orders.SendCancelOrderByID(pas.Inst1, worstOrd.OrderID)
		}
	}
}

// cancelWorstBidIfBetter 如果新价格比最差 bid 更好（更高），撤最差 bid
func (pas *PairwiseArbStrategy) cancelWorstBidIfBetter(newPrice float64) {
	var worstPrice float64
	var worstOrd *types.OrderStats
	first := true
	for price, ord := range pas.Leg1.Orders.BidMap {
		if first || price < worstPrice {
			worstPrice = price
			worstOrd = ord
			first = false
		}
	}
	if worstOrd != nil && newPrice > worstPrice {
		_, exists := pas.Leg1.Orders.BidMap[newPrice]
		if !exists {
			pas.Leg1.Orders.SendCancelOrderByID(pas.Inst1, worstOrd.OrderID)
		}
	}
}
