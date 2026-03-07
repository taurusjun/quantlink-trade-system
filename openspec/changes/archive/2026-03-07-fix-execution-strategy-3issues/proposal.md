## Why

ExecutionStrategy.java 与 C++ 原代码对齐验证中发现 3 处缺失，影响后续其他策略（非 PairwiseArb）的正确运行。当前 PairwiseArb 不使用 SelfBook 且有自己的 handleSquareON override，因此暂无功能影响，但必须在迁移更多策略之前补齐。代码修复已完成，需要补齐单元测试用例。

## What Changes

- **撤单拒绝处理 (CANCELREQ_PAUSE)**: 添加 `lastCancelReqRejectSet` / `lastCancelRejectTime` / `lastCancelRejectOrderID` 字段和 CANCELREQ_PAUSE 计时器逻辑，防止撤单被拒后无效重复撤单
- **自营订单簿缓存删除 (bidMapCacheDel / askMapCacheDel)**: 添加撤单中的 self-book 缓存跟踪 Map，完善 removeOrder() 中的条件性清理逻辑
- **handleSquareON() 基类方法**: ExecutionStrategy 添加基类 handleSquareON()，PairwiseArbStrategy 改为调用 super.handleSquareON()
- **配套字段补齐**: ConfigParams 添加 `fillOnCxlReject`/`bSelfBook`；Instrument 添加 `bSnapshot`/`adjustBookWithAggCxl`；ExecutionStrategy 添加 `sweepOrdMap`
- **单元测试**: 为上述 3 项修复编写覆盖核心逻辑的单元测试

## Capabilities

### New Capabilities

（无新能力，均为现有能力的补齐）

### Modified Capabilities

- `strategy`: ExecutionStrategy 补齐 C++ 对齐缺失的 3 项功能 + 单元测试

## Impact

- `tbsrc-java/src/main/java/com/quantlink/trader/strategy/ExecutionStrategy.java` — 主要修改
- `tbsrc-java/src/main/java/com/quantlink/trader/strategy/PairwiseArbStrategy.java` — handleSquareON 添加 super 调用
- `tbsrc-java/src/main/java/com/quantlink/trader/core/ConfigParams.java` — 添加字段
- `tbsrc-java/src/main/java/com/quantlink/trader/core/Instrument.java` — 添加字段
- `tbsrc-java/src/test/java/com/quantlink/trader/strategy/ExecutionStrategyTest.java` — 新增测试
