package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// T3 (Tillson T3) is a smoothed moving average developed by Tim Tillson.
// It applies multiple layers of exponential smoothing with a volume factor
// to create a very smooth, low-lag moving average.
//
// The T3 uses 6 EMAs cascaded together with a volume factor (vFactor)
// that controls the smoothness vs. responsiveness trade-off.
//
// Formula:
// 1. Calculate 6 cascaded EMAs: e1 = EMA(price), e2 = EMA(e1), ..., e6 = EMA(e5)
// 2. Apply Tillson's formula with volume factor:
//    T3 = c1×e6 + c2×e5 + c3×e4 + c4×e3 + c5×e2 + c6×e1
//    where coefficients c1-c6 are calculated from vFactor
//
// Volume Factor (vFactor):
// - Range: 0 to 1
// - vFactor = 0: More responsive, less smooth
// - vFactor = 0.7: Balanced (default)
// - vFactor = 1: Most smooth, more lag
//
// Properties:
// - Very smooth with minimal lag
// - Better than TEMA for trend following
// - Less whipsaws than EMA
// - Good for filtering noise
type T3 struct {
	*BaseIndicator
	period  int
	vFactor float64
	e1      *EMA
	e2      *EMA
	e3      *EMA
	e4      *EMA
	e5      *EMA
	e6      *EMA
	c1      float64 // Coefficient 1
	c2      float64 // Coefficient 2
	c3      float64 // Coefficient 3
	c4      float64 // Coefficient 4
	t3      float64
}

// NewT3 creates a new T3 indicator
func NewT3(period int, vFactor float64, maxHistory int) *T3 {
	if period <= 0 {
		period = 5
	}
	if vFactor < 0 {
		vFactor = 0
	} else if vFactor > 1 {
		vFactor = 1
	}
	// Default vFactor
	if vFactor == 0 {
		vFactor = 0.7
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	// Calculate coefficients based on vFactor
	b := vFactor
	b2 := b * b
	b3 := b2 * b

	c1 := -b3
	c2 := 3*b2 + 3*b3
	c3 := -6*b2 - 3*b - 3*b3
	c4 := 1 + 3*b + b3 + 3*b2

	return &T3{
		BaseIndicator: NewBaseIndicator("T3", maxHistory),
		period:        period,
		vFactor:       vFactor,
		e1:            NewEMA(period, maxHistory),
		e2:            NewEMA(period, maxHistory),
		e3:            NewEMA(period, maxHistory),
		e4:            NewEMA(period, maxHistory),
		e5:            NewEMA(period, maxHistory),
		e6:            NewEMA(period, maxHistory),
		c1:            c1,
		c2:            c2,
		c3:            c3,
		c4:            c4,
	}
}

// NewT3FromConfig creates T3 from configuration
func NewT3FromConfig(config map[string]interface{}) (Indicator, error) {
	period := 5
	vFactor := 0.7
	maxHistory := 1000

	if v, ok := config["period"]; ok {
		if p, ok := v.(float64); ok {
			period = int(p)
		}
	}

	if v, ok := config["v_factor"]; ok {
		if f, ok := v.(float64); ok {
			vFactor = f
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewT3(period, vFactor, maxHistory), nil
}

// Update updates the indicator with new market data
func (t *T3) Update(md *mdpb.MarketDataUpdate) {
	// Update cascaded EMAs
	t.e1.Update(md)

	if t.e1.IsReady() {
		// Create synthetic market data for e2 from e1 value
		e1Val := t.e1.GetValue()
		mdE1 := &mdpb.MarketDataUpdate{
			BidPrice: []float64{e1Val},
			AskPrice: []float64{e1Val},
		}
		t.e2.Update(mdE1)
	}

	if t.e2.IsReady() {
		e2Val := t.e2.GetValue()
		mdE2 := &mdpb.MarketDataUpdate{
			BidPrice: []float64{e2Val},
			AskPrice: []float64{e2Val},
		}
		t.e3.Update(mdE2)
	}

	if t.e3.IsReady() {
		e3Val := t.e3.GetValue()
		mdE3 := &mdpb.MarketDataUpdate{
			BidPrice: []float64{e3Val},
			AskPrice: []float64{e3Val},
		}
		t.e4.Update(mdE3)
	}

	if t.e4.IsReady() {
		e4Val := t.e4.GetValue()
		mdE4 := &mdpb.MarketDataUpdate{
			BidPrice: []float64{e4Val},
			AskPrice: []float64{e4Val},
		}
		t.e5.Update(mdE4)
	}

	if t.e5.IsReady() {
		e5Val := t.e5.GetValue()
		mdE5 := &mdpb.MarketDataUpdate{
			BidPrice: []float64{e5Val},
			AskPrice: []float64{e5Val},
		}
		t.e6.Update(mdE5)
	}

	// Calculate T3 when all EMAs are ready
	if t.e6.IsReady() {
		e1Val := t.e1.GetValue()
		e2Val := t.e2.GetValue()
		e3Val := t.e3.GetValue()
		e4Val := t.e4.GetValue()
		e5Val := t.e5.GetValue()
		e6Val := t.e6.GetValue()

		// T3 = c1×e6 + c2×e5 + c3×e4 + c4×e3 + c5×e2 + c6×e1
		// Note: c5 = -c3, c6 = -c2 (for symmetry)
		t.t3 = t.c1*e6Val + t.c2*e5Val + t.c3*e4Val + t.c4*e3Val - t.c3*e2Val - t.c2*e1Val

		t.AddValue(t.t3)
	}
}

// GetValue returns the current T3 value
func (t *T3) GetValue() float64 {
	return t.t3
}

// Reset resets the indicator
func (t *T3) Reset() {
	t.BaseIndicator.Reset()
	t.e1.Reset()
	t.e2.Reset()
	t.e3.Reset()
	t.e4.Reset()
	t.e5.Reset()
	t.e6.Reset()
	t.t3 = 0
}

// IsReady returns true if the indicator has enough data
func (t *T3) IsReady() bool {
	return t.e6.IsReady()
}

// GetPeriod returns the period
func (t *T3) GetPeriod() int {
	return t.period
}

// GetVFactor returns the volume factor
func (t *T3) GetVFactor() float64 {
	return t.vFactor
}

// GetTrend returns the trend direction
// Returns 1 for uptrend, -1 for downtrend, 0 for neutral
func (t *T3) GetTrend() int {
	if len(t.values) < 2 {
		return 0
	}

	current := t.values[len(t.values)-1]
	previous := t.values[len(t.values)-2]

	if current > previous {
		return 1
	} else if current < previous {
		return -1
	}

	return 0
}

// GetSlope returns the recent slope (last 3 periods)
// Positive slope indicates uptrend, negative indicates downtrend
func (t *T3) GetSlope() float64 {
	if len(t.values) < 3 {
		return 0
	}

	n := len(t.values)
	return (t.values[n-1] - t.values[n-3]) / 2.0
}
