package strategy

import (
	"os"
	"path/filepath"
	"testing"
)

// TestLoadMatrix2 测试加载 daily_init 文件
func TestLoadMatrix2(t *testing.T) {
	// 创建临时测试文件
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "daily_init.92201")

	// 写入测试数据（与 C++ 格式一致）
	content := `StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2
92201 0 -24.441424 ag_F_2_SFE ag_F_4_SFE -2 2
`
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// 测试加载
	mx, err := LoadMatrix2(testFile)
	if err != nil {
		t.Fatalf("LoadMatrix2 failed: %v", err)
	}

	// 验证数据
	row, exists := mx[92201]
	if !exists {
		t.Fatal("StrategyID 92201 not found in result")
	}

	if row.StrategyID != 92201 {
		t.Errorf("StrategyID: expected 92201, got %d", row.StrategyID)
	}
	if row.TwoDay != 0 {
		t.Errorf("TwoDay: expected 0, got %d", row.TwoDay)
	}
	if row.AvgPx != -24.441424 {
		t.Errorf("AvgPx: expected -24.441424, got %f", row.AvgPx)
	}
	if row.OrigBaseName1 != "ag_F_2_SFE" {
		t.Errorf("OrigBaseName1: expected ag_F_2_SFE, got %s", row.OrigBaseName1)
	}
	if row.OrigBaseName2 != "ag_F_4_SFE" {
		t.Errorf("OrigBaseName2: expected ag_F_4_SFE, got %s", row.OrigBaseName2)
	}
	if row.Ytd1 != -2 {
		t.Errorf("Ytd1: expected -2, got %d", row.Ytd1)
	}
	if row.Ytd2 != 2 {
		t.Errorf("Ytd2: expected 2, got %d", row.Ytd2)
	}

	t.Logf("LoadMatrix2 test passed: %+v", row)
}

// TestSaveMatrix2 测试保存 daily_init 文件
func TestSaveMatrix2(t *testing.T) {
	// 创建临时目录
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "daily_init.92201")

	// 保存数据
	err := SaveMatrix2(
		testFile,
		92201,
		-24.441424,
		"ag_F_2_SFE",
		"ag_F_4_SFE",
		-2,
		2,
	)
	if err != nil {
		t.Fatalf("SaveMatrix2 failed: %v", err)
	}

	// 重新加载验证
	mx, err := LoadMatrix2(testFile)
	if err != nil {
		t.Fatalf("LoadMatrix2 after save failed: %v", err)
	}

	row, exists := mx[92201]
	if !exists {
		t.Fatal("StrategyID 92201 not found after save")
	}

	if row.StrategyID != 92201 {
		t.Errorf("StrategyID: expected 92201, got %d", row.StrategyID)
	}
	if row.AvgPx != -24.441424 {
		t.Errorf("AvgPx: expected -24.441424, got %f", row.AvgPx)
	}
	if row.OrigBaseName1 != "ag_F_2_SFE" {
		t.Errorf("OrigBaseName1: expected ag_F_2_SFE, got %s", row.OrigBaseName1)
	}
	if row.OrigBaseName2 != "ag_F_4_SFE" {
		t.Errorf("OrigBaseName2: expected ag_F_4_SFE, got %s", row.OrigBaseName2)
	}
	if row.Ytd1 != -2 {
		t.Errorf("Ytd1: expected -2, got %d", row.Ytd1)
	}
	if row.Ytd2 != 2 {
		t.Errorf("Ytd2: expected 2, got %d", row.Ytd2)
	}

	t.Logf("SaveMatrix2 test passed: %+v", row)
}

// TestGetDailyInitPath 测试路径生成
func TestGetDailyInitPath(t *testing.T) {
	path := GetDailyInitPath(92201)
	expected := "data/daily_init.92201"
	if path != expected {
		t.Errorf("GetDailyInitPath: expected %s, got %s", expected, path)
	}
}

// TestLoadMatrix2_FileNotFound 测试文件不存在的情况
func TestLoadMatrix2_FileNotFound(t *testing.T) {
	_, err := LoadMatrix2("/nonexistent/path/daily_init.99999")
	if err == nil {
		t.Error("Expected error for nonexistent file, got nil")
	}
	t.Logf("Expected error received: %v", err)
}

// TestSaveLoadRoundTrip 测试保存和加载的完整流程
func TestSaveLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "daily_init.12345")

	// 测试数据
	testCases := []struct {
		strategyID    int32
		avgPx         float64
		origBaseName1 string
		origBaseName2 string
		ytd1          int32
		ytd2          int32
	}{
		{12345, 100.5, "cu2401", "cu2402", 10, -10},
		{12345, -50.25, "ag2501", "ag2502", -5, 5},
		{12345, 0.0, "au2401", "au2402", 0, 0},
	}

	for _, tc := range testCases {
		// 保存
		err := SaveMatrix2(testFile, tc.strategyID, tc.avgPx,
			tc.origBaseName1, tc.origBaseName2, tc.ytd1, tc.ytd2)
		if err != nil {
			t.Fatalf("SaveMatrix2 failed: %v", err)
		}

		// 加载
		mx, err := LoadMatrix2(testFile)
		if err != nil {
			t.Fatalf("LoadMatrix2 failed: %v", err)
		}

		row := mx[tc.strategyID]
		if row.AvgPx != tc.avgPx {
			t.Errorf("AvgPx mismatch: expected %f, got %f", tc.avgPx, row.AvgPx)
		}
		if row.OrigBaseName1 != tc.origBaseName1 {
			t.Errorf("OrigBaseName1 mismatch: expected %s, got %s", tc.origBaseName1, row.OrigBaseName1)
		}
		if row.OrigBaseName2 != tc.origBaseName2 {
			t.Errorf("OrigBaseName2 mismatch: expected %s, got %s", tc.origBaseName2, row.OrigBaseName2)
		}
		if row.Ytd1 != tc.ytd1 {
			t.Errorf("Ytd1 mismatch: expected %d, got %d", tc.ytd1, row.Ytd1)
		}
		if row.Ytd2 != tc.ytd2 {
			t.Errorf("Ytd2 mismatch: expected %d, got %d", tc.ytd2, row.Ytd2)
		}
	}

	t.Log("Round-trip test passed")
}
