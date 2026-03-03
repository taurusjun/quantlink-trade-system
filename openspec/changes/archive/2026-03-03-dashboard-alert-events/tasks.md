## 1. AlertEvent 数据结构与 AlertCollector

- [x] 1.1 创建 `AlertEvent.java` 数据类（timestamp, level, type, message, symbol, strategyId）
- [x] 1.2 创建 `AlertCollector.java`（ConcurrentLinkedDeque 环形缓冲区，容量 100，add/getAll 方法）
- [x] 1.3 在 `ExecutionStrategy` 中添加 `alertCollector` 字段并在构造函数中初始化

## 2. 告警采集点接入

- [x] 2.1 `ExecutionStrategy.sendAlert()` 中调用 `alertCollector.add()`，从 reason 参数解析告警类型
- [x] 2.2 `ExecutionStrategy.checkSquareoff()` UPNL_LOSS / STOP_LOSS 触发点添加 alertCollector.add()
- [x] 2.3 `ExecutionStrategy.checkSquareoff()` END_TIME / MAX_LOSS / MAX_ORDERS / MAX_TRADED 触发点添加 alertCollector.add()
- [x] 2.4 `ExecutionStrategy.checkSquareoff()` END_TIME_AGG 触发点添加 alertCollector.add()
- [x] 2.5 `ExecutionStrategy` REJECT_LIMIT 触发点添加 alertCollector.add()
- [x] 2.6 `PairwiseArbStrategy.mdCallBack()` AVG_SPREAD_AWAY 触发点添加 alertCollector.add()

## 3. DashboardSnapshot 扩展

- [x] 3.1 `DashboardSnapshot` 新增 `alerts` 字段（`List<AlertEvent>`）
- [x] 3.2 `DashboardSnapshot.collect()` 中从 `firstStrat.alertCollector.getAll()` 读取告警列表填入 snapshot

## 4. OverviewSnapshot 扩展

- [x] 4.1 `OverviewSnapshot` 新增 `alerts` 字段（`List<AlertRow>`），定义 AlertRow 数据类
- [x] 4.2 `OverviewSnapshot.StrategyRow.alert` 字段填充最新告警摘要（类型 + 时间）
- [x] 4.3 `OverviewSnapshot.aggregate()` 中聚合各策略的 alerts，按时间倒序排列

## 5. Dashboard 前端

- [x] 5.1 `dashboard.html` 新增告警面板 UI（Alert Panel），位于页面顶部或状态区域下方
- [x] 5.2 WebSocket onmessage 处理中解析 `data.alerts`，渲染告警列表（时间倒序，WARNING 黄色、CRITICAL 红色）

## 6. Overview 前端

- [x] 6.1 Overview 页面 Strategy List 表格中 Alert 列渲染告警文本和颜色
- [x] 6.2 Overview 页面新增 Alerts 区域，展示聚合告警事件列表

## 7. 测试验证

- [x] 7.1 AlertCollector 单元测试（add、getAll、溢出淘汰）
- [x] 7.2 模拟器端到端测试：触发 UPNL LOSS → 确认 Dashboard alerts 字段包含告警事件
