## 1. 代码修复（已完成）

- [x] 1.1 添加撤单拒绝处理字段和 CANCELREQ_PAUSE 逻辑到 sendCancelOrder(int)
- [x] 1.2 添加 bidMapCacheDel / askMapCacheDel 字段和 removeOrder() SelfBook 清理逻辑
- [x] 1.3 添加 handleSquareON() 基类方法，PairwiseArbStrategy 添加 super 调用
- [x] 1.4 补齐 ConfigParams (fillOnCxlReject, bSelfBook)、Instrument (bSnapshot, adjustBookWithAggCxl)、sweepOrdMap 字段
- [x] 1.5 编译验证通过

## 2. 单元测试（已完成）

- [x] 2.1 创建 TestableExecutionStrategy 测试子类和测试基础设施（mock CommonClient 等）
- [x] 2.2 测试 sendCancelOrder: 正常撤单、CROSS 不可撤、CANCELREQ_PAUSE 阻止/过期后放行、不同 orderID 不阻止
- [x] 2.3 测试 processCancelReject: 设置拒绝状态字段、SelfBook CacheDel 清理
- [x] 2.4 测试 removeOrder: SelfBook 模式下 bidMapCache/askMapCache 清理、条件性 ordMap 移除
- [x] 2.5 测试 handleSquareON: 基类调用 sendMonitorStratStatus
- [x] 2.6 运行全部测试确认通过
