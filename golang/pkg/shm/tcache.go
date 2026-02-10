// Package shm provides shared memory utilities for inter-process communication
// C++: tbsrc/common/include/tvar.h - tcache class
package shm

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// TCacheNode is a single key-value entry in the cache
// C++: struct node { char key[64]; T value; }
type TCacheNode struct {
	Key   [64]byte // Key string (null-terminated)
	Value float64  // Value
}

// TCacheHeader is the header of the shared memory cache
type TCacheHeader struct {
	Tail uint64 // Current tail index (atomic in C++)
}

// TCache is a shared memory key-value cache for inter-process communication
// C++: hftlib::tcache<T> in tbsrc/common/include/tvar.h
//
// 用于策略向外部程序共享持仓等数据
// 例如：SendTCacheLeg1Pos() 将持仓写入共享内存
type TCache struct {
	shmID    int
	shmAddr  uintptr
	size     int
	key      int
	maxNodes int
	localMap map[string]*float64 // Local cache of pointers
	mu       sync.RWMutex
}

// NewTCache creates a new TCache instance
// C++: tcache::init(shmkey, cnt, flag)
func NewTCache(key int, maxNodes int) (*TCache, error) {
	if key <= 0 {
		return nil, fmt.Errorf("invalid shm key: %d", key)
	}

	if maxNodes <= 0 {
		maxNodes = 100 // Default
	}

	tc := &TCache{
		key:      key,
		maxNodes: maxNodes,
		localMap: make(map[string]*float64),
	}

	// Calculate size: header + nodes
	// C++: sizeof(node) * cnt + sizeof(size_t)
	nodeSize := int(unsafe.Sizeof(TCacheNode{}))
	headerSize := int(unsafe.Sizeof(TCacheHeader{}))
	tc.size = headerSize + nodeSize*maxNodes

	// Try to get existing shared memory first
	shmID, _, errno := syscall.Syscall(
		syscall.SYS_SHMGET,
		uintptr(key),
		uintptr(tc.size),
		uintptr(0666),
	)

	if errno != 0 {
		// Try to create if doesn't exist
		shmID, _, errno = syscall.Syscall(
			syscall.SYS_SHMGET,
			uintptr(key),
			uintptr(tc.size),
			uintptr(IPC_CREAT|0666),
		)
		if errno != 0 {
			return nil, fmt.Errorf("shmget failed: %v", errno)
		}
	}

	tc.shmID = int(shmID)

	// Attach to shared memory
	addr, _, errno := syscall.Syscall(
		syscall.SYS_SHMAT,
		uintptr(tc.shmID),
		0,
		0,
	)

	if errno != 0 {
		return nil, fmt.Errorf("shmat failed: %v", errno)
	}

	tc.shmAddr = addr

	// Update local cache
	tc.updateCache()

	return tc, nil
}

// getHeader returns pointer to the header
func (tc *TCache) getHeader() *TCacheHeader {
	return (*TCacheHeader)(unsafe.Pointer(tc.shmAddr))
}

// getNode returns pointer to a node at index
func (tc *TCache) getNode(index int) *TCacheNode {
	headerSize := int(unsafe.Sizeof(TCacheHeader{}))
	nodeSize := int(unsafe.Sizeof(TCacheNode{}))
	offset := headerSize + nodeSize*index
	return (*TCacheNode)(unsafe.Pointer(tc.shmAddr + uintptr(offset)))
}

// updateCache updates local map from shared memory
// C++: tcache::updateCache()
func (tc *TCache) updateCache() {
	header := tc.getHeader()
	tail := int(header.Tail)

	for i := 0; i < tail && i < tc.maxNodes; i++ {
		node := tc.getNode(i)
		key := string(node.Key[:cstrLen(node.Key[:])])
		if len(key) > 0 {
			if _, exists := tc.localMap[key]; !exists {
				tc.localMap[key] = &node.Value
			}
		}
	}
}

// cstrLen returns length of null-terminated C string
func cstrLen(b []byte) int {
	for i, c := range b {
		if c == 0 {
			return i
		}
	}
	return len(b)
}

// Store writes a value to the cache
// C++: tcache::store(key, val)
func (tc *TCache) Store(key string, value float64) error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.shmAddr == 0 {
		return fmt.Errorf("tcache not initialized")
	}

	// Check local cache first
	if ptr, exists := tc.localMap[key]; exists {
		*ptr = value
		return nil
	}

	// Update cache and try again
	tc.updateCache()
	if ptr, exists := tc.localMap[key]; exists {
		*ptr = value
		return nil
	}

	// Need to add new entry
	header := tc.getHeader()
	tail := int(header.Tail)

	if tail >= tc.maxNodes {
		return fmt.Errorf("tcache is full")
	}

	// Add new node
	node := tc.getNode(tail)
	copy(node.Key[:], key)
	node.Value = value

	// Update local map
	tc.localMap[key] = &node.Value

	// Increment tail
	header.Tail = uint64(tail + 1)

	return nil
}

// Load reads a value from the cache
// C++: tcache::load(key)
func (tc *TCache) Load(key string) (float64, error) {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	if tc.shmAddr == 0 {
		return 0, fmt.Errorf("tcache not initialized")
	}

	// Check local cache
	if ptr, exists := tc.localMap[key]; exists {
		return *ptr, nil
	}

	// Update cache and try again
	tc.updateCache()
	if ptr, exists := tc.localMap[key]; exists {
		return *ptr, nil
	}

	return 0, fmt.Errorf("key not found: %s", key)
}

// Close detaches from shared memory
func (tc *TCache) Close() error {
	tc.mu.Lock()
	defer tc.mu.Unlock()

	if tc.shmAddr == 0 {
		return nil
	}

	_, _, errno := syscall.Syscall(
		syscall.SYS_SHMDT,
		tc.shmAddr,
		0,
		0,
	)

	if errno != 0 {
		return fmt.Errorf("shmdt failed: %v", errno)
	}

	tc.shmAddr = 0
	tc.localMap = nil
	return nil
}

// Key returns the shared memory key
func (tc *TCache) Key() int {
	return tc.key
}

// IsValid returns true if the TCache is properly initialized
func (tc *TCache) IsValid() bool {
	return tc.shmAddr != 0
}

// String returns a string representation of the cache
// C++: tcache::to_string()
func (tc *TCache) String() string {
	tc.mu.RLock()
	defer tc.mu.RUnlock()

	if tc.shmAddr == 0 {
		return "TCache: not initialized"
	}

	header := tc.getHeader()
	tail := int(header.Tail)

	result := fmt.Sprintf("TCache(key=%d, entries=%d):\n", tc.key, tail)
	for i := 0; i < tail && i < tc.maxNodes; i++ {
		node := tc.getNode(i)
		key := string(node.Key[:cstrLen(node.Key[:])])
		result += fmt.Sprintf("  %s: %.6f\n", key, node.Value)
	}
	return result
}
