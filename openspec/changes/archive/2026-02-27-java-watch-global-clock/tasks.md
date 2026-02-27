## 1. Watch 全局时钟单例

- [x] 1.1 新建 `Watch.java`（`tbsrc-java/src/main/java/com/quantlink/trader/core/Watch.java`），迁移自 `tbsrc/common/Watch.hpp` + `Watch.cpp`，包含单例模式、updateTime 单调递增保护、TimeListener 机制、getNanoSecsFromEpoch 工具方法
- [x] 1.2 新建 `WatchTest.java`（`tbsrc-java/src/test/java/com/quantlink/trader/core/WatchTest.java`），覆盖单例创建、单调递增保护、零值重置、TimeListener 每秒触发、getNanoSecsFromEpoch 转换（含 useExchTS 偏移）

## 2. CommonClient 统一更新 Watch

- [x] 2.1 在 `CommonClient.sendINDUpdate()` 中添加 `Watch.updateTime()` 调用，对齐 C++ `CommonClient.cpp:412-415`，根据 `ConfigParams.useExchTS` 选择 timestamp 或 exchTS*10^6 路径
- [x] 2.2 更新 `CommonClientTest.java` 添加 Watch 初始化/重置

## 3. TraderMain 初始化 Watch

- [x] 3.1 在 `TraderMain.init()` 中添加 `Watch.createInstance(0)` 调用，对齐 C++ `main.cpp:650`

## 4. ExecutionStrategy exchTS 替换

- [x] 4.1 替换 `ExecutionStrategy.mdCallBack()` 中 exchTS 赋值为 `Watch.getInstance().getCurrentTime()`
- [x] 4.2 替换 `checkSquareoff()` 中约 10 处 exchTS 读取为 `watchTime`（局部变量取自 Watch）
- [x] 4.3 替换 `handleOrderRejection()` 中 exchTS 读取为 `Watch.getInstance().getCurrentTime()`
- [x] 4.4 更新 `ExecutionStrategyTest.java` 添加 Watch 初始化/重置，替换 `subStrat.exchTS = 2000L` 为 `Watch.getInstance().updateTime(2000L, "test")`

## 5. PairwiseArbStrategy exchTS 替换

- [x] 5.1 替换 `PairwiseArbStrategy.mdCallBack()` 中 exchTS 赋值为 `Watch.getInstance().getCurrentTime()`
- [x] 5.2 替换 endTime 判断中 `long currentTime = exchTS` 为 `Watch.getInstance().getCurrentTime()`
- [x] 5.3 更新 `PairwiseArbStrategyTest.java` 添加 Watch 初始化/重置，在 mdCallBack 调用前注入 Watch 时间

## 6. 其他测试文件更新

- [x] 6.1 更新 `ExtraStrategyTest.java` 添加 Watch 初始化/重置
