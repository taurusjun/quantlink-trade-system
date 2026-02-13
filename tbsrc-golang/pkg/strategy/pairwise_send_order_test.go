package strategy

import (
	"testing"

	"tbsrc-golang/pkg/types"
)

func TestSendOrder_ThresholdsInvalid_EarlyReturn(t *testing.T) {
	pas := newTestPAS()
	// TholdSize=0 means SetThresholds won't set thresholds (all stay -1)
	// With Size=0 and MaxSize=0, computeSizing produces TholdMaxPos=0 and TholdSize=0
	pas.Thold1.Size = 0
	pas.Thold1.MaxSize = 0

	// Should not panic; threshold=-1 means early return
	pas.SendOrder()

	// No orders should have been placed
	if len(pas.Leg1.Orders.OrdMap) != 0 {
		t.Errorf("expected no orders, got %d", len(pas.Leg1.Orders.OrdMap))
	}
}

func TestSendOrder_ZeroPriceProtection(t *testing.T) {
	pas := newTestPAS()

	// Make thresholds valid
	pas.Leg1.State.TholdSize = 1
	pas.Leg1.State.TholdMaxPos = 5

	// Zero out inst2 bid
	pas.Inst2.BidPx[0] = 0

	pas.SendOrder()

	// Should return before phase 5
	if len(pas.Leg1.Orders.OrdMap) != 0 {
		t.Errorf("expected no orders with zero price, got %d", len(pas.Leg1.Orders.OrdMap))
	}
}

func TestSendOrder_PlacesAskWhenSpreadWide(t *testing.T) {
	pas := newTestPAS()

	// Setup: spread is wide enough to trigger ask placement
	// shortSpread = inst1.askPx[0] - inst2.askPx[0] = 5811 - 5801 = 10
	// avgSpread = 10, askPlace = 2 → 10 > 10+2 = 12? No.
	// Need to widen: set avgSpread lower
	pas.Spread.Seed(5.0) // avgSpread = 5, shortSpread=10 > 5+2=7 → yes

	pas.SendOrder()

	// Should have placed at least one ask order
	if len(pas.Leg1.Orders.AskMap) == 0 {
		t.Error("expected ask orders when spread is wide")
	}
}

func TestSendOrder_PlacesBidWhenSpreadTight(t *testing.T) {
	pas := newTestPAS()

	// longSpread = inst1.bidPx[0] - inst2.bidPx[0] = 5810 - 5800 = 10
	// avgSpread = 10, bidPlace = 2 → 10 < 10-2 = 8? No.
	// Need to make avgSpread high: set avgSpread = 15
	pas.Spread.Seed(15.0) // longSpread=10 < 15-2=13 → yes

	pas.SendOrder()

	// Should have placed at least one bid order
	if len(pas.Leg1.Orders.BidMap) == 0 {
		t.Error("expected bid orders when spread is tight")
	}
}

func TestSendOrder_NoOrderWhenSpreadNeutral(t *testing.T) {
	pas := newTestPAS()

	// Spread exactly at avgSpread — neither condition triggers
	// longSpread = 10, shortSpread = 10, avgSpread = 10
	// 10 < 10-2=8? No. 10 > 10+2=12? No.
	pas.Spread.Seed(10.0)

	pas.SendOrder()

	if len(pas.Leg1.Orders.BidMap) != 0 || len(pas.Leg1.Orders.AskMap) != 0 {
		t.Error("expected no orders when spread is neutral")
	}
}

func TestSendOrder_MaxOSOrderLimit(t *testing.T) {
	pas := newTestPAS()
	pas.Spread.Seed(5.0) // 触发 ask placement
	pas.Thold1.MaxOSOrder = 1

	// Pre-fill one ask order so SellOpenOrders=1
	pas.Leg1.State.SellOpenOrders = 1

	pas.SendOrder()

	// Should attempt cancelWorstAskIfBetter, not place new orders directly
	// Since there's no existing order in AskMap to cancel, no new order is placed
	// (cancelWorstAskIfBetter finds nothing to cancel)
	// The exact behavior depends on the state; the key point is no panic
}

func TestSendOrder_PositionLimitBlocksAsk(t *testing.T) {
	pas := newTestPAS()
	pas.Spread.Seed(5.0) // 触发 ask placement

	// AskMaxSize=5 (in lots mode) → tholdAskMaxPos=5
	// NetposPass=-6 → -(-6)=6 >= 5 → blocked
	pas.Inst1.SendInLots = true
	pas.Inst2.SendInLots = true
	pas.Thold1.AskMaxSize = 5
	pas.Thold1.BidMaxSize = 5
	pas.Thold1.AskSize = 1
	pas.Thold1.BidSize = 1
	pas.Leg1.State.NetposPass = -6

	pas.SendOrder()

	// No ask orders should be placed due to position limit;
	// instead all existing asks should be cancelled
	if len(pas.Leg1.Orders.AskMap) != 0 {
		t.Error("expected no ask orders when position limit exceeded")
	}
}

func TestSendOrder_CancelCrossOrders(t *testing.T) {
	pas := newTestPAS()

	// Add a CROSS order to leg1
	pas.Leg1.Orders.OrdMap[200] = &types.OrderStats{
		OrderID: 200,
		OrdType: types.HitCross,
		Side:    types.Buy,
		Price:   5810,
		Status:  types.StatusNewConfirm,
	}
	pas.Leg1.Orders.BidMap[5810] = pas.Leg1.Orders.OrdMap[200]
	pas.Leg1.State.BuyOpenOrders = 1

	pas.SendOrder()

	// The CROSS order should have been cancelled (status=CancelOrder)
	ord := pas.Leg1.Orders.OrdMap[200]
	if ord.Status != types.StatusCancelOrder {
		t.Errorf("CROSS order status = %d, want %d (CancelOrder)", ord.Status, types.StatusCancelOrder)
	}
}

func TestSendOrder_CancelOutOfRangeOrders(t *testing.T) {
	pas := newTestPAS()
	pas.Spread.Seed(10.0) // avgSpread=10

	// Add a bid order that's too tight:
	// longSpread = 5810 - 5800 = 10, bidRemove=1 → 10 > 10-1=9 → cancel
	pas.Leg1.Orders.OrdMap[300] = &types.OrderStats{
		OrderID: 300,
		OrdType: types.HitStandard,
		Side:    types.Buy,
		Price:   5810,
		Status:  types.StatusNewConfirm,
		OpenQty: 1,
	}
	pas.Leg1.Orders.BidMap[5810] = pas.Leg1.Orders.OrdMap[300]
	pas.Leg1.State.BuyOpenOrders = 1

	pas.SendOrder()

	ord := pas.Leg1.Orders.OrdMap[300]
	if ord.Status != types.StatusCancelOrder {
		t.Errorf("out-of-range bid order status = %d, want %d (CancelOrder)", ord.Status, types.StatusCancelOrder)
	}
}

func TestSendOrder_HedgeLeg2_LongExposure(t *testing.T) {
	pas := newTestPAS()
	pas.Spread.Seed(10.0) // neutral spread

	// Create a long exposure
	pas.Leg1.State.NetposPass = 3
	// netExposure = 3 + 0 + 0 = 3 > 0 → sell on leg2

	pas.SendOrder()

	// Should have placed a sell order on leg2
	found := false
	for _, ord := range pas.Leg2.Orders.OrdMap {
		if ord.Side == types.Sell && ord.OrdType == types.HitCross {
			found = true
			if ord.OpenQty != 3 {
				t.Errorf("hedge qty = %d, want 3", ord.OpenQty)
			}
		}
	}
	if !found {
		t.Error("expected a CROSS sell order on leg2 for hedging")
	}
	if pas.SellAggOrder != 1 {
		t.Errorf("SellAggOrder = %d, want 1", pas.SellAggOrder)
	}
}

func TestSendOrder_HedgeLeg2_ShortExposure(t *testing.T) {
	pas := newTestPAS()
	pas.Spread.Seed(10.0)

	// Create a short exposure
	pas.Leg1.State.NetposPass = -2

	pas.SendOrder()

	// Should have placed a buy order on leg2
	found := false
	for _, ord := range pas.Leg2.Orders.OrdMap {
		if ord.Side == types.Buy && ord.OrdType == types.HitCross {
			found = true
			if ord.OpenQty != 2 {
				t.Errorf("hedge qty = %d, want 2", ord.OpenQty)
			}
		}
	}
	if !found {
		t.Error("expected a CROSS buy order on leg2 for hedging")
	}
	if pas.BuyAggOrder != 1 {
		t.Errorf("BuyAggOrder = %d, want 1", pas.BuyAggOrder)
	}
}

func TestSendOrder_MultiLevel(t *testing.T) {
	pas := newTestPAS()
	pas.MaxQuoteLevel = 3
	// SUPPORTING_ORDERS must be >= 2 so multiple outstanding orders are allowed
	// (with 0, after placing 1 order the next level enters cancel-worst path)
	pas.Thold1.SupportingOrders = 5
	// Low avgSpread so multiple levels trigger ask placement
	// level0: shortSpread = 5811 - 5801 = 10 > 3+2 = 5 → yes
	// level1: shortSpread = 5812 - 5801 = 11 > 3+2 = 5 → yes
	// level2: shortSpread = 5813 - 5801 = 12 > 3+2 = 5 → yes
	pas.Spread.Seed(3.0)

	pas.SendOrder()

	// Should have placed ask orders at multiple levels
	if len(pas.Leg1.Orders.AskMap) < 2 {
		t.Errorf("expected multiple ask orders, got %d", len(pas.Leg1.Orders.AskMap))
	}
}
