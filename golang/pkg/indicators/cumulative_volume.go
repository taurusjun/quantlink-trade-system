package indicators

import (
	"fmt"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// CumulativeVolume tracks the cumulative trading volume over time
// 累计成交量：跟踪一段时间内的累计成交量
type CumulativeVolume struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	resetOnNewBar bool // Whether to reset on each new time bar

	// Current state
	cumulativeVolume int64 // Total cumulative volume
	lastVolume      int64 // Last recorded total volume
}

// NewCumulativeVolume creates a new CumulativeVolume indicator
func NewCumulativeVolume(name string, resetOnNewBar bool, maxHistory int) *CumulativeVolume {
	cv := &CumulativeVolume{
		BaseIndicator:  NewBaseIndicator(name, maxHistory),
		resetOnNewBar:  resetOnNewBar,
	}

	return cv
}

// NewCumulativeVolumeFromConfig creates a CumulativeVolume indicator from configuration
func NewCumulativeVolumeFromConfig(config map[string]interface{}) (Indicator, error) {
	name := "CumulativeVolume"
	if v, ok := config["name"]; ok {
		if sv, ok := v.(string); ok {
			name = sv
		}
	}

	resetOnNewBar := false
	if v, ok := config["reset_on_new_bar"]; ok {
		if bv, ok := v.(bool); ok {
			resetOnNewBar = bv
		}
	}

	maxHistory := 1000
	if v, ok := config["max_history"]; ok {
		if fv, ok := v.(float64); ok {
			maxHistory = int(fv)
		}
	}

	return NewCumulativeVolume(name, resetOnNewBar, maxHistory), nil
}

// Update updates the indicator with new market data
func (cv *CumulativeVolume) Update(md *mdpb.MarketDataUpdate) {
	cv.mu.Lock()
	defer cv.mu.Unlock()

	currentVolume := int64(md.TotalVolume)

	// Calculate volume delta
	volumeDelta := int64(0)
	if cv.lastVolume > 0 {
		volumeDelta = currentVolume - cv.lastVolume
		if volumeDelta < 0 {
			// Handle overnight reset or data issues
			volumeDelta = currentVolume
		}
	} else {
		// First update
		volumeDelta = currentVolume
	}

	cv.lastVolume = currentVolume
	cv.cumulativeVolume += volumeDelta

	cv.AddValue(float64(cv.cumulativeVolume))
}

// GetValue returns the current cumulative volume
func (cv *CumulativeVolume) GetValue() float64 {
	cv.mu.RLock()
	defer cv.mu.RUnlock()
	return float64(cv.cumulativeVolume)
}

// Reset resets the cumulative volume to zero
func (cv *CumulativeVolume) Reset() {
	cv.mu.Lock()
	defer cv.mu.Unlock()
	cv.cumulativeVolume = 0
	cv.lastVolume = 0
}

// GetCumulativeVolume returns the current cumulative volume
func (cv *CumulativeVolume) GetCumulativeVolume() int64 {
	cv.mu.RLock()
	defer cv.mu.RUnlock()
	return cv.cumulativeVolume
}

// String returns a string representation of the indicator
func (cv *CumulativeVolume) String() string {
	return fmt.Sprintf("CumulativeVolume(volume=%d)", cv.GetCumulativeVolume())
}
