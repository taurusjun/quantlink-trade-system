package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// DonchianChannels (Donchian Channels) is a trend-following indicator that forms
// an envelope around price action using the highest high and lowest low over a period.
//
// Components:
// - Upper Channel: Highest high over period
// - Lower Channel: Lowest low over period
// - Middle Line: (Upper + Lower) / 2
//
// Interpretation:
// - Breakout above upper channel: Bullish signal
// - Breakout below lower channel: Bearish signal
// - Price near upper: Strong uptrend
// - Price near lower: Strong downtrend
// - Narrow channels: Low volatility, potential breakout
// - Wide channels: High volatility
//
// Properties:
// - Simple and effective
// - Good for trend following
// - Widely used in turtle trading systems
// - Works well in trending markets
// - Can generate false signals in ranging markets
type DonchianChannels struct {
	*BaseIndicator
	period       int
	highs        []float64
	lows         []float64
	upperChannel float64
	lowerChannel float64
	middleLine   float64
}

// NewDonchianChannels creates a new Donchian Channels indicator
func NewDonchianChannels(period int, maxHistory int) *DonchianChannels {
	if period <= 0 {
		period = 20
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &DonchianChannels{
		BaseIndicator: NewBaseIndicator("Donchian Channels", maxHistory),
		period:        period,
		highs:         make([]float64, 0, period),
		lows:          make([]float64, 0, period),
	}
}

// NewDonchianChannelsFromConfig creates Donchian Channels from configuration
func NewDonchianChannelsFromConfig(config map[string]interface{}) (Indicator, error) {
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

	return NewDonchianChannels(period, maxHistory), nil
}

// Update updates the indicator with new market data
func (d *DonchianChannels) Update(md *mdpb.MarketDataUpdate) {
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
	d.highs = append(d.highs, high)
	d.lows = append(d.lows, low)

	// Keep only period values
	if len(d.highs) > d.period {
		d.highs = d.highs[1:]
		d.lows = d.lows[1:]
	}

	// Need at least period values
	if len(d.highs) < d.period {
		return
	}

	// Calculate upper channel (highest high)
	d.upperChannel = d.highs[0]
	for i := 1; i < len(d.highs); i++ {
		if d.highs[i] > d.upperChannel {
			d.upperChannel = d.highs[i]
		}
	}

	// Calculate lower channel (lowest low)
	d.lowerChannel = d.lows[0]
	for i := 1; i < len(d.lows); i++ {
		if d.lows[i] < d.lowerChannel {
			d.lowerChannel = d.lows[i]
		}
	}

	// Calculate middle line
	d.middleLine = (d.upperChannel + d.lowerChannel) / 2.0

	// Store middle line as the main value
	d.AddValue(d.middleLine)
}

// GetValue returns the middle line
func (d *DonchianChannels) GetValue() float64 {
	return d.middleLine
}

// GetUpperChannel returns the upper channel (highest high)
func (d *DonchianChannels) GetUpperChannel() float64 {
	return d.upperChannel
}

// GetLowerChannel returns the lower channel (lowest low)
func (d *DonchianChannels) GetLowerChannel() float64 {
	return d.lowerChannel
}

// GetMiddleLine returns the middle line
func (d *DonchianChannels) GetMiddleLine() float64 {
	return d.middleLine
}

// GetChannelWidth returns the width of the channel
func (d *DonchianChannels) GetChannelWidth() float64 {
	return d.upperChannel - d.lowerChannel
}

// GetChannelWidthPercentage returns the channel width as percentage of middle line
func (d *DonchianChannels) GetChannelWidthPercentage() float64 {
	if d.middleLine == 0 {
		return 0
	}
	return (d.GetChannelWidth() / d.middleLine) * 100.0
}

// Reset resets the indicator
func (d *DonchianChannels) Reset() {
	d.BaseIndicator.Reset()
	d.highs = d.highs[:0]
	d.lows = d.lows[:0]
	d.upperChannel = 0
	d.lowerChannel = 0
	d.middleLine = 0
}

// IsReady returns true if the indicator has enough data
func (d *DonchianChannels) IsReady() bool {
	return len(d.highs) >= d.period
}

// GetPeriod returns the period
func (d *DonchianChannels) GetPeriod() int {
	return d.period
}

// IsBreakoutUp checks if price broke above upper channel
func (d *DonchianChannels) IsBreakoutUp(currentPrice float64) bool {
	return currentPrice > d.upperChannel
}

// IsBreakoutDown checks if price broke below lower channel
func (d *DonchianChannels) IsBreakoutDown(currentPrice float64) bool {
	return currentPrice < d.lowerChannel
}

// GetPosition returns the position of price within the channel
// Returns 0-1, where 0=lower channel, 1=upper channel, 0.5=middle
func (d *DonchianChannels) GetPosition(currentPrice float64) float64 {
	width := d.GetChannelWidth()
	if width == 0 {
		return 0.5
	}

	position := (currentPrice - d.lowerChannel) / width

	// Clamp to [0, 1] range
	if position < 0 {
		position = 0
	} else if position > 1 {
		position = 1
	}

	return position
}

// IsNarrowChannel checks if channel is narrow (low volatility)
// Returns true if channel width percentage < threshold (default 2%)
func (d *DonchianChannels) IsNarrowChannel(thresholdPercent float64) bool {
	if thresholdPercent == 0 {
		thresholdPercent = 2.0
	}
	return d.GetChannelWidthPercentage() < thresholdPercent
}

// IsWideChannel checks if channel is wide (high volatility)
// Returns true if channel width percentage > threshold (default 5%)
func (d *DonchianChannels) IsWideChannel(thresholdPercent float64) bool {
	if thresholdPercent == 0 {
		thresholdPercent = 5.0
	}
	return d.GetChannelWidthPercentage() > thresholdPercent
}

// GetSignal returns trading signal based on channel position
// Returns 1 for buy (near lower), -1 for sell (near upper), 0 for neutral
func (d *DonchianChannels) GetSignal(currentPrice float64) int {
	position := d.GetPosition(currentPrice)

	if position < 0.2 {
		return 1 // Near lower channel - buy signal
	} else if position > 0.8 {
		return -1 // Near upper channel - sell signal
	}

	return 0 // Neutral
}
