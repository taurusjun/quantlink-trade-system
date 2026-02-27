# Spec: 事件级订单历史

## 概述

在 ExecutionStrategy 中添加事件级订单历史环形缓冲区，在订单生命周期事件发生时立即记录，
解决模拟器快速填单导致快照采集遗漏的问题。

## 需求

### Requirement: 订单事件记录
- `sendNewOrder()` 后记录 "NEW" 事件
- `processTrade()` 中全部成交时记录 "TRADED" 事件
- `processCancelConfirm()` 中记录 "CANCEL_CONFIRM" 事件
- `processNewReject()` 中记录 "NEW_REJECT" 事件

### Requirement: DashboardSnapshot 读取 orderHistory
- `collectLeg()` 优先从 `leg.orderHistory` 读取
- 同一 orderID 多条记录保留最后一条（去重）
- 补充 ordMap 中尚未进入 history 的活跃订单

### Requirement: Instrument.instrument 字段对齐 C++
- TraderMain 中创建 Instrument 时同时设置 `symbol` 和 `instrument`
- 对齐 C++: `strcpy(m_instrument, symbol); strcpy(m_symbol, m_instrument)`

### Requirement: 模型文件方向性 SIZE 参数
- 添加 BID_SIZE/BID_MAX_SIZE/ASK_SIZE/ASK_MAX_SIZE
- PairwiseArbStrategy.setThresholds() 使用 `Math.max(BID_MAX_SIZE, ASK_MAX_SIZE)` 计算 tholdMaxPos
