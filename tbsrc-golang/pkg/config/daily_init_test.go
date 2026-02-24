package config

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMatrix2_FileNotExists(t *testing.T) {
	d, err := LoadMatrix2("/tmp/nonexistent_daily_init_test_file", 92201)
	if err != nil {
		t.Fatalf("should not error on missing file: %v", err)
	}
	if d.StrategyID != 0 || d.AvgSpreadOri != 0 || d.NetposYtd1 != 0 {
		t.Error("should return zero values for missing file")
	}
}

func TestLoadMatrix2_CppFormat(t *testing.T) {
	// 模拟 C++ SaveMatrix2 生成的文件
	dir := t.TempDir()
	path := filepath.Join(dir, "daily_init.92201")

	content := "StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2 \n" +
		"92201 0 96.671581 ag2603 ag2605 83 -83\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	d, err := LoadMatrix2(path, 92201)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if d.StrategyID != 92201 {
		t.Errorf("StrategyID = %d, want 92201", d.StrategyID)
	}
	if d.Netpos2day1 != 0 {
		t.Errorf("Netpos2day1 = %d, want 0", d.Netpos2day1)
	}
	if math.Abs(d.AvgSpreadOri-96.671581) > 0.000001 {
		t.Errorf("AvgSpreadOri = %f, want 96.671581", d.AvgSpreadOri)
	}
	if d.OrigBaseName1 != "ag2603" {
		t.Errorf("OrigBaseName1 = %q, want \"ag2603\"", d.OrigBaseName1)
	}
	if d.OrigBaseName2 != "ag2605" {
		t.Errorf("OrigBaseName2 = %q, want \"ag2605\"", d.OrigBaseName2)
	}
	if d.NetposYtd1 != 83 {
		t.Errorf("NetposYtd1 = %d, want 83", d.NetposYtd1)
	}
	if d.NetposAgg2 != -83 {
		t.Errorf("NetposAgg2 = %d, want -83", d.NetposAgg2)
	}
}

func TestLoadMatrix2_StrategyIDNotFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daily_init.92201")

	content := "StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2 \n" +
		"92201 0 96.671581 ag2603 ag2605 83 -83\n"
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := LoadMatrix2(path, 99999)
	if err == nil {
		t.Fatal("should error when strategyID not found")
	}
}

func TestSaveMatrix2_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daily_init.92201")

	original := &DailyInit{
		StrategyID:    92201,
		Netpos2day1:   0,
		AvgSpreadOri:  96.671581,
		OrigBaseName1: "ag2603",
		OrigBaseName2: "ag2605",
		NetposYtd1:    83,
		NetposAgg2:    -83,
	}

	if err := SaveMatrix2(path, original); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	// 验证文件内容格式
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	// Header 必须与 C++ 完全一致
	if !containsLine(content, "StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2 ") {
		t.Errorf("header 不匹配 C++ 格式，文件内容:\n%s", content)
	}

	// Round-trip: 用 LoadMatrix2 读回
	loaded, err := LoadMatrix2(path, 92201)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}

	if loaded.StrategyID != original.StrategyID {
		t.Errorf("StrategyID = %d, want %d", loaded.StrategyID, original.StrategyID)
	}
	if math.Abs(loaded.AvgSpreadOri-original.AvgSpreadOri) > 0.000001 {
		t.Errorf("AvgSpreadOri = %f, want %f", loaded.AvgSpreadOri, original.AvgSpreadOri)
	}
	if loaded.OrigBaseName1 != original.OrigBaseName1 {
		t.Errorf("OrigBaseName1 = %q, want %q", loaded.OrigBaseName1, original.OrigBaseName1)
	}
	if loaded.OrigBaseName2 != original.OrigBaseName2 {
		t.Errorf("OrigBaseName2 = %q, want %q", loaded.OrigBaseName2, original.OrigBaseName2)
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

func TestSaveMatrix2_ZeroValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "daily_init.92201")

	original := &DailyInit{
		StrategyID:    92201,
		OrigBaseName1: "ag2603",
		OrigBaseName2: "ag2605",
	}
	if err := SaveMatrix2(path, original); err != nil {
		t.Fatalf("save failed: %v", err)
	}

	loaded, err := LoadMatrix2(path, 92201)
	if err != nil {
		t.Fatalf("load failed: %v", err)
	}
	if loaded.AvgSpreadOri != 0 {
		t.Errorf("AvgSpreadOri = %f, want 0", loaded.AvgSpreadOri)
	}
	if loaded.NetposYtd1 != 0 {
		t.Errorf("NetposYtd1 = %d, want 0", loaded.NetposYtd1)
	}
}

func TestDailyInitPath(t *testing.T) {
	path := DailyInitPath("/data", 92201)
	if path != "/data/daily_init.92201" {
		t.Errorf("path = %s, want /data/daily_init.92201", path)
	}
}

func containsLine(content, line string) bool {
	for _, l := range splitLines(content) {
		if l == line {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}
