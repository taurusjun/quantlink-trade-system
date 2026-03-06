# Tasks

## 1. ConnectorConfig 完整对齐 C++ ConnectorConfig

- [x] 新增字段: `responseFilterType` (STRATEGY_FILTER / TICKERS_ON_ONE_ACCOUNT_FILTER)
- [x] 新增字段: `interestedAccount` (String)
- [x] 新增字段: `interestedSymbolsForOrs` (Set<String>)
- [x] 新增字段: `asyncMdAndResponse` (boolean)
- [x] 新增字段: 多交易所 SHM 映射 (`Map<String, ExchangeConfig>`)
- [x] 新增枚举: `ResponseFilterType`, `ReadMDShmMode`

## 2. Connector 字段完整对齐

- [x] `m_requestQueue[MAX_EXCHANGE_COUNT]` → `Map<Integer, MWMRQueue> requestQueues`
- [x] `m_response_queue_to_exchange_map[MAX_EXCHANGE_COUNT]` → `int[] responseQueueToExchangeMap`
- [x] `m_interestedsymbols_for_ors` → `Set<String> interestedSymbolsForOrs`
- [x] `m_responseFilterType` → `int responseFilterType`
- [x] `m_mdSeqNum` → `long mdSeqNum`
- [x] `m_runForAllSymbols` → `boolean runForAllSymbols`
- [x] `m_liveReqCb` → `boolean liveReqCb`

## 3. 构造函数多交易所初始化

- [x] 遍历 INTERESTED_EXCHANGES，为每个交易所注册 SHM 队列
- [x] `getExchangeIdFromName()` 静态方法迁移
- [x] `m_response_queue_to_exchange_map` 正确填充
- [x] 支持 ROUND_ROBIN / UNTIL_ENDPACKET 两种 MD 读取模式

## 4. SendNewOrder/PushRequest 使用 Exchange_Type

- [x] `sendNewOrder()` 从 req 中读取 Exchange_Type 传入 getUniqueOrderNumber
- [x] `PushRequest` 逻辑使用 `requestQueues[exchType]` 而非固定 requestQueue

## 5. HandleOrderResponse 完整对齐

- [x] 重命名 `handleOrderResponseFromShmMgr` → `handleOrderResponse`
- [x] 实现 TICKERS_ON_ONE_ACCOUNT_FILTER 分支
- [x] `m_response_queue_to_exchange_map[queueNum]` → exchId 查找

## 6. GetOrderNumberWithNewClientId 多交易所

- [x] 遍历所有 INTERESTED_EXCHANGES 重新注册
- [x] 对齐 C++ connector.cpp:1152-1182 完整逻辑

## 7. 补齐其他方法

- [x] `FillExchangeSpecificReqInfo()` — 按 Exchange_Type 填充 OrdType/PxType/Duration
- [x] `getSymbolIDMap()` — 返回 interestedSymbolsForMd 映射
- [x] `setToRunForAllSymbols()` — 设置 runForAllSymbols = true

## 8. 编译 + 模拟测试

- [x] 编译通过
- [x] 模拟测试：策略 92201 正常启动、无错误、无警告
