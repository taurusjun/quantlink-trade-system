package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// PriceChannel (Price Channel) is a simplified version of Donchian Channels
// that only tracks the highest high and lowest low over a period.
//
// Components:
// - Upper Channel: Highest high over period
// - Lower Channel: Lowest low over period
// - No middle line (unlike Donchian Channels)
//
// Interpretation:
// - Similar to Donchian Channels
// - Breakout above upper: Bullish signal
// - Breakout below lower: Bearish signal
// - Simpler than Donchian (no middle line)
//
// Properties:
// - Very simple and fast
// - Good for breakout trading
// - Less information than Donchian (no middle line)
// - Works well in trending markets
type PriceChannel struct {
	*BaseIndicator
	period       int
	highs        []float64
	lows         []float64
	upperChannel float64
	lowerChannel float64
}

// NewPriceChannel creates a new Price Channel indicator
func NewPriceChannel(period int, maxHistory int) *PriceChannel {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &PriceChannel{
		BaseIndicator: NewBaseIndicator("Price Channel", maxHistory),
		period:        period,
		highs:         make([]float64, 0, period),
		lows:          make([]float64, 0, period),
	}
}

// NewPriceChannelFromConfig creates Price Channel from configuration
func NewPriceChannelFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 20
	maxHistory := 1000

	if v, ok := config["period"]; ok {
		if p, ok := v.(float64); ok {
			period = int(p)
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewPriceChannel(period, maxHistory), nil
}

// Update updates the indicator with new market data
func (p *PriceChannel) Update(md *mdpb.MarketDataUpdate) {
	close := GetMidPrice(md)
	if close <= 0 {
		return
	}

	// Calculate high and low
	high := close
	low := close
	if len(md.AskPrice) > 0 && md.AskPrice[0] > high {
		high = md.AskPrice[0]
	}
	if len(md.BidPrice) > 0 && md.BidPrice[0] < low {
		low = md.BidPrice[0]
	}

	// Store high and low
	p.highs = append(p.highs, high)
	p.lows = append(p.lows, low)

	// Keep only period values
	if len(p.highs) > p.period {
		p.highs = p.highs[1:]
		p.lows = p.lows[1:]
	}

	// Need at least period values
	if len(p.highs) < p.period {
		return
	}

	// Calculate upper channel (highest high)
	p.upperChannel = p.highs[0]
	for i := 1; i < len(p.highs); i++ {
		if p.highs[i] > p.upperChannel {
			p.upperChannel = p.highs[i]
		}
	}

	// Calculate lower channel (lowest low)
	p.lowerChannel = p.lows[0]
	for i := 1; i < len(p.lows); i++ {
		if p.lows[i] < p.lowerChannel {
			p.lowerChannel = p.lows[i]
		}
	}

	// Store channel width as the main value
	p.AddValue(p.GetChannelWidth())
}

// GetValue returns the channel width
func (p *PriceChannel) GetValue() float64 {
	return p.upperChannel - p.lowerChannel
}

// GetUpperChannel returns the upper channel (highest high)
func (p *PriceChannel) GetUpperChannel() float64 {
	return p.upperChannel
}

// GetLowerChannel returns the lower channel (lowest low)
func (p *PriceChannel) GetLowerChannel() float64 {
	return p.lowerChannel
}

// GetChannelWidth returns the width of the channel
func (p *PriceChannel) GetChannelWidth() float64 {
	return p.upperChannel - p.lowerChannel
}

// Reset resets the indicator
func (p *PriceChannel) Reset() {
	p.BaseIndicator.Reset()
	p.highs = p.highs[:0]
	p.lows = p.lows[:0]
	p.upperChannel = 0
	p.lowerChannel = 0
}

// IsReady returns true if the indicator has enough data
func (p *PriceChannel) IsReady() bool {
	return len(p.highs) >= p.period
}

// GetPeriod returns the period
func (p *PriceChannel) GetPeriod() int {
	return p.period
}

// IsBreakoutUp checks if price broke above upper channel
func (p *PriceChannel) IsBreakoutUp(currentPrice float64) bool {
	return currentPrice > p.upperChannel
}

// IsBreakoutDown checks if price broke below lower channel
func (p *PriceChannel) IsBreakoutDown(currentPrice float64) bool {
	return currentPrice < p.lowerChannel
}

// GetPosition returns the position of price within the channel
// Returns 0-1, where 0=lower channel, 1=upper channel
func (p *PriceChannel) GetPosition(currentPrice float64) float64 {
	width := p.GetChannelWidth()
	if width == 0 {
		return 0.5
	}

	position := (currentPrice - p.lowerChannel) / width

	// Clamp to [0, 1] range
	if position < 0 {
		position = 0
	} else if position > 1 {
		position = 1
	}

	return position
}
