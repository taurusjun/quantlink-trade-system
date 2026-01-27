package indicators

import (
	"fmt"
	"math"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// BidAskRatio calculates the ratio of bid volume to ask volume
// Ratio > 1: More buying pressure (bid volume dominates)
// Ratio < 1: More selling pressure (ask volume dominates)
// Ratio = 1: Balanced orderbook
// Optional log transformation for more stable values
type BidAskRatio struct {
	*BaseIndicator
	levels  int
	useLog  bool
	epsilon float64 // Small value to prevent division by zero
}

// NewBidAskRatio creates a new BidAskRatio indicator
func NewBidAskRatio(levels int, useLog bool, epsilon float64, maxHistory int) *BidAskRatio {
	if levels <= 0 {
		levels = 5
	}

	if epsilon <= 0 {
		epsilon = 0.01
	}

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	name := "BidAskRatio"
	if useLog {
		name = "LogBidAskRatio"
	}

	return &BidAskRatio{
		BaseIndicator: NewBaseIndicator(name, maxHistory),
		levels:        levels,
		useLog:        useLog,
		epsilon:       epsilon,
	}
}

// NewBidAskRatioFromConfig creates BidAskRatio from configuration
func NewBidAskRatioFromConfig(config map[string]interface{}) (Indicator, error) {
	levels := 5
	useLog := false
	epsilon := 0.01
	maxHistory := 1000

	if v, ok := config["levels"]; ok {
		if l, ok := v.(float64); ok {
			levels = int(l)
		}
	}

	if v, ok := config["use_log"]; ok {
		if u, ok := v.(bool); ok {
			useLog = u
		}
	}

	if v, ok := config["epsilon"]; ok {
		if e, ok := v.(float64); ok {
			epsilon = e
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

	if epsilon <= 0 {
		epsilon = 0.01
	}

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return NewBidAskRatio(levels, useLog, epsilon, maxHistory), nil
}

// Update calculates the bid-ask ratio from market data
func (b *BidAskRatio) Update(md *mdpb.MarketDataUpdate) {
	bidVol := 0.0
	askVol := 0.0

	// Calculate bid volume
	maxBidLevels := min(b.levels, len(md.BidQty))
	for i := 0; i < maxBidLevels; i++ {
		bidVol += float64(md.BidQty[i])
	}

	// Calculate ask volume
	maxAskLevels := min(b.levels, len(md.AskQty))
	for i := 0; i < maxAskLevels; i++ {
		askVol += float64(md.AskQty[i])
	}

	// Prevent division by zero
	if askVol < b.epsilon {
		askVol = b.epsilon
	}

	var ratio float64
	if b.useLog {
		// Log ratio: log(bid/ask)
		// Symmetric around 0: log(2) = -log(0.5)
		// Positive: more bids, Negative: more asks
		ratio = math.Log(bidVol / askVol)
	} else {
		// Linear ratio: bid/ask
		// > 1: more bids, < 1: more asks
		ratio = bidVol / askVol
	}

	b.AddValue(ratio)
}

// Reset resets the indicator
func (b *BidAskRatio) Reset() {
	b.BaseIndicator.Reset()
}

// IsReady returns true if we have at least one value
func (b *BidAskRatio) IsReady() bool {
	return b.BaseIndicator.IsReady()
}
