package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// TRIX (Triple Exponential Average) is a momentum oscillator that shows the
// percentage rate of change of a triple exponentially smoothed moving average
//
// Formula:
// 1. EMA1 = EMA(Close, period)
// 2. EMA2 = EMA(EMA1, period)
// 3. EMA3 = EMA(EMA2, period)
// 4. TRIX = 100 × (EMA3 - EMA3_prev) / EMA3_prev
//
// Alternative: TRIX = 100 × ROC(EMA3, 1)
//
// Range: Unbounded, typically -5 to +5
// - Positive: Uptrend
// - Negative: Downtrend
// - Crosses above 0: Bullish signal
// - Crosses below 0: Bearish signal
//
// Properties:
// - Filters out market noise with triple smoothing
// - Lagging indicator (high smoothing)
// - Good for identifying major trend changes
// - Less prone to false signals
type TRIX struct {
	*BaseIndicator
	period   int
	ema1     *EMA
	ema2     *EMA
	ema3     *EMA
	prevEMA3 float64
	trix     float64
	prevTRIX float64
}

// NewTRIX creates a new TRIX indicator
func NewTRIX(period int, maxHistory int) *TRIX {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &TRIX{
		BaseIndicator: NewBaseIndicator("TRIX", maxHistory),
		period:        period,
		ema1:          NewEMA(period, maxHistory),
		ema2:          NewEMA(period, maxHistory),
		ema3:          NewEMA(period, maxHistory),
	}
}

// NewTRIXFromConfig creates TRIX from configuration
func NewTRIXFromConfig(config map[string]interface{}) (Indicator, error) {
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

	return NewTRIX(period, maxHistory), nil
}

// Update updates the indicator with new market data
func (t *TRIX) Update(md *mdpb.MarketDataUpdate) {
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

	// Calculate TRIX: 100 × (EMA3 - EMA3_prev) / EMA3_prev
	if t.ema3.IsReady() && t.prevEMA3 > 0 {
		currentEMA3 := t.ema3.GetValue()
		t.prevTRIX = t.trix
		t.trix = 100.0 * (currentEMA3 - t.prevEMA3) / t.prevEMA3
		t.prevEMA3 = currentEMA3
		t.AddValue(t.trix)
	} else if t.ema3.IsReady() {
		// First time EMA3 is ready, store it
		t.prevEMA3 = t.ema3.GetValue()
	}
}

// GetValue returns the current TRIX value
func (t *TRIX) GetValue() float64 {
	return t.trix
}

// Reset resets the indicator
func (t *TRIX) Reset() {
	t.BaseIndicator.Reset()
	t.ema1.Reset()
	t.ema2.Reset()
	t.ema3.Reset()
	t.prevEMA3 = 0
	t.trix = 0
	t.prevTRIX = 0
}

// IsReady returns true if TRIX has valid values
func (t *TRIX) IsReady() bool {
	return t.ema3.IsReady() && t.prevEMA3 > 0 && len(t.GetValues()) > 0
}

// GetPeriod returns the period
func (t *TRIX) GetPeriod() int {
	return t.period
}

// IsPositive returns true if TRIX is positive (uptrend)
func (t *TRIX) IsPositive() bool {
	return t.trix > 0
}

// IsNegative returns true if TRIX is negative (downtrend)
func (t *TRIX) IsNegative() bool {
	return t.trix < 0
}

// IsBullishCross returns true if TRIX just crossed above 0
func (t *TRIX) IsBullishCross() bool {
	return t.prevTRIX <= 0 && t.trix > 0
}

// IsBearishCross returns true if TRIX just crossed below 0
func (t *TRIX) IsBearishCross() bool {
	return t.prevTRIX >= 0 && t.trix < 0
}

// GetEMA3 returns the triple smoothed EMA value
func (t *TRIX) GetEMA3() float64 {
	return t.ema3.GetValue()
}
