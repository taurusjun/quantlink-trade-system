package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// AvgSpread calculates the rolling average of bid-ask spread
// Spread = AskPrice[0] - BidPrice[0]
type AvgSpread struct {
	*BaseIndicator
	period    int
	spreads   []float64
	sum       float64
	avgValue  float64
	spreadType string // "absolute", "percentage", or "bps" (basis points)
}

// NewAvgSpread creates a new AvgSpread indicator
// spreadType: "absolute" (default), "percentage" (spread/mid*100), or "bps" (spread/mid*10000)
func NewAvgSpread(period int, spreadType string, maxHistory int) *AvgSpread {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}
	if spreadType == "" {
		spreadType = "absolute"
	}

	return &AvgSpread{
		BaseIndicator: NewBaseIndicator("AvgSpread", maxHistory),
		period:        period,
		spreads:       make([]float64, 0, period),
		spreadType:    spreadType,
	}
}

// NewAvgSpreadFromConfig creates AvgSpread from configuration
func NewAvgSpreadFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 20
	spreadType := "absolute"
	maxHistory := 1000

	if v, ok := config["period"]; ok {
		if p, ok := v.(float64); ok {
			period = int(p)
		}
	}

	if v, ok := config["spread_type"]; ok {
		if s, ok := v.(string); ok {
			spreadType = s
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewAvgSpread(period, spreadType, maxHistory), nil
}

// Update calculates the spread and updates the moving average
func (a *AvgSpread) Update(md *mdpb.MarketDataUpdate) {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return
	}

	bid := md.BidPrice[0]
	ask := md.AskPrice[0]

	if bid <= 0 || ask <= 0 || ask <= bid {
		return
	}

	var spread float64

	switch a.spreadType {
	case "percentage":
		// Spread as percentage of mid price
		mid := (bid + ask) / 2.0
		spread = ((ask - bid) / mid) * 100.0

	case "bps":
		// Spread in basis points (1 bps = 0.01%)
		mid := (bid + ask) / 2.0
		spread = ((ask - bid) / mid) * 10000.0

	default: // "absolute"
		spread = ask - bid
	}

	a.sum += spread
	a.spreads = append(a.spreads, spread)

	if len(a.spreads) > a.period {
		oldest := a.spreads[0]
		a.spreads = a.spreads[1:]
		a.sum -= oldest
	}

	if len(a.spreads) > 0 {
		a.avgValue = a.sum / float64(len(a.spreads))
		a.AddValue(a.avgValue)
	}
}

// GetValue returns the current average spread
func (a *AvgSpread) GetValue() float64 {
	return a.avgValue
}

// GetSpreadType returns the spread calculation type
func (a *AvgSpread) GetSpreadType() string {
	return a.spreadType
}

// GetPeriod returns the averaging period
func (a *AvgSpread) GetPeriod() int {
	return a.period
}

// Reset resets the indicator
func (a *AvgSpread) Reset() {
	a.BaseIndicator.Reset()
	a.spreads = a.spreads[:0]
	a.sum = 0
	a.avgValue = 0
}

// IsReady returns true if we have at least one full period
func (a *AvgSpread) IsReady() bool {
	return len(a.spreads) >= a.period
}

// GetCurrentSpread returns the most recent spread value (before averaging)
func (a *AvgSpread) GetCurrentSpread() float64 {
	if len(a.spreads) == 0 {
		return 0.0
	}
	return a.spreads[len(a.spreads)-1]
}
