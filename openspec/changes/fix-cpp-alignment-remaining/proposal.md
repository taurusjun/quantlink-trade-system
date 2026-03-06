# Proposal: 修复剩余 C++ 对齐问题

## 问题

Java 翻译层仍有 8 处与 C++ 原代码的对齐缺失，按优先级分为 HIGH (3) 和 MEDIUM (5)。

## HIGH — 逻辑缺失

1. **SetCheckCancelQuantity()** — C++ 构造函数中调用，初始化撤单数量追踪变量（cancelQtyBid/cancelQtyAsk 等），Java 完全缺失
2. **FillMsg()** — ExtraStrategy 用于多合约下单的 RequestMsg 构建方法，后续迁移 ExtraStrategy 完整逻辑需要
3. **SendInfraReqUpdate()** — C++ 有请求队列监控回调（发单后调用），Java 未实现

## MEDIUM — 语义差异

4. **夜盘 currDate** — C++ 夜盘用 `m_dateConfig.m_currDate`（下一交易日），Java 用 `LocalDate.now()`，跨日时日期不同
5. **Tick 对象未迁移** — C++ 有独立 Tick 类维护 tickLevel，Java 用 updateLevel 替代
6. **全局 DateConfig vs per-SimConfig** — C++ 有独立全局 `m_dateConfig`，Java 用每个 SimConfig 的 dateConfig
7. **Connector OrderID 溢出** — C++ 溢出时申请新 clientId，Java 未处理
8. **lastTradePx 更新条件** — C++ 仅在 TRADE 类型更新，Java 每次行情都更新

## 范围

- 修改文件：ExecutionStrategy.java, CommonClient.java, Connector.java, Instrument.java, SimConfig.java
- 新增文件：无（所有逻辑补入现有文件）
- 不涉及 LOW 优先级问题（期权/CommonBook/SelfBook/SMARTMD 等中国期货不使用的功能）
