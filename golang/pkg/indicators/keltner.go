package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// KeltnerChannels (Keltner Channels) is a volatility-based indicator that uses
// EMA and ATR to create dynamic upper and lower bands around price.
//
// Components:
// - Middle Line: EMA of close price
// - Upper Channel: Middle + (ATR × multiplier)
// - Lower Channel: Middle - (ATR × multiplier)
//
// Default parameters:
// - EMA period: 20
// - ATR period: 10 (or same as EMA)
// - Multiplier: 2.0
//
// Interpretation:
// - Breakout above upper: Bullish signal
// - Breakout below lower: Bearish signal
// - Price near upper: Overbought
// - Price near lower: Oversold
// - Channel width indicates volatility
//
// Properties:
// - Similar to Bollinger Bands but uses ATR instead of standard deviation
// - Less sensitive to price spikes
// - Better for trending markets
// - Multiplier controls channel width
type KeltnerChannels struct {
	*BaseIndicator
	emaPeriod    int
	atrPeriod    int
	multiplier   float64
	ema          *EMA
	atr          *ATR
	upperChannel float64
	lowerChannel float64
	middleLine   float64
}

// NewKeltnerChannels creates a new Keltner Channels indicator
func NewKeltnerChannels(emaPeriod int, atrPeriod int, multiplier float64, maxHistory int) *KeltnerChannels {
	if emaPeriod <= 0 {
		emaPeriod = 20
	}
	if atrPeriod <= 0 {
		atrPeriod = 10
	}
	if multiplier <= 0 {
		multiplier = 2.0
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &KeltnerChannels{
		BaseIndicator: NewBaseIndicator("Keltner Channels", maxHistory),
		emaPeriod:     emaPeriod,
		atrPeriod:     atrPeriod,
		multiplier:    multiplier,
		ema:           NewEMA(emaPeriod, maxHistory),
		atr:           NewATR(float64(atrPeriod), maxHistory),
	}
}

// NewKeltnerChannelsFromConfig creates Keltner Channels from configuration
func NewKeltnerChannelsFromConfig(config map[string]interface{}) (Indicator, error) {
	emaPeriod := 20
	atrPeriod := 10
	multiplier := 2.0
	maxHistory := 1000

	if v, ok := config["ema_period"]; ok {
		if p, ok := v.(float64); ok {
			emaPeriod = int(p)
		}
	}

	if v, ok := config["atr_period"]; ok {
		if p, ok := v.(float64); ok {
			atrPeriod = int(p)
		}
	}

	if v, ok := config["multiplier"]; ok {
		if m, ok := v.(float64); ok {
			multiplier = m
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewKeltnerChannels(emaPeriod, atrPeriod, multiplier, maxHistory), nil
}

// Update updates the indicator with new market data
func (k *KeltnerChannels) Update(md *mdpb.MarketDataUpdate) {
	// Update EMA and ATR
	k.ema.Update(md)
	k.atr.Update(md)

	// Need both EMA and ATR ready
	if !k.ema.IsReady() || !k.atr.IsReady() {
		return
	}

	// Get EMA value (middle line)
	k.middleLine = k.ema.GetValue()

	// Get ATR value
	atrValue := k.atr.GetValue()

	// Calculate upper and lower channels
	k.upperChannel = k.middleLine + (atrValue * k.multiplier)
	k.lowerChannel = k.middleLine - (atrValue * k.multiplier)

	// Store middle line as main value
	k.AddValue(k.middleLine)
}

// GetValue returns the middle line (EMA)
func (k *KeltnerChannels) GetValue() float64 {
	return k.middleLine
}

// GetUpperChannel returns the upper channel
func (k *KeltnerChannels) GetUpperChannel() float64 {
	return k.upperChannel
}

// GetLowerChannel returns the lower channel
func (k *KeltnerChannels) GetLowerChannel() float64 {
	return k.lowerChannel
}

// GetMiddleLine returns the middle line (EMA)
func (k *KeltnerChannels) GetMiddleLine() float64 {
	return k.middleLine
}

// GetChannelWidth returns the width of the channel
func (k *KeltnerChannels) GetChannelWidth() float64 {
	return k.upperChannel - k.lowerChannel
}

// GetChannelWidthPercentage returns the channel width as percentage of middle line
func (k *KeltnerChannels) GetChannelWidthPercentage() float64 {
	if k.middleLine == 0 {
		return 0
	}
	return (k.GetChannelWidth() / k.middleLine) * 100.0
}

// Reset resets the indicator
func (k *KeltnerChannels) Reset() {
	k.BaseIndicator.Reset()
	k.ema.Reset()
	k.atr.Reset()
	k.upperChannel = 0
	k.lowerChannel = 0
	k.middleLine = 0
}

// IsReady returns true if the indicator has enough data
func (k *KeltnerChannels) IsReady() bool {
	return k.ema.IsReady() && k.atr.IsReady()
}

// GetEMAPeriod returns the EMA period
func (k *KeltnerChannels) GetEMAPeriod() int {
	return k.emaPeriod
}

// GetATRPeriod returns the ATR period
func (k *KeltnerChannels) GetATRPeriod() int {
	return k.atrPeriod
}

// GetMultiplier returns the multiplier
func (k *KeltnerChannels) GetMultiplier() float64 {
	return k.multiplier
}

// IsBreakoutUp checks if price broke above upper channel
func (k *KeltnerChannels) IsBreakoutUp(currentPrice float64) bool {
	return currentPrice > k.upperChannel
}

// IsBreakoutDown checks if price broke below lower channel
func (k *KeltnerChannels) IsBreakoutDown(currentPrice float64) bool {
	return currentPrice < k.lowerChannel
}

// GetPosition returns the position of price within the channel
// Returns 0-1, where 0=lower channel, 1=upper channel, 0.5=middle
func (k *KeltnerChannels) GetPosition(currentPrice float64) float64 {
	width := k.GetChannelWidth()
	if width == 0 {
		return 0.5
	}

	position := (currentPrice - k.lowerChannel) / width

	// Clamp to [0, 1] range
	if position < 0 {
		position = 0
	} else if position > 1 {
		position = 1
	}

	return position
}

// IsBandSqueeze checks if bands are squeezing (low volatility)
// Returns true if channel width percentage < threshold (default 2%)
func (k *KeltnerChannels) IsBandSqueeze(thresholdPercent float64) bool {
	if thresholdPercent == 0 {
		thresholdPercent = 2.0
	}
	return k.GetChannelWidthPercentage() < thresholdPercent
}

// IsBandExpansion checks if bands are expanding (high volatility)
// Returns true if channel width percentage > threshold (default 5%)
func (k *KeltnerChannels) IsBandExpansion(thresholdPercent float64) bool {
	if thresholdPercent == 0 {
		thresholdPercent = 5.0
	}
	return k.GetChannelWidthPercentage() > thresholdPercent
}

// GetSignal returns trading signal based on channel position
// Returns 1 for buy (near lower), -1 for sell (near upper), 0 for neutral
func (k *KeltnerChannels) GetSignal(currentPrice float64) int {
	position := k.GetPosition(currentPrice)

	if position < 0.2 {
		return 1 // Near lower channel - buy signal
	} else if position > 0.8 {
		return -1 // Near upper channel - sell signal
	}

	return 0 // Neutral
}

// GetBandWidth returns the ATR-based bandwidth
// Useful for volatility analysis
func (k *KeltnerChannels) GetBandWidth() float64 {
	return k.atr.GetValue() * k.multiplier
}
