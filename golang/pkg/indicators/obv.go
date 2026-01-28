package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// OBV (On-Balance Volume) is a momentum indicator that uses volume flow to predict
// changes in stock price. It is based on the premise that volume precedes price.
//
// Calculation:
// - If close > prev_close: OBV = OBV_prev + volume
// - If close < prev_close: OBV = OBV_prev - volume
// - If close = prev_close: OBV = OBV_prev (unchanged)
//
// Interpretation:
// - Rising OBV indicates accumulation (buying pressure)
// - Falling OBV indicates distribution (selling pressure)
// - OBV divergence from price can signal reversals
//
// Properties:
// - Simple cumulative volume indicator
// - Confirms price trends with volume
// - Leading indicator for breakouts
// - Works best with trending markets
type OBV struct {
	*BaseIndicator
	obv       float64
	prevClose float64
}

// NewOBV creates a new OBV indicator
func NewOBV(maxHistory int) *OBV {
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &OBV{
		BaseIndicator: NewBaseIndicator("OBV", maxHistory),
	}
}

// NewOBVFromConfig creates OBV from configuration
func NewOBVFromConfig(config map[string]interface{}) (Indicator, error) {
	maxHistory := 1000

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewOBV(maxHistory), nil
}

// Update updates the indicator with new market data
func (o *OBV) Update(md *mdpb.MarketDataUpdate) {
	close := GetMidPrice(md)
	if close <= 0 {
		return
	}

	// Get volume (use TotalVolume or calculate from bid/ask volumes)
	volume := float64(md.TotalVolume)
	if volume == 0 && len(md.BidQty) > 0 && len(md.AskQty) > 0 {
		// If TotalVolume not available, use bid/ask volume
		volume = float64(md.BidQty[0] + md.AskQty[0])
	}

	// First update: initialize with zero OBV
	if o.prevClose == 0 {
		o.prevClose = close
		o.obv = 0
		o.AddValue(o.obv)
		return
	}

	// Calculate OBV based on price direction
	if close > o.prevClose {
		// Price up: add volume
		o.obv += volume
	} else if close < o.prevClose {
		// Price down: subtract volume
		o.obv -= volume
	}
	// If close == prevClose, OBV remains unchanged

	o.prevClose = close
	o.AddValue(o.obv)
}

// GetValue returns the current OBV value
func (o *OBV) GetValue() float64 {
	return o.obv
}

// Reset resets the indicator
func (o *OBV) Reset() {
	o.BaseIndicator.Reset()
	o.obv = 0
	o.prevClose = 0
}

// IsReady returns true if the indicator has enough data
func (o *OBV) IsReady() bool {
	return o.prevClose > 0 && len(o.values) > 0
}

// GetTrend returns the trend direction based on OBV slope
// Returns 1 for uptrend, -1 for downtrend, 0 for neutral
func (o *OBV) GetTrend() int {
	if len(o.values) < 2 {
		return 0
	}

	current := o.values[len(o.values)-1]
	previous := o.values[len(o.values)-2]

	if current > previous {
		return 1 // Uptrend
	} else if current < previous {
		return -1 // Downtrend
	}

	return 0 // Neutral
}

// IsDivergence checks for bullish or bearish divergence
// Requires at least 5 data points
// Returns: 1 for bullish divergence, -1 for bearish divergence, 0 for no divergence
func (o *OBV) IsDivergence(prices []float64) int {
	if len(o.values) < 5 || len(prices) < 5 {
		return 0
	}

	// Get recent OBV values
	n := len(o.values)
	obvRecent := o.values[n-5:]
	priceRecent := prices[len(prices)-5:]

	// Calculate trends (simple slope check)
	obvTrend := obvRecent[4] - obvRecent[0]
	priceTrend := priceRecent[4] - priceRecent[0]

	// Bullish divergence: price falling but OBV rising
	if priceTrend < 0 && obvTrend > 0 {
		return 1
	}

	// Bearish divergence: price rising but OBV falling
	if priceTrend > 0 && obvTrend < 0 {
		return -1
	}

	return 0
}
