# PairwiseArbStrategy 规格

## 概述
迁移自 `tbsrc/Strategies/include/PairwiseArbStrategy.h` + `PairwiseArbStrategy.cpp`。
双腿配对套利策略，继承 ExecutionStrategy。

## 构造函数
- 创建 m_firstStrat, m_secondStrat (ExtraStrategy)
- m_secondStrat.instru = instru_sec
- 加载 daily_init 文件（avgPx, ytd1, 2day, ytd2）
- 初始化 m_ordMap1 = &m_firstStrat.ordMap, m_ordMap2 = &m_secondStrat.ordMap

## 核心方法
1. **sendOrder()** — 被动挂单逻辑：
   - 撤销 firstStrat/secondStrat 中的 CROSS/MATCH 订单
   - 检查并撤销价差超出范围的订单
   - 多层被动挂单（MAX_QUOTE_LEVEL层）
   - 对冲检查：netpos_pass + netpos_agg 不平衡时发对冲单
2. **sendAggressiveOrder()** — 主动追单（敞口时重复追单）
3. **setThresholds()** — 覆盖基类，设置 firstStrat/secondStrat 的阈值
4. **orsCallBack()** — 路由到 firstStrat 或 secondStrat
5. **mdCallBack()** — 转发到两腿，计算价差，检查有效性
6. **handleSquareoff()** — 双腿平仓
7. **loadMatrix2()** — 加载 daily_init 文件
8. **saveMatrix2()** — 保存 daily_init 文件
9. **handlePassOrder/handleAggOrder** — 被动/主动订单成交处理
10. **calcPendingNetposAgg()** — 计算挂起的净仓位

## 关键字段
- firstStrat, secondStrat (ExtraStrategy)
- firstinstru, secondinstru (Instrument)
- avgSpreadRatio_ori, avgSpreadRatio, currSpreadRatio
- thold_first, thold_second (ThresholdSet)
- bidMap1/2, askMap1/2 (PriceMap)
- ordMap1, ordMap2 (指向 firstStrat/secondStrat 的 ordMap)
- tValue, netpos_agg1, netpos_agg2, agg_repeat
