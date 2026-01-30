# 文档与脚本交叉索引

**最后更新**: 2026-01-30

---

## 📚 按功能分类的文档与脚本对应关系

### 🏗️ 构建与部署

| 功能 | 脚本 | 相关文档 |
|------|------|---------|
| 编译 C++ Gateway | `scripts/build_gateway.sh` | [BUILD_GUIDE.md](docs/核心文档/BUILD_GUIDE.md) |
| 编译 Golang Trader | `scripts/build_golang.sh` | [BUILD_GUIDE.md](docs/核心文档/BUILD_GUIDE.md) |
| 生成 Protobuf 代码 | `scripts/generate_proto.sh` | [BUILD_GUIDE.md](docs/核心文档/BUILD_GUIDE.md) |
| 快速部署 | `scripts/quick_deploy.sh` | [系统_编译部署启动指南](docs/系统分析/系统_编译部署启动指南_2026-01-24-16_15.md) |
| 准备部署环境 | `scripts/prepare_deploy.sh` | [系统_编译部署启动指南](docs/系统分析/系统_编译部署启动指南_2026-01-24-16_15.md) |

### 🧪 端到端测试

| 功能 | 脚本 | 相关文档 |
|------|------|---------|
| 完整链路测试 | `scripts/test/e2e/test_full_chain.sh` | [USAGE.md](docs/核心文档/USAGE.md) |
| CTP 端到端测试 | `scripts/test/e2e/test_ctp_e2e.sh` | [端到端测试报告_20260130](docs/测试报告/端到端测试报告_20260130_002214.md) |
| CTP 完整测试 | `scripts/test/e2e/test_ctp_e2e_full.sh` | [端到端测试报告_20260130](docs/测试报告/端到端测试报告_20260130_002214.md) |
| **Simulator 端到端测试** | `scripts/test/e2e/test_simulator_e2e.sh` | [模拟交易所_完整实施报告](docs/功能实现/模拟交易所_完整实施报告_2026-01-30-15_00.md) |
| 检查测试状态 | `scripts/test/e2e/check_ctp_e2e.sh` | - |
| 停止测试 | `scripts/test/e2e/stop_ctp_e2e.sh` | - |

### 🔗 集成测试

| 功能 | 脚本 | 相关文档 |
|------|------|---------|
| 多策略 Dashboard | `scripts/test/integration/test_multi_strategy_dashboard.sh` | [多策略热加载实现报告](docs/功能实现/多策略热加载实现报告_2026-01-29-15_35.md) |
| 多策略热加载 | `scripts/test/integration/test_multi_strategy_hot_reload.sh` | [多策略热加载端到端测试报告](docs/测试报告/多策略热加载端到端测试报告_2026-01-29-15_50.md) |
| WebSocket 测试 | `scripts/test/integration/test_multi_strategy_websocket_e2e.sh` | - |
| 多策略+热加载 | `scripts/test/integration/test_multi_strategy_with_hotreload.sh` | [多策略热加载实现报告](docs/功能实现/多策略热加载实现报告_2026-01-29-15_35.md) |
| Dashboard 模拟器 | `scripts/test/integration/test_dashboard_simulator.sh` | - |

### 🧬 单元测试

| 功能 | 脚本 | 相关文档 |
|------|------|---------|
| CTP 账户查询 | `scripts/test/unit/test_ctp_account.sh` | [CTP_POSITION_GUIDE.md](docs/实盘/CTP_POSITION_GUIDE.md) |
| CTP 查询功能 | `scripts/test/unit/test_ctp_query.sh` | [任务1_CTP行情接入实施指南](docs/功能实现/任务1_CTP行情接入实施指南_2026-01-26-15_40.md) |
| CTP 交易功能 | `scripts/test/unit/test_ctp_trading.sh` | [任务1_CTP行情接入实施指南](docs/功能实现/任务1_CTP行情接入实施指南_2026-01-26-15_40.md) |
| WebSocket 功能 | `scripts/test/unit/test_websocket.sh` | - |
| 参数加载验证 | `scripts/test/unit/verify_param_loading.sh` | [参数加载修复报告](docs/实盘/参数加载修复报告_2026-01-30-11_05.md) |

### ⚙️ 功能测试

| 功能 | 脚本 | 相关文档 |
|------|------|---------|
| 持仓持久化 | `scripts/test/feature/test_position_persistence.sh` | [Phase2-5_完整持仓管理功能实施报告](docs/实盘/Phase2-5_完整持仓管理功能实施报告_2026-01-30-11_35.md) |
| 持仓查询 | `scripts/test/feature/test_position_query.sh` | [持仓查询功能实现](docs/功能实现/持仓查询功能实现_2026-01-28-11_30.md) |

### 📈 实盘脚本

| 功能 | 脚本 | 相关文档 |
|------|------|---------|
| 启动实盘测试 | `scripts/live/start_live_test.sh` | [实盘测试快速参考](docs/实盘/实盘测试快速参考.md) |
| 启动完整测试 | `scripts/live/start_full_test.sh` | [使用实盘配置启动](docs/实盘/使用实盘配置启动.md) |
| **启动模拟交易所** | `scripts/live/start_simulator.sh` | [模拟交易所_完整实施报告](docs/功能实现/模拟交易所_完整实施报告_2026-01-30-15_00.md) |
| 监控实盘测试 | `scripts/live/monitor_live_test.sh` | [实盘测试运行报告](docs/实盘/实盘测试运行报告_2026-01-30-10_55.md) |
| 实盘监控 | `scripts/live/monitor_live.sh` | [实盘测试运行报告](docs/实盘/实盘测试运行报告_2026-01-30-10_55.md) |
| 停止所有服务 | `scripts/live/stop_all.sh` | - |

### 💼 交易操作

| 功能 | 脚本 | 相关文档 |
|------|------|---------|
| 交易 ag2603 | `scripts/trading/trade_ag2603.sh` | [实盘测试快速参考](docs/实盘/实盘测试快速参考.md) |
| 平仓 ag2603 | `scripts/trading/close_ag2603.sh` | [实盘测试快速参考](docs/实盘/实盘测试快速参考.md) |
| 查询持仓 | `scripts/trading/query_position.sh` | [持仓查询功能实现](docs/功能实现/持仓查询功能实现_2026-01-28-11_30.md) |
| 获取市场价格 | `scripts/trading/get_market_price.sh` | - |

### 📊 回测

| 功能 | 脚本 | 相关文档 |
|------|------|---------|
| 运行回测 | `scripts/backtest/run_backtest.sh` | [回测_使用指南](docs/回测/回测_使用指南_2026-01-24-19_00.md) |

---

## 📋 按文档分类的相关脚本

### 核心文档

#### BUILD_GUIDE.md
**相关脚本**:
- `scripts/build_gateway.sh` - 编译 C++ Gateway
- `scripts/build_golang.sh` - 编译 Golang Trader
- `scripts/generate_proto.sh` - 生成 Protobuf 代码
- `scripts/install_dependencies.sh` - 安装系统依赖

#### USAGE.md
**相关脚本**:
- `scripts/test/e2e/test_full_chain.sh` - 完整链路测试
- `scripts/live/start_live_test.sh` - 启动实盘测试
- `scripts/trading/query_position.sh` - 查询持仓

#### CURRENT_ARCHITECTURE_FLOW.md
**相关脚本**:
- `scripts/test/e2e/test_full_chain.sh` - 验证完整数据流
- `scripts/test/e2e/test_ctp_e2e.sh` - 验证 CTP 集成

### 实盘文档

#### Phase2-5_完整持仓管理功能实施报告
**相关脚本**:
- `scripts/test/feature/test_position_persistence.sh` - 测试持仓持久化
- `scripts/test/feature/test_position_query.sh` - 测试持仓查询
- `scripts/trading/query_position.sh` - 查询持仓

#### 参数加载修复报告
**相关脚本**:
- `scripts/test/unit/verify_param_loading.sh` - 验证参数加载

#### 实盘测试快速参考
**相关脚本**:
- `scripts/live/start_live_test.sh` - 启动实盘测试
- `scripts/live/monitor_live.sh` - 监控实盘
- `scripts/trading/trade_ag2603.sh` - 交易操作
- `scripts/trading/close_ag2603.sh` - 平仓操作

### 功能实现文档

#### 多策略热加载实现报告
**相关脚本**:
- `scripts/test/integration/test_multi_strategy_hot_reload.sh`
- `scripts/test/integration/test_multi_strategy_with_hotreload.sh`
- `scripts/test/integration/test_multi_strategy_dashboard.sh`

#### 持仓查询功能实现
**相关脚本**:
- `scripts/test/feature/test_position_query.sh`
- `scripts/trading/query_position.sh`

#### 任务1_CTP行情接入实施指南
**相关脚本**:
- `scripts/test/unit/test_ctp_query.sh`
- `scripts/test/unit/test_ctp_trading.sh`
- `scripts/test/e2e/test_ctp_e2e.sh`

### 回测文档

#### 回测_使用指南
**相关脚本**:
- `scripts/backtest/run_backtest.sh`

---

## 🔍 快速查找

### 我想测试某个功能，应该运行哪个脚本？

| 需求 | 脚本 |
|------|------|
| 测试完整系统 | `scripts/test/e2e/test_full_chain.sh` |
| 测试 CTP 对接 | `scripts/test/e2e/test_ctp_e2e.sh` |
| 测试持仓管理 | `scripts/test/feature/test_position_query.sh` |
| 测试热加载 | `scripts/test/integration/test_multi_strategy_hot_reload.sh` |
| 启动实盘测试 | `scripts/live/start_live_test.sh` |
| 查询持仓 | `scripts/trading/query_position.sh` |
| 运行回测 | `scripts/backtest/run_backtest.sh` |

### 我读了某个文档，想验证功能，应该运行哪个脚本？

| 文档主题 | 推荐脚本 |
|---------|---------|
| 持仓管理 | `scripts/test/feature/test_position_persistence.sh` |
| 多策略热加载 | `scripts/test/integration/test_multi_strategy_hot_reload.sh` |
| CTP 对接 | `scripts/test/e2e/test_ctp_e2e.sh` |
| 系统架构 | `scripts/test/e2e/test_full_chain.sh` |
| 回测功能 | `scripts/backtest/run_backtest.sh` |

### 我运行了某个脚本出错，应该查看哪个文档？

| 脚本 | 排查文档 |
|------|---------|
| `test_full_chain.sh` | [USAGE.md](docs/核心文档/USAGE.md), [端到端测试报告](docs/测试报告/) |
| `test_ctp_e2e.sh` | [CTP_POSITION_GUIDE.md](docs/实盘/CTP_POSITION_GUIDE.md) |
| `test_position_*.sh` | [Phase2-5_完整持仓管理功能实施报告](docs/实盘/Phase2-5_完整持仓管理功能实施报告_2026-01-30-11_35.md) |
| `start_live_test.sh` | [实盘测试快速参考](docs/实盘/实盘测试快速参考.md) |
| `verify_param_loading.sh` | [参数加载修复报告](docs/实盘/参数加载修复报告_2026-01-30-11_05.md) |

---

## 📝 维护说明

**更新时机**:
- 新增脚本时，添加对应的文档链接
- 新增重要文档时，关联相关脚本
- 脚本重命名或移动时，更新所有引用

**维护责任**:
- 脚本作者负责在脚本头部添加文档引用
- 文档作者负责在文档中引用相关脚本
- 定期审查本索引文件，确保链接有效

---

**最后更新**: 2026-01-30
**维护者**: QuantLink Team
