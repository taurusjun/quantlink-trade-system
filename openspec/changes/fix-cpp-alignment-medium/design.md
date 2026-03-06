## Context

继 fix-cpp-alignment-critical 修复后，继续处理 Java 迁移审计中剩余的 MEDIUM/LOW 级别问题。代码均位于 `tbsrc-java/`，对标 C++ 原代码 `tbsrc/`。

## Goals / Non-Goals

**Goals:**
- baseNameToSymbol 对齐 C++ FillChinaFields2() 的跨年推断
- fillOrderBook 对齐 C++ Instrument::FillOrderBook() + CopyOrderBook() 的完整字段
- sendNewOrder 对齐 C++ CommonClient::SendNewOrder() 的完整字段
- 补齐 m_sendMail 字段（完整性）

**Non-Goals:**
- SmartBook/Combined instrument 逻辑（中国期货不使用，已有用户确认）
- sweepordMap（sweep 策略独有）
- AddtoCache/self-book（中国期货不使用）

## Decisions

### 1. baseNameToSymbol: 内联推断 vs 外部传参

**选择**: 在 baseNameToSymbol 内部根据当前月份推断年份

**理由**: C++ FillChinaFields2() 在 Instrument 内部推断，不依赖外部传入年份。Java 当前依赖 TraderMain 传入 yearPrefix，但没有 rollover 逻辑。修改 baseNameToSymbol 使其在 month < currentMonth 时自动 +1 年，与 C++ 一致。

### 2. fillOrderBook: 从 SHM 读取 vs 本地计算

**选择**: 从 MarketUpdateNew SHM 结构读取缺失字段

**理由**: C++ CopyOrderBook 从 update 结构直接 memcpy bidOrderCount/askOrderCount/validBids/validAsks。Java 需要从 SHM MemorySegment 对应偏移读取。lastTradeQty 同理。

### 3. sendNewOrder: 哪些字段对 counter_bridge 有效

**选择**: 补齐所有 C++ 字段，即使 counter_bridge 可能不全使用

**理由**: 遵循"完整迁移"原则。counter_bridge 解析 RequestMsg 的字段可能随版本变化，保持完整性避免未来兼容问题。Duration (FAK/DAY) 字段对 CTP 撤单行为有直接影响。

## Risks / Trade-offs

- **[Risk] SHM 偏移计算错误** → fillOrderBook 读取新字段时 offset 必须与 C++ MarketUpdateNew 结构完全匹配。Mitigation: 对照 hftbase/CommonUtils/include/marketupdateNew.h 验证
- **[Risk] 年份推断在跨年夜盘时刻** → 12月31日夜盘23:59 vs 1月1日00:01 时刻 currentMonth 不同。Mitigation: 与 C++ 行为一致，使用 currentDate 推断
