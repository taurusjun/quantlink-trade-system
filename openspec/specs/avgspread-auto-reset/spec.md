## ADDED Requirements

### Requirement: inactive 状态下 AVG_SPREAD_AWAY 不触发退出
PairwiseArbStrategy.mdCallBack() 中的 AVG_SPREAD_AWAY 检查 SHALL 仅在 active=true 时触发策略退出。inactive 状态下仅输出 warning 日志。

#### Scenario: inactive 时 avgSpread 漂移超过阈值
- **WHEN** 策略 active=false 且 |currSpread - avgSpread| > AVG_SPREAD_AWAY
- **THEN** 输出 log.warning 标记漂移但不设置 onExit=true，策略继续等待激活

#### Scenario: active 时 avgSpread 漂移超过阈值
- **WHEN** 策略 active=true 且 |currSpread - avgSpread| > AVG_SPREAD_AWAY
- **THEN** 保持现有行为：设置 onExit=true，输出 error 日志，策略退出

### Requirement: 激活时 avgSpread 漂移自动修复日志
PairwiseArbStrategy.handleSquareON() 在重置 avgSpreadRatio 时，如果检测到旧值与市场价差存在显著漂移，SHALL 输出显著的 warning 日志标记自动修复。

#### Scenario: 激活时 avgSpread 与市场价差偏差大于 AVG_SPREAD_AWAY
- **WHEN** handleSquareON() 执行重置，且 |oldAvgSpread - liveSpread| > AVG_SPREAD_AWAY
- **THEN** 输出 log.warning 格式: `[AVG-SPREAD-DRIFT] 检测到跨天漂移，自动修复: oldAvg=371.3 -> newAvg=523.5 (drift=152.2, threshold=110)`

#### Scenario: 激活时 avgSpread 与市场价差偏差在正常范围
- **WHEN** handleSquareON() 执行重置，且 |oldAvgSpread - liveSpread| <= AVG_SPREAD_AWAY
- **THEN** 输出 log.info（现有日志），无额外漂移警告
