package strategy

import (
	"log"

	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/shm"
	"tbsrc-golang/pkg/types"
)

// MDCallBack 处理行情更新
// 参考: PairwiseArbStrategy.cpp:479-569
//
// C++ 逻辑:
//  1. 加载 tValue（tvar SHM）
//  2. 检查跨腿 maxLoss
//  3. 委托给两腿 MDCallBack
//  4. 计算价差 + EWA 更新（仅 leg1 行情时）
//  5. AVG_SPREAD_AWAY 安全检查
//  6. 调用 SendOrder()
func (pas *PairwiseArbStrategy) MDCallBack(inst *instrument.Instrument, md *shm.MarketUpdateNew) {
	// 识别是哪条腿
	isLeg1 := inst == pas.Inst1

	// C++: 委托给对应腿的 MDCallBack（更新 LTP、PNL 等）
	if isLeg1 {
		pas.Leg1.MDCallBack(inst, md)
	} else {
		pas.Leg2.MDCallBack(inst, md)
	}

	// 需要两腿都有有效行情才能计算价差
	if pas.Inst1.ValidBids == 0 || pas.Inst1.ValidAsks == 0 ||
		pas.Inst2.ValidBids == 0 || pas.Inst2.ValidAsks == 0 {
		return
	}

	// C++: 计算 mid 价格
	mid1 := (pas.Inst1.BidPx[0] + pas.Inst1.AskPx[0]) / 2
	mid2 := (pas.Inst2.BidPx[0] + pas.Inst2.AskPx[0]) / 2

	// C++: 更新价差 + EWA（仅 leg1 更新时刷新 EWA）
	// 参考: PairwiseArbStrategy.cpp:496-523
	valid := pas.Spread.Update(mid1, mid2, isLeg1)

	if !valid {
		// C++: AVG_SPREAD_AWAY 超限，触发 HandleSquareoff
		log.Printf("[PairwiseArb] AVG_SPREAD_AWAY exceeded: curr=%.4f avg=%.4f tick=%.2f limit=%d",
			pas.Spread.CurrSpread, pas.Spread.AvgSpread, pas.Spread.TickSize, pas.Spread.AvgSpreadAway)
		if pas.Active {
			pas.HandleSquareoff()
		}
		return
	}

	// C++: 检查跨腿 maxLoss
	// 参考: PairwiseArbStrategy.cpp:487-492
	maxLoss := pas.Thold1.MaxLoss
	if maxLoss > 0 {
		combinedPNL := pas.Leg1.State.NetPNL + pas.Leg2.State.NetPNL
		if combinedPNL < -maxLoss {
			log.Printf("[PairwiseArb] MaxLoss breached: combinedPNL=%.2f maxLoss=%.2f",
				combinedPNL, maxLoss)
			if pas.Active {
				pas.HandleSquareoff()
			}
			return
		}
	}

	// C++: 如果策略激活，调用 SendOrder
	if pas.Active {
		pas.SendOrder()
	}
}

// ORSCallBack 处理订单回报
// 参考: PairwiseArbStrategy.cpp:428-477
//
// C++ 逻辑:
//  1. 通过 orderID 查找属于哪条腿
//  2. Leg2: 先调 HandleAggOrder
//  3. 委托给对应腿的 ORSCallBack
//  4. Leg1 TRADE_CONFIRM: 重置 aggRepeat
//  5. 如果活跃且有未对冲头寸，调用 SendAggressiveOrder
func (pas *PairwiseArbStrategy) ORSCallBack(resp *shm.ResponseMsg) {
	orderID := resp.OrderID

	// C++: 查找属于哪条腿
	_, inLeg1 := pas.Leg1.Orders.OrdMap[orderID]
	_, inLeg2 := pas.Leg2.Orders.OrdMap[orderID]

	if inLeg1 {
		// C++: 委托给 leg1
		pas.Leg1.ORSCallBack(resp)

		// C++: TRADE_CONFIRM 时重置 agg_repeat
		if resp.Response_Type == shm.TRADE_CONFIRM {
			pas.AggRepeat = 1
			log.Printf("[PairwiseArb] Leg1 trade: orderID=%d price=%.2f qty=%d netpos_pass=%d",
				orderID, resp.Price, resp.Quantity, pas.Leg1.State.NetposPass)
		}
	} else if inLeg2 {
		// C++: 先处理 aggressive order 计数器
		pas.handleAggOrder(resp)

		// C++: 委托给 leg2
		pas.Leg2.ORSCallBack(resp)

		// C++: TRADE_CONFIRM 时重置 agg_repeat
		if resp.Response_Type == shm.TRADE_CONFIRM {
			pas.AggRepeat = 1
			log.Printf("[PairwiseArb] Leg2 trade: orderID=%d price=%.2f qty=%d netpos_agg=%d",
				orderID, resp.Price, resp.Quantity, pas.Leg2.State.NetposAgg)
		}
	} else {
		log.Printf("[PairwiseArb] unknown orderID=%d responseType=%d",
			orderID, resp.Response_Type)
		return
	}

	// C++: 如果活跃，尝试补对冲
	if pas.Active {
		pas.SendAggressiveOrder()
	}
}

// handleAggOrder 处理 leg2 aggressive order 的计数器
// 参考: PairwiseArbStrategy.cpp:402-426
//
// C++: 在终态事件（reject, cancel confirm, full fill）时
//      减少 buyAggOrder 或 sellAggOrder 计数器
func (pas *PairwiseArbStrategy) handleAggOrder(resp *shm.ResponseMsg) {
	ord, ok := pas.Leg2.Orders.OrdMap[resp.OrderID]
	if !ok {
		return
	}

	// 只处理 CROSS/MATCH 类型的订单
	if ord.OrdType != types.HitCross && ord.OrdType != types.HitMatch {
		return
	}

	isTerminal := false
	switch resp.Response_Type {
	case shm.TRADE_CONFIRM:
		// 仅完全成交时算终态
		if ord.OpenQty-resp.Quantity <= 0 {
			isTerminal = true
		}
	case shm.CANCEL_ORDER_CONFIRM:
		isTerminal = true
	case shm.ORS_REJECT, shm.RMS_REJECT, shm.NEW_ORDER_FREEZE:
		isTerminal = true
	}

	if isTerminal {
		if ord.Side == types.Buy {
			if pas.BuyAggOrder > 0 {
				pas.BuyAggOrder--
			}
		} else {
			if pas.SellAggOrder > 0 {
				pas.SellAggOrder--
			}
		}
	}
}
