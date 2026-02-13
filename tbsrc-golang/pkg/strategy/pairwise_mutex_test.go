package strategy

import (
	"sync"
	"testing"

	"tbsrc-golang/pkg/shm"
	"tbsrc-golang/pkg/types"
)

// TestConcurrentMDAndORS 验证 MDCallBack 和 ORSCallBack 并发调用不会 panic
// 模拟 connector 的 pollMD 和 pollORS 两个 goroutine
func TestConcurrentMDAndORS(t *testing.T) {
	pas := newTestPAS()
	pas.Leg1.ORSCallbackOverride = pas
	pas.Leg2.ORSCallbackOverride = pas
	pas.SetActive(true)

	// 添加一些订单供 ORS 回调处理
	for i := uint32(100); i < 110; i++ {
		price := 5810.0 - float64(i-100)
		pas.Leg1.Orders.OrdMap[i] = &types.OrderStats{
			OrderID: i,
			Side:    types.Buy,
			OrdType: types.HitStandard,
			OpenQty: 1,
			Qty:     1,
			Price:   price,
			Status:  types.StatusNewConfirm,
		}
		pas.Leg1.Orders.BidMap[price] = pas.Leg1.Orders.OrdMap[i]
		pas.Leg1.State.BuyOpenOrders++
		pas.Leg1.State.BuyOpenQty++
	}

	var wg sync.WaitGroup

	// Simulate MD updates from pollMD goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		md := &shm.MarketUpdateNew{}
		md.Data.LastTradedPrice = 5810
		for i := 0; i < 100; i++ {
			pas.MDCallBack(pas.Inst1, md)
			pas.MDCallBack(pas.Inst2, md)
		}
	}()

	// Simulate ORS responses from pollORS goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := uint32(100); i < 110; i++ {
			resp := &shm.ResponseMsg{}
			resp.OrderID = i
			resp.Response_Type = shm.NEW_ORDER_CONFIRM
			resp.Price = 5810.0 - float64(i-100)
			pas.ORSCallBack(resp)
		}
	}()

	// Simulate SetActive from main goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			pas.SetActive(true)
		}
	}()

	wg.Wait()
	// Test passes if no panic/deadlock occurred
}

// TestHandleSquareoff_NoDeadlock 验证 HandleSquareoff 不会造成死锁
// HandleSquareoff 可以从外部调用（有锁），也可以从 MDCallBack 内部调用（已持锁）
func TestHandleSquareoff_NoDeadlock(t *testing.T) {
	pas := newTestPAS()
	pas.SetActive(true)

	var wg sync.WaitGroup

	// 外部调用 HandleSquareoff
	wg.Add(1)
	go func() {
		defer wg.Done()
		pas.HandleSquareoff()
	}()

	// 并发调用 HandleSquareON
	wg.Add(1)
	go func() {
		defer wg.Done()
		pas.HandleSquareON()
	}()

	wg.Wait()
	// Test passes if no deadlock
}
