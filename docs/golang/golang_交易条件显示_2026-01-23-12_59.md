# QuantlinkTrader 交易条件显示功能实现

**文档创建时间**: 2026-01-23 12:59
**实现版本**: v1.0.0
**对应需求**: 为交易员提供实时指标显示和条件满足提示

---

## 1. 需求背景

### 原始需求
交易员需要一个页面，实时显示：
1. 所有交易指标值（Z-Score、价差、相关性等）
2. 当前市场条件是否满足交易要求
3. 策略是否处于可激活状态
4. 明确的视觉提示，告知何时应该激活策略

### tbsrc 参考设计
tbsrc 使用两层控制机制：
- **第1层**：手动激活 (`m_Active`) - 交易员通过信号控制
- **第2层**：条件检查 (`signal > BEGIN_PLACE`) - 自动计算

```cpp
// tbsrc 示例
if (m_Active && signal > m_thold->BEGIN_PLACE) {
    // 两个条件都满足才下单
    SendOrder();
}
```

---

## 2. 实现架构

### 2.1 核心组件扩展

#### StrategyControlState 增强
**文件**: `pkg/strategy/state_control.go`

新增字段：
```go
type StrategyControlState struct {
    // 原有字段
    RunState       StrategyRunState
    Active         bool
    FlattenMode    bool
    // ... 其他控制字段

    // 新增：交易条件状态
    ConditionsMet   bool              // 市场条件是否满足
    SignalStrength  float64           // 当前信号强度（如 z-score）
    LastSignalTime  time.Time         // 最后信号时间
    Eligible        bool              // 是否可激活（条件满足但未激活）
    EligibleReason  string            // 说明原因
    Indicators      map[string]float64 // 所有指标值
}
```

新增方法：
```go
// 更新交易条件状态
func (scs *StrategyControlState) UpdateConditions(
    conditionsMet bool,
    signalStrength float64,
    indicators map[string]float64
)

// 获取条件状态摘要
func (scs *StrategyControlState) GetConditionStatus() map[string]interface{}
```

#### PairwiseArbStrategy 集成
**文件**: `pkg/strategy/pairwise_arb_strategy.go`

在 `OnMarketData()` 中添加条件计算：
```go
func (pas *PairwiseArbStrategy) OnMarketData(md *mdpb.MarketDataUpdate) {
    // ... 原有逻辑

    // 计算所有指标
    indicators := map[string]float64{
        "z_score":         pas.currentZScore,
        "entry_threshold": pas.entryZScore,
        "exit_threshold":  pas.exitZScore,
        "spread":          pas.currentSpread,
        "spread_mean":     pas.spreadMean,
        "spread_std":      pas.spreadStd,
        "correlation":     correlation,
        "min_correlation": pas.minCorrelation,
        "hedge_ratio":     pas.hedgeRatio,
        "price1":          pas.price1,
        "price2":          pas.price2,
    }

    // 判断条件是否满足
    conditionsMet := pas.spreadStd > 1e-10 &&
        math.Abs(pas.currentZScore) >= pas.entryZScore &&
        correlation >= pas.minCorrelation &&
        len(pas.spreadHistory) >= pas.lookbackPeriod

    // 更新控制状态
    pas.ControlState.UpdateConditions(conditionsMet, pas.currentZScore, indicators)
}
```

### 2.2 API 扩展

#### StrategyStatusResponse 增强
**文件**: `pkg/trader/api.go`

新增字段：
```go
type StrategyStatusResponse struct {
    // 原有字段
    StrategyID string
    Running    bool
    Active     bool
    // ...

    // 新增：交易条件字段
    ConditionsMet   bool               `json:"conditions_met"`
    Eligible        bool               `json:"eligible"`
    EligibleReason  string             `json:"eligible_reason"`
    SignalStrength  float64            `json:"signal_strength"`
    LastSignalTime  string             `json:"last_signal_time"`
    Indicators      map[string]float64 `json:"indicators"`
}
```

#### API 响应示例
```json
{
  "success": true,
  "data": {
    "running": true,
    "active": false,
    "conditions_met": true,
    "eligible": true,
    "eligible_reason": "Conditions met (signal: 2.34)",
    "signal_strength": 2.34,
    "last_signal_time": "12:45:30",
    "indicators": {
      "z_score": 2.34,
      "entry_threshold": 2.00,
      "correlation": 0.85,
      "min_correlation": 0.70,
      "spread": 5.23,
      "hedge_ratio": 1.05,
      "price1": 7128.50,
      "price2": 7123.27
    }
  }
}
```

### 2.3 Web UI 增强

#### 新增交易条件卡片
**文件**: `web/control.html`

新增 HTML 结构：
```html
<!-- 交易条件卡片 -->
<div class="conditions-card" id="conditionsCard">
    <h3>📊 交易条件</h3>

    <div class="condition-status" id="conditionStatus">
        等待数据...
    </div>

    <div class="indicator-grid" id="indicatorGrid">
        <!-- 指标动态生成 -->
    </div>
</div>
```

#### 新增 CSS 样式
```css
/* 条件状态样式 */
.condition-status.met {
    background: #d4edda;
    color: #155724;
    border: 2px solid #28a745;
}

.condition-status.not-met {
    background: #fff3cd;
    color: #856404;
    border: 2px solid #ffc107;
}

.condition-status.eligible {
    background: #d1ecf1;
    color: #0c5460;
    border: 2px solid #17a2b8;
    animation: pulse 2s infinite;
}

/* 指标项样式 */
.indicator-item.met {
    border-left-color: #28a745;
    background: #d4edda;
}

/* 激活按钮高亮 */
.btn-activate.highlight {
    background: linear-gradient(135deg, #28a745 0%, #20c997 100%);
    animation: glow 1.5s infinite;
    box-shadow: 0 0 20px rgba(40, 167, 69, 0.5);
}
```

#### JavaScript 逻辑
```javascript
function updateConditionsDisplay(status) {
    if (status.eligible) {
        // 条件满足但未激活 - 提示交易员可以激活
        conditionStatus.className = 'condition-status eligible';
        conditionStatus.innerHTML = `🎯 ${status.eligible_reason}<br>
            <small>点击下方按钮激活策略开始交易</small>`;
        btnActivate.classList.add('highlight');
    } else if (status.conditions_met && status.active) {
        // 条件满足且已激活 - 正在交易
        conditionStatus.className = 'condition-status met';
        conditionStatus.innerHTML = `✅ 交易条件满足，策略正在运行`;
        btnActivate.classList.remove('highlight');
    } else if (!status.conditions_met) {
        // 条件不满足
        conditionStatus.className = 'condition-status not-met';
        conditionStatus.innerHTML = `⏳ ${status.eligible_reason}`;
        btnActivate.classList.remove('highlight');
    }

    // 生成指标卡片
    Object.keys(status.indicators).forEach(key => {
        const value = status.indicators[key];
        const isMet = checkIfMet(key, value, status.indicators);
        const itemClass = isMet ? 'indicator-item met' : 'indicator-item';

        const item = document.createElement('div');
        item.className = itemClass;
        item.innerHTML = `
            <div class="indicator-label">${getLabel(key)}</div>
            <div class="indicator-value">${value.toFixed(4)}</div>
        `;
        indicatorGrid.appendChild(item);
    });
}
```

---

## 3. 状态管理优化

### 3.1 状态变量语义明确

#### Active vs RunState
**问题**：原设计中 `Active` 和 `RunState` 语义混淆

**解决**：明确区分
```go
// Active: 策略是否激活（可交易）
// 对应 tbsrc: m_Active
Active bool

// RunState: 进程运行状态
// Active/Paused/Flattening/Exiting/Stopped
RunState StrategyRunState
```

#### IsRunning() vs IsActive()
```go
// IsRunning() - 进程是否在运行
func (bs *BaseStrategy) IsRunning() bool {
    return bs.ControlState.RunState != StrategyRunStateStopped
}

// IsActive() - 策略是否已激活（可交易）
func (scs *StrategyControlState) IsActive() bool {
    return scs.Active
}
```

### 3.2 Live 模式初始状态

**文件**: `pkg/trader/trader.go`

在策略初始化后，根据模式设置激活状态：
```go
func (t *Trader) Initialize() error {
    // ... 初始化策略

    // 设置初始激活状态
    baseStrat := t.getBaseStrategy()
    if baseStrat != nil {
        if t.Config.System.Mode == "live" {
            // Live 模式：初始未激活
            baseStrat.ControlState.Deactivate()
            log.Println("[Trader] Initial state: NOT activated (live mode)")
        } else {
            // Simulation/Backtest 模式：默认激活
            baseStrat.ControlState.Activate()
            log.Println("[Trader] Initial state: Activated (non-live mode)")
        }
    }
    // ...
}
```

### 3.3 Deactivate() 行为修正

**问题**：之前 `Deactivate()` 会将 `RunState` 设为 `Stopped`，导致 `IsRunning()` 返回 false

**解决**：只修改 `Active` 字段
```go
func (scs *StrategyControlState) Deactivate() {
    scs.Active = false
    // RunState 保持不变
    // - Active=false 表示"不能交易"
    // - 但进程仍在运行（对应 tbsrc: 进程还在，只是 m_Active=false）
}
```

---

## 4. 用户体验设计

### 4.1 三种状态展示

#### 状态1：条件未满足
```
⏳ 等待交易条件满足

指标显示：
Z-Score:         0.50 (灰色)
入场阈值:        2.00 (黄色标注)
相关性:          0.60 (灰色)
最小相关性:      0.70 (黄色标注)
```

#### 状态2：条件满足但未激活 ⭐
```
🎯 策略可激活！(signal: 2.34)
点击下方按钮激活策略开始交易

指标显示：
Z-Score:         2.34 (绿色高亮) ✓
入场阈值:        2.00
相关性:          0.85 (绿色高亮) ✓
最小相关性:      0.70

[激活策略] 按钮闪烁高亮
```

#### 状态3：已激活且交易中
```
✅ 交易条件满足，策略正在运行

指标显示：
Z-Score:         2.34 (绿色) ✓
价差:            5.23
对冲比率:        1.05
...

[停止策略] 按钮可用
```

### 4.2 视觉反馈

- **脉冲动画**：条件满足时，状态卡片脉动
- **按钮高亮**：激活按钮闪烁提示
- **颜色编码**：
  - 绿色 = 条件满足
  - 黄色 = 阈值/条件不满足
  - 灰色 = 普通指标

---

## 5. 完整工作流程

### 5.1 启动流程
```
1. 启动 QuantlinkTrader (live 模式)
   ├─ 加载配置
   ├─ 初始化策略
   └─ 设置 Active=false (未激活)

2. Web UI 刷新
   ├─ GET /api/v1/strategy/status
   ├─ 显示"运行中"+"未激活"
   └─ 显示"交易条件"卡片（如果有行情数据）

3. 行情数据到达
   ├─ OnMarketData() 计算指标
   ├─ UpdateConditions() 更新状态
   └─ Web UI 自动刷新（每10秒）
```

### 5.2 激活流程
```
交易员观察指标
    ↓
条件满足（Z-Score >= 2.0, 相关性 >= 0.7）
    ↓
Web UI 提示：🎯 策略可激活！
激活按钮闪烁
    ↓
交易员点击"激活策略"
    ↓
POST /api/v1/strategy/activate
    ├─ ControlState.Activate()
    ├─ Strategy.Start()
    └─ Active=true, RunState=Active
    ↓
策略开始交易（如果条件仍然满足）
```

### 5.3 交易逻辑
```go
func (pas *PairwiseArbStrategy) generateSignals(md *mdpb.MarketDataUpdate) {
    // 检查1：策略是否激活
    if !pas.ControlState.IsActivated() {
        return  // 未激活，不交易
    }

    // 检查2：条件是否满足
    if math.Abs(pas.currentZScore) < pas.entryZScore {
        return  // 条件不满足，不交易
    }

    if correlation < pas.minCorrelation {
        return  // 相关性不足，不交易
    }

    // 两个条件都满足，生成交易信号
    pas.generateSpreadSignals(md, direction, qty)
}
```

---

## 6. 关键配置

### 6.1 配置文件
**文件**: `config/trader.ag2502.ag2504.yaml`

```yaml
system:
  strategy_id: "92201"
  mode: "live"  # live 模式，等待手动激活

strategy:
  type: "pairwise_arb"
  symbols: ["ag2502", "ag2504"]
  parameters:
    entry_zscore: 2.0      # 入场阈值
    exit_zscore: 0.5       # 出场阈值
    min_correlation: 0.7   # 最小相关性
    lookback_period: 100   # 回看周期

api:
  enabled: true
  port: 9201
```

### 6.2 启动命令
```bash
# 编译
go build -o bin/trader cmd/trader/main.go

# 启动
./bin/trader -config config/trader.ag2502.ag2504.yaml

# 或后台运行
nohup ./bin/trader -config config/trader.ag2502.ag2504.yaml \
    >> ./log/trader.log 2>&1 &
```

---

## 7. API 端点

### GET /api/v1/strategy/status
获取策略状态（包含指标）

**请求**：
```bash
curl http://localhost:9201/api/v1/strategy/status
```

**响应**：
```json
{
  "success": true,
  "message": "Strategy status retrieved",
  "data": {
    "strategy_id": "92201",
    "running": true,
    "active": false,
    "conditions_met": true,
    "eligible": true,
    "eligible_reason": "Conditions met (signal: 2.34)",
    "signal_strength": 2.34,
    "indicators": {
      "z_score": 2.34,
      "entry_threshold": 2.00,
      "correlation": 0.85,
      "min_correlation": 0.70,
      "spread": 5.23
    }
  }
}
```

### POST /api/v1/strategy/activate
激活策略

**请求**：
```bash
curl -X POST http://localhost:9201/api/v1/strategy/activate
```

**响应**：
```json
{
  "success": true,
  "message": "Strategy activated successfully",
  "data": {
    "strategy_id": "92201",
    "active": true,
    "running": true
  }
}
```

### POST /api/v1/strategy/deactivate
停止策略

**请求**：
```bash
curl -X POST http://localhost:9201/api/v1/strategy/deactivate
```

---

## 8. 文件修改清单

### 核心文件
| 文件 | 修改内容 | 行数 |
|------|---------|------|
| `pkg/strategy/state_control.go` | 新增条件状态字段和方法 | +100 |
| `pkg/strategy/pairwise_arb_strategy.go` | 集成条件计算和更新 | +45 |
| `pkg/trader/api.go` | 扩展 API 响应字段 | +15 |
| `pkg/trader/trader.go` | 根据模式设置初始状态 | +15 |
| `pkg/strategy/strategy.go` | 修正 IsRunning() 逻辑 | +5 |
| `web/control.html` | 新增条件卡片和 JS 逻辑 | +150 |

### 配置文件
| 文件 | 修改内容 |
|------|---------|
| `config/trader.ag2502.ag2504.yaml` | mode: simulation → live |

---

## 9. 测试验证

### 9.1 功能测试
```bash
# 1. 启动系统
./bin/trader -config config/trader.ag2502.ag2504.yaml

# 2. 检查初始状态
curl http://localhost:9201/api/v1/strategy/status | jq '.data | {running, active}'
# 期望：{"running": true, "active": false}

# 3. 打开 Web UI
open web/control.html

# 4. 验证显示
#    - 运行状态：运行中 ✓
#    - 激活状态：未激活 ✓
#    - 交易条件：根据行情数据显示

# 5. 激活策略
curl -X POST http://localhost:9201/api/v1/strategy/activate

# 6. 验证激活后状态
curl http://localhost:9201/api/v1/strategy/status | jq '.data.active'
# 期望：true
```

### 9.2 状态转换测试
```
未激活 → 激活 → 停止 → 重新激活
  ↓        ↓      ↓        ↓
false    true   false    true  (验证通过 ✓)
```

---

## 10. 与 tbsrc 对比

| 特性 | tbsrc | QuantlinkTrader |
|------|-------|-----------------|
| **控制方式** | Unix 信号 (SIGUSR1/2) | Unix 信号 + HTTP API |
| **激活状态** | `m_Active` (bool) | `Active` (bool) |
| **条件检查** | `signal > BEGIN_PLACE` | `UpdateConditions()` |
| **指标显示** | 日志输出 | Web UI 实时显示 |
| **状态管理** | 多个 bool 标志 | `Active` + `RunState` 枚举 |
| **可见性** | 需要查看日志 | 实时图形界面 |
| **用户体验** | 命令行 | 现代化 Web UI |

---

## 11. 核心优势

### 11.1 交易员友好
✅ 一目了然：所有指标实时显示
✅ 智能提示：条件满足时高亮闪烁
✅ 减少失误：只有条件满足才提示激活
✅ 双重保护：激活 + 条件检查两层机制

### 11.2 风险可控
✅ 手动激活：交易员完全控制
✅ 自动检查：系统自动验证条件
✅ 实时监控：指标异常立即可见
✅ 防止误操作：条件不满足时不建议激活

### 11.3 技术优势
✅ 完全对应 tbsrc 设计哲学
✅ 清晰的状态管理（Active + RunState）
✅ 扩展性强（易于添加新指标）
✅ 类型安全（Go 类型系统保障）

---

## 12. 后续改进建议

### 12.1 短期优化
- [ ] 添加历史信号强度图表
- [ ] 支持自定义阈值（Web UI 动态调整）
- [ ] 添加声音提示（条件满足时）
- [ ] 支持多策略并行显示

### 12.2 长期规划
- [ ] 移除 `Active` 字段，统一使用 `RunState`
- [ ] 添加回测模式下的条件回放
- [ ] 集成机器学习模型预测条件满足概率
- [ ] 支持策略组合的条件聚合显示

---

## 13. 常见问题

### Q1: 为什么有 Active 和 RunState 两个状态？
**A**: 历史原因。`Active` 对应 tbsrc 的 `m_Active`，`RunState` 是后来添加的枚举状态。未来建议统一。

### Q2: 条件满足但激活后不交易？
**A**: 检查：
1. 行情数据是否持续更新
2. 交易时间是否在 session 范围内
3. 持仓是否已达上限

### Q3: Web UI 不显示条件卡片？
**A**: 需要等待至少一次行情数据更新，系统才会计算指标并返回。

### Q4: 如何调整入场阈值？
**A**: 修改配置文件 `entry_zscore` 参数，重启系统。未来版本将支持 Web UI 动态调整。

---

## 14. 总结

本次实现完成了**交易条件实时显示**功能，核心特点：

1. **两层控制机制** - 手动激活 + 自动条件检查（对应 tbsrc 设计）
2. **实时指标显示** - 所有关键指标实时更新，一目了然
3. **智能提示系统** - 条件满足时视觉高亮，提示交易员
4. **状态管理优化** - 明确区分进程运行状态和策略激活状态
5. **完整测试验证** - 功能完整，状态转换正确

系统现已完全满足需求：**只有交易员看到条件满足并手动激活，策略才会开始交易**。

---

**文档版本**: 1.0
**最后更新**: 2026-01-23 12:59
**维护者**: Claude Code
