// Package stats provides statistical functions and time series analysis tools
package stats

import (
	"math"
)

// RollingWindowStats 滚动窗口统计结果
type RollingWindowStats struct {
	Mean     float64
	Std      float64
	Variance float64
	Count    int
}

// CalculateRollingStats 计算滚动窗口统计（均值、方差、标准差）
// 一次遍历计算多个统计值，提高性能
func CalculateRollingStats(data []float64, period int) RollingWindowStats {
	if len(data) == 0 {
		return RollingWindowStats{}
	}

	n := len(data)
	if period <= 0 || period > n {
		period = n
	}

	// 使用最近 period 个数据点
	recent := data[n-period:]

	// 计算均值
	var sum float64
	for _, val := range recent {
		sum += val
	}
	mean := sum / float64(len(recent))

	// 计算方差
	var variance float64
	for _, val := range recent {
		diff := val - mean
		variance += diff * diff
	}
	variance /= float64(len(recent))

	return RollingWindowStats{
		Mean:     mean,
		Std:      math.Sqrt(variance),
		Variance: variance,
		Count:    len(recent),
	}
}

// Mean 计算均值
func Mean(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}

	var sum float64
	for _, val := range data {
		sum += val
	}
	return sum / float64(len(data))
}

// Variance 计算方差
func Variance(data []float64) float64 {
	if len(data) == 0 {
		return 0
	}

	mean := Mean(data)
	var variance float64
	for _, val := range data {
		diff := val - mean
		variance += diff * diff
	}
	return variance / float64(len(data))
}

// StdDev 计算标准差
func StdDev(data []float64) float64 {
	return math.Sqrt(Variance(data))
}

// ZScore 计算 Z-Score
// z = (x - μ) / σ
func ZScore(value, mean, std float64) float64 {
	if std < 1e-10 {
		return 0
	}
	return (value - mean) / std
}

// Correlation 计算 Pearson 相关系数
// r = Σ[(xi - x̄)(yi - ȳ)] / sqrt[Σ(xi - x̄)² * Σ(yi - ȳ)²]
func Correlation(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	// 计算均值
	meanX := Mean(x)
	meanY := Mean(y)

	// 计算相关系数组成部分
	var numerator, varX, varY float64
	for i := range x {
		diffX := x[i] - meanX
		diffY := y[i] - meanY
		numerator += diffX * diffY
		varX += diffX * diffX
		varY += diffY * diffY
	}

	denominator := math.Sqrt(varX * varY)
	if denominator < 1e-10 {
		return 0
	}

	return numerator / denominator
}

// Covariance 计算协方差
// cov(X,Y) = Σ[(xi - x̄)(yi - ȳ)] / n
func Covariance(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 0
	}

	meanX := Mean(x)
	meanY := Mean(y)

	var covariance float64
	for i := range x {
		diffX := x[i] - meanX
		diffY := y[i] - meanY
		covariance += diffX * diffY
	}

	return covariance / float64(len(x))
}

// Beta 计算 Beta 系数（用于对冲比率）
// β = Cov(X,Y) / Var(Y)
// 其中 X 是依赖变量，Y 是自变量
func Beta(x, y []float64) float64 {
	if len(x) != len(y) || len(x) == 0 {
		return 1.0 // 默认 1:1 对冲
	}

	cov := Covariance(x, y)
	variance := Variance(y)

	if variance < 1e-10 {
		return 1.0
	}

	beta := cov / variance

	// 限制在合理范围内
	if beta < 0.5 {
		return 0.5
	}
	if beta > 2.0 {
		return 2.0
	}

	return beta
}

// LinearRegression 计算线性回归 y = slope * x + intercept
// 返回斜率和截距
func LinearRegression(x, y []float64) (slope, intercept float64) {
	if len(x) != len(y) || len(x) == 0 {
		return 0, 0
	}

	meanX := Mean(x)
	meanY := Mean(y)

	var numerator, denominator float64
	for i := range x {
		diffX := x[i] - meanX
		numerator += diffX * (y[i] - meanY)
		denominator += diffX * diffX
	}

	if denominator < 1e-10 {
		return 0, meanY
	}

	slope = numerator / denominator
	intercept = meanY - slope*meanX

	return slope, intercept
}

// CorrelationStats 相关性分析结果
type CorrelationStats struct {
	Correlation float64 // Pearson 相关系数
	Covariance  float64 // 协方差
	Beta        float64 // Beta 系数（X 对 Y 的回归系数）
}

// CalculateCorrelation 一次计算所有相关性统计
func CalculateCorrelation(x, y []float64) CorrelationStats {
	return CorrelationStats{
		Correlation: Correlation(x, y),
		Covariance:  Covariance(x, y),
		Beta:        Beta(x, y),
	}
}
