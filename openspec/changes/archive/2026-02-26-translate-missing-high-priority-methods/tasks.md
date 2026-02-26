## Tasks

### HIGH 优先级 — ExecutionStrategy.java 支撑字段与辅助方法

- [x] 添加 `bidPxStrat[20]` / `askPxStrat[20]` 到 Instrument.java
- [x] 添加 `bUseInvisibleBook` / `bUseStratBook` / `bCrossBook` / `bCrossBook2` / `bCrossBookEnd` 到 ConfigParams.java
- [x] 添加 `lastStsTS` 字段到 ExecutionStrategy.java
- [x] 翻译 `CalculatePNL(double buyprice, double sellprice)` — ExecutionStrategy.cpp:1215-1223

### HIGH 优先级 — ExecutionStrategy.java 核心方法

- [x] 翻译 `GetBidPrice(double&, OrderHitType&, int32_t&)` → `getBidPrice(double[], HitType[], int[])` — ExecutionStrategy.cpp:1225-1309
- [x] 翻译 `GetAskPrice(double&, OrderHitType&, int32_t&)` → `getAskPrice(double[], HitType[], int[])` — ExecutionStrategy.cpp:1357-1440
- [x] 翻译 `SendBidOrder(RequestType, int32_t, double, OrderHitType, uint32_t, double)` → `sendBidOrder(...)` — ExecutionStrategy.cpp:1311-1355
- [x] 翻译 `SendAskOrder(RequestType, int32_t, double, OrderHitType, uint32_t, double)` → `sendAskOrder(...)` — ExecutionStrategy.cpp:1442-1485
- [x] 翻译 `SetTargetValue(double&, double&, double*, double*)` → `setTargetValue(...)` — ExecutionStrategy.cpp:422-482
- [x] 翻译 `SetQuantAhead(MarketUpdateNew*)` → `setQuantAhead(MemorySegment)` — ExecutionStrategy.cpp:691-757
- [x] 翻译 `HandleTimeLimitSquareoff()` → `handleTimeLimitSquareoff()` — ExecutionStrategy.cpp:2442-2506

### HIGH 优先级 — PairwiseArbStrategy.java 核心方法

- [x] 翻译 `GetBidPrice_second(double&, OrderHitType&, int32_t&)` → `getBidPriceSecond(...)` — PairwiseArbStrategy.cpp:842-861
- [x] 翻译 `GetAskPrice_second(double&, OrderHitType&, int32_t&)` → `getAskPriceSecond(...)` — PairwiseArbStrategy.cpp:863-883
- [x] 翻译 `SendTCacheLeg1Pos()` → `sendTCacheLeg1Pos()` — PairwiseArbStrategy.cpp:885-900

### MEDIUM 优先级 — 支撑字段

- [x] 添加 `bidQtyStrat[20]` / `askQtyStrat[20]` / `bidOrderCountStrat[20]` / `askOrderCountStrat[20]` / `validBids` / `validAsks` 到 Instrument.java

### MEDIUM 优先级 — 监控与告警方法

- [x] 翻译 `SendMonitorStratPos(...)` → `sendMonitorStratPos(...)` — ExecutionStrategy.cpp:133-161
- [x] 翻译 `SendMonitorStratDetail(...)` → `sendMonitorStratDetail(...)` — ExecutionStrategy.cpp:163-195
- [x] 翻译 `SendMonitorStratPNL(...)` → `sendMonitorStratPNL(...)` — ExecutionStrategy.cpp:196-219
- [x] 翻译 `SendMonitorStratStatus(...)` → `sendMonitorStratStatus(...)` — ExecutionStrategy.cpp:221-243
- [x] 翻译 `SendMonitorStratCancelSts(...)` → `sendMonitorStratCancelSts(...)` — ExecutionStrategy.cpp:244-264
- [x] 翻译 `SendAlert(...)` → `sendAlert(...)` — ExecutionStrategy.cpp:1156-1173

### MEDIUM 优先级 — 核心方法

- [x] 翻译 `ProcessSelfTrade(ResponseMsg*)` → `processSelfTrade(double)` — ExecutionStrategy.cpp:835-949
- [x] 翻译 `fillFixedFields(Instrument*)` → `fillFixedFields(Instrument)` — ExecutionStrategy.cpp:1487-1520

### MEDIUM 优先级 — ExtraStrategy.java

- [x] 翻译 `InitMonitorStratDatas()` → `initMonitorStratDatas()` — ExtraStrategy.cpp:12-17

### 验证

- [x] HIGH: 编译通过 + 184 测试通过
- [x] MEDIUM: 编译通过 + 184 测试通过
- [x] 部署测试通过（start/activate/deactivate 正常）
