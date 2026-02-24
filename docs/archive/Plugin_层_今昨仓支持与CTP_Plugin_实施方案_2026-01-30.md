# Plugin 层今昨仓支持与 CTP Plugin 实施方案

**文档日期**: 2026-01-30
**作者**: QuantLink Team
**版本**: v1.0
**相关模块**: Simulator Plugin, CTP Plugin

---

## 1. 当前状态分析

### 1.1 Simulator Plugin

**持仓结构**：
```cpp
// gateway/plugins/simulator/include/simulator_plugin.h
struct InternalPosition {
    std::string symbol;
    std::string exchange;
    hft::plugin::OrderDirection direction;
    uint32_t volume;              // Total position
    uint32_t today_volume;        // ✅ 有今仓字段
    uint32_t yesterday_volume;    // ✅ 有昨仓字段
    double avg_price;
    double total_cost;
    double total_volume_traded;
    double margin;
    double unrealized_pnl;
};

std::map<std::string, InternalPosition> m_positions;  // key: symbol
```

**SetOpenClose 实现（当前是简化版）**：
```cpp
void SimulatorPlugin::SetOpenClose(hft::plugin::OrderRequest& request) {
    // ⚠️ 简化版：只区分多空，不区分今昨仓

    if (无持仓) → OPEN
    if (持有空仓 && 买入) → CLOSE
    if (持有多仓 && 卖出) → CLOSE
    否则 → OPEN
}
```

**问题**：
- ❌ 不支持 CLOSE_TODAY / CLOSE_YESTERDAY
- ❌ 不区分今昨仓优先级
- ❌ 与上期所（SHFE）规则不一致

### 1.2 CTP Plugin

**当前状态**：
- ❌ **没有** SetOpenClose 方法
- ❌ **没有** 持仓管理（mapContractPos）
- ❌ **没有** 今昨仓支持

**问题**：
- 策略层发送的 offset 直接传给 CTP API
- 如果 offset 错误，CTP 会拒绝订单
- 没有自动判断和修正机制

---

## 2. ors/China 的完整实现（参考标准）

### 2.1 持仓结构

```cpp
// ors/China 的 contractPos 结构
struct contractPos {
    int ONLongPos;        // 昨日多头持仓
    int todayLongPos;     // 今日多头持仓
    int ONShortPos;       // 昨日空头持仓
    int todayShortPos;    // 今日空头持仓
};

std::map<std::string, contractPos> mapContractPos;  // key: InstrumentID
std::mutex contractPosLock;
```

### 2.2 SetCombOffsetFlag 完整逻辑

```cpp
void ORSServer::SetCombOffsetFlag(CThostFtdcInputOrderField &req,
                                  RequestMsg *request,
                                  char &flag, int &orderType) {
    contractPosLock.lock();

    mapContractPosIter = mapContractPos.find(std::string(req.InstrumentID));

    switch (request->Transaction_Type) {
    case BUY:
        if (request->Quantity <= mapContractPosIter->second.todayShortPos) {
            // 平今空仓（上期所需要 CLOSE_TODAY）
            if (request->Exchange_Type == CHINA_SHFE) {
                flag = THOST_FTDC_OF_CloseToday;
            } else {
                flag = THOST_FTDC_OF_Close;
            }
            mapContractPosIter->second.todayShortPos -= request->Quantity;
            orderType = CLOSE_TODAY;
        }
        else if (request->Quantity <= mapContractPosIter->second.ONShortPos) {
            // 平昨空仓
            flag = THOST_FTDC_OF_Close;  // 或 THOST_FTDC_OF_CloseYesterday
            mapContractPosIter->second.ONShortPos -= request->Quantity;
            orderType = CLOSE_YESTERDAY;
        }
        else {
            // 开多仓
            flag = THOST_FTDC_OF_Open;
            orderType = OPEN_ORDER;
        }
        break;

    case SELL:
        if (request->Quantity <= mapContractPosIter->second.todayLongPos) {
            // 平今多仓
            if (request->Exchange_Type == CHINA_SHFE) {
                flag = THOST_FTDC_OF_CloseToday;
            } else {
                flag = THOST_FTDC_OF_Close;
            }
            mapContractPosIter->second.todayLongPos -= request->Quantity;
            orderType = CLOSE_TODAY;
        }
        else if (request->Quantity <= mapContractPosIter->second.ONLongPos) {
            // 平昨多仓
            flag = THOST_FTDC_OF_Close;
            mapContractPosIter->second.ONLongPos -= request->Quantity;
            orderType = CLOSE_YESTERDAY;
        }
        else {
            // 开空仓
            flag = THOST_FTDC_OF_Open;
            orderType = OPEN_ORDER;
        }
        break;
    }

    contractPosLock.unlock();
}
```

**关键特性**：
1. ✅ 优先平今仓（todayPos）
2. ✅ 再平昨仓（ONPos / yesterdayPos）
3. ✅ 上期所（SHFE）区分 CLOSE_TODAY
4. ✅ 其他交易所统一用 CLOSE
5. ✅ 线程安全（mutex）
6. ✅ 实时更新持仓（减去已发订单量）

---

## 3. 实施方案

### Phase 1: Simulator Plugin 升级今昨仓支持

#### 3.1 修改 InternalPosition 结构

**当前问题**：`direction` 字段只能表示净持仓方向（多或空），无法同时表示今昨仓。

**方案**：改为类似 ors/China 的结构

```cpp
// gateway/plugins/simulator/include/simulator_plugin.h
struct InternalPosition {
    std::string symbol;
    std::string exchange;

    // 多头持仓（净持仓模型）
    uint32_t long_volume;          // 多头总持仓
    uint32_t today_long_volume;    // 今日多头持仓
    uint32_t yesterday_long_volume; // 昨日多头持仓
    double long_avg_price;         // 多头均价
    double long_total_cost;        // 多头总成本

    // 空头持仓（净持仓模型）
    uint32_t short_volume;          // 空头总持仓
    uint32_t today_short_volume;    // 今日空头持仓
    uint32_t yesterday_short_volume; // 昨日空头持仓
    double short_avg_price;         // 空头均价
    double short_total_cost;        // 空头总成本

    double margin;                  // 保证金
    double unrealized_pnl;          // 浮动盈亏
};
```

**迁移策略**：
- 保持向后兼容：如果现有代码使用 `direction` 和 `volume`，提供兼容方法
- 新代码使用 `long_volume` / `short_volume`

#### 3.2 升级 SetOpenClose（完整版）

```cpp
// gateway/plugins/simulator/src/simulator_plugin.cpp

void SimulatorPlugin::SetOpenClose(hft::plugin::OrderRequest& request) {
    std::lock_guard<std::mutex> lock(m_position_mutex);

    auto it = m_positions.find(request.symbol);

    if (it == m_positions.end()) {
        // 无持仓，开仓
        request.offset = hft::plugin::OffsetFlag::OPEN;
        return;
    }

    auto& pos = it->second;

    if (request.direction == hft::plugin::OrderDirection::BUY) {
        // 买入：先平今空，再平昨空，最后开多
        if (request.volume <= pos.today_short_volume) {
            // 平今空仓（上期所需要 CLOSE_TODAY）
            if (strcmp(request.exchange, "SHFE") == 0) {
                request.offset = hft::plugin::OffsetFlag::CLOSE_TODAY;
            } else {
                request.offset = hft::plugin::OffsetFlag::CLOSE;
            }
        }
        else if (request.volume <= pos.today_short_volume + pos.yesterday_short_volume) {
            // 平昨空仓（或部分平今+平昨，简化处理为 CLOSE）
            request.offset = hft::plugin::OffsetFlag::CLOSE;
        }
        else {
            // 开多仓
            request.offset = hft::plugin::OffsetFlag::OPEN;
        }
    }
    else { // SELL
        // 卖出：先平今多，再平昨多，最后开空
        if (request.volume <= pos.today_long_volume) {
            // 平今多仓
            if (strcmp(request.exchange, "SHFE") == 0) {
                request.offset = hft::plugin::OffsetFlag::CLOSE_TODAY;
            } else {
                request.offset = hft::plugin::OffsetFlag::CLOSE;
            }
        }
        else if (request.volume <= pos.today_long_volume + pos.yesterday_long_volume) {
            // 平昨多仓
            request.offset = hft::plugin::OffsetFlag::CLOSE;
        }
        else {
            // 开空仓
            request.offset = hft::plugin::OffsetFlag::OPEN;
        }
    }
}
```

#### 3.3 升级 UpdatePosition（今昨仓更新）

```cpp
void SimulatorPlugin::UpdatePosition(const hft::plugin::TradeInfo& trade) {
    std::lock_guard<std::mutex> lock(m_position_mutex);

    auto& pos = m_positions[trade.symbol];

    switch (trade.offset) {
    case hft::plugin::OffsetFlag::OPEN:
        if (trade.direction == hft::plugin::OrderDirection::BUY) {
            // 开多
            pos.long_volume += trade.volume;
            pos.today_long_volume += trade.volume;  // 全部计入今仓

            // 更新均价
            double old_cost = pos.long_total_cost;
            pos.long_total_cost += trade.price * trade.volume;
            pos.long_avg_price = pos.long_total_cost / pos.long_volume;
        } else {
            // 开空
            pos.short_volume += trade.volume;
            pos.today_short_volume += trade.volume;

            double old_cost = pos.short_total_cost;
            pos.short_total_cost += trade.price * trade.volume;
            pos.short_avg_price = pos.short_total_cost / pos.short_volume;
        }
        break;

    case hft::plugin::OffsetFlag::CLOSE:
    case hft::plugin::OffsetFlag::CLOSE_YESTERDAY:
        if (trade.direction == hft::plugin::OrderDirection::BUY) {
            // 平空：优先扣昨仓
            pos.short_volume -= trade.volume;

            if (pos.yesterday_short_volume >= trade.volume) {
                pos.yesterday_short_volume -= trade.volume;
            } else {
                uint32_t remaining = trade.volume - pos.yesterday_short_volume;
                pos.yesterday_short_volume = 0;
                pos.today_short_volume -= remaining;
            }

            if (pos.short_volume == 0) {
                pos.short_avg_price = 0;
                pos.short_total_cost = 0;
            }
        } else {
            // 平多：优先扣昨仓
            pos.long_volume -= trade.volume;

            if (pos.yesterday_long_volume >= trade.volume) {
                pos.yesterday_long_volume -= trade.volume;
            } else {
                uint32_t remaining = trade.volume - pos.yesterday_long_volume;
                pos.yesterday_long_volume = 0;
                pos.today_long_volume -= remaining;
            }

            if (pos.long_volume == 0) {
                pos.long_avg_price = 0;
                pos.long_total_cost = 0;
            }
        }
        break;

    case hft::plugin::OffsetFlag::CLOSE_TODAY:
        if (trade.direction == hft::plugin::OrderDirection::BUY) {
            // 平今空
            pos.short_volume -= trade.volume;
            pos.today_short_volume -= trade.volume;

            if (pos.short_volume == 0) {
                pos.short_avg_price = 0;
                pos.short_total_cost = 0;
            }
        } else {
            // 平今多
            pos.long_volume -= trade.volume;
            pos.today_long_volume -= trade.volume;

            if (pos.long_volume == 0) {
                pos.long_avg_price = 0;
                pos.long_total_cost = 0;
            }
        }
        break;
    }

    // 持仓归零时清除
    if (pos.long_volume == 0 && pos.short_volume == 0) {
        m_positions.erase(trade.symbol);
    }
}
```

#### 3.4 添加每日结算（今仓 → 昨仓）

```cpp
// gateway/plugins/simulator/include/simulator_plugin.h
class SimulatorPlugin : public hft::plugin::ITDPlugin {
    // ...
    void OnDayEnd();  // 日终结算：今仓转昨仓
};

// gateway/plugins/simulator/src/simulator_plugin.cpp
void SimulatorPlugin::OnDayEnd() {
    std::lock_guard<std::mutex> lock(m_position_mutex);

    for (auto& [symbol, pos] : m_positions) {
        // 今仓转昨仓
        pos.yesterday_long_volume += pos.today_long_volume;
        pos.today_long_volume = 0;

        pos.yesterday_short_volume += pos.today_short_volume;
        pos.today_short_volume = 0;

        std::cout << "[SimulatorPlugin] Day-end settlement: " << symbol
                  << " long=" << pos.long_volume
                  << " (yesterday=" << pos.yesterday_long_volume << ")"
                  << " short=" << pos.short_volume
                  << " (yesterday=" << pos.yesterday_short_volume << ")"
                  << std::endl;
    }
}
```

**调用时机**：
- 模拟器：可以在每天固定时间调用（如 15:30）
- 或者在第一笔交易日期变化时调用

---

### Phase 2: CTP Plugin 实施 SetOpenClose

#### 2.1 CTP Plugin 架构

**优势**：CTP Plugin 可以查询真实持仓，不需要自己维护！

```cpp
// gateway/plugins/ctp/include/ctp_td_plugin.h

class CTPTDPlugin : public hft::plugin::ITDPlugin {
public:
    // ...

private:
    // 持仓管理（查询自 CTP API）
    std::map<std::string, CTPPosition> m_positions;
    std::mutex m_position_mutex;

    // 新增方法
    void SetOpenClose(hft::plugin::OrderRequest& request);
    void UpdatePositionFromCTP();  // 从 CTP 查询持仓
};

struct CTPPosition {
    std::string symbol;
    std::string exchange;

    int long_position;           // 多头总持仓
    int today_long_position;     // 今日多头持仓
    int yesterday_long_position; // 昨日多头持仓（可选，CTP 统一用 Position - TodayPosition）

    int short_position;          // 空头总持仓
    int today_short_position;    // 今日空头持仓
    int yesterday_short_position;

    double long_avg_price;
    double short_avg_price;
};
```

#### 2.2 从 CTP 查询持仓

```cpp
void CTPTDPlugin::UpdatePositionFromCTP() {
    CThostFtdcQryInvestorPositionField req;
    memset(&req, 0, sizeof(req));
    strcpy(req.BrokerID, m_broker_id.c_str());
    strcpy(req.InvestorID, m_investor_id.c_str());

    int ret = m_trader_api->ReqQryInvestorPosition(&req, ++m_request_id);

    // 在 OnRspQryInvestorPosition 回调中更新 m_positions
}

void CTPTDPlugin::OnRspQryInvestorPosition(
    CThostFtdcInvestorPositionField *pInvestorPosition,
    CThostFtdcRspInfoField *pRspInfo, int nRequestID, bool bIsLast) {

    if (pInvestorPosition) {
        std::lock_guard<std::mutex> lock(m_position_mutex);

        std::string symbol = pInvestorPosition->InstrumentID;
        auto& pos = m_positions[symbol];

        pos.symbol = symbol;
        pos.exchange = pInvestorPosition->ExchangeID;

        if (pInvestorPosition->PosiDirection == THOST_FTDC_PD_Long) {
            // 多头持仓
            pos.long_position = pInvestorPosition->Position;
            pos.today_long_position = pInvestorPosition->TodayPosition;
            pos.yesterday_long_position = pos.long_position - pos.today_long_position;
            pos.long_avg_price = pInvestorPosition->PositionCost / pos.long_position;
        }
        else if (pInvestorPosition->PosiDirection == THOST_FTDC_PD_Short) {
            // 空头持仓
            pos.short_position = pInvestorPosition->Position;
            pos.today_short_position = pInvestorPosition->TodayPosition;
            pos.yesterday_short_position = pos.short_position - pos.today_short_position;
            pos.short_avg_price = pInvestorPosition->PositionCost / pos.short_position;
        }
    }
}
```

#### 2.3 CTP Plugin SetOpenClose

```cpp
void CTPTDPlugin::SetOpenClose(hft::plugin::OrderRequest& request) {
    std::lock_guard<std::mutex> lock(m_position_mutex);

    auto it = m_positions.find(request.symbol);

    if (it == m_positions.end()) {
        request.offset = hft::plugin::OffsetFlag::OPEN;
        return;
    }

    auto& pos = it->second;

    if (request.direction == hft::plugin::OrderDirection::BUY) {
        // 买入：平空
        if (request.volume <= pos.today_short_position) {
            // 平今空
            if (strcmp(request.exchange, "SHFE") == 0) {
                request.offset = hft::plugin::OffsetFlag::CLOSE_TODAY;
            } else {
                request.offset = hft::plugin::OffsetFlag::CLOSE;
            }
        }
        else if (request.volume <= pos.short_position) {
            // 平昨空（或部分平今+平昨）
            request.offset = hft::plugin::OffsetFlag::CLOSE;
        }
        else {
            // 开多
            request.offset = hft::plugin::OffsetFlag::OPEN;
        }
    }
    else { // SELL
        // 卖出：平多
        if (request.volume <= pos.today_long_position) {
            // 平今多
            if (strcmp(request.exchange, "SHFE") == 0) {
                request.offset = hft::plugin::OffsetFlag::CLOSE_TODAY;
            } else {
                request.offset = hft::plugin::OffsetFlag::CLOSE;
            }
        }
        else if (request.volume <= pos.long_position) {
            // 平昨多
            request.offset = hft::plugin::OffsetFlag::CLOSE;
        }
        else {
            // 开空
            request.offset = hft::plugin::OffsetFlag::OPEN;
        }
    }
}
```

#### 2.4 在 SendOrder 中调用

```cpp
std::string CTPTDPlugin::SendOrder(const hft::plugin::OrderRequest& request) {
    // 复制 request
    hft::plugin::OrderRequest modified_request = request;

    // 自动设置 offset
    SetOpenClose(modified_request);

    // 转换为 CTP 订单
    CThostFtdcInputOrderField ctp_order;
    // ... 转换逻辑

    // 设置 CombOffsetFlag
    switch (modified_request.offset) {
    case hft::plugin::OffsetFlag::OPEN:
        ctp_order.CombOffsetFlag[0] = THOST_FTDC_OF_Open;
        break;
    case hft::plugin::OffsetFlag::CLOSE:
        ctp_order.CombOffsetFlag[0] = THOST_FTDC_OF_Close;
        break;
    case hft::plugin::OffsetFlag::CLOSE_TODAY:
        ctp_order.CombOffsetFlag[0] = THOST_FTDC_OF_CloseToday;
        break;
    case hft::plugin::OffsetFlag::CLOSE_YESTERDAY:
        ctp_order.CombOffsetFlag[0] = THOST_FTDC_OF_CloseYesterday;
        break;
    }
    ctp_order.CombOffsetFlag[1] = '\0';

    // 发送订单到 CTP
    int ret = m_trader_api->ReqOrderInsert(&ctp_order, ++m_request_id);
    // ...
}
```

#### 2.5 定期查询持仓

```cpp
// 在 Login 成功后查询一次
void CTPTDPlugin::OnRspUserLogin(...) {
    // ...
    UpdatePositionFromCTP();
}

// 在订单成交后查询（可选，或根据成交回报更新）
void CTPTDPlugin::OnRtnTrade(CThostFtdcTradeField *pTrade) {
    // 方案1：定时查询（推荐）
    // 方案2：根据成交回报实时更新（复杂，可能不准）
}
```

---

## 4. 实施步骤

### Step 1: Simulator Plugin 升级（优先级：中）

**原因**：模拟环境，可以晚些实施今昨仓

1. 修改 `InternalPosition` 结构
2. 升级 `SetOpenClose` 支持今昨仓
3. 升级 `UpdatePosition` 支持今昨仓
4. 添加 `OnDayEnd` 日终结算
5. 测试验证

**时间估计**：4-6 小时

### Step 2: CTP Plugin 实施（优先级：高）

**原因**：真实交易，必须支持今昨仓

1. 添加 `CTPPosition` 结构和 `m_positions`
2. 实现 `UpdatePositionFromCTP` 查询持仓
3. 实现 `SetOpenClose` 方法
4. 修改 `SendOrder` 调用 `SetOpenClose`
5. 在 Login 后查询持仓
6. 测试验证

**时间估计**：6-8 小时

### Step 3: 测试验证

**测试场景**：

1. **Simulator 今昨仓测试**：
   - Day 1: 开多 10 手
   - Day 2: 卖出 5 手 → 应该平昨 5 手（CLOSE 或 CLOSE_YESTERDAY）
   - Day 2: 买入 5 手 → 应该开多 5 手（OPEN）
   - 验证：持仓今昨仓数量正确

2. **CTP Plugin 测试**（实盘模拟）：
   - 查询当前持仓（有今仓和昨仓）
   - 发送平仓订单
   - 验证：上期所品种使用 CLOSE_TODAY，其他交易所使用 CLOSE
   - 验证：CTP 不拒绝订单

---

## 5. 风险和注意事项

### 5.1 持仓不一致问题

**问题**：Plugin 维护的持仓可能与真实持仓不一致

**解决方案**：
- **Simulator**：启动时清空持仓，或从快照恢复
- **CTP Plugin**：定期查询真实持仓（推荐每分钟查询一次）

### 5.2 部分平仓

**问题**：订单数量大于持仓时需要拆单

**当前方案**：简化处理，不拆单（订单会被拒绝或部分成交）

**完整方案**（将来实施）：
```cpp
std::vector<OrderRequest> SplitOrder(const OrderRequest& request) {
    // 拆分为：平仓订单 + 开仓订单
}
```

### 5.3 上期所今昨仓规则

**规则**：
- 平今仓手续费较低
- 必须使用 `THOST_FTDC_OF_CloseToday`
- 其他交易所可统一用 `THOST_FTDC_OF_Close`

**实现**：检查 `exchange == "SHFE"`

### 5.4 线程安全

**所有持仓访问必须加锁**：
```cpp
std::lock_guard<std::mutex> lock(m_position_mutex);
```

---

## 6. 成功标准

### Simulator Plugin 升级完成标准
- ✅ InternalPosition 支持今昨仓分离
- ✅ SetOpenClose 区分 CLOSE_TODAY / CLOSE
- ✅ UpdatePosition 正确更新今昨仓
- ✅ OnDayEnd 今仓转昨仓
- ✅ 单元测试通过

### CTP Plugin 实施完成标准
- ✅ 能查询 CTP 真实持仓
- ✅ SetOpenClose 自动设置 offset
- ✅ SendOrder 调用 SetOpenClose
- ✅ 上期所品种使用 CLOSE_TODAY
- ✅ 真实环境测试：零订单拒绝

---

## 7. 优先级建议

**立即实施**：
- ✅ CTP Plugin SetOpenClose（高优先级，真实交易必需）

**中期实施**：
- ⏳ Simulator Plugin 今昨仓支持（中优先级，模拟测试需要）

**长期优化**：
- 🔄 部分平仓自动拆单
- 🔄 持仓定期校验与恢复
- 🔄 持仓快照持久化

---

**最后更新**: 2026-01-30 02:30
**状态**: 📋 待实施
