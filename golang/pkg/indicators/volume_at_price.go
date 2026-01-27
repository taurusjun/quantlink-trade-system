package indicators

import (
	"fmt"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// VolumeAtPrice tracks volume at specific price levels
// 特定价位挂单量：跟踪特定价格档位的挂单量
type VolumeAtPrice struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	targetPrice float64 // Target price to track (0 = use best bid/ask)
	side        string  // "bid" or "ask"
	tolerance   float64 // Price matching tolerance

	// Current state
	volume      float64 // Volume at target price
	actualPrice float64 // Actual price found
}

// NewVolumeAtPrice creates a new VolumeAtPrice indicator
func NewVolumeAtPrice(name string, targetPrice float64, side string, tolerance float64, maxHistory int) *VolumeAtPrice {
	if side == "" {
		side = "bid"
	}
	if tolerance <= 0 {
		tolerance = 0.01 // Default 1 cent tolerance
	}

	return &VolumeAtPrice{
		BaseIndicator: NewBaseIndicator(name, maxHistory),
		targetPrice:   targetPrice,
		side:          side,
		tolerance:     tolerance,
	}
}

// NewVolumeAtPriceFromConfig creates a VolumeAtPrice indicator from configuration
func NewVolumeAtPriceFromConfig(name string, config map[string]interface{}) (Indicator, error) {
	targetPrice := 0.0
	if v, ok := config["target_price"]; ok {
		if fv, ok := v.(float64); ok {
			targetPrice = fv
		}
	}

	side := "bid"
	if v, ok := config["side"]; ok {
		if sv, ok := v.(string); ok {
			side = sv
		}
	}

	tolerance := 0.01
	if v, ok := config["tolerance"]; ok {
		if fv, ok := v.(float64); ok {
			tolerance = fv
		}
	}

	maxHistory := 1000
	if v, ok := config["max_history"]; ok {
		if fv, ok := v.(float64); ok {
			maxHistory = int(fv)
		}
	}

	return NewVolumeAtPrice(name, targetPrice, side, tolerance, maxHistory), nil
}

// Update updates the indicator with new market data
func (vap *VolumeAtPrice) Update(md *mdpb.MarketDataUpdate) {
	vap.mu.Lock()
	defer vap.mu.Unlock()

	var prices []float64
	var quantities []uint32

	if vap.side == "bid" {
		prices = md.BidPrice
		quantities = md.BidQty
	} else {
		prices = md.AskPrice
		quantities = md.AskQty
	}

	// If target price is 0, use best price
	effectiveTarget := vap.targetPrice
	if effectiveTarget == 0 && len(prices) > 0 {
		effectiveTarget = prices[0]
	}

	// Find volume at target price
	vap.volume = 0
	vap.actualPrice = 0

	for i := 0; i < len(prices); i++ {
		priceDiff := prices[i] - effectiveTarget
		if priceDiff < 0 {
			priceDiff = -priceDiff
		}

		if priceDiff <= vap.tolerance {
			vap.volume = float64(quantities[i])
			vap.actualPrice = prices[i]
			break
		}
	}

	vap.AddValue(vap.volume)
}

// GetValue returns the current volume at price
func (vap *VolumeAtPrice) GetValue() float64 {
	vap.mu.RLock()
	defer vap.mu.RUnlock()
	return vap.volume
}

// GetActualPrice returns the actual price where volume was found
func (vap *VolumeAtPrice) GetActualPrice() float64 {
	vap.mu.RLock()
	defer vap.mu.RUnlock()
	return vap.actualPrice
}

// SetTargetPrice updates the target price
func (vap *VolumeAtPrice) SetTargetPrice(price float64) {
	vap.mu.Lock()
	defer vap.mu.Unlock()
	vap.targetPrice = price
}

// String returns a string representation of the indicator
func (vap *VolumeAtPrice) String() string {
	return fmt.Sprintf("VolumeAtPrice(target=%.2f, side=%s, volume=%.0f)",
		vap.targetPrice, vap.side, vap.GetValue())
}
