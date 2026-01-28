package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// MFI (Money Flow Index) is a volume-weighted momentum indicator that measures
// buying and selling pressure. It is often called "volume-weighted RSI".
//
// Calculation:
// 1. Typical Price (TP) = (High + Low + Close) / 3
// 2. Raw Money Flow (RMF) = TP × Volume
// 3. If TP > TP_prev: Positive Money Flow, else: Negative Money Flow
// 4. Money Flow Ratio (MFR) = Sum(Positive MF, period) / Sum(Negative MF, period)
// 5. MFI = 100 - (100 / (1 + MFR))
//
// Range: 0 to 100
// - MFI > 80: Overbought (potential sell signal)
// - MFI < 20: Oversold (potential buy signal)
// - Divergence from price can signal reversals
//
// Properties:
// - Volume-weighted version of RSI
// - More sensitive to volume changes
// - Good for detecting money flow reversals
// - Works best with liquid markets
type MFI struct {
	*BaseIndicator
	period           int
	typicalPrices    []float64
	rawMoneyFlows    []float64
	positiveFlows    []float64
	negativeFlows    []float64
	mfi              float64
	prevTypicalPrice float64
}

// NewMFI creates a new MFI indicator
func NewMFI(period int, maxHistory int) *MFI {
	if period <= 0 {
		period = 14
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &MFI{
		BaseIndicator:    NewBaseIndicator("MFI", maxHistory),
		period:           period,
		typicalPrices:    make([]float64, 0, period+1),
		rawMoneyFlows:    make([]float64, 0, period+1),
		positiveFlows:    make([]float64, 0, period),
		negativeFlows:    make([]float64, 0, period),
	}
}

// NewMFIFromConfig creates MFI from configuration
func NewMFIFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 14
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

	return NewMFI(period, maxHistory), nil
}

// Update updates the indicator with new market data
func (m *MFI) Update(md *mdpb.MarketDataUpdate) {
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

	// Get volume
	volume := float64(md.TotalVolume)
	if volume == 0 && len(md.BidQty) > 0 && len(md.AskQty) > 0 {
		volume = float64(md.BidQty[0] + md.AskQty[0])
	}

	// If no volume, skip this update
	if volume == 0 {
		return
	}

	// Calculate Typical Price: (High + Low + Close) / 3
	typicalPrice := (high + low + close) / 3.0

	// Calculate Raw Money Flow: TP × Volume
	rawMoneyFlow := typicalPrice * volume

	// Store values
	m.typicalPrices = append(m.typicalPrices, typicalPrice)
	m.rawMoneyFlows = append(m.rawMoneyFlows, rawMoneyFlow)

	// Keep only period+1 values
	if len(m.typicalPrices) > m.period+1 {
		m.typicalPrices = m.typicalPrices[1:]
		m.rawMoneyFlows = m.rawMoneyFlows[1:]
	}

	// Need at least 2 values to calculate flow direction
	if len(m.typicalPrices) < 2 {
		return
	}

	// Determine if positive or negative money flow
	prevTP := m.typicalPrices[len(m.typicalPrices)-2]
	currentTP := m.typicalPrices[len(m.typicalPrices)-1]
	currentRMF := m.rawMoneyFlows[len(m.rawMoneyFlows)-1]

	if currentTP > prevTP {
		// Positive money flow
		m.positiveFlows = append(m.positiveFlows, currentRMF)
		m.negativeFlows = append(m.negativeFlows, 0)
	} else if currentTP < prevTP {
		// Negative money flow
		m.positiveFlows = append(m.positiveFlows, 0)
		m.negativeFlows = append(m.negativeFlows, currentRMF)
	} else {
		// No change - neutral
		m.positiveFlows = append(m.positiveFlows, 0)
		m.negativeFlows = append(m.negativeFlows, 0)
	}

	// Keep only period values
	if len(m.positiveFlows) > m.period {
		m.positiveFlows = m.positiveFlows[1:]
		m.negativeFlows = m.negativeFlows[1:]
	}

	// Need at least period values to calculate MFI
	if len(m.positiveFlows) < m.period {
		return
	}

	// Calculate sums
	var sumPositive, sumNegative float64
	for i := 0; i < m.period; i++ {
		sumPositive += m.positiveFlows[i]
		sumNegative += m.negativeFlows[i]
	}

	// Calculate Money Flow Ratio and MFI
	if sumNegative == 0 {
		// All positive flow - MFI = 100
		m.mfi = 100.0
	} else {
		mfr := sumPositive / sumNegative
		m.mfi = 100.0 - (100.0 / (1.0 + mfr))
	}

	m.AddValue(m.mfi)
}

// GetValue returns the current MFI value
func (m *MFI) GetValue() float64 {
	return m.mfi
}

// Reset resets the indicator
func (m *MFI) Reset() {
	m.BaseIndicator.Reset()
	m.typicalPrices = m.typicalPrices[:0]
	m.rawMoneyFlows = m.rawMoneyFlows[:0]
	m.positiveFlows = m.positiveFlows[:0]
	m.negativeFlows = m.negativeFlows[:0]
	m.mfi = 0
	m.prevTypicalPrice = 0
}

// IsReady returns true if the indicator has enough data
func (m *MFI) IsReady() bool {
	return len(m.positiveFlows) >= m.period
}

// GetPeriod returns the period
func (m *MFI) GetPeriod() int {
	return m.period
}

// IsOverbought returns true if MFI > 80
func (m *MFI) IsOverbought() bool {
	return m.mfi > 80.0
}

// IsOversold returns true if MFI < 20
func (m *MFI) IsOversold() bool {
	return m.mfi < 20.0
}

// GetSignal returns trading signal based on MFI level
// Returns 1 for buy signal (oversold), -1 for sell signal (overbought), 0 for neutral
func (m *MFI) GetSignal() int {
	if m.mfi < 20.0 {
		return 1 // Oversold - buy signal
	} else if m.mfi > 80.0 {
		return -1 // Overbought - sell signal
	}
	return 0 // Neutral
}

// IsBullishDivergence checks for bullish divergence
// Price making lower lows but MFI making higher lows
func (m *MFI) IsBullishDivergence(prices []float64) bool {
	if len(m.values) < 5 || len(prices) < 5 {
		return false
	}

	n := len(m.values)
	pn := len(prices)

	// Check recent trend
	mfiRecentLow := m.values[n-5]
	mfiCurrentLow := m.values[n-1]
	priceRecentLow := prices[pn-5]
	priceCurrentLow := prices[pn-1]

	// Bullish divergence: price making lower low, MFI making higher low
	return priceCurrentLow < priceRecentLow && mfiCurrentLow > mfiRecentLow
}

// IsBearishDivergence checks for bearish divergence
// Price making higher highs but MFI making lower highs
func (m *MFI) IsBearishDivergence(prices []float64) bool {
	if len(m.values) < 5 || len(prices) < 5 {
		return false
	}

	n := len(m.values)
	pn := len(prices)

	// Check recent trend
	mfiRecentHigh := m.values[n-5]
	mfiCurrentHigh := m.values[n-1]
	priceRecentHigh := prices[pn-5]
	priceCurrentHigh := prices[pn-1]

	// Bearish divergence: price making higher high, MFI making lower high
	return priceCurrentHigh > priceRecentHigh && mfiCurrentHigh < mfiRecentHigh
}
