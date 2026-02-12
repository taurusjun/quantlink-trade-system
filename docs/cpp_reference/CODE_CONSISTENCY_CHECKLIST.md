# C++ 与 Go 代码一致性检查清单

**文档日期**: 2026-02-12
**版本**: v1.0
**目的**: 系统性地对照 C++ 原代码与 Go 迁移代码，确保关键逻辑一致

---

## 检查方法论

### 1. 变量生命周期追踪

对于每个关键变量，必须检查完整的生命周期：

```
┌─────────────────┐
│   初始化        │  ← 从哪里读取？默认值是什么？
├─────────────────┤
│   更新          │  ← 何时更新？更新公式是什么？
├─────────────────┤
│   使用          │  ← 在哪里使用？如何影响交易逻辑？
├─────────────────┤
│   保存          │  ← 保存到哪里？保存的是什么值？
└─────────────────┘
```

### 2. 对照检查流程

1. **找到 C++ 原代码**
   - 路径：`/Users/user/PWorks/RD/tbsrc/Strategies/`
   - 头文件：`include/ExecutionStrategy.h`

2. **列出变量的所有读写点**
   - 使用 grep 搜索所有引用
   - 记录每个引用的行号和上下文

3. **对照 Go 代码**
   - 确认每个读写点都有对应实现
   - 确认逻辑完全一致

---

## 关键变量检查清单

### 1. avgSpreadRatio_ori (价差均值)

| 检查项 | C++ | Go | 状态 |
|--------|-----|-----|------|
| **初始化来源** | daily_init 文件 | daily_init 文件 | ✅ 一致 |
| **默认值** | 0 | 0 | ✅ 一致 |
| **更新公式** | `(1-ALPHA)*ori + ALPHA*curr` (EMA) | `spreadSeries.Stats().Mean` (SMA) | ❌ **不一致** |
| **更新触发** | 收到 Leg1 行情时 | UpdateStatistics 调用时 | ⚠️ 需验证 |
| **保存位置** | daily_init avgPx 字段 | daily_init avgPx 字段 | ✅ 一致 |
| **保存值** | avgSpreadRatio_ori | spreadAnalyzer.Mean | ✅ 已修复 |

**问题**: Go 使用简单移动平均 (SMA)，C++ 使用指数移动平均 (EMA)

**修复建议**:
1. 在 SpreadAnalyzer 中添加 EMA 模式
2. 或在 PairwiseArbStrategy 中手动维护 avgSpreadRatio_ori

### 2. m_netpos_pass / NetPosPass (Leg1 被动持仓)

| 检查项 | C++ | Go | 状态 |
|--------|-----|-----|------|
| **初始化来源** | daily_init ytd1 + 2day | daily_init ytd1 + 2day | ✅ 一致 |
| **成交更新** | 被动单成交时 += / -= | 被动单成交时 += / -= | ✅ 一致 |
| **保存位置** | daily_init ytd1 字段 | daily_init ytd1 字段 | ✅ 一致 |

### 3. m_netpos_agg / NetPosAgg (Leg2 主动持仓)

| 检查项 | C++ | Go | 状态 |
|--------|-----|-----|------|
| **初始化来源** | daily_init ytd2 | daily_init ytd2 | ✅ 一致 |
| **CTP 持仓初始化** | N/A | InitializePositions | ✅ 已修复 |
| **成交更新** | 主动单成交时 += / -= | 主动单成交时 += / -= | ✅ 一致 |
| **保存位置** | daily_init ytd2 字段 | daily_init ytd2 字段 | ✅ 一致 |

### 4. m_netpos_pass_ytd / NetPosPassYtd (Leg1 昨仓)

| 检查项 | C++ | Go | 状态 |
|--------|-----|-----|------|
| **初始化来源** | daily_init ytd1 | daily_init ytd1 | ✅ 一致 |
| **更新** | 不更新（只在初始化时设置） | 不更新（只在初始化时设置） | ✅ 一致 |
| **使用** | 区分昨仓/今仓 | 区分昨仓/今仓 | ✅ 一致 |

### 5. tValue (外部调整值)

| 检查项 | C++ | Go | 状态 |
|--------|-----|-----|------|
| **初始化** | 默认 0 | 默认 0 | ✅ 一致 |
| **更新来源** | 共享内存 m_tvar | 共享内存 TVar | ✅ 一致 |
| **使用公式** | `avgSpreadRatio = ori + tValue` | `Mean + tValue` 调整 ZScore | ⚠️ 语义等价 |

### 6. ALPHA (EMA 平滑因子)

| 检查项 | C++ | Go | 状态 |
|--------|-----|-----|------|
| **来源** | 配置文件 m_firstStrat->m_thold->ALPHA | 无对应字段 | ❌ **缺失** |
| **默认值** | 通常 0.01 - 0.1 | N/A | ❌ **缺失** |
| **使用** | EMA 公式 | 无 | ❌ **缺失** |

---

## 算法公式对照

### 1. 价差均值更新

**C++ (EMA)**:
```cpp
// PairwiseArbStrategy.cpp:519-522
avgSpreadRatio_ori = (1 - ALPHA) * avgSpreadRatio_ori + ALPHA * currSpreadRatio;
avgSpreadRatio = avgSpreadRatio_ori + tValue;
```

**Go (当前实现 - SMA)**:
```go
// spread/analyzer.go:137-138
spreadStats := sa.spreadSeries.Stats(lookbackPeriod)
sa.spreadMean = spreadStats.Mean  // 简单平均
```

**应改为 (EMA)**:
```go
// 伪代码
alpha := pas.params.Alpha  // 从配置读取
pas.avgSpreadRatio_ori = (1 - alpha) * pas.avgSpreadRatio_ori + alpha * currentSpread
```

### 2. Z-Score 计算

**C++**:
```cpp
// PairwiseArbStrategy.cpp:205-206
currSpreadRatio = mid1 - mid2 * PRICE_RATIO;
expectedRatio = (currSpreadRatio - avgSpreadRatio) / m_stdevSpreadRatio;
```

**Go**:
```go
// spread/analyzer.go:142
sa.currentZScore = stats.ZScore(sa.currentSpread, sa.spreadMean, sa.spreadStd)
```

✅ 公式一致（假设 PRICE_RATIO = 1）

### 3. 动态阈值调整

**C++ (ExecutionStrategy.cpp)**:
```cpp
void ExecutionStrategy::SetThresholds() {
    auto long_place_diff_thold = m_thold_first->LONG_PLACE - m_thold_first->BEGIN_PLACE;
    auto short_place_diff_thold = m_thold_first->BEGIN_PLACE - m_thold_first->SHORT_PLACE;

    if (m_netpos_pass > 0) {
        // 多头持仓，调整做多阈值
        m_thold_first->LONG_PLACE = m_thold_first->BEGIN_PLACE +
            (1.0 - (double)m_netpos_pass / (double)MAX_POSITION_SIZE) * long_place_diff_thold;
    }
    // ...
}
```

**Go**: 需检查 `setDynamicThresholds()` 实现是否一致

---

## 待修复问题清单

| # | 问题 | 严重程度 | 状态 | 修复方案 |
|---|------|---------|------|---------|
| 1 | avgSpreadRatio_ori 使用 SMA 而非 EMA | **高** | ✅ 已修复 | 添加 ALPHA 参数，使用 EMA |
| 2 | ALPHA 参数缺失 | **高** | ✅ 已修复 | 添加 alpha 到配置文件 |
| 3 | avgPx 保存时使用 SMA 均值 | 中 | ✅ 已修复 | 现在保存 avgSpreadRatio_ori |
| 4 | ytd2 初始化时未同步 NetPosAgg | 中 | ✅ 已修复 | 初始化时同步 |
| 5 | setDynamicThresholds 使用 leg1Position 而非 NetPosPass | 中 | ✅ 已修复 | 改为使用 firstStrat.NetPosPass |

---

## 测试验证方法

### 1. 单元测试

```go
// daily_init_test.go
func TestDailyInitRoundTrip(t *testing.T) {
    // 1. 创建策略，设置初始状态
    // 2. 运行一段时间，产生成交
    // 3. 停止策略，保存 daily_init
    // 4. 重新启动策略，加载 daily_init
    // 5. 验证所有字段恢复正确
}
```

### 2. 端到端测试

```bash
# scripts/test/feature/test_daily_init_persistence.sh
# 1. 启动系统，运行策略
# 2. 记录当前状态（avgPx, ytd1, ytd2）
# 3. 优雅停止
# 4. 检查 daily_init 文件内容
# 5. 重启系统
# 6. 验证状态恢复正确
```

### 3. C++ 对照测试

```bash
# 使用相同的输入数据，对比 C++ 和 Go 的输出
# 1. 准备测试数据（行情序列）
# 2. 分别用 C++ 和 Go 运行
# 3. 对比 avgSpreadRatio_ori 的值
# 4. 对比 Z-Score 的值
```

---

## 代码审查清单

迁移 C++ 代码时，必须完成以下检查：

- [ ] 找到并引用 C++ 原代码（使用 `// C++:` 注释）
- [ ] 列出所有涉及的变量
- [ ] 对每个变量完成生命周期检查（初始化、更新、使用、保存）
- [ ] 对照算法公式（特别是 EMA vs SMA）
- [ ] 添加单元测试
- [ ] 运行端到端测试验证

---

## 参考资料

- C++ 原代码: `/Users/user/PWorks/RD/tbsrc/Strategies/`
- Go 代码: `/Users/user/PWorks/RD/quantlink-trade-system/golang/pkg/strategy/`
- daily_init 文档: `docs/功能实现/daily_init文件分析_2026-02-10-17_10.md`

---

**最后更新**: 2026-02-12
