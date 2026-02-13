package strategy

import (
	"math"
	"testing"
)

func TestSetThresholds_FlatPosition(t *testing.T) {
	pas := newTestPAS()
	pas.Leg1.State.NetposPass = 0

	pas.setThresholds()

	state := pas.Leg1.State
	if state.TholdBidPlace != pas.Thold1.BeginPlace {
		t.Errorf("TholdBidPlace = %f, want %f", state.TholdBidPlace, pas.Thold1.BeginPlace)
	}
	if state.TholdAskPlace != pas.Thold1.BeginPlace {
		t.Errorf("TholdAskPlace = %f, want %f", state.TholdAskPlace, pas.Thold1.BeginPlace)
	}
	if state.TholdBidRemove != pas.Thold1.BeginRemove {
		t.Errorf("TholdBidRemove = %f, want %f", state.TholdBidRemove, pas.Thold1.BeginRemove)
	}
	if state.TholdAskRemove != pas.Thold1.BeginRemove {
		t.Errorf("TholdAskRemove = %f, want %f", state.TholdAskRemove, pas.Thold1.BeginRemove)
	}
}

func TestSetThresholds_LongPosition(t *testing.T) {
	pas := newTestPAS()
	// sendInLots=false → TholdMaxPos = MaxSize * LotSize = 5 * 15 = 75
	pas.Leg1.State.NetposPass = 30 // 30/75 = 0.4 fraction

	pas.setThresholds()

	state := pas.Leg1.State
	maxPos := float64(75)

	// C++: TholdBidPlace = BEGIN_PLACE + (LONG_PLACE - BEGIN_PLACE) * netposPass / maxPos
	expectedBidPlace := 2.0 + (3.0-2.0)*30.0/maxPos
	if math.Abs(state.TholdBidPlace-expectedBidPlace) > 0.0001 {
		t.Errorf("TholdBidPlace = %f, want %f", state.TholdBidPlace, expectedBidPlace)
	}

	// C++: TholdAskPlace = BEGIN_PLACE - (BEGIN_PLACE - SHORT_PLACE) * netposPass / maxPos
	expectedAskPlace := 2.0 - (2.0-1.5)*30.0/maxPos
	if math.Abs(state.TholdAskPlace-expectedAskPlace) > 0.0001 {
		t.Errorf("TholdAskPlace = %f, want %f", state.TholdAskPlace, expectedAskPlace)
	}
}

func TestSetThresholds_ShortPosition(t *testing.T) {
	pas := newTestPAS()
	pas.Leg1.State.NetposPass = -30

	pas.setThresholds()

	state := pas.Leg1.State
	maxPos := float64(75)
	netpos := float64(-30)

	// C++: short → TholdBidPlace = BEGIN_PLACE + short_diff * netposPass / maxPos
	shortPlaceDiff := pas.Thold1.BeginPlace - pas.Thold1.ShortPlace // 2.0 - 1.5 = 0.5
	expectedBidPlace := 2.0 + shortPlaceDiff*netpos/maxPos          // 2.0 + 0.5*(-30)/75 = 2.0 - 0.2 = 1.8
	if math.Abs(state.TholdBidPlace-expectedBidPlace) > 0.0001 {
		t.Errorf("TholdBidPlace = %f, want %f", state.TholdBidPlace, expectedBidPlace)
	}

	// C++: short → TholdAskPlace = BEGIN_PLACE - long_diff * netposPass / maxPos
	longPlaceDiff := pas.Thold1.LongPlace - pas.Thold1.BeginPlace // 3.0 - 2.0 = 1.0
	expectedAskPlace := 2.0 - longPlaceDiff*netpos/maxPos          // 2.0 - 1.0*(-30)/75 = 2.0 + 0.4 = 2.4
	if math.Abs(state.TholdAskPlace-expectedAskPlace) > 0.0001 {
		t.Errorf("TholdAskPlace = %f, want %f", state.TholdAskPlace, expectedAskPlace)
	}
}

func TestSetThresholds_SendInLots(t *testing.T) {
	pas := newTestPAS()
	pas.Inst1.SendInLots = true
	pas.Inst2.SendInLots = true
	pas.Thold1.BidMaxSize = 10
	pas.Thold1.AskMaxSize = 8
	pas.Thold1.BidSize = 2
	pas.Thold1.AskSize = 3

	pas.setThresholds()

	state := pas.Leg1.State
	// maxPos = max(BidMaxSize, AskMaxSize) = max(10, 8) = 10
	if state.TholdMaxPos != 10 {
		t.Errorf("TholdMaxPos = %d, want 10", state.TholdMaxPos)
	}
	if state.TholdBidSize != 2 {
		t.Errorf("TholdBidSize = %d, want 2", state.TholdBidSize)
	}
	if state.TholdAskSize != 3 {
		t.Errorf("TholdAskSize = %d, want 3", state.TholdAskSize)
	}
	if state.TholdBidMaxPos != 10 {
		t.Errorf("TholdBidMaxPos = %d, want 10", state.TholdBidMaxPos)
	}
	if state.TholdAskMaxPos != 8 {
		t.Errorf("TholdAskMaxPos = %d, want 8", state.TholdAskMaxPos)
	}
}

func TestSetThresholds_NonSendInLots_DefaultsBidAsk(t *testing.T) {
	pas := newTestPAS()
	// SendInLots=false (default)

	pas.setThresholds()

	state := pas.Leg1.State
	// TholdMaxPos = MaxSize * LotSize = 5 * 15 = 75
	if state.TholdMaxPos != 75 {
		t.Errorf("TholdMaxPos = %d, want 75", state.TholdMaxPos)
	}
	// BidSize/AskSize should default to TholdSize
	if state.TholdBidSize != state.TholdSize {
		t.Errorf("TholdBidSize = %d, want %d (TholdSize)", state.TholdBidSize, state.TholdSize)
	}
	if state.TholdAskSize != state.TholdSize {
		t.Errorf("TholdAskSize = %d, want %d (TholdSize)", state.TholdAskSize, state.TholdSize)
	}
	// BidMaxPos/AskMaxPos should default to TholdMaxPos
	if state.TholdBidMaxPos != state.TholdMaxPos {
		t.Errorf("TholdBidMaxPos = %d, want %d", state.TholdBidMaxPos, state.TholdMaxPos)
	}
	if state.TholdAskMaxPos != state.TholdMaxPos {
		t.Errorf("TholdAskMaxPos = %d, want %d", state.TholdAskMaxPos, state.TholdMaxPos)
	}
}

func TestSetThresholds_UsesNetposPass_NotNetpos(t *testing.T) {
	pas := newTestPAS()
	// Set Netpos (total) to different value than NetposPass
	pas.Leg1.State.Netpos = 50
	pas.Leg1.State.NetposPass = 0

	pas.setThresholds()

	// Should use NetposPass=0 → flat thresholds
	state := pas.Leg1.State
	if state.TholdBidPlace != pas.Thold1.BeginPlace {
		t.Errorf("TholdBidPlace = %f, want %f (flat, using NetposPass not Netpos)",
			state.TholdBidPlace, pas.Thold1.BeginPlace)
	}
}
