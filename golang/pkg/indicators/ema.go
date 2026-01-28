package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// EMA (Exponential Moving Average) calculates the exponential moving average
// EMA is more responsive to recent price changes than SMA
//
// Formula: EMA_t = α × Price_t + (1-α) × EMA_{t-1}
// where α = 2 / (period + 1) is the smoothing factor
//
// Properties:
// - More weight on recent prices
// - Less lag than SMA
// - Smooth curve
type EMA struct {
	*BaseIndicator
	period  int
	alpha   float64 // Smoothing factor: 2/(period+1)
	ema     float64
	isFirst bool
}

// NewEMA creates a new EMA indicator
func NewEMA(period int, maxHistory int) *EMA {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	alpha := 2.0 / float64(period+1)

	return &EMA{
		BaseIndicator: NewBaseIndicator("EMA", maxHistory),
		period:        period,
		alpha:         alpha,
		isFirst:       true,
	}
}

// NewEMAFromConfig creates EMA from configuration
func NewEMAFromConfig(config map[string]interface{}) (Indicator, error) {
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

	return NewEMA(period, maxHistory), nil
}

// Update updates the indicator with new market data
func (e *EMA) Update(md *mdpb.MarketDataUpdate) {
	price := GetMidPrice(md)
	if price <= 0 {
		return
	}

	if e.isFirst {
		// First value: use price as initial EMA
		e.ema = price
		e.isFirst = false
	} else {
		// EMA formula: EMA_t = α × Price_t + (1-α) × EMA_{t-1}
		e.ema = e.alpha*price + (1-e.alpha)*e.ema
	}

	e.AddValue(e.ema)
}

// GetValue returns the current EMA value
func (e *EMA) GetValue() float64 {
	return e.ema
}

// Reset resets the indicator
func (e *EMA) Reset() {
	e.BaseIndicator.Reset()
	e.ema = 0
	e.isFirst = true
}

// IsReady returns true if the indicator has at least one value
func (e *EMA) IsReady() bool {
	return !e.isFirst
}

// GetPeriod returns the period
func (e *EMA) GetPeriod() int {
	return e.period
}

// GetAlpha returns the smoothing factor
func (e *EMA) GetAlpha() float64 {
	return e.alpha
}
