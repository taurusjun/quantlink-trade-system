package stats

import (
	"math"
	"testing"
)

// 测试辅助函数：比较浮点数是否近似相等
func almostEqual(a, b, tolerance float64) bool {
	return math.Abs(a-b) < tolerance
}

func TestMean(t *testing.T) {
	tests := []struct {
		name     string
		data     []float64
		expected float64
	}{
		{
			name:     "Simple average",
			data:     []float64{1, 2, 3, 4, 5},
			expected: 3.0,
		},
		{
			name:     "Empty array",
			data:     []float64{},
			expected: 0.0,
		},
		{
			name:     "Single value",
			data:     []float64{5.5},
			expected: 5.5,
		},
		{
			name:     "Negative values",
			data:     []float64{-2, -4, -6},
			expected: -4.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Mean(tt.data)
			if !almostEqual(result, tt.expected, 1e-10) {
				t.Errorf("Mean() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestVariance(t *testing.T) {
	tests := []struct {
		name     string
		data     []float64
		expected float64
	}{
		{
			name:     "Simple variance",
			data:     []float64{2, 4, 4, 4, 5, 5, 7, 9},
			expected: 4.0,
		},
		{
			name:     "No variance",
			data:     []float64{5, 5, 5, 5},
			expected: 0.0,
		},
		{
			name:     "Empty array",
			data:     []float64{},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Variance(tt.data)
			if !almostEqual(result, tt.expected, 1e-10) {
				t.Errorf("Variance() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestStdDev(t *testing.T) {
	tests := []struct {
		name     string
		data     []float64
		expected float64
	}{
		{
			name:     "Simple std dev",
			data:     []float64{2, 4, 4, 4, 5, 5, 7, 9},
			expected: 2.0,
		},
		{
			name:     "No deviation",
			data:     []float64{5, 5, 5, 5},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := StdDev(tt.data)
			if !almostEqual(result, tt.expected, 1e-10) {
				t.Errorf("StdDev() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestZScore(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		mean     float64
		std      float64
		expected float64
	}{
		{
			name:     "Positive z-score",
			value:    15.0,
			mean:     10.0,
			std:      2.5,
			expected: 2.0,
		},
		{
			name:     "Negative z-score",
			value:    5.0,
			mean:     10.0,
			std:      2.5,
			expected: -2.0,
		},
		{
			name:     "Zero z-score",
			value:    10.0,
			mean:     10.0,
			std:      2.5,
			expected: 0.0,
		},
		{
			name:     "Zero std dev",
			value:    10.0,
			mean:     10.0,
			std:      0.0,
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ZScore(tt.value, tt.mean, tt.std)
			if !almostEqual(result, tt.expected, 1e-10) {
				t.Errorf("ZScore() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCalculateRollingStats(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	tests := []struct {
		name         string
		period       int
		expectedMean float64
		expectedStd  float64
	}{
		{
			name:         "Last 5 points",
			period:       5,
			expectedMean: 8.0,                     // (6+7+8+9+10)/5
			expectedStd:  math.Sqrt(2.0),          // std of [6,7,8,9,10]
		},
		{
			name:         "Last 3 points",
			period:       3,
			expectedMean: 9.0,                     // (8+9+10)/3
			expectedStd:  math.Sqrt(2.0 / 3.0),    // std of [8,9,10]
		},
		{
			name:         "All points",
			period:       10,
			expectedMean: 5.5,                     // (1+2+...+10)/10
			expectedStd:  math.Sqrt(8.25),         // std of [1..10]
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateRollingStats(data, tt.period)
			if !almostEqual(result.Mean, tt.expectedMean, 1e-10) {
				t.Errorf("Mean = %v, want %v", result.Mean, tt.expectedMean)
			}
			if !almostEqual(result.Std, tt.expectedStd, 1e-10) {
				t.Errorf("Std = %v, want %v", result.Std, tt.expectedStd)
			}
			if result.Count != tt.period {
				t.Errorf("Count = %v, want %v", result.Count, tt.period)
			}
		})
	}
}

func TestCorrelation(t *testing.T) {
	tests := []struct {
		name     string
		x        []float64
		y        []float64
		expected float64
	}{
		{
			name:     "Perfect positive correlation",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{2, 4, 6, 8, 10},
			expected: 1.0,
		},
		{
			name:     "Perfect negative correlation",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{10, 8, 6, 4, 2},
			expected: -1.0,
		},
		{
			name:     "No correlation",
			x:        []float64{1, 2, 3},
			y:        []float64{5, 5, 5},
			expected: 0.0,
		},
		{
			name:     "Empty arrays",
			x:        []float64{},
			y:        []float64{},
			expected: 0.0,
		},
		{
			name:     "Different lengths",
			x:        []float64{1, 2, 3},
			y:        []float64{1, 2},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Correlation(tt.x, tt.y)
			if !almostEqual(result, tt.expected, 1e-10) {
				t.Errorf("Correlation() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCovariance(t *testing.T) {
	tests := []struct {
		name     string
		x        []float64
		y        []float64
		expected float64
	}{
		{
			name:     "Positive covariance",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{2, 4, 6, 8, 10},
			expected: 4.0,
		},
		{
			name:     "Negative covariance",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{10, 8, 6, 4, 2},
			expected: -4.0,
		},
		{
			name:     "Zero covariance",
			x:        []float64{1, 2, 3},
			y:        []float64{5, 5, 5},
			expected: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Covariance(tt.x, tt.y)
			if !almostEqual(result, tt.expected, 1e-10) {
				t.Errorf("Covariance() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestBeta(t *testing.T) {
	tests := []struct {
		name     string
		x        []float64
		y        []float64
		expected float64
	}{
		{
			name:     "Beta = 2.0",
			x:        []float64{2, 4, 6, 8, 10},
			y:        []float64{1, 2, 3, 4, 5},
			expected: 2.0,
		},
		{
			name:     "Beta = 0.5",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{2, 4, 6, 8, 10},
			expected: 0.5,
		},
		{
			name:     "Beta clamped to 0.5 (too low)",
			x:        []float64{1, 2, 3, 4, 5},
			y:        []float64{10, 20, 30, 40, 50},
			expected: 0.5, // 实际计算 0.1，但会被限制到 0.5
		},
		{
			name:     "Empty arrays",
			x:        []float64{},
			y:        []float64{},
			expected: 1.0, // 默认值
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Beta(tt.x, tt.y)
			if !almostEqual(result, tt.expected, 1e-10) {
				t.Errorf("Beta() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLinearRegression(t *testing.T) {
	tests := []struct {
		name              string
		x                 []float64
		y                 []float64
		expectedSlope     float64
		expectedIntercept float64
	}{
		{
			name:              "Simple linear relationship y=2x+1",
			x:                 []float64{1, 2, 3, 4, 5},
			y:                 []float64{3, 5, 7, 9, 11},
			expectedSlope:     2.0,
			expectedIntercept: 1.0,
		},
		{
			name:              "Horizontal line y=5",
			x:                 []float64{1, 2, 3, 4, 5},
			y:                 []float64{5, 5, 5, 5, 5},
			expectedSlope:     0.0,
			expectedIntercept: 5.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slope, intercept := LinearRegression(tt.x, tt.y)
			if !almostEqual(slope, tt.expectedSlope, 1e-10) {
				t.Errorf("Slope = %v, want %v", slope, tt.expectedSlope)
			}
			if !almostEqual(intercept, tt.expectedIntercept, 1e-10) {
				t.Errorf("Intercept = %v, want %v", intercept, tt.expectedIntercept)
			}
		})
	}
}

func TestCalculateCorrelation(t *testing.T) {
	x := []float64{1, 2, 3, 4, 5}
	y := []float64{2, 4, 6, 8, 10}

	result := CalculateCorrelation(x, y)

	if !almostEqual(result.Correlation, 1.0, 1e-10) {
		t.Errorf("Correlation = %v, want 1.0", result.Correlation)
	}
	if !almostEqual(result.Covariance, 4.0, 1e-10) {
		t.Errorf("Covariance = %v, want 4.0", result.Covariance)
	}
	if !almostEqual(result.Beta, 0.5, 1e-10) {
		t.Errorf("Beta = %v, want 0.5", result.Beta)
	}
}

// Benchmark tests
func BenchmarkMean(b *testing.B) {
	data := make([]float64, 1000)
	for i := range data {
		data[i] = float64(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Mean(data)
	}
}

func BenchmarkCalculateRollingStats(b *testing.B) {
	data := make([]float64, 1000)
	for i := range data {
		data[i] = float64(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateRollingStats(data, 100)
	}
}

func BenchmarkCorrelation(b *testing.B) {
	x := make([]float64, 1000)
	y := make([]float64, 1000)
	for i := range x {
		x[i] = float64(i)
		y[i] = float64(i) * 2
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Correlation(x, y)
	}
}
