# QuantlinkTrader 项目开发规则

## 项目概述

QuantlinkTrader 是一个高性能量化交易系统，采用 C++ 网关 + Golang 策略引擎的混合架构。

**关键文档**:
- 文档索引中心: @docs/README.md
- 部署指南: @docs/核心文档/DEPLOY_GUIDE_2026-02-24.md
- 架构设计: @docs/系统分析/tbsrc-golang_v2_架构更新_2026-02-13-16_00.md
- MWMR 技术规格: @docs/系统分析/hftbase_MWMR_Go复刻技术规格_2026-02-13-16_00.md

---

## ⚠️ 变更管理规则（强制）

**所有代码和配置变更必须通过 OpenSpec (opsx) 工作流进行管理。**

### 流程要求

1. **开始变更前**: 使用 `/opsx:new` 创建新的 change
2. **推进变更**: 使用 `/opsx:continue` 逐步创建 artifacts（proposal → design → specs → tasks）
3. **实施变更**: 使用 `/opsx:apply` 按 tasks 执行实施
4. **验证变更**: 使用 `/opsx:verify` 验证实施完整性
5. **归档变更**: 使用 `/opsx:archive` 归档已完成的 change

### 快速流程（简单变更）

对于明确的小变更，可使用 `/opsx:ff` 一次性生成所有 artifacts，然后直接实施和归档。

### 禁止事项

- ❌ 不经过 opsx 直接修改代码
- ❌ 实施完成后不归档 change
- ❌ 跳过 artifact 创建直接写代码

### 豁免情况

- 纯调试操作（查看日志、检查状态等不修改代码的操作）
- 紧急 hotfix（事后补建 change 并归档）

---

## ⚠️ 代码库位置定义（重要）

**必须区分以下两个代码库，绝对不可混淆！**

### C++ 原代码（旧系统 - 迁移源）

| 项目 | 路径 | 说明 |
|------|------|------|
| **tbsrc** | `/Users/user/PWorks/RD/tbsrc/` | C++ 原始交易系统（策略层） |
| **hftbase** | `/Users/user/PWorks/RD/hftbase/` | HFT 基础设施库 |
| **ors** | `/Users/user/PWorks/RD/ors/` | 订单路由服务 |

**tbsrc 目录结构**:
| 子目录 | 说明 |
|--------|------|
| `tbsrc/Strategies/` | 原始策略实现 |
| `tbsrc/Strategies/include/` | 策略类定义 |
| `tbsrc/common/` | 公共组件 |
| `tbsrc/main/` | 主程序 |

**关键 C++ 原代码文件**:
- `tbsrc/Strategies/PairwiseArbStrategy.cpp` - 配对套利策略
- `tbsrc/Strategies/ExecutionStrategy.cpp` - 执行策略基类
- `tbsrc/Strategies/include/ExecutionStrategy.h` - 策略头文件
- `hftbase/` - HFT 基础库（行情处理、订单管理等）
- `ors/` - 订单路由服务

### 新代码（当前项目 - 迁移目标）

| 项目 | 路径 | 说明 |
|------|------|------|
| **quantlink-trade-system** | `/Users/user/PWorks/RD/quantlink-trade-system/` | 新系统（迁移目标） |
| Golang 策略 | `tbsrc-golang/pkg/strategy/` | Go 策略实现 |
| C++ 网关（新写） | `gateway/` | 新写的 C++ 网关代码 |

**注意**: `quantlink-trade-system/gateway/` 下的 C++ 代码是**新写的网关代码**，**不是原代码**！

### 迁移对照表

| C++ 原代码 (tbsrc) | Go 新代码 (quantlink-trade-system) |
|-------------------|-----------------------------------|
| `tbsrc/Strategies/PairwiseArbStrategy.cpp` | `tbsrc-golang/pkg/strategy/pairwise_arb_strategy.go` |
| `tbsrc/Strategies/ExecutionStrategy.cpp` | `tbsrc-golang/pkg/strategy/base_strategy.go` |
| `tbsrc/Strategies/include/ExecutionStrategy.h` | `tbsrc-golang/pkg/strategy/types.go` |

### 搜索原代码的正确方式

```bash
# ✅ 正确：在原代码目录中搜索
grep -r "m_netpos_pass_ytd" /Users/user/PWorks/RD/tbsrc/
grep -r "某关键字" /Users/user/PWorks/RD/hftbase/
grep -r "某关键字" /Users/user/PWorks/RD/ors/

# ❌ 错误：在 quantlink-trade-system 中搜索（这里是新代码）
grep -r "m_netpos_pass_ytd" /Users/user/PWorks/RD/quantlink-trade-system/
```

**原代码根目录汇总**:
- `/Users/user/PWorks/RD/tbsrc/` - 策略和交易逻辑
- `/Users/user/PWorks/RD/hftbase/` - HFT 基础设施
- `/Users/user/PWorks/RD/ors/` - 订单路由服务

---

## 系统架构

### 核心组件

1. **C++ 网关层** (`gateway/`)
   - `md_shm_feeder`: 行情 SHM 注入器（支持 simulator / CTP 模式），写入 SysV MWMR SHM
   - `counter_bridge`: 统一成交网关（支持 CTP/Simulator 插件），读取订单 SHM、写入回报 SHM

2. **Golang 策略层** (`tbsrc-golang/`)
   - `cmd/trader/`: 策略引擎主程序（直接读写 SysV SHM）
   - `pkg/shm/`: SysV SHM + MWMR Queue 封装
   - `pkg/connector/`: Connector（SHM 轮询 + 发单/撤单）
   - `pkg/common/`: CommonClient（回调分发）
   - `pkg/strategy/`: 策略实现（ExecutionStrategy、PairwiseArbStrategy 等）
   - `pkg/indicator/`: 技术指标库
   - `pkg/config/`: 配置加载

3. **通信机制**
   - SysV MWMR 共享内存: 所有进程间通信（行情、订单、回报）
   - SHM Key 分配: `0x1001`（行情）、`0x2001`（订单请求）、`0x3001`（订单回报）、`0x4001`（ClientStore）

### 数据流向

```
md_shm_feeder → [SysV MWMR SHM 0x1001] → Go trader → [SysV MWMR SHM 0x2001] → counter_bridge → CTP/Simulator
                                                    ← [SysV MWMR SHM 0x3001] ← (回报)
```

---

## 代码风格规范

### C++ 代码 (`gateway/`)

- **风格指南**: 遵循 Google C++ Style Guide
- **命名规范**:
  - 类名: PascalCase (例如: `MarketDataGateway`)
  - 函数名: camelCase (例如: `processMarketData()`)
  - 成员变量: `m_` 前缀 (例如: `m_isRunning`)
  - 常量: UPPER_SNAKE_CASE (例如: `MAX_QUEUE_SIZE`)
- **头文件**:
  - 使用 `#pragma once` 而不是 include guards
  - 头文件包含顺序: 标准库 → 第三方库 → 项目头文件
- **共享内存**:
  - 使用 SysV `shmget` / `shmat`（整数 key）
  - MWMR 队列格式: 与 hftbase 二进制兼容

### Golang 代码 (`tbsrc-golang/`)

- **风格指南**: 使用 `gofmt` 自动格式化
- **包命名**: 全小写，单数形式 (例如: `trader`, `strategy`, `risk`)
- **接口命名**:
  - 单方法接口以 `-er` 结尾 (例如: `Reader`, `Writer`)
  - 多方法接口使用描述性名称 (例如: `StrategyEngine`)
- **错误处理**:
  - 总是检查 error 返回值
  - 使用 `log.Printf` 记录错误，不要 panic（除非初始化失败）
- **日志格式**: `log.Printf("[模块名] 消息内容")`

### Java 代码 (`tbsrc-java/`) — C++ → Java 翻译原则

**核心目标**: Java 代码必须与 C++ 原代码保持最大程度的一致性，使开发者能够在 C++ 和 Java 之间轻松对照。

**原则 1: 文件结构与 C++ 保持一致**
- Java 包结构必须映射 C++ 目录结构
- 示例: `tbsrc/Strategies/` → `com/quantlink/trader/strategy/`
- 示例: `tbsrc/common/` → `com/quantlink/trader/common/`
- 不得随意拆分或合并 C++ 中的文件

**原则 2: 文件名与 C++ 尽可能保持一致**
- C++ 文件名直接作为 Java 类名（PascalCase）
- 示例: `ExecutionStrategy.cpp` → `ExecutionStrategy.java`
- 示例: `PairwiseArbStrategy.cpp` → `PairwiseArbStrategy.java`
- 示例: `ExtraStrategy.cpp` → `ExtraStrategy.java`
- 头文件中的结构体/类型定义拆为独立 Java 文件时，保留原名

**原则 3: 方法名与 C++ 尽可能保持一致**
- C++ 方法名直接驼峰化作为 Java 方法名
- 示例: `SendOrder()` → `sendOrder()`
- 示例: `MDCallBack()` → `mdCallBack()`
- 示例: `ORSCallBack()` → `orsCallBack()`
- 示例: `SetThresholds()` → `setThresholds()`
- 示例: `SendAggressiveOrder()` → `sendAggressiveOrder()`
- 不得自行重命名方法（如不能把 `SendOrder` 改为 `placeOrder`）

**原则 4: 变量名与 C++ 尽可能保持一致**
- C++ 成员变量去掉 `m_` 前缀后驼峰化
- 示例: `m_netpos` → `netpos`
- 示例: `m_netpos_pass` → `netposPass`
- 示例: `m_firstStrat` → `firstStrat`
- 局部变量保持原名驼峰化
- 不得自行重命名变量（如不能把 `m_netpos` 改为 `position`）

**原则 5: 方法必须加入 C++ 来源注释**
- 每个从 C++ 迁移的方法必须在方法上方注释说明来源
- 格式: `// 迁移自: tbsrc/Strategies/文件名.cpp:方法名() (行号)`
- 关键逻辑行使用 `// C++: 原始代码` 标注对应的 C++ 代码
- 示例:
  ```java
  // 迁移自: tbsrc/Strategies/PairwiseArbStrategy.cpp:SendAggressiveOrder() (L150-L220)
  private void sendAggressiveOrder() {
      // C++: auto long_place_diff_thold = m_thold_first->LONG_PLACE - m_thold_first->BEGIN_PLACE;
      double longPlaceDiff = firstThold.longPlace - firstThold.beginPlace;
      ...
  }
  ```

**原则 6: 无法对齐的逻辑必须详细注释说明**
- 当 Java 实现与 C++ 存在结构性差异时，必须加注释说明:
  - 差异是什么
  - 为什么无法完全对齐
  - Java 采用了什么替代方案
- 格式: `// [C++差异] 说明...`
- 示例:
  ```java
  // [C++差异] C++ 使用 function pointer callback (m_ORSCallBackFn)，
  // Java 使用 @Override 虚方法派发，语义等价但机制不同。
  // 参考: tbsrc/Strategies/include/ExecutionStrategy.h:85
  ```
- 常见差异场景:
  - 指针操作 → Java 引用
  - 宏定义 → Java 常量/方法
  - 模板 → Java 泛型
  - `friend class` → Java 包级访问
  - `union` → Java 多字段
  - `#ifdef` 条件编译 → Java 配置开关

**原则 7: 严禁 workaround，必须严格对齐 C++ 调用链路**
- C++ 中存在的回调、调用链路，Java 必须完整对齐实现，不得跳过或用 workaround 替代
- 如果 C++ 有 `A → B → C` 的调用链，Java 不得把 `C` 的逻辑直接塞进 `A` 里
- 缺失的中间层（如 `IndicatorCallBack`、`CommonClient` 回调注册等）必须先补齐，再按 C++ 原有链路调用
- 示例（禁止）:
  ```java
  // ❌ 错误: C++ 的 IndicatorCallBack → SetTargetValue → SendOrder → SetThresholds
  // 不得跳过 IndicatorCallBack，直接在 mdCallBack() 里调用 setTargetValue()
  public void mdCallBack(MemorySegment update) {
      // ... 行情处理 ...
      setTargetValue(currPrice, targetPrice, targetBidPNL, targetAskPNL); // ❌ workaround
  }
  ```
- 示例（正确）:
  ```java
  // ✅ 正确: 先在 CommonClient 中实现 indicatorCallBack 注册机制，
  // 然后在 main.cpp 对应的 TraderMain.java 中注册 indicatorCallBack，
  // 由 CommonClient 在正确时机触发，与 C++ 调用链路完全一致
  client.setIndicatorCallback(indicatorList -> {
      strategy.setTargetValue(currPrice, targetPrice, targetBidPNL, targetAskPNL);
  });
  ```
- 当发现 Java 缺少 C++ 中的某个回调/机制时，必须:
  1. 先找到 C++ 原代码中该机制的完整实现（`tbsrc/`、`hftbase/`、`ors/`）
  2. 向用户展示 C++ 原代码
  3. 在 Java 中对齐实现该机制
  4. 再按 C++ 链路完成后续调用

**原则 8: 严禁任何形式的省略或简化，必须完整迁移 C++ 逻辑**
- 迁移 C++ 代码时，**任何省略都必须先询问用户并获得明确确认**
- 不得自行判断"某逻辑在当前场景不需要"而省略
- 不得以"中国期货场景不使用"、"PairwiseArbStrategy 不需要"、"待补齐"等理由跳过 C++ 逻辑
- 不得自行标注"待补齐"/"TODO"来掩盖省略行为——省略就是省略，未经用户确认的省略一律禁止
- 后续会迁移其他策略和场景，所有 C++ 逻辑必须完整迁移
- **唯一允许省略的情况**: 用户明确确认可以省略，且必须在注释中标注"经用户确认"
- 如果遇到无法直接迁移的 C++ 逻辑，必须:
  1. **停下来，询问用户**，说明该段 C++ 逻辑是什么、为什么无法直接迁移
  2. **等待用户回复**，得到明确确认后才能省略或替代
  3. 在注释中标注 `[C++差异-用户确认]`，说明省略内容、原因、用户确认的替代方案
- 示例（禁止）:
  ```java
  // ❌ 错误: 自行判断简化
  // [C++差异] C++ 有 commonBook/selfBook 更新逻辑，
  // 以上简化均不影响 PairwiseArbStrategy 在中国期货场景的核心功能。

  // ❌ 错误: 自行标注"待补齐"来掩盖省略
  // [C++差异] C++ 包含 invisible book 逻辑，待补齐。

  // ❌ 错误: 自行判断不需要
  // C++: optionStrategy 相关逻辑省略（中国期货不使用）
  ```
- 示例（正确）:
  ```java
  // ✅ 正确: 完整迁移 C++ 逻辑，无省略

  // ✅ 正确: 经用户确认后的省略
  // [C++差异-用户确认] C++ 使用 MemLog SHM 推送监控数据（依赖 hftbase MemLog 库），
  // 经用户确认（2026-02-26），Java 使用日志输出替代。
  ```

### 配置文件 (`config/`)

- **格式**: YAML
- **命名**:
  - 每策略配置: `trader.{strategy_id}.yaml`（如 `trader.92201.yaml`）
  - 模拟器配置: `simulator.yaml`
  - CTP 配置: `ctp/ctp_md.secret.yaml`, `ctp/ctp_td.secret.yaml`
- **必填字段**:
  - `system.strategy_id`: 策略唯一标识
  - `strategy.symbols`: 交易品种列表
  - `shm.request_key`: 订单请求 SHM key
  - `shm.response_key`: 订单回报 SHM key
  - `shm.md_key`: 行情 SHM key

---

## C++ 代码迁移规则

本项目从 C++ 旧系统 (tbsrc) 迁移到 Golang 新系统。迁移代码时必须严格遵循以下规则。

### ⚠️ 原代码位置（重要）

**C++ 原代码路径（三个独立目录）**:
- `/Users/user/PWorks/RD/tbsrc/` - 策略和交易逻辑
- `/Users/user/PWorks/RD/hftbase/` - HFT 基础设施库
- `/Users/user/PWorks/RD/ors/` - 订单路由服务

```
/Users/user/PWorks/RD/
├── tbsrc/                         # 策略层原代码
│   ├── Strategies/                # 策略实现
│   │   ├── PairwiseArbStrategy.cpp
│   │   ├── ExecutionStrategy.cpp
│   │   ├── PairwiseArbETFStrategy.cpp
│   │   ├── PairwiseArbOptStrategy.cpp
│   │   └── include/
│   │       └── ExecutionStrategy.h
│   ├── common/                    # 公共组件
│   └── main/                      # 主程序
│
├── hftbase/                       # HFT 基础设施原代码
│   └── (行情处理、订单管理等)
│
└── ors/                           # 订单路由服务原代码
    └── (订单路由相关)
```

**注意**: `/Users/user/PWorks/RD/quantlink-trade-system/gateway/` 是**新写的 C++ 网关代码**，不是原代码！

### 强制要求

**规则 1: 禁止自设默认值**
- 迁移 C++ 代码时，所有参数必须从配置文件读取
- 不得自行设定默认值（如 `+ 1.5`、`* 0.3` 等）
- 如果 C++ 中参数来自配置，Go 中也必须来自配置

**规则 2: 必须先展示 C++ 原代码**
- 实现任何 C++ 功能迁移前，必须先找到并展示 C++ 原代码
- **原代码路径**: `/Users/user/PWorks/RD/tbsrc/`
- 在 `docs/cpp_reference/` 目录中保存关键 C++ 代码片段
- 如果找不到 C++ 原代码，必须向用户确认后再实现

**规则 3: 逐行对照注释**
- Go 代码中的关键逻辑必须在注释中写明对应的 C++ 代码
- 使用 `// C++:` 前缀标注原始 C++ 代码
- 注释中引用原代码时使用格式: `// 参考: tbsrc/Strategies/xxx.cpp:行号`

**规则 4: 架构差异必须提醒用户**
- 当 C++ 和 Go 的架构存在差异时（如继承 vs 组合、类结构不同等），**必须先提醒用户**
- 不得自行决定架构方案，必须向用户说明：
  - C++ 原代码的架构是什么
  - Go 代码当前的架构是什么
  - 两者的差异在哪里
  - 可选的解决方案有哪些
- 等待用户确认后再实施
- **示例**：C++ 中 `PairwiseArbStrategy` 继承自 `ExecutionStrategy`，但 Go 使用组合，这种差异需要提醒用户

### 代码注释格式

```go
// setDynamicThresholds 根据持仓动态调整入场阈值
// 与 C++ SetThresholds() 完全一致：
//
//   C++: auto long_place_diff_thold = m_thold_first->LONG_PLACE - m_thold_first->BEGIN_PLACE;
//   C++: auto short_place_diff_thold = m_thold_first->BEGIN_PLACE - m_thold_first->SHORT_PLACE;
//
func (pas *PairwiseArbStrategy) setDynamicThresholds() {
    // C++: auto long_place_diff_thold = LONG_PLACE - BEGIN_PLACE
    longPlaceDiff := pas.longZScore - pas.beginZScore
    // C++: auto short_place_diff_thold = BEGIN_PLACE - SHORT_PLACE
    shortPlaceDiff := pas.beginZScore - pas.shortZScore
    ...
}
```

### 参数映射表

迁移时必须维护 C++ 与 Go 的参数映射关系：

| C++ 参数 | Go 参数 | 配置文件字段 | 原代码位置 |
|---------|--------|-------------|-----------|
| `BEGIN_PLACE` | `beginZScore` | `begin_zscore` | ExecutionStrategy.h |
| `LONG_PLACE` | `longZScore` | `long_zscore` | ExecutionStrategy.h |
| `SHORT_PLACE` | `shortZScore` | `short_zscore` | ExecutionStrategy.h |
| `BEGIN_REMOVE` | `exitZScore` | `exit_zscore` | ExecutionStrategy.h |
| `m_netpos_pass` | `leg1Position` | - | ExecutionStrategy.h:112 |
| `m_netpos_pass_ytd` | `leg1YtdPosition` | - | ExecutionStrategy.h:113 |
| `avgSpreadRatio_ori` | `spreadAnalyzer.Mean` | - | PairwiseArbStrategy.cpp:31 |
| `tValue` | `tValue` | `t_value` | PairwiseArbStrategy.cpp |

### C++ 参考代码目录

关键 C++ 代码保存在 `docs/cpp_reference/` 目录：
- `SetThresholds.cpp` - 动态阈值调整逻辑
- `SendAggressiveOrder.cpp` - 主动追单逻辑
- `ExecutionStrategy.cpp` - 执行策略基类
- `README.md` - 索引和说明

### C++ 原代码关键文件速查

| 功能 | 原代码文件 | 行号 |
|------|-----------|------|
| 策略初始化 | `tbsrc/Strategies/PairwiseArbStrategy.cpp` | 7-84 |
| 昨仓初始化 | `tbsrc/Strategies/PairwiseArbStrategy.cpp` | 33-38 |
| 动态阈值 | `tbsrc/Strategies/ExecutionStrategy.cpp` | SetThresholds() |
| 主动追单 | `tbsrc/Strategies/PairwiseArbStrategy.cpp` | SendAggressiveOrder() |
| 成交回调 | `tbsrc/Strategies/PairwiseArbStrategy.cpp` | ORSCallBack() |
| 持仓字段定义 | `tbsrc/Strategies/include/ExecutionStrategy.h` | 111-114 |

### 代码审查清单

迁移 C++ 代码的 PR 必须包含以下检查项：
- [ ] 已找到并引用 C++ 原代码
- [ ] 无自定义默认值（所有参数来自配置）
- [ ] 注释中包含 C++ 对照（使用 `// C++:` 前缀）
- [ ] 测试用例数据来自 C++ 运行结果
- [ ] 已更新参数映射表（如有新参数）
- [ ] 已在 `docs/cpp_reference/` 保存 C++ 代码片段

---

## 文档规范

### 文档存放位置

**规则 1: 文档目录结构（2026-02-24 更新）**

文档已按主题分类，根目录仅保留 `README.md` 索引文件。

**活跃目录**:
- `docs/核心文档/`: 部署指南等核心文档
- `docs/实盘/`: 实盘部署、CTP 相关
- `docs/回测/`: 回测系统使用、参数优化
- `docs/功能实现/`: Phase2-9 实施计划、C++ 对照等
- `docs/系统分析/`: 架构分析、MWMR 技术规格、代码对比
- `docs/gateway/`: Gateway 模块文档
- `docs/java迁移/`: Java 迁移评估、设计、实施文档

**已清空/归档目录**:
- `docs/golang/`: 已全部归档（旧 `golang/` 代码库文档）
- `docs/测试报告/`: 已全部归档（旧架构测试报告）
- `docs/archive/`: 已归档旧文档（131 个）

**IMPORTANT: 创建新文档时的放置规则**:
1. **实盘部署、问题修复** → `docs/实盘/`
2. **回测系统、参数优化** → `docs/回测/`
3. **新功能实施** → `docs/功能实现/`
4. **系统分析、架构设计** → `docs/系统分析/`
5. **Gateway实现** → `docs/gateway/`
6. **Java 迁移相关** → `docs/java迁移/`
7. **过时文档** → `docs/archive/`
8. **核心文档更新** → 谨慎操作，这些是长期维护的基础文档

**目录结构**:
```
quantlink-trade-system/
├── docs/                                    # 文档根目录
│   ├── README.md                            # 文档索引中心（必看）
│   ├── 核心文档/                            # 系统核心文档
│   │   └── DEPLOY_GUIDE_2026-02-24.md       # 部署指南（SysV MWMR SHM 架构）
│   ├── 实盘/                                # 实盘交易相关
│   ├── 回测/                                # 回测系统文档
│   ├── 功能实现/                            # Phase2-9 实施计划等
│   ├── 系统分析/                            # 架构分析、MWMR 规格、代码对比
│   ├── gateway/                             # Gateway模块文档
│   ├── java迁移/                            # Java 迁移评估、设计、实施
│   └── archive/                             # 已归档文档（131个）
│
├── gateway/                                 # C++ 网关代码（md_shm_feeder, counter_bridge 等）
│   └── src/
├── tbsrc-golang/                            # Golang 策略代码（活跃，SysV SHM 直连）
│   └── pkg/
└── deploy_new/                              # 编译部署产物（由 build_deploy_new.sh 生成）
```

**查找文档**:
- 首先查看 `docs/README.md` 获取完整索引和导航
- 按主题进入相应目录查找具体文档
- 使用 `find docs -name "*关键词*"` 搜索特定文档

### 文档命名规范

**规则 2: 文档命名格式（2026-02-25 更新）**

**格式**: `YYYY-MM-DD-HH_mm_模块_摘要.md`

**组成部分**:
- **时间戳**: `YYYY-MM-DD-HH_mm` 格式（24 小时制），**放在最前面**
- **模块**: 文档所属模块或主题（小写或驼峰）
  - 单模块: `gateway`, `golang`, `java`, `config`, `strategy`, `risk` 等
  - 多模块/系统级: `QuantlinkTrader`, `系统`, `项目`, `部署` 等
- **摘要**: 简短描述文档内容（2-5 个字，中文）

**命名示例**:

```bash
# ✅ 正确示例（时间在前）
docs/2026-02-25-10_00_系统_架构设计.md                       # 通用文档
docs/2026-02-25-14_30_部署_生产环境指南.md                   # 通用文档

docs/gateway/2026-02-25-09_20_gateway_共享内存优化.md       # Gateway 模块文档
docs/java迁移/2026-02-25-10_00_java_迁移评估.md             # Java 迁移文档

# ❌ 错误示例
docs/test_report.md                                         # 缺少时间戳
docs/模块_摘要_2026-01-24-15_32.md                          # 时间在后面（旧格式）
docs/EndToEndTest_2026-01-24.md                            # 使用英文，缺少时分
gateway/docs/gateway_共享内存优化.md                         # 错误位置
```

### 文档内容规范

**规则 3: 使用中文编写**

- **文档正文**: 必须使用中文
- **标题**: 使用中文
- **注释**: 使用中文
- **例外情况**:
  - 代码片段: 保持原始语言（C++/Golang/Shell 等）
  - 命令示例: 保持原始命令
  - 技术术语: 可保留英文，但首次出现时提供中文说明
  - 文件路径: 保持原始路径
  - URL 链接: 保持原始链接

**示例**:

```markdown
# ✅ 正确示例

## 策略引擎设计

策略引擎（Strategy Engine）负责接收市场数据并生成交易信号。

### 核心接口

\`\`\`go
type StrategyEngine interface {
    Start() error
    Stop() error
}
\`\`\`

### 运行方式

执行以下命令启动策略引擎：

\`\`\`bash
./bin/trader -config config/trader.yaml
\`\`\`

---

# ❌ 错误示例

## Strategy Engine Design

The strategy engine is responsible for receiving market data...
```

### 文档模板

**标准文档模板**:

```markdown
# 模块名_文档标题

**文档日期**: YYYY-MM-DD
**作者**: [作者名]
**版本**: v1.0
**相关模块**: [模块列表]

---

## 概述

[简要描述文档目的和背景]

## 详细内容

### 章节 1

[内容...]

### 章节 2

[内容...]

## 总结

[总结要点]

## 参考资料

- 相关文档1: @docs/xxx.md
- 相关文档2: @gateway/docs/xxx.md

---

**最后更新**: YYYY-MM-DD HH:mm
```

### 文档维护

**规则 4: 文档生命周期**

- **创建文档**: 重大功能、架构变更、测试报告时创建
- **更新文档**: 不修改原文件，而是创建新版本（带新时间戳）
- **废弃文档**: 移动到 `docs/archive/` 目录
- **文档索引**: 在 `docs/README.md` 中维护文档索引（重要文档需要添加）

**IMPORTANT: 文档索引维护**

当创建重要文档时，需要更新 `docs/README.md` 中的相应章节：

1. **核心文档** - 很少更新，除非重大架构变更
2. **实盘相关** - 添加重大问题修复、新功能实施
3. **回测相关** - 添加新的使用指南或重大优化
4. **测试报告** - 添加重大测试报告（里程碑式的）
5. **功能实现** - 添加新功能实施报告
6. **系统分析** - 添加重要的架构分析文档

**文档索引结构** (`docs/README.md`):

```markdown
# QuantLink Trade System - 文档中心

## 📁 文档目录结构
[显示10个主题目录]

## 🚀 快速开始
[新用户必读、开发者指南]

## 📚 主题文档导航

### 💼 实盘交易
**目录**: `docs/实盘/`
**最新文档**:
- ✅ [Phase2-5_完整持仓管理功能实施报告](实盘/Phase2-5_完整持仓管理功能实施报告_2026-01-30-11_35.md)
- ✅ [参数加载修复报告](实盘/参数加载修复报告_2026-01-30-11_05.md)

### 🧪 回测系统
[回测相关文档导航]

### [其他主题...]

## 🔍 常见问题速查
[问题 → 参考文档映射表]
```

---

## 脚本管理规范

### 脚本目录结构

**规则 5: scripts/ 目录组织（2026-01-30 重组）**

所有脚本按功能分类存放在 `scripts/` 目录下，根目录不应存放任何 .sh 脚本文件。

**目录结构**:
```
scripts/
├── README.md                      # 脚本使用指南
│
├── 构建脚本 (根目录)
│   ├── build_gateway.sh          # 编译 C++ Gateway
│   ├── build_golang.sh           # 编译 Golang Trader
│   └── generate_proto.sh         # 生成 Protobuf 代码
│
├── 依赖安装 (根目录)
│   ├── install_dependencies.sh   # 安装系统依赖
│   └── install_nats_c.sh         # 安装 NATS C 客户端
│
├── 部署脚本 (根目录)
│   ├── prepare_deploy.sh         # 准备部署环境
│   └── quick_deploy.sh           # 快速部署
│
├── test/                          # 测试脚本
│   ├── e2e/                      # 端到端测试
│   │   ├── test_full_chain.sh
│   │   ├── test_ctp_e2e.sh
│   │   └── ...
│   ├── integration/              # 集成测试
│   │   ├── test_multi_strategy_*.sh
│   │   └── ...
│   ├── unit/                     # 单元测试
│   │   ├── test_ctp_*.sh
│   │   └── ...
│   └── feature/                  # 功能测试
│       ├── test_position_*.sh
│       └── ...
│
├── live/                         # 实盘脚本
│   ├── start_live_test.sh
│   ├── monitor_live.sh
│   └── ...
│
├── trading/                      # 交易操作
│   ├── trade_*.sh
│   ├── query_position.sh
│   └── ...
│
└── backtest/                     # 回测脚本
    └── run_backtest.sh
```

### 脚本分类规则

**规则 6: 脚本存放位置**

新建脚本时按以下规则分类：

1. **构建和安装脚本** → `scripts/` 根目录
   - 编译脚本（build_*.sh）
   - 依赖安装（install_*.sh）
   - 代码生成（generate_*.sh）

2. **测试脚本** → `scripts/test/` 子目录
   - 端到端测试 → `test/e2e/`
   - 集成测试 → `test/integration/`
   - 单元测试 → `test/unit/`
   - 功能测试 → `test/feature/`

3. **实盘脚本** → `scripts/live/`
   - 启动脚本（start_*.sh）
   - 监控脚本（monitor_*.sh）
   - 实盘测试和部署

4. **交易操作脚本** → `scripts/trading/`
   - 下单脚本（trade_*.sh）
   - 平仓脚本（close_*.sh）
   - 查询脚本（query_*.sh）

5. **回测脚本** → `scripts/backtest/`
   - 回测运行和分析脚本

6. **部署脚本** → `scripts/` 根目录
   - 部署相关的高层脚本

### 脚本命名规范

**规则 7: 脚本命名格式**

**格式**: `<动词>_<对象>_<描述>.sh`

**命名模式**:
- **测试脚本**: `test_<功能>_<类型>.sh`
  - 示例: `test_ctp_e2e.sh`, `test_position_query.sh`
- **启动脚本**: `start_<服务>_<模式>.sh`
  - 示例: `start_live_test.sh`, `start_full_test.sh`
- **停止脚本**: `stop_<服务>.sh`
  - 示例: `stop_ctp_e2e.sh`, `stop_all.sh`
- **监控脚本**: `monitor_<对象>.sh`
  - 示例: `monitor_live.sh`, `monitor_health.sh`
- **构建脚本**: `build_<模块>.sh`
  - 示例: `build_gateway.sh`, `build_golang.sh`
- **安装脚本**: `install_<依赖>.sh`
  - 示例: `install_dependencies.sh`, `install_nats_c.sh`
- **交易操作**: `<动作>_<标的>.sh`
  - 示例: `trade_ag2603.sh`, `close_ag2603.sh`, `query_position.sh`

**禁止使用模糊命名**:
- ❌ `test.sh`, `run.sh`, `script.sh`
- ✅ `test_full_chain.sh`, `run_backtest.sh`

### 脚本开发规范

**规则 8: 脚本标准模板**

```bash
#!/bin/bash
set -e  # 遇到错误立即退出

# ============================================
# 脚本名称: <脚本名>.sh
# 用途: <简要说明脚本用途>
# 作者: <作者名>
# 日期: YYYY-MM-DD
# ============================================

# 获取项目根目录
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"

# 颜色定义（可选）
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# 日志函数
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

# 清理函数（捕获退出信号）
cleanup() {
    log_info "Cleaning up..."
    # 清理临时文件、进程等
}

trap cleanup EXIT

# 主逻辑
main() {
    log_info "Starting <脚本功能>..."

    # 脚本主要逻辑

    log_info "Completed successfully"
}

main "$@"
```

**关键要求**:
1. 必须使用 `set -e` 确保错误时退出
2. 必须使用 `PROJECT_ROOT` 定位项目根目录
3. 使用日志函数（log_info/log_warn/log_error）而非直接 echo
4. 使用 `trap cleanup EXIT` 确保资源清理
5. 将主逻辑放在 `main()` 函数中

### 脚本文档要求

**规则 9: scripts/README.md 维护**

当添加重要脚本时，更新 `scripts/README.md` 中的相应章节：

```markdown
## 📂 目录结构
[更新目录树]

## 🚀 常用脚本
[添加新脚本的使用说明]
```

**重要脚本定义**:
- 新功能的测试脚本
- 实盘部署相关脚本
- 运维监控脚本
- 开发者常用脚本

**临时脚本**:
- 一次性使用的临时脚本可以不加入 README.md
- 但仍需遵循命名规范和放置规则

### 脚本使用权限

**规则 10: 脚本执行权限**

```bash
# 新建脚本后设置执行权限
chmod +x scripts/<category>/<script_name>.sh

# 批量设置脚本权限
find scripts/ -name "*.sh" -exec chmod +x {} \;
```

### 常见问题

**问题 1: 脚本放在哪里？**

| 脚本用途 | 存放位置 |
|---------|---------|
| 编译构建 | `scripts/` (根目录) |
| 端到端测试 | `scripts/test/e2e/` |
| 集成测试 | `scripts/test/integration/` |
| 单元测试 | `scripts/test/unit/` |
| 功能测试 | `scripts/test/feature/` |
| 实盘测试 | `scripts/live/` |
| 交易操作 | `scripts/trading/` |
| 回测 | `scripts/backtest/` |

**问题 2: 如何从脚本中访问项目文件？**

```bash
# 方法1: 使用 PROJECT_ROOT（推荐）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$PROJECT_ROOT"
./bin/trader -config config/trader.yaml

# 方法2: 使用相对路径（不推荐，容易出错）
../../bin/trader -config ../../config/trader.yaml
```

**问题 3: 脚本之间如何相互调用？**

```bash
# 使用绝对路径（推荐）
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
"${PROJECT_ROOT}/scripts/build_gateway.sh"
"${PROJECT_ROOT}/scripts/build_golang.sh"

# 或使用 source（共享变量和函数）
source "${PROJECT_ROOT}/scripts/common_functions.sh"
```

---

## 文档与脚本关联规范

### 关联目标

**规则 11: 建立文档与脚本的双向索引**

确保开发者能够：
- 从文档快速找到相关测试脚本
- 从脚本快速找到相关说明文档
- 理解功能的完整上下文（文档 + 实现 + 测试）

### 关联方式

#### 方式 1: 脚本头部引用文档（推荐）

**规则 12: 脚本必须在头部注释中引用相关文档**

```bash
#!/bin/bash
set -e

# ============================================
# 脚本名称: test_position_persistence.sh
# 用途: 测试持仓持久化功能
# 作者: QuantLink Team
# 日期: 2026-01-30
#
# 相关文档:
#   - 实施报告: @docs/实盘/Phase2-5_完整持仓管理功能实施报告_2026-01-30-11_35.md
#   - 功能文档: @docs/功能实现/持仓查询功能实现_2026-01-28-11_30.md
#   - 架构说明: @docs/核心文档/CURRENT_ARCHITECTURE_FLOW.md
#   - 使用指南: @docs/核心文档/USAGE.md
# ============================================
```

**引用规则**:
- 使用 `@docs/` 前缀表示文档路径
- 按重要性排序（最相关的放最前面）
- 至少引用 1 个相关文档
- 复杂脚本建议引用 2-4 个文档

#### 方式 2: 文档中引用脚本

**规则 13: 文档必须在操作说明中引用相关脚本**

```markdown
## 测试验证

### 持仓持久化测试

运行以下脚本验证持仓持久化功能：

\`\`\`bash
# 完整测试
./scripts/test/feature/test_position_persistence.sh

# 查询测试
./scripts/test/feature/test_position_query.sh
\`\`\`

### 实盘测试

\`\`\`bash
# 启动实盘测试
./scripts/live/start_live_test.sh

# 监控运行状态
./scripts/live/monitor_live.sh
\`\`\`

**相关脚本**:
- 持仓持久化: `scripts/test/feature/test_position_persistence.sh`
- 持仓查询: `scripts/test/feature/test_position_query.sh`
- 实盘启动: `scripts/live/start_live_test.sh`
```

**引用规则**:
- 在"测试验证"、"使用示例"等章节引用脚本
- 提供完整的脚本路径（从项目根目录开始）
- 添加简要说明脚本的用途
- 按使用频率排序（常用脚本放前面）

#### 方式 3: 交叉索引文件

**规则 14: 维护中央交叉索引文件**

**文件位置**: `CROSS_REFERENCE.md`

**索引结构**:
```markdown
# 文档与脚本交叉索引

## 按功能分类

| 功能 | 脚本 | 相关文档 |
|------|------|---------|
| 持仓管理 | scripts/test/feature/test_position_persistence.sh | Phase2-5_完整持仓管理功能实施报告.md |

## 按文档分类

### Phase2-5_完整持仓管理功能实施报告
**相关脚本**:
- scripts/test/feature/test_position_persistence.sh
- scripts/test/feature/test_position_query.sh

## 快速查找

### 我想测试某个功能
| 需求 | 脚本 |
|------|------|
| 测试持仓管理 | scripts/test/feature/test_position_query.sh |

### 脚本出错应该查看哪个文档
| 脚本 | 排查文档 |
|------|---------|
| test_position_*.sh | Phase2-5_完整持仓管理功能实施报告.md |
```

### 关联维护规则

**规则 15: 关联信息的维护责任**

| 场景 | 维护内容 | 责任人 |
|------|---------|--------|
| **新建脚本** | 1. 脚本头部添加相关文档链接<br>2. 更新 CROSS_REFERENCE.md | 脚本作者 |
| **新建重要文档** | 1. 文档中引用相关脚本<br>2. 更新 CROSS_REFERENCE.md | 文档作者 |
| **脚本重命名/移动** | 1. 更新所有引用此脚本的文档<br>2. 更新 CROSS_REFERENCE.md | 重构人员 |
| **文档重命名/移动** | 1. 更新所有引用此文档的脚本<br>2. 更新 CROSS_REFERENCE.md | 重构人员 |
| **定期审查** | 1. 检查链接有效性<br>2. 清理过期引用 | 项目维护者 |

**更新时机**:
- ✅ 创建脚本后立即添加文档引用
- ✅ 完成重要文档后添加脚本引用
- ✅ 重构文件结构后更新所有引用
- ✅ 每月审查一次 CROSS_REFERENCE.md

### 关联完整性检查

**规则 16: 关联完整性要求**

**重要脚本必须关联文档**:
```bash
# 检查脚本是否缺少文档引用
find scripts/ -name "*.sh" -type f | while read script; do
    if ! grep -q "相关文档:" "$script"; then
        echo "WARNING: $script 缺少文档引用"
    fi
done
```

**重要文档必须关联脚本**:
- 实施报告类文档：必须引用测试脚本
- 功能实现文档：必须引用测试脚本
- 使用指南文档：必须引用操作脚本

**豁免条件**:
- 纯理论分析文档（无实现）
- 临时测试脚本（一次性使用）
- 工具脚本（通用工具，无特定文档）

### 关联示例

#### 示例 1: 持仓管理功能

**脚本**: `scripts/test/feature/test_position_persistence.sh`
```bash
# 相关文档:
#   - @docs/实盘/Phase2-5_完整持仓管理功能实施报告_2026-01-30-11_35.md
#   - @docs/功能实现/持仓查询功能实现_2026-01-28-11_30.md
```

**文档**: `docs/实盘/Phase2-5_完整持仓管理功能实施报告_2026-01-30-11_35.md`
```markdown
## 测试验证

\`\`\`bash
./scripts/test/feature/test_position_persistence.sh
./scripts/test/feature/test_position_query.sh
\`\`\`
```

**索引**: `CROSS_REFERENCE.md`
```markdown
| 持仓管理 | scripts/test/feature/test_position_persistence.sh | Phase2-5报告 |
```

#### 示例 2: CTP 对接功能

**脚本**: `scripts/test/e2e/test_ctp_e2e.sh`
```bash
# 相关文档:
#   - @docs/功能实现/任务1_CTP行情接入实施指南_2026-01-26-15_40.md
#   - @docs/实盘/CTP_POSITION_GUIDE.md
#   - @docs/测试报告/端到端测试报告_20260130_002214.md
```

**文档**: `docs/功能实现/任务1_CTP行情接入实施指南_2026-01-26-15_40.md`
```markdown
## 测试方法

\`\`\`bash
# 端到端测试
./scripts/test/e2e/test_ctp_e2e.sh

# 单元测试
./scripts/test/unit/test_ctp_account.sh
./scripts/test/unit/test_ctp_query.sh
\`\`\`
```

### 快速查找指南

**从功能找脚本**:
```bash
# 查看 CROSS_REFERENCE.md 的"我想测试某个功能"表格
cat CROSS_REFERENCE.md | grep -A 20 "我想测试某个功能"
```

**从文档找脚本**:
```bash
# 查看 CROSS_REFERENCE.md 的"按文档分类"章节
cat CROSS_REFERENCE.md | grep -A 50 "按文档分类"
```

**从脚本找文档**:
```bash
# 查看脚本头部的"相关文档"注释
head -20 scripts/test/feature/test_position_persistence.sh | grep -A 5 "相关文档"
```

**脚本出错找排查文档**:
```bash
# 查看 CROSS_REFERENCE.md 的"脚本出错应该查看哪个文档"表格
cat CROSS_REFERENCE.md | grep -A 20 "脚本出错"
```

### 关联效益

建立文档与脚本关联后：

✅ **提高可维护性**:
- 修改功能时能快速找到所有相关文件
- 避免遗漏测试脚本或文档更新

✅ **加速问题排查**:
- 脚本出错时快速定位排查文档
- 文档描述不清时快速找到测试用例

✅ **改善协作效率**:
- 新成员能快速理解功能全貌
- 交接工作时信息完整不遗漏

✅ **保证文档质量**:
- 强制文档与实现保持同步
- 文档必须包含可执行的测试方法

---

## 开发工作流

### 构建系统

```bash
# 一键编译部署（推荐）
./scripts/build_deploy_new.sh           # 完整编译 → deploy_new/
./scripts/build_deploy_new.sh --go      # 仅 Go
./scripts/build_deploy_new.sh --cpp     # 仅 C++
./scripts/build_deploy_new.sh --clean   # 清理后重编译

# 手动编译 C++ 网关
cd gateway && mkdir -p build && cd build
cmake -DCMAKE_BUILD_TYPE=Release ..
make -j4 md_shm_feeder counter_bridge

# 手动编译 Go 策略
cd tbsrc-golang
go build -o ../bin/trader ./cmd/trader/main.go
go build -o ../bin/webserver ./cmd/webserver/main.go
```

### 运行测试

**端到端测试** (推荐):
```bash
# 1. 编译部署
./scripts/build_deploy_new.sh

# 2. 启动网关（模拟模式）
cd deploy_new
./scripts/start_gateway.sh sim

# 3. 启动策略
./scripts/start_strategy.sh 92201

# 4. 查看日志
tail -f nohup.out.92201

# 5. 停止所有
./scripts/stop_all.sh
```

**单元测试**:
```bash
cd tbsrc-golang
go test ./pkg/...
```

### 调试方法

**查看日志**:
```bash
# 策略日志
tail -f deploy_new/nohup.out.92201

# 网关日志
tail -f deploy_new/log/md_shm_feeder.$(date +%Y%m%d).log
tail -f deploy_new/log/counter_bridge.$(date +%Y%m%d).log
```

**检查进程状态**:
```bash
ps aux | grep -E "md_shm_feeder|counter_bridge|trader|webserver"
```

**检查共享内存**:
```bash
ipcs -m
```

---

## 配置管理

### 每策略配置

每个策略一个配置文件：`config/trader.{strategy_id}.yaml`

```yaml
# config/trader.92201.yaml
system:
  strategy_id: 92201
  strategy_type: "TB_PAIR_STRAT"

shm:
  request_key: 0x2001
  response_key: 0x3001
  md_key: 0x1001
  client_store_key: 0x4001

strategy:
  symbols:
    - ag2603
    - ag2605
  parameters:
    begin_place: 0.5          # 测试用低阈值
    long_place: 2.0
    short_place: -2.0
    size: 1
    max_size: 10
```

### 关键配置项说明

- **shm.request_key / response_key / md_key**: SysV SHM 队列 key，必须与 counter_bridge / md_shm_feeder 一致
- **begin_place**: 开始挂单阈值（测试用低值，生产用保守值）
- **max_size**: 最大持仓，根据账户资金和风险承受能力设置

---

## 重要约定

### SysV SHM Key 分配

| Key | 十进制 | 用途 | 写入方 | 读取方 |
|-----|--------|------|--------|--------|
| `0x1001` | 4097 | 行情队列（MarketUpdateNew） | md_shm_feeder | trader |
| `0x2001` | 8193 | 订单请求队列（RequestMsg） | trader | counter_bridge |
| `0x3001` | 12289 | 订单回报队列（ResponseMsg） | counter_bridge | trader |
| `0x4001` | 16385 | ClientStore（客户端 ID 分配） | trader / counter_bridge | trader / counter_bridge |

### 订单 ID 格式

- 格式: `clientId * 1_000_000 + seq`（与 hftbase Connector 一致）
- 回报过滤: `OrderID / ORDERID_RANGE` 匹配 clientId

### 消息结构体

- `MarketUpdateNew`: ~900+ bytes（含 `bookElement_t[20] x 2`），来自 `hftbase/CommonUtils/include/marketupdateNew.h`
- `RequestMsg`: `__attribute__((aligned(64)))`，Go 需手动补 padding
- `ResponseMsg`: `ResponseType` 19 种枚举（NEW_ORDER_CONFIRM=0, TRADE_CONFIRM=4, ORDER_ERROR=5 等）

---

## 常见问题排查

### 问题：无行情数据

```bash
# 检查 md_shm_feeder 进程
ps aux | grep md_shm_feeder

# 检查 SHM 是否创建
ipcs -m

# 检查日志
tail -f deploy_new/log/md_shm_feeder.*.log
```

### 问题：无订单生成

**检查清单**:
1. trader 是否在运行？
   ```bash
   ps aux | grep trader
   ```

2. counter_bridge 是否在运行？
   ```bash
   ps aux | grep counter_bridge
   ```

3. 策略参数是否正确？
   ```bash
   grep -i "begin_place\|threshold" deploy_new/nohup.out.*
   ```

### 问题：共享内存错误

```bash
# 清理所有 SysV 共享内存段
ipcs -m | grep "$(whoami)" | awk '{print $2}' | xargs -I{} ipcrm -m {}

# 重启
cd deploy_new
./scripts/stop_all.sh
./scripts/start_gateway.sh sim
```

---

## 安全规范

### 禁止事项

- ❌ 在代码中硬编码密钥、密码
- ❌ 提交敏感配置文件（使用 `.gitignore`）
- ❌ 在生产环境使用 `auto_activate: true`
- ❌ 跳过风险检查
- ❌ 在不了解的情况下修改共享内存结构

### 推荐实践

- ✅ 使用环境变量或外部配置管理敏感信息
- ✅ 测试新策略时先用小仓位
- ✅ 定期备份配置和日志
- ✅ 代码审查关注风控逻辑
- ✅ 提交前运行完整测试

---

## 文件组织结构

```
quantlink-trade-system/
├── gateway/              # C++ 网关代码（md_shm_feeder, counter_bridge 等）
│   ├── src/             # 源文件
│   ├── include/         # 头文件
│   └── build/           # 编译产物（不提交）
├── tbsrc-golang/        # Golang 策略代码（活跃，SysV SHM 直连）
│   ├── cmd/             # 主程序入口（trader, webserver, backtest）
│   ├── pkg/             # 业务逻辑包（shm, connector, common, strategy, indicator, config）
│   └── web/             # Web 资源
├── deploy_new/          # 编译部署产物（由 build_deploy_new.sh 生成，不提交）
├── data_new/            # 持久配置模板（config, controls, models）
├── scripts/             # 脚本文件（按功能分类）
│   ├── build_deploy_new.sh  # 一键编译部署
│   ├── test/            # 测试脚本
│   ├── live/            # 实盘脚本
│   └── backtest/        # 回测脚本
├── docs/                # 文档（按主题分类）
│   ├── README.md        # 文档索引中心
│   ├── 核心文档/        # 部署指南
│   ├── 功能实现/        # Phase2-9 实施计划
│   ├── 系统分析/        # 架构分析、MWMR 规格
│   └── archive/         # 已归档文档（131个）
└── .claude/             # Claude Code 规则
```

### .gitignore 重点

```gitignore
# 编译产物
gateway/build/
bin/
*.o
*.so

# 日志
log/
test_logs/
*.log

# 临时文件
*.swp
*.tmp
.DS_Store

# 敏感配置
config/*.local.yaml
.env
```

---

## 性能要求

### 延迟指标

- SysV SHM 读写: < 1us（微秒级）
- 策略计算: < 10ms
- **端到端延迟（行情 → 订单）**: < 20ms

### 资源限制

- CPU 使用: 单进程 < 20%
- 内存占用: 单进程 < 100MB
- 网络流量: < 10MB/min

---

## Git 工作流

### 分支策略

- `main`: 生产稳定版本
- `develop`: 开发主分支
- `feature/*`: 新功能开发
- `bugfix/*`: Bug 修复
- `hotfix/*`: 紧急修复

### 提交规范

使用 Conventional Commits:

```
feat: 添加配对套利策略
fix: 修复共享内存泄漏问题
docs: 更新端到端测试文档
refactor: 重构订单路由逻辑
test: 添加策略引擎单元测试
chore: 更新依赖版本
```

### 提交前检查

1. ✅ 代码已格式化（C++/Golang）
2. ✅ 通过编译
3. ✅ 通过单元测试
4. ✅ 更新相关文档
5. ✅ 检查 .gitignore（不提交日志、二进制文件）

---

## 部署检查清单

### 上线前验证

- [ ] 完整端到端测试通过
- [ ] 配置文件已切换到生产配置
- [ ] 策略参数已调整到保守值（entry_zscore ≥ 2.0）
- [ ] 风控参数已设置合理值
- [ ] 日志级别设置正确（info 或 warn）
- [ ] 资源监控已就绪
- [ ] 回滚方案已准备

### 监控指标

- 订单成功率
- 策略信号频率
- 系统延迟
- CPU/内存使用
- 错误日志数量

---

## 端到端测试规则

### ⚠️ 重要：测试前必须编译和部署

**端到端测试的完整流程是：编译 → 部署 → 测试**

如果修改了代码，必须先重新编译并部署到 `deploy_new/` 目录，否则测试的是旧代码！

```bash
# 1. 编译部署
./scripts/build_deploy_new.sh --go      # 只部署 Go
./scripts/build_deploy_new.sh --cpp     # 只部署 C++
./scripts/build_deploy_new.sh           # 全部部署

# 2. 运行测试
cd deploy_new
./scripts/start_gateway.sh sim          # 模拟测试
./scripts/start_strategy.sh 92201
# 观察日志确认正常后停止
./scripts/stop_all.sh
```

### 模拟测试

```bash
cd deploy_new
./scripts/start_gateway.sh sim
./scripts/start_strategy.sh 92201
```

**架构**:
```
md_shm_feeder (simulator) → [SysV SHM 0x1001] → trader → [SysV SHM 0x2001] → counter_bridge (simulator)
```

### CTP 实盘测试

```bash
cd deploy_new
./scripts/start_gateway.sh ctp
./scripts/start_strategy.sh 92201
```

**架构**:
```
md_shm_feeder (CTP) → [SysV SHM 0x1001] → trader → [SysV SHM 0x2001] → counter_bridge (CTP) → CTP交易服务器
```

### 测试前置条件

**模拟测试**: 无需额外配置，使用 `config/trader.{id}.yaml`

**CTP实盘测试**:
- 需要 `config/ctp/ctp_md.secret.yaml` (行情账号)
- 需要 `config/ctp/ctp_td.secret.yaml` (交易账号)
- SimNow 标准环境交易时段：周一至周五 9:00-15:00

### 停止服务

```bash
cd deploy_new
./scripts/stop_all.sh
```

---

## 联系方式

**系统维护**: 参考 @docs/README.md

**问题反馈**: 创建 Issue 或提交 PR

---

## 📋 项目重组历史

**2026-02-25**: 文档命名格式更新 + Java 迁移启动
- 文档命名格式改为时间在前：`YYYY-MM-DD-HH_mm_模块_摘要.md`（旧格式：`模块_摘要_YYYY-MM-DD-HH_mm.md`）
- 新增 `docs/java迁移/` 目录，用于 C++ → Java 迁移相关文档
- 完成 Java 迁移可行性评估

**2026-02-24**: 文档归档 + 架构更新
- 归档 94 个过时文档到 archive/（golang/ 35个、测试报告/ 8个、核心文档/ 4个、实盘/ 27个、功能实现/ 15个、系统分析/ 5个）
- 新建部署指南 `docs/核心文档/DEPLOY_GUIDE_2026-02-24.md`（SysV MWMR SHM 直连架构）
- 更新 CLAUDE.md：移除 NATS/gRPC/ors_gateway/md_gateway 旧架构引用，替换为 SysV MWMR SHM 描述
- 更新 docs/README.md 文档索引

**2026-02-09**: 合并简化测试脚本
- 统一使用 `--run` 参数控制运行模式
- 移除 live/ 目录中的重复脚本，只保留 stop_all.sh
- 核心脚本：test_simulator_e2e.sh、test_ctp_live_e2e.sh

**2026-01-30**: 建立文档与脚本关联体系
- 创建 CROSS_REFERENCE.md 交叉索引文件
- 建立三种关联方式：脚本→文档、文档→脚本、交叉索引
- 新增规则 11-16: 关联管理规范
- 提供功能查找、文档查找、故障排查三类快速查找表

**2026-01-30**: 完成脚本目录重组
- 将25个 .sh 脚本从根目录整理到 scripts/ 目录
- 按功能分类：test/, live/, trading/, backtest/
- 根目录脚本从25个减少到0个
- 创建 scripts/README.md 使用指南
- 测试脚本细分为: e2e/, integration/, unit/, feature/
- 新增规则 5-10: 脚本管理规范

**2026-01-30**: 完成文档目录重组
- 将114个文档从混乱的根目录重组为10个主题目录
- 根目录文档从73个减少到3个（README.md, QUICKSTART.md, TASKS.md）
- 创建完整的文档索引和导航系统
- 归档35个旧文档到 archive/
- 详细报告: @docs/系统分析/文档重组完成报告_2026-01-30-23_30.md

**2026-01-30**: 完成持仓管理功能实施
- Phase 1-5: 持仓查询、初始化、持久化、定期校验
- 修复参数加载类型不匹配问题（min_correlation、slippage_ticks）
- 实施双重保障机制：CTP查询 + JSON文件恢复
- 详细报告: @docs/实盘/Phase2-5_完整持仓管理功能实施报告_2026-01-30-11_35.md

---

**最后更新**: 2026-02-25
**文档版本**: v2.1
