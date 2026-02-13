package config

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDailyInit_FileNotExists(t *testing.T) {
	d, err := LoadDailyInit("/tmp/nonexistent_daily_init_test_file")
	if err != nil {
		t.Fatalf("should not error on missing file: %v", err)
	}
	if d.AvgSpreadOri != 0 || d.NetposYtd1 != 0 || d.Netpos2day1 != 0 || d.NetposAgg2 != 0 {
		t.Error("should return zero values for missing file")
	}
}

func TestDailyInit_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daily_init.92201")

	original := &DailyInit{
		AvgSpreadOri: 10.12345678,
		NetposYtd1:   5,
		Netpos2day1:  -2,
		NetposAgg2:   -3,
	}

	// Save
	err := SaveDailyInit(path, original)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// Load
	loaded, err := LoadDailyInit(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if math.Abs(loaded.AvgSpreadOri-original.AvgSpreadOri) > 0.00000001 {
		t.Errorf("AvgSpreadOri = %f, want %f", loaded.AvgSpreadOri, original.AvgSpreadOri)
	}
	if loaded.NetposYtd1 != original.NetposYtd1 {
		t.Errorf("NetposYtd1 = %d, want %d", loaded.NetposYtd1, original.NetposYtd1)
	}
	if loaded.Netpos2day1 != original.Netpos2day1 {
		t.Errorf("Netpos2day1 = %d, want %d", loaded.Netpos2day1, original.Netpos2day1)
	}
	if loaded.NetposAgg2 != original.NetposAgg2 {
		t.Errorf("NetposAgg2 = %d, want %d", loaded.NetposAgg2, original.NetposAgg2)
	}
}

func TestDailyInit_PartialFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daily_init.partial")

	// Write only 2 lines
	err := os.WriteFile(path, []byte("5.5\n2\n"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	d, err := LoadDailyInit(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if d.AvgSpreadOri != 5.5 {
		t.Errorf("AvgSpreadOri = %f, want 5.5", d.AvgSpreadOri)
	}
	if d.NetposYtd1 != 2 {
		t.Errorf("NetposYtd1 = %d, want 2", d.NetposYtd1)
	}
	if d.Netpos2day1 != 0 {
		t.Errorf("Netpos2day1 = %d, want 0 (missing)", d.Netpos2day1)
	}
}

func TestDailyInit_ZeroValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daily_init.zero")

	original := &DailyInit{}
	err := SaveDailyInit(path, original)
	if err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadDailyInit(path)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.AvgSpreadOri != 0 {
		t.Errorf("AvgSpreadOri = %f, want 0", loaded.AvgSpreadOri)
	}
}

func TestDailyInitPath(t *testing.T) {
	path := DailyInitPath("/data", 92201)
	if path != "/data/daily_init.92201" {
		t.Errorf("path = %s, want /data/daily_init.92201", path)
	}
}
