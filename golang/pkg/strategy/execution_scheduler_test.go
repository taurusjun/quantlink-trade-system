package strategy

import (
	"errors"
	"sync"
	"testing"
	"time"
)

// TestNewExecutionScheduler tests scheduler creation
func TestNewExecutionScheduler(t *testing.T) {
	scheduler := NewExecutionScheduler(100 * time.Millisecond)

	if scheduler == nil {
		t.Fatal("Expected non-nil scheduler")
	}

	if scheduler.checkInterval != 100*time.Millisecond {
		t.Errorf("Expected check interval 100ms, got %v", scheduler.checkInterval)
	}

	if scheduler.maxRetries != 3 {
		t.Errorf("Expected max retries 3, got %d", scheduler.maxRetries)
	}

	if scheduler.running {
		t.Error("Expected scheduler to not be running initially")
	}
}

// TestExecutionSchedulerSetters tests configuration setters
func TestExecutionSchedulerSetters(t *testing.T) {
	scheduler := NewExecutionScheduler(100 * time.Millisecond)

	// Test max retries setter
	scheduler.SetMaxRetries(5)
	if scheduler.maxRetries != 5 {
		t.Errorf("Expected max retries 5, got %d", scheduler.maxRetries)
	}

	// Test retry delay setter
	scheduler.SetRetryDelay(2 * time.Second)
	if scheduler.retryDelay != 2*time.Second {
		t.Errorf("Expected retry delay 2s, got %v", scheduler.retryDelay)
	}

	// Test callback setter
	callback := func(slice *OrderSlice) error {
		return nil
	}
	scheduler.SetExecutionCallback(callback)

	if scheduler.onExecute == nil {
		t.Error("Expected callback to be set")
	}
}

// TestExecutionSchedulerAddSlices tests adding slices
func TestExecutionSchedulerAddSlices(t *testing.T) {
	scheduler := NewExecutionScheduler(100 * time.Millisecond)

	slices := []*OrderSlice{
		{SliceID: 1, Quantity: 100, Status: SliceStatusPending},
		{SliceID: 2, Quantity: 200, Status: SliceStatusPending},
		{SliceID: 3, Quantity: 300, Status: SliceStatusPending},
	}

	scheduler.AddSlices(slices)

	stats := scheduler.GetStatistics()
	if stats.TotalSlices != 3 {
		t.Errorf("Expected 3 total slices, got %d", stats.TotalSlices)
	}

	pending := scheduler.GetPendingSlices()
	if pending != 3 {
		t.Errorf("Expected 3 pending slices, got %d", pending)
	}
}

// TestExecutionSchedulerStartStop tests starting and stopping the scheduler
func TestExecutionSchedulerStartStop(t *testing.T) {
	scheduler := NewExecutionScheduler(100 * time.Millisecond)

	// Test start without callback
	err := scheduler.Start()
	if err == nil {
		t.Error("Expected error when starting without callback")
	}

	// Set callback
	scheduler.SetExecutionCallback(func(slice *OrderSlice) error {
		return nil
	})

	// Test start
	err = scheduler.Start()
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if !scheduler.IsRunning() {
		t.Error("Expected scheduler to be running")
	}

	// Test double start
	err = scheduler.Start()
	if err == nil {
		t.Error("Expected error when starting already running scheduler")
	}

	// Test stop
	scheduler.Stop()
	time.Sleep(50 * time.Millisecond)

	if scheduler.IsRunning() {
		t.Error("Expected scheduler to not be running")
	}
}

// TestExecutionSchedulerExecution tests slice execution
func TestExecutionSchedulerExecution(t *testing.T) {
	scheduler := NewExecutionScheduler(50 * time.Millisecond)

	var executedSlices []*OrderSlice
	var mu sync.Mutex

	// Set callback that tracks executed slices
	scheduler.SetExecutionCallback(func(slice *OrderSlice) error {
		mu.Lock()
		defer mu.Unlock()
		executedSlices = append(executedSlices, slice)
		return nil
	})

	// Create slices with past scheduled times (ready to execute immediately)
	now := time.Now()
	slices := []*OrderSlice{
		{SliceID: 1, Quantity: 100, ScheduledTime: now.Add(-10 * time.Second), Status: SliceStatusPending},
		{SliceID: 2, Quantity: 200, ScheduledTime: now.Add(-5 * time.Second), Status: SliceStatusPending},
		{SliceID: 3, Quantity: 300, ScheduledTime: now.Add(-1 * time.Second), Status: SliceStatusPending},
	}

	scheduler.AddSlices(slices)

	// Start scheduler
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// Wait for execution
	time.Sleep(200 * time.Millisecond)

	// Check executed slices
	mu.Lock()
	executedCount := len(executedSlices)
	mu.Unlock()

	if executedCount != 3 {
		t.Errorf("Expected 3 executed slices, got %d", executedCount)
	}

	// Check statistics
	stats := scheduler.GetStatistics()
	if stats.ExecutedSlices != 3 {
		t.Errorf("Expected 3 executed slices in stats, got %d", stats.ExecutedSlices)
	}

	if stats.FailedSlices != 0 {
		t.Errorf("Expected 0 failed slices, got %d", stats.FailedSlices)
	}

	// Check progress
	progress := scheduler.GetProgress()
	if progress != 1.0 {
		t.Errorf("Expected progress 1.0, got %.2f", progress)
	}
}

// TestExecutionSchedulerRetry tests retry logic
func TestExecutionSchedulerRetry(t *testing.T) {
	scheduler := NewExecutionScheduler(50 * time.Millisecond)
	scheduler.SetMaxRetries(2)
	scheduler.SetRetryDelay(10 * time.Millisecond)

	attemptCount := 0
	var mu sync.Mutex

	// Set callback that fails twice then succeeds
	scheduler.SetExecutionCallback(func(slice *OrderSlice) error {
		mu.Lock()
		defer mu.Unlock()
		attemptCount++
		if attemptCount < 3 {
			return errors.New("simulated failure")
		}
		return nil
	})

	// Create slice
	now := time.Now()
	slices := []*OrderSlice{
		{SliceID: 1, Quantity: 100, ScheduledTime: now.Add(-1 * time.Second), Status: SliceStatusPending},
	}

	scheduler.AddSlices(slices)

	// Start scheduler
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// Wait for execution with retries
	time.Sleep(300 * time.Millisecond)

	// Check attempt count
	mu.Lock()
	attempts := attemptCount
	mu.Unlock()

	if attempts != 3 {
		t.Errorf("Expected 3 attempts (1 initial + 2 retries), got %d", attempts)
	}

	// Check statistics
	stats := scheduler.GetStatistics()
	if stats.RetryCount != 2 {
		t.Errorf("Expected 2 retries in stats, got %d", stats.RetryCount)
	}

	if stats.ExecutedSlices != 1 {
		t.Errorf("Expected 1 executed slice, got %d", stats.ExecutedSlices)
	}
}

// TestExecutionSchedulerFailure tests permanent failure after max retries
func TestExecutionSchedulerFailure(t *testing.T) {
	scheduler := NewExecutionScheduler(50 * time.Millisecond)
	scheduler.SetMaxRetries(2)
	scheduler.SetRetryDelay(10 * time.Millisecond)

	// Set callback that always fails
	scheduler.SetExecutionCallback(func(slice *OrderSlice) error {
		return errors.New("permanent failure")
	})

	// Create slice
	now := time.Now()
	slices := []*OrderSlice{
		{SliceID: 1, Quantity: 100, ScheduledTime: now.Add(-1 * time.Second), Status: SliceStatusPending},
	}

	scheduler.AddSlices(slices)

	// Start scheduler
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// Wait for execution attempts
	time.Sleep(300 * time.Millisecond)

	// Check for error in error channel
	select {
	case err := <-scheduler.GetErrorChannel():
		if err == nil {
			t.Error("Expected error in error channel")
		}
	default:
		t.Error("Expected error in error channel")
	}

	// Check statistics
	stats := scheduler.GetStatistics()
	if stats.FailedSlices != 1 {
		t.Errorf("Expected 1 failed slice, got %d", stats.FailedSlices)
	}

	if stats.ExecutedSlices != 0 {
		t.Errorf("Expected 0 executed slices, got %d", stats.ExecutedSlices)
	}

	// Check slice status
	if slices[0].Status != SliceStatusCanceled {
		t.Errorf("Expected slice status Canceled, got %v", slices[0].Status)
	}
}

// TestExecutionSchedulerFutureSlices tests slices scheduled in the future
func TestExecutionSchedulerFutureSlices(t *testing.T) {
	scheduler := NewExecutionScheduler(50 * time.Millisecond)

	var executedSlices []*OrderSlice
	var mu sync.Mutex

	scheduler.SetExecutionCallback(func(slice *OrderSlice) error {
		mu.Lock()
		defer mu.Unlock()
		executedSlices = append(executedSlices, slice)
		return nil
	})

	// Create slices with future scheduled times
	now := time.Now()
	slices := []*OrderSlice{
		{SliceID: 1, Quantity: 100, ScheduledTime: now.Add(-10 * time.Millisecond), Status: SliceStatusPending}, // Past - execute immediately
		{SliceID: 2, Quantity: 200, ScheduledTime: now.Add(150 * time.Millisecond), Status: SliceStatusPending},  // Future
		{SliceID: 3, Quantity: 300, ScheduledTime: now.Add(300 * time.Millisecond), Status: SliceStatusPending},  // Future
	}

	scheduler.AddSlices(slices)

	// Start scheduler
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// Wait for first slice
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	count1 := len(executedSlices)
	mu.Unlock()

	if count1 != 1 {
		t.Errorf("After 100ms, expected 1 executed slice, got %d", count1)
	}

	// Wait for second slice
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	count2 := len(executedSlices)
	mu.Unlock()

	if count2 != 2 {
		t.Errorf("After 250ms, expected 2 executed slices, got %d", count2)
	}

	// Wait for third slice
	time.Sleep(150 * time.Millisecond)

	mu.Lock()
	count3 := len(executedSlices)
	mu.Unlock()

	if count3 != 3 {
		t.Errorf("After 400ms, expected 3 executed slices, got %d", count3)
	}
}

// TestExecutionSchedulerReset tests resetting the scheduler
func TestExecutionSchedulerReset(t *testing.T) {
	scheduler := NewExecutionScheduler(100 * time.Millisecond)

	slices := []*OrderSlice{
		{SliceID: 1, Quantity: 100, Status: SliceStatusPending},
		{SliceID: 2, Quantity: 200, Status: SliceStatusFilled},
	}

	scheduler.AddSlices(slices)

	// Reset
	scheduler.Reset()

	stats := scheduler.GetStatistics()
	if stats.TotalSlices != 0 {
		t.Errorf("Expected 0 total slices after reset, got %d", stats.TotalSlices)
	}

	pending := scheduler.GetPendingSlices()
	if pending != 0 {
		t.Errorf("Expected 0 pending slices after reset, got %d", pending)
	}
}

// TestExecutionSchedulerCancelPending tests canceling pending slices
func TestExecutionSchedulerCancelPending(t *testing.T) {
	scheduler := NewExecutionScheduler(100 * time.Millisecond)

	slices := []*OrderSlice{
		{SliceID: 1, Quantity: 100, Status: SliceStatusPending},
		{SliceID: 2, Quantity: 200, Status: SliceStatusPending},
		{SliceID: 3, Quantity: 300, Status: SliceStatusFilled},
	}

	scheduler.AddSlices(slices)

	// Cancel pending
	canceled := scheduler.CancelPendingSlices()
	if canceled != 2 {
		t.Errorf("Expected 2 canceled slices, got %d", canceled)
	}

	// Check statuses
	if slices[0].Status != SliceStatusCanceled {
		t.Error("Expected slice 1 to be canceled")
	}
	if slices[1].Status != SliceStatusCanceled {
		t.Error("Expected slice 2 to be canceled")
	}
	if slices[2].Status != SliceStatusFilled {
		t.Error("Expected slice 3 to remain filled")
	}
}

// TestExecutionSchedulerGetRemainingSlices tests getting remaining slices
func TestExecutionSchedulerGetRemainingSlices(t *testing.T) {
	scheduler := NewExecutionScheduler(100 * time.Millisecond)

	slices := []*OrderSlice{
		{SliceID: 1, Quantity: 100, Status: SliceStatusPending},
		{SliceID: 2, Quantity: 200, Status: SliceStatusFilled},
		{SliceID: 3, Quantity: 300, Status: SliceStatusPending},
		{SliceID: 4, Quantity: 400, Status: SliceStatusCanceled},
	}

	scheduler.AddSlices(slices)

	remaining := scheduler.GetRemainingSlices()
	if len(remaining) != 2 {
		t.Errorf("Expected 2 remaining slices, got %d", len(remaining))
	}

	// Check IDs
	ids := make(map[int]bool)
	for _, slice := range remaining {
		ids[slice.SliceID] = true
	}

	if !ids[1] || !ids[3] {
		t.Error("Expected slices 1 and 3 to be remaining")
	}
}

// TestExecutionSchedulerStatistics tests statistics calculation
func TestExecutionSchedulerStatistics(t *testing.T) {
	scheduler := NewExecutionScheduler(50 * time.Millisecond)

	var executionTimes []time.Duration
	var mu sync.Mutex

	scheduler.SetExecutionCallback(func(slice *OrderSlice) error {
		mu.Lock()
		defer mu.Unlock()

		// Simulate execution time
		delay := time.Duration(slice.SliceID) * 10 * time.Millisecond
		time.Sleep(delay)
		executionTimes = append(executionTimes, delay)

		return nil
	})

	// Create slices
	now := time.Now()
	slices := []*OrderSlice{
		{SliceID: 1, Quantity: 100, ScheduledTime: now.Add(-1 * time.Second), Status: SliceStatusPending},
		{SliceID: 2, Quantity: 200, ScheduledTime: now.Add(-1 * time.Second), Status: SliceStatusPending},
	}

	scheduler.AddSlices(slices)

	// Start scheduler
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// Wait for execution
	time.Sleep(300 * time.Millisecond)

	// Check statistics
	stats := scheduler.GetStatistics()

	if stats.TotalSlices != 2 {
		t.Errorf("Expected 2 total slices, got %d", stats.TotalSlices)
	}

	if stats.ExecutedSlices != 2 {
		t.Errorf("Expected 2 executed slices, got %d", stats.ExecutedSlices)
	}

	if stats.AverageLatency == 0 {
		t.Error("Expected non-zero average latency")
	}

	if stats.TotalExecutionTime == 0 {
		t.Error("Expected non-zero total execution time")
	}

	// Check progress
	progress := scheduler.GetProgress()
	if progress != 1.0 {
		t.Errorf("Expected progress 1.0, got %.2f", progress)
	}
}

// TestExecutionSchedulerConcurrency tests concurrent operations
func TestExecutionSchedulerConcurrency(t *testing.T) {
	scheduler := NewExecutionScheduler(50 * time.Millisecond)

	var executedCount int
	var mu sync.Mutex

	scheduler.SetExecutionCallback(func(slice *OrderSlice) error {
		mu.Lock()
		defer mu.Unlock()
		executedCount++
		return nil
	})

	// Create many slices
	now := time.Now()
	var slices []*OrderSlice
	for i := 1; i <= 20; i++ {
		slices = append(slices, &OrderSlice{
			SliceID:       i,
			Quantity:      int64(i * 100),
			ScheduledTime: now.Add(-1 * time.Second),
			Status:        SliceStatusPending,
		})
	}

	scheduler.AddSlices(slices)

	// Start scheduler
	err := scheduler.Start()
	if err != nil {
		t.Fatalf("Failed to start scheduler: %v", err)
	}
	defer scheduler.Stop()

	// Wait for execution
	time.Sleep(500 * time.Millisecond)

	// Check all slices executed
	mu.Lock()
	count := executedCount
	mu.Unlock()

	if count != 20 {
		t.Errorf("Expected 20 executed slices, got %d", count)
	}

	stats := scheduler.GetStatistics()
	if stats.ExecutedSlices != 20 {
		t.Errorf("Expected 20 executed slices in stats, got %d", stats.ExecutedSlices)
	}
}
