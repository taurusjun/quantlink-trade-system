# QuantLink Trade System - Documentation

## 文档目录

### Golang 实现文档

位置：`docs/golang/`

#### 架构与实施
- [ARCHITECTURE_UPGRADE.md](golang/ARCHITECTURE_UPGRADE.md) - 架构升级详情
  - 目标1：优化发单路径
  - 目标2：引入共享指标池
  - 目标3：实现混合模式

- [IMPLEMENTATION_SUMMARY.md](golang/IMPLEMENTATION_SUMMARY.md) - 实施总结
  - 三大目标实施完成情况
  - 性能对比总结
  - 文件清单
  - 使用指南

- [EVENT_CALLBACK_ALIGNMENT.md](golang/EVENT_CALLBACK_ALIGNMENT.md) - 事件回调机制对齐分析
  - tbsrc事件触发机制详解
  - quantlink-trade-system/golang对比
  - 对齐方案与建议

- [EVENT_CALLBACK_IMPLEMENTATION.md](golang/EVENT_CALLBACK_IMPLEMENTATION.md) - 事件回调机制实现报告
  - 竞价行情事件支持（OnAuctionData）
  - 显式指标回调接口（OnIndicatorUpdate）
  - 细粒度订单状态事件（DetailedOrderStrategy）

- [STRATEGY_STATE_CONTROL_ANALYSIS.md](golang/STRATEGY_STATE_CONTROL_ANALYSIS.md) - 策略状态控制机制分析
  - tbsrc状态控制变量深度分析
  - 状态转换流程与使用场景
  - golang对齐方案与实现建议

- [STRATEGY_STATE_CONTROL_IMPLEMENTATION.md](golang/STRATEGY_STATE_CONTROL_IMPLEMENTATION.md) - 策略状态控制实现报告
  - 激活控制（m_Active）
  - 平仓模式控制（m_onFlat/m_onCancel/m_aggFlat）
  - 退出控制（m_onExit）
  - 风险检查集成（CheckSquareoff）

#### 技术实现
- [INDICATOR_IMPLEMENTATION_STATUS.md](golang/INDICATOR_IMPLEMENTATION_STATUS.md) - 指标实现状态
  - 已实现的指标列表
  - 技术指标详情

- [TEST_FIXES_REPORT.md](golang/TEST_FIXES_REPORT.md) - 测试修复报告
  - 测试用例修复记录

### 项目文档

位置：`docs/`

#### 系统设计
- [ARCHITECTURE_INCONSISTENCY_ANALYSIS.md](ARCHITECTURE_INCONSISTENCY_ANALYSIS.md) - 架构一致性分析
- [ARCHITECTURE_UNIFICATION_SUMMARY.md](ARCHITECTURE_UNIFICATION_SUMMARY.md) - 架构统一总结
- [CURRENT_ARCHITECTURE_FLOW.md](CURRENT_ARCHITECTURE_FLOW.md) - 当前架构流程
- [PROJECT_OVERVIEW.md](PROJECT_OVERVIEW.md) - 项目概览

#### 命名规范
- [NAMING_CONVENTION_UPDATE.md](NAMING_CONVENTION_UPDATE.md) - 命名规范更新
- [NAMING_FINAL_UPDATE.md](NAMING_FINAL_UPDATE.md) - 命名规范最终版

#### 性能报告
- [PERFORMANCE_REPORT.md](PERFORMANCE_REPORT.md) - 性能测试报告
- [SHM_COMPARISON.md](SHM_COMPARISON.md) - 共享内存对比
- [SHM_EXAMPLE.md](SHM_EXAMPLE.md) - 共享内存示例
- [SIMPLIFIED_SHM_CAPABILITY_ANALYSIS.md](SIMPLIFIED_SHM_CAPABILITY_ANALYSIS.md) - 共享内存能力分析
- [IPC_COMPARISON_CENTOS.md](IPC_COMPARISON_CENTOS.md) - IPC对比（CentOS）

#### 实现状态
- [GATEWAY_IMPLEMENTATION_STATUS.md](GATEWAY_IMPLEMENTATION_STATUS.md) - Gateway实现状态
- [STAGE3_COMPLETION_STATUS.md](STAGE3_COMPLETION_STATUS.md) - 第三阶段完成状态
- [STAGE3_FINAL_COMPLETION_REPORT.md](STAGE3_FINAL_COMPLETION_REPORT.md) - 第三阶段最终报告
- [STAGE3_TESTING_COMPLETION_REPORT.md](STAGE3_TESTING_COMPLETION_REPORT.md) - 第三阶段测试报告
- [FINAL_TEST_COMPLETION_REPORT.md](FINAL_TEST_COMPLETION_REPORT.md) - 最终测试报告

#### 周计划总结
- [WEEK56_ORS_GATEWAY_SUMMARY.md](WEEK56_ORS_GATEWAY_SUMMARY.md) - Week 5-6: ORS Gateway
- [WEEK78_GOLANG_ORS_CLIENT_SUMMARY.md](WEEK78_GOLANG_ORS_CLIENT_SUMMARY.md) - Week 7-8: Golang ORS Client
- [WEEK11_12_INDICATOR_LIBRARY_SUMMARY.md](WEEK11_12_INDICATOR_LIBRARY_SUMMARY.md) - Week 11-12: 指标库
- [WEEK13_14_STRATEGY_ENGINE_SUMMARY.md](WEEK13_14_STRATEGY_ENGINE_SUMMARY.md) - Week 13-14: 策略引擎
- [WEEK13_14_COMPLETE_STRATEGY_SUITE.md](WEEK13_14_COMPLETE_STRATEGY_SUITE.md) - Week 13-14: 完整策略套件
- [WEEK15_16_PORTFOLIO_RISK_SUMMARY.md](WEEK15_16_PORTFOLIO_RISK_SUMMARY.md) - Week 15-16: 组合与风险管理

#### 数据流与集成
- [INDICATORS_DATA_FLOW.md](INDICATORS_DATA_FLOW.md) - 指标数据流
- [HFTBASE_INTEGRATION_CHALLENGES.md](HFTBASE_INTEGRATION_CHALLENGES.md) - HFTBase集成挑战

#### 使用指南
- [USAGE.md](USAGE.md) - 使用指南
- [VSCODE_DEBUG_QUICKSTART.md](VSCODE_DEBUG_QUICKSTART.md) - VSCode调试快速入门

#### 更新记录
- [DOCUMENTATION_UPDATE.md](DOCUMENTATION_UPDATE.md) - 文档更新记录
- [CLEANUP_SUMMARY.md](CLEANUP_SUMMARY.md) - 代码清理总结

#### 项目任务
- [后续任务_20260120.md](后续任务_20260120.md) - 后续任务清单
- [系统启动_20260120.md](系统启动_20260120.md) - 系统启动说明

---

## 📚 快速导航

### 🚀 新手入门
1. [PROJECT_OVERVIEW.md](PROJECT_OVERVIEW.md) - 了解项目概况
2. [USAGE.md](USAGE.md) - 学习如何使用系统
3. [VSCODE_DEBUG_QUICKSTART.md](VSCODE_DEBUG_QUICKSTART.md) - 配置开发环境

### 🏗️ 架构设计
1. [CURRENT_ARCHITECTURE_FLOW.md](CURRENT_ARCHITECTURE_FLOW.md) - 理解当前架构
2. [ARCHITECTURE_UNIFICATION_SUMMARY.md](ARCHITECTURE_UNIFICATION_SUMMARY.md) - 了解架构统一方案
3. [golang/ARCHITECTURE_UPGRADE.md](golang/ARCHITECTURE_UPGRADE.md) - 最新架构升级（重要！）

### 📊 性能优化
1. [PERFORMANCE_REPORT.md](PERFORMANCE_REPORT.md) - 性能基准测试
2. [golang/IMPLEMENTATION_SUMMARY.md](golang/IMPLEMENTATION_SUMMARY.md) - 最新优化成果
3. [SHM_COMPARISON.md](SHM_COMPARISON.md) - IPC性能对比

### 💻 开发指南
1. [golang/INDICATOR_IMPLEMENTATION_STATUS.md](golang/INDICATOR_IMPLEMENTATION_STATUS.md) - 指标开发状态
2. [NAMING_FINAL_UPDATE.md](NAMING_FINAL_UPDATE.md) - 代码命名规范
3. [golang/TEST_FIXES_REPORT.md](golang/TEST_FIXES_REPORT.md) - 测试指南

---

## 🎯 最新更新（2026-01-22）

### 🎉 重大架构升级 + 事件机制100%对齐

#### 第一阶段：三大目标（性能提升59%）✅

✅ **目标1：优化发单路径**
- 延迟降低 75%：~50-200μs → ~10-50μs

✅ **目标2：引入共享指标池**
- 性能提升 58%：避免重复计算

✅ **目标3：实现混合模式**
- 完全对齐 tbsrc 架构（95%）

详见：[golang/ARCHITECTURE_UPGRADE.md](golang/ARCHITECTURE_UPGRADE.md)

#### 第二阶段：事件回调机制100%对齐 ✅

✅ **新增1：竞价行情事件支持（OnAuctionData）**
- 区分竞价期/连续交易期
- 完全对齐 tbsrc AuctionCallBack

✅ **新增2：显式指标回调接口（OnIndicatorUpdate）**
- 指标更新后显式回调
- 完全对齐 tbsrc INDCallBack

✅ **新增3：细粒度订单状态事件**
- OnOrderNew/OnOrderFilled/OnOrderCanceled/OnOrderRejected
- 更细粒度的 ORSCallBack

**对齐度**: 85% → 100% 🎉

详见：
- [golang/EVENT_CALLBACK_ALIGNMENT.md](golang/EVENT_CALLBACK_ALIGNMENT.md) - 对齐分析
- [golang/EVENT_CALLBACK_IMPLEMENTATION.md](golang/EVENT_CALLBACK_IMPLEMENTATION.md) - 实现报告

#### 第三阶段：策略状态控制100%对齐 ✅

✅ **新增1：策略激活控制（m_Active）**
- 手动激活/禁用策略
- 完全对齐 tbsrc m_Active

✅ **新增2：平仓模式控制（m_onFlat/m_onCancel/m_aggFlat）**
- 风险触发自动平仓
- 自动恢复机制（冷却时间）
- 激进平仓模式（穿越买卖盘）

✅ **新增3：退出控制（m_onExit）**
- 不可恢复的退出流程
- 完全退出前必须平仓

✅ **新增4：风险检查集成（CheckSquareoff）**
- 止损、最大亏损、拒单限制自动检查
- Engine定时器自动执行

**对齐度**: 100% 🎉

详见：
- [golang/STRATEGY_STATE_CONTROL_ANALYSIS.md](golang/STRATEGY_STATE_CONTROL_ANALYSIS.md) - 对齐分析
- [golang/STRATEGY_STATE_CONTROL_IMPLEMENTATION.md](golang/STRATEGY_STATE_CONTROL_IMPLEMENTATION.md) - 实现报告

---

## 📖 文档规范

### 新增文档放置位置

1. **Golang 相关文档** → `docs/golang/`
   - 架构设计
   - 实现细节
   - 性能优化
   - 测试报告

2. **C++ Gateway 相关** → `docs/gateway/`
   - Gateway实现
   - Protocol定义
   - 性能测试

3. **通用项目文档** → `docs/`
   - 项目概览
   - 使用指南
   - 周报总结

### 文档命名规范

- 使用大写字母和下划线：`ARCHITECTURE_UPGRADE.md`
- 中文文档可使用中文命名：`系统启动_20260120.md`
- 日期格式：`YYYYMMDD`

---

## 🔗 相关链接

- **项目根目录**：`/Users/user/PWorks/RD/quantlink-trade-system/`
- **Golang 代码**：`golang/`
- **C++ Gateway**：`gateway/`
- **配置文件**：`config/`
- **示例代码**：`golang/examples/`
