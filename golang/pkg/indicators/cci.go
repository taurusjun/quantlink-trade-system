package indicators

import (
	"math"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// CCI (Commodity Channel Index) is a versatile indicator that identifies cyclical trends
// and overbought/oversold conditions
//
// Formula:
// TP (Typical Price) = (High + Low + Close) / 3
// CCI = (TP - SMA(TP, period)) / (0.015 × Mean Deviation)
//
// Mean Deviation = Average of |TP - SMA(TP)|
//
// Range: Typically -100 to +100, but can exceed
// - Above +100: Overbought, price moving strongly upward
// - Below -100: Oversold, price moving strongly downward
// - Around 0: Price near average
//
// Properties:
// - Unbounded oscillator
// - Identifies trend strength and reversals
// - Works well in trending and ranging markets
// - Constant 0.015 ensures 70-80% of values fall between -100 and +100
type CCI struct {
	*BaseIndicator
	period            int
	typicalPrices     []float64
	cci               float64
	constant          float64 // Usually 0.015
}

// NewCCI creates a new CCI indicator
func NewCCI(period int, maxHistory int) *CCI {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &CCI{
		BaseIndicator: NewBaseIndicator("CCI", maxHistory),
		period:        period,
		typicalPrices: make([]float64, 0, period),
		constant:      0.015,
	}
}

// NewCCIFromConfig creates CCI from configuration
func NewCCIFromConfig(config map[string]interface{}) (Indicator, error) {
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

	return NewCCI(period, maxHistory), nil
}

// Update updates the indicator with new market data
func (c *CCI) Update(md *mdpb.MarketDataUpdate) {
	// Calculate typical price: (High + Low + Close) / 3
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

	typicalPrice := (high + low + close) / 3.0

	// Add to window
	c.typicalPrices = append(c.typicalPrices, typicalPrice)

	// Keep only period elements
	if len(c.typicalPrices) > c.period {
		c.typicalPrices = c.typicalPrices[1:]
	}

	// Need at least period values
	if len(c.typicalPrices) < c.period {
		return
	}

	// Calculate SMA of typical prices
	sum := 0.0
	for _, tp := range c.typicalPrices {
		sum += tp
	}
	smaTp := sum / float64(c.period)

	// Calculate Mean Deviation
	// Mean Deviation = Average of |TP - SMA(TP)|
	sumDeviation := 0.0
	for _, tp := range c.typicalPrices {
		sumDeviation += math.Abs(tp - smaTp)
	}
	meanDeviation := sumDeviation / float64(c.period)

	// Calculate CCI
	// CCI = (TP - SMA(TP)) / (constant × Mean Deviation)
	currentTp := c.typicalPrices[len(c.typicalPrices)-1]

	if meanDeviation == 0 {
		c.cci = 0.0 // No deviation, CCI is 0
	} else {
		c.cci = (currentTp - smaTp) / (c.constant * meanDeviation)
	}

	c.AddValue(c.cci)
}

// GetValue returns the current CCI value
func (c *CCI) GetValue() float64 {
	return c.cci
}

// Reset resets the indicator
func (c *CCI) Reset() {
	c.BaseIndicator.Reset()
	c.typicalPrices = c.typicalPrices[:0]
	c.cci = 0
}

// IsReady returns true if the indicator has enough data
func (c *CCI) IsReady() bool {
	return len(c.typicalPrices) >= c.period
}

// GetPeriod returns the period
func (c *CCI) GetPeriod() int {
	return c.period
}

// IsOverbought returns true if CCI indicates overbought (> 100)
func (c *CCI) IsOverbought() bool {
	return c.cci > 100.0
}

// IsOversold returns true if CCI indicates oversold (< -100)
func (c *CCI) IsOversold() bool {
	return c.cci < -100.0
}

// IsStrongUptrend returns true if CCI shows strong upward momentum (> 200)
func (c *CCI) IsStrongUptrend() bool {
	return c.cci > 200.0
}

// IsStrongDowntrend returns true if CCI shows strong downward momentum (< -200)
func (c *CCI) IsStrongDowntrend() bool {
	return c.cci < -200.0
}
