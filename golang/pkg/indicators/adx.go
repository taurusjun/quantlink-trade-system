package indicators

import (
	"math"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// ADX (Average Directional Index) measures trend strength regardless of direction
//
// Components:
// - +DI (Positive Directional Indicator): Upward trend strength
// - -DI (Negative Directional Indicator): Downward trend strength
// - ADX: Average of DX, measures overall trend strength
//
// Calculation:
// 1. True Range (TR) = max(High-Low, |High-PrevClose|, |Low-PrevClose|)
// 2. +DM = High - PrevHigh (if > 0 and > -DM, else 0)
// 3. -DM = PrevLow - Low (if > 0 and > +DM, else 0)
// 4. Smoothed TR, +DM, -DM using Wilder's smoothing
// 5. +DI = 100 × (Smoothed +DM / Smoothed TR)
// 6. -DI = 100 × (Smoothed -DM / Smoothed TR)
// 7. DX = 100 × |+DI - -DI| / (+DI + -DI)
// 8. ADX = Smoothed DX
//
// Range:
// - ADX: 0 to 100
//   - 0-25: Weak or no trend
//   - 25-50: Strong trend
//   - 50-75: Very strong trend
//   - 75-100: Extremely strong trend
// - +DI, -DI: 0 to 100
//   - +DI > -DI: Uptrend
//   - -DI > +DI: Downtrend
//
// Properties:
// - Trend strength indicator (not direction)
// - Lagging indicator
// - Works well in trending markets
// - Combines well with other indicators for direction
type ADX struct {
	*BaseIndicator
	period         int
	highs          []float64
	lows           []float64
	closes         []float64
	trueRanges     []float64
	plusDM         []float64
	minusDM        []float64
	smoothedTR     float64
	smoothedPlusDM float64
	smoothedMinusDM float64
	plusDI         float64
	minusDI        float64
	dxValues       []float64
	adx            float64
	isFirstSmooth  bool
}

// NewADX creates a new ADX indicator
func NewADX(period int, maxHistory int) *ADX {
	if period <= 0 {
		period = 14
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &ADX{
		BaseIndicator: NewBaseIndicator("ADX", maxHistory),
		period:        period,
		highs:         make([]float64, 0, period+1),
		lows:          make([]float64, 0, period+1),
		closes:        make([]float64, 0, period+1),
		trueRanges:    make([]float64, 0, period),
		plusDM:        make([]float64, 0, period),
		minusDM:       make([]float64, 0, period),
		dxValues:      make([]float64, 0, period),
		isFirstSmooth: true,
	}
}

// NewADXFromConfig creates ADX from configuration
func NewADXFromConfig(config map[string]interface{}) (Indicator, error) {
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

	return NewADX(period, maxHistory), nil
}

// Update updates the indicator with new market data
func (a *ADX) Update(md *mdpb.MarketDataUpdate) {
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
	a.closes = append(a.closes, close)

	// Keep period+1 for calculations
	if len(a.highs) > a.period+1 {
		a.highs = a.highs[1:]
		a.lows = a.lows[1:]
		a.closes = a.closes[1:]
	}

	// Need at least 2 bars
	if len(a.highs) < 2 {
		return
	}

	// Calculate True Range (TR)
	prevClose := a.closes[len(a.closes)-2]
	tr := math.Max(high-low, math.Max(math.Abs(high-prevClose), math.Abs(low-prevClose)))

	// Calculate +DM and -DM
	prevHigh := a.highs[len(a.highs)-2]
	prevLow := a.lows[len(a.lows)-2]

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
	a.trueRanges = append(a.trueRanges, tr)
	a.plusDM = append(a.plusDM, plusDMVal)
	a.minusDM = append(a.minusDM, minusDMVal)

	// Keep only period values
	if len(a.trueRanges) > a.period {
		a.trueRanges = a.trueRanges[1:]
		a.plusDM = a.plusDM[1:]
		a.minusDM = a.minusDM[1:]
	}

	// Need at least period values for initial smoothing
	if len(a.trueRanges) < a.period {
		return
	}

	// Calculate smoothed values using Wilder's smoothing
	if a.isFirstSmooth {
		// Initial smoothing: simple sum
		a.smoothedTR = 0
		a.smoothedPlusDM = 0
		a.smoothedMinusDM = 0

		for i := 0; i < a.period; i++ {
			a.smoothedTR += a.trueRanges[i]
			a.smoothedPlusDM += a.plusDM[i]
			a.smoothedMinusDM += a.minusDM[i]
		}

		a.isFirstSmooth = false
	} else {
		// Subsequent smoothing: Wilder's method
		// Smoothed = (Previous Smoothed × (period-1) + Current) / period
		a.smoothedTR = (a.smoothedTR*(float64(a.period)-1) + tr) / float64(a.period)
		a.smoothedPlusDM = (a.smoothedPlusDM*(float64(a.period)-1) + plusDMVal) / float64(a.period)
		a.smoothedMinusDM = (a.smoothedMinusDM*(float64(a.period)-1) + minusDMVal) / float64(a.period)
	}

	// Calculate +DI and -DI
	// +DI = 100 × (Smoothed +DM / Smoothed TR)
	// -DI = 100 × (Smoothed -DM / Smoothed TR)
	if a.smoothedTR > 0 {
		a.plusDI = 100.0 * a.smoothedPlusDM / a.smoothedTR
		a.minusDI = 100.0 * a.smoothedMinusDM / a.smoothedTR
	} else {
		a.plusDI = 0
		a.minusDI = 0
	}

	// Calculate DX
	// DX = 100 × |+DI - -DI| / (+DI + -DI)
	diSum := a.plusDI + a.minusDI
	var dx float64

	if diSum > 0 {
		dx = 100.0 * math.Abs(a.plusDI-a.minusDI) / diSum
	} else {
		dx = 0
	}

	// Store DX
	a.dxValues = append(a.dxValues, dx)

	// Keep only period DX values for ADX calculation
	if len(a.dxValues) > a.period {
		a.dxValues = a.dxValues[len(a.dxValues)-a.period:]
	}

	// Calculate ADX (smoothed DX)
	// Need at least period DX values
	if len(a.dxValues) >= a.period {
		if a.adx == 0 {
			// Initial ADX: simple average
			sum := 0.0
			for _, dxVal := range a.dxValues {
				sum += dxVal
			}
			a.adx = sum / float64(len(a.dxValues))
		} else {
			// Subsequent ADX: Wilder's smoothing
			a.adx = (a.adx*(float64(a.period)-1) + dx) / float64(a.period)
		}

		a.AddValue(a.adx)
	}
}

// GetValue returns the current ADX value
func (a *ADX) GetValue() float64 {
	return a.adx
}

// GetPlusDI returns the current +DI value
func (a *ADX) GetPlusDI() float64 {
	return a.plusDI
}

// GetMinusDI returns the current -DI value
func (a *ADX) GetMinusDI() float64 {
	return a.minusDI
}

// Reset resets the indicator
func (a *ADX) Reset() {
	a.BaseIndicator.Reset()
	a.highs = a.highs[:0]
	a.lows = a.lows[:0]
	a.closes = a.closes[:0]
	a.trueRanges = a.trueRanges[:0]
	a.plusDM = a.plusDM[:0]
	a.minusDM = a.minusDM[:0]
	a.dxValues = a.dxValues[:0]
	a.smoothedTR = 0
	a.smoothedPlusDM = 0
	a.smoothedMinusDM = 0
	a.plusDI = 0
	a.minusDI = 0
	a.adx = 0
	a.isFirstSmooth = true
}

// IsReady returns true if the indicator has enough data
func (a *ADX) IsReady() bool {
	return len(a.dxValues) >= a.period && a.adx > 0
}

// GetPeriod returns the period
func (a *ADX) GetPeriod() int {
	return a.period
}

// IsTrendingUp returns true if +DI > -DI (uptrend)
func (a *ADX) IsTrendingUp() bool {
	return a.plusDI > a.minusDI
}

// IsTrendingDown returns true if -DI > +DI (downtrend)
func (a *ADX) IsTrendingDown() bool {
	return a.minusDI > a.plusDI
}

// IsStrongTrend returns true if ADX indicates strong trend (> 25)
func (a *ADX) IsStrongTrend() bool {
	return a.adx > 25.0
}

// IsVeryStrongTrend returns true if ADX indicates very strong trend (> 50)
func (a *ADX) IsVeryStrongTrend() bool {
	return a.adx > 50.0
}

// IsWeakTrend returns true if ADX indicates weak trend (< 25)
func (a *ADX) IsWeakTrend() bool {
	return a.adx < 25.0
}

// GetTrendStrength returns a descriptive string for trend strength
func (a *ADX) GetTrendStrength() string {
	if a.adx < 25 {
		return "Weak/No Trend"
	} else if a.adx < 50 {
		return "Strong Trend"
	} else if a.adx < 75 {
		return "Very Strong Trend"
	}
	return "Extremely Strong Trend"
}
