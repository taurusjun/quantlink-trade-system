## MODIFIED Requirements

### Requirement: R2: 7 区域页面布局

页面包含以下 8 个区域（原 7 个 + 新增 Alerts 区域）：

**② 策略列表表格**（主体区域，占页面左侧）
列: Status | Alert | AT | Pro | ID | IP | ModelFile | StrategyType | Key | val | 1(L1持仓) | 2(L2持仓) | PNL | Time | Information | 操作(启动/停止/暂停/日志)

行为：
- 每行一个策略实例
- Status 着色: 运行中(绿)、无进程(灰)、未连接(黄)
- **Alert 列 SHALL 显示最新告警摘要文本（如 "UPNL_LOSS 10:50"），CRITICAL 红色文字，WARNING 黄色文字，无告警时留空**
- L1/L2 持仓着色: 正数绿、负数红、零灰
- PNL 着色: 正数绿、负数红
- 操作按钮: 启动/停止/暂停/日志

**⑧ Alerts 区域**（底部，与 Spread Trades / Orders / Position / Fills 同级）
列: Time | ID | Level | Type | Message
- 聚合所有策略的告警事件
- 按时间倒序排列
- CRITICAL 行红色背景，WARNING 行黄色背景
- 数据来源: 各 trader DashboardSnapshot.alerts 聚合

#### Scenario: Alert 列显示最新告警
- **WHEN** 策略 92201 在 10:50 触发 UPNL_LOSS 告警
- **THEN** Strategy List 中 ID=92201 行的 Alert 列显示 "UPNL_LOSS 10:50"，黄色文字

#### Scenario: Alert 列显示 CRITICAL 告警
- **WHEN** 策略 92201 在 13:49 触发 MAX_LOSS 告警（CRITICAL）
- **THEN** Alert 列更新为 "MAX_LOSS 13:49"，红色文字（覆盖之前的 WARNING 告警）

#### Scenario: Alerts 区域聚合
- **WHEN** 多个策略分别触发告警
- **THEN** Alerts 区域按时间倒序展示所有告警，每条包含策略 ID

### Requirement: R3: 数据刷新

- OverviewServer 作为 WS 客户端连接各 trader `/ws`，实时接收推送
- 每次收到任一 trader 推送 → 重新聚合 → 推送给前端 WebSocket
- 前端收到推送后更新所有区域（**含 Alerts 区域和 Alert 列**）
- 心跳: 30 秒 ping
- 断线: trader 下线 → onClose → 标记"未连接" → 5 秒后自动重连

#### Scenario: 告警实时刷新
- **WHEN** trader 推送包含新告警的 DashboardSnapshot
- **THEN** Overview 页面在下一次 WebSocket 推送后更新 Alerts 区域和对应策略的 Alert 列
