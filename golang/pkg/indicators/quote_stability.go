package indicators

import (
	"fmt"
	"math"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// QuoteStability measures the stability of bid/ask quotes
// 报价稳定性：衡量买卖报价的稳定程度
//
// Tracks how frequently quotes change and by how much
// Higher stability = quotes change less frequently/magnitude
// Lower stability = volatile, frequently changing quotes
type QuoteStability struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	windowSize int // Number of updates to track

	// Historical tracking
	prevBidPrice  float64   // Previous bid price
	prevAskPrice  float64   // Previous ask price
	bidChanges    []float64 // Recent bid price changes
	askChanges    []float64 // Recent ask price changes

	// Change counters
	bidChangeCount int // Number of bid changes
	askChangeCount int // Number of ask changes
	totalUpdates   int // Total updates in window

	// Current state
	stabilityScore   float64 // Overall stability (0-100, higher = more stable)
	bidStability     float64 // Bid stability score
	askStability     float64 // Ask stability score
	changeFrequency  float64 // Frequency of quote changes
}

// NewQuoteStability creates a new Quote Stability indicator
func NewQuoteStability(name string, windowSize int, maxHistory int) *QuoteStability {
	if windowSize <= 0 {
		windowSize = 100
	}

	qs := &QuoteStability{
		BaseIndicator: NewBaseIndicator(name, maxHistory),
		windowSize:    windowSize,
		bidChanges:    make([]float64, 0, windowSize),
		askChanges:    make([]float64, 0, windowSize),
	}

	return qs
}

// NewQuoteStabilityFromConfig creates a QuoteStability from configuration
func NewQuoteStabilityFromConfig(config map[string]interface{}) (Indicator, error) {
	name := "QuoteStability"
	if v, ok := config["name"]; ok {
		if sv, ok := v.(string); ok {
			name = sv
		}
	}

	windowSize := 100
	if v, ok := config["window_size"]; ok {
		if fv, ok := v.(float64); ok {
			windowSize = int(fv)
		}
	}

	maxHistory := 1000
	if v, ok := config["max_history"]; ok {
		if fv, ok := v.(float64); ok {
			maxHistory = int(fv)
		}
	}

	return NewQuoteStability(name, windowSize, maxHistory), nil
}

// Update updates the quote stability
func (qs *QuoteStability) Update(md *mdpb.MarketDataUpdate) {
	qs.mu.Lock()
	defer qs.mu.Unlock()

	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		qs.AddValue(qs.stabilityScore)
		return
	}

	currentBid := md.BidPrice[0]
	currentAsk := md.AskPrice[0]

	if currentBid == 0 || currentAsk == 0 {
		qs.AddValue(qs.stabilityScore)
		return
	}

	// Track changes if we have previous data
	if qs.prevBidPrice > 0 && qs.prevAskPrice > 0 {
		bidChange := math.Abs(currentBid - qs.prevBidPrice)
		askChange := math.Abs(currentAsk - qs.prevAskPrice)

		// Add to rolling window
		qs.addChange(bidChange, askChange)

		// Update change counters
		if bidChange > 0 {
			qs.bidChangeCount++
		}
		if askChange > 0 {
			qs.askChangeCount++
		}
		qs.totalUpdates++

		// Remove oldest if window is full
		if qs.totalUpdates > qs.windowSize {
			qs.removeOldest()
		}

		// Calculate stability scores
		qs.calculateStability()
	}

	// Update previous values
	qs.prevBidPrice = currentBid
	qs.prevAskPrice = currentAsk

	qs.AddValue(qs.stabilityScore)
}

// addChange adds a price change to the rolling window
func (qs *QuoteStability) addChange(bidChange, askChange float64) {
	qs.bidChanges = append(qs.bidChanges, bidChange)
	qs.askChanges = append(qs.askChanges, askChange)
}

// removeOldest removes the oldest change from the window
func (qs *QuoteStability) removeOldest() {
	if len(qs.bidChanges) > 0 {
		oldBidChange := qs.bidChanges[0]
		oldAskChange := qs.askChanges[0]

		qs.bidChanges = qs.bidChanges[1:]
		qs.askChanges = qs.askChanges[1:]

		if oldBidChange > 0 {
			qs.bidChangeCount--
		}
		if oldAskChange > 0 {
			qs.askChangeCount--
		}
		qs.totalUpdates--
	}
}

// calculateStability calculates stability scores
func (qs *QuoteStability) calculateStability() {
	if qs.totalUpdates == 0 {
		qs.stabilityScore = 50 // Neutral
		qs.bidStability = 50
		qs.askStability = 50
		qs.changeFrequency = 0
		return
	}

	// 1. Change frequency (lower = more stable)
	bidFreq := float64(qs.bidChangeCount) / float64(qs.totalUpdates)
	askFreq := float64(qs.askChangeCount) / float64(qs.totalUpdates)
	qs.changeFrequency = (bidFreq + askFreq) / 2.0

	// 2. Average magnitude of changes (lower = more stable)
	bidMagnitude := qs.calculateAverage(qs.bidChanges)
	askMagnitude := qs.calculateAverage(qs.askChanges)

	// 3. Volatility of changes (lower = more stable)
	bidVolatility := qs.calculateStdDev(qs.bidChanges)
	askVolatility := qs.calculateStdDev(qs.askChanges)

	// Calculate stability scores (0-100, higher = more stable)
	// Less frequent changes + smaller magnitude + lower volatility = higher stability
	qs.bidStability = qs.scoreFromMetrics(bidFreq, bidMagnitude, bidVolatility)
	qs.askStability = qs.scoreFromMetrics(askFreq, askMagnitude, askVolatility)

	// Overall stability is the average
	qs.stabilityScore = (qs.bidStability + qs.askStability) / 2.0
}

// scoreFromMetrics converts metrics to a 0-100 stability score
func (qs *QuoteStability) scoreFromMetrics(frequency, magnitude, volatility float64) float64 {
	// Frequency component (0-33 points): less frequent = more points
	freqScore := (1.0 - frequency) * 33.0

	// Magnitude component (0-33 points): smaller changes = more points
	// Normalize magnitude (assume typical change is 0.1% of price)
	normalizedMag := math.Min(magnitude*10.0, 1.0)
	magScore := (1.0 - normalizedMag) * 33.0

	// Volatility component (0-34 points): lower volatility = more points
	normalizedVol := math.Min(volatility*10.0, 1.0)
	volScore := (1.0 - normalizedVol) * 34.0

	return freqScore + magScore + volScore
}

// calculateAverage calculates average of values
func (qs *QuoteStability) calculateAverage(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

// calculateStdDev calculates standard deviation
func (qs *QuoteStability) calculateStdDev(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	avg := qs.calculateAverage(values)
	variance := 0.0
	for _, v := range values {
		diff := v - avg
		variance += diff * diff
	}
	variance /= float64(len(values))

	return math.Sqrt(variance)
}

// GetStabilityScore returns overall stability score
func (qs *QuoteStability) GetStabilityScore() float64 {
	qs.mu.RLock()
	defer qs.mu.RUnlock()
	return qs.stabilityScore
}

// GetBidStability returns bid stability score
func (qs *QuoteStability) GetBidStability() float64 {
	qs.mu.RLock()
	defer qs.mu.RUnlock()
	return qs.bidStability
}

// GetAskStability returns ask stability score
func (qs *QuoteStability) GetAskStability() float64 {
	qs.mu.RLock()
	defer qs.mu.RUnlock()
	return qs.askStability
}

// GetChangeFrequency returns quote change frequency
func (qs *QuoteStability) GetChangeFrequency() float64 {
	qs.mu.RLock()
	defer qs.mu.RUnlock()
	return qs.changeFrequency
}

// GetStabilityLevel returns stability level classification
func (qs *QuoteStability) GetStabilityLevel() string {
	score := qs.GetStabilityScore()
	if score >= 80 {
		return "VeryStable"
	} else if score >= 60 {
		return "Stable"
	} else if score >= 40 {
		return "Moderate"
	} else if score >= 20 {
		return "Unstable"
	}
	return "VeryUnstable"
}

// GetName returns indicator name
func (qs *QuoteStability) GetName() string {
	return qs.BaseIndicator.GetName()
}

// String returns a string representation
func (qs *QuoteStability) String() string {
	return fmt.Sprintf("QuoteStability(score=%.2f, level=%s, bid=%.2f, ask=%.2f, freq=%.2f)",
		qs.stabilityScore, qs.GetStabilityLevel(), qs.bidStability, qs.askStability, qs.changeFrequency)
}
