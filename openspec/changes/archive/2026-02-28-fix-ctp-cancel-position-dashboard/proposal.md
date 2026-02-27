# CTP 撤单修复 + 持仓初始化 + Dashboard 显示修复

## Why

CTP 实盘测试中发现三个关键 bug：
1. counter_bridge 启动时 `g_mapContractPos` 为空，所有订单默认 OPEN 而非 CLOSE
2. CTP SessionID 为负数时撤单失败（order ID 格式解析错误）
3. Dashboard 持仓显示只统计当日成交，不含昨仓

## What Changes

### 1. counter_bridge 持仓初始化
- 启动后通过 `broker->QueryPositions()` 查询 CTP 持仓初始化 `g_mapContractPos`
- 过滤 volume=0 的空记录（CTP 会返回两个方向的记录即使无持仓）

### 2. CTP 撤单对齐 C++ 原代码
- 原代码（ors/China/）直接用成员变量 `FRONT_ID`/`SESSION_ID` + 缓存中的 `OrderRef`
- 新 gateway 原先把三者拼成字符串再解析，负数 SessionID 导致解析失败
- 改为与 C++ 一致：`OrderInfo` 增加 `order_ref` 字段，撤单时直接用成员变量

### 3. Dashboard 持仓显示修复
- `OverviewSnapshot` 中 l1/l2 从 `netpos`（仅当日）改为 `netposPass`/`netposAgg`（含昨仓）
- Position 表格同步修复

## Capabilities

- ctp-cancel-order: CTP 撤单逻辑对齐 C++
- counter-bridge-position: 持仓初始化
- dashboard-position: Dashboard 持仓显示

## Impact

- `gateway/include/plugin/td_plugin_interface.h` — OrderInfo 增加 order_ref 字段
- `gateway/plugins/ctp/src/ctp_td_plugin.cpp` — SendOrder/CancelOrder/ConvertOrder
- `gateway/src/counter_bridge.cpp` — 启动时查询 CTP 持仓
- `tbsrc-java/.../OverviewSnapshot.java` — 持仓显示修复
