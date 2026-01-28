package indicators

import (
	"math"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// Return calculates various types of returns
// Supports: simple return, log return, cumulative return
type Return struct {
	*BaseIndicator
	returnType   string  // "simple", "log", or "cumulative"
	period       int     // lookback period for return calculation
	prices       []float64
	lastValue    float64
	cumulativeReturn float64
}

// NewReturn creates a new Return indicator
// returnType: "simple" (default), "log", or "cumulative"
// period: lookback period (1 for period-to-period return)
func NewReturn(returnType string, period int, maxHistory int) *Return {
	if returnType == "" {
		returnType = "simple"
	}
	if period <= 0 {
		period = 1
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &Return{
		BaseIndicator:    NewBaseIndicator("Return", maxHistory),
		returnType:       returnType,
		period:           period,
		prices:           make([]float64, 0, period+1),
		cumulativeReturn: 1.0, // Start at 1.0 for cumulative return
	}
}

// NewReturnFromConfig creates Return from configuration
func NewReturnFromConfig(config map[string]interface{}) (Indicator, error) {
	returnType := "simple"
	period := 1
	maxHistory := 1000

	if v, ok := config["return_type"]; ok {
		if t, ok := v.(string); ok {
			returnType = t
		}
	}

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

	return NewReturn(returnType, period, maxHistory), nil
}

// Update calculates the return
func (r *Return) Update(md *mdpb.MarketDataUpdate) {
	price := GetMidPrice(md)
	if price <= 0 {
		return
	}

	r.prices = append(r.prices, price)

	// Keep only the necessary price history
	if len(r.prices) > r.period+1 {
		r.prices = r.prices[1:]
	}

	// Need at least period+1 prices to calculate return
	if len(r.prices) < r.period+1 {
		return
	}

	currentPrice := r.prices[len(r.prices)-1]
	oldPrice := r.prices[len(r.prices)-1-r.period]

	var returnValue float64

	switch r.returnType {
	case "log":
		// Log return: ln(P_t / P_{t-n})
		if oldPrice > 0 {
			returnValue = math.Log(currentPrice / oldPrice)
		}

	case "cumulative":
		// Cumulative return: product of (1 + simple returns)
		if oldPrice > 0 {
			periodReturn := (currentPrice - oldPrice) / oldPrice
			r.cumulativeReturn *= (1.0 + periodReturn)
			returnValue = r.cumulativeReturn - 1.0 // Return cumulative gain
		}

	default: // "simple"
		// Simple return: (P_t - P_{t-n}) / P_{t-n}
		if oldPrice > 0 {
			returnValue = (currentPrice - oldPrice) / oldPrice
		}
	}

	r.lastValue = returnValue
	r.AddValue(returnValue)
}

// GetValue returns the current return value
func (r *Return) GetValue() float64 {
	return r.lastValue
}

// GetReturnType returns the return calculation type
func (r *Return) GetReturnType() string {
	return r.returnType
}

// GetPeriod returns the lookback period
func (r *Return) GetPeriod() int {
	return r.period
}

// GetCumulativeReturn returns the cumulative return (only meaningful for cumulative type)
func (r *Return) GetCumulativeReturn() float64 {
	return r.cumulativeReturn - 1.0
}

// Reset resets the indicator
func (r *Return) Reset() {
	r.BaseIndicator.Reset()
	r.prices = r.prices[:0]
	r.lastValue = 0
	r.cumulativeReturn = 1.0
}

// IsReady returns true if we have enough data
func (r *Return) IsReady() bool {
	return len(r.prices) > r.period
}
