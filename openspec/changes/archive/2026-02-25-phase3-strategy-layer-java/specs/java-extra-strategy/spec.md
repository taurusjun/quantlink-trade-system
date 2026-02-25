# ExtraStrategy 规格

## 概述
迁移自 `tbsrc/Strategies/include/ExtraStrategy.h` + `ExtraStrategy.cpp`。
继承 ExecutionStrategy，添加 Instrument 参数化的订单方法。

## 关键差异
ExtraStrategy 的 SendBidOrder/SendAskOrder/SendNewOrder/SendModifyOrder/SendCancelOrder
都接受 `Instrument* instrument` 参数，允许在不同合约上操作。

## 方法
1. **SendBidOrder(instrument, ...)** — 买单，基于 tholdSize 计算数量
2. **SendAskOrder(instrument, ...)** — 卖单
3. **SendBidOrder2(instrument, ...)** — 基于 tholdBidSize，返回 boolean
4. **SendAskOrder2(instrument, ...)** — 基于 tholdAskSize，返回 boolean
5. **SendNewOrder(side, price, qty, level, instrument, ...)** — instrument 参数化
6. **SendModifyOrder(instrument, ...)** — instrument 参数化
7. **SendCancelOrder(instrument, orderID)** — instrument 参数化
8. **SendCancelOrder(instrument, price, side)** — 按价格撤单
9. **sendOrder()** — 空实现
