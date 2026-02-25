# ExecutionStrategy 基类规格

## 概述
迁移自 `tbsrc/Strategies/include/ExecutionStrategy.h` + `ExecutionStrategy.cpp`。
抽象基类，包含位置管理、PNL计算、订单管理、阈值设置、风控检查。

## 字段（~200个，匹配C++）
- 位置字段: netpos, netpos_pass, netpos_pass_ytd, netpos_agg
- PNL字段: realisedPNL, unrealisedPNL, netPNL, grossPNL, maxPNL, drawdown
- 阈值字段: tholdBidPlace/Remove, tholdAskPlace/Remove, tholdMaxPos, tholdBeginPos, tholdSize, tholdInc
- 方向性阈值: tholdBidSize/MaxPos, tholdAskSize/MaxPos
- 订单统计: buyOpenOrders, sellOpenOrders, tradeCount, orderCount, cancelCount, confirmCount
- 交易量: buyQty/sellQty, buyTotalQty/sellTotalQty, buyValue/sellValue
- 状态标志: onExit, onCancel, onFlat, aggFlat, active
- 引用: client(CommonClient), instru(Instrument), thold(ThresholdSet), simConfig(SimConfig), configParams(ConfigParams)
- 订单Map: ordMap, bidMap, askMap

## 核心方法
1. **构造函数** — 初始化client/configParams/simConfig/instru/thold/strategyID
2. **reset()** — 重置所有状态到初始值
3. **setThresholds()** — 基于持仓的阶梯阈值
4. **setLinearThresholds()** — 线性插值阈值
5. **orsCallBack(MemorySegment)** — 订单响应分发（12种类型）
6. **mdCallBack(MemorySegment)** — 行情回调（PNL更新、风控检查）
7. **sendOrder()** — 纯虚方法
8. **sendNewOrder()** — 创建OrderStats、插入ordMap/priceMap/orderIDStrategyMap
9. **sendModifyOrder()** — 修改订单
10. **sendCancelOrder()** — 按orderID或按price+side撤单
11. **processTrade()** — 成交处理（更新位置、量、值、PNL）
12. **calculatePNL()** — unrealisedPNL/grossPNL/netPNL/drawdown
13. **checkSquareoff()** — 风控检查（时间、最大亏损、最大订单数）
14. **handleSquareoff()** — 执行平仓
