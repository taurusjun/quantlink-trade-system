# Spec: Java 配置框架

## 概述
迁移 ThresholdSet、SimConfig、ConfigParams（`tbsrc/main/include/TradeBotUtils.h`）。

## 需求

### ThresholdSet
~120 个策略阈值参数，保留 C++ 全部默认值：
- 关键阈值：BEGIN_PLACE, LONG_PLACE, SHORT_PLACE, BEGIN_REMOVE, LONG_REMOVE, SHORT_REMOVE
- 仓位参数：SIZE, MAX_SIZE, BEGIN_SIZE, TA_SIZE
- 买卖方向：BID_SIZE, BID_MAX_SIZE, ASK_SIZE, ASK_MAX_SIZE
- 风控参数：MAX_OS_ORDER, UPNL_LOSS, STOP_LOSS, MAX_LOSS, PT_LOSS, PT_PROFIT
- EWA 参数：SPREAD_EWA, ALPHA, DECAY
- 追单参数：AVG_SPREAD_AWAY, SLOP
- 时间参数：SQROFF_TIME, SQROFF_AGG, PAUSE, CANCELREQ_PAUSE
- 所有默认值与 C++ ThresholdSet 构造函数完全一致

### SimConfig
每策略配置容器：
- `instrument` — 主合约 Instrument
- `instrumentSec` — 第二腿合约（套利策略用）
- `thresholdSet` — 阈值参数集
- `strategyID` — 策略 ID
- `executionStrategy` — 关联的策略实例
- `dateConfig` — 日期配置
- `controlConfig` — 控制配置
- `useArbStrat` — 是否套利策略

### ConfigParams（单例）
全局配置管理：
- `strategyID` — 当前策略 ID
- `simConfigMap` — `Map<Integer, List<SimConfig>>` symbolID → SimConfig 列表
- `orderIDStrategyMap` — `Map<Integer, ExecutionStrategy>` orderID → 策略映射
- `simConfig` — 当前活跃 SimConfig
- `strategyCount` — 策略数量
- `modeType` — 运行模式（Sim/Live）

### C++ 对照
- ThresholdSet: `tbsrc/main/include/TradeBotUtils.h:237-504`
- SimConfig: `tbsrc/main/include/TradeBotUtils.h:707-747`
- ConfigParams: `tbsrc/main/include/TradeBotUtils.h:615-705`
