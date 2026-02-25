# 实施任务

## 方法命名修复

- [x] 1. Connector.java: `start()` → `startAsync()`, `pollMD()` → `handleLiveMdUpdates()`, `pollORS()` → `handleOrderResponse()`, `nextOrderID()` → `getUniqueOrderNumber()`
- [x] 2. Instrument.java: `getMidPrice()` → `calculateMIDPrice()`, `getMswPrice()` → `calculateMSWPrice()`, `getLtpPrice()` → `calculateLTPPrice()`, `getMswMidPrice()` → `calculateMSWMIDPrice()`
- [x] 3. ControlConfig.java: `parse()` → `loadControlFile()`
- [x] 4. CfgConfig.java: `parse()` → `loadCfg()`
- [x] 5. ModelConfig.java: `parse()` → `loadModelFile()`
- [x] 6. PairwiseArbStrategy.java: 删除 `loadDailyInit()` 方法，逻辑合并到构造函数（与 C++ PairwiseArbStrategy.cpp:18-62 一致）

## symbolID 路由修复

- [x] 7. ConfigParams.java: `simConfigMap` 改为 `Map<String, List<SimConfig>>`; `SimConfig.instruMap` 改为 `Map<String, Instrument>`
- [x] 8. CommonClient.java: `sendINDUpdate()` 使用 `Instrument.readSymbol()` 按字符串路由
- [x] 9. TraderMain.java: `init()` 中注册 simConfigMap/instruMap 使用 symbol 字符串 key，删除 hashCode symbolID 逻辑

## 调用方更新

- [x] 10. TraderMain.java: 更新所有方法调用名（`start()` → `startAsync()`, `ControlConfig.parse()` → `loadControlFile()` 等），删除 `loadDailyInit` 调用
- [x] 11. 更新所有测试文件中的方法调用名
- [x] 12. 编译验证 + 运行测试 — 168 tests, 0 failures, BUILD SUCCESS
