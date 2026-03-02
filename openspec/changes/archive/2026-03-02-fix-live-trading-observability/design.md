## Context

2026-03-02 日盘实盘交易（策略 92201，ag2603/ag2605 配对套利）中发现三个可观测性问题：

1. **策略日志缺失交易记录**：171 单、16 笔成交，nohup.out 中零条订单/成交日志。`sendNewOrder()` 只写内存 deque（`recordOrderEvent`），不输出到 logger；`orsCallBack()` 仅 log unknown orderID，trade/cancel/reject 全部静默。
2. **avgSpreadRatio 跨天漂移**：周末后 avgSpread=371 vs 市场=500+，手动修 daily_init 才能恢复。激活时 `handleSquareON()` 已有重置逻辑，但 mdCallBack 中的 AVG_SPREAD_AWAY 检查在激活前就触发了，导致策略直接 exit。
3. **counter_bridge HTTP 查询阻塞**：`HandleAccount()` 在 HTTP 线程上同步调用 `plugin->QueryAccount()`（5s mutex wait），CTP 回调延迟或死锁导致 8082 端口无响应，Dashboard Account Table 永久空白。

当前代码库：
- Java 策略：`ExecutionStrategy.java`（基类）、`PairwiseArbStrategy.java`（配对套利）
- C++ 网关：`counter_bridge.cpp`（HTTP server + 订单桥接）、`ctp_td_plugin.cpp`（CTP 交易插件）

## Goals / Non-Goals

**Goals:**
- 策略层订单生命周期全链路日志：发单、成交、撤单、拒绝、状态变化
- avgSpreadRatio 跨天漂移自动检测与重置，无需手动修改 daily_init
- counter_bridge HTTP 查询不阻塞，保证 Dashboard 可用性

**Non-Goals:**
- 不修改交易逻辑本身（阈值、下单条件、风控参数）
- 不增加新的监控 UI 或告警系统
- 不改变 CTP 查询的重试策略或频率
- 不修改 SHM 通信机制

## Decisions

### Decision 1: 日志插入点选择 — 在基类方法中统一添加

**选择**: 在 `ExecutionStrategy` 的 `sendNewOrder()`、`orsCallBack()`（及其子方法 `processTrade`、`processCancelConfirm`、`processNewReject`、`processCancelReject`）、`sendCancelOrder()` 中添加 `log.info`/`log.warning`。`PairwiseArbStrategy` 的 `orsCallBack()` 和 `sendAggressiveOrder()` 额外添加配对级别日志。

**理由**: 基类是所有订单操作的统一入口，在此添加保证所有策略子类自动获得日志。`recordOrderEvent()` 已有 in-memory deque，但不输出到 logger — 补充 logger 输出即可，不改变 recordOrderEvent 机制。

**替代方案**: 在 `recordOrderEvent` 中统一 log — 但该方法是通用的事件记录器，添加日志会让已有 caller 的日志格式不可控。直接在业务方法中 log 更清晰。

### Decision 2: avgSpread 自动重置 — 在激活时检测漂移

**选择**: 在 `PairwiseArbStrategy.handleSquareON()` 中，重置 avgSpreadRatio 后额外检查：如果重置前的旧值与当前市场价差之差 > AVG_SPREAD_AWAY，输出 warning 日志标记"自动修复漂移"。当前 `handleSquareON()` 已经将 avgSpreadRatio 重置为 live spread，所以激活时漂移已被修复。

**关键问题**: 真正的问题是 `mdCallBack()` 中的 AVG_SPREAD_AWAY 检查在策略 `active=false` 时也会触发 exit。需要修改 mdCallBack 中的 AVG_SPREAD_AWAY 检查，**仅在 active=true 时才触发 exit**。inactive 状态下遇到 AVG_SPREAD_AWAY 漂移只 log warning 但不 exit，等待用户激活（激活时 handleSquareON 会自动重置）。

**替代方案**: 在 mdCallBack 中自动重置 avgSpread — 但 C++ 原代码的 AVG_SPREAD_AWAY 检查是保护性退出，不应在行情回调中自动修改均值。保持激活时重置（已有逻辑）+ inactive 时跳过 exit 更安全。

### Decision 3: counter_bridge HTTP 查询 — 使用缓存 + 后台刷新

**选择**: HTTP handler 改为读取已缓存的查询结果（`GetCachedAccount()`），后台线程定期（每 10s）调用 `QueryAccount()` 刷新缓存。CTP 插件已有 `m_cached_account` 和 `GetCachedPositions()` 但未被 HTTP 端点使用。

**实现**:
1. `ctp_td_plugin.cpp`: 添加 `GetCachedAccount()` 方法（类似已有的 `GetCachedPositions()`）
2. `counter_bridge.cpp`: `HandleAccount()` 改为调用 `GetCachedAccount()`（非阻塞）
3. `counter_bridge.cpp`: 新增后台线程每 10s 调用 `QueryAccount()` 刷新缓存

**理由**: `GetCachedPositions()` 已存在但未接入 HTTP，说明代码库已预留了缓存模式。HTTP 线程零阻塞，CTP 查询移到后台专用线程避免与订单处理竞争 mutex。

**替代方案**: 使用 cpp-httplib 的 async response — 但 cpp-httplib 不原生支持 async response，改造复杂且无收益。

## Risks / Trade-offs

- **[日志量增大]** → 每笔订单/成交增加 1 行 info 日志。以 171 单/天计，增量极小，不影响性能。
- **[avgSpread inactive 跳过 exit]** → inactive 时即使 avgSpread 异常也不退出，理论上策略可以在 avgSpread 异常状态下被激活。→ `handleSquareON()` 激活时强制重置 avgSpread，保证激活后均值正确。
- **[缓存数据延迟]** → HTTP 返回的 account 数据最多 10s 延迟。→ Dashboard 只需展示级可用性，10s 延迟可接受。可在 HTTP response 中附加 `last_updated` 时间戳。
- **[后台查询线程 + 订单线程共享 mutex]** → `m_query_mutex` 被后台查询线程和 CTP 回调线程共享。→ 查询频率低（10s/次），mutex 争用极低，不影响订单处理延迟。
