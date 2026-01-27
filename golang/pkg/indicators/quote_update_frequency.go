package indicators

import (
	"fmt"
	"sync"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// QuoteUpdateFrequency measures how frequently quotes are updated
// 报价更新频率：衡量报价更新的频率
//
// Tracks the rate of quote changes per unit time
// Higher frequency = more active market making
// Lower frequency = less active market
type QuoteUpdateFrequency struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	windowDuration time.Duration // Time window for frequency calculation

	// State tracking
	prevBidPrice   float64   // Previous bid price
	prevAskPrice   float64   // Previous ask price
	updateTimes    []int64   // Timestamps of quote updates (nanoseconds)

	// Current state
	frequency         float64 // Updates per second
	bidUpdateFreq     float64 // Bid-only update frequency
	askUpdateFreq     float64 // Ask-only update frequency
	avgUpdateInterval float64 // Average time between updates (ms)

	// Counters
	bidUpdates   int // Number of bid updates in window
	askUpdates   int // Number of ask updates in window
	totalUpdates int // Total updates in window
}

// NewQuoteUpdateFrequency creates a new Quote Update Frequency indicator
func NewQuoteUpdateFrequency(name string, windowDuration time.Duration, maxHistory int) *QuoteUpdateFrequency {
	if windowDuration <= 0 {
		windowDuration = 60 * time.Second // Default: 60 seconds
	}

	quf := &QuoteUpdateFrequency{
		BaseIndicator:  NewBaseIndicator(name, maxHistory),
		windowDuration: windowDuration,
		updateTimes:    make([]int64, 0, 1000),
	}

	return quf
}

// NewQuoteUpdateFrequencyFromConfig creates a QuoteUpdateFrequency from configuration
func NewQuoteUpdateFrequencyFromConfig(config map[string]interface{}) (Indicator, error) {
	name := "QuoteUpdateFrequency"
	if v, ok := config["name"]; ok {
		if sv, ok := v.(string); ok {
			name = sv
		}
	}
	windowDuration := 60 * time.Second
	if v, ok := config["window_duration_sec"]; ok {
		if fv, ok := v.(float64); ok {
			windowDuration = time.Duration(fv) * time.Second
		}
	}

	maxHistory := 1000
	if v, ok := config["max_history"]; ok {
		if fv, ok := v.(float64); ok {
			maxHistory = int(fv)
		}
	}

	return NewQuoteUpdateFrequency(name, windowDuration, maxHistory), nil
}

// Update updates the quote update frequency
func (quf *QuoteUpdateFrequency) Update(md *mdpb.MarketDataUpdate) {
	quf.mu.Lock()
	defer quf.mu.Unlock()

	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		quf.AddValue(quf.frequency)
		return
	}

	currentBid := md.BidPrice[0]
	currentAsk := md.AskPrice[0]

	if currentBid == 0 || currentAsk == 0 {
		quf.AddValue(quf.frequency)
		return
	}

	// Check if quote changed
	bidChanged := false
	askChanged := false

	if quf.prevBidPrice > 0 && currentBid != quf.prevBidPrice {
		bidChanged = true
		quf.bidUpdates++
	}

	if quf.prevAskPrice > 0 && currentAsk != quf.prevAskPrice {
		askChanged = true
		quf.askUpdates++
	}

	// Record update time if any quote changed
	if bidChanged || askChanged {
		currentTime := time.Now().UnixNano()
		quf.updateTimes = append(quf.updateTimes, currentTime)
		quf.totalUpdates++

		// Remove old timestamps outside window
		quf.cleanOldTimestamps(currentTime)
	}

	// Calculate frequencies
	quf.calculateFrequencies()

	// Update previous values
	quf.prevBidPrice = currentBid
	quf.prevAskPrice = currentAsk

	quf.AddValue(quf.frequency)
}

// cleanOldTimestamps removes timestamps outside the time window
func (quf *QuoteUpdateFrequency) cleanOldTimestamps(currentTime int64) {
	windowNanos := quf.windowDuration.Nanoseconds()
	cutoffTime := currentTime - windowNanos

	// Find first index within window
	firstValid := 0
	for i, t := range quf.updateTimes {
		if t >= cutoffTime {
			firstValid = i
			break
		}
	}

	// Remove old timestamps
	if firstValid > 0 {
		quf.updateTimes = quf.updateTimes[firstValid:]
	}

	// Recalculate counters
	// Note: This is approximate since we only track timestamps, not which side changed
	// For exact tracking, we'd need to store (timestamp, side) pairs
	quf.totalUpdates = len(quf.updateTimes)
}

// calculateFrequencies calculates update frequencies
func (quf *QuoteUpdateFrequency) calculateFrequencies() {
	if len(quf.updateTimes) < 2 {
		quf.frequency = 0
		quf.bidUpdateFreq = 0
		quf.askUpdateFreq = 0
		quf.avgUpdateInterval = 0
		return
	}

	// Calculate overall frequency (updates per second)
	timespanNanos := quf.updateTimes[len(quf.updateTimes)-1] - quf.updateTimes[0]
	if timespanNanos > 0 {
		timespanSeconds := float64(timespanNanos) / float64(time.Second)
		quf.frequency = float64(len(quf.updateTimes)) / timespanSeconds
	}

	// Calculate average interval between updates (milliseconds)
	if len(quf.updateTimes) > 1 {
		totalInterval := int64(0)
		for i := 1; i < len(quf.updateTimes); i++ {
			totalInterval += quf.updateTimes[i] - quf.updateTimes[i-1]
		}
		avgIntervalNanos := totalInterval / int64(len(quf.updateTimes)-1)
		quf.avgUpdateInterval = float64(avgIntervalNanos) / float64(time.Millisecond)
	}

	// Estimate bid/ask frequencies (approximate)
	// This is a simplified estimate; exact tracking would require more state
	if quf.totalUpdates > 0 {
		bidRatio := float64(quf.bidUpdates) / float64(quf.totalUpdates)
		askRatio := float64(quf.askUpdates) / float64(quf.totalUpdates)

		quf.bidUpdateFreq = quf.frequency * bidRatio
		quf.askUpdateFreq = quf.frequency * askRatio
	}
}

// GetFrequency returns overall update frequency (updates/second)
func (quf *QuoteUpdateFrequency) GetFrequency() float64 {
	quf.mu.RLock()
	defer quf.mu.RUnlock()
	return quf.frequency
}

// GetBidUpdateFrequency returns bid update frequency
func (quf *QuoteUpdateFrequency) GetBidUpdateFrequency() float64 {
	quf.mu.RLock()
	defer quf.mu.RUnlock()
	return quf.bidUpdateFreq
}

// GetAskUpdateFrequency returns ask update frequency
func (quf *QuoteUpdateFrequency) GetAskUpdateFrequency() float64 {
	quf.mu.RLock()
	defer quf.mu.RUnlock()
	return quf.askUpdateFreq
}

// GetAvgUpdateInterval returns average interval between updates (ms)
func (quf *QuoteUpdateFrequency) GetAvgUpdateInterval() float64 {
	quf.mu.RLock()
	defer quf.mu.RUnlock()
	return quf.avgUpdateInterval
}

// GetActivityLevel returns market activity level classification
func (quf *QuoteUpdateFrequency) GetActivityLevel() string {
	freq := quf.GetFrequency()

	if freq > 10.0 {
		return "VeryHigh" // >10 updates/sec
	} else if freq > 5.0 {
		return "High" // 5-10 updates/sec
	} else if freq > 1.0 {
		return "Medium" // 1-5 updates/sec
	} else if freq > 0.1 {
		return "Low" // 0.1-1 updates/sec
	}
	return "VeryLow" // <0.1 updates/sec
}

// IsActive returns true if market is actively updating
func (quf *QuoteUpdateFrequency) IsActive() bool {
	return quf.GetFrequency() > 1.0 // >1 update per second
}

// GetName returns indicator name
func (quf *QuoteUpdateFrequency) GetName() string {
	return quf.BaseIndicator.GetName()
}

// String returns a string representation
func (quf *QuoteUpdateFrequency) String() string {
	return fmt.Sprintf("QuoteUpdateFrequency(freq=%.2f/s, interval=%.1fms, level=%s, bid=%.2f/s, ask=%.2f/s)",
		quf.frequency, quf.avgUpdateInterval, quf.GetActivityLevel(), quf.bidUpdateFreq, quf.askUpdateFreq)
}
