package execution

import (
	"math"
	"testing"

	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/shm"
	"tbsrc-golang/pkg/types"
)

// mockClient 模拟 client.Client 用于单元测试
// 不依赖真实 SHM，直接跟踪发送的请求
type mockClient struct {
	nextOrderID uint32
	sentOrders  []mockOrder
	sentModify  []mockModify
	sentCancel  []mockCancel
	removedIDs  []uint32
}

type mockOrder struct {
	side    types.TransactionType
	price   float64
	qty     int32
	ordType types.OrderHitType
}

type mockModify struct {
	orderID uint32
	price   float64
	qty     int32
}

type mockCancel struct {
	orderID uint32
}

// 为 OrderManager 创建测试辅助函数 — 直接操作 OrdMap/BidMap/AskMap
// 绕过 client.Client 依赖

func newTestOrderManager() (*OrderManager, *instrument.Instrument) {
	state := &ExecutionState{}
	om := &OrderManager{
		OrdMap: make(map[uint32]*types.OrderStats),
		BidMap: make(map[float64]*types.OrderStats),
		AskMap: make(map[float64]*types.OrderStats),
		Client: nil, // 不需要真实 client 做单元测试
		State:  state,
	}
	inst := &instrument.Instrument{
		Symbol:          "ag2506",
		TickSize:        1.0,
		LotSize:         15,
		PriceMultiplier: 15.0,
		SendInLots:      true,
	}
	inst.BidPx[0] = 5819.0
	inst.BidQty[0] = 100
	inst.AskPx[0] = 5820.0
	inst.AskQty[0] = 80
	inst.ValidBids = 1
	inst.ValidAsks = 1
	return om, inst
}

// 手动插入一个订单到 OrderManager（绕过 client.SendNewOrder）
func insertOrder(om *OrderManager, orderID uint32, side types.TransactionType,
	price float64, qty int32, ordType types.OrderHitType) *types.OrderStats {
	ord := types.NewOrderStats(orderID, side, price, qty, types.Quote, ordType)
	om.OrdMap[orderID] = ord
	if side == types.Buy {
		om.BidMap[price] = ord
		om.State.BuyOpenOrders++
		om.State.BuyOpenQty += float64(qty)
	} else {
		om.AskMap[price] = ord
		om.State.SellOpenOrders++
		om.State.SellOpenQty += float64(qty)
	}
	om.State.OrderCount++
	return ord
}

// TestOrderLifecycle_NewConfirmTrade 完整订单生命周期
// NewOrder → Confirm → Trade → Removed
func TestOrderLifecycle_NewConfirmTrade(t *testing.T) {
	om, inst := newTestOrderManager()

	// 1. 插入新买单
	ord := insertOrder(om, 1001, types.Buy, 5819.0, 10, types.HitStandard)

	if len(om.OrdMap) != 1 {
		t.Fatalf("OrdMap size = %d, want 1", len(om.OrdMap))
	}
	if len(om.BidMap) != 1 {
		t.Fatalf("BidMap size = %d, want 1", len(om.BidMap))
	}
	if om.State.BuyOpenOrders != 1 {
		t.Errorf("BuyOpenOrders = %d, want 1", om.State.BuyOpenOrders)
	}

	// 2. NEW_ORDER_CONFIRM
	resp := &shm.ResponseMsg{
		Response_Type: shm.NEW_ORDER_CONFIRM,
		OrderID:       1001,
	}
	om.ProcessORSResponse(resp, inst)

	if ord.Status != types.StatusNewConfirm {
		t.Errorf("Status = %d, want NewConfirm(%d)", ord.Status, types.StatusNewConfirm)
	}
	if om.State.ConfirmCount != 1 {
		t.Errorf("ConfirmCount = %d, want 1", om.State.ConfirmCount)
	}

	// 3. TRADE_CONFIRM (全部成交)
	resp = &shm.ResponseMsg{
		Response_Type: shm.TRADE_CONFIRM,
		OrderID:       1001,
		Price:         5819.0,
		Quantity:      10,
	}
	om.ProcessORSResponse(resp, inst)

	// 验证订单已移除
	if len(om.OrdMap) != 0 {
		t.Errorf("OrdMap should be empty after full trade, got %d", len(om.OrdMap))
	}
	if len(om.BidMap) != 0 {
		t.Errorf("BidMap should be empty after full trade, got %d", len(om.BidMap))
	}

	// 验证持仓更新
	if om.State.Netpos != 10 {
		t.Errorf("Netpos = %d, want 10", om.State.Netpos)
	}
	if om.State.BuyTotalQty != 10 {
		t.Errorf("BuyTotalQty = %f, want 10", om.State.BuyTotalQty)
	}
	if om.State.TradeCount != 1 {
		t.Errorf("TradeCount = %d, want 1", om.State.TradeCount)
	}
	if om.State.BuyAvgPrice != 5819.0 {
		t.Errorf("BuyAvgPrice = %f, want 5819.0", om.State.BuyAvgPrice)
	}
}

// TestOrderLifecycle_PartialTrade 部分成交
func TestOrderLifecycle_PartialTrade(t *testing.T) {
	om, inst := newTestOrderManager()

	ord := insertOrder(om, 2001, types.Sell, 5820.0, 20, types.HitStandard)

	// Confirm
	om.ProcessORSResponse(&shm.ResponseMsg{
		Response_Type: shm.NEW_ORDER_CONFIRM,
		OrderID:       2001,
	}, inst)

	// Partial trade (5 of 20)
	om.ProcessORSResponse(&shm.ResponseMsg{
		Response_Type: shm.TRADE_CONFIRM,
		OrderID:       2001,
		Price:         5820.0,
		Quantity:      5,
	}, inst)

	// 订单应该还在
	if len(om.OrdMap) != 1 {
		t.Fatalf("OrdMap should still have 1 order, got %d", len(om.OrdMap))
	}
	if ord.OpenQty != 15 {
		t.Errorf("OpenQty = %d, want 15", ord.OpenQty)
	}
	if ord.DoneQty != 5 {
		t.Errorf("DoneQty = %d, want 5", ord.DoneQty)
	}
	if om.State.Netpos != -5 {
		t.Errorf("Netpos = %d, want -5", om.State.Netpos)
	}

	// Fill remaining (15 of 20)
	om.ProcessORSResponse(&shm.ResponseMsg{
		Response_Type: shm.TRADE_CONFIRM,
		OrderID:       2001,
		Price:         5820.0,
		Quantity:      15,
	}, inst)

	if len(om.OrdMap) != 0 {
		t.Errorf("OrdMap should be empty after full trade")
	}
	if om.State.Netpos != -20 {
		t.Errorf("Netpos = %d, want -20", om.State.Netpos)
	}
}

// TestOrderLifecycle_DuplicatePrice 重复价格拒绝
func TestOrderLifecycle_DuplicatePrice(t *testing.T) {
	om, _ := newTestOrderManager()

	// 插入一个 bid 在 5819.0
	insertOrder(om, 3001, types.Buy, 5819.0, 10, types.HitStandard)

	// 尝试在相同价格再插入一个 — 应该在 BidMap 中找到重复
	_, exists := om.BidMap[5819.0]
	if !exists {
		t.Fatal("BidMap should have entry at 5819.0")
	}

	// 不同价格应该可以插入
	insertOrder(om, 3002, types.Buy, 5818.0, 10, types.HitStandard)
	if len(om.BidMap) != 2 {
		t.Errorf("BidMap size = %d, want 2", len(om.BidMap))
	}
}

// TestOrderLifecycle_CancelFlow 撤单流程
func TestOrderLifecycle_CancelFlow(t *testing.T) {
	om, inst := newTestOrderManager()

	insertOrder(om, 4001, types.Buy, 5819.0, 10, types.HitStandard)

	// Confirm
	om.ProcessORSResponse(&shm.ResponseMsg{
		Response_Type: shm.NEW_ORDER_CONFIRM,
		OrderID:       4001,
	}, inst)

	// Cancel — 需要先手动设置状态
	ord := om.OrdMap[4001]
	ord.Status = types.StatusCancelOrder

	// Cancel confirm
	om.ProcessORSResponse(&shm.ResponseMsg{
		Response_Type: shm.CANCEL_ORDER_CONFIRM,
		OrderID:       4001,
		Quantity:      10,
	}, inst)

	// 应该已移除
	if len(om.OrdMap) != 0 {
		t.Errorf("OrdMap should be empty after cancel, got %d", len(om.OrdMap))
	}
	if len(om.BidMap) != 0 {
		t.Errorf("BidMap should be empty after cancel, got %d", len(om.BidMap))
	}
	if om.State.BuyOpenOrders != 0 {
		t.Errorf("BuyOpenOrders = %d, want 0", om.State.BuyOpenOrders)
	}
	if om.State.CancelConfirmCnt != 1 {
		t.Errorf("CancelConfirmCnt = %d, want 1", om.State.CancelConfirmCnt)
	}
}

// TestOrderLifecycle_ModifyFlow 改单流程
func TestOrderLifecycle_ModifyFlow(t *testing.T) {
	om, inst := newTestOrderManager()

	insertOrder(om, 5001, types.Buy, 5819.0, 10, types.HitStandard)

	// Confirm
	om.ProcessORSResponse(&shm.ResponseMsg{
		Response_Type: shm.NEW_ORDER_CONFIRM,
		OrderID:       5001,
	}, inst)

	// 模拟改单请求（直接修改 OrderStats，因为没有真实 client）
	ord := om.OrdMap[5001]
	ord.Status = types.StatusModifyOrder
	ord.NewPrice = 5818.0
	ord.NewQty = 15
	// 乐观插入新价格
	om.BidMap[5818.0] = ord

	// Modify confirm
	om.ProcessORSResponse(&shm.ResponseMsg{
		Response_Type: shm.MODIFY_ORDER_CONFIRM,
		OrderID:       5001,
	}, inst)

	if ord.Status != types.StatusModifyConfirm {
		t.Errorf("Status = %d, want ModifyConfirm(%d)", ord.Status, types.StatusModifyConfirm)
	}
	if ord.Price != 5818.0 {
		t.Errorf("Price = %f, want 5818.0", ord.Price)
	}
	if ord.Qty != 15 {
		t.Errorf("Qty = %d, want 15", ord.Qty)
	}

	// 旧价格应该已从 BidMap 移除
	if _, exists := om.BidMap[5819.0]; exists {
		t.Error("old price 5819.0 should be removed from BidMap")
	}
	// 新价格应该在 BidMap 中
	if _, exists := om.BidMap[5818.0]; !exists {
		t.Error("new price 5818.0 should be in BidMap")
	}
}

// TestOrderLifecycle_NewReject 新订单拒绝
func TestOrderLifecycle_NewReject(t *testing.T) {
	om, inst := newTestOrderManager()

	insertOrder(om, 6001, types.Sell, 5820.0, 10, types.HitStandard)

	// ORS_REJECT
	om.ProcessORSResponse(&shm.ResponseMsg{
		Response_Type: shm.ORS_REJECT,
		OrderID:       6001,
	}, inst)

	// 应该已移除
	if len(om.OrdMap) != 0 {
		t.Errorf("OrdMap should be empty after reject")
	}
	if len(om.AskMap) != 0 {
		t.Errorf("AskMap should be empty after reject")
	}
	if om.State.RejectCount != 2 { // processReject increments + processNewReject increments
		t.Errorf("RejectCount = %d, want 2", om.State.RejectCount)
	}
}

// TestOrderLifecycle_CancelReject 撤单拒绝 — 回到 NEW_CONFIRM
func TestOrderLifecycle_CancelReject(t *testing.T) {
	om, inst := newTestOrderManager()

	ord := insertOrder(om, 7001, types.Buy, 5819.0, 10, types.HitStandard)

	// Confirm
	om.ProcessORSResponse(&shm.ResponseMsg{
		Response_Type: shm.NEW_ORDER_CONFIRM,
		OrderID:       7001,
	}, inst)

	// 设置撤单状态
	ord.Status = types.StatusCancelOrder

	// Cancel reject
	om.ProcessORSResponse(&shm.ResponseMsg{
		Response_Type: shm.CANCEL_ORDER_REJECT,
		OrderID:       7001,
	}, inst)

	// 应该回到 NEW_CONFIRM
	if ord.Status != types.StatusNewConfirm {
		t.Errorf("Status = %d, want NewConfirm after cancel reject", ord.Status)
	}
	// 订单还在
	if len(om.OrdMap) != 1 {
		t.Errorf("OrdMap should still have 1 order")
	}
}

// TestProcessTrade_NetposFlat 买卖对冲归零
func TestProcessTrade_NetposFlat(t *testing.T) {
	om, inst := newTestOrderManager()
	inst.PriceMultiplier = 15.0

	// 先买 10 手 @ 5800
	buyOrd := insertOrder(om, 8001, types.Buy, 5800.0, 10, types.HitStandard)
	buyOrd.Status = types.StatusNewConfirm

	om.ProcessORSResponse(&shm.ResponseMsg{
		Response_Type: shm.TRADE_CONFIRM,
		OrderID:       8001,
		Price:         5800.0,
		Quantity:      10,
	}, inst)

	if om.State.Netpos != 10 {
		t.Fatalf("Netpos = %d after buy, want 10", om.State.Netpos)
	}

	// 再卖 10 手 @ 5810 → 平仓
	sellOrd := insertOrder(om, 8002, types.Sell, 5810.0, 10, types.HitStandard)
	sellOrd.Status = types.StatusNewConfirm

	om.ProcessORSResponse(&shm.ResponseMsg{
		Response_Type: shm.TRADE_CONFIRM,
		OrderID:       8002,
		Price:         5810.0,
		Quantity:      10,
	}, inst)

	// 验证平仓
	if om.State.Netpos != 0 {
		t.Errorf("Netpos = %d after flatten, want 0", om.State.Netpos)
	}

	// C++: realisedPNL = (sellTotalValue - buyTotalValue) * priceMultiplier
	// = (58100 - 58000) * 15 = 1500
	expectedRealised := (5810.0*10 - 5800.0*10) * 15.0
	if math.Abs(om.State.RealisedPNL-expectedRealised) > 0.01 {
		t.Errorf("RealisedPNL = %f, want %f", om.State.RealisedPNL, expectedRealised)
	}

	// 当前腿应该重置
	if om.State.BuyValue != 0 {
		t.Errorf("BuyValue = %f, want 0 after flatten", om.State.BuyValue)
	}
	if om.State.SellValue != 0 {
		t.Errorf("SellValue = %f, want 0 after flatten", om.State.SellValue)
	}
}

// TestProcessTrade_NetposPassAgg 跟踪被动/主动成交
func TestProcessTrade_NetposPassAgg(t *testing.T) {
	om, inst := newTestOrderManager()

	// STANDARD 订单 → netpos_pass
	stdOrd := insertOrder(om, 9001, types.Buy, 5800.0, 5, types.HitStandard)
	stdOrd.Status = types.StatusNewConfirm

	om.ProcessORSResponse(&shm.ResponseMsg{
		Response_Type: shm.TRADE_CONFIRM,
		OrderID:       9001,
		Price:         5800.0,
		Quantity:      5,
	}, inst)

	if om.State.NetposPass != 5 {
		t.Errorf("NetposPass = %d, want 5", om.State.NetposPass)
	}

	// CROSS 订单 → netpos_agg
	crossOrd := insertOrder(om, 9002, types.Sell, 5810.0, 3, types.HitCross)
	crossOrd.Status = types.StatusNewConfirm

	om.ProcessORSResponse(&shm.ResponseMsg{
		Response_Type: shm.TRADE_CONFIRM,
		OrderID:       9002,
		Price:         5810.0,
		Quantity:      3,
	}, inst)

	if om.State.NetposAgg != -3 {
		t.Errorf("NetposAgg = %d, want -3", om.State.NetposAgg)
	}
	if om.State.CrossCount != 1 {
		t.Errorf("CrossCount = %d, want 1", om.State.CrossCount)
	}
}

// TestUnknownOrderID 未知 orderID 不崩溃
func TestUnknownOrderID(t *testing.T) {
	om, inst := newTestOrderManager()

	// 应该只打日志，不崩溃
	om.ProcessORSResponse(&shm.ResponseMsg{
		Response_Type: shm.TRADE_CONFIRM,
		OrderID:       99999,
		Price:         5800.0,
		Quantity:      1,
	}, inst)

	// 验证状态没变
	if om.State.TradeCount != 0 {
		t.Errorf("TradeCount = %d, want 0", om.State.TradeCount)
	}
}
