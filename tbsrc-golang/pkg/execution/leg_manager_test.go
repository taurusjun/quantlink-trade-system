package execution

import (
	"math"
	"testing"

	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/shm"
	"tbsrc-golang/pkg/types"
)

// newTestLegManager 创建不依赖真实 SHM 的测试 LegManager
func newTestLegManager() (*LegManager, *instrument.Instrument) {
	inst := &instrument.Instrument{
		Symbol:          "ag2506",
		Exchange:        "SHFE",
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

	state := &ExecutionState{}
	om := &OrderManager{
		OrdMap: make(map[uint32]*types.OrderStats),
		BidMap: make(map[float64]*types.OrderStats),
		AskMap: make(map[float64]*types.OrderStats),
		Client: nil,
		State:  state,
	}

	thold := types.NewThresholdSet()
	thold.Size = 1
	thold.MaxSize = 5
	thold.BeginSize = 1
	thold.BeginPlace = 0.35
	thold.BeginRemove = 0.15
	thold.LongPlace = 0.55
	thold.LongRemove = 0.30
	thold.ShortPlace = 0.20
	thold.ShortRemove = 0.10

	lm := &LegManager{
		State:      state,
		Orders:     om,
		Inst:       inst,
		Thold:      thold,
		Client:     nil,
		StrategyID: 92201,
		Account:    "PRP05",
	}

	return lm, inst
}

// TestLegManager_FullCycle 完整的发单→确认→成交→持仓→PNL 周期
func TestLegManager_FullCycle(t *testing.T) {
	lm, _ := newTestLegManager()

	// 手动插入买单（因为没有真实 client）
	ord := insertOrder(lm.Orders, 1001, types.Buy, 5819.0, 1, types.HitStandard)
	lm.State.TholdSize = 1 // 需要设置阈值大小

	// 确认
	lm.ORSCallBack(&shm.ResponseMsg{
		Response_Type: shm.NEW_ORDER_CONFIRM,
		OrderID:       1001,
	})

	if ord.Status != types.StatusNewConfirm {
		t.Errorf("Status = %d, want NewConfirm", ord.Status)
	}

	// 成交
	lm.ORSCallBack(&shm.ResponseMsg{
		Response_Type: shm.TRADE_CONFIRM,
		OrderID:       1001,
		Price:         5819.0,
		Quantity:      1,
	})

	// 验证持仓
	if lm.State.Netpos != 1 {
		t.Errorf("Netpos = %d, want 1", lm.State.Netpos)
	}
	if lm.State.BuyTotalQty != 1 {
		t.Errorf("BuyTotalQty = %f, want 1", lm.State.BuyTotalQty)
	}

	// 验证 PNL 已计算（不为 NaN）
	if math.IsNaN(lm.State.UnrealisedPNL) {
		t.Error("UnrealisedPNL should not be NaN")
	}
}

// TestLegManager_MDCallBack_RecalcPNL BBO 变化时重算 PNL
func TestLegManager_MDCallBack_RecalcPNL(t *testing.T) {
	lm, inst := newTestLegManager()

	// 设置已有多头持仓
	lm.State.Netpos = 10
	lm.State.BuyQty = 10
	lm.State.BuyPrice = 5810.0
	lm.State.BuyValue = 58100.0
	lm.State.BestBidLastPNL = 0 // 强制首次重算

	md := &shm.MarketUpdateNew{}
	md.Data.BidUpdates[0] = shm.BookElement{Price: 5819.0, Quantity: 100}
	md.Data.AskUpdates[0] = shm.BookElement{Price: 5820.0, Quantity: 80}
	md.Data.ValidBids = 1
	md.Data.ValidAsks = 1
	md.Data.LastTradedPrice = 5819.5

	// 先更新行情
	inst.UpdateFromMD(md)

	// 调用 MDCallBack
	lm.MDCallBack(inst, md)

	// PNL 应该已更新
	if lm.State.BestBidLastPNL != 5819.0 {
		t.Errorf("BestBidLastPNL = %f, want 5819.0", lm.State.BestBidLastPNL)
	}
	if lm.State.LTP != 5819.5 {
		t.Errorf("LTP = %f, want 5819.5", lm.State.LTP)
	}

	// 第二次相同 BBO — 不应重算
	prevPNL := lm.State.UnrealisedPNL
	lm.MDCallBack(inst, md)
	if lm.State.UnrealisedPNL != prevPNL {
		t.Errorf("PNL should not change on same BBO")
	}
}

// TestLegManager_MDCallBack_BBOChange BBO 变化触发 PNL 重算
func TestLegManager_MDCallBack_BBOChange(t *testing.T) {
	lm, inst := newTestLegManager()

	lm.State.Netpos = 5
	lm.State.BuyQty = 5
	lm.State.BuyPrice = 5815.0
	lm.State.BuyValue = 29075.0
	lm.State.BestBidLastPNL = 5819.0
	lm.State.BestAskLastPNL = 5820.0

	// 模拟 BBO 变化
	md1 := &shm.MarketUpdateNew{}
	md1.Data.BidUpdates[0] = shm.BookElement{Price: 5821.0, Quantity: 100}
	md1.Data.AskUpdates[0] = shm.BookElement{Price: 5822.0, Quantity: 80}
	md1.Data.ValidBids = 1
	md1.Data.ValidAsks = 1

	inst.UpdateFromMD(md1)
	lm.MDCallBack(inst, md1)

	// BBO 变了，应该重算
	if lm.State.BestBidLastPNL != 5821.0 {
		t.Errorf("BestBidLastPNL = %f, want 5821.0", lm.State.BestBidLastPNL)
	}
	// PNL 应该是正的（bid 上涨）
	// C++: 5 * ((5821 - 5815) * 15) = 5 * 90 = 450
	expectedPNL := 5.0 * ((5821.0 - 5815.0) * 15.0)
	if math.Abs(lm.State.UnrealisedPNL-expectedPNL) > 0.01 {
		t.Errorf("UnrealisedPNL = %f, want %f", lm.State.UnrealisedPNL, expectedPNL)
	}
}

// TestLegManager_HandleSquareoff_Flat 已平仓时停用
func TestLegManager_HandleSquareoff_Flat(t *testing.T) {
	lm, _ := newTestLegManager()

	lm.State.Netpos = 0
	lm.State.OnExit = true
	lm.State.Active = true

	lm.HandleSquareoff()

	if lm.State.Active {
		t.Error("should be deactivated when flat + exit")
	}
}

// TestLegManager_HandleSquareoff_Long 多头平仓
func TestLegManager_HandleSquareoff_Long(t *testing.T) {
	lm, inst := newTestLegManager()

	lm.State.Netpos = 5
	lm.State.OnExit = true
	lm.State.OnCancel = true
	lm.State.AggFlat = false
	lm.State.Active = true

	// 确保 ask price 有效
	inst.AskPx[0] = 5820.0
	inst.BidPx[0] = 5819.0

	// 无挂单时直接发送平仓单
	// 由于没有真实 client，验证 HandleSquareoff 不崩溃
	// 并且 OnCancel 被清除
	lm.HandleSquareoff()

	if lm.State.OnCancel {
		t.Error("OnCancel should be false after HandleSquareoff")
	}
}

// TestLegManager_HandleSquareoff_AggFlat 激进平仓
func TestLegManager_HandleSquareoff_AggFlat(t *testing.T) {
	lm, inst := newTestLegManager()

	lm.State.Netpos = 3
	lm.State.OnExit = true
	lm.State.AggFlat = true
	lm.State.Active = true

	inst.AskPx[0] = 5820.0
	inst.BidPx[0] = 5819.0

	// 激进平仓应计算穿越价
	// sellPrice = bidPx - tickSize = 5818
	// HandleSquareoff 会尝试发单但没有真实 client，这里验证不崩溃
	lm.HandleSquareoff()

	if lm.State.OnCancel {
		t.Error("OnCancel should be false")
	}
}

// TestLegManager_HandleSquareoff_WithPendingOrders 有挂单时先撤
func TestLegManager_HandleSquareoff_WithPendingOrders(t *testing.T) {
	lm, _ := newTestLegManager()

	lm.State.Netpos = 5
	lm.State.OnExit = true
	lm.State.OnCancel = true
	lm.State.Active = true

	// 插入一个挂单
	ord := insertOrder(lm.Orders, 20001, types.Buy, 5818.0, 1, types.HitStandard)
	ord.Status = types.StatusNewConfirm

	// HandleSquareoff 会尝试撤单，但因为没有真实 client 会失败
	// 关键是不崩溃，且 OnCancel 被清除
	lm.HandleSquareoff()

	if lm.State.OnCancel {
		t.Error("OnCancel should be false after HandleSquareoff")
	}
}

// TestLegManager_Reset 重置清空所有状态和订单
func TestLegManager_Reset(t *testing.T) {
	lm, _ := newTestLegManager()

	// 填充一些状态
	lm.State.Netpos = 10
	lm.State.TradeCount = 50
	insertOrder(lm.Orders, 30001, types.Buy, 5819.0, 10, types.HitStandard)
	insertOrder(lm.Orders, 30002, types.Sell, 5820.0, 10, types.HitStandard)

	lm.Reset()

	if lm.State.Netpos != 0 {
		t.Errorf("Netpos = %d, want 0", lm.State.Netpos)
	}
	if lm.State.TradeCount != 0 {
		t.Errorf("TradeCount = %d, want 0", lm.State.TradeCount)
	}
	if len(lm.Orders.OrdMap) != 0 {
		t.Errorf("OrdMap should be empty, got %d", len(lm.Orders.OrdMap))
	}
	if len(lm.Orders.BidMap) != 0 {
		t.Errorf("BidMap should be empty, got %d", len(lm.Orders.BidMap))
	}
	if len(lm.Orders.AskMap) != 0 {
		t.Errorf("AskMap should be empty, got %d", len(lm.Orders.AskMap))
	}
}

// TestLegManager_SetExchangeCosts 设置手续费
func TestLegManager_SetExchangeCosts(t *testing.T) {
	lm, _ := newTestLegManager()

	lm.SetExchangeCosts(0.00001, 0.00002, 5.0, 10.0)

	if lm.State.BuyExchTx != 0.00001 {
		t.Errorf("BuyExchTx = %f, want 0.00001", lm.State.BuyExchTx)
	}
	if lm.State.SellExchTx != 0.00002 {
		t.Errorf("SellExchTx = %f, want 0.00002", lm.State.SellExchTx)
	}
	if lm.State.BuyExchContractTx != 5.0 {
		t.Errorf("BuyExchContractTx = %f, want 5.0", lm.State.BuyExchContractTx)
	}
	if lm.State.SellExchContractTx != 10.0 {
		t.Errorf("SellExchContractTx = %f, want 10.0", lm.State.SellExchContractTx)
	}
}
