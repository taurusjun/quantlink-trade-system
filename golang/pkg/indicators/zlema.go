package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// ZLEMA (Zero Lag Exponential Moving Average) is a modified EMA that reduces
// lag by using price momentum to adjust the EMA calculation.
//
// The key innovation of ZLEMA is that it eliminates the inherent lag in EMA
// by subtracting a lagged data point, creating a "zero lag" effect.
//
// Formula:
// 1. Lag = (Period - 1) / 2
// 2. EMA_Data = Price + (Price - Price[Lag])
// 3. ZLEMA = EMA(EMA_Data, Period)
//
// The formula essentially adds momentum to the EMA by considering the
// difference between current price and the lagged price.
//
// Properties:
// - Much more responsive than traditional EMA
// - Minimal lag
// - Good for fast-moving markets
// - May generate more signals (including false ones)
// - Better trend detection in trending markets
type ZLEMA struct {
	*BaseIndicator
	period  int
	lag     int
	prices  []float64
	ema     *EMA
	zlema   float64
}

// NewZLEMA creates a new ZLEMA indicator
func NewZLEMA(period int, maxHistory int) *ZLEMA {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	// Calculate lag
	lag := (period - 1) / 2

	return &ZLEMA{
		BaseIndicator: NewBaseIndicator("ZLEMA", maxHistory),
		period:        period,
		lag:           lag,
		prices:        make([]float64, 0, lag+1),
		ema:           NewEMA(period, maxHistory),
	}
}

// NewZLEMAFromConfig creates ZLEMA from configuration
func NewZLEMAFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 20
	maxHistory := 1000

	if v, ok := config["period"]; ok {
		if p, ok := v.(float64); ok {
			period = int(p)
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewZLEMA(period, maxHistory), nil
}

// Update updates the indicator with new market data
func (z *ZLEMA) Update(md *mdpb.MarketDataUpdate) {
	price := GetMidPrice(md)
	if price <= 0 {
		return
	}

	// Store price
	z.prices = append(z.prices, price)

	// Keep only lag+1 values
	if len(z.prices) > z.lag+1 {
		z.prices = z.prices[1:]
	}

	// Need at least lag+1 values to calculate ZLEMA
	if len(z.prices) < z.lag+1 {
		return
	}

	// Calculate EMA_Data = Price + (Price - Price[Lag])
	laggedPrice := z.prices[0]
	emaData := price + (price - laggedPrice)

	// Update EMA with adjusted data
	mdAdjusted := &mdpb.MarketDataUpdate{
		BidPrice: []float64{emaData},
		AskPrice: []float64{emaData},
	}
	z.ema.Update(mdAdjusted)

	if z.ema.IsReady() {
		z.zlema = z.ema.GetValue()
		z.AddValue(z.zlema)
	}
}

// GetValue returns the current ZLEMA value
func (z *ZLEMA) GetValue() float64 {
	return z.zlema
}

// Reset resets the indicator
func (z *ZLEMA) Reset() {
	z.BaseIndicator.Reset()
	z.prices = z.prices[:0]
	z.ema.Reset()
	z.zlema = 0
}

// IsReady returns true if the indicator has enough data
func (z *ZLEMA) IsReady() bool {
	return z.ema.IsReady() && len(z.prices) >= z.lag+1
}

// GetPeriod returns the period
func (z *ZLEMA) GetPeriod() int {
	return z.period
}

// GetLag returns the lag
func (z *ZLEMA) GetLag() int {
	return z.lag
}

// GetTrend returns the trend direction
// Returns 1 for uptrend, -1 for downtrend, 0 for neutral
func (z *ZLEMA) GetTrend() int {
	if len(z.values) < 2 {
		return 0
	}

	current := z.values[len(z.values)-1]
	previous := z.values[len(z.values)-2]

	if current > previous {
		return 1
	} else if current < previous {
		return -1
	}

	return 0
}

// GetSlope returns the recent slope (last 3 periods)
// Positive slope indicates uptrend, negative indicates downtrend
func (z *ZLEMA) GetSlope() float64 {
	if len(z.values) < 3 {
		return 0
	}

	n := len(z.values)
	return (z.values[n-1] - z.values[n-3]) / 2.0
}

// IsCrossAbove checks if price crossed above ZLEMA
// Requires price history
func (z *ZLEMA) IsCrossAbove(currentPrice float64, prevPrice float64) bool {
	if len(z.values) < 2 {
		return false
	}

	prevZLEMA := z.values[len(z.values)-2]
	return prevPrice <= prevZLEMA && currentPrice > z.zlema
}

// IsCrossBelow checks if price crossed below ZLEMA
// Requires price history
func (z *ZLEMA) IsCrossBelow(currentPrice float64, prevPrice float64) bool {
	if len(z.values) < 2 {
		return false
	}

	prevZLEMA := z.values[len(z.values)-2]
	return prevPrice >= prevZLEMA && currentPrice < z.zlema
}
