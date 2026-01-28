package indicators

import (
	"math"

	"github.com/yourusername/quantlink-trade-system/pkg/stats"
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// CointegrationIndicator tests for cointegration between two price series
// Uses a simplified approach: linear regression + residual stationarity test
// A negative score indicates stronger evidence of cointegration (residuals are mean-reverting)
type CointegrationIndicator struct {
	*BaseIndicator
	period      int
	series1     []float64 // dependent variable (Y)
	series2     []float64 // independent variable (X)
	symbol2     string
	lastValue   float64 // cointegration score (lower = more cointegrated)
	priceCache  map[string]float64
	beta        float64 // hedge ratio from linear regression
	residuals   []float64
}

// NewCointegrationIndicator creates a new Cointegration indicator
func NewCointegrationIndicator(period int, symbol2 string, maxHistory int) *CointegrationIndicator {
	if period <= 0 {
		period = 60 // Longer period for cointegration test
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &CointegrationIndicator{
		BaseIndicator: NewBaseIndicator("Cointegration", maxHistory),
		period:        period,
		series1:       make([]float64, 0, period),
		series2:       make([]float64, 0, period),
		symbol2:       symbol2,
		priceCache:    make(map[string]float64),
		residuals:     make([]float64, 0, period),
	}
}

// NewCointegrationIndicatorFromConfig creates Cointegration from configuration
func NewCointegrationIndicatorFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 60
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

	return NewCointegrationIndicator(period, symbol2, maxHistory), nil
}

// Update calculates the cointegration test
func (c *CointegrationIndicator) Update(md *mdpb.MarketDataUpdate) {
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
		c.calculateCointegration()
	}
}

// UpdateWithPair updates with a pair of prices
func (c *CointegrationIndicator) UpdateWithPair(price1, price2 float64) {
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
		c.calculateCointegration()
	}
}

func (c *CointegrationIndicator) calculateCointegration() {
	// Step 1: Perform linear regression Y = α + βX
	slope, intercept := stats.LinearRegression(c.series2, c.series1)
	c.beta = slope

	// Step 2: Calculate residuals
	c.residuals = make([]float64, len(c.series1))
	for i := range c.series1 {
		predicted := intercept + slope*c.series2[i]
		c.residuals[i] = c.series1[i] - predicted
	}

	// Step 3: Simplified stationarity test
	// Calculate the Half-Life of mean reversion
	// A shorter half-life indicates stronger mean reversion (more cointegrated)
	halfLife := c.calculateHalfLife()

	// Step 4: Calculate normalized score
	// Lower score = more cointegrated (residuals revert faster to mean)
	// Score is the half-life normalized by the period
	score := halfLife / float64(c.period)

	c.lastValue = score
	c.AddValue(score)
}

// calculateHalfLife calculates the half-life of mean reversion for residuals
// Using AR(1) model: Δy_t = λ * y_{t-1} + ε_t
// Half-life = -log(2) / log(1 + λ)
func (c *CointegrationIndicator) calculateHalfLife() float64 {
	if len(c.residuals) < 2 {
		return math.Inf(1)
	}

	// Calculate first differences
	n := len(c.residuals)
	y := make([]float64, n-1)      // Δy_t
	yLag := make([]float64, n-1)   // y_{t-1}

	for i := 1; i < n; i++ {
		y[i-1] = c.residuals[i] - c.residuals[i-1]  // Δy_t
		yLag[i-1] = c.residuals[i-1]                // y_{t-1}
	}

	// Perform regression: Δy_t = λ * y_{t-1}
	// This gives us λ (the mean reversion speed)
	lambda, _ := stats.LinearRegression(yLag, y)

	// Calculate half-life
	// If λ >= 0, no mean reversion (infinite half-life)
	if lambda >= 0 {
		return math.Inf(1)
	}

	// Half-life = -ln(2) / ln(1 + λ)
	// For negative λ, (1 + λ) is between 0 and 1
	halfLife := -math.Log(2) / math.Log(1+lambda)

	// Sanity check
	if halfLife < 0 || math.IsNaN(halfLife) || math.IsInf(halfLife, 0) {
		return math.Inf(1)
	}

	return halfLife
}

// GetValue returns the cointegration score
// Lower values indicate stronger cointegration (residuals mean-revert faster)
// Typical interpretation:
// - Score < 0.3: Strong cointegration
// - Score 0.3-0.5: Moderate cointegration
// - Score > 0.5: Weak/no cointegration
func (c *CointegrationIndicator) GetValue() float64 {
	return c.lastValue
}

// GetBeta returns the hedge ratio from linear regression
func (c *CointegrationIndicator) GetBeta() float64 {
	return c.beta
}

// GetResiduals returns the current residuals
func (c *CointegrationIndicator) GetResiduals() []float64 {
	result := make([]float64, len(c.residuals))
	copy(result, c.residuals)
	return result
}

// IsCointegrated returns true if the score indicates cointegration
func (c *CointegrationIndicator) IsCointegrated(threshold float64) bool {
	if threshold <= 0 {
		threshold = 0.5 // Default threshold
	}
	return c.lastValue < threshold && !math.IsInf(c.lastValue, 0)
}

// Reset resets the indicator
func (c *CointegrationIndicator) Reset() {
	c.BaseIndicator.Reset()
	c.series1 = c.series1[:0]
	c.series2 = c.series2[:0]
	c.residuals = c.residuals[:0]
	c.priceCache = make(map[string]float64)
	c.lastValue = 0
	c.beta = 0
}

// IsReady returns true if we have enough data
func (c *CointegrationIndicator) IsReady() bool {
	return len(c.series1) >= c.period && len(c.series2) >= c.period
}
