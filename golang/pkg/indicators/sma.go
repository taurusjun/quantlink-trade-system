// Package indicators provides technical indicators for trading
package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// SMA implements the Simple Moving Average indicator
type SMA struct {
	*BaseIndicator
	period      int
	prices      []float64
	sum         float64
	initialized bool
}

// NewSMA creates a new SMA indicator
func NewSMA(period float64, maxHistory int) *SMA {
	p := int(period)
	return &SMA{
		BaseIndicator: &BaseIndicator{
			name:       "SMA",
			maxHistory: maxHistory,
		},
		period:      p,
		prices:      make([]float64, 0, p),
		sum:         0.0,
		initialized: false,
	}
}

// Update updates the SMA with new market data
func (sma *SMA) Update(md *mdpb.MarketDataUpdate) {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return
	}

	price := (md.BidPrice[0] + md.AskPrice[0]) / 2.0

	// Add new price
	sma.prices = append(sma.prices, price)
	sma.sum += price

	// Remove oldest price if we exceed the period
	if len(sma.prices) > sma.period {
		oldest := sma.prices[0]
		sma.prices = sma.prices[1:]
		sma.sum -= oldest
	}

	// Calculate average
	if len(sma.prices) == sma.period {
		sma.initialized = true
		avg := sma.sum / float64(sma.period)
		sma.AddValue(avg)
	}
}

// GetValue returns the current SMA value
func (sma *SMA) GetValue() float64 {
	if !sma.IsReady() {
		return 0.0
	}
	return sma.sum / float64(sma.period)
}

// IsReady returns true if the indicator has enough data
func (sma *SMA) IsReady() bool {
	return sma.initialized && len(sma.prices) == sma.period
}

// Reset resets the indicator state
func (sma *SMA) Reset() {
	sma.prices = make([]float64, 0, sma.period)
	sma.sum = 0.0
	sma.initialized = false
	sma.BaseIndicator.Reset()
}

// GetPeriod returns the period setting
func (sma *SMA) GetPeriod() int {
	return sma.period
}

// GetPrices returns the current price window (for testing)
func (sma *SMA) GetPrices() []float64 {
	result := make([]float64, len(sma.prices))
	copy(result, sma.prices)
	return result
}

// NewSMAFromConfig creates an SMA indicator from config
func NewSMAFromConfig(config map[string]interface{}) (Indicator, error) {
	period, ok := config["period"].(float64)
	if !ok {
		period = 20.0 // Default 20-period SMA
	}

	maxHistory, ok := config["max_history"].(float64)
	if !ok {
		maxHistory = 1000.0
	}

	return NewSMA(period, int(maxHistory)), nil
}
