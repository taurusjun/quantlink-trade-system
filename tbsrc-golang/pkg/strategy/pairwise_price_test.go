package strategy

import (
	"testing"

	"tbsrc-golang/pkg/types"
)

func TestGetBidPrice_NoInvisibleBook(t *testing.T) {
	pas := newTestPAS()
	pas.UseInvisibleBook = false

	price, ordType := pas.GetBidPrice(5810, types.HitStandard, 0)
	if price != 5810 {
		t.Errorf("price = %f, want 5810 (unchanged)", price)
	}
	if ordType != types.HitStandard {
		t.Errorf("ordType = %d, want HitStandard", ordType)
	}
}

func TestGetBidPrice_Level0_NoChange(t *testing.T) {
	pas := newTestPAS()
	pas.UseInvisibleBook = true

	// level=0 should always return unchanged
	price, _ := pas.GetBidPrice(5810, types.HitStandard, 0)
	if price != 5810 {
		t.Errorf("price = %f, want 5810 (level=0 no change)", price)
	}
}

func TestGetBidPrice_InvisibleBook_NoGap(t *testing.T) {
	pas := newTestPAS()
	pas.UseInvisibleBook = true

	// bidPx[1]=5809, bidPx[0]=5810. Gap = 5810-5809=1 = tickSize → no gap
	// price=5809, 5809 < 5810-1=5809? No → no improvement
	price, _ := pas.GetBidPrice(5809, types.HitStandard, 1)
	if price != 5809 {
		t.Errorf("price = %f, want 5809 (no gap)", price)
	}
}

func TestGetBidPrice_InvisibleBook_WithGap(t *testing.T) {
	pas := newTestPAS()
	pas.UseInvisibleBook = true

	// Create a gap: bidPx[1]=5807 (gap of 2 ticks from bidPx[0]=5810)
	pas.Inst1.BidPx[1] = 5807

	// bidInv = 5807 - 5800 + 1 = 8
	// avgSpread=10, BEGIN_PLACE=2 → 8 <= 10-2=8 → yes
	// But we also need an existing order at 5807 with quantAhead > lotSize
	pas.Leg1.Orders.BidMap[5807] = &types.OrderStats{
		OrderID:    999,
		Price:      5807,
		QuantAhead: 20, // > lotSize(15)
	}

	price, _ := pas.GetBidPrice(5807, types.HitStandard, 1)
	if price != 5808 { // 5807 + 1 tick
		t.Errorf("price = %f, want 5808 (improved by 1 tick)", price)
	}
}

func TestGetBidPrice_InvisibleBook_SpreadTooWide(t *testing.T) {
	pas := newTestPAS()
	pas.UseInvisibleBook = true

	// Create a gap
	pas.Inst1.BidPx[1] = 5807

	// bidInv = 5807 - 5800 + 1 = 8
	// Set avgSpread=8 → 8 <= 8-2=6? No → no improvement
	pas.Spread.Seed(8.0)

	price, _ := pas.GetBidPrice(5807, types.HitStandard, 1)
	if price != 5807 {
		t.Errorf("price = %f, want 5807 (spread check failed)", price)
	}
}

func TestGetAskPrice_NoInvisibleBook(t *testing.T) {
	pas := newTestPAS()
	pas.UseInvisibleBook = false

	price, ordType := pas.GetAskPrice(5811, types.HitStandard, 0)
	if price != 5811 {
		t.Errorf("price = %f, want 5811", price)
	}
	if ordType != types.HitStandard {
		t.Errorf("ordType = %d, want HitStandard", ordType)
	}
}

func TestGetAskPrice_Level0_NoChange(t *testing.T) {
	pas := newTestPAS()
	pas.UseInvisibleBook = true

	price, _ := pas.GetAskPrice(5811, types.HitStandard, 0)
	if price != 5811 {
		t.Errorf("price = %f, want 5811 (level=0)", price)
	}
}

func TestGetAskPrice_InvisibleBook_WithGap(t *testing.T) {
	pas := newTestPAS()
	pas.UseInvisibleBook = true

	// Create a gap: askPx[1]=5814 (gap of 2 ticks from askPx[0]=5811)
	pas.Inst1.AskPx[1] = 5814

	// askInv = 5814 - 5801 - 1 = 12
	// avgSpread=10, BEGIN_PLACE=2 → 12 >= 10+2=12 → yes
	pas.Leg1.Orders.AskMap[5814] = &types.OrderStats{
		OrderID:    998,
		Price:      5814,
		QuantAhead: 20, // > lotSize(15)
	}

	price, _ := pas.GetAskPrice(5814, types.HitStandard, 1)
	if price != 5813 { // 5814 - 1 tick
		t.Errorf("price = %f, want 5813 (improved by 1 tick)", price)
	}
}

func TestGetAskPrice_InvisibleBook_NoGap(t *testing.T) {
	pas := newTestPAS()
	pas.UseInvisibleBook = true

	// askPx[1]=5812, askPx[0]=5811. Gap = 5812-5811=1 = tickSize → no gap
	price, _ := pas.GetAskPrice(5812, types.HitStandard, 1)
	if price != 5812 {
		t.Errorf("price = %f, want 5812 (no gap)", price)
	}
}

func TestGetAskPrice_InvisibleBook_SmallQueueAhead(t *testing.T) {
	pas := newTestPAS()
	pas.UseInvisibleBook = true

	// Create gap
	pas.Inst1.AskPx[1] = 5814
	// Order exists but quantAhead too small
	pas.Leg1.Orders.AskMap[5814] = &types.OrderStats{
		OrderID:    997,
		Price:      5814,
		QuantAhead: 5, // < lotSize(15)
	}

	price, _ := pas.GetAskPrice(5814, types.HitStandard, 1)
	if price != 5814 {
		t.Errorf("price = %f, want 5814 (queue too small, no improvement)", price)
	}
}
