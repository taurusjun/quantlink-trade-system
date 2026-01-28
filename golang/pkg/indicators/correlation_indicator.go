package indicators

import (
	"github.com/yourusername/quantlink-trade-system/pkg/stats"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// CorrelationIndicator calculates rolling correlation between two price series
// This wraps the stats.Correlation function into an Indicator interface
type CorrelationIndicator struct {
	*BaseIndicator
	period      int
	series1     []float64
	series2     []float64
	symbol2     string // symbol to correlate with
	lastValue   float64
	priceCache  map[string]float64 // cache latest prices by symbol
}

// NewCorrelationIndicator creates a new Correlation indicator
// symbol2: the second symbol to correlate with (empty string means self-correlation)
func NewCorrelationIndicator(period int, symbol2 string, maxHistory int) *CorrelationIndicator {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &CorrelationIndicator{
		BaseIndicator: NewBaseIndicator("Correlation", maxHistory),
		period:        period,
		series1:       make([]float64, 0, period),
		series2:       make([]float64, 0, period),
		symbol2:       symbol2,
		priceCache:    make(map[string]float64),
	}
}

// NewCorrelationIndicatorFromConfig creates Correlation from configuration
func NewCorrelationIndicatorFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 20
	symbol2 := ""
	maxHistory := 1000

	if v, ok := config["period"]; ok {
		if p, ok := v.(float64); ok {
			period = int(p)
		}
	}

	if v, ok := config["symbol2"]; ok {
		if s, ok := v.(string); ok {
			symbol2 = s
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewCorrelationIndicator(period, symbol2, maxHistory), nil
}

// Update calculates rolling correlation
func (c *CorrelationIndicator) Update(md *mdpb.MarketDataUpdate) {
	price := GetMidPrice(md)
	if price <= 0 {
		return
	}

	// Cache the price by symbol
	c.priceCache[md.Symbol] = price

	// Add to series1 (always add current symbol's price)
	c.series1 = append(c.series1, price)
	if len(c.series1) > c.period {
		c.series1 = c.series1[1:]
	}

	// For series2, check if we have the other symbol's price
	if c.symbol2 != "" {
		// If correlating with another symbol, use cached price
		if price2, ok := c.priceCache[c.symbol2]; ok {
			c.series2 = append(c.series2, price2)
			if len(c.series2) > c.period {
				c.series2 = c.series2[1:]
			}
		}
	} else {
		// Self-correlation: use same series
		c.series2 = append(c.series2, price)
		if len(c.series2) > c.period {
			c.series2 = c.series2[1:]
		}
	}

	// Need at least period data points for both series
	if len(c.series1) < c.period || len(c.series2) < c.period {
		return
	}

	// Calculate correlation using stats package
	corr := stats.Correlation(c.series1, c.series2)
	c.lastValue = corr
	c.AddValue(corr)
}

// UpdateWithPair updates with a pair of prices (useful for cross-symbol correlation)
func (c *CorrelationIndicator) UpdateWithPair(price1, price2 float64) {
	if price1 <= 0 || price2 <= 0 {
		return
	}

	c.series1 = append(c.series1, price1)
	if len(c.series1) > c.period {
		c.series1 = c.series1[1:]
	}

	c.series2 = append(c.series2, price2)
	if len(c.series2) > c.period {
		c.series2 = c.series2[1:]
	}

	if len(c.series1) >= c.period && len(c.series2) >= c.period {
		corr := stats.Correlation(c.series1, c.series2)
		c.lastValue = corr
		c.AddValue(corr)
	}
}

// GetValue returns the current correlation value
func (c *CorrelationIndicator) GetValue() float64 {
	return c.lastValue
}

// GetPeriod returns the calculation period
func (c *CorrelationIndicator) GetPeriod() int {
	return c.period
}

// Reset resets the indicator
func (c *CorrelationIndicator) Reset() {
	c.BaseIndicator.Reset()
	c.series1 = c.series1[:0]
	c.series2 = c.series2[:0]
	c.priceCache = make(map[string]float64)
	c.lastValue = 0
}

// IsReady returns true if we have enough data
func (c *CorrelationIndicator) IsReady() bool {
	return len(c.series1) >= c.period && len(c.series2) >= c.period
}
