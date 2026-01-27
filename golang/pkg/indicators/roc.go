package indicators

import (
	"fmt"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// ROC (Rate of Change) measures the percentage change in price over a period
// Formula: ROC = ((CurrentPrice - Price[N periods ago]) / Price[N periods ago]) * 100
// Positive ROC indicates upward momentum, negative indicates downward momentum
type ROC struct {
	*BaseIndicator
	period      int
	prices      []float64
	initialized bool
}

// NewROC creates a new ROC indicator
func NewROC(period int, maxHistory int) *ROC {
	if period <= 0 {
		period = 10
	}

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &ROC{
		BaseIndicator: NewBaseIndicator("ROC", maxHistory),
		period:        period,
		prices:        make([]float64, 0, period+1),
		initialized:   false,
	}
}

// NewROCFromConfig creates ROC from configuration
func NewROCFromConfig(config map[string]interface{}) (Indicator, error) {
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

	return NewROC(period, maxHistory), nil
}

// Update updates the ROC with new market data
func (r *ROC) Update(md *mdpb.MarketDataUpdate) {
	midPrice := GetMidPrice(md)
	if midPrice == 0 {
		return
	}

	// Add new price
	r.prices = append(r.prices, midPrice)

	// Keep period+1 prices (current + N historical)
	if len(r.prices) > r.period+1 {
		r.prices = r.prices[1:]
	}

	// Calculate ROC when we have enough data
	if len(r.prices) == r.period+1 {
		r.initialized = true
		roc := r.calculateROC()
		r.AddValue(roc)
	}
}

// calculateROC computes the rate of change
func (r *ROC) calculateROC() float64 {
	currentPrice := r.prices[len(r.prices)-1]
	pastPrice := r.prices[0]

	// Avoid division by zero
	if pastPrice == 0 {
		return 0.0
	}

	// ROC = ((Current - Past) / Past) * 100
	return ((currentPrice - pastPrice) / pastPrice) * 100.0
}

// GetValue returns the current ROC value
func (r *ROC) GetValue() float64 {
	if !r.IsReady() {
		return 0.0
	}
	return r.calculateROC()
}

// IsReady returns true if the indicator has enough data
func (r *ROC) IsReady() bool {
	return r.initialized && len(r.prices) == r.period+1
}

// Reset resets the indicator state
func (r *ROC) Reset() {
	r.BaseIndicator.Reset()
	r.prices = make([]float64, 0, r.period+1)
	r.initialized = false
}

// GetPeriod returns the period setting
func (r *ROC) GetPeriod() int {
	return r.period
}

// GetPrices returns the current price window (for testing)
func (r *ROC) GetPrices() []float64 {
	result := make([]float64, len(r.prices))
	copy(result, r.prices)
	return result
}
