# 设计文档

## 决策

### 1. counter_bridge 持仓初始化
- 在 broker login 成功后、启动 order processor 之前查询 CTP 持仓
- 仅当 `g_mapContractPos` 为空时执行（`--position-file` 优先）
- 过滤 `volume == 0` 的记录避免 CTP 返回的空方向记录

### 2. CTP 撤单对齐 C++ 原代码架构
- C++ 原代码（ors/China/src/ORSServer.cpp）：
  - `OrderRef` 存储在 `ordinfo.exchID`
  - 撤单直接用成员变量 `FRONT_ID`/`SESSION_ID`，不涉及字符串解析
- 新 gateway 对齐方案：
  - `OrderInfo` 增加 `order_ref[16]` 字段（对应 C++ `ordinfo.exchID`）
  - `SendOrder` 和 `ConvertOrder` 中保存 `order_ref`
  - `CancelOrder` 用 `m_front_id`/`m_session_id` + `order_info.order_ref`

### 3. Dashboard 持仓显示
- `netpos = buyTotalQty - sellTotalQty`（仅当日交易）
- `netposPass`（leg1, 含昨仓+今仓）和 `netposAgg`（leg2, 含昨仓）才是完整持仓
- OverviewSnapshot 改用后者
