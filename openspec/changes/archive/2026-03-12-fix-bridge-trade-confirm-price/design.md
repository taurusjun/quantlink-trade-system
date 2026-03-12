## Context

`counter_bridge.cpp` 是 C++ 网关层的订单回报桥，负责将 CTP/Simulator 插件的回调转换为 hftbase ResponseMsg 写入 SHM。当前 `OnBrokerOrderCallback` 在 FILLED/PARTIAL_FILLED 状态下生成 TRADE_CONFIRM，使用了 `OrderInfo.price`（限价）和 `OrderInfo.traded_volume`（累计量），而非 CTP 的实际成交价和单笔成交量。

参考 legacy ORS (`ors/China/src/ORSServer.cpp`)，正确做法是 `OnRtnOrder` 只处理 NEW_ORDER_CONFIRM 和 CANCEL，`OnRtnTrade` 处理 TRADE_CONFIRM。

CTP 数据流中的 ID 映射问题：
- `OrderInfo.order_id` = `"FrontID-SessionID-OrderRef"`（`g_order_map` 的 key）
- `OrderInfo.client_order_id` = `OrderSysID`（交易所分配）
- `TradeInfo.order_id` = `OrderSysID`
- 需要反向映射才能从 TradeInfo 关联到 CachedOrderInfo

## Goals / Non-Goals

**Goals:**
- TRADE_CONFIRM 使用实际成交价和单笔成交量
- CTP 和 Simulator 两种模式行为一致（以 CTP 为基准）
- 与 legacy ORS 的回调分工对齐

**Non-Goals:**
- 不修改 ResponseMsg 结构体
- 不修改策略层代码
- 不在 counter_bridge 中为 Simulator 做特殊适配

## Decisions

### 1. TRADE_CONFIRM 统一由 OnBrokerTradeCallback 生成
- **选择**: 移除 OnBrokerOrderCallback 中的 TRADE_CONFIRM 生成，改由 OnBrokerTradeCallback 负责
- **理由**: 与 legacy ORS 一致；OnRtnTrade 提供正确的 price 和 volume

### 2. 新增 g_order_sys_id_map 反向映射
- **选择**: 在 OnBrokerOrderCallback 中建立 `OrderSysID → broker_order_id` 映射
- **备选**: 修改 TradeInfo 增加 order_ref 字段 → 侵入性太强，需改接口
- **理由**: 最小改动，CTP 保证 OnRtnOrder 先于 OnRtnTrade 到达

### 3. Simulator 修改与 CTP 一致
- **选择**: 修改 Simulator 的 `ConvertToOrderInfo`，将 `order_id` 写入 `client_order_id`
- **理由**: 以 CTP 实盘为基准，不在 counter_bridge 中做双重查找适配

## Risks / Trade-offs

- **[CTP OnRtnOrder 延迟]** → CTP 保证同线程内 OnRtnOrder 先于 OnRtnTrade，无竞态风险
- **[Simulator client_order_id 语义变化]** → 原 client_order_id 存的是 counter_bridge 传入的值，修改后变为 order_id。不影响 counter_bridge（它只读 order_id 和 client_order_id 用于映射）
