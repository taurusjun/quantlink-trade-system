package strategy

import (
	"fmt"
	"time"
)

// OrderSlice represents a sliced portion of the parent order
type OrderSlice struct {
	SliceID       int
	Quantity      int64
	ScheduledTime time.Time
	Status        SliceStatus
}

// SliceStatus represents the status of an order slice
type SliceStatus int

const (
	SliceStatusPending SliceStatus = iota
	SliceStatusSent
	SliceStatusFilled
	SliceStatusCanceled
)

// OrderSlicer splits a large order into smaller slices based on time or volume profile
type OrderSlicer struct {
	totalQuantity int64
	slices        []*OrderSlice
	sliceMethod   SliceMethod
}

// SliceMethod determines how the order is sliced
type SliceMethod int

const (
	SliceMethodTimeWeighted   SliceMethod = iota // Evenly distributed over time
	SliceMethodVolumeWeighted                    // Based on historical volume profile
)

// NewOrderSlicer creates a new order slicer
func NewOrderSlicer(totalQuantity int64, method SliceMethod) *OrderSlicer {
	return &OrderSlicer{
		totalQuantity: totalQuantity,
		slices:        make([]*OrderSlice, 0),
		sliceMethod:   method,
	}
}

// SliceByTime slices the order evenly over a time period
func (os *OrderSlicer) SliceByTime(startTime, endTime time.Time, numSlices int) error {
	if numSlices <= 0 {
		return fmt.Errorf("number of slices must be positive")
	}

	if endTime.Before(startTime) {
		return fmt.Errorf("end time must be after start time")
	}

	duration := endTime.Sub(startTime)
	sliceInterval := duration / time.Duration(numSlices)
	sliceQuantity := os.totalQuantity / int64(numSlices)
	remainder := os.totalQuantity % int64(numSlices)

	for i := 0; i < numSlices; i++ {
		quantity := sliceQuantity
		// Add remainder to last slice
		if i == numSlices-1 {
			quantity += remainder
		}

		slice := &OrderSlice{
			SliceID:       i + 1,
			Quantity:      quantity,
			ScheduledTime: startTime.Add(sliceInterval * time.Duration(i)),
			Status:        SliceStatusPending,
		}

		os.slices = append(os.slices, slice)
	}

	return nil
}

// SliceByVolumeProfile slices the order based on historical volume distribution
// volumeProfile should sum to 1.0 and represent the percentage of volume at each interval
func (os *OrderSlicer) SliceByVolumeProfile(startTime time.Time, interval time.Duration, volumeProfile []float64) error {
	if len(volumeProfile) == 0 {
		return fmt.Errorf("volume profile cannot be empty")
	}

	// Validate volume profile sums to approximately 1.0
	sum := 0.0
	for _, v := range volumeProfile {
		sum += v
	}
	if sum < 0.99 || sum > 1.01 {
		return fmt.Errorf("volume profile must sum to 1.0, got %.2f", sum)
	}

	currentTime := startTime
	for i, volumePct := range volumeProfile {
		quantity := int64(float64(os.totalQuantity) * volumePct)

		// Ensure last slice gets any remainder
		if i == len(volumeProfile)-1 {
			executed := int64(0)
			for _, s := range os.slices {
				executed += s.Quantity
			}
			quantity = os.totalQuantity - executed
		}

		if quantity > 0 {
			slice := &OrderSlice{
				SliceID:       i + 1,
				Quantity:      quantity,
				ScheduledTime: currentTime,
				Status:        SliceStatusPending,
			}
			os.slices = append(os.slices, slice)
		}

		currentTime = currentTime.Add(interval)
	}

	return nil
}

// GetSlices returns all order slices
func (os *OrderSlicer) GetSlices() []*OrderSlice {
	return os.slices
}

// GetPendingSlices returns slices that haven't been sent yet
func (os *OrderSlicer) GetPendingSlices() []*OrderSlice {
	pending := make([]*OrderSlice, 0)
	for _, slice := range os.slices {
		if slice.Status == SliceStatusPending {
			pending = append(pending, slice)
		}
	}
	return pending
}

// GetSlicesAt returns slices scheduled for execution at or before the given time
func (os *OrderSlicer) GetSlicesAt(t time.Time) []*OrderSlice {
	result := make([]*OrderSlice, 0)
	for _, slice := range os.slices {
		if slice.Status == SliceStatusPending && !slice.ScheduledTime.After(t) {
			result = append(result, slice)
		}
	}
	return result
}

// UpdateSliceStatus updates the status of a slice
func (os *OrderSlicer) UpdateSliceStatus(sliceID int, status SliceStatus) error {
	for _, slice := range os.slices {
		if slice.SliceID == sliceID {
			slice.Status = status
			return nil
		}
	}
	return fmt.Errorf("slice ID %d not found", sliceID)
}

// GetProgress returns the execution progress (0.0 to 1.0)
func (os *OrderSlicer) GetProgress() float64 {
	if os.totalQuantity == 0 {
		return 0.0
	}

	executed := int64(0)
	for _, slice := range os.slices {
		if slice.Status == SliceStatusFilled {
			executed += slice.Quantity
		}
	}

	return float64(executed) / float64(os.totalQuantity)
}

// GetRemainingQuantity returns the quantity yet to be executed
func (os *OrderSlicer) GetRemainingQuantity() int64 {
	executed := int64(0)
	for _, slice := range os.slices {
		if slice.Status == SliceStatusFilled {
			executed += slice.Quantity
		}
	}
	return os.totalQuantity - executed
}
