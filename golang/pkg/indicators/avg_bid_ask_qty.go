package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// AvgBidQty calculates the rolling average of total bid quantities
type AvgBidQty struct {
	*BaseIndicator
	period    int
	bidQties  []float64
	sum       float64
	avgValue  float64
	numLevels int // number of orderbook levels to consider
}

// NewAvgBidQty creates a new AvgBidQty indicator
func NewAvgBidQty(period int, numLevels int, maxHistory int) *AvgBidQty {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}
	if numLevels < 0 {
		numLevels = 0
	}

	return &AvgBidQty{
		BaseIndicator: NewBaseIndicator("AvgBidQty", maxHistory),
		period:        period,
		bidQties:      make([]float64, 0, period),
		numLevels:     numLevels,
	}
}

// NewAvgBidQtyFromConfig creates AvgBidQty from configuration
func NewAvgBidQtyFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 20
	numLevels := 5
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

	return NewAvgBidQty(period, numLevels, maxHistory), nil
}

// Update calculates the total bid quantity and updates the moving average
func (a *AvgBidQty) Update(md *mdpb.MarketDataUpdate) {
	totalBidQty := 0.0

	bidLevels := len(md.BidQty)
	if a.numLevels > 0 && bidLevels > a.numLevels {
		bidLevels = a.numLevels
	}

	for i := 0; i < bidLevels; i++ {
		totalBidQty += float64(md.BidQty[i])
	}

	a.sum += totalBidQty
	a.bidQties = append(a.bidQties, totalBidQty)

	if len(a.bidQties) > a.period {
		oldest := a.bidQties[0]
		a.bidQties = a.bidQties[1:]
		a.sum -= oldest
	}

	if len(a.bidQties) > 0 {
		a.avgValue = a.sum / float64(len(a.bidQties))
		a.AddValue(a.avgValue)
	}
}

// GetValue returns the current average bid quantity
func (a *AvgBidQty) GetValue() float64 {
	return a.avgValue
}

// Reset resets the indicator
func (a *AvgBidQty) Reset() {
	a.BaseIndicator.Reset()
	a.bidQties = a.bidQties[:0]
	a.sum = 0
	a.avgValue = 0
}

// IsReady returns true if we have at least one full period
func (a *AvgBidQty) IsReady() bool {
	return len(a.bidQties) >= a.period
}

// AvgAskQty calculates the rolling average of total ask quantities
type AvgAskQty struct {
	*BaseIndicator
	period    int
	askQties  []float64
	sum       float64
	avgValue  float64
	numLevels int
}

// NewAvgAskQty creates a new AvgAskQty indicator
func NewAvgAskQty(period int, numLevels int, maxHistory int) *AvgAskQty {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}
	if numLevels < 0 {
		numLevels = 0
	}

	return &AvgAskQty{
		BaseIndicator: NewBaseIndicator("AvgAskQty", maxHistory),
		period:        period,
		askQties:      make([]float64, 0, period),
		numLevels:     numLevels,
	}
}

// NewAvgAskQtyFromConfig creates AvgAskQty from configuration
func NewAvgAskQtyFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 20
	numLevels := 5
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

	return NewAvgAskQty(period, numLevels, maxHistory), nil
}

// Update calculates the total ask quantity and updates the moving average
func (a *AvgAskQty) Update(md *mdpb.MarketDataUpdate) {
	totalAskQty := 0.0

	askLevels := len(md.AskQty)
	if a.numLevels > 0 && askLevels > a.numLevels {
		askLevels = a.numLevels
	}

	for i := 0; i < askLevels; i++ {
		totalAskQty += float64(md.AskQty[i])
	}

	a.sum += totalAskQty
	a.askQties = append(a.askQties, totalAskQty)

	if len(a.askQties) > a.period {
		oldest := a.askQties[0]
		a.askQties = a.askQties[1:]
		a.sum -= oldest
	}

	if len(a.askQties) > 0 {
		a.avgValue = a.sum / float64(len(a.askQties))
		a.AddValue(a.avgValue)
	}
}

// GetValue returns the current average ask quantity
func (a *AvgAskQty) GetValue() float64 {
	return a.avgValue
}

// Reset resets the indicator
func (a *AvgAskQty) Reset() {
	a.BaseIndicator.Reset()
	a.askQties = a.askQties[:0]
	a.sum = 0
	a.avgValue = 0
}

// IsReady returns true if we have at least one full period
func (a *AvgAskQty) IsReady() bool {
	return len(a.askQties) >= a.period
}
