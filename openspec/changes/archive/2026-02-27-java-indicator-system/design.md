## Context

C++ 使用 `Indicator` 类层次 + `CalculatePNL::CalculateTargetPNL()` 计算每笔行情更新后的公允价。非 arb 策略（`useArbStrategy!=1`）的核心链路：行情 → CommonClient.Update() 更新指标 → IndicatorCallBack → CalculateTargetPNL 计算 targetPrice → SetTargetValue → SendOrder。Java 当前只有 arb 路径（跳过 CalculateTargetPNL），需补齐框架。

## Goals / Non-Goals

**Goals:**
- 迁移 Indicator 基类 + Dependant 指标
- 迁移 CalculatePNL（CalculateTargetPNL 方法），支持 MKTW_PX / MKTW_PX2 价格模式
- 在 CommonClient 中实现 `update()` 方法（遍历指标列表调用 QuoteUpdate/TickUpdate）
- 在 TraderMain.indCallback 中实现非 arb 路径
- 在 SimConfig 中添加 indicatorList / calculatePNL 字段
- 在 Instrument 中添加 indList / SubscribeTBPriceType / GetTBPriceType / strat book 方法

**Non-Goals:**
- 不迁移 170+ 个具体指标子类（按需后续迁移）
- 不迁移 VOL/RATIO 价格模式（期权相关）
- 不迁移 OptionManager/DeltaStrategy 相关逻辑
- 不迁移 ProcessSelfTrade 指标路径（Phase 3 范围）
- 不迁移模型文件解析器 LoadModelFile（当前已有模型文件解析）
- 不迁移 Regress/PrintIndicatorList/LeadLag 统计路径

## Decisions

1. **Indicator 基类**: Java 抽象类，保持 C++ 全部公共字段（value, last_value, diff_value, isDep, isValid, level, instrument 等）

2. **IndElem 数据结构**: Java 类对应 C++ struct，包含 baseName, type, indName, coefficient, index, argList, indicator 引用

3. **CalculateTargetPNL**: 仅实现 MKTW_PX 和 MKTW_PX2 两种价格模式（中国期货使用的模式）。VOL/RATIO 路径保留结构但不执行（加注释标注）

4. **CommonClient.update()**: 对齐 C++ `CommonClient::Update(iter, tick)` — 遍历合约的 indList，按 tickType 调用 QuoteUpdate 或 TickUpdate。Java 中 Tick 对象用 Instrument 当前状态传递（bidPx/askPx 已在 FillOrderBook 中更新）

5. **indCallback 双路径**: TraderMain 中的 `setINDCallback` 使用 `Consumer<List<IndElem>>` 替代 `Runnable`，根据 `useArbStrat` 分流到 arb（直接 SetTargetValue）或非 arb（CalculateTargetPNL → SetTargetValue）路径

6. **Instrument.indList**: 每个 Instrument 持有自己的指标列表引用（`List<IndElem>`），与 C++ `InstruElem.m_indList` 对齐

## Risks / Trade-offs

- [指标子类缺失] → Dependant 是唯一必须的指标。非 arb 策略的模型文件如果引用其他指标，需要后续逐个迁移对应子类
- [Tick 对象缺失] → C++ 有完整的 Tick 类（tickType, tickLevel 等），Java 用 Instrument 状态 + updateLevel 替代，Dependant 只需 bidPx/askPx 有效即可
