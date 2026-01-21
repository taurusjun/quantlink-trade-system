package indicators

import (
	"fmt"
	"math"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// EWMA (Exponentially Weighted Moving Average) calculates exponential moving average
type EWMA struct {
	*BaseIndicator
	alpha      float64 // Decay factor (0 < alpha <= 1)
	value      float64 // Current EWMA value
	isInit     bool    // Initialization flag
	useLogPrices bool  // Use log prices for calculation
}

// NewEWMA creates a new EWMA indicator
// alpha: decay factor, typically 2/(N+1) where N is the period
// For example, alpha=0.1 is approximately a 19-period EMA
func NewEWMA(alpha float64, maxHistory int) *EWMA {
	if alpha <= 0 || alpha > 1 {
		alpha = 0.1 // Default to ~19-period EMA
	}

	return &EWMA{
		BaseIndicator: NewBaseIndicator("EWMA", maxHistory),
		alpha:         alpha,
		useLogPrices:  false,
	}
}

// NewEWMAFromPeriod creates EWMA from period (e.g., 10, 20, 50, 200)
func NewEWMAFromPeriod(period int, maxHistory int) *EWMA {
	alpha := 2.0 / float64(period+1)
	return NewEWMA(alpha, maxHistory)
}

// NewEWMAFromConfig creates EWMA from configuration map
func NewEWMAFromConfig(config map[string]interface{}) (Indicator, error) {
	alpha := 0.1
	maxHistory := 1000
	useLogPrices := false

	if v, ok := config["alpha"]; ok {
		if a, ok := v.(float64); ok {
			alpha = a
		}
	}

	if v, ok := config["period"]; ok {
		if p, ok := v.(float64); ok {
			alpha = 2.0 / (p + 1)
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	if v, ok := config["use_log_prices"]; ok {
		if l, ok := v.(bool); ok {
			useLogPrices = l
		}
	}

	if alpha <= 0 || alpha > 1 {
		return nil, fmt.Errorf("%w: alpha must be in (0, 1]", ErrInvalidParameter)
	}

	ewma := NewEWMA(alpha, maxHistory)
	ewma.useLogPrices = useLogPrices
	return ewma, nil
}

// Update updates the EWMA with new market data
func (e *EWMA) Update(md *mdpb.MarketDataUpdate) {
	midPrice := GetMidPrice(md)
	if midPrice == 0 {
		return
	}

	price := midPrice
	if e.useLogPrices && midPrice > 0 {
		price = math.Log(midPrice)
	}

	if !e.isInit {
		e.value = price
		e.isInit = true
	} else {
		// EWMA formula: value = alpha * price + (1 - alpha) * previous_value
		e.value = e.alpha*price + (1-e.alpha)*e.value
	}

	// Store the actual value (not log if using log prices)
	if e.useLogPrices {
		e.AddValue(math.Exp(e.value))
	} else {
		e.AddValue(e.value)
	}
}

// GetValue returns the current EWMA value
func (e *EWMA) GetValue() float64 {
	if !e.isInit {
		return 0.0
	}

	if e.useLogPrices {
		return math.Exp(e.value)
	}
	return e.value
}

// Reset resets the EWMA to initial state
func (e *EWMA) Reset() {
	e.BaseIndicator.Reset()
	e.value = 0.0
	e.isInit = false
}

// IsReady returns true if EWMA has been initialized
func (e *EWMA) IsReady() bool {
	return e.isInit
}

// GetAlpha returns the alpha parameter
func (e *EWMA) GetAlpha() float64 {
	return e.alpha
}

// GetEquivalentPeriod returns the equivalent simple moving average period
func (e *EWMA) GetEquivalentPeriod() int {
	return int(math.Round((2.0/e.alpha) - 1))
}

// DEMA (Double Exponential Moving Average) for trend smoothing
type DEMA struct {
	*BaseIndicator
	ema1  *EWMA
	ema2  *EWMA
	alpha float64
}

// NewDEMA creates a new DEMA indicator
func NewDEMA(alpha float64, maxHistory int) *DEMA {
	return &DEMA{
		BaseIndicator: NewBaseIndicator("DEMA", maxHistory),
		ema1:          NewEWMA(alpha, maxHistory),
		ema2:          NewEWMA(alpha, maxHistory),
		alpha:         alpha,
	}
}

// Update updates the DEMA with new market data
func (d *DEMA) Update(md *mdpb.MarketDataUpdate) {
	d.ema1.Update(md)
	if d.ema1.IsReady() {
		// Create synthetic market data for second EMA
		syntheticMd := &mdpb.MarketDataUpdate{
			Symbol:    md.Symbol,
			Exchange:  md.Exchange,
			Timestamp: md.Timestamp,
			BidPrice:  []float64{d.ema1.GetValue()},
			AskPrice:  []float64{d.ema1.GetValue()},
		}
		d.ema2.Update(syntheticMd)
	}

	if d.IsReady() {
		// DEMA = 2 * EMA1 - EMA2
		value := 2*d.ema1.GetValue() - d.ema2.GetValue()
		d.AddValue(value)
	}
}

// Reset resets the DEMA to initial state
func (d *DEMA) Reset() {
	d.BaseIndicator.Reset()
	d.ema1.Reset()
	d.ema2.Reset()
}

// IsReady returns true if DEMA is ready
func (d *DEMA) IsReady() bool {
	return d.ema1.IsReady() && d.ema2.IsReady()
}
