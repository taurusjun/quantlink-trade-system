package types

import (
	"testing"
)

// TestEnumValues 验证枚举值与 C++ 完全一致
func TestEnumValues(t *testing.T) {
	// OrderStatus — ExecutionStrategyStructs.h:20
	tests := []struct {
		name string
		got  OrderStatus
		want int32
	}{
		{"NEW_ORDER", StatusNewOrder, 0},
		{"NEW_CONFIRM", StatusNewConfirm, 1},
		{"NEW_REJECT", StatusNewReject, 2},
		{"MODIFY_ORDER", StatusModifyOrder, 3},
		{"MODIFY_CONFIRM", StatusModifyConfirm, 4},
		{"MODIFY_REJECT", StatusModifyReject, 5},
		{"CANCEL_ORDER", StatusCancelOrder, 6},
		{"CANCEL_CONFIRM", StatusCancelConfirm, 7},
		{"CANCEL_REJECT", StatusCancelReject, 8},
		{"TRADED", StatusTraded, 9},
		{"INIT", StatusInit, 10},
	}
	for _, tt := range tests {
		if int32(tt.got) != tt.want {
			t.Errorf("OrderStatus %s = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}

func TestOrderHitTypeValues(t *testing.T) {
	// OrderHitType — ExecutionStrategyStructs.h:35
	tests := []struct {
		name string
		got  OrderHitType
		want int32
	}{
		{"STANDARD", HitStandard, 0},
		{"IMPROVE", HitImprove, 1},
		{"CROSS", HitCross, 2},
		{"DETECT", HitDetect, 3},
		{"MATCH", HitMatch, 4},
	}
	for _, tt := range tests {
		if int32(tt.got) != tt.want {
			t.Errorf("OrderHitType %s = %d, want %d", tt.name, tt.got, tt.want)
		}
	}
}

func TestTransactionTypeValues(t *testing.T) {
	// TransactionType — ORSBase.h:9
	if int32(Buy) != 1 {
		t.Errorf("Buy = %d, want 1", Buy)
	}
	if int32(Sell) != 2 {
		t.Errorf("Sell = %d, want 2", Sell)
	}
}

func TestTypeOfOrderValues(t *testing.T) {
	// TypeOfOrder — ORSBase.h:15
	if int32(Quote) != 0 {
		t.Errorf("Quote = %d, want 0", Quote)
	}
	if int32(PHedge) != 1 {
		t.Errorf("PHedge = %d, want 1", PHedge)
	}
	if int32(AHedge) != 2 {
		t.Errorf("AHedge = %d, want 2", AHedge)
	}
}

func TestNewOrderStats(t *testing.T) {
	ord := NewOrderStats(12345, Buy, 5800.0, 10, Quote, HitStandard)

	if ord.OrderID != 12345 {
		t.Errorf("OrderID = %d, want 12345", ord.OrderID)
	}
	if ord.Side != Buy {
		t.Errorf("Side = %d, want Buy", ord.Side)
	}
	if ord.Price != 5800.0 {
		t.Errorf("Price = %f, want 5800.0", ord.Price)
	}
	if ord.Qty != 10 {
		t.Errorf("Qty = %d, want 10", ord.Qty)
	}
	if ord.OpenQty != 10 {
		t.Errorf("OpenQty = %d, want 10", ord.OpenQty)
	}
	if ord.DoneQty != 0 {
		t.Errorf("DoneQty = %d, want 0", ord.DoneQty)
	}
	if ord.Status != StatusNewOrder {
		t.Errorf("Status = %d, want StatusNewOrder", ord.Status)
	}
	if ord.Active != false {
		t.Errorf("Active = %v, want false", ord.Active)
	}
	if ord.New != true {
		t.Errorf("New = %v, want true", ord.New)
	}
}

func TestThresholdSetDefaults(t *testing.T) {
	ts := NewThresholdSet()

	// 布尔默认值
	if ts.ClosePNL != true {
		t.Error("ClosePNL should default to true")
	}
	if ts.CheckPNL != true {
		t.Error("CheckPNL should default to true")
	}
	if ts.UseNotional != false {
		t.Error("UseNotional should default to false")
	}

	// 数值默认值
	if ts.MaxOSOrder != 5 {
		t.Errorf("MaxOSOrder = %d, want 5", ts.MaxOSOrder)
	}
	if ts.PercentLevel != 1 {
		t.Errorf("PercentLevel = %d, want 1", ts.PercentLevel)
	}
	if ts.StopLoss != 10000000000 {
		t.Errorf("StopLoss = %f, want 10000000000", ts.StopLoss)
	}
	if ts.MaxLoss != 100000000000 {
		t.Errorf("MaxLoss = %f, want 100000000000", ts.MaxLoss)
	}
	if ts.Cross != 1000000000 {
		t.Errorf("Cross = %f, want 1000000000", ts.Cross)
	}
	if ts.SpreadEWA != 0.6 {
		t.Errorf("SpreadEWA = %f, want 0.6", ts.SpreadEWA)
	}
	if ts.AvgSpreadAway != 20 {
		t.Errorf("AvgSpreadAway = %d, want 20", ts.AvgSpreadAway)
	}
	if ts.Slop != 20 {
		t.Errorf("Slop = %d, want 20", ts.Slop)
	}
	if ts.TVarKey != -1 {
		t.Errorf("TVarKey = %d, want -1", ts.TVarKey)
	}
	if ts.TCacheKey != -1 {
		t.Errorf("TCacheKey = %d, want -1", ts.TCacheKey)
	}
	if ts.MaxQuoteLevel != 3 {
		t.Errorf("MaxQuoteLevel = %d, want 3", ts.MaxQuoteLevel)
	}
	if ts.VWAPRatio != 1 {
		t.Errorf("VWAPRatio = %f, want 1", ts.VWAPRatio)
	}
}
