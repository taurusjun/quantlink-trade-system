## 1. Constants 补齐

- [x] 1.1 Constants.java 添加 DEFAULT_NOT_POSSIBLE_CLIENTID = 99999999 常量（D11）
- [x] 1.2 Constants.java 添加 getExchangeIdFromName(String) 静态方法，从 Connector 迁移过来（D7）

## 2. Connector.java — 补齐 C++ 缺失方法

- [x] 2.1 添加 handleORSRequests(MemorySegment request) 空实现 + 注释说明 REQUEST_CALLBACK 模式（D1）
- [x] 2.2 添加 pushRequest(int exchType, List<MemorySegment> requests) 批量版方法（D2）
- [x] 2.3 添加 setInstrumentCache() 空实现 + 注释（D10）
- [x] 2.4 添加 ifRunningForRussianExchange() 空实现 + 注释 + [C++差异] 标注参数差异（D10）
- [x] 2.5 添加 handleLiveMdUpdates() 空实现（0 参数，对齐 C++ header）+ 注释（D10）
- [x] 2.6 添加 startSync() 空实现 + [C++差异] 标注省略 SimEngineType（验证发现的遗漏）
- [x] 2.7 添加 blockSignals() 空实现 + [C++差异-语言适配] 标注（验证发现的遗漏）
- [x] 2.8 添加 handleSignals() 空实现 + [C++差异-语言适配] 标注（验证发现的遗漏）
- [x] 2.9 添加 getSymbolList(String, String, Set<String>) 空实现 + [C++差异] 标注（验证发现的遗漏）

## 3. Connector.java — 移除 Java 发明代码

- [x] 3.1 移除冗余字段 allClientIds（D4）
- [x] 3.2 移除 destroy() 方法，保留 close() 标注 [C++差异-Java 资源管理]（D3）
- [x] 3.3 移除 addClientId(int) 独立方法（D6）
- [x] 3.4 移除 addInterestedSymbol() 和 addInterestedSymbolForOrs() 方法（D5）
- [x] 3.5 将 getExchangeIdFromName() 移除，调用方改为 Constants.getExchangeIdFromName()（D7）

## 4. Connector.java — 签名/访问级别修复

- [x] 4.1 pushRequest 访问级别从 private 改为 public（D8）
- [x] 4.2 startAsync() 标注 [C++差异-仅LIVE模式，省略 SimEngineType 参数]（D9）

## 5. 调用方适配

- [x] 5.1 TraderMain.java — 无需修改（未调用被移除的方法）
- [x] 5.2 ConnectorTestHelper.java — 无需修改（destroy() 是 MWMRQueue/ClientStore 的方法）
- [x] 5.3 ConnectorTest.java — 无需修改

## 6. 验证

- [x] 6.1 编译通过: ./scripts/build_deploy_java.sh --mode live
- [x] 6.2 Sim 测试通过: 启动 gateway sim + strategy，确认日志正常
- [x] 6.3 原则 9 重新比对：确认所有原始问题已修复 + 发现新遗漏已补齐
