package indicators

import (
	"fmt"
	"sync"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// TradeIntensity measures the trading activity level (trades per unit time)
// 交易强度：衡量单位时间内的交易活跃程度
//
// Intensity = Number of trades / Time window
// Higher intensity = more active trading
type TradeIntensity struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	windowDuration time.Duration // Time window (e.g., 1 minute)
	useVolume      bool          // Use volume instead of trade count

	// Tracking
	tradeTimestamps []time.Time // Recent trade timestamps
	tradeVolumes    []float64   // Recent trade volumes (if useVolume=true)

	// Current state
	intensity float64 // Current trading intensity
}

// NewTradeIntensity creates a new Trade Intensity indicator
func NewTradeIntensity(name string, windowDuration time.Duration, useVolume bool, maxHistory int) *TradeIntensity {
	if windowDuration <= 0 {
		windowDuration = 1 * time.Minute // Default 1 minute window
	}

	ti := &TradeIntensity{
		BaseIndicator:   NewBaseIndicator(name, maxHistory),
		windowDuration:  windowDuration,
		useVolume:       useVolume,
		tradeTimestamps: make([]time.Time, 0, 1000),
		tradeVolumes:    make([]float64, 0, 1000),
	}

	return ti
}

// NewTradeIntensityFromConfig creates a TradeIntensity from configuration
func NewTradeIntensityFromConfig(config map[string]interface{}) (Indicator, error) {
	name := "TradeIntensity"
	if v, ok := config["name"]; ok {
		if sv, ok := v.(string); ok {
			name = sv
		}
	}

	// Default 1 minute window
	windowSeconds := 60.0
	if v, ok := config["window_seconds"]; ok {
		if fv, ok := v.(float64); ok {
			windowSeconds = fv
		}
	}
	windowDuration := time.Duration(windowSeconds) * time.Second

	useVolume := false
	if v, ok := config["use_volume"]; ok {
		if bv, ok := v.(bool); ok {
			useVolume = bv
		}
	}

	maxHistory := 1000
	if v, ok := config["max_history"]; ok {
		if fv, ok := v.(float64); ok {
			maxHistory = int(fv)
		}
	}

	return NewTradeIntensity(name, windowDuration, useVolume, maxHistory), nil
}

// Update updates the trade intensity
func (ti *TradeIntensity) Update(md *mdpb.MarketDataUpdate) {
	ti.mu.Lock()
	defer ti.mu.Unlock()

	now := time.Now()

	// Add current trade
	ti.tradeTimestamps = append(ti.tradeTimestamps, now)
	if ti.useVolume {
		ti.tradeVolumes = append(ti.tradeVolumes, float64(md.LastQty))
	}

	// Remove trades outside the window
	cutoffTime := now.Add(-ti.windowDuration)
	ti.removeOldTrades(cutoffTime)

	// Calculate intensity
	if ti.useVolume {
		// Volume-based intensity (volume per second)
		totalVolume := 0.0
		for _, vol := range ti.tradeVolumes {
			totalVolume += vol
		}
		ti.intensity = totalVolume / ti.windowDuration.Seconds()
	} else {
		// Count-based intensity (trades per second)
		ti.intensity = float64(len(ti.tradeTimestamps)) / ti.windowDuration.Seconds()
	}

	ti.AddValue(ti.intensity)
}

// removeOldTrades removes trades outside the time window
func (ti *TradeIntensity) removeOldTrades(cutoffTime time.Time) {
	// Find first index to keep
	firstKeep := 0
	for i, ts := range ti.tradeTimestamps {
		if ts.After(cutoffTime) {
			firstKeep = i
			break
		}
	}

	// Remove old trades
	if firstKeep > 0 {
		ti.tradeTimestamps = ti.tradeTimestamps[firstKeep:]
		if ti.useVolume {
			ti.tradeVolumes = ti.tradeVolumes[firstKeep:]
		}
	}
}

// GetIntensity returns current trading intensity
func (ti *TradeIntensity) GetIntensity() float64 {
	ti.mu.RLock()
	defer ti.mu.RUnlock()
	return ti.intensity
}

// GetTradeCount returns number of trades in window
func (ti *TradeIntensity) GetTradeCount() int {
	ti.mu.RLock()
	defer ti.mu.RUnlock()
	return len(ti.tradeTimestamps)
}

// GetIntensityLevel returns intensity level classification
func (ti *TradeIntensity) GetIntensityLevel() string {
	intensity := ti.GetIntensity()

	if ti.useVolume {
		// Volume-based classification
		if intensity > 100 {
			return "VeryHigh"
		} else if intensity > 50 {
			return "High"
		} else if intensity > 20 {
			return "Medium"
		} else if intensity > 5 {
			return "Low"
		}
		return "VeryLow"
	} else {
		// Count-based classification
		if intensity > 10 {
			return "VeryHigh" // >10 trades/sec
		} else if intensity > 5 {
			return "High" // 5-10 trades/sec
		} else if intensity > 2 {
			return "Medium" // 2-5 trades/sec
		} else if intensity > 0.5 {
			return "Low" // 0.5-2 trades/sec
		}
		return "VeryLow" // <0.5 trades/sec
	}
}

// GetName returns indicator name
func (ti *TradeIntensity) GetName() string {
	return ti.BaseIndicator.GetName()
}

// String returns a string representation
func (ti *TradeIntensity) String() string {
	return fmt.Sprintf("TradeIntensity(window=%.0fs, intensity=%.2f, level=%s, trades=%d)",
		ti.windowDuration.Seconds(), ti.intensity, ti.GetIntensityLevel(), len(ti.tradeTimestamps))
}
