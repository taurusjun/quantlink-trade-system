## Context

C++ 交易系统使用 `Watch` 全局单例（`tbsrc/common/Watch.hpp`）作为行情驱动时钟。所有策略和指标通过 `Watch::GetUniqueInstance()->GetCurrentTime()` 获取统一的单调递增时间戳。Java 迁移中此机制缺失，各策略直接从行情 MemorySegment 读取 `exchTS` 字段，导致：
1. 无单调递增保护
2. 多处重复的时间读取代码
3. 与 C++ 架构不一致，增加对照维护成本

## Goals / Non-Goals

**Goals:**
- 1:1 迁移 C++ Watch 类到 Java，保持完整的 API 对应
- 在 CommonClient 中统一更新 Watch 时钟（与 C++ CommonClient.cpp:412-415 对齐）
- 替换所有策略中直接读取 exchTS 的代码
- TimeListener 接口保留（为未来指标系统预留）

**Non-Goals:**
- 不迁移 C++ SimConfig/ModeType 体系（已有 Java 等价实现）
- 不实现多线程安全的 Watch（单线程轮询架构，与 C++ 一致）
- 不迁移 Indicator 系统（TimeListener 暂无订阅者）

## Decisions

1. **单例模式**: 使用静态字段 + synchronized createInstance()，与 C++ `unique_instance_` 一致。添加 `resetInstance()` 方法用于测试隔离（C++ 无此需求因测试不重用进程）。

2. **时间更新位置**: 在 `CommonClient.sendINDUpdate()` 中更新，精确对齐 C++ 位置。使用 `ConfigParams.useExchTS` 选择 timestamp vs exchTS*10^6 路径。

3. **exchTS 字段保留**: ExecutionStrategy.exchTS 字段保留并从 Watch 同步赋值，避免破坏可能的遗留引用。

4. **getNanoSecsFromEpoch**: 使用 `Calendar(UTC)` 替代 C++ `mktime()-timezone`，语义等价。

## Risks / Trade-offs

- [Watch.getInstance() 为 null] → 在 CommonClient 中加 null 检查守卫；测试中 @BeforeEach 必须调用 createInstance(0)
- [exchTS 字段冗余] → 保留字段并同步赋值，后续可标记 @Deprecated 逐步清理
