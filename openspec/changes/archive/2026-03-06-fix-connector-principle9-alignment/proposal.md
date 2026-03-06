## Why

原则 9 比对发现 Connector.java 与 C++ connector.h/connector.cpp 存在大量结构不一致：缺失 C++ 方法、发明 C++ 不存在的字段/方法、入参个数不匹配。这些差异违反"C++ 与 Java 结构必须严格一一对应"的核心规则，增加维护成本和潜在 bug 风险。

## What Changes

### 严重问题修复
- 补齐 `HandleORSRequests(RequestMsg*)` 回调（C++ `REQUEST_CALLBACK=true` 时使用）
- 补齐 `PushRequest(Container<RequestMsg>&)` 批量推送方法

### 入参/签名修复
- `startAsync()` 入参对齐 C++ `StartAsync(SimEngineType)` — 添加 SimEngineType 参数（Java 仅使用默认值）
- `pushRequest` 访问级别从 private 改为 public，对齐 C++ `PushRequest`

### 缺失常量/字段补齐
- 添加 `DEFAULT_NOT_POSSIBLE_CLIENTID` 常量
- 补齐 `setInstrumentCache()` 和 `ifRunningForRussianExchange()` 方法（空实现 + 注释说明）
- 补齐 `HandleLiveMdUpdates()` 方法

### Java 发明代码清理
- 移除冗余字段 `allClientIds`（核心逻辑已用 `allClientIdsByExchange`）
- 合并 `close()`/`destroy()` 为 `stop()`（对齐 C++ 仅有 `Stop()`）
- 移除 `addClientId(int)` 独立方法（C++ 无此方法）
- 移除 `addInterestedSymbol()`/`addInterestedSymbolForOrs()` 动态注册方法（C++ 仅构造函数内注册）
- 将 `getExchangeIdFromName()` 移至正确位置（C++ 属于 MarketUpdateNew）
- 保留 `getClientId()`/`getShmMgr()`/`getRequestQueue()` getter 但标注 [C++差异-Java getter 惯例]
- 保留 `readAccountIdFromResponse()`/`readSymbolFromResponse()` 但标注 [C++差异-语言适配]

### 调用方更新
- TraderMain.java: 适配 `close()`→`stop()` 更名、移除 `addInterestedSymbol` 调用
- ConnectorTest.java / ConnectorTestHelper.java: 适配 API 变更

## Capabilities

### New Capabilities

### Modified Capabilities

## Impact

- `Connector.java` — 主要修改文件，方法增删、签名调整
- `TraderMain.java` — 适配 Connector API 变更
- `ConnectorTestHelper.java` — 适配 destroy→stop 等变更
- `ConnectorTest.java` — 适配测试 API
- `Constants.java` — 可能需要添加 `DEFAULT_NOT_POSSIBLE_CLIENTID` 常量
