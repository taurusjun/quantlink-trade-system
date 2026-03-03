## Why

实盘交易中策略会触发各类风控告警（UPNL LOSS、MAX LOSS、AVG_SPREAD_AWAY、END TIME 等），这些事件目前只能从日志文件中查看。运维人员需要 `tail -f nohup.out.*` 并手动搜索才能发现告警，无法在 Dashboard 或 Overview 页面上实时看到。这导致告警响应延迟，影响实盘运维效率。

## What Changes

- 在策略层新增告警事件收集器（AlertCollector），收集 `sendAlert()`、AVG_SPREAD_AWAY exit、UPNL/STOP_LOSS 触发、onFlat/onExit 状态变更等事件
- 扩展 DashboardSnapshot，新增 `alerts` 字段，通过 WebSocket 实时推送告警事件到前端
- 扩展 OverviewSnapshot 的 `StrategyRow.alert` 字段（当前为预留空字符串），展示最新告警摘要
- Dashboard 前端新增告警事件列表 UI（时间、级别、原因），醒目展示
- Overview 前端在 Strategy List 表格中展示告警状态列

## Capabilities

### New Capabilities
- `alert-events`: 策略告警事件的采集、传输和前端展示

### Modified Capabilities
- `overview-page`: StrategyRow.alert 字段从预留空值改为实时告警摘要
- `strategy-trade-logging`: sendAlert() 调用点同时写入 AlertCollector

## Impact

- **Java 策略层**: `ExecutionStrategy.java`（sendAlert 扩展）、`PairwiseArbStrategy.java`（AVG_SPREAD_AWAY 事件采集）
- **API 层**: `DashboardSnapshot.java`（新增 alerts 字段）、`OverviewSnapshot.java`（alert 字段填充）、`SnapshotCollector.java`（采集告警）
- **前端**: `dashboard.html`（新增告警面板）、Overview 页面（alert 列渲染）
- **无破坏性变更**: 纯新增功能，现有 WebSocket 协议向后兼容（新增字段，旧前端忽略）
