# ORS Gateway Offset 自动设置方案

**文档日期**: 2026-01-30
**作者**: QuantLink Team
**版本**: v1.0
**相关模块**: ORS Gateway, Strategy Layer

---

## 1. 问题背景

### 1.1 当前实现（临时方案）

**策略层设置 OpenClose**：

```go
// pairwise_arb_strategy.go
func (pas *PairwiseArbStrategy) generateEntrySignals(...) {
    var leg1OpenClose OpenClose
    if signal1Side == OrderSideBuy {
        if pas.leg1Position < 0 {
            leg1OpenClose = OpenCloseClose  // 平空
        } else {
            leg1OpenClose = OpenCloseOpen   // 开多
        }
    } else {
        if pas.leg1Position > 0 {
            leg1OpenClose = OpenCloseClose  // 平多
        } else {
            leg1OpenClose = OpenCloseOpen   // 开空
        }
    }

    signal1 := &TradingSignal{
        Side:      signal1Side,
        OpenClose: leg1OpenClose,  // ✅ 策略层设置
    }
}
```

**问题**：
- ❌ 每个策略都需要实现相同的判断逻辑
- ❌ 策略层需要维护持仓状态
- ❌ 无法统一处理今昨仓区分
- ❌ 无法自动处理部分平仓

### 1.2 为什么需要改进

中国期货市场的特殊规则：
1. **必须明确指定开平标志**：CTP 要求订单必须有 `CombOffsetFlag`
2. **今昨仓区分**：上期所（SHFE）需要区分平今/平昨
3. **部分平仓**：订单数量可能大于持仓，需要拆分为平仓+开仓
4. **多策略共享**：多个策略操作同一合约时，需要统一的持仓视图

---

## 2. 三种实现方案对比

### 2.1 方案A：策略层设置（当前实现）

**架构**：
```
策略层 → 判断 OpenClose → 设置 signal.OpenClose → ORS Gateway → CTP
```

**优点**：
- ✅ 策略完全控制开平行为
- ✅ 不需要 Gateway 查询持仓
- ✅ 逻辑清晰，易于调试

**缺点**：
- ❌ 每个策略需要实现判断逻辑（代码重复）
- ❌ 策略层需要维护持仓状态
- ❌ 无法统一处理今昨仓区分
- ❌ 无法自动处理部分平仓

### 2.2 方案B：ORS Gateway 层设置（长期方案）

**架构**：
```
策略层 → 只传 side/price/qty → ORS Gateway → SetOpenClose() → CTP
                                      ↓
                              mapPositions (持仓查询)
```

**优点**：
- ✅ **策略层代码极简**：只需传 side, price, quantity
- ✅ **统一处理**：所有策略共享持仓逻辑
- ✅ **支持今昨仓区分**：Gateway 维护详细持仓
- ✅ **支持部分平仓**：自动拆分订单
- ✅ **多策略协同**：统一的持仓视图
- ✅ **与 ors/China 一致**：参考成熟实现

**缺点**：
- ⚠️ Gateway 需要维护持仓状态（增加复杂度）
- ⚠️ 需要线程安全（使用 mutex）
- ⚠️ 初次实施工作量较大

### 2.3 方案C：Connector 中间层设置

类似 tbsrc 的 illuminati::Connector 层处理，需要引入新的中间层。

**不采用原因**：
- ❌ 增加系统复杂度
- ❌ 需要开发新的中间层
- ❌ 不如直接在 Gateway 层处理

---

## 3. 选择方案B的理由

### 3.1 参考 ors/China 的成熟实现

ors/China 项目是一个**成熟的 CTP 交易系统**，已在生产环境验证：

```cpp
// ORSServer.cpp:488
void ORSServer::SetCombOffsetFlag(CThostFtdcInputOrderField &req,
                                  RequestMsg *request,
                                  char &flag, int &orderType) {
    // 查询持仓
    mapContractPosIter = mapContractPos.find(std::string(req.InstrumentID));

    switch (request->Transaction_Type) {
    case BUY:
        if (request->Quantity <= mapContractPosIter->second.todayShortPos) {
            flag = THOST_FTDC_OF_CloseToday;  // 平今空仓
            mapContractPosIter->second.todayShortPos -= request->Quantity;
        }
        else if (request->Quantity <= mapContractPosIter->second.ONShortPos) {
            flag = THOST_FTDC_OF_Close;  // 平昨空仓
            mapContractPosIter->second.ONShortPos -= request->Quantity;
        }
        else {
            flag = THOST_FTDC_OF_Open;  // 开多仓
        }
        break;
    case SELL:
        // 同理...
        break;
    }
}
```

### 3.2 架构优势

**关注点分离**：
- **策略层**：只关心交易逻辑（何时买卖、价格、数量）
- **Gateway 层**：处理市场规则（开平标志、今昨仓、拆单）

**复用性**：
- 所有策略自动获得今昨仓支持
- 所有策略自动获得部分平仓支持
- 新增策略无需重复实现 offset 判断

**正确性**：
- 统一的持仓视图，避免多策略冲突
- 参考成熟实现，减少错误

### 3.3 长期价值

1. **可扩展性**：支持更多策略
2. **可维护性**：offset 逻辑集中维护
3. **生产就绪**：符合真实交易系统架构

---

## 4. 实施方案设计

### 4.1 持仓数据结构

```go
// pkg/gateway/position_manager.go

// Position 持仓信息（与 ors/China 一致）
type Position struct {
    Symbol          string  // 合约代码
    Exchange        string  // 交易所

    // 多头持仓
    LongPosition    int64   // 多头总持仓
    TodayLongPos    int64   // 今日多头持仓
    YesterdayLongPos int64  // 昨日多头持仓
    LongAvgPrice    float64 // 多头均价

    // 空头持仓
    ShortPosition   int64   // 空头总持仓
    TodayShortPos   int64   // 今日空头持仓
    YesterdayShortPos int64 // 昨日空头持仓
    ShortAvgPrice   float64 // 空头均价

    // 冻结持仓（挂单占用）
    LongFrozen      int64   // 多头冻结
    ShortFrozen     int64   // 空头冻结

    LastUpdate      time.Time
}

// PositionManager 持仓管理器
type PositionManager struct {
    positions map[string]*Position  // key: symbol
    mutex     sync.RWMutex
}
```

### 4.2 SetOpenClose 方法

```go
// pkg/gateway/position_manager.go

func (pm *PositionManager) SetOpenClose(req *orspb.OrderRequest) error {
    pm.mutex.Lock()
    defer pm.mutex.Unlock()

    pos := pm.getOrCreatePosition(req.Symbol, req.Exchange)

    switch req.Side {
    case orspb.OrderSide_BUY:
        // 买入：先平空，再开多
        if req.Quantity <= pos.TodayShortPos {
            // 平今空仓（上期所需要）
            if req.Exchange == commonpb.Exchange_SHFE {
                req.OpenClose = orspb.OpenClose_CLOSE_TODAY
            } else {
                req.OpenClose = orspb.OpenClose_CLOSE
            }
            pos.TodayShortPos -= req.Quantity
        } else if req.Quantity <= pos.ShortPosition {
            // 平昨空仓
            req.OpenClose = orspb.OpenClose_CLOSE
            pos.YesterdayShortPos -= req.Quantity
        } else {
            // 开多仓
            req.OpenClose = orspb.OpenClose_OPEN
        }

    case orspb.OrderSide_SELL:
        // 卖出：先平多，再开空
        if req.Quantity <= pos.TodayLongPos {
            // 平今多仓
            if req.Exchange == commonpb.Exchange_SHFE {
                req.OpenClose = orspb.OpenClose_CLOSE_TODAY
            } else {
                req.OpenClose = orspb.OpenClose_CLOSE
            }
            pos.TodayLongPos -= req.Quantity
        } else if req.Quantity <= pos.LongPosition {
            // 平昨多仓
            req.OpenClose = orspb.OpenClose_CLOSE
            pos.YesterdayLongPos -= req.Quantity
        } else {
            // 开空仓
            req.OpenClose = orspb.OpenClose_OPEN
        }
    }

    return nil
}
```

### 4.3 持仓更新（接收成交回报）

```go
// UpdatePositionFromTrade 根据成交回报更新持仓
func (pm *PositionManager) UpdatePositionFromTrade(trade *orspb.TradeResponse) {
    pm.mutex.Lock()
    defer pm.mutex.Unlock()

    pos := pm.getOrCreatePosition(trade.Symbol, trade.Exchange)

    switch trade.OpenClose {
    case orspb.OpenClose_OPEN:
        if trade.Side == orspb.OrderSide_BUY {
            // 开多
            pos.LongPosition += trade.Volume
            pos.TodayLongPos += trade.Volume
            // 更新均价
            pos.LongAvgPrice = (pos.LongAvgPrice*float64(pos.LongPosition-trade.Volume) +
                               trade.Price*float64(trade.Volume)) / float64(pos.LongPosition)
        } else {
            // 开空
            pos.ShortPosition += trade.Volume
            pos.TodayShortPos += trade.Volume
            pos.ShortAvgPrice = (pos.ShortAvgPrice*float64(pos.ShortPosition-trade.Volume) +
                                trade.Price*float64(trade.Volume)) / float64(pos.ShortPosition)
        }

    case orspb.OpenClose_CLOSE, orspb.OpenClose_CLOSE_TODAY, orspb.OpenClose_CLOSE_YESTERDAY:
        if trade.Side == orspb.OrderSide_BUY {
            // 平空
            pos.ShortPosition -= trade.Volume
            if trade.OpenClose == orspb.OpenClose_CLOSE_TODAY {
                pos.TodayShortPos -= trade.Volume
            } else {
                pos.YesterdayShortPos -= trade.Volume
            }
        } else {
            // 平多
            pos.LongPosition -= trade.Volume
            if trade.OpenClose == orspb.OpenClose_CLOSE_TODAY {
                pos.TodayLongPos -= trade.Volume
            } else {
                pos.YesterdayLongPos -= trade.Volume
            }
        }
    }

    pos.LastUpdate = time.Now()
}
```

### 4.4 集成到 ORS Gateway

```go
// pkg/gateway/ors_gateway.go

type ORSGateway struct {
    // ... 现有字段
    positionMgr *PositionManager  // 新增：持仓管理器
}

func (gw *ORSGateway) handleOrderRequest(ctx context.Context, req *orspb.OrderRequest) {
    // 1. 如果 OpenClose 未设置或为 UNKNOWN，自动判断
    if req.OpenClose == orspb.OpenClose_OC_UNKNOWN {
        if err := gw.positionMgr.SetOpenClose(req); err != nil {
            log.Printf("Failed to set open_close: %v", err)
            return
        }
    }

    // 2. 发送订单到 Counter Bridge (现有逻辑)
    // ...
}

func (gw *ORSGateway) handleTradeResponse(ctx context.Context, trade *orspb.TradeResponse) {
    // 1. 更新持仓
    gw.positionMgr.UpdatePositionFromTrade(trade)

    // 2. 转发给策略层 (现有逻辑)
    // ...
}
```

### 4.5 策略层简化

**修改前**：
```go
func (pas *PairwiseArbStrategy) generateEntrySignals(...) {
    var leg1OpenClose OpenClose
    if signal1Side == OrderSideBuy {
        if pas.leg1Position < 0 {
            leg1OpenClose = OpenCloseClose
        } else {
            leg1OpenClose = OpenCloseOpen
        }
    } else {
        if pas.leg1Position > 0 {
            leg1OpenClose = OpenCloseClose
        } else {
            leg1OpenClose = OpenCloseOpen
        }
    }

    signal1 := &TradingSignal{
        Side:      signal1Side,
        OpenClose: leg1OpenClose,  // ← 需要设置
    }
}
```

**修改后**：
```go
func (pas *PairwiseArbStrategy) generateEntrySignals(...) {
    signal1 := &TradingSignal{
        Symbol:    pas.leg1Symbol,
        Exchange:  pas.leg1Exchange,
        Side:      signal1Side,
        // OpenClose: 不需要设置，Gateway 自动判断
        Price:     signal1Price,
        Quantity:  signal1Qty,
    }
}
```

---

## 5. 实施步骤

### Phase 1: 创建持仓管理器（不影响现有功能）

**文件**：`golang/pkg/gateway/position_manager.go`

**任务**：
1. 定义 `Position` 结构体
2. 实现 `PositionManager` 类
3. 实现 `SetOpenClose()` 方法
4. 实现 `UpdatePositionFromTrade()` 方法
5. 实现 `GetPosition()` 查询方法
6. 单元测试

**验证**：
- ✅ 单元测试通过
- ✅ 编译成功

### Phase 2: 集成到 ORS Gateway

**文件**：`golang/pkg/gateway/ors_gateway.go`

**任务**：
1. 在 `ORSGateway` 中添加 `positionMgr` 字段
2. 初始化 `PositionManager`
3. 在 `handleOrderRequest` 中调用 `SetOpenClose`（仅当 `OpenClose == OC_UNKNOWN`）
4. 在 `handleTradeResponse` 中调用 `UpdatePositionFromTrade`
5. 添加日志记录

**验证**：
- ✅ 编译成功
- ✅ 启动测试（不影响现有功能）
- ✅ 日志显示 `SetOpenClose` 被调用

### Phase 3: 策略层适配（向后兼容）

**策略层保持两种模式**：

```go
// TradingSignal.ToOrderRequest()
func (ts *TradingSignal) ToOrderRequest() *orspb.OrderRequest {
    req := &orspb.OrderRequest{
        // ... 现有字段
    }

    // 如果策略设置了 OpenClose，使用策略的值
    if ts.OpenClose != OpenCloseUnknown {
        switch ts.OpenClose {
        case OpenCloseOpen:
            req.OpenClose = orspb.OpenClose_OPEN
        case OpenCloseClose:
            req.OpenClose = orspb.OpenClose_CLOSE
        // ...
        }
    } else {
        // 否则设置为 UNKNOWN，让 Gateway 自动判断
        req.OpenClose = orspb.OpenClose_OC_UNKNOWN
    }

    return req
}
```

**任务**：
1. 修改 `ToOrderRequest`，支持 `OpenClose == Unknown` 时传 `OC_UNKNOWN`
2. 保持向后兼容（策略仍可设置 OpenClose）

**验证**：
- ✅ 编译成功
- ✅ 现有策略正常运行

### Phase 4: 端到端测试

**测试场景**：

1. **空仓开仓**：
   - 初始：无持仓
   - 发送：BUY 2手
   - 预期：`OpenClose=OPEN`，开多成功

2. **持仓反向平仓**：
   - 初始：持有多仓 2手
   - 发送：SELL 2手
   - 预期：`OpenClose=CLOSE`，平多成功

3. **部分平仓**（Phase 5 支持）：
   - 初始：持有多仓 2手
   - 发送：SELL 5手
   - 预期：拆分为 CLOSE 2手 + OPEN 3手

4. **今昨仓区分**（上期所）：
   - 初始：昨仓 1手，今仓 1手
   - 发送：SELL 1手
   - 预期：`OpenClose=CLOSE_TODAY`（优先平今）

**验证**：
- ✅ 所有场景通过
- ✅ 日志显示 `OpenClose` 正确设置
- ✅ 持仓状态正确更新

### Phase 5: 策略层简化（可选）

移除配对套利策略中的 OpenClose 判断逻辑。

**任务**：
1. 删除 `generateEntrySignals` 中的 `leg1OpenClose` 判断
2. 删除 `generateExitSignals` 中的 `OpenClose=Close` 设置
3. 测试验证

**验证**：
- ✅ 策略正常运行
- ✅ 开平仓逻辑正确
- ✅ 代码更简洁

### Phase 6: 支持部分平仓（可选，长期）

**目标**：订单数量大于持仓时，自动拆分为平仓+开仓

**实现**：
```go
func (pm *PositionManager) SplitOrder(req *orspb.OrderRequest) []*orspb.OrderRequest {
    // 判断是否需要拆分
    // 如果需要，返回两个订单：[平仓订单, 开仓订单]
    // 否则返回 [原订单]
}
```

**集成**：
```go
func (gw *ORSGateway) handleOrderRequest(ctx context.Context, req *orspb.OrderRequest) {
    // 拆分订单（如需要）
    orders := gw.positionMgr.SplitOrder(req)

    for _, order := range orders {
        // 发送每个订单
        gw.sendToCounterBridge(order)
    }
}
```

---

## 6. 风险控制

### 6.1 向后兼容

**策略层仍可设置 OpenClose**：
- 如果 `signal.OpenClose != Unknown`，使用策略的值
- 如果 `signal.OpenClose == Unknown`，Gateway 自动判断

**渐进式迁移**：
- Phase 1-4：不影响现有策略
- Phase 5：可选择性简化策略代码

### 6.2 线程安全

**持仓管理器使用 mutex**：
```go
type PositionManager struct {
    positions map[string]*Position
    mutex     sync.RWMutex  // ← 读写锁
}

func (pm *PositionManager) SetOpenClose(req *orspb.OrderRequest) error {
    pm.mutex.Lock()         // 写锁
    defer pm.mutex.Unlock()
    // ...
}

func (pm *PositionManager) GetPosition(symbol string) *Position {
    pm.mutex.RLock()        // 读锁
    defer pm.mutex.RUnlock()
    // ...
}
```

### 6.3 持仓状态恢复

**启动时从 Counter Bridge 查询持仓**：
```go
func (pm *PositionManager) Initialize(client CounterBridgeClient) error {
    // 查询所有持仓
    positions, err := client.QueryPositions()
    if err != nil {
        return err
    }

    // 初始化持仓状态
    for _, pos := range positions {
        pm.positions[pos.Symbol] = convertPosition(pos)
    }

    return nil
}
```

### 6.4 异常处理

**持仓状态不一致时的处理**：
1. 记录错误日志
2. 发送告警
3. 可选：拒绝订单（保守策略）

---

## 7. 测试计划

### 7.1 单元测试

**文件**：`golang/pkg/gateway/position_manager_test.go`

**测试用例**：
1. `TestSetOpenClose_EmptyPosition_Buy` - 空仓买入（开多）
2. `TestSetOpenClose_EmptyPosition_Sell` - 空仓卖出（开空）
3. `TestSetOpenClose_LongPosition_Sell` - 持多卖出（平多）
4. `TestSetOpenClose_ShortPosition_Buy` - 持空买入（平空）
5. `TestSetOpenClose_TodayPosition_SHFE` - 今仓平仓（上期所）
6. `TestUpdatePositionFromTrade_Open` - 开仓更新持仓
7. `TestUpdatePositionFromTrade_Close` - 平仓更新持仓
8. `TestConcurrency` - 并发安全测试

### 7.2 集成测试

**测试场景**：
1. 启动系统（ORS Gateway + Counter Bridge + Trader）
2. 激活策略
3. 观察订单的 `OpenClose` 字段
4. 验证持仓更新

### 7.3 回归测试

确保现有功能不受影响：
- ✅ 策略仍可显式设置 `OpenClose`
- ✅ 订单正常发送和成交
- ✅ 持仓回报正常

---

## 8. 成功标准

### Phase 1-4 完成标准
- ✅ PositionManager 实现并测试通过
- ✅ ORS Gateway 集成成功
- ✅ 向后兼容，现有策略正常运行
- ✅ 端到端测试通过（4个基本场景）
- ✅ 无订单拒绝
- ✅ 持仓状态正确

### Phase 5 完成标准
- ✅ 策略代码简化
- ✅ 不再需要维护 `leg1Position`, `leg2Position`
- ✅ 策略层测试通过

### 最终成功标准
- ✅ 所有 Phase 完成
- ✅ 文档完整
- ✅ 代码质量良好
- ✅ 生产环境就绪

---

## 9. 参考资料

- ors/China 项目：`/Users/user/PWorks/RD/ors/China/src/ORSServer.cpp`
- CTP API 文档：`ThostFtdcUserApiStruct.h`
- 策略层 Offset 设置实施报告：`docs/实盘/策略层Offset设置实施报告_2026-01-30-21_30.md`
- 模拟器订单拒绝机制改进：`docs/实盘/模拟器订单拒绝机制改进_2026-01-30-21_00.md`

---

**最后更新**: 2026-01-30
**状态**: 📝 待实施
