# Phase 2: 应用框架层 — C++ → Java 迁移

## 为什么

Phase 1 已完成 SHM 底层通信（SysVShm、MWMRQueue、Types、Constants、ClientStore、Connector）。
Phase 2 需要构建应用框架层，包括：
- 行情数据模型（Instrument 20 档订单簿）
- 订单状态管理（OrderStats、OrderMap、PriceMap）
- 策略参数集（ThresholdSet）
- 配置框架（ConfigParams、SimConfig）
- 回调分发中枢（CommonClient —— 按 symbolID 分发 MD，按 orderID 分发 ORS）

这些是 ExecutionStrategy 和 PairwiseArbStrategy 运行的前置依赖。

## 变更内容

将以下 C++ 组件迁移为 Java 类：

1. **Instrument** — 20 档订单簿（bidPx/askPx/bidQty/askQty[20]）、行情价格计算、合约属性
2. **OrderStats / OrderMap / PriceMap** — 订单生命周期状态追踪
3. **ThresholdSet** — 策略阈值参数集（~120 个参数，含默认值）
4. **SimConfig / ConfigParams** — 每策略配置容器与全局配置单例
5. **CommonClient** — MD/ORS 回调分发：按 `m_symbolID` 路由行情，按 `OrderID` 路由回报

## 涉及的能力（Capabilities）

- java-instrument: Instrument 行情数据模型
- java-order-tracking: 订单状态追踪（OrderStats、OrderMap、PriceMap）
- java-config-framework: 配置框架（ThresholdSet、SimConfig、ConfigParams）
- java-common-client: CommonClient 回调分发中枢

## 影响范围

- 新增 Java 文件在 `tbsrc-java/src/main/java/com/quantlink/trader/` 下
- 依赖 Phase 1 的 Types、Constants、Connector
- 为 Phase 3 的 ExecutionStrategy/PairwiseArbStrategy 提供基础
