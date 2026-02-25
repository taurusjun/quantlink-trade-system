## ADDED Requirements

### Requirement: ClientStore SHM 布局

系统 SHALL 实现与 C++ `LocklessShmClientStore<uint64_t>` 完全二进制兼容的 SHM 布局。

- 迁移自: `hftbase/Ipc/include/locklessshmclientstore.h` — `LocklessShmClientStore<IntType>`

SHM 布局（16 bytes）:
```
offset 0: atomic<int64> data           (8 bytes) — 当前 clientId（原子递增）
offset 8: int64 firstClientId          (8 bytes) — 初始 clientId 值（只读）
```

#### Scenario: ClientStore SHM 大小
- **WHEN** 创建 ClientStore
- **THEN** SHM 段大小 MUST 至少 16 bytes

### Requirement: 原子 clientId 分配

系统 SHALL 提供原子 `getClientIdAndIncrement()` 方法，语义与 C++ `LocklessShmClientStore::getClientIdAndIncrement()` 完全一致。

- 迁移自: `hftbase/Ipc/include/locklessshmclientstore.h` — `getClientIdAndIncrement()`

实现: `data.fetch_add(1, acq_rel)`，返回递增前的值。

#### Scenario: 分配唯一 clientId
- **WHEN** 两个进程先后调用 `getClientIdAndIncrement()`，初始值为 1
- **THEN** 第一个进程获得 1，第二个进程获得 2，data 变为 3

#### Scenario: 多线程并发分配
- **WHEN** 10 个线程并发调用 `getClientIdAndIncrement()`
- **THEN** 每个线程获得不同的 clientId，无重复值

### Requirement: 读取 clientId

系统 SHALL 提供 `getClientId()` 方法，读取当前 clientId 值而不递增。

- 迁移自: `hftbase/Ipc/include/locklessshmclientstore.h` — `getClientId()`

实现: `data.load(acquire)`

#### Scenario: 读取当前值
- **WHEN** data 当前值为 5，调用 `getClientId()`
- **THEN** 返回 5，data 值不变

### Requirement: 读取初始 clientId

系统 SHALL 提供 `getFirstClientIdValue()` 方法，读取 offset 8 处的初始值。

- 迁移自: `hftbase/Ipc/include/locklessshmclientstore.h` — `getFirstClientIdValue()`

#### Scenario: 读取初始值
- **WHEN** ClientStore 以 initialValue=1 创建
- **THEN** `getFirstClientIdValue()` 返回 1

### Requirement: 连接模式与创建模式

- **连接模式**（生产用）：连接已有 SHM，读取已有的 data 和 firstClientId
- **创建模式**（测试用）：创建新 SHM，初始化 `data = initialValue`，`firstClientId = initialValue`

- 迁移自: `hftbase/Ipc/include/locklessshmclientstore.h` — `init(shmkey, flag, initialValue)`

#### Scenario: 连接已有 ClientStore
- **WHEN** C++ `counter_bridge` 已创建 SHM 0x4001，Java 调用连接模式
- **THEN** 读取到 C++ 进程写入的 data 和 firstClientId 值

#### Scenario: 创建新 ClientStore
- **WHEN** 调用创建模式，initialValue=1
- **THEN** SHM 中 data=1, firstClientId=1
