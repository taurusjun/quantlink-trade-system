package indicators

import (
	"fmt"
	"math"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// ResilienceScore measures how quickly the orderbook recovers from changes
// 恢复能力评分：衡量订单簿从变化中恢复的速度
//
// Measures:
// - Depth replenishment rate
// - Spread recovery speed
// - Overall orderbook stability
type ResilienceScore struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	levels      int     // Number of levels to monitor
	windowSize  int     // Number of updates to calculate rate
	alpha       float64 // Smoothing factor for EMA

	// Historical tracking
	prevTotalDepth float64   // Previous total depth
	prevSpread     float64   // Previous spread
	depthChanges   []float64 // Recent depth changes
	spreadChanges  []float64 // Recent spread changes

	// Current state
	depthRecoveryRate  float64 // How fast depth is replenished
	spreadRecoveryRate float64 // How fast spread narrows
	stabilityScore     float64 // Overall stability (0-100)
}

// NewResilienceScore creates a new Resilience Score indicator
func NewResilienceScore(name string, levels int, windowSize int, maxHistory int) *ResilienceScore {
	if levels <= 0 {
		levels = 5
	}
	if windowSize <= 0 {
		windowSize = 10
	}

	rs := &ResilienceScore{
		BaseIndicator: NewBaseIndicator(name, maxHistory),
		levels:        levels,
		windowSize:    windowSize,
		alpha:         0.2, // Default smoothing
		depthChanges:  make([]float64, 0, windowSize),
		spreadChanges: make([]float64, 0, windowSize),
	}

	return rs
}

// NewResilienceScoreFromConfig creates a ResilienceScore from configuration
func NewResilienceScoreFromConfig(name string, config map[string]interface{}) (Indicator, error) {
	levels := 5
	if v, ok := config["levels"]; ok {
		if fv, ok := v.(float64); ok {
			levels = int(fv)
		}
	}

	windowSize := 10
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

	rs := NewResilienceScore(name, levels, windowSize, maxHistory)

	if v, ok := config["alpha"]; ok {
		if fv, ok := v.(float64); ok {
			rs.alpha = fv
		}
	}

	return rs, nil
}

// Update updates the resilience score
func (rs *ResilienceScore) Update(md *mdpb.MarketDataUpdate) {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	// Calculate current total depth
	totalDepth := 0.0
	for i := 0; i < rs.levels && i < len(md.BidQty); i++ {
		totalDepth += float64(md.BidQty[i])
	}
	for i := 0; i < rs.levels && i < len(md.AskQty); i++ {
		totalDepth += float64(md.AskQty[i])
	}

	// Calculate current spread
	spread := 0.0
	if len(md.BidPrice) > 0 && len(md.AskPrice) > 0 {
		spread = md.AskPrice[0] - md.BidPrice[0]
		if spread < 0 {
			spread = 0
		}
	}

	// Track changes if we have previous data
	if rs.prevTotalDepth > 0 {
		depthChange := totalDepth - rs.prevTotalDepth
		rs.trackDepthChange(depthChange)

		spreadChange := spread - rs.prevSpread
		rs.trackSpreadChange(spreadChange)

		// Calculate recovery rates
		rs.depthRecoveryRate = rs.calculateDepthRecoveryRate()
		rs.spreadRecoveryRate = rs.calculateSpreadRecoveryRate()

		// Calculate overall stability score
		rs.stabilityScore = rs.calculateStabilityScore()
	}

	// Update previous values
	rs.prevTotalDepth = totalDepth
	rs.prevSpread = spread

	rs.AddValue(rs.stabilityScore)
}

// trackDepthChange tracks depth change
func (rs *ResilienceScore) trackDepthChange(change float64) {
	rs.depthChanges = append(rs.depthChanges, change)
	if len(rs.depthChanges) > rs.windowSize {
		rs.depthChanges = rs.depthChanges[1:]
	}
}

// trackSpreadChange tracks spread change
func (rs *ResilienceScore) trackSpreadChange(change float64) {
	rs.spreadChanges = append(rs.spreadChanges, change)
	if len(rs.spreadChanges) > rs.windowSize {
		rs.spreadChanges = rs.spreadChanges[1:]
	}
}

// calculateDepthRecoveryRate calculates how fast depth recovers
func (rs *ResilienceScore) calculateDepthRecoveryRate() float64 {
	if len(rs.depthChanges) < 2 {
		return 0
	}

	// Count positive changes (depth increasing)
	positiveChanges := 0
	totalPositive := 0.0

	for _, change := range rs.depthChanges {
		if change > 0 {
			positiveChanges++
			totalPositive += change
		}
	}

	if positiveChanges == 0 {
		return 0
	}

	// Recovery rate = proportion of positive changes * average magnitude
	proportion := float64(positiveChanges) / float64(len(rs.depthChanges))
	avgMagnitude := totalPositive / float64(positiveChanges)

	return proportion * avgMagnitude
}

// calculateSpreadRecoveryRate calculates how fast spread narrows
func (rs *ResilienceScore) calculateSpreadRecoveryRate() float64 {
	if len(rs.spreadChanges) < 2 {
		return 0
	}

	// Count negative changes (spread narrowing)
	negativeChanges := 0
	totalNegative := 0.0

	for _, change := range rs.spreadChanges {
		if change < 0 {
			negativeChanges++
			totalNegative += math.Abs(change)
		}
	}

	if negativeChanges == 0 {
		return 0
	}

	// Recovery rate = proportion of negative changes * average magnitude
	proportion := float64(negativeChanges) / float64(len(rs.spreadChanges))
	avgMagnitude := totalNegative / float64(negativeChanges)

	return proportion * avgMagnitude
}

// calculateStabilityScore calculates overall stability (0-100)
func (rs *ResilienceScore) calculateStabilityScore() float64 {
	if len(rs.depthChanges) < 2 {
		return 50 // Neutral score
	}

	// Calculate depth volatility (lower is better)
	depthVol := rs.calculateVolatility(rs.depthChanges)

	// Calculate spread volatility (lower is better)
	spreadVol := rs.calculateVolatility(rs.spreadChanges)

	// Normalize volatilities to 0-100 scale (lower volatility = higher score)
	depthScore := math.Max(0, 100-depthVol*10)
	spreadScore := math.Max(0, 100-spreadVol*100)

	// Weighted average (depth matters more)
	score := 0.6*depthScore + 0.4*spreadScore

	return math.Max(0, math.Min(100, score))
}

// calculateVolatility calculates standard deviation
func (rs *ResilienceScore) calculateVolatility(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}

	// Calculate mean
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	mean := sum / float64(len(values))

	// Calculate variance
	variance := 0.0
	for _, v := range values {
		diff := v - mean
		variance += diff * diff
	}
	variance /= float64(len(values))

	return math.Sqrt(variance)
}

// GetStabilityScore returns the current stability score
func (rs *ResilienceScore) GetStabilityScore() float64 {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.stabilityScore
}

// GetDepthRecoveryRate returns the depth recovery rate
func (rs *ResilienceScore) GetDepthRecoveryRate() float64 {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.depthRecoveryRate
}

// GetSpreadRecoveryRate returns the spread recovery rate
func (rs *ResilienceScore) GetSpreadRecoveryRate() float64 {
	rs.mu.RLock()
	defer rs.mu.RUnlock()
	return rs.spreadRecoveryRate
}

// GetName returns indicator name
func (rs *ResilienceScore) GetName() string {
	return rs.BaseIndicator.GetName()
}

// String returns a string representation
func (rs *ResilienceScore) String() string {
	return fmt.Sprintf("ResilienceScore(stability=%.2f, depth_recovery=%.2f, spread_recovery=%.4f)",
		rs.stabilityScore, rs.depthRecoveryRate, rs.spreadRecoveryRate)
}
