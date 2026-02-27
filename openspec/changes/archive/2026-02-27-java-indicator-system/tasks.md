## 1. Indicator 基类 + Dependant 指标

- [x] 1.1 新建 `Indicator.java`（`tbsrc-java/.../core/Indicator.java`），迁移自 `tbsrc/Indicators/include/Indicator.h`，包含抽象方法 quoteUpdate/tickUpdate/orderBookStratUpdate/reset 和字段 value/last_value/diff_value/isDep/isValid/level/instrument/index
- [x] 1.2 新建 `Dependant.java`（`tbsrc-java/.../indicator/Dependant.java`），迁移自 `tbsrc/Indicators/Dependant.cpp`，支持 MID_PX/MKTW_PX/WGT_PX/LTP_PX 等价格类型
- [x] 1.3 新建 `IndElem.java`（`tbsrc-java/.../core/IndElem.java`），迁移自 `TradeBotUtils.h:IndElem`，包含 baseName/type/indName/coefficient/index/argList/indicator

## 2. Instrument 扩展

- [x] 2.1 在 `Instrument.java` 中添加 `subscribeTBPriceType()`、`getTBPriceType()`、`getTBStratPriceType()` 方法，迁移自 `Instrument.h:169-232`
- [x] 2.2 在 `Instrument.java` 中添加 strat book 字段（`bidPxStrat[]`, `askPxStrat[]`, `bidQtyStrat[]`, `askQtyStrat[]`）和 `calculateStratPrices()` 方法
- [x] 2.3 在 `Instrument.java` 中添加 `indList` 字段（`List<IndElem>`），每个合约持有自己的指标列表引用

## 3. CalculateTargetPNL

- [x] 3.1 新建 `CalculateTargetPNL.java`（`tbsrc-java/.../core/CalculateTargetPNL.java`），迁移自 `TradeBotUtils.cpp:3749-4025`，实现 MKTW_PX 和 MKTW_PX2 价格模式，VOL/RATIO 保留结构注释
- [x] 3.2 在 `SimConfig.java` 中添加 `indicatorList`（`List<IndElem>`）和 `calculatePNL`（`CalculateTargetPNL`）字段，以及 `CONST` 阈值

## 4. CommonClient Indicator 更新

- [x] 4.1 在 `CommonClient.java` 中添加 `update(Instrument, int updateLevel)` 方法，迁移自 `CommonClient::Update(iter, tick)` — 遍历合约 indList 调用 quoteUpdate/tickUpdate
- [x] 4.2 在 `CommonClient.sendINDUpdate()` 的 significantUpdate 路径中调用 `update()`

## 5. TraderMain indCallback 非 arb 路径

- [x] 5.1 修改 `CommonClient.setINDCallback` 参数类型为 `Consumer<SimConfig>`，传入 simConfig 上下文
- [x] 5.2 修改 `TraderMain.java` 的 indCallback：非 arb 路径调用 `calculatePNL.calculateTargetPNL()` → `setTargetValue()`；arb 路径保持不变

## 6. 测试

- [x] 6.1 新建 `IndicatorTest.java` — 测试 Indicator.calculate()、Dependant 各价格类型、isValid 边界
- [x] 6.2 新建 `CalculateTargetPNLTest.java` — 测试 MKTW_PX2/MKTW_PX 模式、PNL 计算、CHECK_PNL 逻辑、指标无效返回 false
- [x] 6.3 更新现有测试确保 215+ 测试全部通过（249 测试通过）
