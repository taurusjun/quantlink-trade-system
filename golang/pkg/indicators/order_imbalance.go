package indicators

import (
	"fmt"
	"math"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// OrderImbalance calculates the order book imbalance
// A positive value indicates more buying pressure, negative indicates selling pressure
type OrderImbalance struct {
	*BaseIndicator
	levels      int     // Number of price levels to consider
	volumeWeight bool    // Weight by volume vs simple count
	normalized  bool    // Normalize to [-1, 1] range
}

// NewOrderImbalance creates a new OrderImbalance indicator
func NewOrderImbalance(levels int, volumeWeight bool, maxHistory int) *OrderImbalance {
	if levels <= 0 {
		levels = 5 // Default to top 5 levels
	}

	return &OrderImbalance{
		BaseIndicator: NewBaseIndicator("OrderImbalance", maxHistory),
		levels:        levels,
		volumeWeight:  volumeWeight,
		normalized:    true,
	}
}

// NewOrderImbalanceFromConfig creates OrderImbalance from configuration
func NewOrderImbalanceFromConfig(config map[string]interface{}) (Indicator, error) {
	levels := 5
	volumeWeight := true
	maxHistory := 1000

	if v, ok := config["levels"]; ok {
		if l, ok := v.(float64); ok {
			levels = int(l)
		}
	}

	if v, ok := config["volume_weight"]; ok {
		if w, ok := v.(bool); ok {
			volumeWeight = w
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

	return NewOrderImbalance(levels, volumeWeight, maxHistory), nil
}

// Update calculates the order imbalance from market data
func (oi *OrderImbalance) Update(md *mdpb.MarketDataUpdate) {
	var imbalance float64

	if oi.volumeWeight {
		imbalance = oi.calculateVolumeWeightedImbalance(md)
	} else {
		imbalance = oi.calculateSimpleImbalance(md)
	}

	// Normalize to [-1, 1] if enabled
	if oi.normalized && !math.IsNaN(imbalance) && !math.IsInf(imbalance, 0) {
		// Already normalized in calculation functions
		oi.AddValue(imbalance)
	} else {
		oi.AddValue(imbalance)
	}
}

// calculateVolumeWeightedImbalance calculates imbalance weighted by volume
func (oi *OrderImbalance) calculateVolumeWeightedImbalance(md *mdpb.MarketDataUpdate) float64 {
	maxLevels := min(oi.levels, min(len(md.BidQty), len(md.AskQty)))
	if maxLevels == 0 {
		return 0.0
	}

	var bidVolume, askVolume float64
	for i := 0; i < maxLevels; i++ {
		bidVolume += float64(md.BidQty[i])
		askVolume += float64(md.AskQty[i])
	}

	totalVolume := bidVolume + askVolume
	if totalVolume == 0 {
		return 0.0
	}

	// Return normalized imbalance: (bid - ask) / (bid + ask)
	// Range: [-1, 1] where 1 = all bids, -1 = all asks
	return (bidVolume - askVolume) / totalVolume
}

// calculateSimpleImbalance calculates simple level imbalance
func (oi *OrderImbalance) calculateSimpleImbalance(md *mdpb.MarketDataUpdate) float64 {
	maxLevels := min(oi.levels, min(len(md.BidPrice), len(md.AskPrice)))
	if maxLevels == 0 {
		return 0.0
	}

	bidLevels := float64(len(md.BidPrice))
	askLevels := float64(len(md.AskPrice))

	totalLevels := bidLevels + askLevels
	if totalLevels == 0 {
		return 0.0
	}

	return (bidLevels - askLevels) / totalLevels
}

// Reset resets the indicator
func (oi *OrderImbalance) Reset() {
	oi.BaseIndicator.Reset()
}

// IsReady returns true (always ready)
func (oi *OrderImbalance) IsReady() bool {
	return true
}

// WeightedOrderImbalance calculates order imbalance with price distance weighting
type WeightedOrderImbalance struct {
	*BaseIndicator
	levels       int
	decayFactor  float64 // Decay factor for price distance weighting
}

// NewWeightedOrderImbalance creates a new weighted order imbalance indicator
func NewWeightedOrderImbalance(levels int, decayFactor float64, maxHistory int) *WeightedOrderImbalance {
	if levels <= 0 {
		levels = 10
	}
	if decayFactor <= 0 {
		decayFactor = 1.0
	}

	return &WeightedOrderImbalance{
		BaseIndicator: NewBaseIndicator("WeightedOrderImbalance", maxHistory),
		levels:        levels,
		decayFactor:   decayFactor,
	}
}

// Update calculates weighted order imbalance
func (woi *WeightedOrderImbalance) Update(md *mdpb.MarketDataUpdate) {
	maxLevels := min(woi.levels, min(len(md.BidPrice), len(md.AskPrice)))
	if maxLevels == 0 || len(md.BidQty) < maxLevels || len(md.AskQty) < maxLevels {
		woi.AddValue(0.0)
		return
	}

	midPrice := GetMidPrice(md)
	if midPrice == 0 {
		woi.AddValue(0.0)
		return
	}

	var weightedBid, weightedAsk float64

	for i := 0; i < maxLevels; i++ {
		// Calculate price distance from mid
		bidDistance := math.Abs(md.BidPrice[i] - midPrice)
		askDistance := math.Abs(md.AskPrice[i] - midPrice)

		// Apply exponential decay based on distance
		bidWeight := math.Exp(-woi.decayFactor * bidDistance / midPrice)
		askWeight := math.Exp(-woi.decayFactor * askDistance / midPrice)

		// Accumulate weighted volumes
		weightedBid += float64(md.BidQty[i]) * bidWeight
		weightedAsk += float64(md.AskQty[i]) * askWeight
	}

	total := weightedBid + weightedAsk
	if total == 0 {
		woi.AddValue(0.0)
		return
	}

	// Normalized imbalance
	imbalance := (weightedBid - weightedAsk) / total
	woi.AddValue(imbalance)
}

// Reset resets the indicator
func (woi *WeightedOrderImbalance) Reset() {
	woi.BaseIndicator.Reset()
}

// IsReady returns true
func (woi *WeightedOrderImbalance) IsReady() bool {
	return true
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
