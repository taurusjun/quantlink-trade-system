package indicators

import (
	"fmt"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// Momentum measures the rate of price change over a specified period
// Formula: Momentum = CurrentPrice - Price[N periods ago]
// Positive momentum indicates upward trend, negative indicates downward trend
type Momentum struct {
	*BaseIndicator
	period      int
	prices      []float64
	initialized bool
}

// NewMomentum creates a new Momentum indicator
func NewMomentum(period int, maxHistory int) *Momentum {
	if period <= 0 {
		period = 10
	}

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &Momentum{
		BaseIndicator: NewBaseIndicator("Momentum", maxHistory),
		period:        period,
		prices:        make([]float64, 0, period+1),
		initialized:   false,
	}
}

// NewMomentumFromConfig creates Momentum from configuration
func NewMomentumFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 10
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

	return NewMomentum(period, maxHistory), nil
}

// Update updates the Momentum with new market data
func (m *Momentum) Update(md *mdpb.MarketDataUpdate) {
	midPrice := GetMidPrice(md)
	if midPrice == 0 {
		return
	}

	// Add new price
	m.prices = append(m.prices, midPrice)

	// Keep period+1 prices (current + N historical)
	if len(m.prices) > m.period+1 {
		m.prices = m.prices[1:]
	}

	// Calculate momentum when we have enough data
	if len(m.prices) == m.period+1 {
		m.initialized = true
		momentum := m.calculateMomentum()
		m.AddValue(momentum)
	}
}

// calculateMomentum computes the momentum value
func (m *Momentum) calculateMomentum() float64 {
	// Momentum = Current Price - Price[period] ago
	currentPrice := m.prices[len(m.prices)-1]
	pastPrice := m.prices[0]
	return currentPrice - pastPrice
}

// GetValue returns the current Momentum value
func (m *Momentum) GetValue() float64 {
	if !m.IsReady() {
		return 0.0
	}
	return m.calculateMomentum()
}

// IsReady returns true if the indicator has enough data
func (m *Momentum) IsReady() bool {
	return m.initialized && len(m.prices) == m.period+1
}

// Reset resets the indicator state
func (m *Momentum) Reset() {
	m.BaseIndicator.Reset()
	m.prices = make([]float64, 0, m.period+1)
	m.initialized = false
}

// GetPeriod returns the period setting
func (m *Momentum) GetPeriod() int {
	return m.period
}

// GetPrices returns the current price window (for testing)
func (m *Momentum) GetPrices() []float64 {
	result := make([]float64, len(m.prices))
	copy(result, m.prices)
	return result
}
