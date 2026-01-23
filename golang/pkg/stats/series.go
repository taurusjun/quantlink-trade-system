package stats

import (
	"sync"
	"time"
)

// TimeSeries 管理一个时间序列数据
type TimeSeries struct {
	Name       string
	Data       []float64
	Timestamps []int64 // Unix nano timestamps
	MaxLength  int
	mu         sync.RWMutex
}

// NewTimeSeries 创建新的时间序列
func NewTimeSeries(name string, maxLength int) *TimeSeries {
	return &TimeSeries{
		Name:       name,
		Data:       make([]float64, 0, maxLength),
		Timestamps: make([]int64, 0, maxLength),
		MaxLength:  maxLength,
	}
}

// Append 添加新数据点（线程安全）
func (ts *TimeSeries) Append(value float64, timestamp int64) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.Data = append(ts.Data, value)
	ts.Timestamps = append(ts.Timestamps, timestamp)

	// 限制长度
	if len(ts.Data) > ts.MaxLength {
		ts.Data = ts.Data[1:]
		ts.Timestamps = ts.Timestamps[1:]
	}
}

// AppendNow 添加数据点，使用当前时间戳
func (ts *TimeSeries) AppendNow(value float64) {
	ts.Append(value, time.Now().UnixNano())
}

// GetLast 获取最近 n 个数据点
func (ts *TimeSeries) GetLast(n int) []float64 {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if n <= 0 || n > len(ts.Data) {
		n = len(ts.Data)
	}

	if n == 0 {
		return []float64{}
	}

	// 返回副本，避免外部修改
	result := make([]float64, n)
	copy(result, ts.Data[len(ts.Data)-n:])
	return result
}

// GetAll 获取所有数据点
func (ts *TimeSeries) GetAll() []float64 {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	result := make([]float64, len(ts.Data))
	copy(result, ts.Data)
	return result
}

// GetRange 获取指定时间范围的数据
func (ts *TimeSeries) GetRange(startTime, endTime int64) []float64 {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	result := make([]float64, 0)
	for i, timestamp := range ts.Timestamps {
		if timestamp >= startTime && timestamp <= endTime {
			result = append(result, ts.Data[i])
		}
	}
	return result
}

// Len 返回当前数据点数量
func (ts *TimeSeries) Len() int {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	return len(ts.Data)
}

// Last 获取最新的数据点
func (ts *TimeSeries) Last() (float64, bool) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	if len(ts.Data) == 0 {
		return 0, false
	}
	return ts.Data[len(ts.Data)-1], true
}

// Stats 计算滚动窗口统计
func (ts *TimeSeries) Stats(period int) RollingWindowStats {
	ts.mu.RLock()
	defer ts.mu.RUnlock()

	return CalculateRollingStats(ts.Data, period)
}

// Mean 计算均值（使用指定周期）
func (ts *TimeSeries) Mean(period int) float64 {
	return ts.Stats(period).Mean
}

// StdDev 计算标准差（使用指定周期）
func (ts *TimeSeries) StdDev(period int) float64 {
	return ts.Stats(period).Std
}

// Clear 清空时间序列
func (ts *TimeSeries) Clear() {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.Data = make([]float64, 0, ts.MaxLength)
	ts.Timestamps = make([]int64, 0, ts.MaxLength)
}

// SeriesManager 管理多个时间序列
type SeriesManager struct {
	series map[string]*TimeSeries
	mu     sync.RWMutex
}

// NewSeriesManager 创建序列管理器
func NewSeriesManager() *SeriesManager {
	return &SeriesManager{
		series: make(map[string]*TimeSeries),
	}
}

// AddSeries 添加新序列
func (sm *SeriesManager) AddSeries(name string, maxLength int) *TimeSeries {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	ts := NewTimeSeries(name, maxLength)
	sm.series[name] = ts
	return ts
}

// Get 获取序列
func (sm *SeriesManager) Get(name string) (*TimeSeries, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	ts, ok := sm.series[name]
	return ts, ok
}

// GetOrCreate 获取或创建序列
func (sm *SeriesManager) GetOrCreate(name string, maxLength int) *TimeSeries {
	ts, ok := sm.Get(name)
	if ok {
		return ts
	}
	return sm.AddSeries(name, maxLength)
}

// Remove 移除序列
func (sm *SeriesManager) Remove(name string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	delete(sm.series, name)
}

// Clear 清空所有序列
func (sm *SeriesManager) Clear() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	sm.series = make(map[string]*TimeSeries)
}

// List 列出所有序列名称
func (sm *SeriesManager) List() []string {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	names := make([]string, 0, len(sm.series))
	for name := range sm.series {
		names = append(names, name)
	}
	return names
}

// Count 返回序列数量
func (sm *SeriesManager) Count() int {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return len(sm.series)
}
