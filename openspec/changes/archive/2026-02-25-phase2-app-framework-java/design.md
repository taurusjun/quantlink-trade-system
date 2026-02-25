# Phase 2 设计: 应用框架层

## 架构决策

### 1. 类继承 vs 组合

C++ 中 ExecutionStrategy 使用继承体系。Java 中保持相同的继承结构：
- `ExecutionStrategy` (abstract base)
- `ExtraStrategy extends ExecutionStrategy`
- `PairwiseArbStrategy extends ExecutionStrategy`

本 Phase 仅创建框架类，策略类在 Phase 3 实现。

### 2. CommonClient 回调机制

C++ 使用函数指针回调（`MDcb`, `ORScb`）。Java 使用 `@FunctionalInterface`：
- `MDCallback`: `Consumer<MemorySegment>` — 行情回调
- `ORSCallback`: `Consumer<MemorySegment>` — 回报回调

CommonClient 持有 Connector 引用，通过 `Connector.setMDListener()` / `setORSListener()` 注册。

### 3. Instrument — 纯 Java 类

C++ Instrument 有大量交易所特定逻辑（CME/ICE/China 等）。Java 版仅保留中国期货相关核心：
- 20 档订单簿（double[] bidPx/askPx/bidQty/askQty）
- 价格计算（MID, MSW, LTP）
- 行情更新（从 MarketUpdateNew MemorySegment 填充）
- 合约属性（tickSize, lotSize, symbol, exchange）

### 4. 配置框架

C++ ConfigParams 是全局单例持有 `m_simConfigList[100]` (symbolID→SimConfigList)、`m_orderIDStrategyMap`。
Java 版保持相同设计：
- `ConfigParams` — 单例，持有策略配置映射
- `SimConfig` — 每策略配置容器
- `ThresholdSet` — 阈值参数集（~120 参数，C++ 默认值完全保留）

### 5. 文件组织

```
src/main/java/com/quantlink/trader/
├── shm/           (Phase 1 — 已完成)
├── connector/     (Phase 1 — 已完成)
├── core/          (Phase 2 — 本次新增)
│   ├── Instrument.java
│   ├── OrderStats.java
│   ├── CommonClient.java
│   ├── ThresholdSet.java
│   ├── SimConfig.java
│   └── ConfigParams.java
└── strategy/      (Phase 3 — 待实现)
```

## C++ → Java 映射

| C++ | Java | 说明 |
|-----|------|------|
| `Instrument` | `Instrument` | 20 档订单簿 + 价格计算 |
| `OrderStats` | `OrderStats` | 订单生命周期状态 |
| `OrderMap (map<uint32_t, OrderStats*>)` | `Map<Integer, OrderStats>` | 按 OrderID 索引 |
| `PriceMap (map<double, OrderStats*>)` | `TreeMap<Double, OrderStats>` | 按价格排序 |
| `ThresholdSet` | `ThresholdSet` | 阈值参数集 |
| `SimConfig` | `SimConfig` | 每策略配置 |
| `ConfigParams` | `ConfigParams` | 全局单例 |
| `CommonClient` | `CommonClient` | 回调分发中枢 |
| `OrderStatus enum` | `OrderStats.Status enum` | 内部枚举 |
| `OrderHitType enum` | `OrderStats.HitType enum` | 内部枚举 |

## 依赖关系

```
Connector (Phase 1) ← CommonClient → ExecutionStrategy (Phase 3)
                           ↓
                     ConfigParams → SimConfig → ThresholdSet
                                              → Instrument
                                              → OrderStats
```
