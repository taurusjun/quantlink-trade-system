package indicators

import (
	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// Stochastic (KD Indicator) is a momentum indicator comparing closing price
// to the price range over a given time period
//
// Components:
// - %K (Fast Stochastic): Current position within the range
// - %D (Slow Stochastic): Moving average of %K (signal line)
//
// Formula:
// %K = 100 × (Close - Lowest Low) / (Highest High - Lowest Low)
// %D = SMA(%K, smooth_period)
//
// Range: 0 to 100
// - Above 80: Overbought zone
// - Below 20: Oversold zone
// - %K crosses above %D: Bullish signal
// - %K crosses below %D: Bearish signal
//
// Properties:
// - Identifies overbought/oversold conditions
// - Generates crossover signals
// - Works best in ranging markets
type Stochastic struct {
	*BaseIndicator
	period       int       // Lookback period for %K
	smoothK      int       // Smoothing period for %K
	smoothD      int       // Smoothing period for %D (SMA of %K)
	highs        []float64 // High prices window
	lows         []float64 // Low prices window
	closes       []float64 // Close prices window
	kValues      []float64 // %K values for %D calculation
	percentK     float64   // Current %K value
	percentD     float64   // Current %D value
	prevK        float64   // Previous %K for crossover detection
	prevD        float64   // Previous %D for crossover detection
}

// NewStochastic creates a new Stochastic indicator
// period: lookback period for high/low (typically 14)
// smoothK: smoothing for %K (typically 3, use 1 for fast stochastic)
// smoothD: smoothing for %D (typically 3)
func NewStochastic(period int, smoothK int, smoothD int, maxHistory int) *Stochastic {
	if period <= 0 {
		period = 14
	}
	if smoothK <= 0 {
		smoothK = 3
	}
	if smoothD <= 0 {
		smoothD = 3
	}
	if maxHistory <= 0 {
		maxHistory = 1000
	}

	return &Stochastic{
		BaseIndicator: NewBaseIndicator("Stochastic", maxHistory),
		period:        period,
		smoothK:       smoothK,
		smoothD:       smoothD,
		highs:         make([]float64, 0, period),
		lows:          make([]float64, 0, period),
		closes:        make([]float64, 0, period),
		kValues:       make([]float64, 0, smoothD),
	}
}

// NewStochasticFromConfig creates Stochastic from configuration
func NewStochasticFromConfig(config map[string]interface{}) (Indicator, error) {
	period := 14
	smoothK := 3
	smoothD := 3
	maxHistory := 1000

	if v, ok := config["period"]; ok {
		if p, ok := v.(float64); ok {
			period = int(p)
		}
	}

	if v, ok := config["smooth_k"]; ok {
		if k, ok := v.(float64); ok {
			smoothK = int(k)
		}
	}

	if v, ok := config["smooth_d"]; ok {
		if d, ok := v.(float64); ok {
			smoothD = int(d)
		}
	}

	if v, ok := config["max_history"]; ok {
		if h, ok := v.(float64); ok {
			maxHistory = int(h)
		}
	}

	return NewStochastic(period, smoothK, smoothD, maxHistory), nil
}

// Update updates the indicator with new market data
func (s *Stochastic) Update(md *mdpb.MarketDataUpdate) {
	// Use mid price as close
	close := GetMidPrice(md)
	if close <= 0 {
		return
	}

	// For high/low, use bid/ask extremes
	high := close
	low := close
	if len(md.AskPrice) > 0 && md.AskPrice[0] > high {
		high = md.AskPrice[0]
	}
	if len(md.BidPrice) > 0 && md.BidPrice[0] < low {
		low = md.BidPrice[0]
	}

	// Add to windows
	s.highs = append(s.highs, high)
	s.lows = append(s.lows, low)
	s.closes = append(s.closes, close)

	// Keep only period elements
	if len(s.highs) > s.period {
		s.highs = s.highs[1:]
		s.lows = s.lows[1:]
		s.closes = s.closes[1:]
	}

	// Need at least period values to calculate
	if len(s.closes) < s.period {
		return
	}

	// Calculate raw %K
	// %K = 100 × (Close - Lowest Low) / (Highest High - Lowest Low)
	highestHigh := s.highs[0]
	lowestLow := s.lows[0]

	for i := 1; i < len(s.highs); i++ {
		if s.highs[i] > highestHigh {
			highestHigh = s.highs[i]
		}
		if s.lows[i] < lowestLow {
			lowestLow = s.lows[i]
		}
	}

	currentClose := s.closes[len(s.closes)-1]
	denominator := highestHigh - lowestLow

	var rawK float64
	if denominator == 0 {
		rawK = 50.0 // Neutral when no range
	} else {
		rawK = 100.0 * (currentClose - lowestLow) / denominator
	}

	// Add raw %K to buffer for smoothing
	s.kValues = append(s.kValues, rawK)

	// Keep only smoothK elements for %K smoothing
	if len(s.kValues) > s.smoothK {
		s.kValues = s.kValues[len(s.kValues)-s.smoothK:]
	}

	// Calculate smoothed %K (SMA of raw %K)
	if len(s.kValues) >= s.smoothK {
		sum := 0.0
		for _, k := range s.kValues {
			sum += k
		}
		s.prevK = s.percentK
		s.percentK = sum / float64(len(s.kValues))

		// Store %K values for %D calculation
		s.AddValue(s.percentK)

		// Calculate %D (SMA of %K)
		values := s.GetValues()
		if len(values) >= s.smoothD {
			// Get last smoothD values
			recentK := values[len(values)-s.smoothD:]
			sumD := 0.0
			for _, k := range recentK {
				sumD += k
			}
			s.prevD = s.percentD
			s.percentD = sumD / float64(s.smoothD)
		}
	}
}

// GetValue returns the current %K value
func (s *Stochastic) GetValue() float64 {
	return s.percentK
}

// GetPercentK returns the current %K value
func (s *Stochastic) GetPercentK() float64 {
	return s.percentK
}

// GetPercentD returns the current %D value
func (s *Stochastic) GetPercentD() float64 {
	return s.percentD
}

// Reset resets the indicator
func (s *Stochastic) Reset() {
	s.BaseIndicator.Reset()
	s.highs = s.highs[:0]
	s.lows = s.lows[:0]
	s.closes = s.closes[:0]
	s.kValues = s.kValues[:0]
	s.percentK = 0
	s.percentD = 0
	s.prevK = 0
	s.prevD = 0
}

// IsReady returns true if the indicator has enough data
func (s *Stochastic) IsReady() bool {
	return len(s.closes) >= s.period && len(s.kValues) >= s.smoothK && len(s.GetValues()) >= s.smoothD
}

// GetPeriod returns the period
func (s *Stochastic) GetPeriod() int {
	return s.period
}

// IsOverbought returns true if %K indicates overbought (> 80)
func (s *Stochastic) IsOverbought() bool {
	return s.percentK > 80.0
}

// IsOversold returns true if %K indicates oversold (< 20)
func (s *Stochastic) IsOversold() bool {
	return s.percentK < 20.0
}

// IsBullishCrossover returns true if %K just crossed above %D (bullish signal)
func (s *Stochastic) IsBullishCrossover() bool {
	return s.prevK <= s.prevD && s.percentK > s.percentD
}

// IsBearishCrossover returns true if %K just crossed below %D (bearish signal)
func (s *Stochastic) IsBearishCrossover() bool {
	return s.prevK >= s.prevD && s.percentK < s.percentD
}
