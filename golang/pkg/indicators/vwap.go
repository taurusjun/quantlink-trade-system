package indicators

import (
	"fmt"
	"time"

	mdpb "github.com/yourusername/hft-poc/pkg/proto/md"
)

// VWAP (Volume Weighted Average Price) calculates the volume-weighted average price
type VWAP struct {
	*BaseIndicator
	cumulativeVolume   float64
	cumulativeValue    float64
	resetDaily         bool
	lastResetTime      time.Time
	resetHour          int // Hour to reset (e.g., 9 for 9:00 AM)
}

// NewVWAP creates a new VWAP indicator
func NewVWAP(resetDaily bool, resetHour int, maxHistory int) *VWAP {
	return &VWAP{
		BaseIndicator:    NewBaseIndicator("VWAP", maxHistory),
		resetDaily:       resetDaily,
		resetHour:        resetHour,
		lastResetTime:    time.Time{},
	}
}

// NewVWAPFromConfig creates VWAP from configuration
func NewVWAPFromConfig(config map[string]interface{}) (Indicator, error) {
	resetDaily := true
	resetHour := 9
	maxHistory := 1000

	if v, ok := config["reset_daily"]; ok {
		if r, ok := v.(bool); ok {
			resetDaily = r
		}
	}

	if v, ok := config["reset_hour"]; ok {
		if h, ok := v.(float64); ok {
			resetHour = int(h)
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	if resetHour < 0 || resetHour > 23 {
		return nil, fmt.Errorf("%w: reset_hour must be between 0 and 23", ErrInvalidParameter)
	}

	return NewVWAP(resetDaily, resetHour, maxHistory), nil
}

// Update updates the VWAP with new market data
func (v *VWAP) Update(md *mdpb.MarketDataUpdate) {
	// Check if we need to reset
	now := time.Unix(0, int64(md.Timestamp))
	if v.resetDaily && v.shouldReset(now) {
		v.resetAccumulators()
		v.lastResetTime = now
	}

	// Use last price if available, otherwise use mid price
	price := md.LastPrice
	if price == 0 {
		price = GetMidPrice(md)
	}

	if price == 0 {
		return
	}

	// Volume is the incremental volume since last update
	// For simplicity, we use the current volume directly
	volume := float64(md.TotalVolume)
	if volume <= 0 {
		return
	}

	// Update cumulative values
	v.cumulativeValue += price * volume
	v.cumulativeVolume += volume

	// Calculate VWAP
	if v.cumulativeVolume > 0 {
		vwap := v.cumulativeValue / v.cumulativeVolume
		v.AddValue(vwap)
	}
}

// shouldReset checks if we should reset the VWAP
func (v *VWAP) shouldReset(now time.Time) bool {
	if v.lastResetTime.IsZero() {
		return true
	}

	// Reset if we've crossed the reset hour
	if now.Hour() >= v.resetHour && v.lastResetTime.Hour() < v.resetHour {
		return true
	}

	// Also reset if we've moved to a new day
	if now.Day() != v.lastResetTime.Day() {
		return true
	}

	return false
}

// resetAccumulators resets the cumulative values
func (v *VWAP) resetAccumulators() {
	v.cumulativeVolume = 0
	v.cumulativeValue = 0
}

// Reset resets the VWAP to initial state
func (v *VWAP) Reset() {
	v.BaseIndicator.Reset()
	v.resetAccumulators()
	v.lastResetTime = time.Time{}
}

// IsReady returns true if VWAP has been initialized
func (v *VWAP) IsReady() bool {
	return v.cumulativeVolume > 0
}

// GetCumulativeVolume returns the cumulative volume
func (v *VWAP) GetCumulativeVolume() float64 {
	return v.cumulativeVolume
}

// TimeWeightedVWAP (TWVWAP) calculates time-weighted VWAP
type TimeWeightedVWAP struct {
	*BaseIndicator
	window            time.Duration
	priceVolumePairs  []priceVolumePair
	resetDaily        bool
	lastResetTime     time.Time
	resetHour         int
}

type priceVolumePair struct {
	price     float64
	volume    float64
	timestamp time.Time
}

// NewTimeWeightedVWAP creates a new time-weighted VWAP indicator
func NewTimeWeightedVWAP(window time.Duration, resetDaily bool, resetHour int, maxHistory int) *TimeWeightedVWAP {
	return &TimeWeightedVWAP{
		BaseIndicator:    NewBaseIndicator("TWVWAP", maxHistory),
		window:           window,
		priceVolumePairs: make([]priceVolumePair, 0, 1000),
		resetDaily:       resetDaily,
		resetHour:        resetHour,
	}
}

// Update updates the time-weighted VWAP
func (tw *TimeWeightedVWAP) Update(md *mdpb.MarketDataUpdate) {
	now := time.Unix(0, int64(md.Timestamp))

	// Check if we need to reset
	if tw.resetDaily && tw.shouldReset(now) {
		tw.priceVolumePairs = tw.priceVolumePairs[:0]
		tw.lastResetTime = now
	}

	price := md.LastPrice
	if price == 0 {
		price = GetMidPrice(md)
	}

	if price == 0 {
		return
	}

	volume := float64(md.TotalVolume)
	if volume <= 0 {
		volume = 1.0 // Use 1 for tick updates without volume
	}

	// Add new price-volume pair
	tw.priceVolumePairs = append(tw.priceVolumePairs, priceVolumePair{
		price:     price,
		volume:    volume,
		timestamp: now,
	})

	// Remove old pairs outside the window
	cutoff := now.Add(-tw.window)
	validStart := 0
	for i, pair := range tw.priceVolumePairs {
		if pair.timestamp.After(cutoff) {
			validStart = i
			break
		}
	}
	if validStart > 0 {
		tw.priceVolumePairs = tw.priceVolumePairs[validStart:]
	}

	// Calculate TWVWAP
	if len(tw.priceVolumePairs) > 0 {
		var sumValue, sumVolume float64
		for _, pair := range tw.priceVolumePairs {
			sumValue += pair.price * pair.volume
			sumVolume += pair.volume
		}

		if sumVolume > 0 {
			twvwap := sumValue / sumVolume
			tw.AddValue(twvwap)
		}
	}
}

// shouldReset checks if we should reset
func (tw *TimeWeightedVWAP) shouldReset(now time.Time) bool {
	if tw.lastResetTime.IsZero() {
		return true
	}

	if now.Hour() >= tw.resetHour && tw.lastResetTime.Hour() < tw.resetHour {
		return true
	}

	if now.Day() != tw.lastResetTime.Day() {
		return true
	}

	return false
}

// Reset resets the indicator
func (tw *TimeWeightedVWAP) Reset() {
	tw.BaseIndicator.Reset()
	tw.priceVolumePairs = tw.priceVolumePairs[:0]
	tw.lastResetTime = time.Time{}
}

// IsReady returns true if there's data
func (tw *TimeWeightedVWAP) IsReady() bool {
	return len(tw.priceVolumePairs) > 0
}
