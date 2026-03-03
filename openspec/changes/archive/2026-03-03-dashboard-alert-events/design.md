## Context

当前策略触发的风控告警（UPNL LOSS、MAX LOSS、AVG_SPREAD_AWAY、END TIME 等）仅写入日志文件，需要 `tail -f nohup.out.*` 才能看到。Dashboard（端口 9201）和 Overview（端口 8080）页面没有告警展示能力。

现有数据流：
- `ExecutionStrategy.sendAlert()` → 日志 WARNING + `sendMonitorStratDetail()`（仅更新 netpos 状态，不保存事件）
- `SnapshotCollector` 每秒采集 `DashboardSnapshot` → WebSocket 推送前端
- `OverviewServer` 聚合各 trader 的 `DashboardSnapshot` → `OverviewSnapshot` → WebSocket 推送前端
- `OverviewSnapshot.StrategyRow.alert` 字段已预留，但始终为空

## Goals / Non-Goals

**Goals:**
- 策略触发告警时，Dashboard 和 Overview 页面实时展示告警事件
- 保留告警历史（当天），用户打开页面即可看到历史告警
- 告警类型覆盖：UPNL LOSS、STOP LOSS、MAX LOSS、MAX ORDERS、MAX TRADED、AVG_SPREAD_AWAY、END TIME、REJECT LIMIT
- 最小改动量，利用现有 WebSocket 推送通道

**Non-Goals:**
- 外部通知（邮件、微信、声音等）— 后续可扩展
- 告警确认/消除机制 — 本次只做展示
- 自定义告警规则 — 本次只采集已有的 C++ 对齐告警点

## Decisions

### 1. 告警采集方式：策略层内置 AlertCollector

**方案**: 在 `ExecutionStrategy` 中新增 `AlertCollector` 实例（环形缓冲区），所有触发告警的代码点调用 `alertCollector.add(event)` 记录事件。

**备选方案**: 在日志 handler 中拦截 WARNING 级别日志并解析 → 太脆弱，依赖日志格式。

**理由**: 告警调用点已经明确（`sendAlert()`、`checkSquareoff()` 中的各种 limit 检查、`PairwiseArbStrategy` 中的 AVG_SPREAD_AWAY），直接在这些位置记录事件最可靠。

### 2. 告警事件数据结构

```java
public class AlertEvent {
    public long timestamp;        // System.currentTimeMillis()
    public String level;          // "WARNING" | "CRITICAL"
    public String type;           // "UPNL_LOSS" | "MAX_LOSS" | "AVG_SPREAD_AWAY" | "END_TIME" | ...
    public String message;        // 人可读描述
    public String symbol;         // 触发合约
    public int strategyId;        // 策略 ID
    public Map<String, Object> details;  // 可选附加数据 (netpos, pnl, threshold 等)
}
```

**级别划分**:
- `WARNING`: 临时性告警，策略可自动恢复（UPNL_LOSS → 15 分钟冷却恢复）
- `CRITICAL`: 策略永久退出当天交易（MAX_LOSS、END_TIME、MAX_ORDERS）

### 3. 传输方式：复用现有 WebSocket 快照通道

**方案**: 在 `DashboardSnapshot` 中新增 `alerts` 字段（`List<AlertEvent>`），随每秒快照一起推送。

**备选方案**: 新开独立 WebSocket 通道 `/ws/alerts` → 增加复杂度，前端需要管理两个连接。

**理由**: 告警事件量很小（一天通常 0-5 条），附加在现有快照中几乎无额外开销，前端只需在现有 `onmessage` 处理中增加 `data.alerts` 渲染。Overview 同理，`OverviewSnapshot` 新增 `alerts` 聚合字段。

### 4. AlertCollector 设计：环形缓冲区

- 固定容量 100 条（一天不可能超过）
- 线程安全（ConcurrentLinkedDeque 或 synchronized ArrayList）
- 每次快照采集时读取全量列表推送给前端
- 不需要增量推送 — 告警量极小，全量推送无性能问题

### 5. 前端展示

**Dashboard（dashboard.html）**:
- 在页面顶部或底部新增告警面板（Alert Panel）
- 按时间倒序展示告警列表
- WARNING 黄色背景，CRITICAL 红色背景
- 显示：时间、类型、消息

**Overview（index.html 由 OverviewServer 提供）**:
- `StrategyRow.alert` 字段填充最新告警摘要（如 "UPNL LOSS 10:50"）
- Strategy List 表格的 Alert 列渲染告警状态
- 新增独立的 Alerts 表格，聚合所有策略的告警事件

## Risks / Trade-offs

**[风险] 告警采集点遗漏** → 对照 C++ 原代码 `SendAlert()` 和 `CheckSquareoff()` 所有路径，逐一确认。新增告警类型时需要同步更新 AlertCollector 调用。

**[风险] WebSocket 消息体增大** → 告警量极小（通常 0-5 条/天），JSON 增量 < 1KB，可忽略。

**[权衡] 全量推送 vs 增量推送** → 选择全量推送。告警量小，简单可靠；增量推送需要前端维护状态，复杂度高收益低。

**[权衡] 告警不持久化** → 本次不写文件/数据库，进程重启后告警历史丢失。当天的运维场景足够，后续可扩展持久化。
