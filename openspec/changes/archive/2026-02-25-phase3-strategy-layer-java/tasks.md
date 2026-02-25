# Phase 3 Tasks: 策略层

## 3.1 Phase 2 类型细化
- [x] 3.1.1 ConfigParams.orderIDStrategyMap 改为 `Map<Integer, ExecutionStrategy>`
- [x] 3.1.2 SimConfig.executionStrategy 改为 `ExecutionStrategy`

## 3.2 ExecutionStrategy 基类
- [x] 3.2.1 创建 `strategy/ExecutionStrategy.java` — 字段 + 构造函数 + reset()
- [x] 3.2.2 实现 setThresholds() + setLinearThresholds()
- [x] 3.2.3 实现 sendNewOrder() + sendModifyOrder() + sendCancelOrder()
- [x] 3.2.4 实现 orsCallBack() — 12种响应类型分发
- [x] 3.2.5 实现 processTrade() + processNewReject/ModifyReject/CancelReject/ModifyConfirm/CancelConfirm
- [x] 3.2.6 实现 calculatePNL() + checkSquareoff() + handleSquareoff()
- [x] 3.2.7 实现 mdCallBack() + getBidPrice/getAskPrice + sendBidOrder/sendAskOrder
- [x] 3.2.8 创建 ExecutionStrategyTest.java — 基础单元测试

## 3.3 ExtraStrategy
- [x] 3.3.1 创建 `strategy/ExtraStrategy.java` — Instrument 参数化订单方法
- [x] 3.3.2 创建 ExtraStrategyTest.java

## 3.4 PairwiseArbStrategy
- [x] 3.4.1 创建 `strategy/PairwiseArbStrategy.java` — 构造函数 + daily_init 加载
- [x] 3.4.2 实现 sendOrder() — 被动挂单 + 对冲逻辑
- [x] 3.4.3 实现 sendAggressiveOrder() + setThresholds() + orsCallBack() + mdCallBack()
- [x] 3.4.4 实现 handleSquareoff() + loadMatrix2/saveMatrix2 + calcPendingNetposAgg
- [x] 3.4.5 创建 PairwiseArbStrategyTest.java

## 3.5 编译验证
- [x] 3.5.1 全量编译通过（`mvn compile`）
- [x] 3.5.2 全量测试通过（`mvn test`）— 139 tests passed
