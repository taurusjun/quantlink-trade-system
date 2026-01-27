package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// MidPrice calculates the mid price (average of best bid and ask)
// MidPrice = (BidPrice[0] + AskPrice[0]) / 2
type MidPrice struct {
	*BaseIndicator
}

// NewMidPrice creates a new MidPrice indicator
func NewMidPrice(maxHistory int) *MidPrice {
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &MidPrice{
		BaseIndicator: NewBaseIndicator("MidPrice", maxHistory),
	}
}

// NewMidPriceFromConfig creates MidPrice from configuration
func NewMidPriceFromConfig(config map[string]interface{}) (Indicator, error) {
	maxHistory := 1000

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return NewMidPrice(maxHistory), nil
}

// Update calculates the mid price from market data
func (m *MidPrice) Update(md *mdpb.MarketDataUpdate) {
	midPrice := GetMidPrice(md)
	if midPrice > 0 {
		m.AddValue(midPrice)
	}
}

// Reset resets the indicator
func (m *MidPrice) Reset() {
	m.BaseIndicator.Reset()
}

// IsReady returns true if we have at least one value
func (m *MidPrice) IsReady() bool {
	return m.BaseIndicator.IsReady()
}
