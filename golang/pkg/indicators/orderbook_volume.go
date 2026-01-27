package indicators

import (
	"fmt"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// OrderBookVolume calculates the cumulative volume in the orderbook
// Supports three modes: "bid", "ask", or "both"
type OrderBookVolume struct {
	*BaseIndicator
	levels    int
	side      string  // "bid", "ask", "both"
	bidVolume float64 // Current bid volume (for tracking)
	askVolume float64 // Current ask volume (for tracking)
}

// NewOrderBookVolume creates a new OrderBookVolume indicator
func NewOrderBookVolume(levels int, side string, maxHistory int) *OrderBookVolume {
	if levels <= 0 {
		levels = 5
	}

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	// Validate side parameter
	if side != "bid" && side != "ask" && side != "both" {
		side = "both"
	}

	return &OrderBookVolume{
		BaseIndicator: NewBaseIndicator(fmt.Sprintf("OrderBookVolume_%s", side), maxHistory),
		levels:        levels,
		side:          side,
	}
}

// NewOrderBookVolumeFromConfig creates OrderBookVolume from configuration
func NewOrderBookVolumeFromConfig(config map[string]interface{}) (Indicator, error) {
	levels := 5
	side := "both"
	maxHistory := 1000

	if v, ok := config["levels"]; ok {
		if l, ok := v.(float64); ok {
			levels = int(l)
		}
	}

	if v, ok := config["side"]; ok {
		if s, ok := v.(string); ok {
			side = s
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

	if side != "bid" && side != "ask" && side != "both" {
		return nil, fmt.Errorf("%w: side must be 'bid', 'ask', or 'both'", ErrInvalidParameter)
	}

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return NewOrderBookVolume(levels, side, maxHistory), nil
}

// Update calculates the orderbook volume from market data
func (o *OrderBookVolume) Update(md *mdpb.MarketDataUpdate) {
	bidVol := 0.0
	askVol := 0.0

	// Calculate bid volume
	maxBidLevels := min(o.levels, len(md.BidQty))
	for i := 0; i < maxBidLevels; i++ {
		bidVol += float64(md.BidQty[i])
	}

	// Calculate ask volume
	maxAskLevels := min(o.levels, len(md.AskQty))
	for i := 0; i < maxAskLevels; i++ {
		askVol += float64(md.AskQty[i])
	}

	// Store current volumes
	o.bidVolume = bidVol
	o.askVolume = askVol

	// Add value based on side
	var value float64
	switch o.side {
	case "bid":
		value = bidVol
	case "ask":
		value = askVol
	case "both":
		value = bidVol + askVol
	}

	o.AddValue(value)
}

// GetBidVolume returns the current bid volume
func (o *OrderBookVolume) GetBidVolume() float64 {
	return o.bidVolume
}

// GetAskVolume returns the current ask volume
func (o *OrderBookVolume) GetAskVolume() float64 {
	return o.askVolume
}

// Reset resets the indicator
func (o *OrderBookVolume) Reset() {
	o.BaseIndicator.Reset()
	o.bidVolume = 0
	o.askVolume = 0
}

// IsReady returns true if we have at least one value
func (o *OrderBookVolume) IsReady() bool {
	return o.BaseIndicator.IsReady()
}
