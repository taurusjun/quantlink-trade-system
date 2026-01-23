package stats

import (
	"testing"
	"time"
)

func TestTimeSeries_Append(t *testing.T) {
	ts := NewTimeSeries("test", 5)

	// 添加数据
	ts.Append(1.0, 100)
	ts.Append(2.0, 200)
	ts.Append(3.0, 300)

	if ts.Len() != 3 {
		t.Errorf("Len() = %v, want 3", ts.Len())
	}

	data := ts.GetAll()
	expected := []float64{1.0, 2.0, 3.0}
	for i, val := range expected {
		if !almostEqual(data[i], val, 1e-10) {
			t.Errorf("Data[%d] = %v, want %v", i, data[i], val)
		}
	}
}

func TestTimeSeries_MaxLength(t *testing.T) {
	ts := NewTimeSeries("test", 3)

	// 添加超过最大长度的数据
	ts.Append(1.0, 100)
	ts.Append(2.0, 200)
	ts.Append(3.0, 300)
	ts.Append(4.0, 400) // 超出最大长度
	ts.Append(5.0, 500) // 超出最大长度

	if ts.Len() != 3 {
		t.Errorf("Len() = %v, want 3 (max length)", ts.Len())
	}

	data := ts.GetAll()
	expected := []float64{3.0, 4.0, 5.0} // 最老的数据被移除
	for i, val := range expected {
		if !almostEqual(data[i], val, 1e-10) {
			t.Errorf("Data[%d] = %v, want %v", i, data[i], val)
		}
	}
}

func TestTimeSeries_AppendNow(t *testing.T) {
	ts := NewTimeSeries("test", 5)

	before := time.Now().UnixNano()
	ts.AppendNow(1.0)
	after := time.Now().UnixNano()

	if ts.Len() != 1 {
		t.Errorf("Len() = %v, want 1", ts.Len())
	}

	if len(ts.Timestamps) != 1 {
		t.Errorf("Timestamps length = %v, want 1", len(ts.Timestamps))
	}

	timestamp := ts.Timestamps[0]
	if timestamp < before || timestamp > after {
		t.Errorf("Timestamp %v not in range [%v, %v]", timestamp, before, after)
	}
}

func TestTimeSeries_GetLast(t *testing.T) {
	ts := NewTimeSeries("test", 10)

	for i := 1; i <= 5; i++ {
		ts.Append(float64(i), int64(i*100))
	}

	tests := []struct {
		name     string
		n        int
		expected []float64
	}{
		{
			name:     "Last 3",
			n:        3,
			expected: []float64{3.0, 4.0, 5.0},
		},
		{
			name:     "Last 5 (all)",
			n:        5,
			expected: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
		},
		{
			name:     "Last 10 (more than available)",
			n:        10,
			expected: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
		},
		{
			name:     "Last 0",
			n:        0,
			expected: []float64{1.0, 2.0, 3.0, 4.0, 5.0},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ts.GetLast(tt.n)
			if len(result) != len(tt.expected) {
				t.Errorf("GetLast(%d) length = %v, want %v", tt.n, len(result), len(tt.expected))
				return
			}
			for i, val := range tt.expected {
				if !almostEqual(result[i], val, 1e-10) {
					t.Errorf("GetLast(%d)[%d] = %v, want %v", tt.n, i, result[i], val)
				}
			}
		})
	}
}

func TestTimeSeries_Last(t *testing.T) {
	ts := NewTimeSeries("test", 5)

	// 空序列
	_, ok := ts.Last()
	if ok {
		t.Error("Last() on empty series should return false")
	}

	// 添加数据
	ts.Append(1.0, 100)
	ts.Append(2.0, 200)

	val, ok := ts.Last()
	if !ok {
		t.Error("Last() should return true")
	}
	if !almostEqual(val, 2.0, 1e-10) {
		t.Errorf("Last() = %v, want 2.0", val)
	}
}

func TestTimeSeries_GetRange(t *testing.T) {
	ts := NewTimeSeries("test", 10)

	ts.Append(1.0, 100)
	ts.Append(2.0, 200)
	ts.Append(3.0, 300)
	ts.Append(4.0, 400)
	ts.Append(5.0, 500)

	tests := []struct {
		name      string
		startTime int64
		endTime   int64
		expected  []float64
	}{
		{
			name:      "Middle range",
			startTime: 200,
			endTime:   400,
			expected:  []float64{2.0, 3.0, 4.0},
		},
		{
			name:      "Full range",
			startTime: 100,
			endTime:   500,
			expected:  []float64{1.0, 2.0, 3.0, 4.0, 5.0},
		},
		{
			name:      "No data in range",
			startTime: 600,
			endTime:   700,
			expected:  []float64{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ts.GetRange(tt.startTime, tt.endTime)
			if len(result) != len(tt.expected) {
				t.Errorf("GetRange() length = %v, want %v", len(result), len(tt.expected))
				return
			}
			for i, val := range tt.expected {
				if !almostEqual(result[i], val, 1e-10) {
					t.Errorf("GetRange()[%d] = %v, want %v", i, result[i], val)
				}
			}
		})
	}
}

func TestTimeSeries_Stats(t *testing.T) {
	ts := NewTimeSeries("test", 10)

	// 添加数据: [1, 2, 3, 4, 5]
	for i := 1; i <= 5; i++ {
		ts.Append(float64(i), int64(i*100))
	}

	stats := ts.Stats(5)

	expectedMean := 3.0
	if !almostEqual(stats.Mean, expectedMean, 1e-10) {
		t.Errorf("Stats().Mean = %v, want %v", stats.Mean, expectedMean)
	}

	if stats.Count != 5 {
		t.Errorf("Stats().Count = %v, want 5", stats.Count)
	}
}

func TestTimeSeries_Clear(t *testing.T) {
	ts := NewTimeSeries("test", 5)

	ts.Append(1.0, 100)
	ts.Append(2.0, 200)

	if ts.Len() != 2 {
		t.Errorf("Len() = %v, want 2", ts.Len())
	}

	ts.Clear()

	if ts.Len() != 0 {
		t.Errorf("Len() after Clear() = %v, want 0", ts.Len())
	}
}

func TestSeriesManager_AddAndGet(t *testing.T) {
	sm := NewSeriesManager()

	// 添加序列
	ts1 := sm.AddSeries("series1", 10)
	if ts1.Name != "series1" {
		t.Errorf("Series name = %v, want series1", ts1.Name)
	}

	// 获取序列
	ts2, ok := sm.Get("series1")
	if !ok {
		t.Error("Get() should return true for existing series")
	}
	if ts2 != ts1 {
		t.Error("Get() should return the same series instance")
	}

	// 获取不存在的序列
	_, ok = sm.Get("nonexistent")
	if ok {
		t.Error("Get() should return false for non-existent series")
	}
}

func TestSeriesManager_GetOrCreate(t *testing.T) {
	sm := NewSeriesManager()

	// 第一次调用：创建
	ts1 := sm.GetOrCreate("series1", 10)
	if ts1 == nil {
		t.Fatal("GetOrCreate() should not return nil")
	}

	// 第二次调用：获取现有的
	ts2 := sm.GetOrCreate("series1", 20) // maxLength 不同也应该返回同一个
	if ts2 != ts1 {
		t.Error("GetOrCreate() should return the same instance")
	}
}

func TestSeriesManager_Remove(t *testing.T) {
	sm := NewSeriesManager()

	sm.AddSeries("series1", 10)
	sm.AddSeries("series2", 10)

	if sm.Count() != 2 {
		t.Errorf("Count() = %v, want 2", sm.Count())
	}

	sm.Remove("series1")

	if sm.Count() != 1 {
		t.Errorf("Count() after Remove() = %v, want 1", sm.Count())
	}

	_, ok := sm.Get("series1")
	if ok {
		t.Error("Get() should return false after Remove()")
	}
}

func TestSeriesManager_List(t *testing.T) {
	sm := NewSeriesManager()

	sm.AddSeries("series1", 10)
	sm.AddSeries("series2", 10)
	sm.AddSeries("series3", 10)

	names := sm.List()
	if len(names) != 3 {
		t.Errorf("List() length = %v, want 3", len(names))
	}

	// 检查所有名称都在列表中
	nameMap := make(map[string]bool)
	for _, name := range names {
		nameMap[name] = true
	}

	expected := []string{"series1", "series2", "series3"}
	for _, name := range expected {
		if !nameMap[name] {
			t.Errorf("List() missing name: %v", name)
		}
	}
}

func TestSeriesManager_Clear(t *testing.T) {
	sm := NewSeriesManager()

	sm.AddSeries("series1", 10)
	sm.AddSeries("series2", 10)

	if sm.Count() != 2 {
		t.Errorf("Count() = %v, want 2", sm.Count())
	}

	sm.Clear()

	if sm.Count() != 0 {
		t.Errorf("Count() after Clear() = %v, want 0", sm.Count())
	}
}

// 测试线程安全性（基本检查）
func TestTimeSeries_Concurrent(t *testing.T) {
	ts := NewTimeSeries("test", 100)

	// 启动多个 goroutine 同时写入
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 10; j++ {
				ts.Append(float64(id*10+j), int64(id*10+j))
			}
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证数据没有丢失（应该有 100 个数据点）
	if ts.Len() != 100 {
		t.Errorf("Len() = %v, want 100", ts.Len())
	}
}

func TestSeriesManager_Concurrent(t *testing.T) {
	sm := NewSeriesManager()

	// 启动多个 goroutine 同时添加序列
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 5; j++ {
				name := string(rune('A' + id*5 + j))
				sm.AddSeries(name, 10)
			}
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证所有序列都被添加
	if sm.Count() != 50 {
		t.Errorf("Count() = %v, want 50", sm.Count())
	}
}

// Benchmark tests
func BenchmarkTimeSeries_Append(b *testing.B) {
	ts := NewTimeSeries("test", 1000)
	timestamp := time.Now().UnixNano()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts.Append(float64(i), timestamp)
	}
}

func BenchmarkTimeSeries_GetLast(b *testing.B) {
	ts := NewTimeSeries("test", 1000)
	for i := 0; i < 1000; i++ {
		ts.Append(float64(i), int64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts.GetLast(100)
	}
}

func BenchmarkTimeSeries_Stats(b *testing.B) {
	ts := NewTimeSeries("test", 1000)
	for i := 0; i < 1000; i++ {
		ts.Append(float64(i), int64(i))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ts.Stats(100)
	}
}
