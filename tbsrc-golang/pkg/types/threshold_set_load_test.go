package types

import (
	"testing"
)

func TestLoadFromMap_BasicFields(t *testing.T) {
	ts := NewThresholdSet()
	m := map[string]float64{
		"begin_place":  0.35,
		"begin_remove": 0.15,
		"long_place":   0.55,
		"long_remove":  0.30,
		"short_place":  0.20,
		"short_remove": 0.10,
	}
	ts.LoadFromMap(m)

	if ts.BeginPlace != 0.35 {
		t.Errorf("BeginPlace = %f, want 0.35", ts.BeginPlace)
	}
	if ts.BeginRemove != 0.15 {
		t.Errorf("BeginRemove = %f, want 0.15", ts.BeginRemove)
	}
	if ts.LongPlace != 0.55 {
		t.Errorf("LongPlace = %f, want 0.55", ts.LongPlace)
	}
	if ts.LongRemove != 0.30 {
		t.Errorf("LongRemove = %f, want 0.30", ts.LongRemove)
	}
	if ts.ShortPlace != 0.20 {
		t.Errorf("ShortPlace = %f, want 0.20", ts.ShortPlace)
	}
	if ts.ShortRemove != 0.10 {
		t.Errorf("ShortRemove = %f, want 0.10", ts.ShortRemove)
	}
}

func TestLoadFromMap_IntFields(t *testing.T) {
	ts := NewThresholdSet()
	m := map[string]float64{
		"size":         1,
		"max_size":     5,
		"begin_size":   1,
		"max_os_order": 3,
		"slop":         20,
	}
	ts.LoadFromMap(m)

	if ts.Size != 1 {
		t.Errorf("Size = %d, want 1", ts.Size)
	}
	if ts.MaxSize != 5 {
		t.Errorf("MaxSize = %d, want 5", ts.MaxSize)
	}
	if ts.BeginSize != 1 {
		t.Errorf("BeginSize = %d, want 1", ts.BeginSize)
	}
	if ts.MaxOSOrder != 3 {
		t.Errorf("MaxOSOrder = %d, want 3", ts.MaxOSOrder)
	}
	if ts.Slop != 20 {
		t.Errorf("Slop = %d, want 20", ts.Slop)
	}
}

func TestLoadFromMap_BoolFields(t *testing.T) {
	ts := NewThresholdSet()
	m := map[string]float64{
		"use_linear_thold": 1,
		"news_flat":        1,
	}
	ts.LoadFromMap(m)

	if !ts.UseLinearThold {
		t.Error("UseLinearThold should be true")
	}
	if !ts.NewsFlat {
		t.Error("NewsFlat should be true")
	}
}

func TestLoadFromMap_BoolFalse(t *testing.T) {
	ts := NewThresholdSet()
	// ClosePNL defaults to true in NewThresholdSet
	m := map[string]float64{
		"close_pnl": 0,
	}
	ts.LoadFromMap(m)

	if ts.ClosePNL {
		t.Error("ClosePNL should be false after setting to 0")
	}
}

func TestLoadFromMap_ArbitrageFields(t *testing.T) {
	ts := NewThresholdSet()
	m := map[string]float64{
		"alpha":           0.01,
		"spread_ewa":      0.6,
		"avg_spread_away": 30,
		"hedge_ratio":     1.0,
		"const":           0.5,
	}
	ts.LoadFromMap(m)

	if ts.Alpha != 0.01 {
		t.Errorf("Alpha = %f, want 0.01", ts.Alpha)
	}
	if ts.SpreadEWA != 0.6 {
		t.Errorf("SpreadEWA = %f, want 0.6", ts.SpreadEWA)
	}
	if ts.AvgSpreadAway != 30 {
		t.Errorf("AvgSpreadAway = %d, want 30", ts.AvgSpreadAway)
	}
	if ts.Const != 0.5 {
		t.Errorf("Const = %f, want 0.5", ts.Const)
	}
}

func TestLoadFromMap_PreservesDefaults(t *testing.T) {
	ts := NewThresholdSet()
	// Only set one field, defaults should remain
	m := map[string]float64{
		"begin_place": 0.35,
	}
	ts.LoadFromMap(m)

	// Default MaxOSOrder should remain 5
	if ts.MaxOSOrder != 5 {
		t.Errorf("MaxOSOrder = %d, want 5 (default)", ts.MaxOSOrder)
	}
	// Default Slop should remain 20
	if ts.Slop != 20 {
		t.Errorf("Slop = %d, want 20 (default)", ts.Slop)
	}
	// ClosePNL should remain true
	if !ts.ClosePNL {
		t.Error("ClosePNL should be true (default)")
	}
}

func TestLoadFromMap_SupportingOrders(t *testing.T) {
	ts := NewThresholdSet()
	m := map[string]float64{
		"supporting_orders": 2,
		"tailing_orders":    1,
	}
	ts.LoadFromMap(m)

	if ts.SupportingOrders != 2 {
		t.Errorf("SupportingOrders = %d, want 2", ts.SupportingOrders)
	}
	if ts.TailingOrders != 1 {
		t.Errorf("TailingOrders = %d, want 1", ts.TailingOrders)
	}
}

func TestLoadFromMap_TVarKey(t *testing.T) {
	ts := NewThresholdSet()
	m := map[string]float64{
		"tvar_key":   12345,
		"tcache_key": 67890,
	}
	ts.LoadFromMap(m)

	if ts.TVarKey != 12345 {
		t.Errorf("TVarKey = %d, want 12345", ts.TVarKey)
	}
	if ts.TCacheKey != 67890 {
		t.Errorf("TCacheKey = %d, want 67890", ts.TCacheKey)
	}
}

func TestLoadFromMap_EmptyMap(t *testing.T) {
	ts := NewThresholdSet()
	ts.LoadFromMap(map[string]float64{})

	// All defaults should remain
	if ts.MaxOSOrder != 5 {
		t.Errorf("MaxOSOrder = %d, want 5", ts.MaxOSOrder)
	}
}

func TestLoadFromMap_UnknownKeysIgnored(t *testing.T) {
	ts := NewThresholdSet()
	m := map[string]float64{
		"begin_place":     0.35,
		"unknown_field_1": 99,
		"unknown_field_2": 42,
	}
	// Should not panic
	ts.LoadFromMap(m)
	if ts.BeginPlace != 0.35 {
		t.Errorf("BeginPlace = %f, want 0.35", ts.BeginPlace)
	}
}
