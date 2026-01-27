package strategy

import (
	"fmt"
	"log"
	"sync"
	"time"

	mdpb "github.com/yourusername/quantlink-trade-system/pkg/proto/md"
)

// VWAPStrategy implements a VWAP (Volume Weighted Average Price) execution strategy
type VWAPStrategy struct {
	mu sync.RWMutex

	// Components
	tracker   *VWAPTracker
	slicer    *OrderSlicer
	scheduler *ExecutionScheduler

	// Configuration
	symbol        string
	totalQuantity int64
	side          string // "buy" or "sell"
	startTime     time.Time
	endTime       time.Time
	sliceMethod   SliceMethod

	// Optional configuration
	numSlices      int       // For time-weighted slicing
	volumeProfile  []float64 // For volume-weighted slicing
	checkInterval  time.Duration
	targetVWAP     float64 // Optional target VWAP for deviation tracking

	// State
	status      VWAPStrategyStatus
	startedAt   time.Time
	completedAt time.Time

	// Callbacks
	onSliceExecute func(slice *OrderSlice, price float64) error

	// Statistics
	stats *VWAPStrategyStatistics
}

// VWAPStrategyStatus represents the status of the VWAP strategy
type VWAPStrategyStatus int

const (
	VWAPStatusPending   VWAPStrategyStatus = iota // Not started
	VWAPStatusRunning                             // Currently executing
	VWAPStatusCompleted                           // Successfully completed
	VWAPStatusFailed                              // Failed
	VWAPStatusCanceled                            // Canceled by user
)

// VWAPStrategyStatistics contains VWAP strategy statistics
type VWAPStrategyStatistics struct {
	TotalQuantity      int64
	ExecutedQuantity   int64
	RemainingQuantity  int64
	ExecutedVWAP       float64
	TargetVWAP         float64
	VWAPDeviation      float64 // Absolute deviation
	VWAPDeviationPct   float64 // Percentage deviation
	TotalSlices        int
	ExecutedSlices     int
	FailedSlices       int
	ExecutionProgress  float64       // 0.0 to 1.0
	AverageSliceSize   float64
	ExecutionRate      float64       // Quantity per second
	ElapsedTime        time.Duration
	EstimatedRemaining time.Duration
}

// NewVWAPStrategy creates a new VWAP strategy
func NewVWAPStrategy(symbol string, totalQuantity int64, side string, startTime, endTime time.Time) *VWAPStrategy {
	return &VWAPStrategy{
		symbol:        symbol,
		totalQuantity: totalQuantity,
		side:          side,
		startTime:     startTime,
		endTime:       endTime,
		sliceMethod:   SliceMethodTimeWeighted,
		numSlices:     10,
		checkInterval: 100 * time.Millisecond,
		status:        VWAPStatusPending,
		stats: &VWAPStrategyStatistics{
			TotalQuantity:     totalQuantity,
			RemainingQuantity: totalQuantity,
		},
	}
}

// SetNumSlices sets the number of slices for time-weighted slicing
func (vs *VWAPStrategy) SetNumSlices(numSlices int) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.numSlices = numSlices
	vs.sliceMethod = SliceMethodTimeWeighted
}

// SetVolumeProfile sets the volume profile for volume-weighted slicing
func (vs *VWAPStrategy) SetVolumeProfile(profile []float64) error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	// Validate profile sums to ~1.0
	sum := 0.0
	for _, v := range profile {
		sum += v
	}
	if sum < 0.99 || sum > 1.01 {
		return fmt.Errorf("volume profile must sum to 1.0, got %.4f", sum)
	}

	vs.volumeProfile = profile
	vs.sliceMethod = SliceMethodVolumeWeighted
	return nil
}

// SetTargetVWAP sets the target VWAP for deviation tracking
func (vs *VWAPStrategy) SetTargetVWAP(targetVWAP float64) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.targetVWAP = targetVWAP
	vs.stats.TargetVWAP = targetVWAP
}

// SetCheckInterval sets the scheduler check interval
func (vs *VWAPStrategy) SetCheckInterval(interval time.Duration) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.checkInterval = interval
}

// SetSliceExecutionCallback sets the callback for slice execution
func (vs *VWAPStrategy) SetSliceExecutionCallback(callback func(slice *OrderSlice, price float64) error) {
	vs.mu.Lock()
	defer vs.mu.Unlock()
	vs.onSliceExecute = callback
}

// Initialize initializes the VWAP strategy components
func (vs *VWAPStrategy) Initialize() error {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if vs.tracker != nil || vs.slicer != nil || vs.scheduler != nil {
		return fmt.Errorf("strategy already initialized")
	}

	// Create VWAP tracker
	vs.tracker = NewVWAPTracker(vs.startTime, vs.endTime)

	// Create order slicer
	vs.slicer = NewOrderSlicer(vs.totalQuantity, vs.sliceMethod)

	// Slice the order
	var err error
	if vs.sliceMethod == SliceMethodTimeWeighted {
		err = vs.slicer.SliceByTime(vs.startTime, vs.endTime, vs.numSlices)
	} else {
		interval := vs.endTime.Sub(vs.startTime) / time.Duration(len(vs.volumeProfile))
		err = vs.slicer.SliceByVolumeProfile(vs.startTime, interval, vs.volumeProfile)
	}
	if err != nil {
		return fmt.Errorf("failed to slice order: %v", err)
	}

	// Create execution scheduler
	vs.scheduler = NewExecutionScheduler(vs.checkInterval)
	vs.scheduler.AddSlices(vs.slicer.GetSlices())

	// Update statistics
	vs.stats.TotalSlices = len(vs.slicer.GetSlices())

	return nil
}

// Start starts the VWAP strategy execution
func (vs *VWAPStrategy) Start() error {
	vs.mu.Lock()

	if vs.status != VWAPStatusPending {
		vs.mu.Unlock()
		return fmt.Errorf("strategy already started or completed")
	}

	if vs.onSliceExecute == nil {
		vs.mu.Unlock()
		return fmt.Errorf("slice execution callback not set")
	}

	// Set scheduler callback
	vs.scheduler.SetExecutionCallback(func(slice *OrderSlice) error {
		return vs.executeSlice(slice)
	})

	vs.status = VWAPStatusRunning
	vs.startedAt = time.Now()
	vs.mu.Unlock()

	// Start scheduler
	err := vs.scheduler.Start()
	if err != nil {
		vs.mu.Lock()
		vs.status = VWAPStatusFailed
		vs.mu.Unlock()
		return fmt.Errorf("failed to start scheduler: %v", err)
	}

	// Monitor execution in background
	go vs.monitorExecution()

	log.Printf("[VWAPStrategy] Started VWAP strategy for %s, quantity=%d, side=%s",
		vs.symbol, vs.totalQuantity, vs.side)

	return nil
}

// Stop stops the VWAP strategy execution
func (vs *VWAPStrategy) Stop() {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if vs.status != VWAPStatusRunning {
		return
	}

	vs.scheduler.Stop()
	vs.status = VWAPStatusCanceled
	vs.completedAt = time.Now()

	log.Printf("[VWAPStrategy] Stopped VWAP strategy for %s", vs.symbol)
}

// Cancel cancels all pending slices
func (vs *VWAPStrategy) Cancel() {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	if vs.status != VWAPStatusRunning {
		return
	}

	canceled := vs.scheduler.CancelPendingSlices()
	vs.status = VWAPStatusCanceled
	vs.completedAt = time.Now()

	log.Printf("[VWAPStrategy] Canceled %d pending slices for %s", canceled, vs.symbol)
}

// GetStatus returns the current strategy status
func (vs *VWAPStrategy) GetStatus() VWAPStrategyStatus {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.status
}

// GetStatistics returns the current strategy statistics
func (vs *VWAPStrategy) GetStatistics() *VWAPStrategyStatistics {
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	// Update statistics
	vs.updateStatistics()

	// Create a copy
	statsCopy := *vs.stats
	return &statsCopy
}

// UpdateMarketData updates the strategy with new market data
func (vs *VWAPStrategy) UpdateMarketData(md *mdpb.MarketDataUpdate) {
	// VWAP strategy doesn't need real-time market data updates
	// It executes based on schedule, not market conditions
	// Market data is used during actual execution via the callback
}

// executeSlice executes a single order slice
func (vs *VWAPStrategy) executeSlice(slice *OrderSlice) error {
	vs.mu.RLock()
	callback := vs.onSliceExecute
	vs.mu.RUnlock()

	// Call user callback to execute the slice
	// The callback should return the execution price
	err := callback(slice, 0) // Price will be determined by callback
	if err != nil {
		return err
	}

	// In a real implementation, the callback would provide the execution price
	// For now, we assume the callback updates the tracker
	// vs.tracker.AddTrade(price, slice.Quantity, time.Now())

	// Update slice status via slicer
	vs.slicer.UpdateSliceStatus(slice.SliceID, SliceStatusFilled)

	return nil
}

// RecordTrade records a trade execution for VWAP tracking
func (vs *VWAPStrategy) RecordTrade(price float64, volume int64, timestamp time.Time) {
	vs.mu.Lock()
	defer vs.mu.Unlock()

	vs.tracker.AddTrade(price, volume, timestamp)
	vs.updateStatistics()
}

// monitorExecution monitors the execution progress
func (vs *VWAPStrategy) monitorExecution() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		vs.mu.RLock()
		status := vs.status
		vs.mu.RUnlock()

		if status != VWAPStatusRunning {
			break
		}

		// Check if execution is complete
		if vs.scheduler.GetProgress() >= 1.0 {
			vs.mu.Lock()
			vs.status = VWAPStatusCompleted
			vs.completedAt = time.Now()
			vs.mu.Unlock()

			log.Printf("[VWAPStrategy] Completed VWAP strategy for %s", vs.symbol)
			break
		}

		<-ticker.C
	}
}

// updateStatistics updates the strategy statistics
func (vs *VWAPStrategy) updateStatistics() {
	// Update from tracker
	if vs.tracker != nil {
		vwapStats := vs.tracker.GetStatistics()
		vs.stats.ExecutedQuantity = vwapStats.TotalVolume
		vs.stats.ExecutedVWAP = vwapStats.ExecutedVWAP

		// Calculate deviation from target VWAP
		if vs.targetVWAP > 0 {
			abs, pct := vs.tracker.CalculateDeviation(vs.targetVWAP)
			vs.stats.VWAPDeviation = abs
			vs.stats.VWAPDeviationPct = pct
		}

		vs.stats.ExecutionRate = vwapStats.ExecutionRate
	}

	// Update from scheduler
	if vs.scheduler != nil {
		schedStats := vs.scheduler.GetStatistics()
		vs.stats.ExecutedSlices = schedStats.ExecutedSlices
		vs.stats.FailedSlices = schedStats.FailedSlices
		vs.stats.ExecutionProgress = vs.scheduler.GetProgress()
	}

	// Update from slicer
	if vs.slicer != nil {
		vs.stats.RemainingQuantity = vs.slicer.GetRemainingQuantity()
		vs.stats.AverageSliceSize = float64(vs.totalQuantity) / float64(vs.stats.TotalSlices)
	}

	// Calculate elapsed time
	if !vs.startedAt.IsZero() {
		if !vs.completedAt.IsZero() {
			vs.stats.ElapsedTime = vs.completedAt.Sub(vs.startedAt)
		} else {
			vs.stats.ElapsedTime = time.Since(vs.startedAt)
		}

		// Estimate remaining time
		if vs.stats.ExecutionProgress > 0 && vs.stats.ExecutionProgress < 1.0 {
			totalEstimated := time.Duration(float64(vs.stats.ElapsedTime) / vs.stats.ExecutionProgress)
			vs.stats.EstimatedRemaining = totalEstimated - vs.stats.ElapsedTime
		}
	}
}

// GetExecutedVWAP returns the executed VWAP
func (vs *VWAPStrategy) GetExecutedVWAP() float64 {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.tracker.GetVWAP()
}

// GetRemainingQuantity returns the remaining quantity to execute
func (vs *VWAPStrategy) GetRemainingQuantity() int64 {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.slicer.GetRemainingQuantity()
}

// GetProgress returns the execution progress (0.0 to 1.0)
func (vs *VWAPStrategy) GetProgress() float64 {
	vs.mu.RLock()
	defer vs.mu.RUnlock()
	return vs.scheduler.GetProgress()
}
