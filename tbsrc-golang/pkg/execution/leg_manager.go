package execution

import (
	"log"
	"math"

	"tbsrc-golang/pkg/client"
	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/shm"
	"tbsrc-golang/pkg/types"
)

// LegManager 对应 C++ ExtraStrategy
// 组合 ExecutionState + OrderManager，管理单腿的订单/持仓/PNL
// 参考: tbsrc/Strategies/ExtraStrategy.cpp
type LegManager struct {
	State      *ExecutionState
	Orders     *OrderManager
	Inst       *instrument.Instrument
	Thold      *types.ThresholdSet
	Client     *client.Client
	StrategyID int32
	Account    string
}

// NewLegManager 创建 LegManager
func NewLegManager(c *client.Client, inst *instrument.Instrument, thold *types.ThresholdSet,
	strategyID int32, account string) *LegManager {
	state := &ExecutionState{}
	return &LegManager{
		State:      state,
		Orders:     NewOrderManager(c, state),
		Inst:       inst,
		Thold:      thold,
		Client:     c,
		StrategyID: strategyID,
		Account:    account,
	}
}

// SetExchangeCosts 设置交易所手续费率（从配置加载）
func (lm *LegManager) SetExchangeCosts(buyTx, sellTx, buyContractTx, sellContractTx float64) {
	lm.State.BuyExchTx = buyTx
	lm.State.SellExchTx = sellTx
	lm.State.BuyExchContractTx = buyContractTx
	lm.State.SellExchContractTx = sellContractTx
}

// SendBidOrder 发送买单
// 参考: tbsrc/Strategies/ExtraStrategy.cpp:33-84
//
// C++ 逻辑:
//   1. 计算数量（默认 tholdSize，可用 percent-based）
//   2. 如果 price == 0，调用 GetBidPrice（Phase 3 实现）
//   3. 分派到 SendNewOrder 或 SendModifyOrder
func (lm *LegManager) SendBidOrder(reqType shm.RequestType, level int32,
	price float64, ordType types.OrderHitType, qty int32, ordID uint32, oldPx float64) {

	// C++: int32_t qty = m_tholdSize
	actualQty := lm.State.TholdSize
	if qty > 0 {
		actualQty = qty
	}

	if actualQty <= 0 {
		return
	}

	if price == 0 {
		return // GetBidPrice 在 Phase 3 实现
	}

	if reqType == shm.NEWORDER {
		lm.Orders.SendNewOrder(types.Buy, price, actualQty, level, lm.Inst,
			types.Quote, ordType, lm)
	} else {
		lm.Orders.SendModifyOrder(lm.Inst, ordID, price, actualQty, level,
			types.Quote, ordType)
	}
}

// SendAskOrder 发送卖单
// 参考: tbsrc/Strategies/ExtraStrategy.cpp:86-136
func (lm *LegManager) SendAskOrder(reqType shm.RequestType, level int32,
	price float64, ordType types.OrderHitType, qty int32, ordID uint32, oldPx float64) {

	actualQty := lm.State.TholdSize
	if qty > 0 {
		actualQty = qty
	}

	if actualQty <= 0 {
		return
	}

	if price == 0 {
		return
	}

	if reqType == shm.NEWORDER {
		lm.Orders.SendNewOrder(types.Sell, price, actualQty, level, lm.Inst,
			types.Quote, ordType, lm)
	} else {
		lm.Orders.SendModifyOrder(lm.Inst, ordID, price, actualQty, level,
			types.Quote, ordType)
	}
}

// SendBidOrder2 发送买单（使用独立 bid 大小），返回是否成功
// 参考: tbsrc/Strategies/ExtraStrategy.cpp:138-168
func (lm *LegManager) SendBidOrder2(reqType shm.RequestType, level int32,
	price float64, ordType types.OrderHitType, qty int32, ordID uint32, oldPx float64) bool {

	// C++: int32_t qty = m_tholdBidSize
	actualQty := lm.State.TholdBidSize
	if qty > 0 {
		actualQty = qty
	}

	if actualQty <= 0 {
		return false
	}

	if price == 0 {
		return false
	}

	if reqType == shm.NEWORDER {
		_, ok := lm.Orders.SendNewOrder(types.Buy, price, actualQty, level, lm.Inst,
			types.Quote, ordType, lm)
		return ok
	}
	return lm.Orders.SendModifyOrder(lm.Inst, ordID, price, actualQty, level,
		types.Quote, ordType)
}

// SendAskOrder2 发送卖单（使用独立 ask 大小），返回是否成功
// 参考: tbsrc/Strategies/ExtraStrategy.cpp:170-199
func (lm *LegManager) SendAskOrder2(reqType shm.RequestType, level int32,
	price float64, ordType types.OrderHitType, qty int32, ordID uint32, oldPx float64) bool {

	// C++: int32_t qty = m_tholdAskSize
	actualQty := lm.State.TholdAskSize
	if qty > 0 {
		actualQty = qty
	}

	if actualQty <= 0 {
		return false
	}

	if price == 0 {
		return false
	}

	if reqType == shm.NEWORDER {
		_, ok := lm.Orders.SendNewOrder(types.Sell, price, actualQty, level, lm.Inst,
			types.Quote, ordType, lm)
		return ok
	}
	return lm.Orders.SendModifyOrder(lm.Inst, ordID, price, actualQty, level,
		types.Quote, ordType)
}

// MDCallBack 行情回调
// 参考: tbsrc/Strategies/ExtraStrategy.cpp:487-540
//
// C++ 逻辑:
//   1. 更新 LTP
//   2. 仅在 BBO 变化时重算 PNL
func (lm *LegManager) MDCallBack(inst *instrument.Instrument, md *shm.MarketUpdateNew) {
	// C++: 更新 LTP（成交类型更新）
	if md.Data.LastTradedPrice > 0 {
		lm.State.LTP = md.Data.LastTradedPrice
	}

	lm.State.ExchTS = md.Header.ExchTS

	// C++: 仅在 BBO 变化时重算 PNL
	if lm.State.BestBidLastPNL != inst.BidPx[0] || lm.State.BestAskLastPNL != inst.AskPx[0] {
		lm.State.CalculatePNL(inst)
		lm.State.BestBidLastPNL = inst.BidPx[0]
		lm.State.BestAskLastPNL = inst.AskPx[0]
	}
}

// ORSCallBack ORS 回调，委托给 OrderManager
func (lm *LegManager) ORSCallBack(resp *shm.ResponseMsg) {
	lm.Orders.ProcessORSResponse(resp, lm.Inst)
}

// HandleSquareoff 处理平仓
// 参考: tbsrc/Strategies/ExtraStrategy.cpp:542-623
//
// C++ 逻辑:
//   1. 如果已平且退出中，停用策略
//   2. 计算平仓价格（aggFlat 用穿越价，普通用被动价）
//   3. 撤销所有不匹配的挂单
//   4. 所有挂单撤完后，发送平仓单
func (lm *LegManager) HandleSquareoff() {
	inst := lm.Inst
	state := lm.State

	// C++: 如果已平且退出中且无挂单，停用
	if state.Netpos == 0 && state.OnExit && len(lm.Orders.AskMap) == 0 && len(lm.Orders.BidMap) == 0 {
		if state.Active {
			state.Active = false
			log.Printf("[LegManager] strategy deactivated (flat + exit)")
		}
		return
	}

	// C++: 计算平仓价格
	var sellPrice, buyPrice float64
	if state.AggFlat {
		// 激进平仓：穿越 BBO
		sellPrice = inst.BidPx[0] - inst.TickSize
		buyPrice = inst.AskPx[0] + inst.TickSize
	} else {
		// 被动平仓
		sellPrice = inst.AskPx[0]
		buyPrice = inst.BidPx[0]
	}

	// C++: 确保正价格
	if sellPrice <= 0 {
		sellPrice = inst.BidPx[0]
	}
	if buyPrice <= 0 {
		buyPrice = inst.AskPx[0]
	}

	// C++: 撤销不匹配的挂单
	if len(lm.Orders.AskMap) > 0 || len(lm.Orders.BidMap) > 0 {
		// 收集要撤销的 ask 订单
		for _, ord := range lm.Orders.AskMap {
			if state.OnCancel || sellPrice < ord.Price || state.Netpos == 0 {
				lm.Orders.SendCancelOrderByID(inst, ord.OrderID)
			}
		}
		// 收集要撤销的 bid 订单
		for _, ord := range lm.Orders.BidMap {
			if state.OnCancel || buyPrice > ord.Price || state.Netpos == 0 {
				lm.Orders.SendCancelOrderByID(inst, ord.OrderID)
			}
		}
	}
	state.OnCancel = false

	// C++: 所有挂单撤完后发送平仓单
	qty := int32(math.Abs(float64(state.Netpos)))
	if state.RmsQty == 0 {
		state.RmsQty = qty
	}
	if qty > state.RmsQty {
		qty = state.RmsQty
	}

	if len(lm.Orders.AskMap) == 0 && len(lm.Orders.BidMap) == 0 {
		if state.Netpos > 0 {
			// 多头平仓：卖出
			var ordType types.OrderHitType
			if state.AggFlat {
				ordType = types.HitCross
			} else {
				ordType = types.HitStandard
			}
			lm.Orders.SendNewOrder(types.Sell, sellPrice, qty, 0, inst,
				types.Quote, ordType, lm)
		} else if state.Netpos < 0 {
			// 空头平仓：买入
			var ordType types.OrderHitType
			if state.AggFlat {
				ordType = types.HitCross
			} else {
				ordType = types.HitStandard
			}
			lm.Orders.SendNewOrder(types.Buy, buyPrice, qty, 0, inst,
				types.Quote, ordType, lm)
		}
	}
}

// Reset 重置 LegManager 状态
func (lm *LegManager) Reset() {
	lm.State.Reset()
	lm.Orders.OrdMap = make(map[uint32]*types.OrderStats)
	lm.Orders.BidMap = make(map[float64]*types.OrderStats)
	lm.Orders.AskMap = make(map[float64]*types.OrderStats)
}
