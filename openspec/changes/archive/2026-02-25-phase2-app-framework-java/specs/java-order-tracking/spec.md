# Spec: Java 订单状态追踪

## 概述
迁移 C++ `OrderStats`、`OrderMap`、`PriceMap`（`tbsrc/Strategies/include/ExecutionStrategyStructs.h`）。

## 需求

### OrderStats
保留 C++ 全部字段：
- `active`, `isNew`, `modifyWait`, `cancel` — 布尔标志
- `modifyCount` — 修改计数
- `lastTS` — 最后时间戳
- `orderID` — 订单 ID (int)
- `oldQty`, `newQty`, `qty`, `openQty`, `cxlQty`, `doneQty` — 数量跟踪
- `quantAhead`, `quantBehind` — 队列前后量
- `price`, `newPrice`, `oldPrice` — 价格跟踪
- `typeOfOrder` — QUOTE 类型
- `hitType` — STANDARD/IMPROVE/CROSS/DETECT/MATCH
- `status` — NEW_ORDER/.../TRADED/INIT
- `side` — BUY/SELL

### 枚举类型
- `OrderStats.Status` — 11 种状态（对应 C++ OrderStatus）
- `OrderStats.HitType` — 5 种类型（对应 C++ OrderHitType）

### 容器类型别名
- `OrderMap` = `Map<Integer, OrderStats>` — 按 orderID
- `PriceMap` = `TreeMap<Double, OrderStats>` — 按价格排序

### C++ 对照
- 迁移自: `tbsrc/Strategies/include/ExecutionStrategyStructs.h`
- `OrderStatus` 枚举: 11 values, `OrderHitType` 枚举: 5 values
