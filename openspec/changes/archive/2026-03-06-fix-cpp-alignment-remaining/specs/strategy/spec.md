## ADDED Requirements

### Requirement: SetCheckCancelQuantity 撤单数量追踪

ExecutionStrategy 必须实现 `setCheckCancelQuantity()` 方法，对齐 C++ `ExecutionStrategy::SetCheckCancelQuantity()`：当交易所为 FORTS/KRX/SFE/SGX 时 `checkCancelQuantity=true`，其他交易所为 `false`。构造函数中必须调用此方法。

#### Scenario: SFE 交易所启用撤单数量检查
- **WHEN** 合约交易所为 SFE（上海期货）
- **THEN** checkCancelQuantity 为 true

#### Scenario: 非支持交易所禁用撤单数量检查
- **WHEN** 合约交易所为 CME
- **THEN** checkCancelQuantity 为 false

### Requirement: FillMsg 请求消息构建方法

ExecutionStrategy 必须实现 `fillMsg()` 方法，对齐 C++ `ExecutionStrategy::FillMsg()`，将 sendNewOrder 中的 RequestMsg 字段设置逻辑抽取为可复用的独立方法，供 ExtraStrategy 调用。

#### Scenario: ExtraStrategy 调用 fillMsg 构建订单
- **WHEN** ExtraStrategy 需要为多合约发单
- **THEN** 可调用 fillMsg() 构建完整 RequestMsg，无需重复字段设置逻辑

### Requirement: SendInfraReqUpdate 请求队列回调

CommonClient 必须实现 `sendInfraReqUpdate()` 方法骨架，对齐 C++ `CommonClient::SendInfraReqUpdate()`。当前仅在 CommonBook 启用时执行（中国期货场景不启用），方法体为空壳 + 注释。

#### Scenario: CommonBook 未启用时不执行
- **WHEN** CommonBook 未启用（中国期货默认）
- **THEN** sendInfraReqUpdate() 立即返回，不执行任何操作

### Requirement: SimConfig currDate 进程启动日期

SimConfig 必须包含 `currDate` 字段（String, YYYYMMDD 格式），在 `initDateConfigEpoch()` 中初始化为进程启动时的日期。夜盘跨日后此字段不变（与 C++ `m_dateConfig.m_currDate` 行为一致）。

#### Scenario: 夜盘跨日后 currDate 不变
- **WHEN** 进程在 21:00 启动（currDate=20260306），运行到次日 01:00
- **THEN** currDate 仍为 "20260306"（进程启动日期）

### Requirement: lastTradePx 条件更新

Instrument.fillOrderBook() 中 lastTradePx/lastTradeQty 仅在 updateType 为 TRADE 相关类型时更新，对齐 C++ `Instrument::FillOrderBook()` 中的 `MDUPDTYPE_TRADE || MDUPDTYPE_TRADE_IMPLIED || MDUPDTYPE_TRADE_INFO` 条件。

#### Scenario: 非 TRADE 行情不更新 lastTradePx
- **WHEN** 收到 ORDER_ENTRY 类型行情更新
- **THEN** lastTradePx 保持上一次值不变

#### Scenario: TRADE 行情更新 lastTradePx
- **WHEN** 收到 TRADE 类型行情更新
- **THEN** lastTradePx 更新为 newPrice

### Requirement: Connector OrderID 溢出防护

Connector.getUniqueOrderNumber() 必须检测 orderCount 达到 ORDERID_RANGE 上限时的溢出情况，记录 SEVERE 级别日志告警。

#### Scenario: OrderID 溢出告警
- **WHEN** orderCount 达到 ORDERID_RANGE (1000000)
- **THEN** 记录 SEVERE 日志告警，返回 -1 表示失败

### Requirement: Instrument tickType 枚举

Instrument 必须包含 `tickType` 字段（枚举：BIDQUOTE/ASKQUOTE/BIDTRADE/ASKTRADE/TRADEINFO/INVALID），对齐 C++ `Tick::ticktype_t`。fillOrderBook() 后根据 updateType 设置 tickType。

#### Scenario: TRADE 行情设置 tickType
- **WHEN** 收到 TRADE 类型行情
- **THEN** tickType 设为 BIDTRADE 或 ASKTRADE（根据 side）

#### Scenario: ORDER_ENTRY 行情设置 tickType
- **WHEN** 收到 ORDER_ENTRY 类型行情
- **THEN** tickType 设为 BIDQUOTE 或 ASKQUOTE（根据 side）
