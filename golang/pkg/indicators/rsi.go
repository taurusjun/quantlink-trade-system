// Package indicators provides technical indicators for trading
package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// RSI implements the Relative Strength Index indicator
type RSI struct {
	*BaseIndicator
	period     int
	gains      []float64
	losses     []float64
	avgGain    float64
	avgLoss    float64
	lastPrice  float64
	isInit     bool
	value      float64
}

// NewRSI creates a new RSI indicator
func NewRSI(period float64, maxHistory int) *RSI {
	return &RSI{
		BaseIndicator: &BaseIndicator{
			name:       "RSI",
			maxHistory: maxHistory,
		},
		period:  int(period),
		gains:   make([]float64, 0, maxHistory),
		losses:  make([]float64, 0, maxHistory),
		isInit:  false,
	}
}

// Update updates the RSI with new market data
func (rsi *RSI) Update(md *mdpb.MarketDataUpdate) {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return
	}

	currentPrice := (md.BidPrice[0] + md.AskPrice[0]) / 2.0

	if rsi.isInit {
		// Calculate price change
		change := currentPrice - rsi.lastPrice

		var gain, loss float64
		if change > 0 {
			gain = change
			loss = 0
		} else {
			gain = 0
			loss = -change
		}

		// Add to history
		rsi.gains = append(rsi.gains, gain)
		rsi.losses = append(rsi.losses, loss)

		// Limit history size
		if len(rsi.gains) > rsi.maxHistory {
			rsi.gains = rsi.gains[1:]
			rsi.losses = rsi.losses[1:]
		}

		// Calculate average gain/loss
		if len(rsi.gains) < rsi.period {
			// Not enough data yet, use simple average
			sumGain, sumLoss := 0.0, 0.0
			for i := 0; i < len(rsi.gains); i++ {
				sumGain += rsi.gains[i]
				sumLoss += rsi.losses[i]
			}
			rsi.avgGain = sumGain / float64(len(rsi.gains))
			rsi.avgLoss = sumLoss / float64(len(rsi.losses))
		} else if len(rsi.gains) == rsi.period {
			// First time we have enough data, calculate simple average
			sumGain, sumLoss := 0.0, 0.0
			for i := 0; i < rsi.period; i++ {
				sumGain += rsi.gains[i]
				sumLoss += rsi.losses[i]
			}
			rsi.avgGain = sumGain / float64(rsi.period)
			rsi.avgLoss = sumLoss / float64(rsi.period)
		} else {
			// Use smoothed moving average (Wilder's smoothing)
			rsi.avgGain = (rsi.avgGain*float64(rsi.period-1) + gain) / float64(rsi.period)
			rsi.avgLoss = (rsi.avgLoss*float64(rsi.period-1) + loss) / float64(rsi.period)
		}

		// Calculate RSI
		if rsi.avgLoss == 0 {
			rsi.value = 100.0
		} else {
			rs := rsi.avgGain / rsi.avgLoss
			rsi.value = 100.0 - (100.0 / (1.0 + rs))
		}
	}

	rsi.lastPrice = currentPrice
	rsi.isInit = true
}

// GetValue returns the current RSI value (0-100)
func (rsi *RSI) GetValue() float64 {
	return rsi.value
}

// GetValues returns the RSI history
func (rsi *RSI) GetValues() []float64 {
	if !rsi.IsReady() {
		return []float64{}
	}
	return []float64{rsi.value}
}

// IsReady returns true if the indicator has enough data
func (rsi *RSI) IsReady() bool {
	return rsi.isInit && len(rsi.gains) >= rsi.period
}

// Reset resets the indicator state
func (rsi *RSI) Reset() {
	rsi.gains = make([]float64, 0, rsi.maxHistory)
	rsi.losses = make([]float64, 0, rsi.maxHistory)
	rsi.avgGain = 0
	rsi.avgLoss = 0
	rsi.lastPrice = 0
	rsi.isInit = false
	rsi.value = 0
}

// NewRSIFromConfig creates an RSI indicator from config
func NewRSIFromConfig(config map[string]interface{}) (Indicator, error) {
	period, ok := config["period"].(float64)
	if !ok {
		period = 14.0 // Default RSI period
	}

	maxHistory, ok := config["max_history"].(float64)
	if !ok {
		maxHistory = 1000.0
	}

	return NewRSI(period, int(maxHistory)), nil
}
