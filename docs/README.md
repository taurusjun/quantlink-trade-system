# QuantLink Trade System - 文档中心

**最后更新**: 2026-02-24

---

## 📁 文档目录结构

```
docs/
├── README.md                  # 本文档（文档索引）
│
├── 核心文档/                  # 系统核心文档
│   └── DEPLOY_GUIDE_2026-02-24.md    # 部署指南（SysV MWMR SHM 直连架构）
│
├── 实盘/                      # 实盘交易相关（4个文档）
│   ├── 部署运行指南_2026-02-10.md
│   ├── CTP_PnL计算修复报告_2026-02-11-22_30.md
│   ├── CTP_POSITION_GUIDE.md
│   └── 中国期货市场规则修复报告_2026-01-30-20_01.md
│
├── 回测/                      # 回测系统文档（7个文档）
│   ├── 回测_使用指南_2026-01-24-19_00.md
│   ├── 回测_参数优化使用指南_2026-01-24-20_30.md
│   └── ...
│
├── 功能实现/                  # Phase2-9 实施计划、C++ 对照（18个文档）
│   ├── Phase2_CommonClient_ExecutionStrategy_实施计划_2026-02-13-10_00.md
│   ├── Phase3_PairwiseArbStrategy_实施计划_2026-02-13-21_20.md
│   ├── Phase4_系统集成与启动_实施计划_2026-02-13-21_40.md
│   ├── md_shm_feeder_实施报告_2026-02-14-02_00.md
│   ├── counter_bridge_MWMR改造实施报告_2026-02-14-01_30.md
│   └── ...
│
├── 系统分析/                  # 架构分析、MWMR 规格、代码对比（10个文档）
│   ├── tbsrc-golang_v2_架构更新_2026-02-13-16_00.md          # 架构权威文档
│   ├── hftbase_MWMR_Go复刻技术规格_2026-02-13-16_00.md       # MWMR 技术规格
│   ├── counter_bridge_MWMR改造方案_2026-02-13-19_00.md
│   ├── ExtraStrategy_PairwiseArb_Go与CPP代码对比_2026-02-10-00_30.md
│   ├── PairwiseArbStrategy_C++_Go代码比对报告_2026-02-10-14_55.md
│   └── ...
│
├── gateway/                   # Gateway模块文档（4个文档）
│   └── ...
│
└── archive/                   # 已归档文档（131个文档）
    └── 旧版本文档、旧架构文档、NATS/gRPC 时期文档等
```

---

## 🚀 快速开始

### 新用户必读

1. **🏗️ 部署指南** → [核心文档/DEPLOY_GUIDE_2026-02-24.md](核心文档/DEPLOY_GUIDE_2026-02-24.md)
   - 系统架构、编译、配置、启动、监控、停止

2. **📐 架构设计** → [系统分析/tbsrc-golang_v2_架构更新_2026-02-13-16_00.md](系统分析/tbsrc-golang_v2_架构更新_2026-02-13-16_00.md)
   - C++ → Go 翻译方案、文件对照表、方法对照表

3. **🔧 MWMR 技术规格** → [系统分析/hftbase_MWMR_Go复刻技术规格_2026-02-13-16_00.md](系统分析/hftbase_MWMR_Go复刻技术规格_2026-02-13-16_00.md)
   - SysV SHM 封装、MWMR Queue 实现细节

### 开发者指南

- **编译部署**: `./scripts/build_deploy_new.sh` → 输出到 `deploy_new/`
- **启动模拟**: `cd deploy_new && ./scripts/start_gateway.sh sim && ./scripts/start_strategy.sh 92201`
- **启动实盘**: `cd deploy_new && ./scripts/start_gateway.sh ctp && ./scripts/start_strategy.sh 92201`
- **停止所有**: `cd deploy_new && ./scripts/stop_all.sh`

---

## 📚 主题文档导航

### 💼 实盘交易

**目录**: `docs/实盘/`

**当前文档**:
- [部署运行指南_2026-02-10.md](实盘/部署运行指南_2026-02-10.md) — 实盘部署步骤
- [CTP_PnL计算修复报告_2026-02-11-22_30.md](实盘/CTP_PnL计算修复报告_2026-02-11-22_30.md) — PnL 计算修复
- [CTP_POSITION_GUIDE.md](实盘/CTP_POSITION_GUIDE.md) — CTP 持仓管理指南
- [中国期货市场规则修复报告_2026-01-30-20_01.md](实盘/中国期货市场规则修复报告_2026-01-30-20_01.md) — SHFE 规则

---

### 🧪 回测系统

**目录**: `docs/回测/`

**核心文档**:
- [回测_使用指南_2026-01-24-19_00.md](回测/回测_使用指南_2026-01-24-19_00.md)
- [回测_参数优化使用指南_2026-01-24-20_30.md](回测/回测_参数优化使用指南_2026-01-24-20_30.md)
- [回测_集成说明_2026-01-24-19_30.md](回测/回测_集成说明_2026-01-24-19_30.md)

---

### ⚙️ 功能实现

**目录**: `docs/功能实现/`

**Phase2-9 实施计划（SysV MWMR SHM 架构）**:
- [Phase2_CommonClient_ExecutionStrategy_实施计划](功能实现/Phase2_CommonClient_ExecutionStrategy_实施计划_2026-02-13-10_00.md)
- [Phase3_PairwiseArbStrategy_实施计划](功能实现/Phase3_PairwiseArbStrategy_实施计划_2026-02-13-21_20.md)
- [Phase4_系统集成与启动_实施计划](功能实现/Phase4_系统集成与启动_实施计划_2026-02-13-21_40.md)
- [Phase5_C++对照修正与完善_实施计划](功能实现/Phase5_C++对照修正与完善_实施计划_2026-02-13-21_55.md)
- [Phase6_ORS路由与线程安全修复_实施计划](功能实现/Phase6_ORS路由与线程安全修复_实施计划_2026-02-13-22_10.md)
- [Phase7_风控安全检查_实施计划](功能实现/Phase7_风控安全检查_实施计划_2026-02-13-23_30.md)
- [Phase8_订单流与风控修正_实施计划](功能实现/Phase8_订单流与风控修正_实施计划_2026-02-13-23_50.md)
- [Phase9_生产安全机制_实施计划](功能实现/Phase9_生产安全机制_实施计划_2026-02-14-00_10.md)

**网关实施报告**:
- [md_shm_feeder_实施报告](功能实现/md_shm_feeder_实施报告_2026-02-14-02_00.md)
- [counter_bridge_MWMR改造实施报告](功能实现/counter_bridge_MWMR改造实施报告_2026-02-14-01_30.md)

**C++ 对照与修复**:
- [Go_vs_CPP_详细对比与修复计划](功能实现/Go_vs_CPP_详细对比与修复计划_2026-02-10-15_00.md)
- [BaseStrategy_ExecutionStrategy_对比与修复计划](功能实现/BaseStrategy_ExecutionStrategy_对比与修复计划_2026-02-10-10_20.md)
- [C++功能遗漏检查报告](功能实现/C++功能遗漏检查报告_2026-02-10-17_00.md)

---

### 🔬 系统分析

**目录**: `docs/系统分析/`

**架构文档**:
- [tbsrc-golang_v2_架构更新](系统分析/tbsrc-golang_v2_架构更新_2026-02-13-16_00.md) — 架构权威文档
- [hftbase_MWMR_Go复刻技术规格](系统分析/hftbase_MWMR_Go复刻技术规格_2026-02-13-16_00.md)
- [counter_bridge_MWMR改造方案](系统分析/counter_bridge_MWMR改造方案_2026-02-13-19_00.md)

**代码对比**:
- [ExtraStrategy_PairwiseArb_Go与CPP代码对比](系统分析/ExtraStrategy_PairwiseArb_Go与CPP代码对比_2026-02-10-00_30.md)
- [PairwiseArbStrategy_C++_Go代码比对报告](系统分析/PairwiseArbStrategy_C++_Go代码比对报告_2026-02-10-14_55.md)
- [策略对比_PairwiseArbStrategy_2026-01-31](系统分析/策略对比_PairwiseArbStrategy_2026-01-31.md)

---

### 🔧 Gateway模块

**目录**: `docs/gateway/`

---

### 📦 已归档文档

**目录**: `docs/archive/`（131个文档）

包含：
- 旧 `golang/` 代码库文档（35个）
- 旧架构测试报告（8个）
- 旧核心文档（PROJECT_OVERVIEW, CURRENT_ARCHITECTURE_FLOW, USAGE, BUILD_GUIDE）
- 旧系统分析（NATS/gRPC 时期）
- 旧实盘文档（NATS/gRPC 链路）
- 旧功能实现文档

**说明**: 这些文档描述 NATS/gRPC 5 进程旧架构，已被 SysV MWMR SHM 直连架构替代。保留供历史参考。

---

## 🔍 常见问题速查

### 部署问题

| 问题 | 参考文档 |
|------|---------|
| 编译部署流程 | [核心文档/DEPLOY_GUIDE_2026-02-24.md](核心文档/DEPLOY_GUIDE_2026-02-24.md) |
| CTP 持仓管理 | [实盘/CTP_POSITION_GUIDE.md](实盘/CTP_POSITION_GUIDE.md) |
| PnL 计算问题 | [实盘/CTP_PnL计算修复报告_2026-02-11-22_30.md](实盘/CTP_PnL计算修复报告_2026-02-11-22_30.md) |
| SHFE 交易规则 | [实盘/中国期货市场规则修复报告_2026-01-30-20_01.md](实盘/中国期货市场规则修复报告_2026-01-30-20_01.md) |

### 架构问题

| 问题 | 参考文档 |
|------|---------|
| 整体架构设计 | [系统分析/tbsrc-golang_v2_架构更新_2026-02-13-16_00.md](系统分析/tbsrc-golang_v2_架构更新_2026-02-13-16_00.md) |
| SHM MWMR 实现 | [系统分析/hftbase_MWMR_Go复刻技术规格_2026-02-13-16_00.md](系统分析/hftbase_MWMR_Go复刻技术规格_2026-02-13-16_00.md) |
| C++ vs Go 对比 | [功能实现/Go_vs_CPP_详细对比与修复计划_2026-02-10-15_00.md](功能实现/Go_vs_CPP_详细对比与修复计划_2026-02-10-15_00.md) |

---

## 📝 文档贡献规范

### 文档放置规则

1. **核心文档** (`核心文档/`) — 部署指南等长期维护文档
2. **实盘相关** (`实盘/`) — 实盘部署、CTP 相关
3. **回测相关** (`回测/`) — 回测系统使用、参数优化
4. **功能实现** (`功能实现/`) — 新功能实施报告
5. **系统分析** (`系统分析/`) — 架构分析、代码对比
6. **Gateway** (`gateway/`) — C++ 网关文档
7. **归档** (`archive/`) — 已过时的文档

### 文档命名规范

**格式**: `模块_摘要_YYYY-MM-DD-HH_mm.md`

### 更新本索引

当添加重要文档时，请更新本 README.md 中的相应章节。

---

## 🔗 相关链接

- **项目根目录**: `/Users/user/PWorks/RD/quantlink-trade-system/`
- **Go 策略代码**: `tbsrc-golang/`
- **C++ Gateway**: `gateway/`
- **编译部署**: `scripts/build_deploy_new.sh`
- **部署产物**: `deploy_new/`
- **配置模板**: `data_new/`

---

**文档组织**: 2026-02-24 更新（归档旧架构文档，更新为 SysV MWMR SHM 直连架构）
