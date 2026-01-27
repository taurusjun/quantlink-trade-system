package indicators

import (
	"fmt"
	"math"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// PriceImpact estimates the price impact of executing an order of specified size
// It simulates walking through the orderbook to estimate the execution price
type PriceImpact struct {
	*BaseIndicator
	volume   float64 // Order volume to simulate
	side     string  // "buy" or "sell"
	relative bool    // true: return relative impact (%), false: absolute impact
}

// NewPriceImpact creates a new PriceImpact indicator
func NewPriceImpact(volume float64, side string, relative bool, maxHistory int) *PriceImpact {
	if volume <= 0 {
		volume = 100
	}

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	// Validate side parameter
	if side != "buy" && side != "sell" {
		side = "buy"
	}

	return &PriceImpact{
		BaseIndicator: NewBaseIndicator(fmt.Sprintf("PriceImpact_%s", side), maxHistory),
		volume:        volume,
		side:          side,
		relative:      relative,
	}
}

// NewPriceImpactFromConfig creates PriceImpact from configuration
func NewPriceImpactFromConfig(config map[string]interface{}) (Indicator, error) {
	volume := 100.0
	side := "buy"
	relative := true
	maxHistory := 1000

	if v, ok := config["volume"]; ok {
		if vol, ok := v.(float64); ok {
			volume = vol
		}
	}

	if v, ok := config["side"]; ok {
		if s, ok := v.(string); ok {
			side = s
		}
	}

	if v, ok := config["relative"]; ok {
		if r, ok := v.(bool); ok {
			relative = r
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	if volume <= 0 {
		return nil, fmt.Errorf("%w: volume must be positive", ErrInvalidParameter)
	}

	if side != "buy" && side != "sell" {
		return nil, fmt.Errorf("%w: side must be 'buy' or 'sell'", ErrInvalidParameter)
	}

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return NewPriceImpact(volume, side, relative, maxHistory), nil
}

// Update calculates the price impact from market data
func (p *PriceImpact) Update(md *mdpb.MarketDataUpdate) {
	midPrice := GetMidPrice(md)
	if midPrice == 0 {
		return
	}

	execPrice := p.simulateExecution(md)
	if execPrice == 0 {
		return
	}

	var impact float64
	if p.relative {
		// Relative impact as percentage
		impact = (execPrice - midPrice) / midPrice
	} else {
		// Absolute impact
		impact = execPrice - midPrice
	}

	p.AddValue(impact)
}

// simulateExecution simulates executing an order through the orderbook
func (p *PriceImpact) simulateExecution(md *mdpb.MarketDataUpdate) float64 {
	remaining := p.volume
	totalCost := 0.0

	// Select the correct side of the book
	// For buy orders, we take liquidity from the ask side
	// For sell orders, we take liquidity from the bid side
	var prices []float64
	var volumes []uint32

	if p.side == "buy" {
		prices = md.AskPrice
		volumes = md.AskQty
	} else {
		prices = md.BidPrice
		volumes = md.BidQty
	}

	if len(prices) == 0 || len(volumes) == 0 {
		return 0.0
	}

	// Walk through the orderbook levels
	for i := 0; i < len(prices) && remaining > 0; i++ {
		availableQty := float64(volumes[i])
		fillQty := math.Min(remaining, availableQty)

		totalCost += fillQty * prices[i]
		remaining -= fillQty
	}

	// If we couldn't fill the entire order, use the last available price
	// This represents maximum slippage
	if remaining > 0 && len(prices) > 0 {
		lastPrice := prices[len(prices)-1]
		totalCost += remaining * lastPrice
	}

	// Calculate average execution price
	avgExecPrice := totalCost / p.volume

	return avgExecPrice
}

// SetVolume updates the simulation volume
func (p *PriceImpact) SetVolume(volume float64) {
	if volume > 0 {
		p.volume = volume
	}
}

// GetVolume returns the current simulation volume
func (p *PriceImpact) GetVolume() float64 {
	return p.volume
}

// Reset resets the indicator
func (p *PriceImpact) Reset() {
	p.BaseIndicator.Reset()
}

// IsReady returns true if we have at least one value
func (p *PriceImpact) IsReady() bool {
	return p.BaseIndicator.IsReady()
}
