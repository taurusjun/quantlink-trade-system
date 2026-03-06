## ADDED Requirements

### Requirement: baseNameToSymbol 年份 rollover

baseNameToSymbol() 必须实现跨年推断逻辑，对齐 C++ `Instrument::FillChinaFields2()`：当合约月份小于当前月份时，自动使用下一年的 2 位年份。

#### Scenario: 年内合约（无 rollover）
- **WHEN** 当前月份为 3 月，baseName 为 `ag_F_6_SFE`（6 月合约）
- **THEN** 使用当前年份，返回 `ag2606`

#### Scenario: 跨年合约（需 rollover）
- **WHEN** 当前月份为 11 月，baseName 为 `ag_F_3_SFE`（3 月合约）
- **THEN** 自动使用下一年，返回 `ag2703`（非 `ag2603`）

#### Scenario: 当前月份等于合约月份
- **WHEN** 当前月份为 6 月，baseName 为 `ag_F_6_SFE`
- **THEN** 使用当前年份，返回 `ag2606`

### Requirement: fillOrderBook 完整字段填充

Instrument.fillOrderBook() 必须从 SHM MarketUpdateNew 读取以下字段，对齐 C++ `Instrument::FillOrderBook()` + `CopyOrderBook()`：
- `bidOrderCount[i]` / `askOrderCount[i]`（20 档）
- `validBids` / `validAsks`（实际有效档位数）
- `lastTradeQty`（最新成交量）
- `updateIndicators = true`（标志位）

#### Scenario: 完整字段填充
- **WHEN** 收到 MarketUpdateNew 行情包
- **THEN** bidOrderCount/askOrderCount/validBids/validAsks/lastTradeQty 均从 SHM 读取填充

#### Scenario: validBids 反映实际档位
- **WHEN** 行情只有 5 档有效买盘
- **THEN** validBids 为 5（非默认 20）

### Requirement: sendNewOrder 完整字段填充

CommonClient.sendNewOrder() 必须设置以下 C++ `CommonClient::SendNewOrder()` 中的字段：
- `Token`（合约 token）
- `QuantityFilled = 0`
- `DisclosedQnty = Quantity`
- `Product`（策略产品名）
- `AccountID`（策略账户）
- `Contract_Description.InstrumentName`
- `Contract_Description.OptionType`（CE/PE/XX）
- `Contract_Description.CALevel = 0`
- `Contract_Description.ExpiryDate`
- `Contract_Description.StrikePrice`
- `Duration`（CROSS 时为 FAK，其他为 DAY）

#### Scenario: CROSS 订单使用 FAK
- **WHEN** 发送 CROSS 类型订单
- **THEN** Duration 字段设为 FAK（Fill-and-Kill）

#### Scenario: 普通订单使用 DAY
- **WHEN** 发送普通限价订单
- **THEN** Duration 字段设为 DAY

### Requirement: m_sendMail 字段补齐

ExecutionStrategy 必须包含 `sendMail` boolean 字段，默认 false，对齐 C++ `ExecutionStrategy::m_sendMail`。

#### Scenario: 默认值
- **WHEN** 策略初始化
- **THEN** sendMail 为 false
