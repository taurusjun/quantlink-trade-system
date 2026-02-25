## ADDED Requirements

### Requirement: 三队列 SHM 管理

系统 SHALL 管理三个 MWMR Queue 和一个 ClientStore：

| 队列 | SHM Key | 方向 | 消息类型 | elemSize |
|------|---------|------|---------|----------|
| 行情队列 | 0x1001 | 读取 | MarketUpdateNew | 824 (816+8) |
| 请求队列 | 0x2001 | 写入 | RequestMsg | 320 (aligned(64)) |
| 回报队列 | 0x3001 | 读取 | ResponseMsg | 184 (176+8) |
| ClientStore | 0x4001 | 读写 | int64 | 16 |

- 迁移自: `hftbase/Connector/include/connector.h` — `Connector` 类
- 迁移自: `hftbase/Ipc/include/shmmanager.h` — `ShmManager`

SHM key 和 queue size MUST 从配置中读取，不得硬编码。

#### Scenario: 初始化连接所有 SHM 段
- **WHEN** 调用 `Connector.create(config, mdCallback, orsCallback)`
- **THEN** 成功连接 4 个 SHM 段（0x1001/0x2001/0x3001/0x4001），从 ClientStore 获取唯一 clientId

#### Scenario: ClientStore 分配 clientId
- **WHEN** Connector 初始化时
- **THEN** 调用 `clientStore.getClientIdAndIncrement()` 获取唯一 clientId 并保存

### Requirement: OrderID 生成

系统 SHALL 实现 OrderID 生成逻辑：`OrderID = clientId × ORDERID_RANGE + seq`，其中 `seq` 从 0 原子递增。

- 迁移自: `hftbase/Connector/include/connector.h` — `GetUniqueOrderNumber()`

`ORDERID_RANGE = 1_000_000`

#### Scenario: 第一个 OrderID
- **WHEN** clientId=3，发送第一笔订单
- **THEN** OrderID MUST 等于 `3 × 1_000_000 + 0 = 3_000_000`

#### Scenario: 连续 OrderID
- **WHEN** clientId=3，连续发送 3 笔订单
- **THEN** OrderID 依次为 3_000_000, 3_000_001, 3_000_002

### Requirement: 发送新订单

系统 SHALL 实现 `sendNewOrder(req)` 方法：
1. 设置 `req.Request_Type = NEWORDER`
2. 生成 OrderID 并设置 `req.OrderID`
3. 设置时间戳 `req.TimeStamp`
4. 入队到请求队列（0x2001）
5. 返回分配的 OrderID

- 迁移自: `hftbase/Connector/include/connector.h` — `SendNewOrder(RequestMsg&)`

#### Scenario: 发送限价单
- **WHEN** 调用 `sendNewOrder(req)`，req 中已填充 symbol、price、quantity
- **THEN** req.Request_Type 被设为 NEWORDER，req.OrderID 被设为生成的值，消息入队到 SHM 0x2001

### Requirement: 发送撤单

系统 SHALL 实现 `sendCancelOrder(req)` 方法：
1. 设置 `req.Request_Type = CANCELORDER`
2. 设置时间戳
3. 入队到请求队列

- 迁移自: `hftbase/Connector/include/connector.h` — `SendCancelOrder(RequestMsg&)`

#### Scenario: 撤销订单
- **WHEN** 调用 `sendCancelOrder(req)`，req.OrderID 为要撤销的订单 ID
- **THEN** req.Request_Type 被设为 CANCELORDER，消息入队到 SHM 0x2001

### Requirement: 发送改单

系统 SHALL 实现 `sendModifyOrder(req)` 方法：
1. 设置 `req.Request_Type = MODIFYORDER`
2. 设置时间戳
3. 入队到请求队列

- 迁移自: `hftbase/Connector/include/connector.h` — `SendModifyOrder(RequestMsg&)`

#### Scenario: 修改订单价格
- **WHEN** 调用 `sendModifyOrder(req)`，req 中包含新价格和原 OrderID
- **THEN** req.Request_Type 被设为 MODIFYORDER，消息入队到 SHM 0x2001

### Requirement: 行情轮询与回调

系统 SHALL 启动独立线程持续轮询行情队列（0x1001），每收到一条 `MarketUpdateNew` 即调用 `mdCallback`。

- 迁移自: `hftbase/Connector/include/connector.h` — MD 处理循环

#### Scenario: 收到行情回调
- **WHEN** C++ `md_shm_feeder` 向 SHM 0x1001 写入一条行情
- **THEN** Java Connector 的 pollMD 线程出队该消息并调用 `mdCallback(marketUpdate)`

#### Scenario: 无行情时不阻塞
- **WHEN** 行情队列为空
- **THEN** pollMD 线程持续轮询（busy-wait 或短暂 yield），不阻塞

### Requirement: ORS 回报轮询与过滤

系统 SHALL 启动独立线程持续轮询回报队列（0x3001），仅处理属于本 Connector 的回报，然后调用 `orsCallback`。

- 迁移自: `hftbase/Connector/include/connector.h` — ORS Response 处理

过滤条件: `resp.OrderID / ORDERID_RANGE == clientId`

#### Scenario: 收到属于本 client 的回报
- **WHEN** 回报队列中有 OrderID=3_000_001 的回报，本 Connector 的 clientId=3
- **THEN** `3_000_001 / 1_000_000 == 3 == clientId`，调用 `orsCallback(response)`

#### Scenario: 过滤其他 client 的回报
- **WHEN** 回报队列中有 OrderID=5_000_001 的回报，本 Connector 的 clientId=3
- **THEN** `5_000_001 / 1_000_000 == 5 != 3`，跳过不处理

### Requirement: 启动与停止

系统 SHALL 提供 `start()` 和 `stop()` 方法。

- `start()`: 启动 pollMD 和 pollORS 两个线程
- `stop()`: 设置 running=false，等待两个线程退出

- 迁移自: `hftbase/Connector/include/connector.h` — `StartSync()` / `Stop()`

#### Scenario: 启动 Connector
- **WHEN** 调用 `start()`
- **THEN** pollMD 和 pollORS 两个线程开始运行

#### Scenario: 停止 Connector
- **WHEN** 调用 `stop()`
- **THEN** 两个轮询线程优雅退出，不再调用回调
