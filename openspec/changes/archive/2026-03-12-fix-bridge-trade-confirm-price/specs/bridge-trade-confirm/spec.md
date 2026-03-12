## ADDED Requirements

### Requirement: OnBrokerTradeCallback generates TRADE_CONFIRM
The counter_bridge SHALL generate TRADE_CONFIRM ResponseMsg exclusively in `OnBrokerTradeCallback`, using `TradeInfo.price` (actual fill price) and `TradeInfo.volume` (per-trade quantity).

#### Scenario: CTP fill with actual price
- **WHEN** CTP OnRtnTrade fires with price=8100.0 and volume=1
- **THEN** counter_bridge writes a TRADE_CONFIRM ResponseMsg with Price=8100.0 and Quantity=1 to the response SHM queue

#### Scenario: Partial fill generates per-trade quantity
- **WHEN** an order for 3 lots is partially filled with 1 lot at price=8100.0, then 2 lots at price=8101.0
- **THEN** counter_bridge writes two TRADE_CONFIRM messages: first with Quantity=1 Price=8100.0, second with Quantity=2 Price=8101.0

#### Scenario: Simulator fill uses same path
- **WHEN** Simulator plugin fires trade callback with price=8100.0 and volume=1
- **THEN** counter_bridge writes a TRADE_CONFIRM ResponseMsg with Price=8100.0 and Quantity=1 (same code path as CTP)

### Requirement: OrderSysID reverse mapping for CachedOrderInfo lookup
The counter_bridge SHALL maintain a reverse mapping `g_order_sys_id_map` (OrderSysID → broker_order_id) built during `OnBrokerOrderCallback`, enabling `OnBrokerTradeCallback` to find the corresponding `CachedOrderInfo`.

#### Scenario: CTP trade callback finds cached order via reverse mapping
- **WHEN** OnBrokerOrderCallback receives an order with order_id="1-1-00001" and client_order_id="12345"
- **AND** OnBrokerTradeCallback later receives a trade with order_id="12345"
- **THEN** the trade callback finds CachedOrderInfo via g_order_sys_id_map["12345"] → "1-1-00001" → g_order_map["1-1-00001"]

#### Scenario: Trade with unknown order_id is dropped
- **WHEN** OnBrokerTradeCallback receives a trade with order_id not in g_order_sys_id_map
- **THEN** the trade is logged as error and not forwarded to the response queue

### Requirement: Simulator client_order_id aligned with CTP
The Simulator plugin's `ConvertToOrderInfo` SHALL set `client_order_id` to `order_id` (e.g., "SIM_xxx"), mirroring CTP's behavior where `client_order_id` holds OrderSysID.

#### Scenario: Simulator order callback sets client_order_id = order_id
- **WHEN** Simulator generates an order with order_id="SIM_123_1"
- **THEN** ConvertToOrderInfo sets client_order_id="SIM_123_1"
- **AND** counter_bridge builds reverse mapping g_order_sys_id_map["SIM_123_1"] → "SIM_123_1"
