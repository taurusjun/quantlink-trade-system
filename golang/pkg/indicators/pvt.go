package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// PVT (Price Volume Trend) is a momentum indicator that combines price and volume
// to identify the strength of price trends. It is an improved version of OBV that
// considers the magnitude of price changes, not just direction.
//
// Calculation:
// PVT = PVT_prev + (Volume × ((Close - Close_prev) / Close_prev))
//
// Interpretation:
// - Rising PVT confirms uptrend
// - Falling PVT confirms downtrend
// - PVT divergence from price can signal reversals
// - More sensitive than OBV to price magnitude
//
// Properties:
// - Cumulative volume indicator
// - Weighted by percentage price change
// - Good for confirming trends
// - Less prone to false signals than OBV
// - Works well with trending markets
type PVT struct {
	*BaseIndicator
	pvt       float64
	prevClose float64
}

// NewPVT creates a new PVT indicator
func NewPVT(maxHistory int) *PVT {
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &PVT{
		BaseIndicator: NewBaseIndicator("PVT", maxHistory),
	}
}

// NewPVTFromConfig creates PVT from configuration
func NewPVTFromConfig(config map[string]interface{}) (Indicator, error) {
	maxHistory := 1000

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewPVT(maxHistory), nil
}

// Update updates the indicator with new market data
func (p *PVT) Update(md *mdpb.MarketDataUpdate) {
	close := GetMidPrice(md)
	if close <= 0 {
		return
	}

	// Get volume
	volume := float64(md.TotalVolume)
	if volume == 0 && len(md.BidQty) > 0 && len(md.AskQty) > 0 {
		volume = float64(md.BidQty[0] + md.AskQty[0])
	}

	// First update: initialize
	if p.prevClose == 0 {
		p.prevClose = close
		p.pvt = 0
		p.AddValue(p.pvt)
		return
	}

	// Calculate percentage price change
	priceChange := (close - p.prevClose) / p.prevClose

	// Update PVT: PVT = PVT_prev + (Volume × Price_Change_Percentage)
	p.pvt += volume * priceChange

	p.prevClose = close
	p.AddValue(p.pvt)
}

// GetValue returns the current PVT value
func (p *PVT) GetValue() float64 {
	return p.pvt
}

// Reset resets the indicator
func (p *PVT) Reset() {
	p.BaseIndicator.Reset()
	p.pvt = 0
	p.prevClose = 0
}

// IsReady returns true if the indicator has enough data
func (p *PVT) IsReady() bool {
	return p.prevClose > 0 && len(p.values) > 0
}

// GetTrend returns the trend direction based on PVT slope
// Returns 1 for uptrend, -1 for downtrend, 0 for neutral
func (p *PVT) GetTrend() int {
	if len(p.values) < 2 {
		return 0
	}

	current := p.values[len(p.values)-1]
	previous := p.values[len(p.values)-2]

	if current > previous {
		return 1 // Uptrend
	} else if current < previous {
		return -1 // Downtrend
	}

	return 0 // Neutral
}

// GetSlope returns the average slope over the last N periods
// Positive slope indicates accumulation, negative indicates distribution
func (p *PVT) GetSlope(periods int) float64 {
	if len(p.values) < periods {
		return 0
	}

	n := len(p.values)
	start := p.values[n-periods]
	end := p.values[n-1]

	// Average slope per period
	return (end - start) / float64(periods)
}

// IsDivergence checks for bullish or bearish divergence
// Returns: 1 for bullish divergence, -1 for bearish divergence, 0 for no divergence
func (p *PVT) IsDivergence(prices []float64, lookback int) int {
	if len(p.values) < lookback || len(prices) < lookback {
		return 0
	}

	// Get recent values
	n := len(p.values)
	pn := len(prices)

	pvtStart := p.values[n-lookback]
	pvtEnd := p.values[n-1]
	priceStart := prices[pn-lookback]
	priceEnd := prices[pn-1]

	// Calculate trends
	pvtTrend := pvtEnd - pvtStart
	priceTrend := priceEnd - priceStart

	// Bullish divergence: price falling but PVT rising
	if priceTrend < 0 && pvtTrend > 0 {
		return 1
	}

	// Bearish divergence: price rising but PVT falling
	if priceTrend > 0 && pvtTrend < 0 {
		return -1
	}

	return 0
}

// IsConfirmingTrend checks if PVT confirms the price trend
// Returns true if both price and PVT are moving in the same direction
func (p *PVT) IsConfirmingTrend(prices []float64, lookback int) bool {
	if len(p.values) < lookback || len(prices) < lookback {
		return false
	}

	n := len(p.values)
	pn := len(prices)

	pvtStart := p.values[n-lookback]
	pvtEnd := p.values[n-1]
	priceStart := prices[pn-lookback]
	priceEnd := prices[pn-1]

	pvtTrend := pvtEnd - pvtStart
	priceTrend := priceEnd - priceStart

	// Both moving in same direction
	return (pvtTrend > 0 && priceTrend > 0) || (pvtTrend < 0 && priceTrend < 0)
}

// GetStrength returns the trend strength based on PVT absolute change
// Higher values indicate stronger trends
func (p *PVT) GetStrength(periods int) float64 {
	if len(p.values) < periods {
		return 0
	}

	n := len(p.values)
	start := p.values[n-periods]
	end := p.values[n-1]

	// Absolute change normalized by periods
	return (end - start) / float64(periods)
}
