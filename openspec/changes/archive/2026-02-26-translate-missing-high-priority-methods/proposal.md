## Why

C++ → Java 翻译审计发现 ExecutionStrategy 和 PairwiseArbStrategy 中 7 个 HIGH 优先级方法遗漏未翻译，影响核心交易逻辑的完整性。需要逐行对照补全。

## What Changes

- 补全 ExecutionStrategy 中 5 个遗漏方法：HandleTimeLimitSquareoff, SetQuantAhead, GetBidPrice/GetAskPrice(基类), SendBidOrder/SendAskOrder(基类), SetTargetValue
- 补全 PairwiseArbStrategy 中 3 个遗漏方法：GetBidPrice_second, GetAskPrice_second, SendTCacheLeg1Pos
- 补全 ExecutionStrategy 中辅助方法 CalculatePNL(double, double) 双参数重载
- 所有方法严格按 C++ 原代码逐行翻译，添加 `// 迁移自:` 和 `// C++:` 注释

## Capabilities

### New Capabilities
- `missing-method-translation`: 补全 ExecutionStrategy 和 PairwiseArbStrategy 中遗漏的 HIGH 优先级方法翻译

### Modified Capabilities

（无）

## Impact

- `tbsrc-java/src/main/java/com/quantlink/trader/strategy/ExecutionStrategy.java` — 新增 7 个方法
- `tbsrc-java/src/main/java/com/quantlink/trader/strategy/PairwiseArbStrategy.java` — 新增 3 个方法
- 可能需要在 ThresholdSet.java、Instrument.java、ConfigParams.java 中补全缺失字段
