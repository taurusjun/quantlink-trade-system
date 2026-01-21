// Package indicators provides technical indicators for trading
package indicators

import (
	"math"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// ATR implements the Average True Range indicator
type ATR struct {
	*BaseIndicator
	period      int
	atr         float64
	prevClose   float64
	hasPrevious bool
	dataPoints  int
	trValues    []float64 // Store TR values for calculation
}

// NewATR creates a new ATR indicator
func NewATR(period float64, maxHistory int) *ATR {
	p := int(period)
	return &ATR{
		BaseIndicator: &BaseIndicator{
			name:       "ATR",
			maxHistory: maxHistory,
		},
		period:      p,
		atr:         0.0,
		prevClose:   0.0,
		hasPrevious: false,
		dataPoints:  0,
		trValues:    make([]float64, 0, p),
	}
}

// Update updates the ATR with new market data
func (atr *ATR) Update(md *mdpb.MarketDataUpdate) {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return
	}

	// Approximate high and low from bid/ask
	high := md.AskPrice[0]
	low := md.BidPrice[0]
	close := (md.BidPrice[0] + md.AskPrice[0]) / 2.0

	// Calculate True Range
	tr := atr.calculateTrueRange(high, low, close)

	// Store TR value
	atr.trValues = append(atr.trValues, tr)
	if len(atr.trValues) > atr.period {
		atr.trValues = atr.trValues[1:]
	}

	atr.dataPoints++

	// Calculate ATR
	if atr.dataPoints < atr.period {
		// Initial period: use simple average of TR
		sum := 0.0
		for _, trVal := range atr.trValues {
			sum += trVal
		}
		atr.atr = sum / float64(len(atr.trValues))
	} else if atr.dataPoints == atr.period {
		// First ATR: simple average
		sum := 0.0
		for _, trVal := range atr.trValues {
			sum += trVal
		}
		atr.atr = sum / float64(atr.period)
	} else {
		// Subsequent ATR: Wilder's smoothing
		// ATR = [(prior ATR * (n-1)) + current TR] / n
		atr.atr = ((atr.atr * float64(atr.period-1)) + tr) / float64(atr.period)
	}

	// Store current close as previous for next iteration
	atr.prevClose = close
	atr.hasPrevious = true

	// Store ATR value in history
	if atr.IsReady() {
		atr.AddValue(atr.atr)
	}
}

// calculateTrueRange calculates the True Range
// TR = max(high - low, |high - prev_close|, |low - prev_close|)
func (atr *ATR) calculateTrueRange(high, low, close float64) float64 {
	if !atr.hasPrevious {
		// For first bar, TR is just high - low
		return high - low
	}

	// Calculate three possible ranges
	highLow := high - low
	highClose := math.Abs(high - atr.prevClose)
	lowClose := math.Abs(low - atr.prevClose)

	// Return maximum
	tr := highLow
	if highClose > tr {
		tr = highClose
	}
	if lowClose > tr {
		tr = lowClose
	}

	return tr
}

// GetValue returns the current ATR value
func (atr *ATR) GetValue() float64 {
	return atr.atr
}

// IsReady returns true if the indicator has enough data
func (atr *ATR) IsReady() bool {
	return atr.dataPoints >= atr.period
}

// Reset resets the indicator state
func (atr *ATR) Reset() {
	atr.atr = 0.0
	atr.prevClose = 0.0
	atr.hasPrevious = false
	atr.dataPoints = 0
	atr.trValues = make([]float64, 0, atr.period)
	atr.BaseIndicator.Reset()
}

// GetPeriod returns the period setting
func (atr *ATR) GetPeriod() int {
	return atr.period
}

// GetDataPoints returns the number of data points received
func (atr *ATR) GetDataPoints() int {
	return atr.dataPoints
}

// GetTRValues returns the current TR values (for testing)
func (atr *ATR) GetTRValues() []float64 {
	result := make([]float64, len(atr.trValues))
	copy(result, atr.trValues)
	return result
}

// NewATRFromConfig creates an ATR indicator from config
func NewATRFromConfig(config map[string]interface{}) (Indicator, error) {
	period, ok := config["period"].(float64)
	if !ok {
		period = 14.0 // Default 14-period ATR
	}

	maxHistory, ok := config["max_history"].(float64)
	if !ok {
		maxHistory = 1000.0
	}

	return NewATR(period, int(maxHistory)), nil
}
