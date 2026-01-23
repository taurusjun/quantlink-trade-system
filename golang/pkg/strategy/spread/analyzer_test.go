package spread

import (
	"math"
	"testing"
	"time"
)

func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

func TestNewSpreadAnalyzer(t *testing.T) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeDifference, 100)

	if sa == nil {
		t.Fatal("NewSpreadAnalyzer returned nil")
	}
	if sa.symbol1 != "AAPL" {
		t.Errorf("symbol1 = %s, want AAPL", sa.symbol1)
	}
	if sa.symbol2 != "MSFT" {
		t.Errorf("symbol2 = %s, want MSFT", sa.symbol2)
	}
	if sa.spreadType != SpreadTypeDifference {
		t.Errorf("spreadType = %s, want %s", sa.spreadType, SpreadTypeDifference)
	}
	if sa.hedgeRatio != 1.0 {
		t.Errorf("hedgeRatio = %f, want 1.0", sa.hedgeRatio)
	}
}

func TestSpreadAnalyzer_UpdatePrices(t *testing.T) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeDifference, 100)

	timestamp := time.Now().UnixNano()
	sa.UpdatePrices(100.0, 95.0, timestamp)

	if sa.price1 != 100.0 {
		t.Errorf("price1 = %f, want 100.0", sa.price1)
	}
	if sa.price2 != 95.0 {
		t.Errorf("price2 = %f, want 95.0", sa.price2)
	}
	if sa.price1Series.Len() != 1 {
		t.Errorf("price1Series.Len() = %d, want 1", sa.price1Series.Len())
	}
	if sa.price2Series.Len() != 1 {
		t.Errorf("price2Series.Len() = %d, want 1", sa.price2Series.Len())
	}
}

func TestSpreadAnalyzer_CalculateSpread_Difference(t *testing.T) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeDifference, 100)
	sa.hedgeRatio = 1.0

	sa.UpdatePricesNow(100.0, 95.0)

	// Spread = 100 - 1.0 * 95 = 5.0
	expected := 5.0
	if !almostEqual(sa.currentSpread, expected, 1e-10) {
		t.Errorf("currentSpread = %f, want %f", sa.currentSpread, expected)
	}

	// 修改对冲比率
	sa.SetHedgeRatio(1.05)
	sa.UpdatePricesNow(100.0, 95.0)

	// Spread = 100 - 1.05 * 95 = 0.25
	expected = 0.25
	if !almostEqual(sa.currentSpread, expected, 1e-10) {
		t.Errorf("currentSpread = %f, want %f", sa.currentSpread, expected)
	}
}

func TestSpreadAnalyzer_CalculateSpread_Ratio(t *testing.T) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeRatio, 100)

	sa.UpdatePricesNow(100.0, 95.0)

	// Spread = 100 / 95 = 1.0526...
	expected := 100.0 / 95.0
	if !almostEqual(sa.currentSpread, expected, 1e-10) {
		t.Errorf("currentSpread = %f, want %f", sa.currentSpread, expected)
	}
}

func TestSpreadAnalyzer_CalculateSpread_Log(t *testing.T) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeLog, 100)

	sa.UpdatePricesNow(100.0, 95.0)

	// Spread = log(100) - log(95)
	expected := math.Log(100.0) - math.Log(95.0)
	if !almostEqual(sa.currentSpread, expected, 1e-10) {
		t.Errorf("currentSpread = %f, want %f", sa.currentSpread, expected)
	}
}

func TestSpreadAnalyzer_UpdateStatistics(t *testing.T) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeDifference, 100)

	// 添加测试数据：spread = [5, 5, 5, 15, 15, 15, 5, 5, 5, 15]
	// mean = 9, std = 5
	testData := []struct {
		price1 float64
		price2 float64
	}{
		{105, 100}, // spread = 5
		{105, 100}, // spread = 5
		{105, 100}, // spread = 5
		{115, 100}, // spread = 15
		{115, 100}, // spread = 15
		{115, 100}, // spread = 15
		{105, 100}, // spread = 5
		{105, 100}, // spread = 5
		{105, 100}, // spread = 5
		{115, 100}, // spread = 15
	}

	for _, td := range testData {
		sa.UpdatePricesNow(td.price1, td.price2)
	}

	sa.UpdateStatistics(10)

	// 验证均值
	expectedMean := 9.0
	if !almostEqual(sa.spreadMean, expectedMean, 1e-10) {
		t.Errorf("spreadMean = %f, want %f", sa.spreadMean, expectedMean)
	}

	// 验证标准差（使用总体标准差公式）
	expectedStd := 4.898979 // sqrt(24) for population std
	if !almostEqual(sa.spreadStd, expectedStd, 0.01) {
		t.Errorf("spreadStd = %f, want %f", sa.spreadStd, expectedStd)
	}

	// 当前 spread = 15, z-score = (15 - 9) / 4.898979 ≈ 1.2247
	expectedZScore := 1.2247
	if !almostEqual(sa.currentZScore, expectedZScore, 0.01) {
		t.Errorf("currentZScore = %f, want %f", sa.currentZScore, expectedZScore)
	}
}

func TestSpreadAnalyzer_UpdateHedgeRatio(t *testing.T) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeDifference, 100)

	// 添加完美相关的数据：price1 = 2 * price2
	for i := 0; i < 30; i++ {
		price2 := 90.0 + float64(i)
		price1 := 2.0 * price2
		sa.UpdatePricesNow(price1, price2)
	}

	sa.UpdateHedgeRatio(20)

	// Beta 应该接近 2.0
	expectedRatio := 2.0
	if !almostEqual(sa.hedgeRatio, expectedRatio, 0.01) {
		t.Errorf("hedgeRatio = %f, want %f", sa.hedgeRatio, expectedRatio)
	}
}

func TestSpreadAnalyzer_UpdateCorrelation(t *testing.T) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeDifference, 100)

	// 添加完美正相关的数据
	for i := 0; i < 30; i++ {
		price := 100.0 + float64(i)
		sa.UpdatePricesNow(price, price*0.95)
	}

	correlation := sa.UpdateCorrelation(20)

	// 相关系数应该接近 1.0
	if correlation < 0.99 {
		t.Errorf("correlation = %f, want >= 0.99", correlation)
	}
}

func TestSpreadAnalyzer_UpdateAll(t *testing.T) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeDifference, 100)

	// 添加测试数据
	for i := 0; i < 30; i++ {
		price1 := 100.0 + float64(i)*0.5
		price2 := 95.0 + float64(i)*0.45
		sa.UpdatePricesNow(price1, price2)
	}

	sa.UpdateAll(20)

	// 验证所有统计都被更新
	if sa.spreadMean == 0 {
		t.Error("spreadMean should be updated")
	}
	if sa.spreadStd == 0 {
		t.Error("spreadStd should be updated")
	}
	if sa.hedgeRatio == 1.0 {
		t.Error("hedgeRatio should be updated from default 1.0")
	}
	if sa.correlation == 0 {
		t.Error("correlation should be updated")
	}
}

func TestSpreadAnalyzer_GetStats(t *testing.T) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeDifference, 100)

	// 添加数据并更新统计
	for i := 0; i < 20; i++ {
		sa.UpdatePricesNow(100.0+float64(i), 95.0+float64(i)*0.95)
	}
	sa.UpdateAll(20)

	stats := sa.GetStats()

	// 验证所有字段都有值
	if stats.CurrentSpread == 0 {
		t.Error("CurrentSpread should not be 0")
	}
	if stats.Mean == 0 {
		t.Error("Mean should not be 0")
	}
	if stats.Std == 0 {
		t.Error("Std should not be 0")
	}
	if stats.HedgeRatio == 0 {
		t.Error("HedgeRatio should not be 0")
	}
}

func TestSpreadAnalyzer_IsReady(t *testing.T) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeDifference, 100)

	// 初始状态不 ready
	if sa.IsReady(10) {
		t.Error("Should not be ready with no data")
	}

	// 添加 5 个数据点
	for i := 0; i < 5; i++ {
		sa.UpdatePricesNow(100.0, 95.0)
	}

	// 仍然不 ready (需要 10 个)
	if sa.IsReady(10) {
		t.Error("Should not be ready with only 5 data points")
	}

	// 添加更多数据
	for i := 0; i < 5; i++ {
		sa.UpdatePricesNow(100.0, 95.0)
	}

	// 现在应该 ready
	if !sa.IsReady(10) {
		t.Error("Should be ready with 10 data points")
	}
}

func TestSpreadAnalyzer_Reset(t *testing.T) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeDifference, 100)

	// 添加数据
	for i := 0; i < 20; i++ {
		sa.UpdatePricesNow(100.0+float64(i), 95.0+float64(i))
	}
	sa.UpdateAll(20)

	// 验证有数据
	if sa.price1Series.Len() == 0 {
		t.Fatal("Should have data before reset")
	}
	if sa.currentSpread == 0 {
		t.Fatal("currentSpread should not be 0 before reset")
	}

	// 重置
	sa.Reset()

	// 验证数据被清空
	if sa.price1Series.Len() != 0 {
		t.Error("price1Series should be empty after reset")
	}
	if sa.price2Series.Len() != 0 {
		t.Error("price2Series should be empty after reset")
	}
	if sa.spreadSeries.Len() != 0 {
		t.Error("spreadSeries should be empty after reset")
	}
	if sa.price1 != 0 {
		t.Error("price1 should be 0 after reset")
	}
	if sa.currentSpread != 0 {
		t.Error("currentSpread should be 0 after reset")
	}
}

func TestSpreadAnalyzer_GettersSetters(t *testing.T) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeDifference, 100)

	// 设置对冲比率
	sa.SetHedgeRatio(1.5)
	if sa.GetHedgeRatio() != 1.5 {
		t.Errorf("GetHedgeRatio() = %f, want 1.5", sa.GetHedgeRatio())
	}

	// 更新价格并获取
	sa.UpdatePricesNow(100.0, 95.0)

	spread := sa.GetCurrentSpread()
	// Spread = 100 - 1.5 * 95 = -42.5
	expected := -42.5
	if !almostEqual(spread, expected, 1e-10) {
		t.Errorf("GetCurrentSpread() = %f, want %f", spread, expected)
	}
}

func TestSpreadAnalyzer_Concurrent(t *testing.T) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeDifference, 1000)

	// 并发写入
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			for j := 0; j < 100; j++ {
				price := 100.0 + float64(id*100+j)*0.01
				sa.UpdatePricesNow(price, price*0.95)
			}
			done <- true
		}(i)
	}

	// 并发读取
	for i := 0; i < 5; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				_ = sa.GetStats()
				_ = sa.GetZScore()
				_ = sa.IsReady(10)
			}
			done <- true
		}()
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 15; i++ {
		<-done
	}

	// 验证没有数据丢失
	if sa.spreadSeries.Len() != 1000 {
		t.Errorf("spreadSeries.Len() = %d, want 1000", sa.spreadSeries.Len())
	}
}

// Benchmark tests
func BenchmarkSpreadAnalyzer_UpdatePrices(b *testing.B) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeDifference, 1000)
	timestamp := time.Now().UnixNano()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sa.UpdatePrices(100.0+float64(i)*0.01, 95.0+float64(i)*0.01, timestamp)
	}
}

func BenchmarkSpreadAnalyzer_UpdateAll(b *testing.B) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeDifference, 1000)

	// 预填充数据
	for i := 0; i < 1000; i++ {
		sa.UpdatePricesNow(100.0+float64(i)*0.01, 95.0+float64(i)*0.01)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		sa.UpdateAll(100)
	}
}

func BenchmarkSpreadAnalyzer_GetStats(b *testing.B) {
	sa := NewSpreadAnalyzer("AAPL", "MSFT", SpreadTypeDifference, 1000)

	// 预填充数据
	for i := 0; i < 1000; i++ {
		sa.UpdatePricesNow(100.0+float64(i)*0.01, 95.0+float64(i)*0.01)
	}
	sa.UpdateAll(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sa.GetStats()
	}
}
