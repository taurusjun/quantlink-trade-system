# Phase 3 设计: 策略层

## 架构决策

### 继承 vs 组合
C++ 使用继承: `PairwiseArbStrategy -> ExecutionStrategy`, `ExtraStrategy -> ExecutionStrategy`。
Java 同样使用继承，保持与 C++ 一致。

### 类层次
```
ExecutionStrategy (abstract)
├── ExtraStrategy (Instrument 参数化订单)
└── PairwiseArbStrategy (双腿套利，组合两个 ExtraStrategy)
```

### 关键设计

1. **ExecutionStrategy** 是 abstract class（`SendOrder()` 为 pure virtual → Java abstract）
2. **OrderMap/PriceMap** 使用 `Map<Integer, OrderStats>` / `Map<Double, OrderStats>`
3. **C++ TransactionType** 映射到 `Constants.SIDE_BUY/SELL` (byte)
4. **memlog/LOGGER** 相关代码不迁移（监控日志用 Java logging 替代）
5. **Option/VolParams** 相关代码不迁移（期权策略不在本次范围）
6. **SelfBook/SelfTrade** 相关代码不迁移（快照相关功能暂不需要）

### Phase 2 类型细化
- `ConfigParams.orderIDStrategyMap` 从 `Map<Integer, Object>` 改为 `Map<Integer, ExecutionStrategy>`
- `SimConfig.executionStrategy` 从 `Object` 改为 `ExecutionStrategy`

### 简化范围
Phase 3 聚焦核心策略逻辑：
- ✅ 构造函数、Reset、SetThresholds/SetLinearThresholds
- ✅ ORSCallBack（12种响应处理）、ProcessTrade、ProcessCancelConfirm 等
- ✅ SendNewOrder、SendModifyOrder、SendCancelOrder
- ✅ CalculatePNL、CheckSquareoff、HandleSquareoff
- ✅ PairwiseArbStrategy: SendOrder、SendAggressiveOrder、MDCallBack、ORSCallBack
- ✅ ExtraStrategy: Instrument 参数化订单方法
- ✅ daily_init 文件加载
- ❌ memlog SHM 监控（用 Java log 替代）
- ❌ 期权相关（OptionManager、VolParams）
- ❌ SelfBook/SelfTrade 处理
- ❌ LOGGER 序列化（TBRequestMsg/TBResponseMsg）
