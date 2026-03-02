## Why

Java CommonClient 发单时未设置 RequestMsg.Exchange_Type 字段（默认为 0），导致 counter_bridge 的 `SetCombOffsetFlag()` 将 SHFE 合约误判为非 SHFE，对今仓使用 CLOSE_YESTERDAY 而非 CLOSE_TODAY，CTP 返回 ErrorID=51（平仓位不足）拒绝订单。同时 counter_bridge 启动时 `QueryPositions()` 因 CTP 限频返回空导致 `g_mapContractPos` 初始化失败，以及 CTP 错误消息 GBK 编码在 UTF-8 终端显示乱码。

## What Changes

- **Java CommonClient**: `sendNewOrder()`、`sendModifyOrder()`、`sendCancelOrder()` 添加 `Exchange_Type` 字段设置，与 C++ `FillReqInfo()` 中 `m_reqMsg.Exchange_Type = m_exchangeType` 对齐
- **Java CommonClient**: 新增 `exchangeType` 字段，从 `CfgConfig.exchanges` 解析映射（如 "CHINA_SHFE" → 57）
- **counter_bridge.cpp**: `g_mapContractPos` 初始化时，`QueryPositions()` 返回空则 fallback 到 `GetCachedPositions()`
- **ctp_td_plugin.cpp**: CTP `pRspInfo->ErrorMsg` 从 GBK 转 UTF-8 后再输出日志

## Capabilities

### New Capabilities
- `exchange-type-request`: Java 发单请求正确填充 Exchange_Type 字段，确保 counter_bridge 能正确识别交易所类型并执行 SHFE 开平标志逻辑
- `position-init-fallback`: counter_bridge 启动时持仓查询容错，QueryPositions 失败时使用 GetCachedPositions 兜底
- `gbk-utf8-logging`: CTP 插件错误消息 GBK→UTF-8 转码，确保日志可读

### Modified Capabilities

## Impact

- `tbsrc-java/src/main/java/com/quantlink/trader/core/CommonClient.java` — 发单方法添加 Exchange_Type
- `tbsrc-java/src/main/java/com/quantlink/trader/config/CfgConfig.java` — 添加 exchange 字符串→字节映射
- `tbsrc-java/src/main/java/com/quantlink/trader/TraderMain.java` — 初始化时设置 CommonClient.exchangeType
- `gateway/src/counter_bridge.cpp` — g_mapContractPos 初始化 fallback 逻辑
- `gateway/plugins/ctp/src/ctp_td_plugin.cpp` — GBK→UTF-8 转码
- 所有 SHFE 合约的 BUY 订单受影响（之前 CLOSE 方向订单全部被拒）
