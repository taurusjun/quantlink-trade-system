# Phase 3: 策略层 — ExecutionStrategy + ExtraStrategy + PairwiseArbStrategy

## 为什么
Phase 1 (SHM/Connector) 和 Phase 2 (应用框架) 已完成。现在需要迁移核心策略逻辑，这是交易系统的业务核心。

## 变更内容
将 C++ 策略层三个类迁移到 Java：
1. **ExecutionStrategy** — 执行策略基类（~300行头文件，~2500行实现）
2. **ExtraStrategy** — 多合约策略变体（Instrument* 参数化订单方法）
3. **PairwiseArbStrategy** — 双腿套利策略（被动挂单 + 对冲）

## 能力
- **java-execution-strategy**: ExecutionStrategy 基类（位置管理、PNL计算、阈值设置、订单管理、ORS回调、风控检查）
- **java-extra-strategy**: ExtraStrategy（Instrument 参数化的订单方法）
- **java-pairwise-arb-strategy**: PairwiseArbStrategy（双腿套利、价差跟踪、被动挂单、对冲逻辑、daily_init 加载）

## 影响
- 依赖 Phase 1: Types, Constants, Connector
- 依赖 Phase 2: Instrument, OrderStats, ThresholdSet, SimConfig, ConfigParams, CommonClient
- 完成后系统具备完整策略执行能力
