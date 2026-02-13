package execution

import (
	"math"
	"testing"

	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/types"
)

func TestStateReset(t *testing.T) {
	s := &ExecutionState{
		Netpos:        10,
		BuyTotalQty:   100,
		SellTotalQty:  90,
		RealisedPNL:   500,
		UnrealisedPNL: 200,
		NetPNL:        700,
		TradeCount:    50,
		OrderCount:    100,
		Active:        true,
		OnExit:        true,
		BuyOpenOrders: 3,
	}

	s.Reset()

	if s.Netpos != 0 {
		t.Errorf("Netpos = %d, want 0", s.Netpos)
	}
	if s.BuyTotalQty != 0 {
		t.Errorf("BuyTotalQty = %f, want 0", s.BuyTotalQty)
	}
	if s.RealisedPNL != 0 {
		t.Errorf("RealisedPNL = %f, want 0", s.RealisedPNL)
	}
	if s.NetPNL != 0 {
		t.Errorf("NetPNL = %f, want 0", s.NetPNL)
	}
	if s.TradeCount != 0 {
		t.Errorf("TradeCount = %d, want 0", s.TradeCount)
	}
	if s.Active != false {
		t.Error("Active should be false after Reset")
	}
	if s.OnExit != false {
		t.Error("OnExit should be false after Reset")
	}
	if s.BuyOpenOrders != 0 {
		t.Errorf("BuyOpenOrders = %d, want 0", s.BuyOpenOrders)
	}
}

func TestCalculatePNL_Long(t *testing.T) {
	// 场景：持有多头 10 手，买入均价 5800，当前 bid=5810
	// priceMultiplier = 15
	// 无手续费
	inst := &instrument.Instrument{
		PriceMultiplier: 15.0,
	}
	inst.BidPx[0] = 5810.0
	inst.AskPx[0] = 5811.0

	s := &ExecutionState{
		Netpos:   10,
		BuyQty:   10,
		BuyPrice: 5800.0,
		BuyValue: 58000.0,
	}

	s.CalculatePNL(inst)

	// C++: netpos * ((bid - buyPrice - bid*sellExchTx) * mult - sellExchContractTx)
	// = 10 * ((5810 - 5800 - 0) * 15 - 0) = 10 * 150 = 1500
	expected := 10.0 * ((5810.0 - 5800.0) * 15.0)
	if math.Abs(s.UnrealisedPNL-expected) > 0.01 {
		t.Errorf("UnrealisedPNL = %f, want %f", s.UnrealisedPNL, expected)
	}
}

func TestCalculatePNL_Short(t *testing.T) {
	// 场景：持有空头 5 手，卖出均价 5820，当前 ask=5815
	inst := &instrument.Instrument{
		PriceMultiplier: 15.0,
	}
	inst.BidPx[0] = 5814.0
	inst.AskPx[0] = 5815.0

	s := &ExecutionState{
		Netpos:    -5,
		SellQty:   5,
		SellPrice: 5820.0,
		SellValue: 29100.0,
	}

	s.CalculatePNL(inst)

	// C++: -netpos * ((sellPrice - ask - ask*buyExchTx) * mult - buyExchContractTx)
	// = 5 * ((5820 - 5815) * 15) = 5 * 75 = 375
	expected := 5.0 * ((5820.0 - 5815.0) * 15.0)
	if math.Abs(s.UnrealisedPNL-expected) > 0.01 {
		t.Errorf("UnrealisedPNL = %f, want %f", s.UnrealisedPNL, expected)
	}
}

func TestCalculatePNL_Flat(t *testing.T) {
	inst := &instrument.Instrument{
		PriceMultiplier: 15.0,
	}
	inst.BidPx[0] = 5810.0
	inst.AskPx[0] = 5811.0

	s := &ExecutionState{
		Netpos: 0,
	}

	s.CalculatePNL(inst)

	if s.UnrealisedPNL != 0 {
		t.Errorf("UnrealisedPNL = %f, want 0 (flat)", s.UnrealisedPNL)
	}
}

func TestCalculatePNL_WithTransactionCosts(t *testing.T) {
	// 多头 10 手，买入均价 5800，bid=5810
	// buyExchTx=0, sellExchTx=0.00001, sellExchContractTx=5.0
	inst := &instrument.Instrument{
		PriceMultiplier: 15.0,
	}
	inst.BidPx[0] = 5810.0
	inst.AskPx[0] = 5811.0

	s := &ExecutionState{
		Netpos:             10,
		BuyQty:             10,
		BuyPrice:           5800.0,
		SellExchTx:         0.00001,
		SellExchContractTx: 5.0,
	}

	s.CalculatePNL(inst)

	// C++: 10 * ((5810 - 5800 - 5810*0.00001) * 15 - 5.0)
	// = 10 * ((10 - 0.0581) * 15 - 5) = 10 * (149.1285 - 5) = 10 * 144.1285 = 1441.285
	expected := 10.0 * ((5810.0 - 5800.0 - 5810.0*0.00001) * 15.0 - 5.0)
	if math.Abs(s.UnrealisedPNL-expected) > 0.01 {
		t.Errorf("UnrealisedPNL = %f, want %f", s.UnrealisedPNL, expected)
	}
}

func TestCalculatePNL_MaxPNLAndDrawdown(t *testing.T) {
	inst := &instrument.Instrument{PriceMultiplier: 15.0}
	inst.BidPx[0] = 5810.0
	inst.AskPx[0] = 5811.0

	s := &ExecutionState{
		Netpos:   10,
		BuyQty:   10,
		BuyPrice: 5800.0,
	}

	// First PNL calculation — establishes MaxPNL
	s.CalculatePNL(inst)
	if s.MaxPNL != s.NetPNL {
		t.Errorf("MaxPNL = %f, want %f", s.MaxPNL, s.NetPNL)
	}
	if s.Drawdown != 0 {
		t.Errorf("Drawdown = %f, want 0", s.Drawdown)
	}

	// Price drops — PNL decreases → drawdown
	inst.BidPx[0] = 5805.0
	s.CalculatePNL(inst)
	if s.Drawdown >= 0 {
		t.Errorf("Drawdown = %f, should be negative", s.Drawdown)
	}
}

func TestSetThresholds_Flat(t *testing.T) {
	s := &ExecutionState{Netpos: 0}
	thold := types.NewThresholdSet()
	thold.BeginPlace = 0.35
	thold.BeginRemove = 0.15
	thold.Size = 1
	thold.MaxSize = 5

	inst := &instrument.Instrument{SendInLots: true, LotSize: 15}

	s.SetThresholds(inst, thold)

	if s.TholdBidPlace != 0.35 {
		t.Errorf("TholdBidPlace = %f, want 0.35", s.TholdBidPlace)
	}
	if s.TholdAskPlace != 0.35 {
		t.Errorf("TholdAskPlace = %f, want 0.35", s.TholdAskPlace)
	}
	if s.TholdBidRemove != 0.15 {
		t.Errorf("TholdBidRemove = %f, want 0.15", s.TholdBidRemove)
	}
	if s.TholdMaxPos != 5 {
		t.Errorf("TholdMaxPos = %d, want 5", s.TholdMaxPos)
	}
	if s.TholdSize != 1 {
		t.Errorf("TholdSize = %d, want 1", s.TholdSize)
	}
}

func TestSetThresholds_Long(t *testing.T) {
	thold := types.NewThresholdSet()
	thold.BeginPlace = 0.35
	thold.BeginRemove = 0.15
	thold.LongPlace = 0.55
	thold.LongRemove = 0.30
	thold.ShortPlace = 0.20
	thold.ShortRemove = 0.10
	thold.Size = 1
	thold.MaxSize = 5
	thold.BeginSize = 1

	inst := &instrument.Instrument{SendInLots: true, LotSize: 15}
	s := &ExecutionState{Netpos: 3} // 大于 beginPos(1)

	s.SetThresholds(inst, thold)

	// 离散跳变到 LONG
	if s.TholdBidPlace != 0.55 {
		t.Errorf("TholdBidPlace = %f, want 0.55 (LONG_PLACE)", s.TholdBidPlace)
	}
	if s.TholdAskPlace != 0.20 {
		t.Errorf("TholdAskPlace = %f, want 0.20 (SHORT_PLACE)", s.TholdAskPlace)
	}
}

func TestSetThresholds_Short(t *testing.T) {
	thold := types.NewThresholdSet()
	thold.BeginPlace = 0.35
	thold.BeginRemove = 0.15
	thold.LongPlace = 0.55
	thold.LongRemove = 0.30
	thold.ShortPlace = 0.20
	thold.ShortRemove = 0.10
	thold.Size = 1
	thold.MaxSize = 5
	thold.BeginSize = 1

	inst := &instrument.Instrument{SendInLots: true, LotSize: 15}
	s := &ExecutionState{Netpos: -3}

	s.SetThresholds(inst, thold)

	// 空头：ask 用 LONG, bid 用 SHORT
	if s.TholdAskPlace != 0.55 {
		t.Errorf("TholdAskPlace = %f, want 0.55 (LONG_PLACE)", s.TholdAskPlace)
	}
	if s.TholdBidPlace != 0.20 {
		t.Errorf("TholdBidPlace = %f, want 0.20 (SHORT_PLACE)", s.TholdBidPlace)
	}
}

func TestSetLinearThresholds_Long(t *testing.T) {
	thold := types.NewThresholdSet()
	thold.BeginPlace = 0.35
	thold.BeginRemove = 0.15
	thold.LongPlace = 0.55
	thold.LongRemove = 0.30
	thold.ShortPlace = 0.20
	thold.ShortRemove = 0.10
	thold.Size = 1
	thold.MaxSize = 10
	thold.BeginSize = 1

	inst := &instrument.Instrument{SendInLots: true, LotSize: 15}
	s := &ExecutionState{Netpos: 5} // 50% of maxPos

	s.SetLinearThresholds(inst, thold)

	// bidPlace = BEGIN + (LONG - BEGIN) * 5/10 = 0.35 + 0.20*0.5 = 0.45
	expectedBidPlace := 0.35 + (0.55-0.35)*5.0/10.0
	if math.Abs(s.TholdBidPlace-expectedBidPlace) > 0.001 {
		t.Errorf("TholdBidPlace = %f, want %f", s.TholdBidPlace, expectedBidPlace)
	}

	// askPlace = BEGIN - (BEGIN - SHORT) * 5/10 = 0.35 - 0.15*0.5 = 0.275
	expectedAskPlace := 0.35 - (0.35-0.20)*5.0/10.0
	if math.Abs(s.TholdAskPlace-expectedAskPlace) > 0.001 {
		t.Errorf("TholdAskPlace = %f, want %f", s.TholdAskPlace, expectedAskPlace)
	}
}

func TestCheckSquareoff_Time(t *testing.T) {
	s := &ExecutionState{
		EndTimeEpoch: 1000000,
	}

	s.CheckSquareoff(999999, 0, nil, nil)
	if s.OnExit {
		t.Error("should not exit before EndTimeEpoch")
	}

	s.CheckSquareoff(1000001, 0, nil, nil)
	if !s.OnExit {
		t.Error("should exit after EndTimeEpoch")
	}
	if !s.OnCancel {
		t.Error("OnCancel should be true")
	}
	if !s.OnFlat {
		t.Error("OnFlat should be true")
	}
}

func TestCheckSquareoff_AggFlat(t *testing.T) {
	s := &ExecutionState{
		EndTimeAggEpoch: 500000,
	}

	s.CheckSquareoff(500001, 0, nil, nil)
	if !s.AggFlat {
		t.Error("AggFlat should be true after EndTimeAggEpoch")
	}
	if !s.OnExit {
		t.Error("OnExit should be true")
	}
}

func TestCheckSquareoff_MaxLoss(t *testing.T) {
	s := &ExecutionState{
		NetPNL: -1500,
	}

	// maxLoss=0 → no trigger
	s.CheckSquareoff(100, 0, nil, nil)
	if s.OnExit {
		t.Error("should not exit when maxLoss=0")
	}

	// maxLoss=2000, netPNL=-1500 → no trigger
	s.CheckSquareoff(100, 2000, nil, nil)
	if s.OnExit {
		t.Error("should not exit when loss < maxLoss")
	}

	// maxLoss=1000, netPNL=-1500 → trigger
	s.CheckSquareoff(100, 1000, nil, nil)
	if !s.OnExit {
		t.Error("should exit when loss > maxLoss")
	}
	if !s.OnFlat {
		t.Error("OnFlat should be true")
	}
}

func TestCheckSquareoff_MaxOrders(t *testing.T) {
	s := &ExecutionState{
		MaxOrderCount: 100,
		OrderCount:    99,
	}

	s.CheckSquareoff(100, 0, nil, nil)
	if s.OnExit {
		t.Error("should not exit when orderCount < maxOrderCount")
	}

	s.OrderCount = 100
	s.CheckSquareoff(100, 0, nil, nil)
	if !s.OnExit {
		t.Error("should exit when orderCount >= maxOrderCount")
	}
}

func TestCheckSquareoff_UPNLLoss(t *testing.T) {
	s := &ExecutionState{
		Netpos:        5,
		UnrealisedPNL: -800,
	}

	// upnlLoss=1000 → no trigger
	upnl := 1000.0
	s.CheckSquareoff(100, 0, &upnl, nil)
	if s.OnStopLoss {
		t.Error("should not trigger stop loss when upnl > -upnlLoss")
	}

	// upnlLoss=500, unrealisedPNL=-800 → trigger
	upnl = 500.0
	s.CheckSquareoff(100, 0, &upnl, nil)
	if !s.OnStopLoss {
		t.Error("should trigger stop loss when upnl < -upnlLoss")
	}
	if !s.OnFlat {
		t.Error("OnFlat should be true on stop loss")
	}
	if s.StopLossTS != 100 {
		t.Errorf("StopLossTS = %d, want 100", s.StopLossTS)
	}
	// C++: threshold should be doubled after trigger
	if upnl != 1000.0 {
		t.Errorf("upnlLoss = %f, want 1000 (doubled from 500)", upnl)
	}
}

func TestCheckSquareoff_StopLoss(t *testing.T) {
	s := &ExecutionState{
		Netpos:   -3,
		Drawdown: -600,
	}

	// stopLoss=1000 → no trigger
	sl := 1000.0
	s.CheckSquareoff(100, 0, nil, &sl)
	if s.OnStopLoss {
		t.Error("should not trigger when drawdown > -stopLoss")
	}

	// stopLoss=500, drawdown=-600 → trigger
	sl = 500.0
	s.CheckSquareoff(100, 0, nil, &sl)
	if !s.OnStopLoss {
		t.Error("should trigger stop loss when drawdown < -stopLoss")
	}
	// C++: threshold should be doubled
	if sl != 1000.0 {
		t.Errorf("stopLoss = %f, want 1000 (doubled from 500)", sl)
	}
}

func TestCheckSquareoff_AutoResume(t *testing.T) {
	// Simulate stop loss triggered at time 100
	s := &ExecutionState{
		OnStopLoss: true,
		StopLossTS: 100_000_000_000, // 100 seconds in ns
		OnExit:     false,
		OnCancel:   true,
		OnFlat:     true,
	}

	// Before 15-min cool-off (900 seconds = 900_000_000_000 ns)
	s.CheckSquareoff(500_000_000_000, 0, nil, nil) // 500 seconds
	if !s.OnStopLoss {
		t.Error("should still be on stop loss before 15-min cool-off")
	}

	// After 15-min cool-off
	s.CheckSquareoff(1_000_000_000_000, 0, nil, nil) // 1000 seconds
	if s.OnStopLoss {
		t.Error("should auto-resume after 15-min cool-off")
	}
	if s.OnExit {
		t.Error("OnExit should be false after auto-resume")
	}
	if s.OnCancel {
		t.Error("OnCancel should be false after auto-resume")
	}
	if s.OnFlat {
		t.Error("OnFlat should be false after auto-resume")
	}
}

func TestCheckRejectLimit(t *testing.T) {
	s := &ExecutionState{RejectCount: 199}
	if s.CheckRejectLimit() {
		t.Error("should not exceed limit at 199")
	}

	s.RejectCount = 200
	if !s.CheckRejectLimit() {
		t.Error("should exceed limit at 200")
	}
}

func TestHandleRMSReject(t *testing.T) {
	inst := &instrument.Instrument{LotSize: 15}

	s := &ExecutionState{RmsQty: 10}
	s.HandleRMSReject(inst)
	if !s.AggFlat {
		t.Error("AggFlat should be true")
	}
	if s.RmsQty != 5 {
		t.Errorf("RmsQty = %d, want 5 (halved)", s.RmsQty)
	}

	// Halve again → 2
	s.HandleRMSReject(inst)
	if s.RmsQty != 2 {
		t.Errorf("RmsQty = %d, want 2", s.RmsQty)
	}

	// Halve again → 1
	s.HandleRMSReject(inst)
	if s.RmsQty != 1 {
		t.Errorf("RmsQty = %d, want 1", s.RmsQty)
	}

	// Halve again → 0, then reset to lotSize
	s.HandleRMSReject(inst)
	if s.RmsQty != 15 {
		t.Errorf("RmsQty = %d, want 15 (lotSize)", s.RmsQty)
	}
}
