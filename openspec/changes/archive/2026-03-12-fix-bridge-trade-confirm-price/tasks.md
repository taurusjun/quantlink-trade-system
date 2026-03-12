## 1. counter_bridge.cpp 修改

- [x] 1.1 新增 `g_order_sys_id_map` 全局变量（`std::map<std::string, std::string>`，OrderSysID → broker_order_id）
- [x] 1.2 在 `OnBrokerOrderCallback` 的 lock 作用域内建立反向映射（`client_order_id` → `order_id`）
- [x] 1.3 修改 `OnBrokerOrderCallback` 的 PARTIAL_FILLED/FILLED 分支：移除 TRADE_CONFIRM 生成，改为 log + return
- [x] 1.4 重写 `OnBrokerTradeCallback`：通过 `g_order_sys_id_map` 查找 CachedOrderInfo，构建 TRADE_CONFIRM ResponseMsg（price=trade_info.price, qty=trade_info.volume）

## 2. Simulator 插件修改

- [x] 2.1 修改 `simulator_plugin.cpp` 的 `ConvertToOrderInfo`：将 `order_id` 写入 `client_order_id`（与 CTP 的 OrderSysID 行为一致）

## 3. 编译验证

- [x] 3.1 编译 C++ gateway（cmake + make counter_bridge）
- [x] 3.2 部署到 deploy_java 并编译验证通过（binary 包含新 OnBrokerTradeCallback 实现，Simulator 启动正常，无编译/运行时错误）
