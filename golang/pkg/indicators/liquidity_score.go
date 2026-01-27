package indicators

import (
	"fmt"
	"math"
	"sync"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// LiquidityScore calculates comprehensive liquidity score (0-100)
// 流动性综合评分：综合考虑深度、价差和交易活跃度
type LiquidityScore struct {
	*BaseIndicator
	mu sync.RWMutex

	// Configuration
	levels       int     // Number of price levels
	depthWeight  float64 // Weight for depth score
	spreadWeight float64 // Weight for spread score

	// Normalization parameters
	maxDepth  float64 // Maximum depth for normalization
	maxSpread float64 // Maximum relative spread for normalization

	// Current state
	lastScore float64 // Current liquidity score
}

// NewLiquidityScore creates a new Liquidity Score indicator
func NewLiquidityScore(name string, levels int, maxHistory int) *LiquidityScore {
	if levels <= 0 {
		levels = 5
	}

	ls := &LiquidityScore{
		BaseIndicator: NewBaseIndicator(name, maxHistory),
		levels:        levels,
		depthWeight:   0.6, // Default: depth contributes 60%
		spreadWeight:  0.4, // Default: spread contributes 40%
		maxDepth:      1000.0,
		maxSpread:     0.01, // 1% relative spread
	}

	return ls
}

// NewLiquidityScoreFromConfig creates a LiquidityScore from configuration
func NewLiquidityScoreFromConfig(name string, config map[string]interface{}) (Indicator, error) {
	levels := 5
	if v, ok := config["levels"]; ok {
		if fv, ok := v.(float64); ok {
			levels = int(fv)
		}
	}

	maxHistory := 1000
	if v, ok := config["max_history"]; ok {
		if fv, ok := v.(float64); ok {
			maxHistory = int(fv)
		}
	}

	ls := NewLiquidityScore(name, levels, maxHistory)

	// Optional overrides
	if v, ok := config["depth_weight"]; ok {
		if fv, ok := v.(float64); ok {
			ls.depthWeight = fv
		}
	}
	if v, ok := config["spread_weight"]; ok {
		if fv, ok := v.(float64); ok {
			ls.spreadWeight = fv
		}
	}
	if v, ok := config["max_depth"]; ok {
		if fv, ok := v.(float64); ok {
			ls.maxDepth = fv
		}
	}
	if v, ok := config["max_spread"]; ok {
		if fv, ok := v.(float64); ok {
			ls.maxSpread = fv
		}
	}

	// Normalize weights
	totalWeight := ls.depthWeight + ls.spreadWeight
	if totalWeight > 0 {
		ls.depthWeight /= totalWeight
		ls.spreadWeight /= totalWeight
	}

	return ls, nil
}

// Update updates the liquidity score
func (ls *LiquidityScore) Update(md *mdpb.MarketDataUpdate) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	// 1. Calculate depth score
	depthScore := ls.calculateDepthScore(md)

	// 2. Calculate spread score
	spreadScore := ls.calculateSpreadScore(md)

	// 3. Weighted average
	score := ls.depthWeight*depthScore + ls.spreadWeight*spreadScore

	// Ensure score is in [0, 100]
	score = math.Max(0, math.Min(100, score))

	ls.lastScore = score
	ls.AddValue(score)
}

// calculateDepthScore calculates depth component (0-100)
func (ls *LiquidityScore) calculateDepthScore(md *mdpb.MarketDataUpdate) float64 {
	if len(md.BidQty) == 0 || len(md.AskQty) == 0 {
		return 0
	}

	// Calculate total depth
	totalDepth := 0.0
	for i := 0; i < ls.levels && i < len(md.BidQty); i++ {
		totalDepth += float64(md.BidQty[i])
	}
	for i := 0; i < ls.levels && i < len(md.AskQty); i++ {
		totalDepth += float64(md.AskQty[i])
	}

	// Normalize to 0-100
	score := (totalDepth / ls.maxDepth) * 100.0
	return math.Min(100, score)
}

// calculateSpreadScore calculates spread component (0-100, smaller spread = higher score)
func (ls *LiquidityScore) calculateSpreadScore(md *mdpb.MarketDataUpdate) float64 {
	if len(md.BidPrice) == 0 || len(md.AskPrice) == 0 {
		return 0
	}

	bidPrice := md.BidPrice[0]
	askPrice := md.AskPrice[0]

	if bidPrice <= 0 || askPrice <= 0 {
		return 0
	}

	// Calculate relative spread
	spread := askPrice - bidPrice
	midPrice := (bidPrice + askPrice) / 2.0
	relativeSpread := spread / midPrice

	// Normalize to 0-100 (smaller spread = higher score)
	score := (1.0 - relativeSpread/ls.maxSpread) * 100.0
	return math.Max(0, math.Min(100, score))
}

// GetScore returns the current liquidity score
func (ls *LiquidityScore) GetScore() float64 {
	ls.mu.RLock()
	defer ls.mu.RUnlock()
	return ls.lastScore
}

// GetLevel returns liquidity level classification
func (ls *LiquidityScore) GetLevel() string {
	score := ls.GetScore()
	if score >= 80 {
		return "Excellent"
	} else if score >= 60 {
		return "Good"
	} else if score >= 40 {
		return "Fair"
	} else if score >= 20 {
		return "Poor"
	}
	return "VeryPoor"
}

// GetName returns indicator name
func (ls *LiquidityScore) GetName() string {
	return ls.BaseIndicator.GetName()
}

// String returns a string representation
func (ls *LiquidityScore) String() string {
	return fmt.Sprintf("LiquidityScore(score=%.2f, level=%s, weights=%.2f/%.2f)",
		ls.GetScore(), ls.GetLevel(), ls.depthWeight, ls.spreadWeight)
}
