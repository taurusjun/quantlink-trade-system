package instrument

import (
	"testing"

	"tbsrc-golang/pkg/shm"
)

func TestUpdateFromMD(t *testing.T) {
	inst := &Instrument{
		Symbol:   "ag2506",
		Exchange: "SHFE",
		TickSize: 1.0,
		LotSize:  15,
	}

	// 构造 MarketUpdateNew
	md := &shm.MarketUpdateNew{}
	md.Data.ValidBids = 3
	md.Data.ValidAsks = 3
	md.Data.LastTradedPrice = 5820.0
	md.Data.LastTradedQuantity = 5

	// 设置 3 档 bid/ask
	md.Data.BidUpdates[0] = shm.BookElement{Quantity: 100, Price: 5819.0}
	md.Data.BidUpdates[1] = shm.BookElement{Quantity: 200, Price: 5818.0}
	md.Data.BidUpdates[2] = shm.BookElement{Quantity: 50, Price: 5817.0}
	md.Data.AskUpdates[0] = shm.BookElement{Quantity: 80, Price: 5820.0}
	md.Data.AskUpdates[1] = shm.BookElement{Quantity: 150, Price: 5821.0}
	md.Data.AskUpdates[2] = shm.BookElement{Quantity: 300, Price: 5822.0}

	inst.UpdateFromMD(md)

	// 验证行情簿
	if inst.ValidBids != 3 {
		t.Errorf("ValidBids = %d, want 3", inst.ValidBids)
	}
	if inst.ValidAsks != 3 {
		t.Errorf("ValidAsks = %d, want 3", inst.ValidAsks)
	}
	if inst.BidPx[0] != 5819.0 {
		t.Errorf("BidPx[0] = %f, want 5819.0", inst.BidPx[0])
	}
	if inst.BidQty[0] != 100 {
		t.Errorf("BidQty[0] = %f, want 100", inst.BidQty[0])
	}
	if inst.AskPx[0] != 5820.0 {
		t.Errorf("AskPx[0] = %f, want 5820.0", inst.AskPx[0])
	}
	if inst.AskQty[0] != 80 {
		t.Errorf("AskQty[0] = %f, want 80", inst.AskQty[0])
	}
	if inst.LastTradePx != 5820.0 {
		t.Errorf("LastTradePx = %f, want 5820.0", inst.LastTradePx)
	}
}

func TestMidPrice(t *testing.T) {
	inst := &Instrument{}
	inst.BidPx[0] = 5819.0
	inst.AskPx[0] = 5820.0

	mid := inst.MidPrice()
	if mid != 5819.5 {
		t.Errorf("MidPrice = %f, want 5819.5", mid)
	}
}

func TestMSWPrice(t *testing.T) {
	inst := &Instrument{}
	inst.BidPx[0] = 5819.0
	inst.BidQty[0] = 100
	inst.AskPx[0] = 5820.0
	inst.AskQty[0] = 80

	// MSW = (askQty*bidPx + bidQty*askPx) / (askQty+bidQty)
	// = (80*5819 + 100*5820) / (80+100) = (465520 + 582000) / 180 = 1047520/180 = 5819.555...
	msw := inst.MSWPrice()
	expected := (80.0*5819.0 + 100.0*5820.0) / 180.0
	if abs(msw-expected) > 0.001 {
		t.Errorf("MSWPrice = %f, want %f", msw, expected)
	}
}

func TestMSWPriceZeroQty(t *testing.T) {
	inst := &Instrument{}
	inst.BidPx[0] = 5819.0
	inst.BidQty[0] = 0
	inst.AskPx[0] = 5820.0
	inst.AskQty[0] = 0

	// 当数量为零，应返回 MidPrice
	msw := inst.MSWPrice()
	if msw != 5819.5 {
		t.Errorf("MSWPrice (zero qty) = %f, want 5819.5", msw)
	}
}

func TestHasValidBook(t *testing.T) {
	inst := &Instrument{}

	if inst.HasValidBook() {
		t.Error("empty book should not be valid")
	}

	inst.BidPx[0] = 5819.0
	inst.AskPx[0] = 5820.0
	if !inst.HasValidBook() {
		t.Error("valid book should return true")
	}
}

func TestSpread(t *testing.T) {
	inst := &Instrument{}
	inst.BidPx[0] = 5819.0
	inst.AskPx[0] = 5821.0

	if inst.Spread() != 2.0 {
		t.Errorf("Spread = %f, want 2.0", inst.Spread())
	}
}

func TestUpdateFromMD_20Levels(t *testing.T) {
	inst := &Instrument{}
	md := &shm.MarketUpdateNew{}
	md.Data.ValidBids = 20
	md.Data.ValidAsks = 20

	for i := range 20 {
		md.Data.BidUpdates[i] = shm.BookElement{Quantity: int32(100 + i), Price: 5819.0 - float64(i)}
		md.Data.AskUpdates[i] = shm.BookElement{Quantity: int32(80 + i), Price: 5820.0 + float64(i)}
	}

	inst.UpdateFromMD(md)

	// 验证所有 20 档
	for i := range 20 {
		expectedBidPx := 5819.0 - float64(i)
		expectedAskPx := 5820.0 + float64(i)
		if inst.BidPx[i] != expectedBidPx {
			t.Errorf("BidPx[%d] = %f, want %f", i, inst.BidPx[i], expectedBidPx)
		}
		if inst.AskPx[i] != expectedAskPx {
			t.Errorf("AskPx[%d] = %f, want %f", i, inst.AskPx[i], expectedAskPx)
		}
		expectedBidQty := float64(100 + i)
		expectedAskQty := float64(80 + i)
		if inst.BidQty[i] != expectedBidQty {
			t.Errorf("BidQty[%d] = %f, want %f", i, inst.BidQty[i], expectedBidQty)
		}
		if inst.AskQty[i] != expectedAskQty {
			t.Errorf("AskQty[%d] = %f, want %f", i, inst.AskQty[i], expectedAskQty)
		}
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
