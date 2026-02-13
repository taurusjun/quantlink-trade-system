package shm

import (
	"fmt"
	"sync/atomic"
	"unsafe"
)

// ClientStore wraps LocklessShmClientStore<uint64_t> from hftbase.
// SHM layout: [atomic<int64> data][int64 firstClientId] = 16 bytes total
type ClientStore struct {
	seg  *ShmSegment
	data *int64 // pointer to atomic data counter in SHM
}

// NewClientStore attaches to an existing ClientStore SHM segment.
func NewClientStore(shmKey int) (*ClientStore, error) {
	seg, err := ShmOpen(shmKey, int(unsafe.Sizeof(ClientData{})))
	if err != nil {
		return nil, fmt.Errorf("ClientStore: ShmOpen(key=0x%x): %w", shmKey, err)
	}
	return &ClientStore{
		seg:  seg,
		data: (*int64)(unsafe.Pointer(seg.Addr)),
	}, nil
}

// NewClientStoreCreate creates a new ClientStore SHM segment (for tests).
func NewClientStoreCreate(shmKey int, initialValue int64) (*ClientStore, error) {
	seg, err := ShmCreate(shmKey, int(unsafe.Sizeof(ClientData{})))
	if err != nil {
		return nil, fmt.Errorf("ClientStore: ShmCreate(key=0x%x): %w", shmKey, err)
	}
	cs := &ClientStore{
		seg:  seg,
		data: (*int64)(unsafe.Pointer(seg.Addr)),
	}
	// Initialize: data = initialValue, firstClientId = initialValue
	atomic.StoreInt64(cs.data, initialValue)
	firstPtr := (*int64)(unsafe.Pointer(seg.Addr + 8))
	*firstPtr = initialValue
	return cs, nil
}

// GetClientID returns the current client ID counter value.
// C++: m_data->data.load(memory_order_acquire)
func (cs *ClientStore) GetClientID() int64 {
	return atomic.LoadInt64(cs.data)
}

// GetClientIDAndIncrement atomically increments and returns the previous value.
// C++: m_data->data.fetch_add(1, memory_order_acq_rel)
func (cs *ClientStore) GetClientIDAndIncrement() int64 {
	return atomic.AddInt64(cs.data, 1) - 1
}

// GetFirstClientIDValue returns the initial client ID value.
// C++: m_data->firstCliendId
func (cs *ClientStore) GetFirstClientIDValue() int64 {
	firstPtr := (*int64)(unsafe.Pointer(cs.seg.Addr + 8))
	return *firstPtr
}

// Close detaches the SHM segment.
func (cs *ClientStore) Close() error {
	return cs.seg.Detach()
}

// Destroy detaches and removes the SHM segment.
func (cs *ClientStore) Destroy() error {
	if err := cs.seg.Detach(); err != nil {
		return err
	}
	return cs.seg.Remove()
}
