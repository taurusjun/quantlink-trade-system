## ADDED Requirements

### Requirement: SysV SHM 段创建与连接

系统 SHALL 通过 Panama FFI `Linker.downcallHandle()` 封装 libc 的 `shmget` 和 `shmat` 函数，支持创建新 SHM 段和连接已有 SHM 段。

- 迁移自: `hftbase/Ipc/include/shmallocator.h` — `ShmAllocator::init()`
- 迁移自: `hftbase/Ipc/src/sharedmemory.cpp` — `SharedMemory::init()`

连接模式（生产用）：`shmget(key, size, 0)` + `shmat(shmid, NULL, 0)`
创建模式（测试用）：`shmget(key, size, IPC_CREAT|IPC_EXCL|0666)` + `shmat(shmid, NULL, 0)`

返回的 `MemorySegment` MUST 通过 `reinterpret(size)` 设置正确的边界。

#### Scenario: 连接已有 SHM 段
- **WHEN** 调用 `SysVShm.open(key=0x1001, size=1048576)`，且该 SHM 段已由 C++ `md_shm_feeder` 创建
- **THEN** 返回 `ShmSegment` 对象，包含有效的 `MemorySegment`（可读写），`shmid > 0`

#### Scenario: 创建新 SHM 段
- **WHEN** 调用 `SysVShm.create(key=0x9999, size=4096)`，且该 key 不存在
- **THEN** 创建新 SHM 段并返回 `ShmSegment` 对象，SHM 内容初始化为全零

#### Scenario: 创建已存在的 SHM 段
- **WHEN** 调用 `SysVShm.create(key, size)`，且该 key 已存在（`IPC_EXCL` 返回 `EEXIST`）
- **THEN** 回退为连接模式（`shmget(key, size, 0)` + `shmat`），不报错

### Requirement: SHM 段分离与删除

系统 SHALL 提供 `detach()` 和 `remove()` 方法，分别对应 C++ 的 `shmdt()` 和 `shmctl(IPC_RMID)`。

- 迁移自: `hftbase/Ipc/include/shmallocator.h` — `ShmAllocator::cleanup()`

#### Scenario: 分离 SHM 段
- **WHEN** 对已连接的 `ShmSegment` 调用 `detach()`
- **THEN** 调用 `shmdt(addr)` 成功，后续对该 `MemorySegment` 的读写 SHALL 抛出异常或产生未定义行为

#### Scenario: 删除 SHM 段
- **WHEN** 对已连接的 `ShmSegment` 调用 `remove()`
- **THEN** 调用 `shmctl(shmid, IPC_RMID, NULL)` 标记该 SHM 段为删除，最后一个进程 detach 后 OS 回收

### Requirement: 跨平台 libc 符号查找

系统 SHALL 通过 `Linker.defaultLookup().find("shmget")` 等方式查找 libc 函数符号，在 Linux 和 macOS 上均可运行，无需平台特定的 syscall 号。

- 设计决策参考: design.md Decision 6

#### Scenario: macOS 上运行
- **WHEN** 在 macOS (darwin) 上调用 `SysVShm.open(key, size)`
- **THEN** 通过 `Linker.defaultLookup()` 找到 macOS libc 中的 `shmget` 符号并成功调用

#### Scenario: Linux 上运行
- **WHEN** 在 Linux 上调用 `SysVShm.open(key, size)`
- **THEN** 通过 `Linker.defaultLookup()` 找到 Linux libc 中的 `shmget` 符号并成功调用

### Requirement: 页对齐

`SysVShm.create()` 在分配 SHM 时 SHALL 将 size 向上对齐到系统页大小（通常 4096 bytes）。

- 迁移自: `hftbase/Ipc/src/sharedmemory.cpp` 中的页对齐逻辑

#### Scenario: 非页对齐 size
- **WHEN** 调用 `SysVShm.create(key, size=5000)`
- **THEN** 实际分配 `8192` bytes（向上对齐到 4096 的倍数）
