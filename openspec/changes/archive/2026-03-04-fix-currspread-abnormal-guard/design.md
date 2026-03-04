## Context

PairwiseArbStrategy 在每次行情回调中计算 currSpreadRatio = midPx(leg1) - midPx(leg2)。C++ 原代码的守卫条件为：

```cpp
if (firstBid <= 0 || firstAsk <= 0 || secondBid <= 0 && secondAsk <= 0)
```

由于 `&&` 优先级高于 `||`，second leg 只有在 bid AND ask 同时 <= 0 时才跳过。当 secondBid=0 但 secondAsk>0 时，midPx = (0+askPx)/2 = askPx/2，导致 spread 偏差达万级。

实盘日志验证：20260303 夜盘和 20260304 日盘多次出现 currSpread=-10000~-20000。

## Goals / Non-Goals

**Goals:**
- 防止任一腿单边价格为 0 时计算出极端 spread 值
- 避免 AVG_SPREAD_AWAY 因异常 spread 误触发
- 避免 avgSpreadRatio EWA 被极端值污染

**Non-Goals:**
- 不修改 AVG_SPREAD_AWAY 检查逻辑本身
- 不修改 EWA 更新逻辑
- 不修改下单、成交等其他功能

## Decisions

**决策 1: 将守卫条件从 `secondBid <= 0 && secondAsk <= 0` 改为 `secondBid <= 0 || secondAsk <= 0`**

- 方案 A（选定）: 四个价格全部 > 0 才计算 spread → 简单直接，彻底杜绝单边为 0 的问题
- 方案 B: 在 spread 计算后加范围检查（如 abs(spread) > 10000 则丢弃）→ 需要选择阈值，不够通用
- 方案 C: 保持 C++ 原行为不改 → 实盘已证明有问题

选择方案 A，因为价格为 0 本身就是无效行情，不应参与任何计算。

## Risks / Trade-offs

- [与 C++ 不一致] 这是对 C++ 原代码行为的改进，需在注释中标注差异 → 按 CLAUDE.md 规则加 `[C++差异]` 注释
- [可能延迟 spread 更新] 当某腿行情暂时缺失时 spread 保持上一次值 → 可接受，比用错误值计算更安全
