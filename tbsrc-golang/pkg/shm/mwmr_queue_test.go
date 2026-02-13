package shm

import (
	"sync"
	"testing"
)

// Test key range for SHM (avoid collisions)
const testMDQueueKey = 0xBEEF01
const testReqQueueKey = 0xBEEF02
const testRespQueueKey = 0xBEEF03

func TestMWMRQueueSingleWriterSingleReader(t *testing.T) {
	q, err := NewMWMRQueueCreate[RequestMsg](testReqQueueKey, 16)
	if err != nil {
		t.Fatalf("NewMWMRQueueCreate: %v", err)
	}
	defer q.Destroy()

	// Queue should start empty
	if !q.IsEmpty() {
		t.Fatal("expected empty queue")
	}

	// Enqueue 10 messages
	for i := 0; i < 10; i++ {
		var msg RequestMsg
		msg.OrderID = uint32(i + 100)
		msg.Quantity = int32(i * 10)
		msg.Price = float64(i) * 1.5
		q.Enqueue(&msg)
	}

	// Queue should not be empty
	if q.IsEmpty() {
		t.Fatal("expected non-empty queue")
	}

	// Dequeue and verify
	for i := 0; i < 10; i++ {
		var out RequestMsg
		ok := q.Dequeue(&out)
		if !ok {
			t.Fatalf("Dequeue(%d) returned false", i)
		}
		if out.OrderID != uint32(i+100) {
			t.Errorf("Dequeue(%d): OrderID = %d, want %d", i, out.OrderID, i+100)
		}
		if out.Quantity != int32(i*10) {
			t.Errorf("Dequeue(%d): Quantity = %d, want %d", i, out.Quantity, i*10)
		}
		if out.Price != float64(i)*1.5 {
			t.Errorf("Dequeue(%d): Price = %f, want %f", i, out.Price, float64(i)*1.5)
		}
	}

	// Queue should be empty again
	if !q.IsEmpty() {
		t.Fatal("expected empty queue after dequeue all")
	}
}

func TestMWMRQueueMultiWriterSingleReader(t *testing.T) {
	q, err := NewMWMRQueueCreate[RequestMsg](testReqQueueKey+0x10, 256)
	if err != nil {
		t.Fatalf("NewMWMRQueueCreate: %v", err)
	}
	defer q.Destroy()

	numWriters := 4
	msgsPerWriter := 50
	total := numWriters * msgsPerWriter

	var wg sync.WaitGroup
	wg.Add(numWriters)

	for w := 0; w < numWriters; w++ {
		go func(writerID int) {
			defer wg.Done()
			for i := 0; i < msgsPerWriter; i++ {
				var msg RequestMsg
				// Encode writerID and sequence in OrderID
				msg.OrderID = uint32(writerID*10000 + i)
				msg.Quantity = int32(writerID)
				q.Enqueue(&msg)
			}
		}(w)
	}

	wg.Wait()

	// Read all messages
	count := 0
	for count < total {
		var out RequestMsg
		if q.Dequeue(&out) {
			count++
		}
	}

	if count != total {
		t.Errorf("dequeued %d messages, expected %d", count, total)
	}
}

func TestMWMRQueueMarketData(t *testing.T) {
	q, err := NewMWMRQueueCreate[MarketUpdateNew](testMDQueueKey, 32)
	if err != nil {
		t.Fatalf("NewMWMRQueueCreate: %v", err)
	}
	defer q.Destroy()

	// Enqueue market data
	var md MarketUpdateNew
	md.Header.ExchTS = 1234567890
	md.Header.SymbolID = 42
	md.Header.ExchangeName = ChinaSHFE
	copy(md.Header.Symbol[:], "ag2506")
	md.Data.BidUpdates[0] = BookElement{Quantity: 100, OrderCount: 5, Price: 5678.0}
	md.Data.AskUpdates[0] = BookElement{Quantity: 200, OrderCount: 3, Price: 5680.0}
	md.Data.ValidBids = 1
	md.Data.ValidAsks = 1
	md.Data.LastTradedPrice = 5679.0

	q.Enqueue(&md)

	// Dequeue and verify
	var out MarketUpdateNew
	if !q.Dequeue(&out) {
		t.Fatal("Dequeue returned false")
	}

	if out.Header.ExchTS != 1234567890 {
		t.Errorf("ExchTS = %d, want 1234567890", out.Header.ExchTS)
	}
	if out.Header.SymbolID != 42 {
		t.Errorf("SymbolID = %d, want 42", out.Header.SymbolID)
	}
	if string(out.Header.Symbol[:6]) != "ag2506" {
		t.Errorf("Symbol = %q, want ag2506", string(out.Header.Symbol[:6]))
	}
	if out.Data.BidUpdates[0].Price != 5678.0 {
		t.Errorf("BidUpdates[0].Price = %f, want 5678.0", out.Data.BidUpdates[0].Price)
	}
	if out.Data.AskUpdates[0].Quantity != 200 {
		t.Errorf("AskUpdates[0].Quantity = %d, want 200", out.Data.AskUpdates[0].Quantity)
	}
}

func TestMWMRQueueResponseMsg(t *testing.T) {
	q, err := NewMWMRQueueCreate[ResponseMsg](testRespQueueKey, 16)
	if err != nil {
		t.Fatalf("NewMWMRQueueCreate: %v", err)
	}
	defer q.Destroy()

	var resp ResponseMsg
	resp.Response_Type = TRADE_CONFIRM
	resp.OrderID = 12345
	resp.Quantity = 10
	resp.Price = 5000.5
	resp.Side = SideBuy
	copy(resp.Symbol[:], "ag2506")
	resp.OpenClose = OC_OPEN
	resp.ExchangeID = TS_SHFE

	q.Enqueue(&resp)

	var out ResponseMsg
	if !q.Dequeue(&out) {
		t.Fatal("Dequeue returned false")
	}
	if out.Response_Type != TRADE_CONFIRM {
		t.Errorf("Response_Type = %d, want %d", out.Response_Type, TRADE_CONFIRM)
	}
	if out.OrderID != 12345 {
		t.Errorf("OrderID = %d, want 12345", out.OrderID)
	}
	if out.Price != 5000.5 {
		t.Errorf("Price = %f, want 5000.5", out.Price)
	}
	if out.OpenClose != OC_OPEN {
		t.Errorf("OpenClose = %d, want %d", out.OpenClose, OC_OPEN)
	}
}

func TestNextPowerOf2(t *testing.T) {
	tests := []struct {
		in   int64
		want int64
	}{
		{0, 1},
		{1, 1},
		{2, 2},
		{3, 4},
		{4, 4},
		{5, 8},
		{7, 8},
		{8, 8},
		{9, 16},
		{100, 128},
		{1024, 1024},
		{1025, 2048},
	}
	for _, tt := range tests {
		got := nextPowerOf2(tt.in)
		if got != tt.want {
			t.Errorf("nextPowerOf2(%d) = %d, want %d", tt.in, got, tt.want)
		}
	}
}
