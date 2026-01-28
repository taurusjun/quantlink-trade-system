package indicators

import (
	"math"

	"github.com/yourusername/quantlink-trade-system/pkg/stats"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// SharpeRatioIndicator calculates rolling Sharpe ratio
// Sharpe = (mean_return - risk_free_rate) / std_dev_return
type SharpeRatioIndicator struct {
	*BaseIndicator
	period       int
	returns      []float64
	prevPrice    float64
	riskFreeRate float64 // annualized risk-free rate
	lastValue    float64
	annualize    bool // whether to annualize the ratio
}

// NewSharpeRatioIndicator creates a new Sharpe Ratio indicator
// riskFreeRate: annualized risk-free rate (e.g., 0.03 for 3%)
// annualize: whether to annualize the Sharpe ratio
func NewSharpeRatioIndicator(period int, riskFreeRate float64, annualize bool, maxHistory int) *SharpeRatioIndicator {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &SharpeRatioIndicator{
		BaseIndicator: NewBaseIndicator("SharpeRatio", maxHistory),
		period:        period,
		returns:       make([]float64, 0, period),
		riskFreeRate:  riskFreeRate,
		annualize:     annualize,
	}
}

// NewSharpeRatioIndicatorFromConfig creates SharpeRatio from configuration
func NewSharpeRatioIndicatorFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 20
	riskFreeRate := 0.0
	annualize := true
	maxHistory := 1000

	if v, ok := config["period"]; ok {
		if p, ok := v.(float64); ok {
			period = int(p)
		}
	}

	if v, ok := config["risk_free_rate"]; ok {
		if r, ok := v.(float64); ok {
			riskFreeRate = r
		}
	}

	if v, ok := config["annualize"]; ok {
		if a, ok := v.(bool); ok {
			annualize = a
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewSharpeRatioIndicator(period, riskFreeRate, annualize, maxHistory), nil
}

// Update calculates the Sharpe ratio
func (s *SharpeRatioIndicator) Update(md *mdpb.MarketDataUpdate) {
	price := GetMidPrice(md)
	if price <= 0 {
		return
	}

	// Calculate return if we have previous price
	if s.prevPrice > 0 {
		ret := (price - s.prevPrice) / s.prevPrice
		s.returns = append(s.returns, ret)

		if len(s.returns) > s.period {
			s.returns = s.returns[1:]
		}
	}

	s.prevPrice = price

	// Need at least period returns
	if len(s.returns) < s.period {
		return
	}

	// Calculate mean and std dev of returns
	meanReturn := stats.Mean(s.returns)
	stdReturn := stats.StdDev(s.returns)

	if stdReturn == 0 {
		s.lastValue = 0
		s.AddValue(0)
		return
	}

	// Calculate Sharpe ratio
	// Daily risk-free rate (assuming 252 trading days)
	dailyRF := s.riskFreeRate / 252.0
	sharpe := (meanReturn - dailyRF) / stdReturn

	// Annualize if requested (multiply by sqrt(252))
	if s.annualize {
		sharpe *= math.Sqrt(252.0)
	}

	s.lastValue = sharpe
	s.AddValue(sharpe)
}

// GetValue returns the current Sharpe ratio
func (s *SharpeRatioIndicator) GetValue() float64 {
	return s.lastValue
}

// Reset resets the indicator
func (s *SharpeRatioIndicator) Reset() {
	s.BaseIndicator.Reset()
	s.returns = s.returns[:0]
	s.prevPrice = 0
	s.lastValue = 0
}

// IsReady returns true if we have enough data
func (s *SharpeRatioIndicator) IsReady() bool {
	return len(s.returns) >= s.period
}

// MaxDrawdownIndicator calculates rolling maximum drawdown
// MaxDD = max((peak - current) / peak) over the period
type MaxDrawdownIndicator struct {
	*BaseIndicator
	period      int
	prices      []float64
	lastValue   float64
	lastDD      float64
	lastDDDuration int
}

// NewMaxDrawdownIndicator creates a new MaxDrawdown indicator
func NewMaxDrawdownIndicator(period int, maxHistory int) *MaxDrawdownIndicator {
	if period <= 0 {
		period = 100 // Default longer period for drawdown
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &MaxDrawdownIndicator{
		BaseIndicator: NewBaseIndicator("MaxDrawdown", maxHistory),
		period:        period,
		prices:        make([]float64, 0, period),
	}
}

// NewMaxDrawdownIndicatorFromConfig creates MaxDrawdown from configuration
func NewMaxDrawdownIndicatorFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 100
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

	return NewMaxDrawdownIndicator(period, maxHistory), nil
}

// Update calculates the maximum drawdown
func (m *MaxDrawdownIndicator) Update(md *mdpb.MarketDataUpdate) {
	price := GetMidPrice(md)
	if price <= 0 {
		return
	}

	m.prices = append(m.prices, price)
	if len(m.prices) > m.period {
		m.prices = m.prices[1:]
	}

	if len(m.prices) < 2 {
		return
	}

	// Calculate max drawdown over the period
	var maxDD float64
	var maxDDDuration int
	peak := m.prices[0]
	peakIdx := 0

	for i := 1; i < len(m.prices); i++ {
		if m.prices[i] > peak {
			peak = m.prices[i]
			peakIdx = i
		}

		if peak > 0 {
			dd := (peak - m.prices[i]) / peak
			if dd > maxDD {
				maxDD = dd
				maxDDDuration = i - peakIdx
			}
		}
	}

	m.lastValue = maxDD
	m.lastDD = maxDD
	m.lastDDDuration = maxDDDuration
	m.AddValue(maxDD)
}

// GetValue returns the current max drawdown (as a positive value, e.g., 0.15 for 15% drawdown)
func (m *MaxDrawdownIndicator) GetValue() float64 {
	return m.lastValue
}

// GetDrawdownDuration returns the duration of the max drawdown in periods
func (m *MaxDrawdownIndicator) GetDrawdownDuration() int {
	return m.lastDDDuration
}

// Reset resets the indicator
func (m *MaxDrawdownIndicator) Reset() {
	m.BaseIndicator.Reset()
	m.prices = m.prices[:0]
	m.lastValue = 0
	m.lastDD = 0
	m.lastDDDuration = 0
}

// IsReady returns true if we have at least 2 prices
func (m *MaxDrawdownIndicator) IsReady() bool {
	return len(m.prices) >= 2
}
