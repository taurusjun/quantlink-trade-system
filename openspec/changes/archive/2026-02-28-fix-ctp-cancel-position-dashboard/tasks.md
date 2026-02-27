# Tasks

- [x] counter_bridge: 启动时通过 QueryPositions() 查询 CTP 持仓初始化 g_mapContractPos
- [x] counter_bridge: 过滤 volume=0 的空持仓记录
- [x] OrderInfo: 增加 order_ref 字段（对齐 C++ ordinfo.exchID）
- [x] CTPTDPlugin::SendOrder: 保存 order_ref 到 OrderInfo 缓存
- [x] CTPTDPlugin::ConvertOrder: 从 CTP 回调中保存 order_ref
- [x] CTPTDPlugin::CancelOrder: 用成员变量 m_front_id/m_session_id + order_ref 撤单（对齐 C++ ORSServer::SendCancelOrder）
- [x] OverviewSnapshot: l1/l2 从 netpos 改为 netposPass/netposAgg
- [x] OverviewSnapshot: Position 表格使用含昨仓的完整持仓
- [x] CTP 实盘验证: 撤单成功、持仓显示正确、策略正常运行
