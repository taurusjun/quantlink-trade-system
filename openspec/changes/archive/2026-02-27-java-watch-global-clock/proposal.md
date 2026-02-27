## Why

C++ 使用 `Watch` 全局单例作为行情驱动的模拟时钟，所有策略通过 `Watch::GetUniqueInstance()->GetCurrentTime()` 获取统一时间。Java 当前各策略在 `mdCallBack()` 中各自从行情读取 `exchTS`，与 C++ 架构不一致，缺少单调递增保护和全局一致性。

## What Changes

- 新建 `Watch.java` 全局时钟单例（迁移自 `tbsrc/common/Watch.hpp` + `Watch.cpp`）
- `CommonClient.sendINDUpdate()` 中统一调用 `Watch.updateTime()`，对齐 C++ `CommonClient.cpp:412-415`
- `TraderMain.init()` 中创建 Watch 单例，对齐 C++ `main.cpp:650`
- `ExecutionStrategy` 中 ~14 处 `exchTS` 读取替换为 `Watch.getInstance().getCurrentTime()`
- `PairwiseArbStrategy` 中 2 处 `exchTS` 读取替换为 `Watch.getInstance().getCurrentTime()`
- 新建 `WatchTest.java`（17 个测试），更新 4 个现有测试文件

## Capabilities

### New Capabilities
- `java-watch-clock`: Java Watch 全局时钟单例，提供行情驱动的单调递增时钟、TimeListener 订阅机制、`getNanoSecsFromEpoch` 时间转换

### Modified Capabilities
- `strategy`: ExecutionStrategy/PairwiseArbStrategy 时间获取方式从直接读取 `exchTS` 改为通过 Watch 全局时钟

## Impact

- 新文件: `tbsrc-java/.../core/Watch.java`, `tbsrc-java/.../core/WatchTest.java`
- 修改文件: `CommonClient.java`, `TraderMain.java`, `ExecutionStrategy.java`, `PairwiseArbStrategy.java`
- 测试文件: `ExecutionStrategyTest.java`, `PairwiseArbStrategyTest.java`, `ExtraStrategyTest.java`, `CommonClientTest.java`
- 全部 215 个测试通过
