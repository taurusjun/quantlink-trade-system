package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// CMO (Chande Momentum Oscillator) measures the momentum using the sum of upward
// and downward price changes
//
// Formula:
// CMO = 100 × (Su - Sd) / (Su + Sd)
//
// Where:
// - Su = Sum of upward price changes over period
// - Sd = Sum of downward price changes over period (absolute values)
//
// Range: -100 to +100
// - Above +50: Strong upward momentum
// - Below -50: Strong downward momentum
// - Between -50 and +50: Weak or neutral momentum
// - Crosses above 0: Bullish signal
// - Crosses below 0: Bearish signal
//
// Properties:
// - Bounded oscillator (-100 to +100)
// - Similar to RSI but uses raw price changes
// - More responsive than RSI
// - Good for identifying overbought/oversold conditions
type CMO struct {
	*BaseIndicator
	period       int
	prices       []float64
	upChanges    []float64
	downChanges  []float64
	cmo          float64
	prevCMO      float64
}

// NewCMO creates a new CMO indicator
func NewCMO(period int, maxHistory int) *CMO {
	if period <= 0 {
		period = 14
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &CMO{
		BaseIndicator: NewBaseIndicator("CMO", maxHistory),
		period:        period,
		prices:        make([]float64, 0, period+1),
		upChanges:     make([]float64, 0, period),
		downChanges:   make([]float64, 0, period),
	}
}

// NewCMOFromConfig creates CMO from configuration
func NewCMOFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 14
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

	return NewCMO(period, maxHistory), nil
}

// Update updates the indicator with new market data
func (c *CMO) Update(md *mdpb.MarketDataUpdate) {
	price := GetMidPrice(md)
	if price <= 0 {
		return
	}

	// Add current price
	c.prices = append(c.prices, price)

	// Keep period+1 prices (need previous price to calculate change)
	if len(c.prices) > c.period+1 {
		c.prices = c.prices[1:]
	}

	// Need at least 2 prices to calculate change
	if len(c.prices) < 2 {
		return
	}

	// Calculate price change
	change := c.prices[len(c.prices)-1] - c.prices[len(c.prices)-2]

	// Store upward and downward changes
	if change > 0 {
		c.upChanges = append(c.upChanges, change)
		c.downChanges = append(c.downChanges, 0.0)
	} else if change < 0 {
		c.upChanges = append(c.upChanges, 0.0)
		c.downChanges = append(c.downChanges, -change) // Store as positive
	} else {
		c.upChanges = append(c.upChanges, 0.0)
		c.downChanges = append(c.downChanges, 0.0)
	}

	// Keep only period elements
	if len(c.upChanges) > c.period {
		c.upChanges = c.upChanges[1:]
		c.downChanges = c.downChanges[1:]
	}

	// Need at least period values
	if len(c.upChanges) < c.period {
		return
	}

	// Calculate sum of upward and downward changes
	sumUp := 0.0
	sumDown := 0.0

	for i := 0; i < c.period; i++ {
		sumUp += c.upChanges[i]
		sumDown += c.downChanges[i]
	}

	// Calculate CMO
	// CMO = 100 × (Su - Sd) / (Su + Sd)
	denominator := sumUp + sumDown

	c.prevCMO = c.cmo

	if denominator == 0 {
		c.cmo = 0.0 // No change, CMO is 0
	} else {
		c.cmo = 100.0 * (sumUp - sumDown) / denominator
	}

	c.AddValue(c.cmo)
}

// GetValue returns the current CMO value
func (c *CMO) GetValue() float64 {
	return c.cmo
}

// Reset resets the indicator
func (c *CMO) Reset() {
	c.BaseIndicator.Reset()
	c.prices = c.prices[:0]
	c.upChanges = c.upChanges[:0]
	c.downChanges = c.downChanges[:0]
	c.cmo = 0
	c.prevCMO = 0
}

// IsReady returns true if the indicator has enough data
func (c *CMO) IsReady() bool {
	return len(c.upChanges) >= c.period
}

// GetPeriod returns the period
func (c *CMO) GetPeriod() int {
	return c.period
}

// IsStrongUpMomentum returns true if CMO shows strong upward momentum (> 50)
func (c *CMO) IsStrongUpMomentum() bool {
	return c.cmo > 50.0
}

// IsStrongDownMomentum returns true if CMO shows strong downward momentum (< -50)
func (c *CMO) IsStrongDownMomentum() bool {
	return c.cmo < -50.0
}

// IsBullishCross returns true if CMO just crossed above 0 (bullish signal)
func (c *CMO) IsBullishCross() bool {
	return c.prevCMO <= 0 && c.cmo > 0
}

// IsBearishCross returns true if CMO just crossed below 0 (bearish signal)
func (c *CMO) IsBearishCross() bool {
	return c.prevCMO >= 0 && c.cmo < 0
}

// IsOverbought returns true if CMO indicates overbought (> 50)
func (c *CMO) IsOverbought() bool {
	return c.cmo > 50.0
}

// IsOversold returns true if CMO indicates oversold (< -50)
func (c *CMO) IsOversold() bool {
	return c.cmo < -50.0
}
