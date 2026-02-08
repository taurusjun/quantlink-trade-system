# PairwiseArbStrategy 新旧系统代码级对比分析

**文档日期**: 2026-01-31
**作者**: QuantLink Team
**版本**: v1.0
**相关模块**: strategy

---

## 概述

本文档对 QuantLink 交易系统中的 **PairwiseArbStrategy（配对套利策略）** 进行新旧系统的代码级对比分析，包括：
- 变量一致性对比
- 方法输入输出对比
- 核心逻辑分支对比
- 功能差异分析

**源文件**:
- 旧系统（C++）: `tbsrc/Strategies/PairwiseArbStrategy.cpp` (948行)
- 新系统（Go）: `golang/pkg/strategy/pairwise_arb_strategy.go` (1055行)

---

## 1. 类结构对比

### 1.1 继承关系

| 对比项 | 旧系统 (C++) | 新系统 (Go) |
|--------|--------------|-------------|
| 基类 | `ExecutionStrategy` | `*BaseStrategy` (组合) |
| 腿策略 | 使用 `ExtraStrategy*` 对象 | 内置 leg1/leg2 状态变量 |
| 设计模式 | 继承 + 组合 | 组合 + 接口 |

### 1.2 类定义概览

**旧系统 (C++)**:
```cpp
class PairwiseArbStrategy : public ExecutionStrategy
{
public:
    ExtraStrategy *m_secondStrat;
    ExtraStrategy *m_firstStrat;
    // ...
};
```

**新系统 (Go)**:
```go
type PairwiseArbStrategy struct {
    *BaseStrategy
    // 内置leg状态
    leg1Position int64
    leg2Position int64
    // ...
}
```

**差异分析**:
- ✅ 新系统将腿管理内联化，减少了对象间耦合
- ✅ Go 使用组合而非继承，更灵活
- ⚠️ 旧系统的 `ExtraStrategy` 功能更丰富，包含订单管理等

---

## 2. 成员变量对比

### 2.1 策略参数变量

| 功能 | 旧系统变量 | 新系统变量 | 一致性 |
|------|-----------|-----------|--------|
| 品种1 | `m_firstinstru->m_instrument` | `symbol1` | ✅ 一致 |
| 品种2 | `m_secondinstru->m_instrument` | `symbol2` | ✅ 一致 |
| 入场阈值 | `m_thold_first->BEGIN_PLACE` | `entryZScore` | ⚠️ 不同体系 |
| 出场阈值 | `m_thold_first->BEGIN_REMOVE` | `exitZScore` | ⚠️ 不同体系 |
| 下单量 | `m_thold->SIZE` | `orderSize` | ✅ 一致 |
| 最大持仓 | `m_thold->MAX_SIZE` | `maxPositionSize` | ✅ 一致 |
| 回看周期 | `count` (固定100000) | `lookbackPeriod` | ✅ 新系统可配置 |
| 最小相关性 | N/A | `minCorrelation` | 🆕 新增 |
| 对冲比例 | `m_thold->HEDGE_SIZE_RATIO` | `hedgeRatio` | ✅ 一致 |
| 价差类型 | 固定差值型 | `spreadType` (ratio/difference) | 🆕 新增选项 |
| 协整检验 | N/A | `useCointegration` | 🆕 新增 |

### 2.2 行情数据变量

| 功能 | 旧系统变量 | 新系统变量 | 一致性 |
|------|-----------|-----------|--------|
| 品种1买价 | `i1_bestBid`, `m_firstinstru->bidPx[0]` | `bid1` | ✅ 一致 |
| 品种1卖价 | `i1_bestAsk`, `m_firstinstru->askPx[0]` | `ask1` | ✅ 一致 |
| 品种2买价 | `i2_bestBid`, `m_secondinstru->bidPx[0]` | `bid2` | ✅ 一致 |
| 品种2卖价 | `i2_bestAsk`, `m_secondinstru->askPx[0]` | `ask2` | ✅ 一致 |
| 品种1中间价 | 计算方式: `(bidPx[0]+askPx[0])/2` | `price1` | ✅ 一致 |
| 品种2中间价 | 计算方式: `(bidPx[0]+askPx[0])/2` | `price2` | ✅ 一致 |

### 2.3 价差统计变量

| 功能 | 旧系统变量 | 新系统变量 | 一致性 |
|------|-----------|-----------|--------|
| 当前价差 | `currSpreadRatio` | `spreadAnalyzer.CurrentSpread` | ✅ 一致 |
| 历史均价差 | `avgSpreadRatio` | `spreadAnalyzer.Mean` | ✅ 一致 |
| 原始均价差 | `avgSpreadRatio_ori` | N/A (合并到Mean) | ⚠️ 简化 |
| 标准差 | N/A (隐式) | `spreadAnalyzer.Std` | 🆕 显式计算 |
| Z-Score | 隐式计算 | `spreadAnalyzer.ZScore` | 🆕 显式变量 |
| 相关系数 | N/A | `spreadAnalyzer.Correlation` | 🆕 新增 |

### 2.4 持仓状态变量

| 功能 | 旧系统变量 | 新系统变量 | 一致性 |
|------|-----------|-----------|--------|
| **Leg1 净持仓** | `m_firstStrat->m_netpos_pass` | `leg1Position` | ✅ 一致 |
| **Leg2 净持仓** | `m_secondStrat->m_netpos_agg` | `leg2Position` | ✅ 一致 |
| Leg1 昨仓 | `m_firstStrat->m_netpos_pass_ytd` | N/A | ⚠️ 新系统未区分 |
| Leg1 买入量 | `m_firstStrat->m_buyQty` | `leg1BuyQty` | ✅ 一致 |
| Leg1 卖出量 | `m_firstStrat->m_sellQty` | `leg1SellQty` | ✅ 一致 |
| Leg1 买入均价 | `m_firstStrat->m_buyAvgPrice` | `leg1BuyAvgPrice` | ✅ 一致 |
| Leg1 卖出均价 | `m_firstStrat->m_sellAvgPrice` | `leg1SellAvgPrice` | ✅ 一致 |
| Leg1 累计买入 | `m_firstStrat->m_buyTotalQty` | `leg1BuyTotalQty` | ✅ 一致 |
| Leg1 累计卖出 | `m_firstStrat->m_sellTotalQty` | `leg1SellTotalQty` | ✅ 一致 |
| Leg2 买入量 | `m_secondStrat->m_buyQty` | `leg2BuyQty` | ✅ 一致 |
| Leg2 卖出量 | `m_secondStrat->m_sellQty` | `leg2SellQty` | ✅ 一致 |
| Leg2 买入均价 | `m_secondStrat->m_buyAvgPrice` | `leg2BuyAvgPrice` | ✅ 一致 |
| Leg2 卖出均价 | `m_secondStrat->m_sellAvgPrice` | `leg2SellAvgPrice` | ✅ 一致 |

### 2.5 订单管理变量

| 功能 | 旧系统变量 | 新系统变量 | 一致性 |
|------|-----------|-----------|--------|
| 订单映射 | `m_ordMap1`, `m_ordMap2` | `BaseStrategy.PendingOrders` | ⚠️ 简化 |
| 价格映射(买) | `m_bidMap1`, `m_bidMap2` | N/A | ⚠️ 未实现 |
| 价格映射(卖) | `m_askMap1`, `m_askMap2` | N/A | ⚠️ 未实现 |
| 追单计数 | `m_agg_repeat` | N/A | ⚠️ 未实现 |
| 主动买单数 | `m_secondStrat->buyAggOrder` | N/A | ⚠️ 未实现 |
| 主动卖单数 | `m_secondStrat->sellAggOrder` | N/A | ⚠️ 未实现 |
| 上次主动方向 | `m_secondStrat->last_agg_side` | N/A | ⚠️ 未实现 |
| 上次主动时间 | `m_secondStrat->last_agg_time` | N/A | ⚠️ 未实现 |

### 2.6 阈值管理变量

| 功能 | 旧系统变量 | 新系统变量 | 一致性 |
|------|-----------|-----------|--------|
| 买入阈值 | `m_firstStrat->m_tholdBidPlace` | N/A (使用entryZScore) | ⚠️ 不同体系 |
| 卖出阈值 | `m_firstStrat->m_tholdAskPlace` | N/A (使用entryZScore) | ⚠️ 不同体系 |
| 买入撤单阈值 | `m_firstStrat->m_tholdBidRemove` | N/A (使用exitZScore) | ⚠️ 不同体系 |
| 卖出撤单阈值 | `m_firstStrat->m_tholdAskRemove` | N/A (使用exitZScore) | ⚠️ 不同体系 |
| 买入最大持仓 | `m_firstStrat->m_tholdBidMaxPos` | `maxPositionSize` | ⚠️ 合并 |
| 卖出最大持仓 | `m_firstStrat->m_tholdAskMaxPos` | `maxPositionSize` | ⚠️ 合并 |

---

## 3. 方法签名对比

### 3.1 核心方法映射

| 功能 | 旧系统方法 | 新系统方法 | 一致性 |
|------|-----------|-----------|--------|
| 构造函数 | `PairwiseArbStrategy(CommonClient*, SimConfig*)` | `NewPairwiseArbStrategy(id string)` | ✅ 功能一致 |
| 初始化 | 构造函数内完成 | `Initialize(config *StrategyConfig)` | ✅ 分离更清晰 |
| 行情处理 | `MDCallBack(MarketUpdateNew*)` | `OnMarketData(md *mdpb.MarketDataUpdate)` | ✅ 一致 |
| 订单回调 | `ORSCallBack(ResponseMsg*)` | `OnOrderUpdate(update *orspb.OrderUpdate)` | ✅ 一致 |
| 发送订单 | `SendOrder()` | `generateSignals(md)` | ⚠️ 概念不同 |
| 主动对冲 | `SendAggressiveOrder()` | N/A | ⚠️ 未实现 |
| 设置阈值 | `SetThresholds()` | N/A | ⚠️ 未实现 |
| 平仓处理 | `HandleSquareoff()` | `Stop()` | ⚠️ 简化 |
| 开仓处理 | `HandleSquareON()` | `Start()` | ⚠️ 简化 |
| 启动 | N/A | `Start()` | 🆕 新增 |
| 停止 | N/A | `Stop()` | 🆕 新增 |
| 定时器 | N/A | `OnTimer(now time.Time)` | 🆕 新增 |
| 参数热加载 | N/A | `ApplyParameters(params map[string]interface{})` | 🆕 新增 |
| 持仓初始化 | N/A | `InitializePositions(positions map[string]int64)` | 🆕 新增 |
| 持仓查询 | N/A | `GetPositionsBySymbol()` | 🆕 新增 |
| 持仓持久化 | `SaveMatrix2()` | `SavePositionSnapshot()` | ✅ 功能一致 |
| 持仓恢复 | `LoadMatrix2()` | `LoadPositionSnapshot()` | ✅ 功能一致 |

### 3.2 辅助方法对比

| 功能 | 旧系统方法 | 新系统方法 | 一致性 |
|------|-----------|-----------|--------|
| 计算Leg1持仓 | `HandlePassOrder()` | `updateLeg1Position()` | ✅ 一致 |
| 计算Leg2持仓 | `HandleAggOrder()` | `updateLeg2Position()` | ✅ 一致 |
| 获取买价优化 | `GetBidPrice_first()` | N/A | ⚠️ 未实现 |
| 获取卖价优化 | `GetAskPrice_first()` | N/A | ⚠️ 未实现 |
| 计算挂单净头寸 | `CalcPendingNetposAgg()` | N/A | ⚠️ 未实现 |
| 生成入场信号 | N/A | `generateSpreadSignals()` | 🆕 新增 |
| 生成出场信号 | N/A | `generateExitSignals()` | 🆕 新增 |
| P&L计算 | 分散在各处 | `updatePairwisePNL()` | 🆕 集中化 |
| 价差状态查询 | N/A | `GetSpreadStatus()` | 🆕 新增 |
| 腿信息查询 | N/A | `GetLegsInfo()` | 🆕 新增 |

---

## 4. 核心逻辑对比

### 4.1 价差计算逻辑

**旧系统 (C++)**:
```cpp
// MDCallBack中
currSpreadRatio = ((m_firstStrat->m_instru->bidPx[0] + m_firstStrat->m_instru->askPx[0]) / 2)
                - ((m_secondStrat->m_instru->bidPx[0] + m_secondStrat->m_instru->askPx[0]) / 2);

// 动态均值更新（指数移动平均）
avgSpreadRatio_ori = (1 - m_firstStrat->m_thold->ALPHA) * avgSpreadRatio_ori
                   + m_firstStrat->m_thold->ALPHA * currSpreadRatio;
avgSpreadRatio = avgSpreadRatio_ori + tValue;  // 加上外部调整值
```

**新系统 (Go)**:
```go
// OnMarketData中
midPrice := (md.BidPrice[0] + md.AskPrice[0]) / 2.0
pas.spreadAnalyzer.UpdatePrice1(midPrice, int64(md.Timestamp))
pas.spreadAnalyzer.UpdatePrice2(midPrice, int64(md.Timestamp))

// SpreadAnalyzer内部计算
pas.spreadAnalyzer.CalculateSpread()  // 计算当前价差
pas.spreadAnalyzer.UpdateAll(pas.lookbackPeriod)  // 更新统计量（均值、标准差、Z-Score）
```

**差异分析**:
- ✅ 核心价差计算逻辑一致（中间价差值）
- ⚠️ 均值计算方式不同：旧系统用 EMA，新系统用简单移动平均
- 🆕 新系统额外计算 Z-Score 和相关系数

### 4.2 入场信号生成逻辑

**旧系统 (C++)**:
```cpp
// SendOrder中 - 被动腿(Leg1)挂单逻辑
for (int32_t level = 0; level < m_thold_first->MAX_QUOTE_LEVEL; level++) {
    LongSpreadRatio1 = m_firstinstru->bidPx[level] - m_secondinstru->bidPx[0];
    ShortSpreadRatio1 = m_firstinstru->askPx[level] - m_secondinstru->askPx[0];

    // 卖出条件：价差足够高
    if (ShortSpreadRatio1 > avgSpreadRatio + m_firstStrat->m_tholdAskPlace) {
        // 检查持仓限制
        if (m_firstStrat->m_netpos_pass * -1 < m_firstStrat->m_tholdAskMaxPos) {
            // 发送卖单
            m_firstStrat->SendAskOrder2(m_firstinstru, NEWORDER, level, passive_sellprice1, ordType);
        }
    }

    // 买入条件：价差足够低
    if (LongSpreadRatio1 < avgSpreadRatio - m_firstStrat->m_tholdBidPlace) {
        if (m_firstStrat->m_netpos_pass < m_firstStrat->m_tholdBidMaxPos) {
            m_firstStrat->SendBidOrder2(m_firstinstru, NEWORDER, level, passive_buyprice1, ordType);
        }
    }
}

// 主动对冲腿(Leg2)
if (m_firstStrat->m_netpos_pass + m_secondStrat->m_netpos_agg + pending_netpos_agg2 > 0) {
    // 净多头敞口，Leg2发卖单对冲
    m_secondStrat->SendAskOrder2(m_secondinstru, NEWORDER, 0,
        m_secondinstru->bidPx[0] - m_secondStrat->m_instru->m_tickSize, CROSS, qty);
}
```

**新系统 (Go)**:
```go
// generateSignals中 - Z-Score驱动
func (pas *PairwiseArbStrategy) generateSignals(md *mdpb.MarketDataUpdate) {
    spreadStats := pas.spreadAnalyzer.GetStats()

    // 入场信号：Z-Score超过阈值
    if math.Abs(spreadStats.ZScore) >= pas.entryZScore {
        if spreadStats.ZScore > 0 {
            // 价差偏高 -> 做空价差 (卖Leg1, 买Leg2)
            pas.generateSpreadSignals(md, "short", pas.orderSize)
        } else {
            // 价差偏低 -> 做多价差 (买Leg1, 卖Leg2)
            pas.generateSpreadSignals(md, "long", pas.orderSize)
        }
    }

    // 出场信号：Z-Score回归
    if pas.leg1Position != 0 && math.Abs(spreadStats.ZScore) <= pas.exitZScore {
        pas.generateExitSignals(md)
    }
}

// generateSpreadSignals - 生成双腿信号
func (pas *PairwiseArbStrategy) generateSpreadSignals(md *mdpb.MarketDataUpdate, direction string, qty int64) {
    // 计算对冲数量
    hedgeQty := int64(math.Round(float64(qty) * spreadStats.HedgeRatio))

    // 生成Leg1信号
    signal1 := &TradingSignal{
        Symbol:   pas.symbol1,
        Side:     signal1Side,
        Price:    GetOrderPrice(signal1Side, pas.bid1, pas.ask1, ...),
        Quantity: qty,
    }
    pas.BaseStrategy.AddSignal(signal1)

    // 生成Leg2信号
    signal2 := &TradingSignal{
        Symbol:   pas.symbol2,
        Side:     signal2Side,
        Price:    GetOrderPrice(signal2Side, pas.bid2, pas.ask2, ...),
        Quantity: hedgeQty,
    }
    pas.BaseStrategy.AddSignal(signal2)
}
```

**差异分析**:

| 对比维度 | 旧系统 | 新系统 |
|---------|--------|--------|
| 信号触发 | 价差绝对值 vs 阈值 | Z-Score vs 阈值 |
| 挂单层级 | 支持多层挂单 (MAX_QUOTE_LEVEL) | 仅支持单层 |
| 订单类型 | STANDARD/CROSS/MATCH | 统一信号 |
| 对冲逻辑 | 实时检测敞口并主动对冲 | 同时生成双腿信号 |
| 阈值动态调整 | `SetThresholds()` 根据持仓调整 | 固定阈值 |

### 4.3 阈值动态调整逻辑（旧系统特有）

**旧系统 (C++)**:
```cpp
void PairwiseArbStrategy::SetThresholds() {
    auto long_place_diff_thold = m_thold_first->LONG_PLACE - m_thold_first->BEGIN_PLACE;
    auto short_place_diff_thold = m_thold_first->BEGIN_PLACE - m_thold_first->SHORT_PLACE;

    if (m_firstStrat->m_netpos_pass == 0) {
        // 无持仓：使用初始阈值
        m_firstStrat->m_tholdBidPlace = m_thold_first->BEGIN_PLACE;
        m_firstStrat->m_tholdAskPlace = m_thold_first->BEGIN_PLACE;
    } else if (m_firstStrat->m_netpos_pass > 0) {
        // 多头持仓：买入阈值变严（需要更好价格才买），卖出阈值放宽
        m_firstStrat->m_tholdBidPlace = m_thold_first->BEGIN_PLACE
            + long_place_diff_thold * m_firstStrat->m_netpos_pass / m_firstStrat->m_tholdMaxPos;
        m_firstStrat->m_tholdAskPlace = m_thold_first->BEGIN_PLACE
            - short_place_diff_thold * m_firstStrat->m_netpos_pass / m_firstStrat->m_tholdMaxPos;
    } else {
        // 空头持仓：卖出阈值变严，买入阈值放宽
        m_firstStrat->m_tholdBidPlace = m_thold_first->BEGIN_PLACE
            + short_place_diff_thold * m_firstStrat->m_netpos_pass / m_firstStrat->m_tholdMaxPos;
        m_firstStrat->m_tholdAskPlace = m_thold_first->BEGIN_PLACE
            - long_place_diff_thold * m_firstStrat->m_netpos_pass / m_firstStrat->m_tholdMaxPos;
    }
}
```

**新系统**: ⚠️ **未实现**，使用固定的 `entryZScore` 和 `exitZScore`

### 4.4 主动追单逻辑（旧系统特有）

**旧系统 (C++)**:
```cpp
void PairwiseArbStrategy::SendAggressiveOrder() {
    auto pending_netpos_agg2 = CalcPendingNetposAgg();

    // 多头敞口 -> 主动卖对冲
    if (m_firstStrat->m_netpos_pass + m_secondStrat->m_netpos_agg + pending_netpos_agg2 > 0) {
        if (m_secondStrat->last_agg_side != SELL || now_ts - m_secondStrat->last_agg_time > 500) {
            // 500ms间隔后按市价发单
            m_secondStrat->SendAskOrder2(m_secondinstru, NEWORDER, 0,
                m_secondinstru->bidPx[0], CROSS, qty);
        } else {
            // 追单逻辑：最多3次，每次降价1个tick
            if (m_agg_repeat > 3) {
                // 报警并停止策略
                HandleSquareoff();
            } else {
                double agg_price = m_agg_repeat < 3
                    ? m_secondinstru->bidPx[0] - m_secondStrat->m_instru->m_tickSize * m_agg_repeat
                    : m_secondinstru->bidPx[0] - m_secondStrat->m_instru->m_tickSize * m_secondStrat->m_thold->SLOP;
                m_secondStrat->SendAskOrder2(m_secondinstru, NEWORDER, 0, agg_price, CROSS, qty);
                m_agg_repeat++;
            }
        }
    }
}
```

**新系统**: ⚠️ **未实现**，双腿信号同时生成，无追单机制

### 4.5 持仓更新逻辑

**旧系统 (C++)** - 在 `ExtraStrategy::ORSCallBack` 中处理:
```cpp
// 成交回调更新持仓
if (response->Response_Type == TRADE_CONFIRM) {
    if (order->m_side == BUY) {
        m_netpos_pass += response->Quantity;
        m_buyQty += response->Quantity;
        m_buyTotalQty += response->Quantity;
        m_buyPrice = (m_buyPrice * oldBuyQty + response->Price * response->Quantity) / m_buyQty;
    } else {
        m_netpos_pass -= response->Quantity;
        m_sellQty += response->Quantity;
        m_sellTotalQty += response->Quantity;
        m_sellPrice = (m_sellPrice * oldSellQty + response->Price * response->Quantity) / m_sellQty;
    }
}
```

**新系统 (Go)**:
```go
func (pas *PairwiseArbStrategy) updateLeg1Position(side orspb.OrderSide, qty int64, price float64) {
    if side == orspb.OrderSide_BUY {
        pas.leg1BuyTotalQty += qty
        pas.leg1BuyTotalValue += float64(qty) * price

        // 检查是否有空头需要平仓
        if pas.leg1Position < 0 {
            closedQty := min(qty, pas.leg1SellQty)
            pas.leg1SellQty -= closedQty
            pas.leg1Position += closedQty
            qty -= closedQty
        }

        // 开多
        if qty > 0 {
            totalCost := pas.leg1BuyAvgPrice * float64(pas.leg1BuyQty)
            totalCost += price * float64(qty)
            pas.leg1BuyQty += qty
            pas.leg1Position += qty
            pas.leg1BuyAvgPrice = totalCost / float64(pas.leg1BuyQty)
        }
    }
    // ... 卖出逻辑类似
}
```

**一致性**: ✅ 核心持仓计算逻辑一致

### 4.6 P&L 计算逻辑

**旧系统 (C++)** - 分散在各处:
```cpp
// 在 ORSCallBack 中
double arbi_unrealisedPNL = m_firstStrat->m_unrealisedPNL + m_secondStrat->m_unrealisedPNL;
double arbi_realisedPNL = (m_firstStrat->m_realisedPNL - m_firstStrat->m_transTotalValue)
                        + (m_secondStrat->m_realisedPNL - m_secondStrat->m_transTotalValue);
double arbi_grossPNL = m_firstStrat->m_grossPNL + m_secondStrat->m_grossPNL;
double arbi_netPNL = m_firstStrat->m_netPNL + m_secondStrat->m_netPNL;
```

**新系统 (Go)**:
```go
func (pas *PairwiseArbStrategy) updatePairwisePNL() {
    var unrealizedPnL float64 = 0

    // Leg1 浮动盈亏
    if pas.leg1Position != 0 {
        if pas.leg1Position > 0 {
            // 多头: 用bid价计算
            leg1PnL = (pas.bid1 - pas.leg1BuyAvgPrice) * float64(pas.leg1Position)
        } else {
            // 空头: 用ask价计算
            leg1PnL = (pas.leg1SellAvgPrice - pas.ask1) * float64(-pas.leg1Position)
        }
        unrealizedPnL += leg1PnL
    }

    // Leg2 浮动盈亏 (类似)
    // ...

    pas.PNL.UnrealizedPnL = unrealizedPnL
    pas.PNL.TotalPnL = pas.PNL.RealizedPnL + pas.PNL.UnrealizedPnL
}
```

**一致性**: ✅ P&L 计算逻辑一致（两腿分别计算后求和）

### 4.7 持久化逻辑

**旧系统 (C++)** - 文本文件格式:
```cpp
void PairwiseArbStrategy::SaveMatrix2(std::string filepath) {
    // 文件格式: StrategyID 2day avgPx m_origbaseName1 m_origbaseName2 ytd1 ytd2
    out << m_strategyID << " " << "0 " << avgSpreadRatio_ori << " "
        << m_firstStrat->m_instru->m_origbaseName << " "
        << m_secondStrat->m_instru->m_origbaseName << " "
        << m_firstStrat->m_netpos_pass << " "
        << m_secondStrat->m_netpos_agg << endl;
}

std::map<int32_t, std::map<std::string, std::string>> LoadMatrix2(std::string filepath) {
    // 解析文本文件
}
```

**新系统 (Go)** - JSON格式:
```go
type PositionSnapshot struct {
    StrategyID    string            `json:"strategy_id"`
    Timestamp     time.Time         `json:"timestamp"`
    TotalLongQty  int64             `json:"total_long_qty"`
    TotalShortQty int64             `json:"total_short_qty"`
    TotalNetQty   int64             `json:"total_net_qty"`
    AvgLongPrice  float64           `json:"avg_long_price"`
    AvgShortPrice float64           `json:"avg_short_price"`
    RealizedPnL   float64           `json:"realized_pnl"`
    SymbolsPos    map[string]int64  `json:"symbols_pos"`  // 🆕 支持多品种
}

func SavePositionSnapshot(snapshot PositionSnapshot) error {
    // JSON 序列化并写入文件
}

func LoadPositionSnapshot(strategyID string) (*PositionSnapshot, error) {
    // JSON 反序列化
}
```

**差异分析**:
- ✅ 功能一致：保存和恢复持仓状态
- 🆕 新系统使用 JSON 格式，更易读、更灵活
- 🆕 新系统支持时间戳、已实现盈亏等额外字段

---

## 5. 功能完整性对比

### 5.1 已实现功能

| 功能 | 旧系统 | 新系统 | 状态 |
|------|--------|--------|------|
| 价差计算 | ✅ | ✅ | ✅ 一致 |
| Z-Score 计算 | 隐式 | ✅ 显式 | ✅ 改进 |
| 入场信号生成 | ✅ | ✅ | ⚠️ 逻辑不同 |
| 出场信号生成 | ✅ | ✅ | ⚠️ 逻辑不同 |
| 双腿持仓管理 | ✅ | ✅ | ✅ 一致 |
| 持仓持久化 | ✅ | ✅ | ✅ 一致 |
| P&L 计算 | ✅ | ✅ | ✅ 一致 |
| 参数热加载 | ❌ | ✅ | 🆕 新增 |
| 相关性检查 | ❌ | ✅ | 🆕 新增 |
| 协整检验 | ❌ | ✅ (预留) | 🆕 新增 |
| 风险度量 | ✅ | ✅ | ✅ 一致 |

### 5.2 未实现功能

| 功能 | 旧系统实现 | 新系统状态 | 优先级 |
|------|-----------|-----------|--------|
| 动态阈值调整 | `SetThresholds()` | ❌ 未实现 | 🔴 高 |
| 主动追单机制 | `SendAggressiveOrder()` | ❌ 未实现 | 🔴 高 |
| 多层挂单 | `MAX_QUOTE_LEVEL` | ❌ 未实现 | 🟡 中 |
| 订单类型区分 | STANDARD/CROSS/MATCH | ❌ 未实现 | 🟡 中 |
| 价格优化 | `GetBidPrice_first()` 等 | ❌ 未实现 | 🟡 中 |
| 挂单队列管理 | `m_bidMap`, `m_askMap` | ❌ 未实现 | 🟡 中 |
| 流控保护 | `last_agg_time` 间隔检查 | ⚠️ 部分实现 | 🟡 中 |
| 昨/今仓区分 | `m_netpos_pass_ytd` | ❌ 未实现 | 🟢 低 |
| 外部 tValue 调整 | `tValue` from `m_tvar` | ❌ 未实现 | 🟢 低 |
| 最大亏损保护 | `m_maxloss_limit` | ⚠️ 在风控模块 | 🟢 低 |

---

## 6. 总结

### 6.1 一致性评估

| 类别 | 一致性 | 说明 |
|------|--------|------|
| **数据结构** | 85% | 核心变量一致，部分订单管理变量缺失 |
| **核心算法** | 70% | 价差/持仓计算一致，信号生成逻辑不同 |
| **功能完整度** | 65% | 主要功能已实现，高级功能（追单、动态阈值）缺失 |
| **接口设计** | 90% | 新系统接口更清晰、更现代化 |

### 6.2 新系统优势

1. **代码组织更清晰**: SpreadAnalyzer 封装价差统计逻辑
2. **参数热加载**: 支持运行时修改参数
3. **相关性检查**: 自动过滤低相关性行情
4. **持久化改进**: JSON 格式更灵活
5. **接口丰富**: 提供更多查询接口（GetSpreadStatus, GetLegsInfo）

### 6.3 待改进项

1. **动态阈值调整**: 应参考旧系统 `SetThresholds()` 实现
2. **主动追单机制**: 对冲腿应支持主动追单
3. **多层挂单**: 支持在多个价位挂单
4. **订单类型区分**: 区分被动单和主动对冲单
5. **价格优化**: 实现隐性订单簿检测逻辑

### 6.4 建议实施路径

1. **Phase 1**: 实现动态阈值调整（参考 `SetThresholds`）
2. **Phase 2**: 实现主动追单机制（参考 `SendAggressiveOrder`）
3. **Phase 3**: 添加多层挂单支持
4. **Phase 4**: 实现订单类型区分和价格优化

---

## 参考资料

- 旧系统源码: `tbsrc/Strategies/PairwiseArbStrategy.cpp`
- 新系统源码: `golang/pkg/strategy/pairwise_arb_strategy.go`
- 价差分析器: `golang/pkg/strategy/spread/spread_analyzer.go`
- 系统架构: `docs/核心文档/CURRENT_ARCHITECTURE_FLOW.md`

---

**最后更新**: 2026-01-31 21:00
