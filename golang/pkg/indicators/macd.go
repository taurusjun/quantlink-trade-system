// Package indicators provides technical indicators for trading
package indicators

import (
	mdpb "github.com/yourusername/hft-poc/pkg/proto/md"
)

// MACD implements the Moving Average Convergence Divergence indicator
type MACD struct {
	*BaseIndicator
	fastPeriod   int
	slowPeriod   int
	signalPeriod int

	// EMA values
	fastEMA   float64
	slowEMA   float64
	signalEMA float64

	// Alpha values for EMA calculation
	fastAlpha   float64
	slowAlpha   float64
	signalAlpha float64

	// MACD values
	macdLine     float64
	signalLine   float64
	histogram    float64

	// State
	dataPoints int
	isInit     bool
}

// NewMACD creates a new MACD indicator
func NewMACD(fastPeriod, slowPeriod, signalPeriod float64, maxHistory int) *MACD {
	fast := int(fastPeriod)
	slow := int(slowPeriod)
	signal := int(signalPeriod)

	return &MACD{
		BaseIndicator: &BaseIndicator{
			name:       "MACD",
			maxHistory: maxHistory,
		},
		fastPeriod:   fast,
		slowPeriod:   slow,
		signalPeriod: signal,
		fastAlpha:    2.0 / (float64(fast) + 1.0),
		slowAlpha:    2.0 / (float64(slow) + 1.0),
		signalAlpha:  2.0 / (float64(signal) + 1.0),
	}
}

// Update updates the MACD with new market data
func (macd *MACD) Update(md *mdpb.MarketDataUpdate) {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return
	}

	price := (md.BidPrice[0] + md.AskPrice[0]) / 2.0

	if !macd.isInit {
		// Initialize with first price
		macd.fastEMA = price
		macd.slowEMA = price
		macd.signalEMA = 0
		macd.isInit = true
		macd.dataPoints = 1
		return
	}

	macd.dataPoints++

	// Calculate fast and slow EMA
	macd.fastEMA = macd.fastAlpha*price + (1.0-macd.fastAlpha)*macd.fastEMA
	macd.slowEMA = macd.slowAlpha*price + (1.0-macd.slowAlpha)*macd.slowEMA

	// Calculate MACD line (fast EMA - slow EMA)
	macd.macdLine = macd.fastEMA - macd.slowEMA

	// Calculate signal line (EMA of MACD line)
	if macd.dataPoints > macd.slowPeriod {
		if macd.signalEMA == 0 {
			// Initialize signal EMA with first MACD value
			macd.signalEMA = macd.macdLine
		} else {
			macd.signalEMA = macd.signalAlpha*macd.macdLine + (1.0-macd.signalAlpha)*macd.signalEMA
		}

		// Calculate histogram (MACD line - signal line)
		macd.histogram = macd.macdLine - macd.signalEMA
	}
}

// GetValue returns the MACD line value
func (macd *MACD) GetValue() float64 {
	return macd.macdLine
}

// GetValues returns [MACD line, signal line, histogram]
func (macd *MACD) GetValues() []float64 {
	if !macd.IsReady() {
		return []float64{}
	}
	return []float64{macd.macdLine, macd.signalEMA, macd.histogram}
}

// GetMACDLine returns the MACD line value
func (macd *MACD) GetMACDLine() float64 {
	return macd.macdLine
}

// GetSignalLine returns the signal line value
func (macd *MACD) GetSignalLine() float64 {
	return macd.signalEMA
}

// GetHistogram returns the histogram value
func (macd *MACD) GetHistogram() float64 {
	return macd.histogram
}

// IsReady returns true if the indicator has enough data
func (macd *MACD) IsReady() bool {
	return macd.isInit && macd.dataPoints > macd.slowPeriod+macd.signalPeriod
}

// Reset resets the indicator state
func (macd *MACD) Reset() {
	macd.fastEMA = 0
	macd.slowEMA = 0
	macd.signalEMA = 0
	macd.macdLine = 0
	macd.signalLine = 0
	macd.histogram = 0
	macd.dataPoints = 0
	macd.isInit = false
}

// NewMACDFromConfig creates a MACD indicator from config
func NewMACDFromConfig(config map[string]interface{}) (Indicator, error) {
	fastPeriod, ok := config["fast_period"].(float64)
	if !ok {
		fastPeriod = 12.0 // Default
	}

	slowPeriod, ok := config["slow_period"].(float64)
	if !ok {
		slowPeriod = 26.0 // Default
	}

	signalPeriod, ok := config["signal_period"].(float64)
	if !ok {
		signalPeriod = 9.0 // Default
	}

	maxHistory, ok := config["max_history"].(float64)
	if !ok {
		maxHistory = 1000.0
	}

	return NewMACD(fastPeriod, slowPeriod, signalPeriod, int(maxHistory)), nil
}
