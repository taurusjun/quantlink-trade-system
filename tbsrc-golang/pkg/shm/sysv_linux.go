package shm

import "syscall"

// Linux SysV SHM syscall numbers
const (
	sysGET = syscall.SYS_SHMGET
	sysAT  = syscall.SYS_SHMAT
	sysDT  = syscall.SYS_SHMDT
	sysCTL = syscall.SYS_SHMCTL
)
