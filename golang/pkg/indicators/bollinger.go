// Package indicators provides technical indicators for trading
package indicators

import (
	"math"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// BollingerBands implements the Bollinger Bands indicator
type BollingerBands struct {
	*BaseIndicator
	period      int
	stdDevMult  float64
	prices      []float64
	sma         float64
	upperBand   float64
	middleBand  float64
	lowerBand   float64
	bandwidth   float64
	percentB    float64
	initialized bool
}

// NewBollingerBands creates a new Bollinger Bands indicator
func NewBollingerBands(period float64, stdDevMult float64, maxHistory int) *BollingerBands {
	p := int(period)
	return &BollingerBands{
		BaseIndicator: &BaseIndicator{
			name:       "BollingerBands",
			maxHistory: maxHistory,
		},
		period:      p,
		stdDevMult:  stdDevMult,
		prices:      make([]float64, 0, p),
		initialized: false,
	}
}

// Update updates the Bollinger Bands with new market data
func (bb *BollingerBands) Update(md *mdpb.MarketDataUpdate) {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return
	}

	price := (md.BidPrice[0] + md.AskPrice[0]) / 2.0

	// Add new price
	bb.prices = append(bb.prices, price)

	// Remove oldest price if we exceed the period
	if len(bb.prices) > bb.period {
		bb.prices = bb.prices[1:]
	}

	// Calculate only when we have enough data
	if len(bb.prices) == bb.period {
		bb.initialized = true
		bb.calculate()
	}
}

// calculate computes the Bollinger Bands values
func (bb *BollingerBands) calculate() {
	// Calculate SMA (middle band)
	sum := 0.0
	for _, price := range bb.prices {
		sum += price
	}
	bb.sma = sum / float64(bb.period)
	bb.middleBand = bb.sma

	// Calculate standard deviation
	variance := 0.0
	for _, price := range bb.prices {
		diff := price - bb.sma
		variance += diff * diff
	}
	variance /= float64(bb.period)
	stdDev := math.Sqrt(variance)

	// Calculate upper and lower bands
	bb.upperBand = bb.middleBand + (bb.stdDevMult * stdDev)
	bb.lowerBand = bb.middleBand - (bb.stdDevMult * stdDev)

	// Calculate bandwidth (volatility indicator)
	if bb.middleBand != 0 {
		bb.bandwidth = (bb.upperBand - bb.lowerBand) / bb.middleBand
	}

	// Calculate %B (position within bands)
	currentPrice := bb.prices[len(bb.prices)-1]
	bandRange := bb.upperBand - bb.lowerBand
	if bandRange != 0 {
		bb.percentB = (currentPrice - bb.lowerBand) / bandRange
	}

	// Store middle band as the primary value
	bb.AddValue(bb.middleBand)
}

// GetValue returns the middle band (SMA)
func (bb *BollingerBands) GetValue() float64 {
	return bb.middleBand
}

// GetValues returns [middle, upper, lower]
func (bb *BollingerBands) GetValues() []float64 {
	if !bb.IsReady() {
		return []float64{}
	}
	return []float64{bb.middleBand, bb.upperBand, bb.lowerBand}
}

// GetUpperBand returns the upper band value
func (bb *BollingerBands) GetUpperBand() float64 {
	return bb.upperBand
}

// GetMiddleBand returns the middle band (SMA) value
func (bb *BollingerBands) GetMiddleBand() float64 {
	return bb.middleBand
}

// GetLowerBand returns the lower band value
func (bb *BollingerBands) GetLowerBand() float64 {
	return bb.lowerBand
}

// GetBandwidth returns the bandwidth (volatility indicator)
func (bb *BollingerBands) GetBandwidth() float64 {
	return bb.bandwidth
}

// GetPercentB returns the %B value (position within bands)
func (bb *BollingerBands) GetPercentB() float64 {
	return bb.percentB
}

// IsReady returns true if the indicator has enough data
func (bb *BollingerBands) IsReady() bool {
	return bb.initialized && len(bb.prices) == bb.period
}

// Reset resets the indicator state
func (bb *BollingerBands) Reset() {
	bb.prices = make([]float64, 0, bb.period)
	bb.sma = 0
	bb.upperBand = 0
	bb.middleBand = 0
	bb.lowerBand = 0
	bb.bandwidth = 0
	bb.percentB = 0
	bb.initialized = false
	bb.BaseIndicator.Reset()
}

// GetPeriod returns the period setting
func (bb *BollingerBands) GetPeriod() int {
	return bb.period
}

// GetStdDevMult returns the standard deviation multiplier
func (bb *BollingerBands) GetStdDevMult() float64 {
	return bb.stdDevMult
}

// IsOverbought returns true if price is above upper band
func (bb *BollingerBands) IsOverbought() bool {
	if !bb.IsReady() || len(bb.prices) == 0 {
		return false
	}
	currentPrice := bb.prices[len(bb.prices)-1]
	return currentPrice > bb.upperBand
}

// IsOversold returns true if price is below lower band
func (bb *BollingerBands) IsOversold() bool {
	if !bb.IsReady() || len(bb.prices) == 0 {
		return false
	}
	currentPrice := bb.prices[len(bb.prices)-1]
	return currentPrice < bb.lowerBand
}

// NewBollingerBandsFromConfig creates a Bollinger Bands indicator from config
func NewBollingerBandsFromConfig(config map[string]interface{}) (Indicator, error) {
	period, ok := config["period"].(float64)
	if !ok {
		period = 20.0 // Default 20-period
	}

	stdDevMult, ok := config["std_dev"].(float64)
	if !ok {
		stdDevMult = 2.0 // Default 2 standard deviations
	}

	maxHistory, ok := config["max_history"].(float64)
	if !ok {
		maxHistory = 1000.0
	}

	return NewBollingerBands(period, stdDevMult, int(maxHistory)), nil
}
