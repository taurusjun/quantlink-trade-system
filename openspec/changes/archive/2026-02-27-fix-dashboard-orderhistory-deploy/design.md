# Design: Dashboard 订单历史修复

## 架构决策

### Decision: 事件级记录 vs 快照推断
采用事件级记录（在订单事件发生时立即写入 orderHistory），而非快照推断（OrderHistoryTracker 的 disappear=TRADED 逻辑）。
两者共存：orderHistory 提供准确的事件数据，OrderHistoryTracker 作为补充层保留。

### Decision: ArrayDeque 环形缓冲区
使用 `ArrayDeque<OrderHistoryEntry>` 容量 100，超出时 pollFirst 淘汰最旧条目。
每个 ExtraStrategy 实例独立维护自己的 orderHistory。

## 文件变更

| 文件 | 变更 |
|------|------|
| `ExecutionStrategy.java` | 添加 OrderHistoryEntry 内部类 + orderHistory 字段 + recordOrderEvent() 方法 |
| `DashboardSnapshot.java` | collectLeg() 从 orderHistory 读取 + ordMap 补充 |
| `TraderMain.java` | 设置 Instrument.instrument 字段 |
| `model.*.par.txt.92201` | 添加 BID_SIZE/BID_MAX_SIZE/ASK_SIZE/ASK_MAX_SIZE |
| `.gitignore` | 添加 io/, org/, .gateway_mode |
