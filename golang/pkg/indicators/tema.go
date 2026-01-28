package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// TEMA (Triple Exponential Moving Average) calculates the triple exponential moving average
// TEMA further reduces lag by using three EMAs
//
// Formula: TEMA = 3 × EMA - 3 × EMA(EMA) + EMA(EMA(EMA))
//
// Properties:
// - Minimal lag
// - Very responsive to price changes
// - Can overshoot in volatile markets
// - Excellent for short-term trading
type TEMA struct {
	*BaseIndicator
	period int
	ema1   *EMA // First EMA
	ema2   *EMA // EMA of EMA
	ema3   *EMA // EMA of EMA of EMA
	tema   float64
}

// NewTEMA creates a new TEMA indicator
func NewTEMA(period int, maxHistory int) *TEMA {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &TEMA{
		BaseIndicator: NewBaseIndicator("TEMA", maxHistory),
		period:        period,
		ema1:          NewEMA(period, maxHistory),
		ema2:          NewEMA(period, maxHistory),
		ema3:          NewEMA(period, maxHistory),
	}
}

// NewTEMAFromConfig creates TEMA from configuration
func NewTEMAFromConfig(config map[string]interface{}) (Indicator, error) {
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

	return NewTEMA(period, maxHistory), nil
}

// Update updates the indicator with new market data
func (t *TEMA) Update(md *mdpb.MarketDataUpdate) {
	price := GetMidPrice(md)
	if price <= 0 {
		return
	}

	// Update first EMA with price
	t.ema1.Update(md)

	// Update second EMA with EMA1's value
	if t.ema1.IsReady() {
		ema1Value := t.ema1.GetValue()
		mdEMA1 := &mdpb.MarketDataUpdate{
			BidPrice: []float64{ema1Value},
			AskPrice: []float64{ema1Value},
		}
		t.ema2.Update(mdEMA1)
	}

	// Update third EMA with EMA2's value
	if t.ema2.IsReady() {
		ema2Value := t.ema2.GetValue()
		mdEMA2 := &mdpb.MarketDataUpdate{
			BidPrice: []float64{ema2Value},
			AskPrice: []float64{ema2Value},
		}
		t.ema3.Update(mdEMA2)
	}

	// Calculate TEMA: 3 × EMA - 3 × EMA(EMA) + EMA(EMA(EMA))
	if t.ema3.IsReady() {
		t.tema = 3*t.ema1.GetValue() - 3*t.ema2.GetValue() + t.ema3.GetValue()
		t.AddValue(t.tema)
	}
}

// GetValue returns the current TEMA value
func (t *TEMA) GetValue() float64 {
	return t.tema
}

// Reset resets the indicator
func (t *TEMA) Reset() {
	t.BaseIndicator.Reset()
	t.ema1.Reset()
	t.ema2.Reset()
	t.ema3.Reset()
	t.tema = 0
}

// IsReady returns true if TEMA has valid values
func (t *TEMA) IsReady() bool {
	return t.ema3.IsReady()
}

// GetPeriod returns the period
func (t *TEMA) GetPeriod() int {
	return t.period
}
