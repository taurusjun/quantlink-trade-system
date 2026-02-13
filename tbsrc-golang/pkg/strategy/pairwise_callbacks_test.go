package strategy

import (
	"testing"

	"tbsrc-golang/pkg/shm"
	"tbsrc-golang/pkg/types"
)

func TestHandleAggOrder_BuyCross_FullFill(t *testing.T) {
	pas := newTestPAS()
	pas.BuyAggOrder = 2

	// Add a CROSS buy order
	pas.Leg2.Orders.OrdMap[500] = &types.OrderStats{
		OrderID: 500,
		Side:    types.Buy,
		OrdType: types.HitCross,
		OpenQty: 3,
	}

	// Simulate full fill
	resp := &shm.ResponseMsg{}
	resp.OrderID = 500
	resp.Response_Type = shm.TRADE_CONFIRM
	resp.Quantity = 3

	pas.handleAggOrder(resp)

	if pas.BuyAggOrder != 1 {
		t.Errorf("BuyAggOrder = %d, want 1 (decremented from 2)", pas.BuyAggOrder)
	}
}

func TestHandleAggOrder_SellCross_CancelConfirm(t *testing.T) {
	pas := newTestPAS()
	pas.SellAggOrder = 1

	pas.Leg2.Orders.OrdMap[501] = &types.OrderStats{
		OrderID: 501,
		Side:    types.Sell,
		OrdType: types.HitCross,
		OpenQty: 2,
	}

	resp := &shm.ResponseMsg{}
	resp.OrderID = 501
	resp.Response_Type = shm.CANCEL_ORDER_CONFIRM

	pas.handleAggOrder(resp)

	if pas.SellAggOrder != 0 {
		t.Errorf("SellAggOrder = %d, want 0", pas.SellAggOrder)
	}
}

func TestHandleAggOrder_PartialFill_NoDecrement(t *testing.T) {
	pas := newTestPAS()
	pas.BuyAggOrder = 1

	pas.Leg2.Orders.OrdMap[502] = &types.OrderStats{
		OrderID: 502,
		Side:    types.Buy,
		OrdType: types.HitCross,
		OpenQty: 5,
	}

	// Partial fill (3 of 5)
	resp := &shm.ResponseMsg{}
	resp.OrderID = 502
	resp.Response_Type = shm.TRADE_CONFIRM
	resp.Quantity = 3

	pas.handleAggOrder(resp)

	// Not terminal — still open
	if pas.BuyAggOrder != 1 {
		t.Errorf("BuyAggOrder = %d, want 1 (partial fill, no decrement)", pas.BuyAggOrder)
	}
}

func TestHandleAggOrder_StandardOrder_Ignored(t *testing.T) {
	pas := newTestPAS()
	pas.SellAggOrder = 1

	pas.Leg2.Orders.OrdMap[503] = &types.OrderStats{
		OrderID: 503,
		Side:    types.Sell,
		OrdType: types.HitStandard, // Not CROSS/MATCH
		OpenQty: 1,
	}

	resp := &shm.ResponseMsg{}
	resp.OrderID = 503
	resp.Response_Type = shm.CANCEL_ORDER_CONFIRM

	pas.handleAggOrder(resp)

	// Should not decrement because it's STANDARD, not CROSS
	if pas.SellAggOrder != 1 {
		t.Errorf("SellAggOrder = %d, want 1 (STANDARD ignored)", pas.SellAggOrder)
	}
}

func TestHandleAggOrder_Reject_Terminal(t *testing.T) {
	pas := newTestPAS()
	pas.BuyAggOrder = 2

	pas.Leg2.Orders.OrdMap[504] = &types.OrderStats{
		OrderID: 504,
		Side:    types.Buy,
		OrdType: types.HitCross,
		OpenQty: 1,
	}

	resp := &shm.ResponseMsg{}
	resp.OrderID = 504
	resp.Response_Type = shm.ORS_REJECT

	pas.handleAggOrder(resp)

	if pas.BuyAggOrder != 1 {
		t.Errorf("BuyAggOrder = %d, want 1 (reject is terminal)", pas.BuyAggOrder)
	}
}

func TestHandleAggOrder_UnknownOrder(t *testing.T) {
	pas := newTestPAS()
	pas.BuyAggOrder = 1

	// Order not in map
	resp := &shm.ResponseMsg{}
	resp.OrderID = 999
	resp.Response_Type = shm.TRADE_CONFIRM
	resp.Quantity = 1

	pas.handleAggOrder(resp)

	// Should not panic, no change
	if pas.BuyAggOrder != 1 {
		t.Errorf("BuyAggOrder = %d, want 1", pas.BuyAggOrder)
	}
}

func TestHandleAggOrder_NeverBelowZero(t *testing.T) {
	pas := newTestPAS()
	pas.SellAggOrder = 0 // already zero

	pas.Leg2.Orders.OrdMap[505] = &types.OrderStats{
		OrderID: 505,
		Side:    types.Sell,
		OrdType: types.HitCross,
		OpenQty: 1,
	}

	resp := &shm.ResponseMsg{}
	resp.OrderID = 505
	resp.Response_Type = shm.CANCEL_ORDER_CONFIRM

	pas.handleAggOrder(resp)

	if pas.SellAggOrder != 0 {
		t.Errorf("SellAggOrder = %d, want 0 (should not go below 0)", pas.SellAggOrder)
	}
}
