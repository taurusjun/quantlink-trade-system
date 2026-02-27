## Why

C++ 非 arb 策略（PassiveStrategy、AggressiveStrategy 独立使用时）依赖 Indicator 系统来计算公允价 `targetPrice`：行情更新 → 指标更新 → CalculateTargetPNL → SetTargetValue → SendOrder。Java 当前 `indCallback` 只实现了 `useArbStrategy==1` 路径（直接传固定值），非 arb 路径的 `CalculateTargetPNL` 完全缺失。后续迁移其他策略时这是功能阻塞。

## What Changes

- 新建 `Indicator.java` 基类（迁移自 `tbsrc/Indicators/include/Indicator.h`）
- 新建 `Dependant.java` 指标（迁移自 `tbsrc/Indicators/Dependant.cpp`）
- 新建 `IndElem.java` 指标元素（迁移自 `TradeBotUtils.h:IndElem`）
- 新建 `CalculateTargetPNL.java`（迁移自 `TradeBotUtils.cpp:CalculatePNL`）
- 修改 `SimConfig.java` 添加 `indicatorList` 和 `calculatePNL` 字段
- 修改 `Instrument.java` 添加 `SubscribeTBPriceType`、`GetTBPriceType`、strat book 价格方法、`indList` 字段
- 修改 `CommonClient.java` 添加 `update()` 方法（更新指标），`indCallback` 传入 `indicatorList`
- 修改 `TraderMain.java` 的 `indCallback` 支持非 arb 路径

## Capabilities

### New Capabilities
- `java-indicator-framework`: Java Indicator 框架，提供指标基类、Dependant 指标、CalculateTargetPNL 定价引擎

### Modified Capabilities
- `strategy`: indCallback 支持非 arb 路径（CalculateTargetPNL → SetTargetValue）

## Impact

- 新文件: `Indicator.java`, `Dependant.java`, `IndElem.java`, `CalculateTargetPNL.java`
- 修改: `SimConfig.java`, `Instrument.java`, `CommonClient.java`, `TraderMain.java`
- 测试: `IndicatorTest.java`, `CalculateTargetPNLTest.java`，更新现有测试
