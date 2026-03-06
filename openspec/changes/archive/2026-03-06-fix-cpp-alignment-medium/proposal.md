## Why

Java 迁移审计发现 5 个 MEDIUM/LOW 级别问题，其中 baseNameToSymbol 年份 rollover 问题升级为 HIGH（年底交易会生成错误合约名）。其余为字段缺失或死代码补齐。

## What Changes

- **baseNameToSymbol 年份 rollover（HIGH）**: 添加 C++ FillChinaFields2() 的跨年推断逻辑 — 当合约月份 < 当前月份时，自动使用下一年
- **fillOrderBook 缺失字段（MEDIUM）**: 补齐 `bidOrderCount`/`askOrderCount`/`validBids`/`validAsks`/`lastTradeQty`/`updateIndicators` 字段填充
- **SendNewOrder 缺失字段（MEDIUM）**: 补齐 `Token`/`Product`/`AccountID`/`Duration`(FAK/DAY)/`QuantityFilled`/`DisclosedQnty`/`ExpiryDate`/`StrikePrice`/`OptionType`
- **m_sendMail（LOW）**: 添加 sendMail 字段（C++ 死代码，为完整性补齐）
- **OrderID overflow（LOW）**: 添加注释说明 uint32_t vs int 差异，实际不影响

## Capabilities

### New Capabilities

（无新增能力）

### Modified Capabilities

- `strategy`: baseNameToSymbol 年份推断、fillOrderBook 字段补齐、SendNewOrder 字段补齐

## Impact

- **ConfigParser.java**: baseNameToSymbol() 添加年份 rollover 逻辑
- **Instrument.java**: fillOrderBook() 补齐 6+ 字段
- **CommonClient.java**: sendNewOrder() 补齐 10+ 字段
- **ExecutionStrategy.java**: 添加 sendMail 字段
