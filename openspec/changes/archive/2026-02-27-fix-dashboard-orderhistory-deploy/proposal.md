# Proposal: Dashboard 订单历史 + Overview 数据修复

## Why

Overview 页面的 Fills、Spread Trades 表格始终为空，Orders 表格仅显示当前活跃挂单，无法展示已成交订单。
根本原因是模拟器填单极快（~150ms），快照采集间隔 1 秒，绝大部分订单在两次快照之间完成生命周期，被 ordMap 错过。

同时发现两个阻塞交易的 bug：
1. `Instrument.instrument` 字段未赋值，导致 `isStratSymbol` 始终为 false，`mdCallBack` 从未被调用
2. 模型文件缺少 `BID_MAX_SIZE`/`ASK_MAX_SIZE` 参数，导致 `tholdMaxPos=0`，所有订单被阻止

## What Changes

1. **ExecutionStrategy 添加事件级订单历史缓冲区** — 在订单创建/成交/撤单/拒绝时记录事件到 `orderHistory` 环形缓冲区
2. **DashboardSnapshot.collectLeg() 从 orderHistory 读取** — 不再仅依赖 ordMap 快照
3. **TraderMain 修复 Instrument.instrument 字段** — 对齐 C++ `strcpy(m_instrument, symbol)`
4. **模型文件添加方向性 SIZE 参数** — BID_SIZE/BID_MAX_SIZE/ASK_SIZE/ASK_MAX_SIZE
5. **.gitignore 添加运行时产物** — io/, org/, .gateway_mode

## Capabilities

- order-history: 事件级订单历史记录与展示

## Impact

- 修复 Overview 页面 Fills/Spread Trades/Orders 数据
- 修复策略无法产生交易的两个 bug
- 无破坏性变更
