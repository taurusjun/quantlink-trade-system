# Design: 修复 handleSquareoff 在 endTime 后启动时误发单

## 根因分析

### 事故链路

```
策略 14:58 启动 (endTime=14:57)
  → firstStrat.mdCallBack()
    → checkSquareoff(): exchTS >= endTimeEpoch → onFlat=true
    → if (onFlat) handleSquareoff()  ← 这是基类 ExecutionStrategy 版本!
      → netpos=82 → sendNewOrder(SELL, 82, flag=POS_OPEN)
  → secondStrat.mdCallBack()
    → 同理 → netpos=-83 → sendNewOrder(BUY, 83, flag=POS_OPEN)
  → ag2605 BUY 83 成交!
```

### 为什么 C++ 不触发

C++ 策略启动即 activate，不等待 SIGUSR1，启动时 endTime 尚未到达。
Java CTP 模式需等待手动激活（active=false），可能在 endTime 后才启动。

### 关键区分

- `PairwiseArbStrategy.handleSquareoff()`: 只撤单 + 设标志 + 保存 daily_init（安全）
- `ExecutionStrategy.handleSquareoff()`: 撤单 + **发送平仓订单**（危险，绕过 active 检查）

## 修复方案

### Fix 1: 子 strat 不调用基类 handleSquareoff

`ExecutionStrategy.checkSquareoff()` 中 `if (onFlat)` 块：
- 当 `simConfig.useArbStrat=true` 时跳过 `handleSquareoff()` 调用
- 子 strat 只负责设置标志，平仓由父级 PairwiseArbStrategy 管理

### Fix 2: 基类 handleSquareoff 加 active 守卫

`ExecutionStrategy.handleSquareoff()` 中发送平仓订单前检查 `active`：
- `active=false` 时拒绝发单，记录警告日志

### Fix 3: 未激活时跳过 endTime 检查

`PairwiseArbStrategy.mdCallBack()` 中 endTime 检查用 `if (active)` 包裹：
- 未激活策略不触发时间限制平仓

## 测试策略

| 测试场景 | 覆盖修复 | 验证点 |
|----------|----------|--------|
| 子 strat checkSquareoff 设 onFlat 后不调用 handleSquareoff | Fix 1 | useArbStrat=true 时 handleSquareoff 不被调用 |
| 基类 handleSquareoff active=false 不发单 | Fix 2 | netpos!=0 但 active=false 时无订单 |
| PairwiseArb active=false 时 endTime 不触发 | Fix 3 | endTime 过后 mdCallBack 不设 onExit |
| PairwiseArb active=true 时 endTime 正常触发 | 回归 | endTime 到达后正确 squareoff |
| 综合: 昨仓+endTime过后+active=false 不发单 | All | 完整事故重现场景 |
