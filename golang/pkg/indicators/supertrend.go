package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// Supertrend is a trend-following indicator that provides dynamic support and resistance levels
//
// Formula:
// 1. Calculate ATR
// 2. Basic Upper Band = (High + Low) / 2 + Multiplier × ATR
// 3. Basic Lower Band = (High + Low) / 2 - Multiplier × ATR
// 4. Final Upper Band = Basic UB < Final UB[-1] or Close[-1] > Final UB[-1] ? Basic UB : Final UB[-1]
// 5. Final Lower Band = Basic LB > Final LB[-1] or Close[-1] < Final LB[-1] ? Basic LB : Final LB[-1]
// 6. Supertrend = Close <= Final UB ? Final UB : Final LB
// 7. Trend = Close > Supertrend ? Up : Down
//
// Parameters:
// - period: ATR period (typically 10)
// - multiplier: ATR multiplier (typically 3.0)
//
// Properties:
// - Provides dynamic support/resistance
// - Stays in trend until reversal
// - Good for trend-following strategies
// - Less prone to whipsaws than simple moving averages
type Supertrend struct {
	*BaseIndicator
	period           int
	multiplier       float64
	atr              *ATR
	prevClose        float64
	prevFinalUpperBand float64
	prevFinalLowerBand float64
	finalUpperBand   float64
	finalLowerBand   float64
	supertrend       float64
	isUptrend        bool
	prevIsUptrend    bool
}

// NewSupertrend creates a new Supertrend indicator
func NewSupertrend(period int, multiplier float64, maxHistory int) *Supertrend {
	if period <= 0 {
		period = 10
	}
	if multiplier <= 0 {
		multiplier = 3.0
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &Supertrend{
		BaseIndicator: NewBaseIndicator("Supertrend", maxHistory),
		period:        period,
		multiplier:    multiplier,
		atr:           NewATR(float64(period), maxHistory),
	}
}

// NewSupertrendFromConfig creates Supertrend from configuration
func NewSupertrendFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 10
	multiplier := 3.0
	maxHistory := 1000

	if v, ok := config["period"]; ok {
		if p, ok := v.(float64); ok {
			period = int(p)
		}
	}

	if v, ok := config["multiplier"]; ok {
		if m, ok := v.(float64); ok {
			multiplier = m
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewSupertrend(period, multiplier, maxHistory), nil
}

// Update updates the indicator with new market data
func (s *Supertrend) Update(md *mdpb.MarketDataUpdate) {
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

	// Update ATR
	s.atr.Update(md)

	if !s.atr.IsReady() {
		s.prevClose = close
		return
	}

	// Calculate HL average (typical price without close component)
	hlAvg := (high + low) / 2.0
	atrValue := s.atr.GetValue()

	// Calculate basic bands
	basicUpperBand := hlAvg + s.multiplier*atrValue
	basicLowerBand := hlAvg - s.multiplier*atrValue

	// Calculate final upper band
	// Final UB = Basic UB < Final UB[-1] or Close[-1] > Final UB[-1] ? Basic UB : Final UB[-1]
	if s.prevFinalUpperBand == 0 || basicUpperBand < s.prevFinalUpperBand || s.prevClose > s.prevFinalUpperBand {
		s.finalUpperBand = basicUpperBand
	} else {
		s.finalUpperBand = s.prevFinalUpperBand
	}

	// Calculate final lower band
	// Final LB = Basic LB > Final LB[-1] or Close[-1] < Final LB[-1] ? Basic LB : Final LB[-1]
	if s.prevFinalLowerBand == 0 || basicLowerBand > s.prevFinalLowerBand || s.prevClose < s.prevFinalLowerBand {
		s.finalLowerBand = basicLowerBand
	} else {
		s.finalLowerBand = s.prevFinalLowerBand
	}

	// Determine trend and supertrend value
	s.prevIsUptrend = s.isUptrend

	if s.prevClose <= s.prevFinalUpperBand {
		s.supertrend = s.finalUpperBand
		s.isUptrend = false
	} else {
		s.supertrend = s.finalLowerBand
		s.isUptrend = true
	}

	// Update for next iteration
	s.prevClose = close
	s.prevFinalUpperBand = s.finalUpperBand
	s.prevFinalLowerBand = s.finalLowerBand

	s.AddValue(s.supertrend)
}

// GetValue returns the current Supertrend value
func (s *Supertrend) GetValue() float64 {
	return s.supertrend
}

// Reset resets the indicator
func (s *Supertrend) Reset() {
	s.BaseIndicator.Reset()
	s.atr.Reset()
	s.prevClose = 0
	s.prevFinalUpperBand = 0
	s.prevFinalLowerBand = 0
	s.finalUpperBand = 0
	s.finalLowerBand = 0
	s.supertrend = 0
	s.isUptrend = false
	s.prevIsUptrend = false
}

// IsReady returns true if the indicator has enough data
func (s *Supertrend) IsReady() bool {
	return s.atr.IsReady() && s.supertrend > 0
}

// GetPeriod returns the period
func (s *Supertrend) GetPeriod() int {
	return s.period
}

// GetMultiplier returns the multiplier
func (s *Supertrend) GetMultiplier() float64 {
	return s.multiplier
}

// IsUptrend returns true if currently in uptrend
func (s *Supertrend) IsUptrend() bool {
	return s.isUptrend
}

// IsDowntrend returns true if currently in downtrend
func (s *Supertrend) IsDowntrend() bool {
	return !s.isUptrend
}

// IsBullishReversal returns true if trend just changed from down to up
func (s *Supertrend) IsBullishReversal() bool {
	return !s.prevIsUptrend && s.isUptrend
}

// IsBearishReversal returns true if trend just changed from up to down
func (s *Supertrend) IsBearishReversal() bool {
	return s.prevIsUptrend && !s.isUptrend
}

// GetUpperBand returns the final upper band
func (s *Supertrend) GetUpperBand() float64 {
	return s.finalUpperBand
}

// GetLowerBand returns the final lower band
func (s *Supertrend) GetLowerBand() float64 {
	return s.finalLowerBand
}
