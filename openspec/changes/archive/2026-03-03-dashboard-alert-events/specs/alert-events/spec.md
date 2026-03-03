## ADDED Requirements

### Requirement: AlertEvent 数据结构
系统 SHALL 定义 AlertEvent 数据类，包含以下字段：
- `timestamp` (long): 事件时间戳（毫秒）
- `level` (String): 告警级别，"WARNING" 或 "CRITICAL"
- `type` (String): 告警类型标识
- `message` (String): 人可读告警描述
- `symbol` (String): 触发合约
- `strategyId` (int): 策略 ID

告警类型 SHALL 包括：
- `UPNL_LOSS`: 未实现亏损超限（WARNING）
- `STOP_LOSS`: 净 PNL 超限（CRITICAL）
- `MAX_LOSS`: 最大亏损超限（CRITICAL）
- `MAX_ORDERS`: 下单次数超限（CRITICAL）
- `MAX_TRADED`: 成交量超限（CRITICAL）
- `END_TIME`: 到达日终时间（CRITICAL）
- `END_TIME_AGG`: 到达激进平仓时间（CRITICAL）
- `AVG_SPREAD_AWAY`: 价差偏离超限（WARNING）
- `REJECT_LIMIT`: 拒单次数超限（CRITICAL）

#### Scenario: UPNL LOSS 告警
- **WHEN** unrealisedPNL 超过 UPNL_LOSS 阈值触发 onFlat
- **THEN** 生成 AlertEvent: level=WARNING, type=UPNL_LOSS, message 包含当前浮亏值和阈值

#### Scenario: MAX LOSS 告警
- **WHEN** netPNL 超过 MAX_LOSS 阈值触发 onExit
- **THEN** 生成 AlertEvent: level=CRITICAL, type=MAX_LOSS, message 包含当前 PNL 和阈值

#### Scenario: AVG_SPREAD_AWAY 告警
- **WHEN** currSpread 偏离 avgSpread 超过 AVG_SPREAD_AWAY 阈值
- **THEN** 生成 AlertEvent: level=WARNING, type=AVG_SPREAD_AWAY, message 包含 currSpread、avgSpread、drift 值

### Requirement: AlertCollector 环形缓冲区
ExecutionStrategy SHALL 持有一个 AlertCollector 实例，用于收集当天所有告警事件。

AlertCollector SHALL：
- 使用线程安全的数据结构（ConcurrentLinkedDeque 或同步列表）
- 固定最大容量 100 条，超出时丢弃最旧事件
- 提供 `add(AlertEvent)` 方法供策略调用
- 提供 `getAll()` 方法返回全量事件列表（按时间升序）

`add()` 操作 SHALL 为 O(1) 时间复杂度，不做任何 I/O 操作。

#### Scenario: 添加告警事件
- **WHEN** 策略触发 UPNL LOSS 告警
- **THEN** AlertCollector 中新增一条 AlertEvent，getAll() 返回列表包含该事件

#### Scenario: 缓冲区溢出
- **WHEN** AlertCollector 已有 100 条事件，再添加新事件
- **THEN** 最旧的事件被移除，新事件添加到末尾，总数保持 100

### Requirement: 告警采集点覆盖
系统 SHALL 在以下代码位置调用 `alertCollector.add()`：

1. `ExecutionStrategy.sendAlert()` — 所有通过 sendAlert 发出的告警
2. `ExecutionStrategy.checkSquareoff()` — UPNL_LOSS / STOP_LOSS 触发点（L2279-2302 对应）
3. `ExecutionStrategy.checkSquareoff()` — END_TIME / MAX_LOSS / MAX_ORDERS / MAX_TRADED 触发点（L2161-2179 对应）
4. `ExecutionStrategy.checkSquareoff()` — END_TIME_AGG 触发点（L2152-2159 对应）
5. `ExecutionStrategy.checkSquareoff()` — REJECT_LIMIT 触发点
6. `PairwiseArbStrategy.mdCallBack()` — AVG_SPREAD_AWAY 触发点

#### Scenario: sendAlert 路径采集
- **WHEN** ExecutionStrategy.sendAlert() 被调用
- **THEN** 同时调用 alertCollector.add()，告警类型从 reason 参数解析

#### Scenario: AVG_SPREAD_AWAY 路径采集
- **WHEN** PairwiseArbStrategy 检测到价差偏离超过 AVG_SPREAD_AWAY
- **THEN** 调用 firstStrat.alertCollector.add() 记录 AVG_SPREAD_AWAY 事件

### Requirement: Dashboard 告警展示
DashboardSnapshot SHALL 新增 `alerts` 字段（`List<AlertEvent>`），SnapshotCollector 每秒采集时从 AlertCollector.getAll() 读取全量告警列表。

Dashboard 前端 SHALL 在页面上显示告警面板：
- 按时间倒序排列（最新在上）
- WARNING 级别黄色背景
- CRITICAL 级别红色背景
- 每条显示：时间（HH:mm:ss）、类型、消息

#### Scenario: 新告警实时展示
- **WHEN** 策略触发 UPNL LOSS 告警
- **THEN** Dashboard 页面在下一次 WebSocket 推送（≤1 秒）后展示该告警

#### Scenario: 页面刷新后保留历史
- **WHEN** 用户刷新 Dashboard 页面
- **THEN** 页面重新连接 WebSocket 后，首次推送包含当天所有历史告警

#### Scenario: 无告警时
- **WHEN** 当天没有触发过告警
- **THEN** 告警面板显示为空或显示 "无告警" 占位

### Requirement: Overview 告警展示
OverviewSnapshot.StrategyRow.alert 字段 SHALL 填充最新告警摘要（如 "UPNL_LOSS 10:50"）。

Overview 前端 SHALL：
- Strategy List 表格的 Alert 列渲染告警文本，CRITICAL 红色，WARNING 黄色
- 新增 Alerts 区域，聚合显示所有策略的告警事件列表

#### Scenario: Overview 显示最新告警
- **WHEN** 策略 92201 触发 MAX LOSS 告警
- **THEN** Overview Strategy List 中 ID=92201 行的 Alert 列显示 "MAX_LOSS 13:49"，红色文字

#### Scenario: Overview 聚合多策略告警
- **WHEN** 策略 92201 和 92202 分别触发告警
- **THEN** Overview Alerts 区域按时间倒序显示两个策略的告警事件
