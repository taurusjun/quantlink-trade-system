# Tasks

- [x] ExecutionStrategy 添加 OrderHistoryEntry 内部类和 orderHistory 环形缓冲区
- [x] ExecutionStrategy 添加 recordOrderEvent() 方法
- [x] sendNewOrder() 中调用 recordOrderEvent(ordStats, "NEW")
- [x] processTrade() 中全部成交时调用 recordOrderEvent(order, "TRADED")
- [x] processCancelConfirm() 中调用 recordOrderEvent(order, "CANCEL_CONFIRM")
- [x] processNewReject() 中调用 recordOrderEvent(order, "NEW_REJECT")
- [x] DashboardSnapshot.collectLeg() 从 orderHistory 读取 + ordMap 补充
- [x] TraderMain 设置 Instrument.instrument = symbol（修复 isStratSymbol 判断）
- [x] 模型文件添加 BID_SIZE/BID_MAX_SIZE/ASK_SIZE/ASK_MAX_SIZE 参数
- [x] .gitignore 添加运行时产物 (io/, org/, .gateway_mode)
- [x] 编译部署验证: Overview Fills=48, Spread Trades=12, Orders=1(活跃)
