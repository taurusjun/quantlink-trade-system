package strategy

import (
	"testing"

	"tbsrc-golang/pkg/shm"
	"tbsrc-golang/pkg/types"
)

// TestORSCallbackOverride_Leg1 验证 Leg1 ORS 回调经过 PairwiseArbStrategy
// 而不是直接由 LegManager 处理（Task #33 修复验证）
func TestORSCallbackOverride_Leg1(t *testing.T) {
	pas := newTestPAS()

	// 手动设置 override（NewPairwiseArbStrategy 已自动设置，
	// 但 newTestPAS 手动构造，需要确认 override 逻辑）
	pas.Leg1.ORSCallbackOverride = pas
	pas.Leg2.ORSCallbackOverride = pas

	// 在 leg1 添加一个订单
	pas.Leg1.Orders.OrdMap[100] = &types.OrderStats{
		OrderID: 100,
		Side:    types.Buy,
		OrdType: types.HitStandard,
		OpenQty: 1,
		Qty:     1,
		Price:   5810,
		Status:  types.StatusNewConfirm,
	}
	pas.Leg1.Orders.BidMap[5810] = pas.Leg1.Orders.OrdMap[100]
	pas.Leg1.State.BuyOpenOrders = 1
	pas.Leg1.State.BuyOpenQty = 1
	pas.AggRepeat = 5 // set to non-1 to detect reset

	// Simulate TRADE_CONFIRM via LegManager.ORSCallBack
	// This should route through PairwiseArbStrategy.ORSCallBack
	resp := &shm.ResponseMsg{}
	resp.OrderID = 100
	resp.Response_Type = shm.TRADE_CONFIRM
	resp.Price = 5810
	resp.Quantity = 1

	// Call via LegManager's ORSCallBack (simulating what client.OnORSUpdate does)
	pas.Leg1.ORSCallBack(resp)

	// Verify PairwiseArbStrategy.ORSCallBack was invoked:
	// AggRepeat should be reset to 1 (only PairwiseArbStrategy does this)
	if pas.AggRepeat != 1 {
		t.Errorf("AggRepeat = %d, want 1 (should be reset by PairwiseArbStrategy.ORSCallBack)", pas.AggRepeat)
	}

	// Verify the trade was processed (netpos updated)
	if pas.Leg1.State.BuyTotalQty != 1 {
		t.Errorf("Leg1.BuyTotalQty = %f, want 1", pas.Leg1.State.BuyTotalQty)
	}
}

// TestORSCallbackOverride_Leg2 验证 Leg2 ORS 回调经过 PairwiseArbStrategy
// 并触发 handleAggOrder
func TestORSCallbackOverride_Leg2(t *testing.T) {
	pas := newTestPAS()
	pas.Leg1.ORSCallbackOverride = pas
	pas.Leg2.ORSCallbackOverride = pas

	// 添加 leg2 CROSS 订单
	pas.Leg2.Orders.OrdMap[200] = &types.OrderStats{
		OrderID: 200,
		Side:    types.Sell,
		OrdType: types.HitCross,
		OpenQty: 2,
		Qty:     2,
		Price:   5800,
		Status:  types.StatusNewConfirm,
	}
	pas.Leg2.Orders.AskMap[5800] = pas.Leg2.Orders.OrdMap[200]
	pas.Leg2.State.SellOpenOrders = 1
	pas.Leg2.State.SellOpenQty = 2
	pas.SellAggOrder = 1

	// Simulate full fill via LegManager.ORSCallBack
	resp := &shm.ResponseMsg{}
	resp.OrderID = 200
	resp.Response_Type = shm.TRADE_CONFIRM
	resp.Price = 5800
	resp.Quantity = 2

	pas.Leg2.ORSCallBack(resp)

	// handleAggOrder should have decremented SellAggOrder (full fill = terminal)
	if pas.SellAggOrder != 0 {
		t.Errorf("SellAggOrder = %d, want 0 (handleAggOrder should decrement on full fill)", pas.SellAggOrder)
	}

	// AggRepeat should be reset to 1
	if pas.AggRepeat != 1 {
		t.Errorf("AggRepeat = %d, want 1", pas.AggRepeat)
	}
}

// TestORSCallbackOverride_NotSet 验证无 override 时 LegManager 直接处理
func TestORSCallbackOverride_NotSet(t *testing.T) {
	pas := newTestPAS()
	// Don't set ORSCallbackOverride — LegManager should process directly

	pas.Leg1.Orders.OrdMap[300] = &types.OrderStats{
		OrderID: 300,
		Side:    types.Buy,
		OrdType: types.HitStandard,
		OpenQty: 1,
		Qty:     1,
		Price:   5810,
		Status:  types.StatusNewConfirm,
	}
	pas.Leg1.Orders.BidMap[5810] = pas.Leg1.Orders.OrdMap[300]
	pas.Leg1.State.BuyOpenOrders = 1
	pas.Leg1.State.BuyOpenQty = 1
	pas.AggRepeat = 5

	resp := &shm.ResponseMsg{}
	resp.OrderID = 300
	resp.Response_Type = shm.TRADE_CONFIRM
	resp.Price = 5810
	resp.Quantity = 1

	// Without override, LegManager processes directly
	pas.Leg1.ORSCallBack(resp)

	// PairwiseArbStrategy.ORSCallBack was NOT invoked, so AggRepeat unchanged
	if pas.AggRepeat != 5 {
		t.Errorf("AggRepeat = %d, want 5 (PairwiseArb.ORSCallBack should NOT be invoked)", pas.AggRepeat)
	}

	// But the trade was still processed
	if pas.Leg1.State.BuyTotalQty != 1 {
		t.Errorf("Leg1.BuyTotalQty = %f, want 1", pas.Leg1.State.BuyTotalQty)
	}
}

// TestNewPairwiseArbStrategy_SetsORSOverride 验证构造函数自动设置 override
func TestNewPairwiseArbStrategy_SetsORSOverride(t *testing.T) {
	inst1 := newTestInstrument("ag2506", 1.0, 15)
	inst2 := newTestInstrument("ag2512", 1.0, 15)
	thold1 := types.NewThresholdSet()
	thold1.Alpha = 0.01
	thold2 := types.NewThresholdSet()

	pas := NewPairwiseArbStrategy(nil, inst1, inst2, thold1, thold2, 92201, "TEST")

	if pas.Leg1.ORSCallbackOverride != pas {
		t.Error("Leg1.ORSCallbackOverride should be set to PairwiseArbStrategy")
	}
	if pas.Leg2.ORSCallbackOverride != pas {
		t.Error("Leg2.ORSCallbackOverride should be set to PairwiseArbStrategy")
	}
}

// TestProcessORSDirectly 验证 ProcessORSDirectly 不经过 override
func TestProcessORSDirectly(t *testing.T) {
	pas := newTestPAS()
	pas.Leg1.ORSCallbackOverride = pas

	pas.Leg1.Orders.OrdMap[400] = &types.OrderStats{
		OrderID: 400,
		Side:    types.Sell,
		OrdType: types.HitStandard,
		OpenQty: 1,
		Qty:     1,
		Price:   5811,
		Status:  types.StatusNewConfirm,
	}
	pas.Leg1.Orders.AskMap[5811] = pas.Leg1.Orders.OrdMap[400]
	pas.Leg1.State.SellOpenOrders = 1
	pas.Leg1.State.SellOpenQty = 1
	pas.AggRepeat = 5

	resp := &shm.ResponseMsg{}
	resp.OrderID = 400
	resp.Response_Type = shm.TRADE_CONFIRM
	resp.Price = 5811
	resp.Quantity = 1

	// ProcessORSDirectly bypasses override
	pas.Leg1.ProcessORSDirectly(resp)

	// AggRepeat should NOT be reset (PairwiseArb.ORSCallBack not invoked)
	if pas.AggRepeat != 5 {
		t.Errorf("AggRepeat = %d, want 5 (ProcessORSDirectly bypasses override)", pas.AggRepeat)
	}

	// But the trade was processed
	if pas.Leg1.State.SellTotalQty != 1 {
		t.Errorf("Leg1.SellTotalQty = %f, want 1", pas.Leg1.State.SellTotalQty)
	}
}
