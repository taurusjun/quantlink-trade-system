## Why

实盘启动时 daily_init 中的 avgSpreadOri 值与实际市场 spread 严重不匹配，导致 AVG_SPREAD_AWAY 安全保护立即触发。手动修正 daily_init 后，旧进程 shutdown 又覆盖回错误值。根因是 Go 实现的 daily_init 保存逻辑与 C++ `SaveMatrix2` 语义不一致。

## What Changes

- 修正 `handleSquareoffLocked()` 中 daily_init 保存字段语义：`NetposYtd1 = NetposPass`（全部仓位），`Netpos2day1 = 0`（固定），对齐 C++ SaveMatrix2
- 移除 `main.go` shutdown 中的重复 daily_init 保存逻辑，避免覆盖策略内部已保存的正确值
- 新增单元测试验证 HandleSquareoff 保存的 daily_init 字段正确性

## Capabilities

### New Capabilities

无

### Modified Capabilities

无（本次为 bug 修复，不涉及 spec 级别的需求变更）

## Impact

- `tbsrc-golang/pkg/strategy/pairwise_arb.go` — HandleSquareoff 保存逻辑
- `tbsrc-golang/cmd/trader/main.go` — shutdown 流程
- `tbsrc-golang/pkg/strategy/pairwise_arb_test.go` — 新增测试用例
- C++ 参考: `tbsrc/Strategies/PairwiseArbStrategy.cpp:653-685` (SaveMatrix2)
