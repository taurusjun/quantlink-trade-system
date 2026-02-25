# Spec: Java Instrument 行情数据模型

## 概述
迁移 C++ `Instrument` 类（`tbsrc/common/include/Instrument.h`）为 Java 类，保留 20 档订单簿和核心价格计算。

## 需求

### 数据结构
- 20 档 bid/ask 价格和数量数组：`bidPx[20]`, `askPx[20]`, `bidQty[20]`, `askQty[20]`
- 合约属性：`symbol`, `origBaseName`, `exchange`, `tickSize`, `lotSize`, `priceFactor`, `contractFactor`, `priceMultiplier`, `symbolID`
- 交易数据：`lastTradePx`, `lastTradeQty`, `totalTradedQty`, `totalTradedValue`
- 时间戳：`lastLocalTime`, `lastExchTime`
- 标志位：`sendInLots`, `perContract`, `active`

### 行情更新
- `fillOrderBook(MemorySegment mdUpdate)` — 从 MarketUpdateNew MemorySegment 填充 20 档订单簿
- 使用 Phase 1 的 `Types.MDD_BID_PRICE_VH` / `Types.MDD_ASK_PRICE_VH` 等 VarHandle 读取

### 价格计算
- `getMidPrice()` — `(bidPx[0] + askPx[0]) / 2.0`
- `getMswPrice()` — 量加权：`(askQty[0]*bidPx[0] + askPx[0]*bidQty[0]) / (askQty[0]+bidQty[0])`
- `getLtpPrice()` — 约束在 bid-ask 范围内的 LTP

### C++ 对照
- 迁移自: `tbsrc/common/include/Instrument.h`
- 保留中国期货相关字段，省略 CME/ICE/KRX 等交易所特定逻辑
