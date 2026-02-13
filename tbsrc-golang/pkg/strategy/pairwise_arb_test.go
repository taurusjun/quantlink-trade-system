package strategy

import (
	"testing"

	"tbsrc-golang/pkg/execution"
	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/types"
)

// newTestInstrument 创建测试用合约（2 档有效行情）
func newTestInstrument(symbol string, tickSize, lotSize float64) *instrument.Instrument {
	inst := &instrument.Instrument{
		Symbol:          symbol,
		Exchange:        "SHFE",
		TickSize:        tickSize,
		LotSize:         lotSize,
		ContractFactor:  lotSize,
		PriceMultiplier: lotSize,
		PriceFactor:     1,
		ValidBids:       3,
		ValidAsks:       3,
	}
	return inst
}

// setBookLevels 设置合约行情簿
func setBookLevels(inst *instrument.Instrument, bidPx, askPx []float64, bidQty, askQty []float64) {
	for i := range bidPx {
		inst.BidPx[i] = bidPx[i]
		inst.BidQty[i] = bidQty[i]
	}
	for i := range askPx {
		inst.AskPx[i] = askPx[i]
		inst.AskQty[i] = askQty[i]
	}
}

// newTestPAS 创建测试用 PairwiseArbStrategy
func newTestPAS() *PairwiseArbStrategy {
	inst1 := newTestInstrument("ag2506", 1.0, 15)
	inst2 := newTestInstrument("ag2512", 1.0, 15)

	// 设置行情簿
	setBookLevels(inst1,
		[]float64{5810, 5809, 5808},
		[]float64{5811, 5812, 5813},
		[]float64{10, 20, 30},
		[]float64{10, 20, 30},
	)
	setBookLevels(inst2,
		[]float64{5800, 5799, 5798},
		[]float64{5801, 5802, 5803},
		[]float64{10, 20, 30},
		[]float64{10, 20, 30},
	)

	thold1 := types.NewThresholdSet()
	thold1.BeginPlace = 2.0
	thold1.BeginRemove = 1.0
	thold1.LongPlace = 3.0
	thold1.LongRemove = 2.0
	thold1.ShortPlace = 1.5
	thold1.ShortRemove = 0.5
	thold1.Size = 1
	thold1.MaxSize = 5
	thold1.MaxOSOrder = 3
	thold1.Alpha = 0.01

	thold2 := types.NewThresholdSet()
	thold2.Size = 1
	thold2.MaxSize = 10

	leg1 := execution.NewLegManager(nil, inst1, thold1, 92201, "TEST")
	leg2 := execution.NewLegManager(nil, inst2, thold2, 92201, "TEST")

	spread := NewSpreadTracker(thold1.Alpha, inst1.TickSize, int32(thold1.AvgSpreadAway))
	// Seed spread: mid1=5810.5, mid2=5800.5, spread=10.0
	spread.Seed(10.0)

	return &PairwiseArbStrategy{
		Leg1:          leg1,
		Leg2:          leg2,
		Spread:        spread,
		Inst1:         inst1,
		Inst2:         inst2,
		Thold1:        thold1,
		Thold2:        thold2,
		StrategyID:    92201,
		Account:       "TEST",
		MaxQuoteLevel: 3,
		AggRepeat:     1,
		Active:        true,
	}
}

func TestNewPairwiseArbStrategy(t *testing.T) {
	inst1 := newTestInstrument("ag2506", 1.0, 15)
	inst2 := newTestInstrument("ag2512", 1.0, 15)
	thold1 := types.NewThresholdSet()
	thold1.Alpha = 0.01
	thold2 := types.NewThresholdSet()

	pas := NewPairwiseArbStrategy(nil, inst1, inst2, thold1, thold2, 92201, "TEST")

	if pas.Leg1 == nil {
		t.Fatal("Leg1 should not be nil")
	}
	if pas.Leg2 == nil {
		t.Fatal("Leg2 should not be nil")
	}
	if pas.Spread == nil {
		t.Fatal("Spread should not be nil")
	}
	if pas.MaxQuoteLevel != 3 {
		t.Errorf("MaxQuoteLevel = %d, want 3", pas.MaxQuoteLevel)
	}
	if pas.AggRepeat != 1 {
		t.Errorf("AggRepeat = %d, want 1", pas.AggRepeat)
	}
}

func TestPairwiseArb_Init(t *testing.T) {
	pas := newTestPAS()
	pas.Init(10.5, 2, 1, -3)

	if pas.Spread.AvgSpreadOri != 10.5 {
		t.Errorf("AvgSpreadOri = %f, want 10.5", pas.Spread.AvgSpreadOri)
	}
	if pas.Leg1.State.NetposPass != 3 { // ytd=2 + 2day=1
		t.Errorf("Leg1.NetposPass = %d, want 3", pas.Leg1.State.NetposPass)
	}
	if pas.Leg1.State.NetposPassYtd != 2 {
		t.Errorf("Leg1.NetposPassYtd = %d, want 2", pas.Leg1.State.NetposPassYtd)
	}
	if pas.Leg2.State.NetposAgg != -3 {
		t.Errorf("Leg2.NetposAgg = %d, want -3", pas.Leg2.State.NetposAgg)
	}
}

func TestPairwiseArb_SetActive(t *testing.T) {
	pas := newTestPAS()
	pas.SetActive(true)

	if !pas.Active {
		t.Error("Active should be true")
	}
	if !pas.Leg1.State.Active {
		t.Error("Leg1.State.Active should be true")
	}
	if !pas.Leg2.State.Active {
		t.Error("Leg2.State.Active should be true")
	}

	pas.SetActive(false)
	if pas.Active {
		t.Error("Active should be false")
	}
}

func TestPairwiseArb_NetExposure_Flat(t *testing.T) {
	pas := newTestPAS()
	exp := pas.NetExposure()
	if exp != 0 {
		t.Errorf("NetExposure = %d, want 0", exp)
	}
}

func TestPairwiseArb_NetExposure_WithPositions(t *testing.T) {
	pas := newTestPAS()
	pas.Leg1.State.NetposPass = 5
	pas.Leg2.State.NetposAgg = -3
	// No pending orders

	exp := pas.NetExposure()
	if exp != 2 {
		t.Errorf("NetExposure = %d, want 2 (5 + -3 + 0)", exp)
	}
}

func TestPairwiseArb_HandleSquareoff(t *testing.T) {
	pas := newTestPAS()
	pas.SetActive(true)

	pas.HandleSquareoff()

	if pas.Active {
		t.Error("should be deactivated after squareoff")
	}
	if !pas.Leg1.State.OnExit {
		t.Error("Leg1 OnExit should be true")
	}
	if !pas.Leg1.State.OnCancel {
		t.Error("Leg1 OnCancel should be true")
	}
	if !pas.Leg2.State.OnExit {
		t.Error("Leg2 OnExit should be true")
	}
}

func TestPairwiseArb_HandleSquareON(t *testing.T) {
	pas := newTestPAS()
	pas.HandleSquareoff() // deactivate first

	pas.HandleSquareON()

	if !pas.Active {
		t.Error("should be active after SquareON")
	}
	if pas.Leg1.State.OnExit {
		t.Error("Leg1 OnExit should be false")
	}
	if pas.AggRepeat != 1 {
		t.Errorf("AggRepeat = %d, want 1", pas.AggRepeat)
	}
}

func TestPairwiseArb_CalcPendingNetposAgg(t *testing.T) {
	pas := newTestPAS()

	// 添加 leg2 订单
	pas.Leg2.Orders.OrdMap[100] = &types.OrderStats{
		OrderID: 100, Side: types.Buy, OpenQty: 3, OrdType: types.HitCross,
	}
	pas.Leg2.Orders.OrdMap[101] = &types.OrderStats{
		OrderID: 101, Side: types.Sell, OpenQty: 1, OrdType: types.HitCross,
	}
	// STANDARD 订单不应被计入
	pas.Leg2.Orders.OrdMap[102] = &types.OrderStats{
		OrderID: 102, Side: types.Buy, OpenQty: 10, OrdType: types.HitStandard,
	}

	pending := pas.CalcPendingNetposAgg()
	if pending != 2 { // +3 - 1 = 2, STANDARD excluded
		t.Errorf("CalcPendingNetposAgg = %d, want 2", pending)
	}
}
