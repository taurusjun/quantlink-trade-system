package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// BBD (Best Bid Distance) calculates the distance from a reference price to the best bid
// Distance = ReferencePrice - BestBidPrice
type BBD struct {
	*BaseIndicator
	referenceType string  // "mid", "last", "vwap", or "fixed"
	fixedPrice    float64 // used when referenceType is "fixed"
	lastValue     float64
}

// NewBBD creates a new BBD indicator
// referenceType: "mid" (default), "last", "vwap", or "fixed"
// fixedPrice: used when referenceType is "fixed"
func NewBBD(referenceType string, fixedPrice float64, maxHistory int) *BBD {
	if referenceType == "" {
		referenceType = "mid"
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &BBD{
		BaseIndicator: NewBaseIndicator("BBD", maxHistory),
		referenceType: referenceType,
		fixedPrice:    fixedPrice,
	}
}

// NewBBDFromConfig creates BBD from configuration
func NewBBDFromConfig(config map[string]interface{}) (Indicator, error) {
	referenceType := "mid"
	fixedPrice := 0.0
	maxHistory := 1000

	if v, ok := config["reference_type"]; ok {
		if t, ok := v.(string); ok {
			referenceType = t
		}
	}

	if v, ok := config["fixed_price"]; ok {
		if p, ok := v.(float64); ok {
			fixedPrice = p
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewBBD(referenceType, fixedPrice, maxHistory), nil
}

// Update calculates the best bid distance
func (b *BBD) Update(md *mdpb.MarketDataUpdate) {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return
	}

	bestBid := md.BidPrice[0]
	var referencePrice float64

	switch b.referenceType {
	case "mid":
		referencePrice = GetMidPrice(md)
	case "last":
		referencePrice = md.LastPrice
	case "fixed":
		referencePrice = b.fixedPrice
	case "vwap":
		referencePrice = GetWeightedMidPrice(md)
	default:
		referencePrice = GetMidPrice(md)
	}

	if referencePrice > 0 && bestBid > 0 {
		distance := referencePrice - bestBid
		b.lastValue = distance
		b.AddValue(distance)
	}
}

// GetValue returns the current BBD value
func (b *BBD) GetValue() float64 {
	return b.lastValue
}

// Reset resets the indicator
func (b *BBD) Reset() {
	b.BaseIndicator.Reset()
	b.lastValue = 0
}

// BAD (Best Ask Distance) calculates the distance from the best ask to a reference price
// Distance = BestAskPrice - ReferencePrice
type BAD struct {
	*BaseIndicator
	referenceType string
	fixedPrice    float64
	lastValue     float64
}

// NewBAD creates a new BAD indicator
func NewBAD(referenceType string, fixedPrice float64, maxHistory int) *BAD {
	if referenceType == "" {
		referenceType = "mid"
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &BAD{
		BaseIndicator: NewBaseIndicator("BAD", maxHistory),
		referenceType: referenceType,
		fixedPrice:    fixedPrice,
	}
}

// NewBADFromConfig creates BAD from configuration
func NewBADFromConfig(config map[string]interface{}) (Indicator, error) {
	referenceType := "mid"
	fixedPrice := 0.0
	maxHistory := 1000

	if v, ok := config["reference_type"]; ok {
		if t, ok := v.(string); ok {
			referenceType = t
		}
	}

	if v, ok := config["fixed_price"]; ok {
		if p, ok := v.(float64); ok {
			fixedPrice = p
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewBAD(referenceType, fixedPrice, maxHistory), nil
}

// Update calculates the best ask distance
func (b *BAD) Update(md *mdpb.MarketDataUpdate) {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return
	}

	bestAsk := md.AskPrice[0]
	var referencePrice float64

	switch b.referenceType {
	case "mid":
		referencePrice = GetMidPrice(md)
	case "last":
		referencePrice = md.LastPrice
	case "fixed":
		referencePrice = b.fixedPrice
	case "vwap":
		referencePrice = GetWeightedMidPrice(md)
	default:
		referencePrice = GetMidPrice(md)
	}

	if referencePrice > 0 && bestAsk > 0 {
		distance := bestAsk - referencePrice
		b.lastValue = distance
		b.AddValue(distance)
	}
}

// GetValue returns the current BAD value
func (b *BAD) GetValue() float64 {
	return b.lastValue
}

// Reset resets the indicator
func (b *BAD) Reset() {
	b.BaseIndicator.Reset()
	b.lastValue = 0
}
