## ADDED Requirements

### Requirement: Watch 全局时钟单例
系统 SHALL 提供 `Watch` 全局单例，作为行情驱动的统一时钟源。Watch 实例通过 `Watch.createInstance(0)` 在系统启动时创建，通过 `Watch.getInstance()` 获取。

#### Scenario: 单例创建
- **WHEN** 调用 `Watch.createInstance(0)`
- **THEN** 返回 Watch 实例，后续 `Watch.getInstance()` 返回同一实例

#### Scenario: 重复创建保留首次
- **WHEN** 先调用 `Watch.createInstance(100)` 再调用 `Watch.createInstance(200)`
- **THEN** 两次返回同一实例，currentTime 为 100（首次创建的值）

### Requirement: 单调递增时间更新
Watch.updateTime() SHALL 保证时间单调递增。仅当新时间大于当前时间（或新时间为 0）时才更新。

#### Scenario: 前进时间
- **WHEN** Watch 当前时间为 100，调用 updateTime(200, "test")
- **THEN** getCurrentTime() 返回 200

#### Scenario: 回退时间被忽略
- **WHEN** Watch 当前时间为 200，调用 updateTime(100, "test")
- **THEN** getCurrentTime() 仍为 200

#### Scenario: 零值重置
- **WHEN** 调用 updateTime(0, "reset")
- **THEN** getCurrentTime() 返回 0（允许重置）

### Requirement: TimeListener 每秒触发
Watch SHALL 支持 TimeListener 订阅，当时间推进超过 1 秒（10^9 纳秒）时触发所有已注册的 listener。

#### Scenario: 1 秒后触发
- **WHEN** Watch 从 0 更新到 1.1 秒（1,100,000,000 ns）
- **THEN** 所有已注册的 TimeListener.onTimeUpdate() 被调用一次

#### Scenario: 不足 1 秒不触发
- **WHEN** Watch 从 0 更新到 0.5 秒
- **THEN** TimeListener 不被调用

### Requirement: CommonClient 统一更新 Watch
CommonClient.sendINDUpdate() SHALL 在行情分发前调用 Watch.updateTime()，使用 ConfigParams.useExchTS 选择时间源。

#### Scenario: useExchTS=false
- **WHEN** useExchTS=false，收到行情 timestamp=1000
- **THEN** Watch.updateTime(1000, symbol) 被调用

#### Scenario: useExchTS=true
- **WHEN** useExchTS=true，收到行情 exchTS=2000
- **THEN** Watch.updateTime(2000 * 1_000_000, symbol) 被调用

### Requirement: 策略通过 Watch 获取时间
ExecutionStrategy 和 PairwiseArbStrategy 中所有时间判断 SHALL 通过 `Watch.getInstance().getCurrentTime()` 获取，不再直接读取行情中的 exchTS 字段。

#### Scenario: checkSquareoff endTime 判断
- **WHEN** Watch.getCurrentTime() >= endTimeEpoch
- **THEN** 触发 END TIME 平仓逻辑

#### Scenario: PairwiseArbStrategy endTime 判断
- **WHEN** Watch.getCurrentTime() >= endTimeEpoch 且 active=true
- **THEN** 触发平仓逻辑

### Requirement: getNanoSecsFromEpoch 时间转换
Watch SHALL 提供静态方法 getNanoSecsFromEpoch(date, time)，将 yyyymmdd + hhmm 转换为纳秒 epoch。当 ConfigParams.useExchTS=true 时减去 315,532,800,000,000,000 偏移量。

#### Scenario: 基本转换
- **WHEN** 调用 getNanoSecsFromEpoch(20260227, 0)
- **THEN** 返回 2026-02-27 00:00 UTC 的纳秒 epoch

#### Scenario: useExchTS 偏移
- **WHEN** useExchTS=true 调用 getNanoSecsFromEpoch(20260227, 0)
- **THEN** 返回值比 useExchTS=false 时小 315,532,800,000,000,000
