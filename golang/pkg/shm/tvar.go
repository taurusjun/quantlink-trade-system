// Package shm provides shared memory utilities for inter-process communication
// C++: hftbase/memlog/include/SHM.h, tbsrc/common/include/tvar.h
package shm

import (
	"fmt"
	"sync"
	"syscall"
	"unsafe"
)

// IPC constants (not defined in syscall package on macOS/Darwin)
const (
	IPC_CREAT = 0001000 // Create key if key does not exist
	IPC_EXCL  = 0002000 // Fail if key exists
	IPC_RMID  = 0       // Remove identifier
)

// TVar is a shared memory variable for inter-process communication
// C++: hftlib::tvar<T> in tbsrc/common/include/tvar.h
//
// 用于外部程序（如 Python 模型）向策略传递实时调整参数
// 例如：tValue 用于调整价差均值
type TVar struct {
	shmID   int
	shmAddr uintptr
	size    int
	key     int
	mu      sync.RWMutex
}

// NewTVar creates a new TVar instance
// C++: tvar::init(shmkey, flag)
func NewTVar(key int) (*TVar, error) {
	if key <= 0 {
		return nil, fmt.Errorf("invalid shm key: %d", key)
	}

	tv := &TVar{
		key:  key,
		size: 8, // sizeof(double) = 8 bytes
	}

	// Try to get existing shared memory first
	// C++: shmget(shmkey, size, flag)
	shmID, _, errno := syscall.Syscall(
		syscall.SYS_SHMGET,
		uintptr(key),
		uintptr(tv.size),
		uintptr(0666), // Read/write for all
	)

	if errno != 0 {
		// Try to create if doesn't exist
		shmID, _, errno = syscall.Syscall(
			syscall.SYS_SHMGET,
			uintptr(key),
			uintptr(tv.size),
			uintptr(IPC_CREAT|0666),
		)
		if errno != 0 {
			return nil, fmt.Errorf("shmget failed: %v", errno)
		}
	}

	tv.shmID = int(shmID)

	// Attach to shared memory
	// C++: shmat(shmid, NULL, 0)
	addr, _, errno := syscall.Syscall(
		syscall.SYS_SHMAT,
		uintptr(tv.shmID),
		0,
		0,
	)

	if errno != 0 {
		return nil, fmt.Errorf("shmat failed: %v", errno)
	}

	tv.shmAddr = addr

	return tv, nil
}

// Load reads the value from shared memory
// C++: m_ptr->load()
func (tv *TVar) Load() float64 {
	tv.mu.RLock()
	defer tv.mu.RUnlock()

	if tv.shmAddr == 0 {
		return 0
	}

	// Read double (8 bytes) from shared memory
	ptr := (*float64)(unsafe.Pointer(tv.shmAddr))
	return *ptr
}

// Store writes a value to shared memory
// C++: m_ptr->store(v)
func (tv *TVar) Store(value float64) {
	tv.mu.Lock()
	defer tv.mu.Unlock()

	if tv.shmAddr == 0 {
		return
	}

	// Write double (8 bytes) to shared memory
	ptr := (*float64)(unsafe.Pointer(tv.shmAddr))
	*ptr = value
}

// Close detaches from shared memory
// C++: shmdt(m_shmadr)
func (tv *TVar) Close() error {
	tv.mu.Lock()
	defer tv.mu.Unlock()

	if tv.shmAddr == 0 {
		return nil
	}

	_, _, errno := syscall.Syscall(
		syscall.SYS_SHMDT,
		tv.shmAddr,
		0,
		0,
	)

	if errno != 0 {
		return fmt.Errorf("shmdt failed: %v", errno)
	}

	tv.shmAddr = 0
	return nil
}

// Key returns the shared memory key
func (tv *TVar) Key() int {
	return tv.key
}

// IsValid returns true if the TVar is properly initialized
func (tv *TVar) IsValid() bool {
	return tv.shmAddr != 0
}
