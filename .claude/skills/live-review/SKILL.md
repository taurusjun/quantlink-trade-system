---
name: live-review
description: 交易复盘分析。读取当日日志，统计交易数据，分析策略表现，生成下一步建议。
---

对当日（或指定日期）的交易进行复盘分析。

**输入**: 可选参数 `<strategy_id>` 和 `<date>`，默认 `92201` 和今天日期（YYYYMMDD）。

## 步骤

### 1. 收集日志数据

读取以下日志文件（deploy_java 目录下）：

**策略日志**（主要数据源）：
```bash
cat deploy_java/nohup.out.<strategy_id>.<date>
```

**C++ 网关日志**：
```bash
cat deploy_java/log/counter_bridge.<date>.log
cat deploy_java/log/md_shm_feeder.<date>.log
```

如果日志文件不存在，检查 deploy_new 目录（Go 版本）作为 fallback。

### 2. 解析订单生命周期

从策略日志中提取以下标签的记录：

| 标签 | 说明 | 关键字段 |
|------|------|---------|
| `[ORDER-NEW]` | 新订单 | symbol, side, price, qty, orderID |
| `[TRADE]` | 成交 | symbol, side, price, qty, orderID, cumQty, remainQty |
| `[ORDER-CANCEL]` | 撤单 | orderID, symbol, side |
| `[CANCEL-CONFIRM]` | 撤单确认 | orderID |
| `[ORDER-REJECT]` | 订单拒绝 | orderID, symbol, price |
| `[CANCEL-REJECT]` | 撤单拒绝 | orderID |
| `[AGG-ORDER]` | 追单 | symbol, side, price |
| `[PAIR-ORS]` | 配对路由 | firstStrat/secondStrat, side, price, qty |
| `[PAIR-EXIT]` | 策略退出 | avgSpread, ytd, netpos |
| `[STATE]` | 状态变化 | aggFlat, onExit, onFlat, reason |

### 3. 统计交易数据

计算以下指标：

**订单统计**:
- 总订单数（ORDER-NEW 计数）
- 总成交数（TRADE 计数）
- 撤单数（ORDER-CANCEL 计数）
- 拒绝数（ORDER-REJECT 计数）
- 成交率 = 成交数 / 订单数

**成交统计**（按 symbol 分组）:
- 买入成交：笔数、总手数、均价
- 卖出成交：笔数、总手数、均价

**配对交易**:
- 配对交易笔数（PAIR-ORS 计数 / 2，因为每对有两腿）
- 追单次数（AGG-ORDER 计数）

**PnL 估算**:
- 基于成交记录计算日内 PnL（买卖配对）
- 对于 pair trading：计算价差 PnL = (卖均价 - 买均价) * 手数，两个 symbol 分别计算
- 注意：这是粗略估算，不含手续费和滑点

### 4. 分析策略行为

**价差行为**:
- 从日志中提取 avgSpreadRatio 相关记录
- 检查 AVG_SPREAD_AWAY 是否触发（搜索 `[AVG-SPREAD-DRIFT]`）
- 如果有 handleSquareON 记录，分析激活时的价差重置

**状态变化**:
- 提取所有 `[STATE]` 记录，分析状态转换链路
- 检查是否有异常状态转换（如频繁的 stopLoss 触发）
- 分析 endTimeAgg / END TIME 触发时间

**异常检测**:
- ORDER-REJECT 或 CANCEL-REJECT：分析原因
- 频繁撤单：检查是否有价格追逐问题
- 成交延迟：对比 ORDER-NEW 和 TRADE 时间差
- counter_bridge 错误：检查 C++ 日志中的异常

### 5. 检查 Counter Bridge 统计

从 counter_bridge 日志中提取：
```
[Statistics] Total=N Success=N Failed=N Filled=N Rejected=N
```

对比策略日志和网关日志的订单数是否一致。

### 6. 检查 daily_init 保存

确认策略退出后 daily_init 文件已正确保存：
```bash
cat deploy_java/live/data/daily_init.<strategy_id>
```

检查 avgPx、avgSpreadRatio、netpos 等字段是否合理。

### 7. 生成复盘报告

输出结构化报告，包含以下章节：

```markdown
## 交易复盘 — <date> <session>

### 概览
| 指标 | 值 |
|------|---|
| 交易时段 | day/night |
| 运行时长 | HH:MM:SS |
| 总订单数 | N |
| 总成交数 | N |
| 成交率 | N% |
| 预估 PnL | ±X |

### 成交明细
（按时间排序的成交记录表）

### 配对交易分析
（价差、追单次数、配对成功率）

### 状态变化记录
（状态转换时间线）

### 异常事件
（拒绝、错误、drift 等）

### Daily Init 状态
（保存的 avgPx、avgSpreadRatio、持仓）
```

### 8. 生成下一步建议

基于分析结果给出具体建议：

**参数调优建议**:
- 如果成交率低 → 建议调整 BEGIN_PLACE 或价格偏移
- 如果追单频繁 → 分析追单成功率，建议调整追单逻辑
- 如果 PnL 为负 → 分析亏损原因（滑点/价差回归/方向错误）

**风控建议**:
- 如果 stopLoss 频繁触发 → 建议调整 stopLoss 阈值或 MAX_SIZE
- 如果 avgSpread drift 严重 → 建议检查模型参数或市场结构变化

**操作建议**:
- 下一个交易时段的注意事项
- 是否需要调整 model 参数（热加载方式）
- daily_init 是否需要手动修正

## 注意事项

- 复盘分析不修改任何文件，纯只读操作
- PnL 估算不含手续费，仅供参考
- 如果日志量很大（>1000 行），使用 grep 提取关键标签而不是读全文件
- 如果用户提供了具体关注点（如"为什么追单这么多"），重点分析该方向
