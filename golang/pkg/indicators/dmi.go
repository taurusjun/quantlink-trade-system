package indicators

import (
	"math"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// DMI (Directional Movement Index) measures the strength and direction of a trend
// This is a simplified version focusing on +DI and -DI components
//
// Components:
// - +DI (Positive Directional Indicator): Upward movement strength
// - -DI (Negative Directional Indicator): Downward movement strength
// - DI Spread: Difference between +DI and -DI
//
// Calculation:
// 1. True Range (TR) = max(High-Low, |High-PrevClose|, |Low-PrevClose|)
// 2. +DM = High - PrevHigh (if > 0 and > -DM, else 0)
// 3. -DM = PrevLow - Low (if > 0 and > +DM, else 0)
// 4. Smoothed TR, +DM, -DM using Wilder's smoothing
// 5. +DI = 100 × (Smoothed +DM / Smoothed TR)
// 6. -DI = 100 × (Smoothed -DM / Smoothed TR)
//
// Range: 0 to 100 (both +DI and -DI)
// - +DI > -DI: Uptrend
// - -DI > +DI: Downtrend
// - +DI crosses above -DI: Bullish signal
// - -DI crosses above +DI: Bearish signal
//
// Properties:
// - Similar to ADX but focuses on directional components
// - Good for identifying trend direction
// - Works well with other trend indicators
type DMI struct {
	*BaseIndicator
	period          int
	highs           []float64
	lows            []float64
	closes          []float64
	trueRanges      []float64
	plusDM          []float64
	minusDM         []float64
	smoothedTR      float64
	smoothedPlusDM  float64
	smoothedMinusDM float64
	plusDI          float64
	minusDI         float64
	prevPlusDI      float64
	prevMinusDI     float64
	isFirstSmooth   bool
}

// NewDMI creates a new DMI indicator
func NewDMI(period int, maxHistory int) *DMI {
	if period <= 0 {
		period = 14
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &DMI{
		BaseIndicator: NewBaseIndicator("DMI", maxHistory),
		period:        period,
		highs:         make([]float64, 0, period+1),
		lows:          make([]float64, 0, period+1),
		closes:        make([]float64, 0, period+1),
		trueRanges:    make([]float64, 0, period),
		plusDM:        make([]float64, 0, period),
		minusDM:       make([]float64, 0, period),
		isFirstSmooth: true,
	}
}

// NewDMIFromConfig creates DMI from configuration
func NewDMIFromConfig(config map[string]interface{}) (Indicator, error) {
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

	return NewDMI(period, maxHistory), nil
}

// Update updates the indicator with new market data
func (d *DMI) Update(md *mdpb.MarketDataUpdate) {
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
	d.highs = append(d.highs, high)
	d.lows = append(d.lows, low)
	d.closes = append(d.closes, close)

	// Keep period+1 for calculations
	if len(d.highs) > d.period+1 {
		d.highs = d.highs[1:]
		d.lows = d.lows[1:]
		d.closes = d.closes[1:]
	}

	// Need at least 2 bars
	if len(d.highs) < 2 {
		return
	}

	// Calculate True Range (TR)
	prevClose := d.closes[len(d.closes)-2]
	tr := math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))

	// Calculate +DM and -DM
	prevHigh := d.highs[len(d.highs)-2]
	prevLow := d.lows[len(d.lows)-2]

	upMove := high - prevHigh
	downMove := prevLow - low

	var plusDMVal, minusDMVal float64

	if upMove > downMove && upMove > 0 {
		plusDMVal = upMove
	}
	if downMove > upMove && downMove > 0 {
		minusDMVal = downMove
	}

	// Store values
	d.trueRanges = append(d.trueRanges, tr)
	d.plusDM = append(d.plusDM, plusDMVal)
	d.minusDM = append(d.minusDM, minusDMVal)

	// Keep only period values
	if len(d.trueRanges) > d.period {
		d.trueRanges = d.trueRanges[1:]
		d.plusDM = d.plusDM[1:]
		d.minusDM = d.minusDM[1:]
	}

	// Need at least period values for initial smoothing
	if len(d.trueRanges) < d.period {
		return
	}

	// Calculate smoothed values using Wilder's smoothing
	if d.isFirstSmooth {
		// Initial smoothing: simple sum
		d.smoothedTR = 0
		d.smoothedPlusDM = 0
		d.smoothedMinusDM = 0

		for i := 0; i < d.period; i++ {
			d.smoothedTR += d.trueRanges[i]
			d.smoothedPlusDM += d.plusDM[i]
			d.smoothedMinusDM += d.minusDM[i]
		}

		d.isFirstSmooth = false
	} else {
		// Subsequent smoothing: Wilder's method
		d.smoothedTR = (d.smoothedTR*(float64(d.period)-1) + tr) / float64(d.period)
		d.smoothedPlusDM = (d.smoothedPlusDM*(float64(d.period)-1) + plusDMVal) / float64(d.period)
		d.smoothedMinusDM = (d.smoothedMinusDM*(float64(d.period)-1) + minusDMVal) / float64(d.period)
	}

	// Calculate +DI and -DI
	d.prevPlusDI = d.plusDI
	d.prevMinusDI = d.minusDI

	if d.smoothedTR > 0 {
		d.plusDI = 100.0 * d.smoothedPlusDM / d.smoothedTR
		d.minusDI = 100.0 * d.smoothedMinusDM / d.smoothedTR
	} else {
		d.plusDI = 0
		d.minusDI = 0
	}

	// Store DI spread as the value
	spread := d.plusDI - d.minusDI
	d.AddValue(spread)
}

// GetValue returns the DI spread (+DI - -DI)
func (d *DMI) GetValue() float64 {
	return d.plusDI - d.minusDI
}

// GetPlusDI returns the +DI value
func (d *DMI) GetPlusDI() float64 {
	return d.plusDI
}

// GetMinusDI returns the -DI value
func (d *DMI) GetMinusDI() float64 {
	return d.minusDI
}

// Reset resets the indicator
func (d *DMI) Reset() {
	d.BaseIndicator.Reset()
	d.highs = d.highs[:0]
	d.lows = d.lows[:0]
	d.closes = d.closes[:0]
	d.trueRanges = d.trueRanges[:0]
	d.plusDM = d.plusDM[:0]
	d.minusDM = d.minusDM[:0]
	d.smoothedTR = 0
	d.smoothedPlusDM = 0
	d.smoothedMinusDM = 0
	d.plusDI = 0
	d.minusDI = 0
	d.prevPlusDI = 0
	d.prevMinusDI = 0
	d.isFirstSmooth = true
}

// IsReady returns true if the indicator has enough data
func (d *DMI) IsReady() bool {
	return len(d.trueRanges) >= d.period && !d.isFirstSmooth
}

// GetPeriod returns the period
func (d *DMI) GetPeriod() int {
	return d.period
}

// IsTrendingUp returns true if +DI > -DI (uptrend)
func (d *DMI) IsTrendingUp() bool {
	return d.plusDI > d.minusDI
}

// IsTrendingDown returns true if -DI > +DI (downtrend)
func (d *DMI) IsTrendingDown() bool {
	return d.minusDI > d.plusDI
}

// IsBullishCross returns true if +DI just crossed above -DI
func (d *DMI) IsBullishCross() bool {
	return d.prevPlusDI <= d.prevMinusDI && d.plusDI > d.minusDI
}

// IsBearishCross returns true if -DI just crossed above +DI
func (d *DMI) IsBearishCross() bool {
	return d.prevMinusDI <= d.prevPlusDI && d.minusDI > d.plusDI
}
