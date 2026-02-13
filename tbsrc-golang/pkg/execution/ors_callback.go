package execution

import (
	"log"

	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/shm"
	"tbsrc-golang/pkg/types"
)

// ProcessORSResponse 处理 ORS 回调，分派到对应处理函数
// 参考: tbsrc/Strategies/ExecutionStrategy.cpp:951-1154 ORSCallBack()
func (om *OrderManager) ProcessORSResponse(resp *shm.ResponseMsg, inst *instrument.Instrument) {
	ord, ok := om.OrdMap[resp.OrderID]
	if !ok {
		log.Printf("[ORS] unknown orderID=%d responseType=%d", resp.OrderID, resp.Response_Type)
		return
	}

	switch resp.Response_Type {
	case shm.NEW_ORDER_CONFIRM:
		om.processNewOrderConfirm(resp, ord, inst)

	case shm.MODIFY_ORDER_CONFIRM:
		om.processModifyConfirm(resp, ord)

	case shm.CANCEL_ORDER_CONFIRM:
		om.processCancelConfirm(resp, ord, inst)

	case shm.TRADE_CONFIRM:
		// C++: keep status as-is if pending modify/cancel
		if ord.Status != types.StatusModifyOrder && ord.Status != types.StatusCancelOrder {
			ord.Status = types.StatusNewConfirm
		}
		om.processTrade(resp, ord, inst)

	case shm.ORS_REJECT, shm.RMS_REJECT, shm.SIM_REJECT:
		om.processReject(resp, ord)

	case shm.NEW_ORDER_FREEZE, shm.ORDER_ERROR:
		om.processNewReject(resp, ord)

	case shm.MODIFY_ORDER_REJECT:
		om.processModifyReject(resp, ord)

	case shm.CANCEL_ORDER_REJECT:
		om.processCancelReject(resp, ord, inst)

	default:
		log.Printf("[ORS] unhandled responseType=%d orderID=%d", resp.Response_Type, resp.OrderID)
	}
}

// processNewOrderConfirm 处理新订单确认
// 参考: ExecutionStrategy.cpp:951-980
func (om *OrderManager) processNewOrderConfirm(resp *shm.ResponseMsg, ord *types.OrderStats, inst *instrument.Instrument) {
	ord.Status = types.StatusNewConfirm
	om.State.RejectCount = 0
	om.State.ConfirmCount++
	ord.QuantBehind = 0

	// C++: 估算 quantAhead — 扫描行情簿找到匹配价格
	if ord.Side == types.Buy {
		for i := int32(0); i < inst.ValidBids; i++ {
			if inst.BidPx[i] == ord.Price {
				ord.QuantAhead = inst.BidQty[i]
				break
			}
		}
	} else {
		for i := int32(0); i < inst.ValidAsks; i++ {
			if inst.AskPx[i] == ord.Price {
				ord.QuantAhead = inst.AskQty[i]
				break
			}
		}
	}

	log.Printf("[ORS] NEW_CONFIRM orderID=%d side=%d price=%.2f qty=%d",
		resp.OrderID, ord.Side, ord.Price, ord.Qty)
}

// processModifyConfirm 处理改单确认
// 参考: ExecutionStrategy.cpp:1888-1910
//
// C++ 逻辑:
//   1. 从 priceMap 移除旧价格
//   2. 更新 price/qty/openQty 为新值
//   3. modifywait = false
func (om *OrderManager) processModifyConfirm(resp *shm.ResponseMsg, ord *types.OrderStats) {
	// C++: 移除旧价格
	if ord.Side == types.Buy {
		delete(om.BidMap, ord.Price)
	} else {
		delete(om.AskMap, ord.Price)
	}

	// C++: 更新为新值
	ord.Price = ord.NewPrice
	ord.Qty = ord.NewQty
	ord.OpenQty = ord.NewQty
	ord.ModifyWait = false
	ord.Status = types.StatusModifyConfirm

	om.State.RejectCount = 0
	om.State.ConfirmCount++

	log.Printf("[ORS] MODIFY_CONFIRM orderID=%d newPrice=%.2f newQty=%d",
		resp.OrderID, ord.Price, ord.Qty)
}

// processCancelConfirm 处理撤单确认
// 参考: ExecutionStrategy.cpp:1912-1981
//
// C++ 逻辑:
//   1. 减少 buyOpenQty/sellOpenQty
//   2. 设置 status = CANCEL_CONFIRM
//   3. 减少 openQty，记录 cxlQty
//   4. openQty <= 0 时 RemoveOrder
func (om *OrderManager) processCancelConfirm(resp *shm.ResponseMsg, ord *types.OrderStats, _ *instrument.Instrument) {
	qty := ord.OpenQty // C++: 默认用本地跟踪的 openQty

	if ord.Side == types.Buy {
		om.State.BuyOpenQty -= float64(qty)
		// C++: 如果正在修改，清理新价格的 priceMap 条目
		if ord.Status == types.StatusModifyOrder {
			delete(om.BidMap, ord.NewPrice)
		}
	} else {
		om.State.SellOpenQty -= float64(qty)
		if ord.Status == types.StatusModifyOrder {
			delete(om.AskMap, ord.NewPrice)
		}
	}

	ord.Status = types.StatusCancelConfirm
	ord.OpenQty -= qty
	ord.CxlQty = qty

	om.State.RejectCount = 0
	om.State.ConfirmCount++
	om.State.CancelConfirmCnt++

	if ord.OpenQty <= 0 {
		om.RemoveOrder(resp.OrderID)
	}

	log.Printf("[ORS] CANCEL_CONFIRM orderID=%d cxlQty=%d", resp.OrderID, qty)
}

// processTrade 处理成交确认
// 参考: ExecutionStrategy.cpp:1983-2122
//
// C++ 关键逻辑:
//   1. 减少 openQty，增加 doneQty
//   2. 更新 buyTotalQty/sellTotalQty 和 value
//   3. 计算 netpos = buyTotalQty - sellTotalQty
//   4. 跟踪 netpos_pass (STANDARD) 和 netpos_agg (CROSS/MATCH)
//   5. 计算手续费 transValue
//   6. netpos == 0 时: 结算 realisedPNL，重置当前腿
//   7. CalculatePNL()
//   8. openQty == 0 时: 标记 TRADED，RemoveOrder
func (om *OrderManager) processTrade(resp *shm.ResponseMsg, ord *types.OrderStats, inst *instrument.Instrument) {
	tradeQty := resp.Quantity
	tradePrice := resp.Price

	ord.OpenQty -= tradeQty
	ord.DoneQty += tradeQty

	om.State.LastTrade = true
	om.State.LastTradePx = tradePrice

	var factor int32
	if ord.Side == types.Buy {
		om.State.LastTradeSide = true

		// C++: m_buyTotalValue += price * qty
		om.State.BuyTotalValue += tradePrice * float64(tradeQty)
		om.State.BuyValue += tradePrice * float64(tradeQty)

		// C++: m_buyTotalQty += qty
		om.State.BuyTotalQty += float64(tradeQty)
		om.State.BuyAvgPrice = om.State.BuyTotalValue / om.State.BuyTotalQty

		// C++: m_buyQty += qty (当前腿)
		om.State.BuyQty += float64(tradeQty)
		if om.State.BuyQty > 0 {
			om.State.BuyPrice = om.State.BuyValue / om.State.BuyQty
		}

		om.State.BuyOpenQty -= float64(tradeQty)
		factor = 1
	} else {
		om.State.LastTradeSide = false

		om.State.SellTotalValue += tradePrice * float64(tradeQty)
		om.State.SellValue += tradePrice * float64(tradeQty)

		om.State.SellTotalQty += float64(tradeQty)
		om.State.SellAvgPrice = om.State.SellTotalValue / om.State.SellTotalQty

		om.State.SellQty += float64(tradeQty)
		if om.State.SellQty > 0 {
			om.State.SellPrice = om.State.SellValue / om.State.SellQty
		}

		om.State.SellOpenQty -= float64(tradeQty)
		factor = -1
	}

	// C++: 按 ordType 跟踪 netpos_pass / netpos_agg
	switch ord.OrdType {
	case types.HitStandard:
		om.State.NetposPass += factor * tradeQty
	case types.HitCross, types.HitMatch:
		om.State.NetposAgg += factor * tradeQty
	case types.HitImprove:
		om.State.ImproveCount++
	}

	if ord.OrdType == types.HitCross {
		om.State.CrossCount++
	}

	om.State.TradeCount++

	// C++: m_netpos = m_buyTotalQty - m_sellTotalQty
	om.State.Netpos = int32(om.State.BuyTotalQty - om.State.SellTotalQty)

	// C++: 计算手续费
	om.State.TransValue = (om.State.BuyExchTx*om.State.BuyValue+om.State.SellExchTx*om.State.SellValue)*inst.PriceMultiplier +
		(om.State.BuyExchContractTx*om.State.BuyQty + om.State.SellExchContractTx*om.State.SellQty)

	// C++: netpos == 0 时结算
	// 参考: ExecutionStrategy.cpp:2052-2072
	if om.State.Netpos == 0 {
		// C++: m_realisedPNL = (sellTotalValue - buyTotalValue) * priceMultiplier
		om.State.RealisedPNL = (om.State.SellTotalValue - om.State.BuyTotalValue) * inst.PriceMultiplier

		// C++: 重新计算 session 级手续费 transTotalValue
		// 参考: ExecutionStrategy.cpp:2067-2072
		om.State.TransTotalValue = om.State.BuyExchTx*om.State.BuyTotalValue*inst.PriceMultiplier +
			om.State.SellExchTx*om.State.SellTotalValue*inst.PriceMultiplier +
			om.State.BuyExchContractTx*om.State.BuyTotalQty +
			om.State.SellExchContractTx*om.State.SellTotalQty

		// C++: 重置当前腿
		om.State.BuyValue = 0
		om.State.BuyQty = 0
		om.State.BuyPrice = 0
		om.State.SellValue = 0
		om.State.SellQty = 0
		om.State.SellPrice = 0
		om.State.TransValue = 0
	}

	// 重算 PNL
	om.State.CalculatePNL(inst)

	// C++: openQty == 0 → TRADED, RemoveOrder
	if ord.OpenQty == 0 {
		// C++: 如果有 pending modify，先清理
		if ord.Status == types.StatusModifyOrder {
			om.processModifyReject(nil, ord)
		}
		ord.Status = types.StatusTraded
		om.RemoveOrder(resp.OrderID)
	} else if ord.OpenQty < 0 {
		log.Printf("[ORS] ERROR: negative openQty=%d for orderID=%d", ord.OpenQty, resp.OrderID)
	}

	log.Printf("[ORS] TRADE orderID=%d side=%d price=%.2f qty=%d netpos=%d pnl=%.2f",
		resp.OrderID, ord.Side, tradePrice, tradeQty, om.State.Netpos, om.State.NetPNL)
}

// processReject 处理 ORS/RMS/SIM 拒绝
// 参考: ExecutionStrategy.cpp:1020-1040
func (om *OrderManager) processReject(resp *shm.ResponseMsg, ord *types.OrderStats) {
	om.State.RejectCount++

	// C++: 根据当前状态决定处理方式
	switch ord.Status {
	case types.StatusCancelOrder, types.StatusTraded:
		// 撤单/已成交后的拒绝，忽略
		return
	case types.StatusModifyOrder:
		// 改单被拒，回退
		om.processModifyReject(resp, ord)
		return
	default:
		// 新订单被拒
		om.processNewReject(resp, ord)
	}
}

// processNewReject 处理新订单拒绝
// 参考: ExecutionStrategy.cpp:1020-1040
func (om *OrderManager) processNewReject(_ *shm.ResponseMsg, ord *types.OrderStats) {
	ord.Status = types.StatusNewReject
	om.State.RejectCount++

	// C++: RemoveOrder — 回退 openQty 和 openOrders
	if ord.Side == types.Buy {
		om.State.BuyOpenQty -= float64(ord.OpenQty)
	} else {
		om.State.SellOpenQty -= float64(ord.OpenQty)
	}

	om.RemoveOrder(ord.OrderID)

	log.Printf("[ORS] NEW_REJECT orderID=%d", ord.OrderID)
}

// processModifyReject 处理改单拒绝
// 参考: ExecutionStrategy.cpp:1080-1100
//
// C++ 逻辑: 回退到旧价格/数量
func (om *OrderManager) processModifyReject(_ *shm.ResponseMsg, ord *types.OrderStats) {
	if ord.Status != types.StatusTraded {
		ord.Status = types.StatusModifyReject
	}

	// C++: 移除乐观插入的新价格
	if ord.Side == types.Buy {
		delete(om.BidMap, ord.NewPrice)
		om.State.BuyOpenQty -= float64(ord.NewQty - ord.Qty)
	} else {
		delete(om.AskMap, ord.NewPrice)
		om.State.SellOpenQty -= float64(ord.NewQty - ord.Qty)
	}

	ord.ModifyWait = false

	log.Printf("[ORS] MODIFY_REJECT orderID=%d, reverting to price=%.2f", ord.OrderID, ord.Price)
}

// processCancelReject 处理撤单拒绝
// 参考: ExecutionStrategy.cpp:1100-1120, 1870-1880
//
// C++ 逻辑:
//   1. 记录撤单拒绝信息（用于 CANCELREQ_PAUSE 冷却）
//   2. 如果 fillOnCxlReject 且 resp.Quantity==0，合成成交事件
//   3. 恢复 status 到 NEW_CONFIRM（订单仍然活跃）
func (om *OrderManager) processCancelReject(resp *shm.ResponseMsg, ord *types.OrderStats, inst *instrument.Instrument) {
	// C++: 记录撤单拒绝信息，用于 CANCELREQ_PAUSE 冷却
	// 参考: ExecutionStrategy.cpp:1870-1872
	om.LastCancelRejectSet = 1
	om.LastCancelRejectOrderID = resp.OrderID
	om.LastCancelRejectTime = om.State.ExchTS

	// C++: fillOnCxlReject — 撤单拒绝且量为 0 表示订单已完全成交
	// 参考: ExecutionStrategy.cpp:1874-1880
	if resp.Quantity == 0 && om.FillOnCxlReject {
		log.Printf("[ORS] ALERT: TRADE ON CANCEL REJECT orderID=%d, synthesizing fill price=%.2f qty=%d",
			resp.OrderID, ord.Price, ord.Qty)
		// 合成成交事件
		synthResp := &shm.ResponseMsg{
			OrderID:  resp.OrderID,
			Price:    ord.Price,
			Quantity: ord.Qty,
		}
		om.processTrade(synthResp, ord, inst)
		return
	}

	if ord.Status != types.StatusTraded {
		ord.Status = types.StatusNewConfirm
	}
	om.State.RejectCount++

	log.Printf("[ORS] CANCEL_REJECT orderID=%d, status reset to NEW_CONFIRM", resp.OrderID)
}
