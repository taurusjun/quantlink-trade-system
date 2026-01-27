package indicators

import (
	"fmt"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// WMA (Weighted Moving Average) gives more weight to recent prices
// Formula: WMA = Σ(Price[i] * Weight[i]) / Σ(Weight[i])
// where Weight[i] = (period - i), so most recent price has highest weight
type WMA struct {
	*BaseIndicator
	period      int
	prices      []float64
	weightSum   float64 // Sum of weights: period + (period-1) + ... + 1 = period*(period+1)/2
	initialized bool
}

// NewWMA creates a new WMA indicator
func NewWMA(period int, maxHistory int) *WMA {
	if period <= 0 {
		period = 20
	}

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	// Calculate sum of weights: 1+2+3+...+period = period*(period+1)/2
	weightSum := float64(period * (period + 1) / 2)

	return &WMA{
		BaseIndicator: NewBaseIndicator("WMA", maxHistory),
		period:        period,
		prices:        make([]float64, 0, period),
		weightSum:     weightSum,
		initialized:   false,
	}
}

// NewWMAFromConfig creates WMA from configuration
func NewWMAFromConfig(config map[string]interface{}) (Indicator, error) {
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

	return NewWMA(period, maxHistory), nil
}

// Update updates the WMA with new market data
func (w *WMA) Update(md *mdpb.MarketDataUpdate) {
	midPrice := GetMidPrice(md)
	if midPrice == 0 {
		return
	}

	// Add new price
	w.prices = append(w.prices, midPrice)

	// Remove oldest price if we exceed the period
	if len(w.prices) > w.period {
		w.prices = w.prices[1:]
	}

	// Calculate WMA when we have enough data
	if len(w.prices) == w.period {
		w.initialized = true
		wma := w.calculateWMA()
		w.AddValue(wma)
	}
}

// calculateWMA computes the weighted moving average
func (w *WMA) calculateWMA() float64 {
	var weightedSum float64

	// Apply weights: oldest price gets weight 1, newest gets weight n
	for i, price := range w.prices {
		weight := float64(i + 1) // Weight increases linearly
		weightedSum += price * weight
	}

	return weightedSum / w.weightSum
}

// GetValue returns the current WMA value
func (w *WMA) GetValue() float64 {
	if !w.IsReady() {
		return 0.0
	}
	return w.calculateWMA()
}

// IsReady returns true if the indicator has enough data
func (w *WMA) IsReady() bool {
	return w.initialized && len(w.prices) == w.period
}

// Reset resets the indicator state
func (w *WMA) Reset() {
	w.BaseIndicator.Reset()
	w.prices = make([]float64, 0, w.period)
	w.initialized = false
}

// GetPeriod returns the period setting
func (w *WMA) GetPeriod() int {
	return w.period
}

// GetPrices returns the current price window (for testing)
func (w *WMA) GetPrices() []float64 {
	result := make([]float64, len(w.prices))
	copy(result, w.prices)
	return result
}
