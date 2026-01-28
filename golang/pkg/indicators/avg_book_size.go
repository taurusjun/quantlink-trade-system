package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// AvgBookSize calculates the rolling average of total orderbook depth
// BookSize = sum of all bid quantities + sum of all ask quantities
type AvgBookSize struct {
	*BaseIndicator
	period      int
	bookSizes   []float64
	sum         float64
	avgValue    float64
	numLevels   int // number of orderbook levels to consider
}

// NewAvgBookSize creates a new AvgBookSize indicator
// period: window size for moving average
// numLevels: number of orderbook levels to include (0 = all levels)
func NewAvgBookSize(period int, numLevels int, maxHistory int) *AvgBookSize {
	if period <= 0 {
		period = 20 // default 20-period average
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}
	if numLevels < 0 {
		numLevels = 0 // 0 means all levels
	}

	return &AvgBookSize{
		BaseIndicator: NewBaseIndicator("AvgBookSize", maxHistory),
		period:        period,
		bookSizes:     make([]float64, 0, period),
		numLevels:     numLevels,
	}
}

// NewAvgBookSizeFromConfig creates AvgBookSize from configuration
func NewAvgBookSizeFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 20
	numLevels := 5 // default to top 5 levels
	maxHistory := 1000

	if v, ok := config["period"]; ok {
		if p, ok := v.(float64); ok {
			period = int(p)
		}
	}

	if v, ok := config["num_levels"]; ok {
		if n, ok := v.(float64); ok {
			numLevels = int(n)
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewAvgBookSize(period, numLevels, maxHistory), nil
}

// Update calculates the total book size and updates the moving average
func (a *AvgBookSize) Update(md *mdpb.MarketDataUpdate) {
	// Calculate total book size
	totalBidQty := 0.0
	totalAskQty := 0.0

	// Determine how many levels to use
	bidLevels := len(md.BidQty)
	askLevels := len(md.AskQty)

	if a.numLevels > 0 {
		if bidLevels > a.numLevels {
			bidLevels = a.numLevels
		}
		if askLevels > a.numLevels {
			askLevels = a.numLevels
		}
	}

	// Sum bid quantities
	for i := 0; i < bidLevels; i++ {
		totalBidQty += float64(md.BidQty[i])
	}

	// Sum ask quantities
	for i := 0; i < askLevels; i++ {
		totalAskQty += float64(md.AskQty[i])
	}

	bookSize := totalBidQty + totalAskQty

	// Update rolling average
	a.sum += bookSize
	a.bookSizes = append(a.bookSizes, bookSize)

	if len(a.bookSizes) > a.period {
		// Remove oldest value from sum
		oldest := a.bookSizes[0]
		a.bookSizes = a.bookSizes[1:]
		a.sum -= oldest
	}

	// Calculate average
	if len(a.bookSizes) > 0 {
		a.avgValue = a.sum / float64(len(a.bookSizes))
		a.AddValue(a.avgValue)
	}
}

// GetValue returns the current average book size
func (a *AvgBookSize) GetValue() float64 {
	return a.avgValue
}

// GetPeriod returns the averaging period
func (a *AvgBookSize) GetPeriod() int {
	return a.period
}

// GetNumLevels returns the number of levels being considered
func (a *AvgBookSize) GetNumLevels() int {
	return a.numLevels
}

// Reset resets the indicator
func (a *AvgBookSize) Reset() {
	a.BaseIndicator.Reset()
	a.bookSizes = a.bookSizes[:0]
	a.sum = 0
	a.avgValue = 0
}

// IsReady returns true if we have at least one full period
func (a *AvgBookSize) IsReady() bool {
	return len(a.bookSizes) >= a.period
}
