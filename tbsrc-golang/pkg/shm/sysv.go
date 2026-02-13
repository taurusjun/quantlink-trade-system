package shm

import (
	"fmt"
	"syscall"
	"unsafe"
)

const (
	IPC_CREAT = 01000
	IPC_EXCL  = 02000
	IPC_RMID  = 0
	SHM_RDONLY = 010000
)

// ShmSegment represents an attached SysV shared memory segment.
type ShmSegment struct {
	ID   int
	Addr uintptr
	Size int
}

// ShmOpen attaches to an existing SysV SHM segment (Go trader use case).
func ShmOpen(key, size int) (*ShmSegment, error) {
	totalBytes := pageAlign(size)
	id, _, errno := syscall.Syscall(sysGET, uintptr(key), uintptr(totalBytes), uintptr(0666))
	if errno != 0 {
		return nil, fmt.Errorf("shmget(key=0x%x, size=%d): %w", key, totalBytes, errno)
	}

	addr, _, errno := syscall.Syscall(sysAT, id, 0, 0)
	if errno != 0 {
		return nil, fmt.Errorf("shmat(id=%d): %w", id, errno)
	}

	return &ShmSegment{ID: int(id), Addr: addr, Size: totalBytes}, nil
}

// ShmCreate creates a new SysV SHM segment (for tests / ORS creator).
func ShmCreate(key, size int) (*ShmSegment, error) {
	totalBytes := pageAlign(size)
	id, _, errno := syscall.Syscall(sysGET, uintptr(key), uintptr(totalBytes), uintptr(IPC_CREAT|IPC_EXCL|0666))
	if errno != 0 {
		// If already exists, try attaching without IPC_EXCL
		if errno == syscall.EEXIST {
			id, _, errno = syscall.Syscall(sysGET, uintptr(key), uintptr(totalBytes), uintptr(IPC_CREAT|0666))
			if errno != 0 {
				return nil, fmt.Errorf("shmget(key=0x%x, size=%d, existing): %w", key, totalBytes, errno)
			}
		} else {
			return nil, fmt.Errorf("shmget(key=0x%x, size=%d, create): %w", key, totalBytes, errno)
		}
	}

	addr, _, errno := syscall.Syscall(sysAT, id, 0, 0)
	if errno != 0 {
		return nil, fmt.Errorf("shmat(id=%d): %w", id, errno)
	}

	return &ShmSegment{ID: int(id), Addr: addr, Size: totalBytes}, nil
}

// Detach detaches the SHM segment from this process.
func (s *ShmSegment) Detach() error {
	_, _, errno := syscall.Syscall(sysDT, s.Addr, 0, 0)
	if errno != 0 {
		return fmt.Errorf("shmdt(addr=0x%x): %w", s.Addr, errno)
	}
	return nil
}

// Remove marks the SHM segment for removal.
func (s *ShmSegment) Remove() error {
	_, _, errno := syscall.Syscall(sysCTL, uintptr(s.ID), IPC_RMID, 0)
	if errno != 0 {
		return fmt.Errorf("shmctl(id=%d, IPC_RMID): %w", s.ID, errno)
	}
	return nil
}

// Ptr returns an unsafe.Pointer to the SHM base address.
func (s *ShmSegment) Ptr() unsafe.Pointer {
	return unsafe.Pointer(s.Addr)
}

// pageAlign rounds up to the next page boundary.
// C++: size_in + sz - (size_in % sz)
func pageAlign(size int) int {
	pageSize := syscall.Getpagesize()
	if size%pageSize == 0 {
		return size
	}
	return size + pageSize - (size % pageSize)
}
