## ADDED Requirements

### Requirement: Java CommonClient SHALL set Exchange_Type in RequestMsg for all order requests

Java `CommonClient.sendNewOrder()`、`sendModifyOrder()`、`sendCancelOrder()` 在填充 RequestMsg 时 MUST 设置 `Exchange_Type` 字段（offset 164），与 C++ `FillReqInfo()` 中 `m_reqMsg.Exchange_Type = m_exchangeType` 对齐。

#### Scenario: SHFE 合约发送 BUY 订单时 Exchange_Type 正确填充
- **WHEN** Java 策略通过 `sendNewOrder()` 发送 ag2606 BUY 订单
- **THEN** RequestMsg 的 Exchange_Type 字段 SHALL 为 57（CHINA_SHFE），counter_bridge 的 `SetCombOffsetFlag()` 中 `isSHFE` 为 true

#### Scenario: SHFE 合约发送 SELL 订单时 Exchange_Type 正确填充
- **WHEN** Java 策略通过 `sendNewOrder()` 发送 ag2606 SELL 订单
- **THEN** RequestMsg 的 Exchange_Type 字段 SHALL 为 57（CHINA_SHFE）

#### Scenario: sendModifyOrder 和 sendCancelOrder 同样设置 Exchange_Type
- **WHEN** Java 策略调用 `sendModifyOrder()` 或 `sendCancelOrder()`
- **THEN** RequestMsg 的 Exchange_Type 字段 SHALL 被正确设置

### Requirement: CfgConfig SHALL provide exchange string to byte mapping

`CfgConfig` MUST 提供静态方法将 exchanges 字符串（如 "CHINA_SHFE"）映射为对应的字节常量（如 57），与 C++ `CommonClient.cpp:850-901` 的映射逻辑对齐。

#### Scenario: CHINA_SHFE 映射
- **WHEN** CfgConfig.exchanges 为 "CHINA_SHFE"
- **THEN** `parseExchangeType("CHINA_SHFE")` SHALL 返回 57

#### Scenario: 未知交易所名称
- **WHEN** CfgConfig.exchanges 为未识别字符串
- **THEN** `parseExchangeType()` SHALL 返回 0 并输出 warning 日志

### Requirement: CommonClient SHALL hold exchangeType field initialized at startup

CommonClient MUST 持有 `exchangeType` 字段，在 `TraderMain.init()` 中从 CfgConfig 解析并通过 setter 注入，与 C++ `CommonClient.h:122` 的 `char m_exchangeType` 对齐。

#### Scenario: TraderMain 初始化时设置 exchangeType
- **WHEN** TraderMain.init() 创建 CommonClient 后
- **THEN** SHALL 调用 `client.setExchangeType()` 设置正确的交易所类型字节值
