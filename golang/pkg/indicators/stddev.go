package indicators

import (
	"fmt"
	"math"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// StdDev (Standard Deviation) measures price volatility
// Formula: StdDev = sqrt(Σ(Price[i] - Mean)² / N)
// Higher values indicate greater volatility
type StdDev struct {
	*BaseIndicator
	period      int
	prices      []float64
	sum         float64
	initialized bool
}

// NewStdDev creates a new StdDev indicator
func NewStdDev(period int, maxHistory int) *StdDev {
	if period <= 0 {
		period = 20
	}

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &StdDev{
		BaseIndicator: NewBaseIndicator("StdDev", maxHistory),
		period:        period,
		prices:        make([]float64, 0, period),
		sum:           0.0,
		initialized:   false,
	}
}

// NewStdDevFromConfig creates StdDev from configuration
func NewStdDevFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 20
	maxHistory := 1000

	if v, ok := config["period"]; ok {
		if p, ok := v.(float64); ok {
			period = int(p)
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	if period <= 0 {
		return nil, fmt.Errorf("%w: period must be positive", ErrInvalidParameter)
	}

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return NewStdDev(period, maxHistory), nil
}

// Update updates the StdDev with new market data
func (s *StdDev) Update(md *mdpb.MarketDataUpdate) {
	midPrice := GetMidPrice(md)
	if midPrice == 0 {
		return
	}

	// Add new price
	s.prices = append(s.prices, midPrice)
	s.sum += midPrice

	// Remove oldest price if we exceed the period
	if len(s.prices) > s.period {
		oldest := s.prices[0]
		s.prices = s.prices[1:]
		s.sum -= oldest
	}

	// Calculate standard deviation when we have enough data
	if len(s.prices) == s.period {
		s.initialized = true
		stddev := s.calculateStdDev()
		s.AddValue(stddev)
	}
}

// calculateStdDev computes the standard deviation
func (s *StdDev) calculateStdDev() float64 {
	n := float64(len(s.prices))
	if n == 0 {
		return 0.0
	}

	// Calculate mean
	mean := s.sum / n

	// Calculate variance: Σ(x - mean)² / N
	var variance float64
	for _, price := range s.prices {
		diff := price - mean
		variance += diff * diff
	}
	variance /= n

	// Standard deviation is the square root of variance
	return math.Sqrt(variance)
}

// GetValue returns the current StdDev value
func (s *StdDev) GetValue() float64 {
	if !s.IsReady() {
		return 0.0
	}
	return s.calculateStdDev()
}

// IsReady returns true if the indicator has enough data
func (s *StdDev) IsReady() bool {
	return s.initialized && len(s.prices) == s.period
}

// Reset resets the indicator state
func (s *StdDev) Reset() {
	s.BaseIndicator.Reset()
	s.prices = make([]float64, 0, s.period)
	s.sum = 0.0
	s.initialized = false
}

// GetPeriod returns the period setting
func (s *StdDev) GetPeriod() int {
	return s.period
}

// GetPrices returns the current price window (for testing)
func (s *StdDev) GetPrices() []float64 {
	result := make([]float64, len(s.prices))
	copy(result, s.prices)
	return result
}

// GetMean returns the current mean price
func (s *StdDev) GetMean() float64 {
	if !s.IsReady() {
		return 0.0
	}
	return s.sum / float64(len(s.prices))
}
