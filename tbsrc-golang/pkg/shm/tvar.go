package shm

import (
	"fmt"
	"math"
	"sync/atomic"
	"unsafe"
)

// TVar 对应 C++ hftlib::tvar<double>
// 通过 SysV SHM 共享一个 float64 值
// 参考: hftbase CommonUtils 中的 tvar 实现
type TVar struct {
	seg *ShmSegment
	ptr unsafe.Pointer // 指向 SHM 中的 uint64（存储 float64 的位模式）
}

// OpenTVar 打开已有的 tvar SHM 段
// key: SysV SHM key，与 C++ tvar 配置一致
// 如果 key <= 0，返回 nil（表示不使用 tvar）
func OpenTVar(key int32) (*TVar, error) {
	if key <= 0 {
		return nil, nil
	}

	// shmget + shmat: 8 bytes for one float64
	seg, err := ShmOpen(int(key), 8)
	if err != nil {
		return nil, fmt.Errorf("tvar: open key=0x%x: %w", key, err)
	}

	return &TVar{
		seg: seg,
		ptr: seg.Ptr(),
	}, nil
}

// Load 原子读取 tvar 值
// C++: m_tvar->load()
func (tv *TVar) Load() float64 {
	if tv == nil || tv.ptr == nil {
		return 0
	}
	bits := atomic.LoadUint64((*uint64)(tv.ptr))
	return math.Float64frombits(bits)
}

// Store 原子写入 tvar 值
func (tv *TVar) Store(v float64) {
	if tv == nil || tv.ptr == nil {
		return
	}
	bits := math.Float64bits(v)
	atomic.StoreUint64((*uint64)(tv.ptr), bits)
}

// Close 分离 SHM 段
func (tv *TVar) Close() error {
	if tv == nil || tv.seg == nil {
		return nil
	}
	return tv.seg.Detach()
}
