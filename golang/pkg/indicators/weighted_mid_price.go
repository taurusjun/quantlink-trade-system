package indicators

import (
	"fmt"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// WeightedMidPrice calculates the volume-weighted mid price
// WeightedMidPrice = (BidPrice * AskVolume + AskPrice * BidVolume) / (BidVolume + AskVolume)
// This gives more weight to the side with less volume, reflecting the market's valuation
type WeightedMidPrice struct {
	*BaseIndicator
	levels int // Number of price levels to consider
}

// NewWeightedMidPrice creates a new WeightedMidPrice indicator
func NewWeightedMidPrice(levels int, maxHistory int) *WeightedMidPrice {
	if levels <= 0 {
		levels = 1
	}

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &WeightedMidPrice{
		BaseIndicator: NewBaseIndicator("WeightedMidPrice", maxHistory),
		levels:        levels,
	}
}

// NewWeightedMidPriceFromConfig creates WeightedMidPrice from configuration
func NewWeightedMidPriceFromConfig(config map[string]interface{}) (Indicator, error) {
	levels := 1
	maxHistory := 1000

	if v, ok := config["levels"]; ok {
		if l, ok := v.(float64); ok {
			levels = int(l)
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

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return NewWeightedMidPrice(levels, maxHistory), nil
}

// Update calculates the weighted mid price from market data
func (w *WeightedMidPrice) Update(md *mdpb.MarketDataUpdate) {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 ||
		len(md.BidQty) == 0 || len(md.AskQty) == 0 {
		return
	}

	// For single level, use the utility function
	if w.levels == 1 {
		weightedMidPrice := GetWeightedMidPrice(md)
		if weightedMidPrice > 0 {
			w.AddValue(weightedMidPrice)
		}
		return
	}

	// For multi-level, calculate weighted average across levels
	maxLevels := min(w.levels, min(len(md.BidPrice), len(md.AskPrice)))
	maxLevels = min(maxLevels, min(len(md.BidQty), len(md.AskQty)))

	if maxLevels == 0 {
		return
	}

	var totalBidVolume, totalAskVolume float64
	var bidWeightedPrice, askWeightedPrice float64

	for i := 0; i < maxLevels; i++ {
		bidVol := float64(md.BidQty[i])
		askVol := float64(md.AskQty[i])

		totalBidVolume += bidVol
		totalAskVolume += askVol

		// Weighted by opposite side volume (standard weighted mid price formula)
		bidWeightedPrice += md.BidPrice[i] * askVol
		askWeightedPrice += md.AskPrice[i] * bidVol
	}

	totalVolume := totalBidVolume + totalAskVolume
	if totalVolume == 0 {
		// Fallback to simple mid price
		var sumBid, sumAsk float64
		for i := 0; i < maxLevels; i++ {
			sumBid += md.BidPrice[i]
			sumAsk += md.AskPrice[i]
		}
		midPrice := (sumBid + sumAsk) / float64(2*maxLevels)
		w.AddValue(midPrice)
		return
	}

	weightedMidPrice := (bidWeightedPrice + askWeightedPrice) / totalVolume
	w.AddValue(weightedMidPrice)
}

// Reset resets the indicator
func (w *WeightedMidPrice) Reset() {
	w.BaseIndicator.Reset()
}

// IsReady returns true if we have at least one value
func (w *WeightedMidPrice) IsReady() bool {
	return w.BaseIndicator.IsReady()
}
