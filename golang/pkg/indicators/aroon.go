package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// Aroon indicator identifies trend strength and direction by measuring
// the time elapsed since the highest high and lowest low
//
// Components:
// - Aroon Up: Measures time since highest high
// - Aroon Down: Measures time since lowest low
// - Aroon Oscillator: Difference between Aroon Up and Aroon Down
//
// Formula:
// Aroon Up = 100 × (period - periods since highest high) / period
// Aroon Down = 100 × (period - periods since lowest low) / period
// Aroon Oscillator = Aroon Up - Aroon Down
//
// Range: 0 to 100 (Aroon Up/Down), -100 to +100 (Oscillator)
// - Aroon Up > 70: Strong uptrend
// - Aroon Down > 70: Strong downtrend
// - Aroon Up crosses above Aroon Down: Bullish signal
// - Aroon Down crosses above Aroon Up: Bearish signal
// - Oscillator > 0: Uptrend dominant
// - Oscillator < 0: Downtrend dominant
//
// Properties:
// - Leading indicator (identifies trends early)
// - Works well in trending markets
// - Simple and intuitive calculation
// - Good for trend confirmation
type Aroon struct {
	*BaseIndicator
	period         int
	highs          []float64
	lows           []float64
	aroonUp        float64
	aroonDown      float64
	oscillator     float64
	prevAroonUp    float64
	prevAroonDown  float64
}

// NewAroon creates a new Aroon indicator
func NewAroon(period int, maxHistory int) *Aroon {
	if period <= 0 {
		period = 25
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &Aroon{
		BaseIndicator: NewBaseIndicator("Aroon", maxHistory),
		period:        period,
		highs:         make([]float64, 0, period),
		lows:          make([]float64, 0, period),
	}
}

// NewAroonFromConfig creates Aroon from configuration
func NewAroonFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 25
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

	return NewAroon(period, maxHistory), nil
}

// Update updates the indicator with new market data
func (a *Aroon) Update(md *mdpb.MarketDataUpdate) {
	close := GetMidPrice(md)
	if close <= 0 {
		return
	}

	high := close
	low := close
	if len(md.AskPrice) > 0 && md.AskPrice[0] > high {
		high = md.AskPrice[0]
	}
	if len(md.BidPrice) > 0 && md.BidPrice[0] < low {
		low = md.BidPrice[0]
	}

	// Store prices
	a.highs = append(a.highs, high)
	a.lows = append(a.lows, low)

	// Keep only period elements
	if len(a.highs) > a.period {
		a.highs = a.highs[1:]
		a.lows = a.lows[1:]
	}

	// Need at least period values
	if len(a.highs) < a.period {
		return
	}

	// Find periods since highest high
	highestIdx := 0
	highestHigh := a.highs[0]
	for i := 1; i < len(a.highs); i++ {
		if a.highs[i] >= highestHigh {
			highestHigh = a.highs[i]
			highestIdx = i
		}
	}
	periodsSinceHigh := len(a.highs) - 1 - highestIdx

	// Find periods since lowest low
	lowestIdx := 0
	lowestLow := a.lows[0]
	for i := 1; i < len(a.lows); i++ {
		if a.lows[i] <= lowestLow {
			lowestLow = a.lows[i]
			lowestIdx = i
		}
	}
	periodsSinceLow := len(a.lows) - 1 - lowestIdx

	// Calculate Aroon Up and Aroon Down
	// Aroon Up = 100 × (period - periods since highest high) / period
	// Aroon Down = 100 × (period - periods since lowest low) / period
	a.prevAroonUp = a.aroonUp
	a.prevAroonDown = a.aroonDown

	a.aroonUp = 100.0 * float64(a.period-periodsSinceHigh) / float64(a.period)
	a.aroonDown = 100.0 * float64(a.period-periodsSinceLow) / float64(a.period)

	// Calculate Aroon Oscillator
	a.oscillator = a.aroonUp - a.aroonDown

	a.AddValue(a.oscillator)
}

// GetValue returns the Aroon Oscillator value
func (a *Aroon) GetValue() float64 {
	return a.oscillator
}

// GetAroonUp returns the Aroon Up value
func (a *Aroon) GetAroonUp() float64 {
	return a.aroonUp
}

// GetAroonDown returns the Aroon Down value
func (a *Aroon) GetAroonDown() float64 {
	return a.aroonDown
}

// GetOscillator returns the Aroon Oscillator value (same as GetValue)
func (a *Aroon) GetOscillator() float64 {
	return a.oscillator
}

// Reset resets the indicator
func (a *Aroon) Reset() {
	a.BaseIndicator.Reset()
	a.highs = a.highs[:0]
	a.lows = a.lows[:0]
	a.aroonUp = 0
	a.aroonDown = 0
	a.oscillator = 0
	a.prevAroonUp = 0
	a.prevAroonDown = 0
}

// IsReady returns true if the indicator has enough data
func (a *Aroon) IsReady() bool {
	return len(a.highs) >= a.period
}

// GetPeriod returns the period
func (a *Aroon) GetPeriod() int {
	return a.period
}

// IsStrongUptrend returns true if Aroon Up > 70
func (a *Aroon) IsStrongUptrend() bool {
	return a.aroonUp > 70.0
}

// IsStrongDowntrend returns true if Aroon Down > 70
func (a *Aroon) IsStrongDowntrend() bool {
	return a.aroonDown > 70.0
}

// IsBullishCross returns true if Aroon Up just crossed above Aroon Down
func (a *Aroon) IsBullishCross() bool {
	return a.prevAroonUp <= a.prevAroonDown && a.aroonUp > a.aroonDown
}

// IsBearishCross returns true if Aroon Down just crossed above Aroon Up
func (a *Aroon) IsBearishCross() bool {
	return a.prevAroonDown <= a.prevAroonUp && a.aroonDown > a.aroonUp
}

// IsConsolidating returns true if both Aroon Up and Down are low (< 50)
func (a *Aroon) IsConsolidating() bool {
	return a.aroonUp < 50.0 && a.aroonDown < 50.0
}

// IsTrendingUp returns true if oscillator > 0 (Aroon Up > Aroon Down)
func (a *Aroon) IsTrendingUp() bool {
	return a.oscillator > 0
}

// IsTrendingDown returns true if oscillator < 0 (Aroon Down > Aroon Up)
func (a *Aroon) IsTrendingDown() bool {
	return a.oscillator < 0
}
