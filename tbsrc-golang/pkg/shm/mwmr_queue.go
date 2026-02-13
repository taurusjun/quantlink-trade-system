package shm

import (
	"fmt"
	"sync/atomic"
	"unsafe"
)

// MWMRQueue is a generic multi-writer multi-reader SHM circular queue.
// It mirrors C++ MultiWriterMultiReaderShmQueue<T> from hftbase.
//
// SHM layout:
//   [MWMRHeader (8 bytes)][QueueElem[0]][QueueElem[1]]...[QueueElem[size-1]]
// Where QueueElem = [T data][uint64 seqNo]
//
// T must be one of: MarketUpdateNew, RequestMsg, ResponseMsg.
// The queue size is rounded up to the next power of 2.
type MWMRQueue[T any] struct {
	seg       *ShmSegment
	header    *int64   // pointer to atomic head in SHM
	elems     uintptr  // pointer to first QueueElem in SHM
	size      int64    // power of 2
	mask      int64    // size - 1
	elemSize  uintptr  // sizeof(QueueElem<T>) = sizeof(T) + 8
	dataSize  uintptr  // sizeof(T)
	localTail int64    // reader-side tail (not in SHM)
}

// NewMWMRQueue attaches to an existing SHM segment containing a MWMR queue.
// shmKey: SysV SHM key
// queueSize: number of elements (will be rounded to next power of 2)
// The type parameter T determines the element size.
// elemSizeOverride: if > 0, use this as the queue element size instead of
// computing sizeof(T)+8. This is needed when the C++ struct has alignment
// attributes (e.g. __attribute__((aligned(64)))) that cause the C++ compiler
// to pad QueueElem<T> beyond sizeof(T)+8.
func NewMWMRQueue[T any](shmKey int, queueSize int, elemSizeOverride ...uintptr) (*MWMRQueue[T], error) {
	size := nextPowerOf2(int64(queueSize))

	var zero T
	dataSize := unsafe.Sizeof(zero)
	elemSize := dataSize + 8 // sizeof(T) + sizeof(uint64 seqNo)

	// Allow override for C++ alignment-padded structs
	if len(elemSizeOverride) > 0 && elemSizeOverride[0] > 0 {
		elemSize = elemSizeOverride[0]
	}

	headerSize := uintptr(8) // sizeof(MWMRHeader) = sizeof(atomic<int64_t>)

	totalBytes := int(headerSize + uintptr(size)*elemSize)

	seg, err := ShmOpen(shmKey, totalBytes)
	if err != nil {
		return nil, fmt.Errorf("MWMRQueue: ShmOpen(key=0x%x): %w", shmKey, err)
	}

	q := &MWMRQueue[T]{
		seg:      seg,
		header:   (*int64)(unsafe.Pointer(seg.Addr)),
		elems:    seg.Addr + headerSize,
		size:     size,
		mask:     size - 1,
		elemSize: elemSize,
		dataSize: dataSize,
	}

	// C++: tail = header->head.load(relaxed) — start reading from current head
	q.localTail = atomic.LoadInt64(q.header)

	return q, nil
}

// NewMWMRQueueCreate creates a new SHM segment for a MWMR queue (for tests).
func NewMWMRQueueCreate[T any](shmKey int, queueSize int, elemSizeOverride ...uintptr) (*MWMRQueue[T], error) {
	size := nextPowerOf2(int64(queueSize))

	var zero T
	dataSize := unsafe.Sizeof(zero)
	elemSize := dataSize + 8

	// Allow override for C++ alignment-padded structs
	if len(elemSizeOverride) > 0 && elemSizeOverride[0] > 0 {
		elemSize = elemSizeOverride[0]
	}

	headerSize := uintptr(8)

	totalBytes := int(headerSize + uintptr(size)*elemSize)

	seg, err := ShmCreate(shmKey, totalBytes)
	if err != nil {
		return nil, fmt.Errorf("MWMRQueue: ShmCreate(key=0x%x): %w", shmKey, err)
	}

	q := &MWMRQueue[T]{
		seg:      seg,
		header:   (*int64)(unsafe.Pointer(seg.Addr)),
		elems:    seg.Addr + headerSize,
		size:     size,
		mask:     size - 1,
		elemSize: elemSize,
		dataSize: dataSize,
	}

	// Initialize header: head = 1 (C++ MultiWriterMultiReaderShmHeader ctor)
	atomic.StoreInt64(q.header, 1)
	q.localTail = 1

	// Zero out all elements
	memZero(unsafe.Pointer(q.elems), uintptr(size)*elemSize)

	return q, nil
}

// Enqueue adds a value to the queue (thread-safe, lock-free).
// C++: int64_t myHead = header->head.fetch_add(1, acq_rel);
//      slot = m_updates + (myHead & (m_size - 1));
//      memcpy(&slot->data, &value, sizeof(T));
//      asm volatile("" ::: "memory");
//      slot->seqNo = myHead;
func (q *MWMRQueue[T]) Enqueue(value *T) {
	myHead := atomic.AddInt64(q.header, 1) - 1

	slotAddr := q.elems + uintptr(myHead&q.mask)*q.elemSize
	// Copy data into slot
	memCopy(unsafe.Pointer(slotAddr), unsafe.Pointer(value), q.dataSize)

	// Write seqNo after data (compiler barrier equivalent)
	seqNoPtr := (*uint64)(unsafe.Pointer(slotAddr + q.dataSize))
	atomic.StoreUint64(seqNoPtr, uint64(myHead))
}

// Dequeue reads and returns the next value from the queue.
// Returns false if the queue is empty.
// C++: QueueElem<T>* value = m_updates + (tail & (m_size-1));
//      tail = value->seqNo + 1;
//      return value->data;
func (q *MWMRQueue[T]) Dequeue(out *T) bool {
	slotAddr := q.elems + uintptr(q.localTail&q.mask)*q.elemSize
	seqNoPtr := (*uint64)(unsafe.Pointer(slotAddr + q.dataSize))

	seqNo := atomic.LoadUint64(seqNoPtr)
	if seqNo < uint64(q.localTail) {
		return false // empty
	}

	// Copy data out
	memCopy(unsafe.Pointer(out), unsafe.Pointer(slotAddr), q.dataSize)
	q.localTail = int64(seqNo) + 1
	return true
}

// IsEmpty checks if there's data available.
// C++: return (m_updates + (tail & (m_size-1)))->seqNo < tail;
func (q *MWMRQueue[T]) IsEmpty() bool {
	slotAddr := q.elems + uintptr(q.localTail&q.mask)*q.elemSize
	seqNoPtr := (*uint64)(unsafe.Pointer(slotAddr + q.dataSize))
	return atomic.LoadUint64(seqNoPtr) < uint64(q.localTail)
}

// Close detaches and removes the SHM segment.
func (q *MWMRQueue[T]) Close() error {
	return q.seg.Detach()
}

// Destroy detaches and removes the SHM segment.
func (q *MWMRQueue[T]) Destroy() error {
	if err := q.seg.Detach(); err != nil {
		return err
	}
	return q.seg.Remove()
}

// Segment returns the underlying SHM segment.
func (q *MWMRQueue[T]) Segment() *ShmSegment {
	return q.seg
}

// nextPowerOf2 returns the smallest power of 2 >= value.
func nextPowerOf2(value int64) int64 {
	if value <= 0 {
		return 1
	}
	// Check if already a power of 2
	if value&(value-1) == 0 {
		return value
	}
	result := int64(1)
	for result < value {
		result <<= 1
	}
	return result
}

// memCopy copies n bytes from src to dst using byte slice operations.
// No CGO needed.
func memCopy(dst, src unsafe.Pointer, n uintptr) {
	dstSlice := unsafe.Slice((*byte)(dst), n)
	srcSlice := unsafe.Slice((*byte)(src), n)
	copy(dstSlice, srcSlice)
}

// memZero zeroes n bytes at ptr.
func memZero(ptr unsafe.Pointer, n uintptr) {
	s := unsafe.Slice((*byte)(ptr), n)
	for i := range s {
		s[i] = 0
	}
}
