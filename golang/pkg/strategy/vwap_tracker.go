package strategy

import (
	"sync"
	"time"
)

// VWAPTracker tracks the Volume Weighted Average Price of executed trades
type VWAPTracker struct {
	mu sync.RWMutex

	// Execution tracking
	totalVolume      int64   // Total executed volume
	totalValue       float64 // Total value (price * volume)
	executedVWAP     float64 // Current executed VWAP

	// Trade history
	trades []Trade

	// Configuration
	trackingStartTime time.Time
	trackingEndTime   time.Time
}

// Trade represents a single executed trade
type Trade struct {
	Price     float64
	Volume    int64
	Timestamp time.Time
}

// NewVWAPTracker creates a new VWAP tracker
func NewVWAPTracker(startTime, endTime time.Time) *VWAPTracker {
	return &VWAPTracker{
		trades:            make([]Trade, 0, 1000),
		trackingStartTime: startTime,
		trackingEndTime:   endTime,
	}
}

// AddTrade adds a new executed trade to the tracker
func (vt *VWAPTracker) AddTrade(price float64, volume int64, timestamp time.Time) {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	// Add to trade history
	trade := Trade{
		Price:     price,
		Volume:    volume,
		Timestamp: timestamp,
	}
	vt.trades = append(vt.trades, trade)

	// Update totals
	vt.totalVolume += volume
	vt.totalValue += price * float64(volume)

	// Recalculate VWAP
	if vt.totalVolume > 0 {
		vt.executedVWAP = vt.totalValue / float64(vt.totalVolume)
	}
}

// GetVWAP returns the current executed VWAP
func (vt *VWAPTracker) GetVWAP() float64 {
	vt.mu.RLock()
	defer vt.mu.RUnlock()
	return vt.executedVWAP
}

// GetTotalVolume returns the total executed volume
func (vt *VWAPTracker) GetTotalVolume() int64 {
	vt.mu.RLock()
	defer vt.mu.RUnlock()
	return vt.totalVolume
}

// GetTotalValue returns the total executed value
func (vt *VWAPTracker) GetTotalValue() float64 {
	vt.mu.RLock()
	defer vt.mu.RUnlock()
	return vt.totalValue
}

// GetTradeCount returns the number of executed trades
func (vt *VWAPTracker) GetTradeCount() int {
	vt.mu.RLock()
	defer vt.mu.RUnlock()
	return len(vt.trades)
}

// GetTrades returns a copy of all trades
func (vt *VWAPTracker) GetTrades() []Trade {
	vt.mu.RLock()
	defer vt.mu.RUnlock()

	result := make([]Trade, len(vt.trades))
	copy(result, vt.trades)
	return result
}

// CalculateDeviation calculates the deviation from a target VWAP
// Returns the absolute deviation and percentage deviation
func (vt *VWAPTracker) CalculateDeviation(targetVWAP float64) (float64, float64) {
	vt.mu.RLock()
	defer vt.mu.RUnlock()

	if vt.executedVWAP == 0 || targetVWAP == 0 {
		return 0, 0
	}

	absoluteDeviation := vt.executedVWAP - targetVWAP
	percentageDeviation := (absoluteDeviation / targetVWAP) * 100.0

	return absoluteDeviation, percentageDeviation
}

// GetAverageTradeSize returns the average trade size
func (vt *VWAPTracker) GetAverageTradeSize() float64 {
	vt.mu.RLock()
	defer vt.mu.RUnlock()

	if len(vt.trades) == 0 {
		return 0
	}

	return float64(vt.totalVolume) / float64(len(vt.trades))
}

// GetExecutionRate returns the execution rate (volume per second)
func (vt *VWAPTracker) GetExecutionRate() float64 {
	vt.mu.RLock()
	defer vt.mu.RUnlock()

	if len(vt.trades) == 0 {
		return 0
	}

	firstTrade := vt.trades[0]
	lastTrade := vt.trades[len(vt.trades)-1]
	duration := lastTrade.Timestamp.Sub(firstTrade.Timestamp).Seconds()

	if duration == 0 {
		return 0
	}

	return float64(vt.totalVolume) / duration
}

// Reset resets the tracker to initial state
func (vt *VWAPTracker) Reset() {
	vt.mu.Lock()
	defer vt.mu.Unlock()

	vt.totalVolume = 0
	vt.totalValue = 0
	vt.executedVWAP = 0
	vt.trades = vt.trades[:0]
}

// GetStatistics returns execution statistics
func (vt *VWAPTracker) GetStatistics() *VWAPStatistics {
	vt.mu.RLock()
	defer vt.mu.RUnlock()

	stats := &VWAPStatistics{
		TotalVolume:      vt.totalVolume,
		TotalValue:       vt.totalValue,
		ExecutedVWAP:     vt.executedVWAP,
		TradeCount:       len(vt.trades),
		AverageTradeSize: vt.GetAverageTradeSize(),
		ExecutionRate:    vt.GetExecutionRate(),
	}

	// Calculate price range
	if len(vt.trades) > 0 {
		minPrice := vt.trades[0].Price
		maxPrice := vt.trades[0].Price

		for _, trade := range vt.trades {
			if trade.Price < minPrice {
				minPrice = trade.Price
			}
			if trade.Price > maxPrice {
				maxPrice = trade.Price
			}
		}

		stats.MinPrice = minPrice
		stats.MaxPrice = maxPrice
		stats.PriceRange = maxPrice - minPrice
	}

	// Calculate time span
	if len(vt.trades) > 1 {
		stats.StartTime = vt.trades[0].Timestamp
		stats.EndTime = vt.trades[len(vt.trades)-1].Timestamp
		stats.Duration = stats.EndTime.Sub(stats.StartTime)
	}

	return stats
}

// VWAPStatistics contains execution statistics
type VWAPStatistics struct {
	TotalVolume      int64
	TotalValue       float64
	ExecutedVWAP     float64
	TradeCount       int
	AverageTradeSize float64
	ExecutionRate    float64 // Volume per second
	MinPrice         float64
	MaxPrice         float64
	PriceRange       float64
	StartTime        time.Time
	EndTime          time.Time
	Duration         time.Duration
}
