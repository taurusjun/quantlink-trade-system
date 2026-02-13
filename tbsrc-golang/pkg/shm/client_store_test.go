package shm

import (
	"sync"
	"testing"
)

const testClientStoreKey = 0xBEEF10

func TestClientStoreBasic(t *testing.T) {
	cs, err := NewClientStoreCreate(testClientStoreKey, 100)
	if err != nil {
		t.Fatalf("NewClientStoreCreate: %v", err)
	}
	defer cs.Destroy()

	// Initial value
	if id := cs.GetClientID(); id != 100 {
		t.Errorf("GetClientID = %d, want 100", id)
	}

	if first := cs.GetFirstClientIDValue(); first != 100 {
		t.Errorf("GetFirstClientIDValue = %d, want 100", first)
	}

	// Increment
	id1 := cs.GetClientIDAndIncrement()
	if id1 != 100 {
		t.Errorf("first GetClientIDAndIncrement = %d, want 100", id1)
	}

	id2 := cs.GetClientIDAndIncrement()
	if id2 != 101 {
		t.Errorf("second GetClientIDAndIncrement = %d, want 101", id2)
	}

	// Current value should be 102
	if id := cs.GetClientID(); id != 102 {
		t.Errorf("GetClientID after 2 increments = %d, want 102", id)
	}
}

func TestClientStoreConcurrent(t *testing.T) {
	cs, err := NewClientStoreCreate(testClientStoreKey+1, 0)
	if err != nil {
		t.Fatalf("NewClientStoreCreate: %v", err)
	}
	defer cs.Destroy()

	numGoroutines := 10
	incrementsPerGoroutine := 100
	total := numGoroutines * incrementsPerGoroutine

	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	ids := make(chan int64, total)

	for g := 0; g < numGoroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < incrementsPerGoroutine; i++ {
				id := cs.GetClientIDAndIncrement()
				ids <- id
			}
		}()
	}

	wg.Wait()
	close(ids)

	// Verify all IDs are unique
	seen := make(map[int64]bool, total)
	for id := range ids {
		if seen[id] {
			t.Errorf("duplicate client ID: %d", id)
		}
		seen[id] = true
	}

	if len(seen) != total {
		t.Errorf("got %d unique IDs, want %d", len(seen), total)
	}

	// Final value should be total
	if id := cs.GetClientID(); id != int64(total) {
		t.Errorf("final GetClientID = %d, want %d", id, total)
	}
}
