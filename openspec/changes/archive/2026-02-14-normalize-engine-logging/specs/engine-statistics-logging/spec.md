# 策略引擎统计日志规范化

## 修改的需求

### 需求：统计输出使用标准日志格式

`PrintStatistics()` 的所有输出必须通过 `log.Printf` 发送，使用 `[StrategyEngine]` 前缀，与同文件中其他日志保持一致。

#### 场景：调用 PrintStatistics() 输出统计信息

- **WHEN** 调用 `StrategyEngine.PrintStatistics()`
- **THEN** 所有输出行通过 `log.Printf` 发送，自动带有时间戳前缀
- **AND** 每行输出包含 `[StrategyEngine]` 模块前缀
- **AND** 不产生任何 `fmt.Println` 或 `fmt.Printf` 的 stdout 直接输出

### 需求：保留原有统计内容

统计信息的业务内容不变，仅改变输出通道。

#### 场景：统计信息包含完整策略数据

- **WHEN** 引擎中存在已注册的策略
- **THEN** 每个策略输出以下信息：策略 ID、类型、运行状态、预估持仓（净/多/空）、盈亏（总/已实现/未实现）、订单统计（信号/发送/成交/拒绝）

#### 场景：引擎无策略时的输出

- **WHEN** 引擎中没有已注册的策略
- **THEN** 仅输出标题分隔线，不输出策略详情
