package execution

import (
	"log"

	"tbsrc-golang/pkg/client"
	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/types"
)

// OrderManager 管理订单映射和下单/撤单逻辑
// 对应 C++ ExtraStrategy 中的 m_ordMap, m_bidMap, m_askMap 及相关方法
// 参考: tbsrc/Strategies/ExtraStrategy.cpp:201-485
type OrderManager struct {
	OrdMap      map[uint32]*types.OrderStats  // orderID → OrderStats
	BidMap      map[float64]*types.OrderStats // price → bid OrderStats
	AskMap      map[float64]*types.OrderStats // price → ask OrderStats
	Client      *client.Client
	State       *ExecutionState
	nextTestOID uint32 // used only when Client is nil (testing)
}

// NewOrderManager 创建 OrderManager
func NewOrderManager(c *client.Client, state *ExecutionState) *OrderManager {
	return &OrderManager{
		OrdMap: make(map[uint32]*types.OrderStats),
		BidMap: make(map[float64]*types.OrderStats),
		AskMap: make(map[float64]*types.OrderStats),
		Client: c,
		State:  state,
	}
}

// SendNewOrder 发送新订单
// 返回 (orderID, success)。如果该价格已有挂单，返回 (0, false)
// 参考: tbsrc/Strategies/ExtraStrategy.cpp:201-278
//
// C++ 逻辑:
//   1. 检查重复价格（bidMap/askMap）
//   2. 更新 openOrders/openQty
//   3. 通过 client 发送，获取 orderID
//   4. 创建 OrderStats，插入 ordMap 和 priceMap
func (om *OrderManager) SendNewOrder(side types.TransactionType, price float64, qty int32,
	level int32, inst *instrument.Instrument, typeOfOrder types.TypeOfOrder,
	ordType types.OrderHitType, cb client.StrategyCallback) (uint32, bool) {

	// C++: duplicate price check
	if side == types.Buy {
		if _, exists := om.BidMap[price]; exists {
			return 0, false
		}
		om.State.BuyOpenOrders++
		om.State.BuyOpenQty += float64(qty)
	} else {
		if _, exists := om.AskMap[price]; exists {
			return 0, false
		}
		om.State.SellOpenOrders++
		om.State.SellOpenQty += float64(qty)
	}

	// 通过 client 发送
	var orderID uint32
	if om.Client != nil {
		orderID = om.Client.SendNewOrder(inst, side, price, qty, ordType, cb)
	} else {
		// testing path: generate a local orderID
		om.nextTestOID++
		orderID = om.nextTestOID
	}

	// 创建 OrderStats
	ordStats := types.NewOrderStats(orderID, side, price, qty, typeOfOrder, ordType)

	// C++: 估计 quantAhead
	if side == types.Buy {
		if level >= 0 && level < int32(instrument.BookDepth) && inst.BidPx[level] == price {
			ordStats.QuantAhead = inst.BidQty[level]
		}
	} else {
		if level >= 0 && level < int32(instrument.BookDepth) && inst.AskPx[level] == price {
			ordStats.QuantAhead = inst.AskQty[level]
		}
	}

	// 插入 maps
	om.OrdMap[orderID] = ordStats
	if side == types.Buy {
		om.BidMap[price] = ordStats
	} else {
		om.AskMap[price] = ordStats
	}

	om.State.OrderCount++

	return orderID, true
}

// SendModifyOrder 发送改单请求
// 参考: tbsrc/Strategies/ExtraStrategy.cpp:280-373
//
// C++ 逻辑:
//   1. 查找订单
//   2. 检查新价格不重复、订单不在 MODIFY_ORDER 状态
//   3. 乐观插入新价格到 priceMap
//   4. 发送改单
//   5. 保存旧价格/数量用于回退
func (om *OrderManager) SendModifyOrder(inst *instrument.Instrument, orderID uint32,
	price float64, qty int32, level int32, typeOfOrder types.TypeOfOrder,
	ordType types.OrderHitType) bool {

	ord, ok := om.OrdMap[orderID]
	if !ok {
		return false
	}

	// C++: prevent modify if new price already in map or already modifying
	if ord.Side == types.Buy {
		if _, exists := om.BidMap[price]; exists {
			return false
		}
		if ord.Status == types.StatusModifyOrder {
			return false
		}
	} else {
		if _, exists := om.AskMap[price]; exists {
			return false
		}
		if ord.Status == types.StatusModifyOrder {
			return false
		}
	}

	ord.Status = types.StatusModifyOrder
	ord.NewPrice = price
	ord.NewQty = qty
	ord.OrdType = ordType

	// C++: optimistic insert new price into price map
	if ord.Side == types.Buy {
		om.BidMap[price] = ord
		om.State.BuyOpenQty += float64(qty - ord.Qty)
	} else {
		om.AskMap[price] = ord
		om.State.SellOpenQty += float64(qty - ord.Qty)
	}

	// 发送改单
	if om.Client != nil {
		om.Client.SendModifyOrder(inst, orderID, ord.Side, price, ord.DoneQty, qty, nil)
	}

	// C++: save old price/qty for rollback
	if ord.Modify == 0 {
		ord.OldPrice = ord.Price
		ord.OldQty = ord.OpenQty
	}
	ord.Modify++
	ord.ModifyWait = true

	return true
}

// SendCancelOrderByID 按 orderID 撤单
// 参考: tbsrc/Strategies/ExtraStrategy.cpp:401-485
func (om *OrderManager) SendCancelOrderByID(inst *instrument.Instrument, orderID uint32) bool {
	ord, ok := om.OrdMap[orderID]
	if !ok {
		return false
	}

	// C++: only cancel if in confirmed/modify states
	if ord.Status != types.StatusNewConfirm &&
		ord.Status != types.StatusModifyConfirm &&
		ord.Status != types.StatusModifyReject {
		return false
	}

	ord.Status = types.StatusCancelOrder

	if om.Client != nil {
		om.Client.SendCancelOrder(inst, orderID, ord.Side, ord.Price, ord.DoneQty, ord.OpenQty)
	}
	om.State.CancelCount++

	return true
}

// SendCancelOrderByPrice 按价格和方向撤单
// 参考: tbsrc/Strategies/ExtraStrategy.cpp:375-399
func (om *OrderManager) SendCancelOrderByPrice(inst *instrument.Instrument, price float64, side types.TransactionType) bool {
	var ord *types.OrderStats
	if side == types.Buy {
		ord = om.BidMap[price]
	} else {
		ord = om.AskMap[price]
	}
	if ord == nil {
		return false
	}
	return om.SendCancelOrderByID(inst, ord.OrderID)
}

// RemoveOrder 从所有 map 中移除订单
// 参考: tbsrc/Strategies/ExecutionStrategy.cpp:1175-1213
func (om *OrderManager) RemoveOrder(orderID uint32) {
	ord, ok := om.OrdMap[orderID]
	if !ok {
		return
	}

	// C++: 从 priceMap 中移除，减少 openOrders
	if ord.Side == types.Buy {
		delete(om.BidMap, ord.Price)
		om.State.BuyOpenOrders--
	} else {
		delete(om.AskMap, ord.Price)
		om.State.SellOpenOrders--
	}

	// C++: 从 ordMap 中移除，清理 orderID 映射
	if om.Client != nil {
		om.Client.RemoveOrderID(orderID)
	}
	delete(om.OrdMap, orderID)

	log.Printf("[OrderManager] removed order %d side=%d price=%.2f",
		orderID, ord.Side, ord.Price)
}
