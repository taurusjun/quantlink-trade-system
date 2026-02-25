# 设计: 方法命名修复 + symbolID 路由

## 方法命名修复

纯重命名操作。Java 使用 camelCase 转换 C++ PascalCase/snake_case:
- `StartAsync` → `startAsync`
- `HandleLiveMdUpdates` → `handleLiveMdUpdates`
- `GetUniqueOrderNumber` → `getUniqueOrderNumber`
- `calculate_MIDPrice` → `calculateMIDPrice`
- `LoadControlFile` → `loadControlFile`

`loadDailyInit()` 方法删除，逻辑移入 PairwiseArbStrategy 构造函数（与 C++ PairwiseArbStrategy.cpp:18-62 一致）。

## symbolID → symbol 字符串路由

### 问题分析

C++ md_shm_feeder: `memset(&md, 0, sizeof(md))` 后不设置 `m_symbolID`，值始终为 0。
Go Client: `extractSymbol(&md.Header)` 按字符串路由。
Java CommonClient: 使用 `Instrument.readSymbolID()` → `configParams.simConfigMap.get(symbolID)` 失败。

### 方案

1. `ConfigParams.simConfigMap` 类型改为 `Map<String, List<SimConfig>>`（key 从 int symbolID 改为 String symbol）
2. `SimConfig.instruMap` 类型改为 `Map<String, Instrument>`（同理）
3. `CommonClient.sendINDUpdate()` 用 `Instrument.readSymbol()` 取 symbol 字符串查找
4. `TraderMain.init()` 注册时使用 symbol 字符串作为 key
5. 删除 `instru.symbolID` 赋值（`hashCode()` 逻辑移除）
