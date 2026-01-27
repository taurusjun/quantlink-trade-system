package indicators

import (
	"fmt"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// LiquidityRatio measures the ratio of orderbook depth to spread
// Higher values indicate better liquidity (more volume per unit of spread)
// Formula: LiquidityRatio = (BidVolume + AskVolume) / Spread
// Normalized: LiquidityRatio = (BidVolume + AskVolume) / (Spread / MidPrice)
type LiquidityRatio struct {
	*BaseIndicator
	levels     int
	normalized bool
	minSpread  float64 // Minimum spread to avoid division by zero
}

// NewLiquidityRatio creates a new LiquidityRatio indicator
func NewLiquidityRatio(levels int, normalized bool, minSpread float64, maxHistory int) *LiquidityRatio {
	if levels <= 0 {
		levels = 5
	}

	if minSpread <= 0 {
		minSpread = 0.0001
	}

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &LiquidityRatio{
		BaseIndicator: NewBaseIndicator("LiquidityRatio", maxHistory),
		levels:        levels,
		normalized:    normalized,
		minSpread:     minSpread,
	}
}

// NewLiquidityRatioFromConfig creates LiquidityRatio from configuration
func NewLiquidityRatioFromConfig(config map[string]interface{}) (Indicator, error) {
	levels := 5
	normalized := true
	minSpread := 0.0001
	maxHistory := 1000

	if v, ok := config["levels"]; ok {
		if l, ok := v.(float64); ok {
			levels = int(l)
		}
	}

	if v, ok := config["normalized"]; ok {
		if n, ok := v.(bool); ok {
			normalized = n
		}
	}

	if v, ok := config["min_spread"]; ok {
		if s, ok := v.(float64); ok {
			minSpread = s
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	if levels <= 0 {
		return nil, fmt.Errorf("%w: levels must be positive", ErrInvalidParameter)
	}

	if minSpread <= 0 {
		minSpread = 0.0001
	}

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return NewLiquidityRatio(levels, normalized, minSpread, maxHistory), nil
}

// Update calculates the liquidity ratio from market data
func (l *LiquidityRatio) Update(md *mdpb.MarketDataUpdate) {
	// Calculate total volume
	totalVolume := 0.0
	maxLevels := min(l.levels, min(len(md.BidQty), len(md.AskQty)))

	for i := 0; i < maxLevels; i++ {
		totalVolume += float64(md.BidQty[i])
	}

	for i := 0; i < maxLevels; i++ {
		totalVolume += float64(md.AskQty[i])
	}

	// Calculate spread
	spread := GetSpread(md)
	if spread < l.minSpread {
		spread = l.minSpread
	}

	// Normalize if enabled
	if l.normalized {
		midPrice := GetMidPrice(md)
		if midPrice > 0 {
			spread = spread / midPrice
		}
	}

	// Calculate liquidity ratio
	ratio := totalVolume / spread
	l.AddValue(ratio)
}

// Reset resets the indicator
func (l *LiquidityRatio) Reset() {
	l.BaseIndicator.Reset()
}

// IsReady returns true if we have at least one value
func (l *LiquidityRatio) IsReady() bool {
	return l.BaseIndicator.IsReady()
}
