package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// MSWPrice (Money Size Weighted Price) calculates a price weighted by price * volume
// MSW Price = sum(Price[i] * Volume[i] * Price[i]) / sum(Volume[i] * Price[i])
// This gives more weight to price levels with higher monetary value
type MSWPrice struct {
	*BaseIndicator
	numLevels int
	lastValue float64
}

// NewMSWPrice creates a new MSWPrice indicator
// numLevels: number of orderbook levels to consider (0 = all levels)
func NewMSWPrice(numLevels int, maxHistory int) *MSWPrice {
	if maxHistory <= 0 {
		maxHistory = 1000
	}
	if numLevels < 0 {
		numLevels = 0
	}

	return &MSWPrice{
		BaseIndicator: NewBaseIndicator("MSWPrice", maxHistory),
		numLevels:     numLevels,
	}
}

// NewMSWPriceFromConfig creates MSWPrice from configuration
func NewMSWPriceFromConfig(config map[string]interface{}) (Indicator, error) {
	numLevels := 5
	maxHistory := 1000

	if v, ok := config["num_levels"]; ok {
		if n, ok := v.(float64); ok {
			numLevels = int(n)
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewMSWPrice(numLevels, maxHistory), nil
}

// Update calculates the money-size weighted price
func (m *MSWPrice) Update(md *mdpb.MarketDataUpdate) {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 ||
		len(md.BidQty) == 0 || len(md.AskQty) == 0 {
		return
	}

	// Determine number of levels to use
	bidLevels := len(md.BidPrice)
	askLevels := len(md.AskPrice)

	if m.numLevels > 0 {
		if bidLevels > m.numLevels {
			bidLevels = m.numLevels
		}
		if askLevels > m.numLevels {
			askLevels = m.numLevels
		}
	}

	// Calculate weighted sum for bid side
	bidWeightedSum := 0.0
	bidWeightTotal := 0.0

	for i := 0; i < bidLevels; i++ {
		price := md.BidPrice[i]
		volume := float64(md.BidQty[i])
		weight := volume * price // money weight
		bidWeightedSum += price * weight
		bidWeightTotal += weight
	}

	// Calculate weighted sum for ask side
	askWeightedSum := 0.0
	askWeightTotal := 0.0

	for i := 0; i < askLevels; i++ {
		price := md.AskPrice[i]
		volume := float64(md.AskQty[i])
		weight := volume * price // money weight
		askWeightedSum += price * weight
		askWeightTotal += weight
	}

	// Combine bid and ask sides
	totalWeightedSum := bidWeightedSum + askWeightedSum
	totalWeight := bidWeightTotal + askWeightTotal

	if totalWeight > 0 {
		mswPrice := totalWeightedSum / totalWeight
		m.lastValue = mswPrice
		m.AddValue(mswPrice)
	}
}

// GetValue returns the current MSW price
func (m *MSWPrice) GetValue() float64 {
	return m.lastValue
}

// Reset resets the indicator
func (m *MSWPrice) Reset() {
	m.BaseIndicator.Reset()
	m.lastValue = 0
}

// MOWPrice (Market Order Weighted Price) calculates a price weighted by order count
// In practice, we approximate this by treating each level as one "order" and weighting by volume
// MOW Price = sum(Price[i] * Volume[i]) / sum(Volume[i])
// This is essentially a volume-weighted average price (VWAP) across all levels
type MOWPrice struct {
	*BaseIndicator
	numLevels int
	lastValue float64
}

// NewMOWPrice creates a new MOWPrice indicator
func NewMOWPrice(numLevels int, maxHistory int) *MOWPrice {
	if maxHistory <= 0 {
		maxHistory = 1000
	}
	if numLevels < 0 {
		numLevels = 0
	}

	return &MOWPrice{
		BaseIndicator: NewBaseIndicator("MOWPrice", maxHistory),
		numLevels:     numLevels,
	}
}

// NewMOWPriceFromConfig creates MOWPrice from configuration
func NewMOWPriceFromConfig(config map[string]interface{}) (Indicator, error) {
	numLevels := 5
	maxHistory := 1000

	if v, ok := config["num_levels"]; ok {
		if n, ok := v.(float64); ok {
			numLevels = int(n)
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewMOWPrice(numLevels, maxHistory), nil
}

// Update calculates the market-order weighted price
func (m *MOWPrice) Update(md *mdpb.MarketDataUpdate) {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 ||
		len(md.BidQty) == 0 || len(md.AskQty) == 0 {
		return
	}

	// Determine number of levels to use
	bidLevels := len(md.BidPrice)
	askLevels := len(md.AskPrice)

	if m.numLevels > 0 {
		if bidLevels > m.numLevels {
			bidLevels = m.numLevels
		}
		if askLevels > m.numLevels {
			askLevels = m.numLevels
		}
	}

	// Calculate weighted sum for bid side
	bidWeightedSum := 0.0
	bidVolumeTotal := 0.0

	for i := 0; i < bidLevels; i++ {
		price := md.BidPrice[i]
		volume := float64(md.BidQty[i])
		bidWeightedSum += price * volume
		bidVolumeTotal += volume
	}

	// Calculate weighted sum for ask side
	askWeightedSum := 0.0
	askVolumeTotal := 0.0

	for i := 0; i < askLevels; i++ {
		price := md.AskPrice[i]
		volume := float64(md.AskQty[i])
		askWeightedSum += price * volume
		askVolumeTotal += volume
	}

	// Combine bid and ask sides
	totalWeightedSum := bidWeightedSum + askWeightedSum
	totalVolume := bidVolumeTotal + askVolumeTotal

	if totalVolume > 0 {
		mowPrice := totalWeightedSum / totalVolume
		m.lastValue = mowPrice
		m.AddValue(mowPrice)
	}
}

// GetValue returns the current MOW price
func (m *MOWPrice) GetValue() float64 {
	return m.lastValue
}

// Reset resets the indicator
func (m *MOWPrice) Reset() {
	m.BaseIndicator.Reset()
	m.lastValue = 0
}
