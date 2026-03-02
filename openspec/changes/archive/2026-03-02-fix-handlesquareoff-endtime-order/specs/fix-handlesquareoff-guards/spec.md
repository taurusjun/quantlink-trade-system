# Spec: handleSquareoff 三层守卫

## 概述

修复 CTP 实盘中策略在 endTime 后启动时 handleSquareoff 误发单的 bug。

## Requirement: 子 strat 不调用基类 handleSquareoff

当 `simConfig.useArbStrat=true` 时，ExecutionStrategy.checkSquareoff() 中
`if (onFlat)` 块不调用 `handleSquareoff()`。

#### Scenario: useArbStrat=true 子 strat checkSquareoff 触发 onFlat

- 给定: ExtraStrategy 作为 PairwiseArbStrategy 的子 strat (useArbStrat=true)
- 当: checkSquareoff 检测到 END TIME，设置 onFlat=true
- 则: 不调用基类 handleSquareoff()，不发送任何订单

#### Scenario: useArbStrat=false 独立策略 checkSquareoff 触发 onFlat

- 给定: ExecutionStrategy 独立运行 (useArbStrat=false)
- 当: checkSquareoff 检测到 END TIME，设置 onFlat=true
- 则: 正常调用 handleSquareoff()

## Requirement: 基类 handleSquareoff active=false 不发单

ExecutionStrategy.handleSquareoff() 中发送平仓订单前检查 active 标志。

#### Scenario: active=false 且有持仓

- 给定: netpos=82, active=false
- 当: handleSquareoff() 被调用
- 则: 不发送任何 sendNewOrder，记录警告日志

#### Scenario: active=true 且有持仓

- 给定: netpos=82, active=true
- 当: handleSquareoff() 被调用
- 则: 正常发送 sendNewOrder 平仓

## Requirement: PairwiseArb active=false 时跳过 endTime 检查

PairwiseArbStrategy.mdCallBack() 中 endTime 检查被 active 守卫包裹。

#### Scenario: active=false 且 endTime 已过

- 给定: PairwiseArbStrategy active=false, 当前时间 > endTimeEpoch
- 当: mdCallBack 收到行情
- 则: 不设置 onExit/onFlat，不调用 handleSquareoff

#### Scenario: active=true 且 endTime 已过

- 给定: PairwiseArbStrategy active=true, 当前时间 > endTimeEpoch
- 当: mdCallBack 收到行情
- 则: 设置 onExit/onFlat=true，调用 handleSquareoff

## Requirement: 综合事故重现场景

#### Scenario: 昨仓 + endTime 过后 + active=false 不发单

- 给定: firstStrat.netpos=82, secondStrat.netpos=-83, active=false, endTime 已过
- 当: 第一次 mdCallBack
- 则: 无任何 sendNewOrder 调用（0 笔订单）
