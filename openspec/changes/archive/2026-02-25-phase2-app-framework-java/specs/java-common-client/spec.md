# Spec: Java CommonClient 回调分发中枢

## 概述
迁移 C++ `CommonClient`（`tbsrc/main/include/CommonClient.h` + `CommonClient.cpp`）。

## 需求

### 核心职责
1. **MD 分发** — 收到 MarketUpdateNew 后按 `symbolID` 路由到对应策略
2. **ORS 分发** — 收到 ResponseMsg 后调用 `ORSCallback` 路由到策略
3. **发单封装** — SendNewOrder/SendModifyOrder/SendCancelOrder 委托给 Connector

### MD 分发逻辑（SendINDUpdate）
对应 C++ `CommonClient::SendINDUpdate()`:
1. 从 MarketUpdateNew 读取 `symbolID`
2. 通过 `configParams.simConfigList[symbolID]` 查找 SimConfigList
3. 遍历 SimConfigList，找到对应 Instrument
4. 更新 Instrument 订单簿（`fillOrderBook`）
5. 调用 `MDCallback`（路由到策略的 `MDCallBack`）

### ORS 分发逻辑（SendInfraORSUpdate）
对应 C++ `CommonClient::SendInfraORSUpdate()`:
1. 直接调用 `ORSCallback`
2. ORSCallback 在 main 中设置为按 orderID 路由到策略

### 发单接口
- `sendNewOrder(...)` — 构造 RequestMsg MemorySegment，写入 Connector request queue
- `sendModifyOrder(...)` — 修改单
- `sendCancelOrder(...)` — 撤单
- 使用 `Connector.getUniqueOrderNumber()` 生成 orderID

### 回调注册
- `setMDCallback(MDCallback)` — 注册行情回调
- `setORSCallback(ORSCallback)` — 注册回报回调
- `setConnector(Connector)` — 注册 Connector

### 信号处理
- `handleSquareOff()` — 平仓信号处理（SIGTSTP 等）
- `setActive(boolean)` — 激活/停用

### C++ 对照
- 迁移自: `tbsrc/main/include/CommonClient.h:78-132` + `CommonClient.cpp`
- SendINDUpdate: `CommonClient.cpp:401-769` — symbolID 分发核心逻辑
- SendInfraORSUpdate: `CommonClient.cpp:277-319` — ORS 回调分发
