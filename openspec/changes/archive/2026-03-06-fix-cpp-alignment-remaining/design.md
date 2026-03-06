# Design: 修复剩余 C++ 对齐问题

## 修复清单

### Task 1: SetCheckCancelQuantity (HIGH)

**C++ 原代码**: `ExecutionStrategy.cpp:266-274`

```cpp
void ExecutionStrategy::SetCheckCancelQuantity() {
    if (!strcmp(m_instru->m_exchange, "FORTS") || !strcmp(m_instru->m_exchange, "KRX")
        || !strcmp(m_instru->m_exchange, "SFE") || !strcmp(m_instru->m_exchange, "SGX"))
        m_checkCancelQuantity = true;
    else
        m_checkCancelQuantity = false;
}
```

构造函数 line 80 调用 `SetCheckCancelQuantity()`。

**修改**: ExecutionStrategy.java
- 添加 `boolean checkCancelQuantity` 字段
- 添加 `setCheckCancelQuantity()` 方法，检查 exchange 是否为 FORTS/KRX/SFE/SGX
- 在构造函数末尾调用

### Task 2: FillMsg (HIGH)

**C++ 原代码**: `ExecutionStrategy.h:357`

```cpp
void FillMsg(uint32_t OrderID, TransactionType s, double price, int32_t qty,
             OrderHitType ordHitType, ExecutionStrategy *eStrategy, uint64_t ts, RequestMsg *req);
```

ExtraStrategy.cpp:344,450 调用此方法构建 RequestMsg。

**修改**: ExecutionStrategy.java
- 添加 `fillMsg()` 方法，将 sendNewOrder 中的 RequestMsg 字段设置逻辑抽取为独立方法
- 供 ExtraStrategy 复用

### Task 3: SendInfraReqUpdate (HIGH)

**C++ 原代码**: `CommonClient.cpp:256-274`

```cpp
void CommonClient::SendInfraReqUpdate(RequestMsg *request) {
    if (m_configParams->m_bCommonBook && m_dateConfig->m_simActive) {
        if (request->Quantity < 0) {
            request->Quantity *= -1;
            request->OrderID += 1000000;
        }
        auto iter = m_configParams->m_instruCBMap.find(request->Contract_Description.Symbol);
        if (iter != m_configParams->m_instruCBMap.end()) {
            iter->second->RequestCallBack(request);
        }
    }
}
```

**分析**: 此方法仅在 `m_bCommonBook=true` 时执行。CommonBook 已明确排除在中国期货场景之外（[C++差异] CommonClient.java:282）。因此 SendInfraReqUpdate 实际上永远不会执行。

**修改**: CommonClient.java — 添加空壳方法 + 注释说明依赖 CommonBook

### Task 4: 夜盘 currDate (MEDIUM)

**C++ 原代码**: `ExecutionStrategy.cpp:44-46`
```cpp
m_endTimeEpoch = Watch::GetNanoSecsFromEpoch(simConfig->m_dateConfig.m_currDate, 0) + (uint64_t)(m_endTime) * 1000000;
```

C++ Live 模式下 `m_currDate = localtime(now)` (TradeBotUtils.cpp:2534)。夜盘跨日时 m_currDate 仍然是今天日期。

**分析**: Java 已在 `initEndTimeEpochs()` (ExecutionStrategy.java:350-364) 中用 `baseDate.plusDays(1)` 处理夜盘跨日。这与 C++ Live 模式行为一致（都是今天日期 + 跨日时 +1）。

**BUT** — C++ 的 `m_currDate` 还被日志、MemLog 输出等大量引用。Java 在 `saveMatrix2()` 等需要格式化日期的地方用 `LocalDate.now()`，夜盘跨日后（凌晨）这个日期会变成第二天，与 C++ 的 `m_currDate`（进程启动时的日期，不变）不同。

**修改**: SimConfig.java — 添加 `String currDate` 字段，在 `initDateConfigEpoch()` 中初始化为进程启动时的日期字符串（YYYYMMDD），后续日志和文件操作统一使用此字段。

### Task 5: lastTradePx 更新条件 (MEDIUM)

**C++ 原代码**: `Instrument.cpp:2199-2202`
```cpp
if (update->m_updateType == MDUPDTYPE_TRADE || update->m_updateType == MDUPDTYPE_TRADE_IMPLIED
    || (update->m_updateType == MDUPDTYPE_TRADE_INFO && m_bUseTradeInfo)) {
    lastTradePx = update->m_newPrice;
    lastTradeqty = update->m_newQuant;
}
```

C++ 仅在 TRADE/TRADE_IMPLIED/TRADE_INFO 类型时更新。Java 当前每次 fillOrderBook 都从 SHM 读取。

**分析**: Java 的 md_shm_feeder 在 SHM 中始终维护最新 lastTradedPrice（包括非 TRADE 更新时保持上一次值）。所以 Java 读取 SHM 中的 lastTradedPrice 字段本身是正确的 — feeder 只在有 trade 时才更新该字段。但需要验证 feeder 行为。

**修改**: Instrument.java — 从 SHM 读取 updateType 字段，仅在 TRADE 类型时更新 lastTradePx/lastTradeQty，对齐 C++ 条件

### Task 6: Connector OrderID 溢出 (MEDIUM)

**C++ 原代码**: `Connector.h:364-371`
```cpp
if (illumiati_likely_branch(m_OrderCount < ORDERID_RANGE)) {
    return m_clientId[exchCode] * ORDERID_RANGE + (m_OrderCount++);
} else {
    return GetOrderNumberWithNewClientId(exchCode);
}
```

**修改**: Connector.java — 在 `getUniqueOrderNumber()` 中添加溢出检测，溢出时日志告警（不申请新 clientId — Java 模式下单个 exchange 不需要多 clientId）

### Task 7: 全局 DateConfig (MEDIUM)

**C++ 原代码**: `CommonClient.h:103 — DateConfig *m_dateConfig`

C++ CommonClient 有独立全局 `m_dateConfig`，用于：
- `SendInfraReqUpdate()` 中检查 `m_simActive`
- `Update()` 中 `m_dateConfig->UpdateActive()`

**分析**: Java 当前在每个 SimConfig 中有独立 dateConfig。CommonClient 的 `update()` 方法遍历所有 simConfig 并调用各自的 `updateActive()`。这与 C++ 行为已经语义等价。唯一差异是 SendInfraReqUpdate（已确认为 CommonBook 专用，不执行）。

**修改**: CommonClient.java — 添加 `currDate` 字段（进程启动日期），与 C++ `m_dateConfig->m_currDate` 对应，用于日志格式化

### Task 8: Tick 对象 (MEDIUM)

**C++ 原代码**: `Tick.h:22-42`
```cpp
class Tick {
    char baseName[50], instrument[50];
    double bidQty, askQty, bidPx, askPx, tickSize, tradePx, tradeQty;
    ticktype_t tickType;  // BIDQUOTE/ASKQUOTE/BIDTRADE/ASKTRADE/TRADEINFO/INVALID
    int tickLevel;
    uint64_t exchTS;
    uint16_t symbolID;
};
```

C++ 在 `SendInfraMDUpdate()` 中创建 Tick 对象传给 IndicatorCallBack。Java 的 `sendInfraMDUpdate()` 直接传 Instrument 和 MemorySegment。

**分析**: Java 的 indicator 系统已经适配了不使用 Tick 对象的方式。添加 Tick 类需要修改整个 indicator 回调链。tickLevel 信息在 Java 中用 updateLevel 替代。

**修改**: 不添加完整 Tick 类（架构差异太大），但在 Instrument 中添加 `tickType` 枚举字段和 `updateTickType()` 方法，在 fillOrderBook 后设置当前 tick 类型，供需要时查询。

## 非目标

- OptionManager/DeltaStrategy/VOLTHREAD — 中国期货不使用
- CommonBook/SelfBook — 中国期货不使用
- SMARTMD — md_shm_feeder 不产生
- 完整 Tick 类迁移 — 架构差异，用 tickType 枚举替代
