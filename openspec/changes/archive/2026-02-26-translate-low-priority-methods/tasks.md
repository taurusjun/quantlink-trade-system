## Tasks

### LOW 优先级 — ExecutionStrategy.java 支撑字段

- [x] 添加 `bidMapCache` / `askMapCache` (Self-book Cache Map) 到 ExecutionStrategy.java
- [x] 添加统计字段 `instruAvgTradeQty` / `volume_ewa` / `SET_HIGH` / `prev_tradeQty` / `statTrTimeQ` / `statTradeQtyQ` 到 ExecutionStrategy.java

### LOW 优先级 — ExecutionStrategy.java 方法

- [x] 翻译 `GetOptionType(char)` → `getOptionType(char)` — ExecutionStrategy.cpp:484-498
- [x] 翻译 `GetInstrumentStats()` → `getInstrumentStats()` — ExecutionStrategy.cpp:398-420
- [x] 翻译 `AddtoCache(OrderMapIter&, double&)` → `addToCache(OrderStats, double)` — ExecutionStrategy.cpp:821-834
- [x] 翻译 `DumpOurBook()` → `dumpOurBook()` — ExecutionStrategy.cpp:1605-1624
- [x] 翻译 `DumpIndicators()` → `dumpIndicators()` — ExecutionStrategy.cpp:1626-1634
- [x] 翻译 `DumpMktBook()` → `dumpMktBook()` — ExecutionStrategy.cpp:1636-1641
- [x] 翻译 `DumpStratBook()` → `dumpStratBook()` — ExecutionStrategy.cpp:1643-1648

### LOW 优先级 — ExtraStrategy.java 方法

- [x] 翻译 `AddtoCache(OrderMapIter&, double&)` → `addToCache(OrderStats, double)` — ExtraStrategy.cpp:19-31

### 验证

- [x] 编译通过 + 184 测试通过
