package connector

import (
	"sync"
	"testing"
	"time"

	"tbsrc-golang/pkg/shm"
)

func testConfig() Config {
	return Config{
		MDShmKey:          0xDEAD01,
		MDQueueSz:         32,
		ReqShmKey:         0xDEAD02,
		ReqQueueSz:        32,
		RespShmKey:        0xDEAD03,
		RespQueueSz:       32,
		ClientStoreShmKey:  0xDEAD04,
	}
}

func TestConnectorMDCallback(t *testing.T) {
	var mu sync.Mutex
	var received []shm.MarketUpdateNew

	mdCb := func(md *shm.MarketUpdateNew) {
		mu.Lock()
		defer mu.Unlock()
		cp := *md
		received = append(received, cp)
	}
	orsCb := func(resp *shm.ResponseMsg) {}

	conn, err := NewForTest(testConfig(), mdCb, orsCb)
	if err != nil {
		t.Fatalf("NewForTest: %v", err)
	}
	defer conn.Destroy()

	conn.Start()

	// Enqueue market data
	var md shm.MarketUpdateNew
	md.Header.ExchTS = 9999
	copy(md.Header.Symbol[:], "ag2506")
	md.Data.LastTradedPrice = 5500.0
	md.Data.ValidBids = 1
	md.Data.BidUpdates[0] = shm.BookElement{Quantity: 10, Price: 5499.0}

	conn.EnqueueMD(&md)

	// Wait for callback
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(received)
		mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}

	conn.Stop()

	mu.Lock()
	defer mu.Unlock()
	if len(received) == 0 {
		t.Fatal("no MD callbacks received")
	}
	if received[0].Header.ExchTS != 9999 {
		t.Errorf("ExchTS = %d, want 9999", received[0].Header.ExchTS)
	}
	if received[0].Data.LastTradedPrice != 5500.0 {
		t.Errorf("LastTradedPrice = %f, want 5500.0", received[0].Data.LastTradedPrice)
	}
}

func TestConnectorORSCallback(t *testing.T) {
	var mu sync.Mutex
	var received []shm.ResponseMsg

	mdCb := func(md *shm.MarketUpdateNew) {}
	orsCb := func(resp *shm.ResponseMsg) {
		mu.Lock()
		defer mu.Unlock()
		cp := *resp
		received = append(received, cp)
	}

	conn, err := NewForTest(testConfig(), mdCb, orsCb)
	if err != nil {
		t.Fatalf("NewForTest: %v", err)
	}
	defer conn.Destroy()

	conn.Start()

	// Send a new order to get the OrderID
	var req shm.RequestMsg
	req.Quantity = 5
	req.Price = 5000.0
	copy(req.ContractDesc.Symbol[:], "ag2506")
	orderID := conn.SendNewOrder(&req)

	// Simulate ORS response for this order
	var resp shm.ResponseMsg
	resp.Response_Type = shm.TRADE_CONFIRM
	resp.OrderID = orderID
	resp.Quantity = 5
	resp.Price = 5000.0
	conn.EnqueueResponse(&resp)

	// Wait for callback
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(received)
		mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(time.Millisecond)
	}

	conn.Stop()

	mu.Lock()
	defer mu.Unlock()
	if len(received) == 0 {
		t.Fatal("no ORS callbacks received")
	}
	if received[0].Response_Type != shm.TRADE_CONFIRM {
		t.Errorf("Response_Type = %d, want %d", received[0].Response_Type, shm.TRADE_CONFIRM)
	}
	if received[0].OrderID != orderID {
		t.Errorf("OrderID = %d, want %d", received[0].OrderID, orderID)
	}
}

func TestConnectorORSFiltering(t *testing.T) {
	var mu sync.Mutex
	var received []shm.ResponseMsg

	mdCb := func(md *shm.MarketUpdateNew) {}
	orsCb := func(resp *shm.ResponseMsg) {
		mu.Lock()
		defer mu.Unlock()
		cp := *resp
		received = append(received, cp)
	}

	conn, err := NewForTest(testConfig(), mdCb, orsCb)
	if err != nil {
		t.Fatalf("NewForTest: %v", err)
	}
	defer conn.Destroy()

	conn.Start()

	// Enqueue response for a DIFFERENT client (clientID=999)
	var resp shm.ResponseMsg
	resp.Response_Type = shm.TRADE_CONFIRM
	resp.OrderID = 999*OrderIDRange + 1 // belongs to client 999
	conn.EnqueueResponse(&resp)

	// Give the poller time to process
	time.Sleep(100 * time.Millisecond)

	conn.Stop()
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	defer mu.Unlock()
	if len(received) != 0 {
		t.Errorf("should not have received callbacks for other clients, got %d", len(received))
	}
}

func TestConnectorOrderIDGeneration(t *testing.T) {
	conn, err := NewForTest(testConfig(), func(md *shm.MarketUpdateNew) {}, func(resp *shm.ResponseMsg) {})
	if err != nil {
		t.Fatalf("NewForTest: %v", err)
	}
	defer conn.Destroy()

	clientID := conn.ClientID()

	var req shm.RequestMsg
	id1 := conn.SendNewOrder(&req)
	id2 := conn.SendNewOrder(&req)
	id3 := conn.SendNewOrder(&req)

	// Verify IDs are sequential and belong to our client
	if id1/OrderIDRange != clientID {
		t.Errorf("id1 client = %d, want %d", id1/OrderIDRange, clientID)
	}
	if id2 != id1+1 {
		t.Errorf("id2 = %d, want %d", id2, id1+1)
	}
	if id3 != id2+1 {
		t.Errorf("id3 = %d, want %d", id3, id2+1)
	}
}
