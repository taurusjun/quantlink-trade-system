package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// ParabolicSAR (Stop and Reverse) is a trend-following indicator that provides
// entry and exit points based on price momentum
//
// Formula:
// SAR(n+1) = SAR(n) + AF × (EP - SAR(n))
// Where:
// - SAR: Stop and Reverse point
// - AF: Acceleration Factor (starts at afStart, increases by afStep, max afMax)
// - EP: Extreme Point (highest high in uptrend, lowest low in downtrend)
//
// Default parameters:
// - afStart: 0.02
// - afStep: 0.02
// - afMax: 0.20
//
// Properties:
// - Provides dynamic stop-loss levels
// - Trend reversal signals
// - Works best in trending markets
// - Can whipsaw in ranging markets
type ParabolicSAR struct {
	*BaseIndicator
	afStart     float64
	afStep      float64
	afMax       float64
	sar         float64
	ep          float64
	af          float64
	isUptrend   bool
	prevHigh    float64
	prevLow     float64
	initialized bool
}

// NewParabolicSAR creates a new Parabolic SAR indicator
func NewParabolicSAR(afStart, afStep, afMax float64, maxHistory int) *ParabolicSAR {
	if afStart <= 0 {
		afStart = 0.02
	}
	if afStep <= 0 {
		afStep = 0.02
	}
	if afMax <= 0 {
		afMax = 0.20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &ParabolicSAR{
		BaseIndicator: NewBaseIndicator("Parabolic SAR", maxHistory),
		afStart:       afStart,
		afStep:        afStep,
		afMax:         afMax,
	}
}

// NewParabolicSARFromConfig creates Parabolic SAR from configuration
func NewParabolicSARFromConfig(config map[string]interface{}) (Indicator, error) {
	afStart := 0.02
	afStep := 0.02
	afMax := 0.20
	maxHistory := 1000

	if v, ok := config["af_start"]; ok {
		if a, ok := v.(float64); ok {
			afStart = a
		}
	}

	if v, ok := config["af_step"]; ok {
		if a, ok := v.(float64); ok {
			afStep = a
		}
	}

	if v, ok := config["af_max"]; ok {
		if a, ok := v.(float64); ok {
			afMax = a
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewParabolicSAR(afStart, afStep, afMax, maxHistory), nil
}

// Update updates the indicator with new market data
func (p *ParabolicSAR) Update(md *mdpb.MarketDataUpdate) {
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

	if !p.initialized {
		// Initialize: assume uptrend, SAR at the low
		p.sar = low
		p.ep = high
		p.af = p.afStart
		p.isUptrend = true
		p.prevHigh = high
		p.prevLow = low
		p.initialized = true
		p.AddValue(p.sar)
		return
	}

	// Calculate new SAR
	// SAR(n+1) = SAR(n) + AF × (EP - SAR(n))
	newSAR := p.sar + p.af*(p.ep-p.sar)

	// Check for trend reversal
	if p.isUptrend {
		// In uptrend, SAR should not be above prior two lows
		if newSAR > low {
			// Trend reversal to downtrend
			p.isUptrend = false
			p.sar = p.ep // Set SAR to previous EP (highest high)
			p.ep = low   // New EP is current low
			p.af = p.afStart
		} else {
			// Continue uptrend
			// SAR should not exceed prior two lows
			if newSAR > p.prevLow {
				newSAR = p.prevLow
			}
			p.sar = newSAR

			// Update EP if new high
			if high > p.ep {
				p.ep = high
				// Increase AF
				p.af += p.afStep
				if p.af > p.afMax {
					p.af = p.afMax
				}
			}
		}
	} else {
		// In downtrend, SAR should not be below prior two highs
		if newSAR < high {
			// Trend reversal to uptrend
			p.isUptrend = true
			p.sar = p.ep // Set SAR to previous EP (lowest low)
			p.ep = high  // New EP is current high
			p.af = p.afStart
		} else {
			// Continue downtrend
			// SAR should not be below prior two highs
			if newSAR < p.prevHigh {
				newSAR = p.prevHigh
			}
			p.sar = newSAR

			// Update EP if new low
			if low < p.ep {
				p.ep = low
				// Increase AF
				p.af += p.afStep
				if p.af > p.afMax {
					p.af = p.afMax
				}
			}
		}
	}

	// Update previous values
	p.prevHigh = high
	p.prevLow = low

	p.AddValue(p.sar)
}

// GetValue returns the current SAR value
func (p *ParabolicSAR) GetValue() float64 {
	return p.sar
}

// Reset resets the indicator
func (p *ParabolicSAR) Reset() {
	p.BaseIndicator.Reset()
	p.sar = 0
	p.ep = 0
	p.af = 0
	p.isUptrend = false
	p.prevHigh = 0
	p.prevLow = 0
	p.initialized = false
}

// IsReady returns true if the indicator has enough data
func (p *ParabolicSAR) IsReady() bool {
	return p.initialized
}

// IsUptrend returns true if currently in uptrend
func (p *ParabolicSAR) IsUptrend() bool {
	return p.isUptrend
}

// IsDowntrend returns true if currently in downtrend
func (p *ParabolicSAR) IsDowntrend() bool {
	return !p.isUptrend
}

// GetEP returns the current Extreme Point
func (p *ParabolicSAR) GetEP() float64 {
	return p.ep
}

// GetAF returns the current Acceleration Factor
func (p *ParabolicSAR) GetAF() float64 {
	return p.af
}
