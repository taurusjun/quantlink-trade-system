# Proposal: 修复 handleSquareoff 在 endTime 后启动时误发单

## Why

CTP 实盘中策略在 endTime（14:57）之后启动（14:58），END TIME limit 立即触发，
ExecutionStrategy 基类 handleSquareoff() 绕过 PairwiseArbStrategy 的 override，
直接对昨仓发出反向订单（SELL 82 ag2603 / BUY 83 ag2605），flag=POS_OPEN 而非 POS_CLOSE。
ag2605 BUY 83 手成交，造成实盘事故。

## What Changes

### 三层修复

1. **ExecutionStrategy.checkSquareoff()**: `useArbStrat=true`（子 strat）时跳过基类 `handleSquareoff()` 调用，
   平仓由父级 PairwiseArbStrategy 统一管理。
2. **ExecutionStrategy.handleSquareoff()**: `active=false` 时拒绝发送平仓订单（防御性守卫）。
3. **PairwiseArbStrategy.mdCallBack()**: `active=false` 时跳过 endTime 检查，未激活策略不触发时间限制平仓。

### 新增测试用例

覆盖以上三个修复场景的单元测试，确保回归可检测。

## Capabilities

- fix-handlesquareoff-guards: handleSquareoff 三层守卫修复

## Impact

- `ExecutionStrategy.java`: checkSquareoff + handleSquareoff 修改
- `PairwiseArbStrategy.java`: mdCallBack endTime 守卫
- `ExecutionStrategyTest.java`: 新增测试
- `PairwiseArbStrategyTest.java`: 新增测试
