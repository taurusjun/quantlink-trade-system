## Why

实盘运行中频繁观测到 currSpreadRatio 出现 -10000~-20000 的极端异常值。根因是 C++ 原代码的价差守卫条件 `secondBid <= 0 && secondAsk <= 0` 允许某一腿单边价格为 0（如 bidPx=0, askPx>0）时仍计算 midPx，导致 spread 偏差达万级。这会造成：
1. AVG_SPREAD_AWAY 误触发，导致策略意外退出
2. avgSpreadRatio EWA 被极端值污染，后续均值偏移

## What Changes

- 加强 currSpreadRatio 计算前的行情有效性守卫：当任一腿的 bidPx 或 askPx 为 0 时跳过 spread 计算
- 这是对 C++ 原代码的改进（C++ 原代码允许 second leg 单边为 0 通过），需加注释说明差异

## Capabilities

### New Capabilities
- `spread-price-guard`: 价差计算行情有效性防护 — 确保两腿四个价格（firstBid, firstAsk, secondBid, secondAsk）全部 > 0 时才计算 currSpreadRatio

### Modified Capabilities

（无已有 spec 需要修改）

## Impact

- `tbsrc-java/src/main/java/com/quantlink/trader/strategy/PairwiseArbStrategy.java` L633-634 的守卫条件
- 仅影响 spread 计算守卫逻辑，不影响下单、成交、持仓等其他功能
- 不涉及配置文件变更
