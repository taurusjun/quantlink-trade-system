## Context

原则 9 比对发现 Connector.java 与 C++ connector.h 存在结构性不一致。比对报告列出了 3 类问题：
- 严重：Java 缺失 C++ 方法（HandleORSRequests、PushRequest 批量版）
- 中等：入参数不一致、访问级别不一致、常量缺失
- 轻微：Java 发明了 C++ 不存在的字段/方法（allClientIds、close/destroy、addClientId 等）

## Goals / Non-Goals

**Goals:**
- Connector.java 方法/字段与 C++ connector.h 严格一一对应
- Java 不存在 C++ 没有的方法或字段（getter 除外，标注 [C++差异]）
- 编译通过 + sim 测试通过

**Non-Goals:**
- 不实现 SIMULATION/PAPERTRADING/PARALLELSIM/GUI 模式的方法（已知差异）
- 不实现 Instinet 相关重载（已知差异）
- 不实现 `#ifdef` 条件编译变体（Raw 版本等）

## Decisions

### D1: HandleORSRequests — 补齐空实现
C++ `HandleORSRequests(RequestMsg*)` 在 LIVE+REQUEST_CALLBACK 模式下被 ShmMgr 回调。Java 需要在构造函数中注册此回调到 shmMgr，方法内部可以为空（当前业务不使用 REQUEST_CALLBACK）。

### D2: PushRequest 批量版 — 补齐
C++ `PushRequest(unsigned char extype, Container<RequestMsg>&)` 遍历列表逐条 enqueue。Java 实现对应的 `pushRequest(List<MemorySegment> requests)` 方法。注意 C++ 签名有 2 个参数（extype + list），Java 对齐。

### D3: close/destroy 合并为功能等价方法
C++ 只有析构函数（~Connector 隐式）和 Stop()。Java 的 close()/destroy() 功能与 stop() 重复。
方案：移除 destroy()，将 close() 重命名为 shutdown()（用于 SHM 分离），并标注 [C++差异]。
但考虑 TraderMain 和 TestHelper 都调用 close()，更简单的方案是：保留 close() 标注 [C++差异-Java 资源管理]，移除 destroy()。

### D4: allClientIds 冗余字段移除
核心逻辑 handleOrderResponse 已使用 allClientIdsByExchange，allClientIds 未被使用，直接删除。

### D5: addInterestedSymbol / addInterestedSymbolForOrs 移除
C++ 中合约注册仅在构造函数内完成。TraderMain 已改为通过 Config.interestedSymbols 传入，不再调用这些方法。

### D6: addClientId 移除
C++ 无独立方法，逻辑内嵌在 GetOrderNumberWithNewClientId 中。

### D7: getExchangeIdFromName 迁移
C++ 中属于 MarketUpdateNew (marketupdateNew.h:881)。应移到 Constants.java 或独立工具类。Connector 内部调用改为 Constants.getExchangeIdFromName()。

### D8: pushRequest 访问级别 → public
C++ PushRequest 是 public inline 方法，Java 应对齐为 public。

### D9: startAsync 签名补齐
C++ StartAsync(SimEngineType = TBTEngine)。Java 添加参数但仅接受默认值，或保留无参版本标注 [C++差异]。
方案：保留无参 startAsync()，标注 [C++差异-仅LIVE模式，省略 SimEngineType 参数]。

### D10: setInstrumentCache / ifRunningForRussianExchange / HandleLiveMdUpdates 补齐
这些是 C++ 公有方法，Java 必须有对应方法。实现为空方法 + 注释说明：
- setInstrumentCache: 仅对俄罗斯交易所有效
- ifRunningForRussianExchange: 仅对俄罗斯交易所有效
- HandleLiveMdUpdates: C++ header 声明但无实现（死代码）

### D11: DEFAULT_NOT_POSSIBLE_CLIENTID 常量补齐
添加到 Constants.java: `public static final int DEFAULT_NOT_POSSIBLE_CLIENTID = 99999999;`

## Risks / Trade-offs

- [Risk] 移除 addInterestedSymbol 后如有外部代码调用会编译失败 → 全局搜索确认无引用
- [Risk] 移除 destroy() 后 TestHelper 需要改用 close() → 同步更新测试代码
- [Risk] getExchangeIdFromName 迁移后引用方需要更新 → 全局搜索替换
