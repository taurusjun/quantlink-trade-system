package strategy

import (
	"fmt"
	"sync"
	"time"
)

// ExecutionScheduler manages the scheduled execution of order slices
type ExecutionScheduler struct {
	mu sync.RWMutex

	// Order slices to execute
	slices []*OrderSlice

	// Execution callbacks
	onExecute func(*OrderSlice) error

	// Configuration
	checkInterval time.Duration // How often to check for ready slices
	maxRetries    int           // Maximum retry attempts per slice
	retryDelay    time.Duration // Delay between retries

	// State
	running      bool
	stopChan     chan struct{}
	executedChan chan *OrderSlice
	errorChan    chan error

	// Statistics
	stats *SchedulerStatistics
}

// SchedulerStatistics tracks execution scheduler statistics
type SchedulerStatistics struct {
	TotalSlices       int
	ExecutedSlices    int
	FailedSlices      int
	RetryCount        int
	AverageLatency    time.Duration
	TotalExecutionTime time.Duration
}

// NewExecutionScheduler creates a new execution scheduler
func NewExecutionScheduler(checkInterval time.Duration) *ExecutionScheduler {
	return &ExecutionScheduler{
		slices:        make([]*OrderSlice, 0),
		checkInterval: checkInterval,
		maxRetries:    3,
		retryDelay:    time.Second,
		stopChan:      make(chan struct{}),
		executedChan:  make(chan *OrderSlice, 100),
		errorChan:     make(chan error, 100),
		stats: &SchedulerStatistics{
			TotalSlices: 0,
		},
	}
}

// SetExecutionCallback sets the callback function for slice execution
func (es *ExecutionScheduler) SetExecutionCallback(callback func(*OrderSlice) error) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.onExecute = callback
}

// SetMaxRetries sets the maximum number of retry attempts
func (es *ExecutionScheduler) SetMaxRetries(maxRetries int) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.maxRetries = maxRetries
}

// SetRetryDelay sets the delay between retry attempts
func (es *ExecutionScheduler) SetRetryDelay(delay time.Duration) {
	es.mu.Lock()
	defer es.mu.Unlock()
	es.retryDelay = delay
}

// AddSlices adds order slices to the scheduler
func (es *ExecutionScheduler) AddSlices(slices []*OrderSlice) {
	es.mu.Lock()
	defer es.mu.Unlock()

	es.slices = append(es.slices, slices...)
	es.stats.TotalSlices = len(es.slices)
}

// Start starts the execution scheduler
func (es *ExecutionScheduler) Start() error {
	es.mu.Lock()
	if es.running {
		es.mu.Unlock()
		return fmt.Errorf("scheduler already running")
	}

	if es.onExecute == nil {
		es.mu.Unlock()
		return fmt.Errorf("execution callback not set")
	}

	es.running = true
	es.stopChan = make(chan struct{})
	es.mu.Unlock()

	// Start the scheduler loop
	go es.schedulerLoop()

	return nil
}

// Stop stops the execution scheduler
func (es *ExecutionScheduler) Stop() {
	es.mu.Lock()
	defer es.mu.Unlock()

	if !es.running {
		return
	}

	es.running = false
	close(es.stopChan)
}

// IsRunning returns whether the scheduler is running
func (es *ExecutionScheduler) IsRunning() bool {
	es.mu.RLock()
	defer es.mu.RUnlock()
	return es.running
}

// GetExecutedChannel returns the channel for executed slices
func (es *ExecutionScheduler) GetExecutedChannel() <-chan *OrderSlice {
	return es.executedChan
}

// GetErrorChannel returns the channel for execution errors
func (es *ExecutionScheduler) GetErrorChannel() <-chan error {
	return es.errorChan
}

// GetStatistics returns scheduler statistics
func (es *ExecutionScheduler) GetStatistics() *SchedulerStatistics {
	es.mu.RLock()
	defer es.mu.RUnlock()

	// Create a copy to avoid race conditions
	statsCopy := *es.stats
	return &statsCopy
}

// GetPendingSlices returns the number of pending slices
func (es *ExecutionScheduler) GetPendingSlices() int {
	es.mu.RLock()
	defer es.mu.RUnlock()

	count := 0
	for _, slice := range es.slices {
		if slice.Status == SliceStatusPending {
			count++
		}
	}
	return count
}

// schedulerLoop is the main scheduler loop
func (es *ExecutionScheduler) schedulerLoop() {
	ticker := time.NewTicker(es.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-es.stopChan:
			return

		case <-ticker.C:
			es.checkAndExecute()
		}
	}
}

// checkAndExecute checks for ready slices and executes them
func (es *ExecutionScheduler) checkAndExecute() {
	es.mu.Lock()
	now := time.Now()
	var readySlices []*OrderSlice

	// Find slices that are ready to execute
	for _, slice := range es.slices {
		if slice.Status == SliceStatusPending && !slice.ScheduledTime.After(now) {
			readySlices = append(readySlices, slice)
		}
	}
	es.mu.Unlock()

	// Execute ready slices
	for _, slice := range readySlices {
		es.executeSlice(slice)
	}
}

// executeSlice executes a single order slice with retry logic
func (es *ExecutionScheduler) executeSlice(slice *OrderSlice) {
	startTime := time.Now()

	// Mark as sent
	slice.Status = SliceStatusSent

	var lastErr error
	for attempt := 0; attempt <= es.maxRetries; attempt++ {
		// Call execution callback
		es.mu.RLock()
		callback := es.onExecute
		es.mu.RUnlock()

		err := callback(slice)
		if err == nil {
			// Execution successful
			slice.Status = SliceStatusFilled
			es.executedChan <- slice

			// Update statistics
			es.mu.Lock()
			es.stats.ExecutedSlices++
			executionTime := time.Since(startTime)
			es.stats.TotalExecutionTime += executionTime

			// Update average latency
			if es.stats.ExecutedSlices > 0 {
				es.stats.AverageLatency = es.stats.TotalExecutionTime / time.Duration(es.stats.ExecutedSlices)
			}
			es.mu.Unlock()

			return
		}

		lastErr = err

		// Update retry count
		es.mu.Lock()
		es.stats.RetryCount++
		es.mu.Unlock()

		// Wait before retry (except on last attempt)
		if attempt < es.maxRetries {
			time.Sleep(es.retryDelay)
		}
	}

	// All retries failed
	slice.Status = SliceStatusCanceled
	es.errorChan <- fmt.Errorf("slice %d failed after %d attempts: %v", slice.SliceID, es.maxRetries+1, lastErr)

	// Update statistics
	es.mu.Lock()
	es.stats.FailedSlices++
	es.mu.Unlock()
}

// GetProgress returns the execution progress (0.0 to 1.0)
func (es *ExecutionScheduler) GetProgress() float64 {
	es.mu.RLock()
	defer es.mu.RUnlock()

	if es.stats.TotalSlices == 0 {
		return 0.0
	}

	return float64(es.stats.ExecutedSlices) / float64(es.stats.TotalSlices)
}

// GetRemainingSlices returns slices that haven't been executed yet
func (es *ExecutionScheduler) GetRemainingSlices() []*OrderSlice {
	es.mu.RLock()
	defer es.mu.RUnlock()

	remaining := make([]*OrderSlice, 0)
	for _, slice := range es.slices {
		if slice.Status == SliceStatusPending {
			remaining = append(remaining, slice)
		}
	}
	return remaining
}

// Reset resets the scheduler to initial state
func (es *ExecutionScheduler) Reset() {
	es.mu.Lock()
	defer es.mu.Unlock()

	es.slices = es.slices[:0]
	es.stats = &SchedulerStatistics{
		TotalSlices: 0,
	}
}

// CancelPendingSlices cancels all pending slices
func (es *ExecutionScheduler) CancelPendingSlices() int {
	es.mu.Lock()
	defer es.mu.Unlock()

	count := 0
	for _, slice := range es.slices {
		if slice.Status == SliceStatusPending {
			slice.Status = SliceStatusCanceled
			count++
		}
	}

	return count
}
