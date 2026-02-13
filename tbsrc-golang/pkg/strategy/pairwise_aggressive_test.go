package strategy

import (
	"testing"
	"time"

	"tbsrc-golang/pkg/types"
)

func TestSendAggressiveOrder_NoExposure(t *testing.T) {
	pas := newTestPAS()

	pas.SendAggressiveOrder()

	// No orders should be placed
	if len(pas.Leg2.Orders.OrdMap) != 0 {
		t.Errorf("expected no orders, got %d", len(pas.Leg2.Orders.OrdMap))
	}
}

func TestSendAggressiveOrder_LongExposure_SellHedge(t *testing.T) {
	pas := newTestPAS()
	pas.Leg1.State.NetposPass = 3

	pas.SendAggressiveOrder()

	// Should have sent a sell order on leg2
	found := false
	for _, ord := range pas.Leg2.Orders.OrdMap {
		if ord.Side == types.Sell && ord.OrdType == types.HitCross {
			found = true
			// Price should be at bidPx[0] = 5800
			if ord.Price != 5800 {
				t.Errorf("sell price = %f, want 5800 (bidPx[0])", ord.Price)
			}
		}
	}
	if !found {
		t.Error("expected a sell hedge order on leg2")
	}
	if pas.SellAggOrder != 1 {
		t.Errorf("SellAggOrder = %d, want 1", pas.SellAggOrder)
	}
	if pas.LastAggSide != types.Sell {
		t.Errorf("LastAggSide = %d, want Sell", pas.LastAggSide)
	}
}

func TestSendAggressiveOrder_ShortExposure_BuyHedge(t *testing.T) {
	pas := newTestPAS()
	pas.Leg1.State.NetposPass = -2

	pas.SendAggressiveOrder()

	found := false
	for _, ord := range pas.Leg2.Orders.OrdMap {
		if ord.Side == types.Buy && ord.OrdType == types.HitCross {
			found = true
			// Price should be at askPx[0] = 5801
			if ord.Price != 5801 {
				t.Errorf("buy price = %f, want 5801 (askPx[0])", ord.Price)
			}
		}
	}
	if !found {
		t.Error("expected a buy hedge order on leg2")
	}
	if pas.BuyAggOrder != 1 {
		t.Errorf("BuyAggOrder = %d, want 1", pas.BuyAggOrder)
	}
	if pas.LastAggSide != types.Buy {
		t.Errorf("LastAggSide = %d, want Buy", pas.LastAggSide)
	}
}

func TestSendAggressiveOrder_SupportingOrdersLimit(t *testing.T) {
	pas := newTestPAS()
	pas.Leg1.State.NetposPass = 3
	pas.Thold2.SupportingOrders = 1
	pas.SellAggOrder = 2 // > SupportingOrders=1

	pas.SendAggressiveOrder()

	// Should not place order because sellAggOrder > SUPPORTING_ORDERS
	if len(pas.Leg2.Orders.OrdMap) != 0 {
		t.Error("expected no orders when supporting orders limit exceeded")
	}
}

func TestSendAggressiveOrder_RetryLadder_Repeat1(t *testing.T) {
	pas := newTestPAS()
	pas.Leg1.State.NetposPass = 3
	pas.AggRepeat = 1
	pas.LastAggSide = types.Sell
	// Set to current time (within 500ms) to trigger the retry path
	pas.LastAggTS = uint64(time.Now().UnixMilli())

	pas.SendAggressiveOrder()

	// Should have placed a sell order with price = bidPx[0] - tickSize*1 = 5800 - 1 = 5799
	found := false
	for _, ord := range pas.Leg2.Orders.OrdMap {
		if ord.Side == types.Sell {
			found = true
			if ord.Price != 5799 {
				t.Errorf("retry sell price = %f, want 5799 (bidPx[0] - 1*tickSize)", ord.Price)
			}
		}
	}
	if !found {
		t.Error("expected a retry sell order")
	}
	if pas.AggRepeat != 2 {
		t.Errorf("AggRepeat = %d, want 2", pas.AggRepeat)
	}
}

func TestSendAggressiveOrder_RetryLadder_Repeat3_Slop(t *testing.T) {
	pas := newTestPAS()
	pas.Leg1.State.NetposPass = 3
	pas.AggRepeat = 3
	pas.LastAggSide = types.Sell
	pas.LastAggTS = uint64(time.Now().UnixMilli())
	pas.Thold2.Slop = 20

	pas.SendAggressiveOrder()

	// Should have placed sell at bidPx[0] - tickSize*SLOP = 5800 - 20 = 5780
	found := false
	for _, ord := range pas.Leg2.Orders.OrdMap {
		if ord.Side == types.Sell {
			found = true
			if ord.Price != 5780 {
				t.Errorf("slop sell price = %f, want 5780 (bidPx[0] - 20*tickSize)", ord.Price)
			}
		}
	}
	if !found {
		t.Error("expected a slop sell order")
	}
}

func TestSendAggressiveOrder_RetryExceeded_Squareoff(t *testing.T) {
	pas := newTestPAS()
	pas.Leg1.State.NetposPass = 3
	pas.AggRepeat = 4 // > 3
	pas.LastAggSide = types.Sell
	pas.LastAggTS = uint64(time.Now().UnixMilli())

	pas.SendAggressiveOrder()

	// Should have triggered squareoff
	if pas.Active {
		t.Error("should be deactivated after retry exceeded")
	}
	if !pas.Leg1.State.OnExit {
		t.Error("Leg1 OnExit should be true after squareoff")
	}
}

func TestSendAggressiveOrder_PendingAggIncluded(t *testing.T) {
	pas := newTestPAS()
	pas.Leg1.State.NetposPass = 3
	pas.Leg2.State.NetposAgg = -2

	// Add a pending CROSS sell for 1 on leg2
	pas.Leg2.Orders.OrdMap[600] = &types.OrderStats{
		OrderID: 600,
		Side:    types.Sell,
		OrdType: types.HitCross,
		OpenQty: 1,
	}

	// Net exposure = 3 + (-2) + (-1) = 0 → no order needed
	pas.SendAggressiveOrder()

	// Only the pre-existing order should be in the map
	if len(pas.Leg2.Orders.OrdMap) != 1 {
		t.Errorf("expected 1 order (pre-existing), got %d", len(pas.Leg2.Orders.OrdMap))
	}
}

func TestCalcPendingNetposAgg_Mixed(t *testing.T) {
	pas := newTestPAS()

	pas.Leg2.Orders.OrdMap[700] = &types.OrderStats{
		OrderID: 700, Side: types.Buy, OrdType: types.HitCross, OpenQty: 5,
	}
	pas.Leg2.Orders.OrdMap[701] = &types.OrderStats{
		OrderID: 701, Side: types.Sell, OrdType: types.HitMatch, OpenQty: 2,
	}
	pas.Leg2.Orders.OrdMap[702] = &types.OrderStats{
		OrderID: 702, Side: types.Buy, OrdType: types.HitStandard, OpenQty: 10,
	}

	pending := pas.CalcPendingNetposAgg()
	// +5 (buy cross) -2 (sell match) = 3, standard excluded
	if pending != 3 {
		t.Errorf("CalcPendingNetposAgg = %d, want 3", pending)
	}
}

func TestCalcPendingNetposAgg_Empty(t *testing.T) {
	pas := newTestPAS()
	pending := pas.CalcPendingNetposAgg()
	if pending != 0 {
		t.Errorf("CalcPendingNetposAgg = %d, want 0", pending)
	}
}
