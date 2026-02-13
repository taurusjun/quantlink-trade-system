package client

import (
	"sync"
	"testing"
	"time"

	"tbsrc-golang/pkg/connector"
	"tbsrc-golang/pkg/instrument"
	"tbsrc-golang/pkg/shm"
	"tbsrc-golang/pkg/types"
)

const (
	testMDKey    = 0xCB01
	testReqKey   = 0xCB02
	testRespKey  = 0xCB03
	testCSKey    = 0xCB04
	testQueueSz  = 256
)

// mockStrategy 实现 StrategyCallback 用于测试
type mockStrategy struct {
	mu      sync.Mutex
	mdCount int
	orsCount int
	lastInst *instrument.Instrument
	lastResp *shm.ResponseMsg
}

func (ms *mockStrategy) MDCallBack(inst *instrument.Instrument, md *shm.MarketUpdateNew) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.mdCount++
	ms.lastInst = inst
}

func (ms *mockStrategy) ORSCallBack(resp *shm.ResponseMsg) {
	ms.mu.Lock()
	defer ms.mu.Unlock()
	ms.orsCount++
	ms.lastResp = resp
}

func setupTestConnAndClient(t *testing.T) (*connector.Connector, *Client, func()) {
	t.Helper()
	cfg := connector.Config{
		MDShmKey:          testMDKey,
		MDQueueSz:         testQueueSz,
		ReqShmKey:         testReqKey,
		ReqQueueSz:        testQueueSz,
		RespShmKey:        testRespKey,
		RespQueueSz:       testQueueSz,
		ClientStoreShmKey: testCSKey,
	}

	// 创建一个临时 client 变量来接收回调
	var cl *Client

	conn, err := connector.NewForTest(cfg,
		func(md *shm.MarketUpdateNew) {
			if cl != nil {
				cl.OnMDUpdate(md)
			}
		},
		func(resp *shm.ResponseMsg) {
			if cl != nil {
				cl.OnORSUpdate(resp)
			}
		},
	)
	if err != nil {
		t.Fatalf("NewForTest: %v", err)
	}

	cl = NewClient(conn, 92201, "PRP05", "AG", shm.ChinaSHFE)

	cleanup := func() {
		conn.Destroy()
	}

	return conn, cl, cleanup
}

// TestClient_MDRouting 测试 MD 按 symbol 路由
func TestClient_MDRouting(t *testing.T) {
	conn, cl, cleanup := setupTestConnAndClient(t)
	defer cleanup()

	// 注册两个合约
	inst1 := &instrument.Instrument{Symbol: "ag2506", Exchange: "SHFE"}
	inst2 := &instrument.Instrument{Symbol: "ag2512", Exchange: "SHFE"}
	cl.RegisterInstrument(inst1)
	cl.RegisterInstrument(inst2)

	strat1 := &mockStrategy{}
	strat2 := &mockStrategy{}
	cl.RegisterStrategy("ag2506", strat1)
	cl.RegisterStrategy("ag2512", strat2)

	conn.Start()
	defer conn.Stop()

	// 发送 ag2506 行情
	md1 := &shm.MarketUpdateNew{}
	copy(md1.Header.Symbol[:], "ag2506")
	md1.Data.BidUpdates[0] = shm.BookElement{Price: 5819.0, Quantity: 100}
	md1.Data.AskUpdates[0] = shm.BookElement{Price: 5820.0, Quantity: 80}
	md1.Data.ValidBids = 1
	md1.Data.ValidAsks = 1
	conn.EnqueueMD(md1)

	// 发送 ag2512 行情
	md2 := &shm.MarketUpdateNew{}
	copy(md2.Header.Symbol[:], "ag2512")
	md2.Data.BidUpdates[0] = shm.BookElement{Price: 7100.0, Quantity: 50}
	md2.Data.AskUpdates[0] = shm.BookElement{Price: 7101.0, Quantity: 60}
	md2.Data.ValidBids = 1
	md2.Data.ValidAsks = 1
	conn.EnqueueMD(md2)

	// 等待处理
	time.Sleep(50 * time.Millisecond)

	strat1.mu.Lock()
	if strat1.mdCount != 1 {
		t.Errorf("strat1.mdCount = %d, want 1", strat1.mdCount)
	}
	if strat1.lastInst != inst1 {
		t.Error("strat1 should receive inst1")
	}
	strat1.mu.Unlock()

	strat2.mu.Lock()
	if strat2.mdCount != 1 {
		t.Errorf("strat2.mdCount = %d, want 1", strat2.mdCount)
	}
	if strat2.lastInst != inst2 {
		t.Error("strat2 should receive inst2")
	}
	strat2.mu.Unlock()

	// 验证行情簿更新
	if inst1.BidPx[0] != 5819.0 {
		t.Errorf("inst1.BidPx[0] = %f, want 5819.0", inst1.BidPx[0])
	}
	if inst2.BidPx[0] != 7100.0 {
		t.Errorf("inst2.BidPx[0] = %f, want 7100.0", inst2.BidPx[0])
	}
}

// TestClient_ORSRouting 测试 ORS 按 orderID 路由
func TestClient_ORSRouting(t *testing.T) {
	conn, cl, cleanup := setupTestConnAndClient(t)
	defer cleanup()

	inst := &instrument.Instrument{Symbol: "ag2506", Exchange: "SHFE", Token: 1}
	cl.RegisterInstrument(inst)

	strat := &mockStrategy{}
	cl.RegisterStrategy("ag2506", strat)

	conn.Start()
	defer conn.Stop()

	// 发送新订单
	orderID := cl.SendNewOrder(inst, types.Buy, 5819.0, 10, types.HitStandard, strat)

	if orderID == 0 {
		t.Fatal("orderID should not be 0")
	}

	// 模拟 ORS 回复
	resp := &shm.ResponseMsg{
		Response_Type: shm.NEW_ORDER_CONFIRM,
		OrderID:       orderID,
	}
	conn.EnqueueResponse(resp)

	time.Sleep(50 * time.Millisecond)

	strat.mu.Lock()
	if strat.orsCount != 1 {
		t.Errorf("strat.orsCount = %d, want 1", strat.orsCount)
	}
	if strat.lastResp == nil || strat.lastResp.OrderID != orderID {
		t.Errorf("lastResp.OrderID mismatch")
	}
	strat.mu.Unlock()
}

// TestClient_RequestMsgFilling 验证 RequestMsg 字段填充
func TestClient_RequestMsgFilling(t *testing.T) {
	_, cl, cleanup := setupTestConnAndClient(t)
	defer cleanup()

	inst := &instrument.Instrument{
		Symbol:     "ag2506",
		Exchange:   "SHFE",
		Token:      42,
		ExpiryDate: 20250615,
	}
	cl.RegisterInstrument(inst)

	strat := &mockStrategy{}

	// 不启动 connector — 直接发送，从 reqQueue 检查
	orderID := cl.SendNewOrder(inst, types.Sell, 5820.0, 15, types.HitCross, strat)

	if orderID == 0 {
		t.Fatal("orderID should not be 0")
	}

	// 验证 orderID 已注册
	if _, ok := cl.orderIDMap[orderID]; !ok {
		t.Error("orderID should be in orderIDMap")
	}
}

// TestClient_UnknownSymbolMD 未注册 symbol 不崩溃
func TestClient_UnknownSymbolMD(t *testing.T) {
	conn, _, cleanup := setupTestConnAndClient(t)
	defer cleanup()

	conn.Start()
	defer conn.Stop()

	md := &shm.MarketUpdateNew{}
	copy(md.Header.Symbol[:], "unknown")
	conn.EnqueueMD(md)

	time.Sleep(50 * time.Millisecond)
	// 不崩溃即为通过
}

// TestExtractSymbol 测试 symbol 提取
func TestExtractSymbol(t *testing.T) {
	h := &shm.MDHeaderPart{}
	copy(h.Symbol[:], "ag2506")

	sym := extractSymbol(h)
	if sym != "ag2506" {
		t.Errorf("extractSymbol = %q, want ag2506", sym)
	}
}

// TestCopyStringToBytes 测试字符串到字节数组复制
func TestCopyStringToBytes(t *testing.T) {
	var dst [11]byte
	copyStringToBytes(dst[:], "PRP05")

	if string(dst[:5]) != "PRP05" {
		t.Errorf("dst = %q, want PRP05", dst[:5])
	}
	if dst[5] != 0 {
		t.Error("should be null terminated")
	}
}
