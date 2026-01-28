package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// WilliamsR (Williams %R) is a momentum indicator that measures overbought/oversold levels
// Williams %R is very similar to Stochastic Oscillator but inverted
//
// Formula: %R = -100 × (Highest High - Close) / (Highest High - Lowest Low)
//
// Range: -100 to 0
// - Below -80: Oversold (potential buy signal)
// - Above -20: Overbought (potential sell signal)
//
// Properties:
// - Oscillates between -100 and 0
// - More negative = more oversold
// - Less negative = more overbought
// - Fast and sensitive to price changes
type WilliamsR struct {
	*BaseIndicator
	period    int
	highs     []float64
	lows      []float64
	closes    []float64
	williamsR float64
}

// NewWilliamsR creates a new Williams %R indicator
func NewWilliamsR(period int, maxHistory int) *WilliamsR {
	if period <= 0 {
		period = 14
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &WilliamsR{
		BaseIndicator: NewBaseIndicator("Williams %R", maxHistory),
		period:        period,
		highs:         make([]float64, 0, period),
		lows:          make([]float64, 0, period),
		closes:        make([]float64, 0, period),
	}
}

// NewWilliamsRFromConfig creates Williams %R from configuration
func NewWilliamsRFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 14
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

	return NewWilliamsR(period, maxHistory), nil
}

// Update updates the indicator with new market data
func (w *WilliamsR) Update(md *mdpb.MarketDataUpdate) {
	// Use mid price as close
	close := GetMidPrice(md)
	if close <= 0 {
		return
	}

	// For high/low, we use bid/ask extremes
	high := close
	low := close
	if len(md.AskPrice) > 0 && md.AskPrice[0] > high {
		high = md.AskPrice[0]
	}
	if len(md.BidPrice) > 0 && md.BidPrice[0] < low {
		low = md.BidPrice[0]
	}

	// Add to windows
	w.highs = append(w.highs, high)
	w.lows = append(w.lows, low)
	w.closes = append(w.closes, close)

	// Keep only period elements
	if len(w.highs) > w.period {
		w.highs = w.highs[1:]
		w.lows = w.lows[1:]
		w.closes = w.closes[1:]
	}

	// Need at least period values to calculate
	if len(w.closes) < w.period {
		return
	}

	// Calculate Williams %R
	// %R = -100 × (Highest High - Close) / (Highest High - Lowest Low)
	highestHigh := w.highs[0]
	lowestLow := w.lows[0]

	for i := 1; i < len(w.highs); i++ {
		if w.highs[i] > highestHigh {
			highestHigh = w.highs[i]
		}
		if w.lows[i] < lowestLow {
			lowestLow = w.lows[i]
		}
	}

	currentClose := w.closes[len(w.closes)-1]
	denominator := highestHigh - lowestLow

	if denominator == 0 {
		w.williamsR = -50.0 // Neutral when no range
	} else {
		w.williamsR = -100.0 * (highestHigh - currentClose) / denominator
	}

	w.AddValue(w.williamsR)
}

// GetValue returns the current Williams %R value
func (w *WilliamsR) GetValue() float64 {
	return w.williamsR
}

// Reset resets the indicator
func (w *WilliamsR) Reset() {
	w.BaseIndicator.Reset()
	w.highs = w.highs[:0]
	w.lows = w.lows[:0]
	w.closes = w.closes[:0]
	w.williamsR = 0
}

// IsReady returns true if the indicator has enough data
func (w *WilliamsR) IsReady() bool {
	return len(w.closes) >= w.period
}

// GetPeriod returns the period
func (w *WilliamsR) GetPeriod() int {
	return w.period
}

// IsOverbought returns true if Williams %R indicates overbought (> -20)
func (w *WilliamsR) IsOverbought() bool {
	return w.williamsR > -20.0
}

// IsOversold returns true if Williams %R indicates oversold (< -80)
func (w *WilliamsR) IsOversold() bool {
	return w.williamsR < -80.0
}
