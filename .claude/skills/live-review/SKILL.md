---
name: live-review
description: 交易复盘分析。读取当日日志，统计交易数据，分析策略表现，生成下一步建议。
---

对当日（或指定日期）的交易进行复盘分析。

**输入**: 可选参数 `<strategy_id>` 和 `<date>`，默认 `92201` 和今天日期（YYYYMMDD）。

## 步骤

### 0. 并行启动日志分析 Agent

**在开始步骤 1 的同时**，使用 Agent 工具在后台启动 `trading-log-analyzer` agent（subagent_type="trading-log-analyzer"，run_in_background=true）：

**传入 prompt**:
```
分析以下交易日志，检查 bugs、错误、不一致和异常行为。

策略 ID: <strategy_id>
日期: <date>

日志文件位置（deploy_java 目录下）：
- 策略日志: deploy_java/log/log.control.*.YYYYMMDD 和 deploy_java/log/nohup.control.*.YYYYMMDD
- Counter Bridge: deploy_java/log/counter_bridge.YYYYMMDD.log
- MD SHM Feeder: deploy_java/log/md_shm_feeder.YYYYMMDD.log

如果日志在归档目录中: deploy_java/log/YYYYMMDD/

重点关注：订单生命周期完整性、持仓一致性、SHM通信错误、CTP连接问题。
```

该 agent 使用 sonnet 模型，在后台并行运行，不阻塞后续步骤。

### 1. 收集日志数据

读取以下日志文件（deploy_java 目录下）：

**策略日志**（主要数据源）：
```bash
# 新格式日志（Java 版本）
cat deploy_java/log/log.control.<CONTROL_BASENAME>.<date>
cat deploy_java/log/nohup.control.<CONTROL_BASENAME>.<date>
```

**C++ 网关日志**：
```bash
cat deploy_java/log/counter_bridge.<date>.log
cat deploy_java/log/md_shm_feeder.<date>.log
```

如果日志不在当前 log/ 目录，检查归档子目录 `log/<date>/`。

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

### 7. 合并日志分析结果并生成复盘报告

**等待步骤 0 启动的 trading-log-analyzer agent 完成**（通常此时已完成），获取其分析报告。

将日志分析中发现的问题整合到复盘报告中，输出结构化报告：

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

### 日志分析（来自 trading-log-analyzer）
（整合 agent 发现的 bugs、错误、不一致等问题）
（按严重程度排序：Critical > High > Medium > Low）
（如果 agent 未发现问题，标注"日志分析未发现异常"）

### Daily Init 状态
（保存的 avgPx、avgSpreadRatio、持仓）
```

### 8. 生成建议并审查（Agent 迭代循环）

完成步骤 1-7 的复盘报告后，进入 **建议生成 → 可行性审查** 的迭代循环。

#### 8a. 调用 trading-review-advisor Agent 生成建议

使用 Agent 工具启动 `trading-review-advisor` agent（subagent_type="trading-review-advisor"），将步骤 1-7 收集到的所有数据作为 prompt 传入：

**传入内容**:
- 步骤 3 的交易统计数据（订单数、成交数、成交率、PnL 估算）
- 步骤 4 的策略行为分析（价差、状态变化、异常事件）
- 步骤 5 的 Counter Bridge 统计
- 步骤 6 的 daily_init 状态
- 当前 model 参数（从 `deploy_java/live/models/model.*.par.txt.<strategy_id>` 读取）
- 当前策略配置（从 `deploy_java/config/trader.<strategy_id>.yaml` 读取）

**要求 agent 输出**:
- 短期建议（1-5个交易日），每条包含：建议、证据、预期效果、风险
- 长期建议（数周至数月），每条包含：建议、证据、实施路径、预期效果、风险与对策

#### 8b. 调用 review-feasibility-checker Agent 审查建议

使用 Agent 工具启动 `review-feasibility-checker` agent（subagent_type="review-feasibility-checker"），将 8a 生成的所有建议传入：

**传入内容**:
- trading-review-advisor 输出的全部建议
- 当前 model 参数和策略配置（供审查参考）

**agent 输出格式**:
```
### 建议 1: [标题]
**判断: ✅ 可行** 或 **判断: ❌ 不可行**
**理由**: [2-4句话]

...

### 总结
可行: X 条 | 不可行: Y 条
```

#### 8c. 检查审查结果，决定是否迭代

- **如果所有建议都通过（全部 ✅）**：输出最终建议清单，流程结束
- **如果有建议未通过（存在 ❌）**：
  1. 收集所有被否决的建议及其否决理由
  2. 回到步骤 8a，调用 trading-review-advisor agent 重新生成建议
     - 将被否决的建议和理由作为上下文传入
     - 要求 agent 针对被否决的建议提出替代方案，已通过的建议保留不变
  3. 对新生成的替代建议，回到步骤 8b 进行审查
  4. 重复此循环，直到所有建议全部通过 ✅
  5. **最多迭代 3 轮**，如果 3 轮后仍有未通过的建议，将其标注为"存在争议"并附上否决理由，一并输出

#### 8d. 输出最终建议报告

将通过审查的建议整合为最终报告：

```markdown
## 复盘建议（已通过可行性审查）

### 短期建议（1-5个交易日）
1. **建议**: [内容] — ✅ 可行
   **证据**: ...
   **预期效果**: ...

2. ...

### 长期建议（数周至数月）
1. **建议**: [内容] — ✅ 可行
   **证据**: ...
   **实施路径**: ...

### 存在争议的建议（如有）
1. **建议**: [内容] — ❌ 未通过审查
   **否决理由**: ...
```

## 注意事项

- 复盘分析不修改任何文件，纯只读操作
- PnL 估算不含手续费，仅供参考
- 如果日志量很大（>1000 行），使用 grep 提取关键标签而不是读全文件
- 如果用户提供了具体关注点（如"为什么追单这么多"），重点分析该方向
- Agent 迭代循环最多 3 轮，避免无限循环
- trading-review-advisor 和 review-feasibility-checker 使用 opus 模型，确保分析质量
- trading-log-analyzer 使用 sonnet 模型，在后台并行运行（run_in_background=true），不阻塞主流程
- 如果 trading-log-analyzer 发现 Critical 级别问题，在复盘报告中醒目标注
