package indicators

import (
	"github.com/yourusername/quantlink-trade-system/pkg/stats"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// CovarianceIndicator calculates rolling covariance between two price series
type CovarianceIndicator struct {
	*BaseIndicator
	period      int
	series1     []float64
	series2     []float64
	symbol2     string
	lastValue   float64
	priceCache  map[string]float64
}

// NewCovarianceIndicator creates a new Covariance indicator
func NewCovarianceIndicator(period int, symbol2 string, maxHistory int) *CovarianceIndicator {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &CovarianceIndicator{
		BaseIndicator: NewBaseIndicator("Covariance", maxHistory),
		period:        period,
		series1:       make([]float64, 0, period),
		series2:       make([]float64, 0, period),
		symbol2:       symbol2,
		priceCache:    make(map[string]float64),
	}
}

// NewCovarianceIndicatorFromConfig creates Covariance from configuration
func NewCovarianceIndicatorFromConfig(config map[string]interface{}) (Indicator, error) {
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

	return NewCovarianceIndicator(period, symbol2, maxHistory), nil
}

// Update calculates rolling covariance
func (c *CovarianceIndicator) Update(md *mdpb.MarketDataUpdate) {
	price := GetMidPrice(md)
	if price <= 0 {
		return
	}

	c.priceCache[md.Symbol] = price
	c.series1 = append(c.series1, price)
	if len(c.series1) > c.period {
		c.series1 = c.series1[1:]
	}

	if c.symbol2 != "" {
		if price2, ok := c.priceCache[c.symbol2]; ok {
			c.series2 = append(c.series2, price2)
			if len(c.series2) > c.period {
				c.series2 = c.series2[1:]
			}
		}
	} else {
		c.series2 = append(c.series2, price)
		if len(c.series2) > c.period {
			c.series2 = c.series2[1:]
		}
	}

	if len(c.series1) >= c.period && len(c.series2) >= c.period {
		cov := stats.Covariance(c.series1, c.series2)
		c.lastValue = cov
		c.AddValue(cov)
	}
}

// UpdateWithPair updates with a pair of prices
func (c *CovarianceIndicator) UpdateWithPair(price1, price2 float64) {
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
		cov := stats.Covariance(c.series1, c.series2)
		c.lastValue = cov
		c.AddValue(cov)
	}
}

// GetValue returns the current covariance value
func (c *CovarianceIndicator) GetValue() float64 {
	return c.lastValue
}

// Reset resets the indicator
func (c *CovarianceIndicator) Reset() {
	c.BaseIndicator.Reset()
	c.series1 = c.series1[:0]
	c.series2 = c.series2[:0]
	c.priceCache = make(map[string]float64)
	c.lastValue = 0
}

// IsReady returns true if we have enough data
func (c *CovarianceIndicator) IsReady() bool {
	return len(c.series1) >= c.period && len(c.series2) >= c.period
}

// BetaIndicator calculates rolling beta coefficient (hedge ratio)
type BetaIndicator struct {
	*BaseIndicator
	period      int
	series1     []float64 // dependent variable (Y)
	series2     []float64 // independent variable (X)
	symbol2     string
	lastValue   float64
	priceCache  map[string]float64
}

// NewBetaIndicator creates a new Beta indicator
func NewBetaIndicator(period int, symbol2 string, maxHistory int) *BetaIndicator {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &BetaIndicator{
		BaseIndicator: NewBaseIndicator("Beta", maxHistory),
		period:        period,
		series1:       make([]float64, 0, period),
		series2:       make([]float64, 0, period),
		symbol2:       symbol2,
		priceCache:    make(map[string]float64),
	}
}

// NewBetaIndicatorFromConfig creates Beta from configuration
func NewBetaIndicatorFromConfig(config map[string]interface{}) (Indicator, error) {
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

	return NewBetaIndicator(period, symbol2, maxHistory), nil
}

// Update calculates rolling beta
func (b *BetaIndicator) Update(md *mdpb.MarketDataUpdate) {
	price := GetMidPrice(md)
	if price <= 0 {
		return
	}

	b.priceCache[md.Symbol] = price
	b.series1 = append(b.series1, price)
	if len(b.series1) > b.period {
		b.series1 = b.series1[1:]
	}

	if b.symbol2 != "" {
		if price2, ok := b.priceCache[b.symbol2]; ok {
			b.series2 = append(b.series2, price2)
			if len(b.series2) > b.period {
				b.series2 = b.series2[1:]
			}
		}
	} else {
		b.series2 = append(b.series2, price)
		if len(b.series2) > b.period {
			b.series2 = b.series2[1:]
		}
	}

	if len(b.series1) >= b.period && len(b.series2) >= b.period {
		beta := stats.Beta(b.series1, b.series2)
		b.lastValue = beta
		b.AddValue(beta)
	}
}

// UpdateWithPair updates with a pair of prices
func (b *BetaIndicator) UpdateWithPair(price1, price2 float64) {
	if price1 <= 0 || price2 <= 0 {
		return
	}

	b.series1 = append(b.series1, price1)
	if len(b.series1) > b.period {
		b.series1 = b.series1[1:]
	}

	b.series2 = append(b.series2, price2)
	if len(b.series2) > b.period {
		b.series2 = b.series2[1:]
	}

	if len(b.series1) >= b.period && len(b.series2) >= b.period {
		beta := stats.Beta(b.series1, b.series2)
		b.lastValue = beta
		b.AddValue(beta)
	}
}

// GetValue returns the current beta value
func (b *BetaIndicator) GetValue() float64 {
	return b.lastValue
}

// Reset resets the indicator
func (b *BetaIndicator) Reset() {
	b.BaseIndicator.Reset()
	b.series1 = b.series1[:0]
	b.series2 = b.series2[:0]
	b.priceCache = make(map[string]float64)
	b.lastValue = 0
}

// IsReady returns true if we have enough data
func (b *BetaIndicator) IsReady() bool {
	return len(b.series1) >= b.period && len(b.series2) >= b.period
}
