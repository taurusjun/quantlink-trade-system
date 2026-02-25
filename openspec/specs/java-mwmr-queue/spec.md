## ADDED Requirements

### Requirement: MWMR Queue SHM 内存布局

系统 SHALL 实现与 C++ `MultiWriterMultiReaderShmQueue<T>` 完全二进制兼容的 SHM 内存布局。

- 迁移自: `hftbase/Ipc/include/multiwritermultireadershmqueue.h`
- 迁移自: `hftbase/Ipc/include/shmallocator.h`

布局:
```
[MWMRHeader (8 bytes: atomic<int64> head)][QueueElem[0]][QueueElem[1]]...[QueueElem[size-1]]
```

每个 `QueueElem<T>`:
```
[T data (sizeof(T) bytes)][uint64 seqNo (8 bytes)]
```

队列大小 MUST 为 2 的幂（由 `nextPowerOf2()` 确保）。

#### Scenario: SHM 总大小计算
- **WHEN** 创建 `MWMRQueue<MarketUpdateNew>` 大小为 1024
- **THEN** SHM 总大小 MUST 等于 `8 + 1024 × 824` bytes（header 8 + slots × elemSize）

#### Scenario: RequestMsg 队列使用 320 bytes elemSize
- **WHEN** 创建 `MWMRQueue<RequestMsg>`
- **THEN** elemSize MUST 使用 320（非 264），对应 C++ `sizeof(QueueElem<RequestMsg>)` 在 `aligned(64)` 下的值

#### Scenario: 初始化 head 值
- **WHEN** 创建新的 MWMR Queue（测试模式）
- **THEN** header 的 head 值 MUST 初始化为 1（与 C++ `MultiWriterMultiReaderShmHeader` 构造函数一致）

### Requirement: Lock-free 入队操作

系统 SHALL 实现 lock-free 的 `enqueue()` 方法，语义与 C++ `MultiWriterMultiReaderShmQueue::enqueue()` 完全一致。

- 迁移自: `hftbase/Ipc/include/multiwritermultireadershmqueue.h` — `enqueue(const T &value)`

步骤:
1. `myHead = head.fetch_add(1, acq_rel)` — 原子占位
2. 计算 slot: `offset = headerSize + (myHead & mask) × elemSize`
3. `memcpy(&slot.data, &value, sizeof(T))` — 拷贝数据
4. store-store fence（compiler barrier）
5. `slot.seqNo = myHead` — 发布序号（release store）

#### Scenario: 单线程入队
- **WHEN** 向空队列入队一条 `RequestMsg`
- **THEN** head 从 1 变为 2，slot[1 & mask] 的 data 区包含正确的 `RequestMsg` 数据，seqNo 等于 1

#### Scenario: 多线程并发入队
- **WHEN** 两个线程同时调用 `enqueue()`
- **THEN** 两条消息分别写入不同的 slot（`fetch_add` 保证唯一 slot 分配），不会数据竞争

### Requirement: 出队操作

系统 SHALL 实现 `dequeue()` 方法，语义与 C++ `MultiWriterMultiReaderShmQueue::dequeue()` / `dequeuePtr()` 完全一致。

- 迁移自: `hftbase/Ipc/include/multiwritermultireadershmqueue.h` — `dequeue()` / `dequeuePtr(T* data)`

步骤:
1. 计算 slot: `offset = headerSize + (localTail & mask) × elemSize`
2. 读取 `seqNo`（acquire load）
3. 如果 `seqNo < localTail`，返回 false（队列为空）
4. `memcpy(out, &slot.data, sizeof(T))` — 拷贝数据
5. `localTail = seqNo + 1` — 推进本地 tail

`localTail` 是读者私有变量，不在 SHM 中，不需要原子操作。

#### Scenario: 出队成功
- **WHEN** 队列中有已发布的消息（seqNo >= localTail），调用 `dequeue(out)`
- **THEN** 返回 true，`out` 包含正确的消息数据，`localTail` 推进

#### Scenario: 队列为空时出队
- **WHEN** 队列为空（slot 的 seqNo < localTail），调用 `dequeue(out)`
- **THEN** 返回 false，`out` 不被修改，`localTail` 不变

### Requirement: 队列空判断

系统 SHALL 实现 `isEmpty()` 方法。

- 迁移自: `hftbase/Ipc/include/multiwritermultireadershmqueue.h` — `isEmpty()`

#### Scenario: 空队列
- **WHEN** 没有新消息入队，调用 `isEmpty()`
- **THEN** 返回 true

#### Scenario: 有新消息
- **WHEN** 入队一条消息后，调用 `isEmpty()`
- **THEN** 返回 false

### Requirement: 连接模式与创建模式

系统 SHALL 支持两种初始化模式：
- **连接模式**（生产用）：连接已有 SHM 段，读取已有的 head 值设置 localTail
- **创建模式**（测试用）：创建新 SHM 段，初始化 head=1

- 迁移自: `hftbase/Ipc/include/multiwritermultireadershmqueue.h` — `init(shmkey, shmsize, flag)`

#### Scenario: 连接已有队列
- **WHEN** C++ `md_shm_feeder` 已创建 SHM 0x1001 并写入行情，Java 调用连接模式
- **THEN** Java 读取当前 head 值作为 localTail，后续 `dequeue()` 从最新位置开始读取

#### Scenario: 创建新队列用于测试
- **WHEN** 调用创建模式创建 SHM 0x9999
- **THEN** SHM 全零初始化，head=1，可立即使用 `enqueue()` / `dequeue()`

### Requirement: 资源清理

系统 SHALL 提供 `close()` 和 `destroy()` 方法。
- `close()`: 分离 SHM（`shmdt`），不删除
- `destroy()`: 分离并删除 SHM（`shmdt` + `shmctl(IPC_RMID)`）

#### Scenario: close 后 SHM 仍存在
- **WHEN** 调用 `close()` 后
- **THEN** SHM 段仍然存在（其他进程可继续访问）

#### Scenario: destroy 后 SHM 被删除
- **WHEN** 调用 `destroy()` 后
- **THEN** SHM 段被标记为删除，`ipcs -m` 中不再可见
