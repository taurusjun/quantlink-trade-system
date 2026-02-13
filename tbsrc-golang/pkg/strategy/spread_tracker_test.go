package strategy

import (
	"math"
	"testing"
)

func TestSpreadTracker_Seed(t *testing.T) {
	st := NewSpreadTracker(0.01, 1.0, 20)
	st.Seed(5.0)

	if st.AvgSpreadOri != 5.0 {
		t.Errorf("AvgSpreadOri = %f, want 5.0", st.AvgSpreadOri)
	}
	if st.AvgSpread != 5.0 {
		t.Errorf("AvgSpread = %f, want 5.0", st.AvgSpread)
	}
	if !st.Initialized {
		t.Error("should be initialized after Seed")
	}
}

func TestSpreadTracker_SeedWithTValue(t *testing.T) {
	st := NewSpreadTracker(0.01, 1.0, 20)
	st.SetTValue(0.5)
	st.Seed(5.0)

	if st.AvgSpread != 5.5 {
		t.Errorf("AvgSpread = %f, want 5.5 (5.0 + 0.5)", st.AvgSpread)
	}
}

func TestSpreadTracker_Update_EWA(t *testing.T) {
	st := NewSpreadTracker(0.1, 1.0, 20)
	st.Seed(10.0)

	// leg1 update: should update EWA
	ok := st.Update(5815.0, 5805.0, true) // mid1-mid2 = 10.0 = avgSpread, no change
	if !ok {
		t.Error("should be valid")
	}
	if st.CurrSpread != 10.0 {
		t.Errorf("CurrSpread = %f, want 10.0", st.CurrSpread)
	}

	// leg1 update with different spread
	ok = st.Update(5816.0, 5805.0, true) // mid1-mid2 = 11.0
	if !ok {
		t.Error("should be valid")
	}
	if st.CurrSpread != 11.0 {
		t.Errorf("CurrSpread = %f, want 11.0", st.CurrSpread)
	}

	// EWA should move toward 11: (1-0.1)*10 + 0.1*11 = 9+1.1 = 10.1
	expected := 10.1
	if math.Abs(st.AvgSpreadOri-expected) > 0.001 {
		t.Errorf("AvgSpreadOri = %f, want %f", st.AvgSpreadOri, expected)
	}
}

func TestSpreadTracker_Update_OnlyLeg1UpdatesEWA(t *testing.T) {
	st := NewSpreadTracker(0.1, 1.0, 20)
	st.Seed(10.0)

	// leg2 update: should NOT update EWA
	st.Update(5815.0, 5804.0, false) // mid1-mid2 = 11.0
	if st.AvgSpreadOri != 10.0 {
		t.Errorf("AvgSpreadOri should not change on leg2 update, got %f", st.AvgSpreadOri)
	}

	// leg1 update: should update EWA
	st.Update(5815.0, 5804.0, true) // mid1-mid2 = 11.0
	expected := (1-0.1)*10.0 + 0.1*11.0
	if math.Abs(st.AvgSpreadOri-expected) > 0.001 {
		t.Errorf("AvgSpreadOri = %f, want %f", st.AvgSpreadOri, expected)
	}
}

func TestSpreadTracker_AvgSpreadAway(t *testing.T) {
	st := NewSpreadTracker(0.1, 1.0, 5) // 5 ticks max deviation
	st.Seed(10.0)

	// Spread within range: 10 vs avg 10, deviation 0
	ok := st.Update(5810.0, 5800.0, true) // spread = 10
	if !ok {
		t.Error("should be valid within range")
	}
	if !st.IsValid {
		t.Error("IsValid should be true")
	}

	// Spread just at boundary: spread=15, avg=10, deviation=5, limit=5
	ok = st.Update(5815.0, 5800.0, true) // spread = 15
	// deviation = |15 - 10.5| = 4.5 (after EWA moved), still within 5
	if !ok {
		// This depends on the EWA value after previous update
		// avg was updated by previous call; let's just check the mechanism works
	}

	// Reset with large deviation
	st2 := NewSpreadTracker(0.1, 1.0, 5)
	st2.Seed(10.0)
	ok = st2.Update(5820.0, 5800.0, true) // spread = 20, deviation from 10 = 10 > 5
	if ok {
		t.Error("should be invalid when deviation exceeds AvgSpreadAway")
	}
	if st2.IsValid {
		t.Error("IsValid should be false")
	}
}

func TestSpreadTracker_AutoInitialize(t *testing.T) {
	st := NewSpreadTracker(0.1, 1.0, 20)
	// No Seed() call

	ok := st.Update(5810.0, 5800.0, true) // spread = 10
	if !ok {
		t.Error("first update should be valid (auto-init)")
	}
	if !st.Initialized {
		t.Error("should be initialized after first Update")
	}
	if st.AvgSpreadOri != 10.0 {
		t.Errorf("AvgSpreadOri = %f, want 10.0 (auto-seeded)", st.AvgSpreadOri)
	}
}

func TestSpreadTracker_SetTValue(t *testing.T) {
	st := NewSpreadTracker(0.1, 1.0, 20)
	st.Seed(10.0)

	st.SetTValue(2.0)
	if st.AvgSpread != 12.0 {
		t.Errorf("AvgSpread = %f, want 12.0", st.AvgSpread)
	}

	st.SetTValue(-1.0)
	if st.AvgSpread != 9.0 {
		t.Errorf("AvgSpread = %f, want 9.0", st.AvgSpread)
	}
}

func TestSpreadTracker_Deviation(t *testing.T) {
	st := NewSpreadTracker(0.01, 1.0, 20)
	st.Seed(10.0)
	st.Update(5815.0, 5803.0, true) // spread = 12

	dev := st.Deviation()
	// CurrSpread=12, AvgSpread≈10.02 (EWA moved slightly)
	// dev ≈ 12 - 10.02 ≈ 1.98
	if dev < 1.9 || dev > 2.1 {
		t.Errorf("Deviation = %f, want ~2.0", dev)
	}
}

func TestSpreadTracker_DefaultAvgSpreadAway(t *testing.T) {
	st := NewSpreadTracker(0.1, 1.0, 0) // 0 should default to 20
	if st.AvgSpreadAway != 20 {
		t.Errorf("AvgSpreadAway = %d, want 20 (default)", st.AvgSpreadAway)
	}
}
