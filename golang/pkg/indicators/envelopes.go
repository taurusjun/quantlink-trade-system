package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// Envelopes is a channel indicator that plots bands at a fixed percentage
// above and below a moving average (typically SMA or EMA).
//
// Components:
// - Middle Line: Moving average (SMA/EMA)
// - Upper Band: Middle × (1 + percentage/100)
// - Lower Band: Middle × (1 - percentage/100)
//
// Default parameters:
// - MA Type: SMA
// - Period: 20
// - Percentage: 2.5% (0.025)
//
// Interpretation:
// - Price above upper band: Overbought
// - Price below lower band: Oversold
// - Bands expand/contract with price volatility
// - Good for range-bound markets
//
// Properties:
// - Simple and effective
// - Fixed percentage bands (unlike Bollinger which uses std dev)
// - Works well in ranging markets
// - Less sensitive to volatility than Bollinger Bands
type Envelopes struct {
	*BaseIndicator
	period       int
	percentage   float64
	useEMA       bool
	sma          *SMA
	ema          *EMA
	upperBand    float64
	lowerBand    float64
	middleLine   float64
}

// NewEnvelopes creates a new Envelopes indicator
func NewEnvelopes(period int, percentage float64, useEMA bool, maxHistory int) *Envelopes {
	if period <= 0 {
		period = 20
	}
	if percentage <= 0 {
		percentage = 2.5 // 2.5%
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	env := &Envelopes{
		BaseIndicator: NewBaseIndicator("Envelopes", maxHistory),
		period:        period,
		percentage:    percentage,
		useEMA:        useEMA,
	}

	// Create moving average indicator
	if useEMA {
		env.ema = NewEMA(period, maxHistory)
	} else {
		env.sma = NewSMA(float64(period), maxHistory)
	}

	return env
}

// NewEnvelopesFromConfig creates Envelopes from configuration
func NewEnvelopesFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 20
	percentage := 2.5
	useEMA := false
	maxHistory := 1000

	if v, ok := config["period"]; ok {
		if p, ok := v.(float64); ok {
			period = int(p)
		}
	}

	if v, ok := config["percentage"]; ok {
		if p, ok := v.(float64); ok {
			percentage = p
		}
	}

	if v, ok := config["use_ema"]; ok {
		if e, ok := v.(bool); ok {
			useEMA = e
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewEnvelopes(period, percentage, useEMA, maxHistory), nil
}

// Update updates the indicator with new market data
func (e *Envelopes) Update(md *mdpb.MarketDataUpdate) {
	// Update moving average
	if e.useEMA {
		e.ema.Update(md)
		if !e.ema.IsReady() {
			return
		}
		e.middleLine = e.ema.GetValue()
	} else {
		e.sma.Update(md)
		if !e.sma.IsReady() {
			return
		}
		e.middleLine = e.sma.GetValue()
	}

	// Calculate bands
	multiplier := e.percentage / 100.0
	e.upperBand = e.middleLine * (1 + multiplier)
	e.lowerBand = e.middleLine * (1 - multiplier)

	// Store middle line as main value
	e.AddValue(e.middleLine)
}

// GetValue returns the middle line (moving average)
func (e *Envelopes) GetValue() float64 {
	return e.middleLine
}

// GetUpperBand returns the upper band
func (e *Envelopes) GetUpperBand() float64 {
	return e.upperBand
}

// GetLowerBand returns the lower band
func (e *Envelopes) GetLowerBand() float64 {
	return e.lowerBand
}

// GetMiddleLine returns the middle line (moving average)
func (e *Envelopes) GetMiddleLine() float64 {
	return e.middleLine
}

// GetBandWidth returns the width of the bands
func (e *Envelopes) GetBandWidth() float64 {
	return e.upperBand - e.lowerBand
}

// GetBandWidthPercentage returns the band width as percentage
func (e *Envelopes) GetBandWidthPercentage() float64 {
	return e.percentage * 2.0 // Total width is percentage × 2
}

// Reset resets the indicator
func (e *Envelopes) Reset() {
	e.BaseIndicator.Reset()
	if e.useEMA {
		e.ema.Reset()
	} else {
		e.sma.Reset()
	}
	e.upperBand = 0
	e.lowerBand = 0
	e.middleLine = 0
}

// IsReady returns true if the indicator has enough data
func (e *Envelopes) IsReady() bool {
	if e.useEMA {
		return e.ema.IsReady()
	}
	return e.sma.IsReady()
}

// GetPeriod returns the period
func (e *Envelopes) GetPeriod() int {
	return e.period
}

// GetPercentage returns the percentage
func (e *Envelopes) GetPercentage() float64 {
	return e.percentage
}

// IsUsingEMA returns true if using EMA, false if using SMA
func (e *Envelopes) IsUsingEMA() bool {
	return e.useEMA
}

// IsBreakoutUp checks if price broke above upper band
func (e *Envelopes) IsBreakoutUp(currentPrice float64) bool {
	return currentPrice > e.upperBand
}

// IsBreakoutDown checks if price broke below lower band
func (e *Envelopes) IsBreakoutDown(currentPrice float64) bool {
	return currentPrice < e.lowerBand
}

// GetPosition returns the position of price within the bands
// Returns 0-1, where 0=lower band, 1=upper band, 0.5=middle
func (e *Envelopes) GetPosition(currentPrice float64) float64 {
	width := e.GetBandWidth()
	if width == 0 {
		return 0.5
	}

	position := (currentPrice - e.lowerBand) / width

	// Clamp to [0, 1] range
	if position < 0 {
		position = 0
	} else if position > 1 {
		position = 1
	}

	return position
}

// IsOverbought checks if price is overbought (above upper band)
func (e *Envelopes) IsOverbought(currentPrice float64) bool {
	return currentPrice > e.upperBand
}

// IsOversold checks if price is oversold (below lower band)
func (e *Envelopes) IsOversold(currentPrice float64) bool {
	return currentPrice < e.lowerBand
}

// GetSignal returns trading signal based on band position
// Returns 1 for buy (near lower), -1 for sell (near upper), 0 for neutral
func (e *Envelopes) GetSignal(currentPrice float64) int {
	position := e.GetPosition(currentPrice)

	if position < 0.2 {
		return 1 // Near lower band - buy signal
	} else if position > 0.8 {
		return -1 // Near upper band - sell signal
	}

	return 0 // Neutral
}

// GetBandDistance returns the distance of price from the nearest band
// Useful for measuring how far price has deviated
func (e *Envelopes) GetBandDistance(currentPrice float64) float64 {
	distToUpper := e.upperBand - currentPrice
	distToLower := currentPrice - e.lowerBand

	// Return smallest distance (can be negative if outside bands)
	if distToUpper < distToLower {
		return distToUpper
	}
	return distToLower
}
