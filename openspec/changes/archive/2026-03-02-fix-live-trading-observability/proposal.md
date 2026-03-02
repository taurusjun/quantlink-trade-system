## Why

2026-03-02 日盘实盘交易中发现三个可观测性问题：策略发了 171 单、成交 16 笔，但 nohup.out 中无任何订单/成交日志；周末后 avgSpreadRatio 从 371 漂移到 500+ 需手动修复 daily_init；counter_bridge HTTP 8082 查询端口阻塞导致 Dashboard Account Table 为空。

## What Changes

- 策略层（ExecutionStrategy / PairwiseArbStrategy）增加发单、成交回报、撤单、状态变化的日志输出
- PairwiseArbStrategy 激活时增加 avgSpreadRatio 自动修复逻辑：若启动后首笔行情 |currSpread - avgSpread| > AVG_SPREAD_AWAY，自动重置均值
- counter_bridge HTTP 查询改为异步超时模式，避免 CTP 查询阻塞 HTTP 线程

## Capabilities

### New Capabilities
- `strategy-trade-logging`: 策略层订单生命周期日志 — 发单、成交、撤单、风控触发、状态变化原因
- `avgspread-auto-reset`: avgSpreadRatio 跨天漂移自动检测与重置
- `counter-bridge-async-query`: counter_bridge HTTP 查询异步化，避免 CTP 查询死锁

### Modified Capabilities

## Impact

- `tbsrc-java/src/main/java/com/quantlink/trader/strategy/ExecutionStrategy.java` — 增加订单日志
- `tbsrc-java/src/main/java/com/quantlink/trader/strategy/PairwiseArbStrategy.java` — 增加订单日志 + avgSpread 自动重置
- `gateway/src/counter_bridge.cpp` — HTTP 查询异步化
- `gateway/plugins/ctp/src/ctp_td_plugin.cpp` — CTP 查询超时保护
