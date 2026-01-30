# Scripts 目录说明

**最后更新**: 2026-01-30

---

## 📂 目录结构

```
scripts/
├── README.md                      # 本文档
│
├── 构建脚本
│   ├── build_gateway.sh          # 编译 C++ Gateway
│   ├── build_golang.sh           # 编译 Golang Trader
│   └── generate_proto.sh         # 生成 Protobuf 代码
│
├── 部署脚本
│   ├── prepare_deploy.sh         # 准备部署环境
│   └── quick_deploy.sh           # 快速部署
│
├── 依赖安装
│   ├── install_dependencies.sh   # 安装系统依赖
│   └── install_nats_c.sh         # 安装 NATS C 客户端
│
├── test/                          # 测试脚本
│   ├── e2e/                      # 端到端测试
│   │   ├── test_full_chain.sh    # 完整链路测试
│   │   ├── test_ctp_e2e.sh       # CTP 端到端测试
│   │   ├── test_ctp_e2e_full.sh  # CTP 完整测试
│   │   ├── check_ctp_e2e.sh      # 检查 CTP 测试状态
│   │   └── stop_ctp_e2e.sh       # 停止 CTP 测试
│   │
│   ├── integration/              # 集成测试
│   │   ├── test_multi_strategy_dashboard.sh          # 多策略 Dashboard 测试
│   │   ├── test_multi_strategy_hot_reload.sh         # 多策略热加载测试
│   │   ├── test_multi_strategy_websocket_e2e.sh      # 多策略 WebSocket 测试
│   │   ├── test_multi_strategy_with_hotreload.sh     # 多策略+热加载集成测试
│   │   └── test_dashboard_simulator.sh               # Dashboard 模拟器测试
│   │
│   ├── unit/                     # 单元测试
│   │   ├── test_ctp_account.sh   # CTP 账户查询测试
│   │   ├── test_ctp_query.sh     # CTP 查询功能测试
│   │   ├── test_ctp_trading.sh   # CTP 交易功能测试
│   │   ├── test_websocket.sh     # WebSocket 功能测试
│   │   └── verify_param_loading.sh # 参数加载验证
│   │
│   └── feature/                  # 功能测试
│       ├── test_position_persistence.sh  # 持仓持久化测试
│       └── test_position_query.sh        # 持仓查询测试
│
├── live/                         # 实盘脚本
│   ├── start_live_test.sh        # 启动实盘测试
│   ├── start_full_test.sh        # 启动完整实盘测试
│   ├── monitor_live_test.sh      # 监控实盘测试
│   └── monitor_live.sh           # 实盘监控
│
├── trading/                      # 交易操作脚本
│   ├── trade_ag2603.sh           # 交易 ag2603
│   ├── close_ag2603.sh           # 平仓 ag2603
│   ├── query_position.sh         # 查询持仓
│   └── get_market_price.sh       # 获取市场价格
│
└── backtest/                     # 回测脚本
    └── run_backtest.sh           # 运行回测
```

---

## 🚀 常用脚本

### 构建项目

```bash
# 编译 C++ Gateway
./scripts/build_gateway.sh

# 编译 Golang Trader
./scripts/build_golang.sh

# 生成 Protobuf 代码
./scripts/generate_proto.sh
```

### 运行测试

```bash
# 端到端测试
./scripts/test/e2e/test_full_chain.sh

# CTP 完整测试
./scripts/test/e2e/test_ctp_e2e_full.sh

# 多策略热加载测试
./scripts/test/integration/test_multi_strategy_hot_reload.sh

# 持仓管理测试
./scripts/test/feature/test_position_query.sh
```

### 实盘操作

```bash
# 启动实盘测试
./scripts/live/start_live_test.sh

# 监控实盘运行
./scripts/live/monitor_live.sh

# 查询持仓
./scripts/trading/query_position.sh

# 获取市场价格
./scripts/trading/get_market_price.sh
```

### 部署

```bash
# 准备部署环境
./scripts/prepare_deploy.sh

# 快速部署
./scripts/quick_deploy.sh
```

---

## 📝 脚本命名规范

- **测试脚本**: `test_*.sh`
- **启动脚本**: `start_*.sh`
- **停止脚本**: `stop_*.sh`
- **监控脚本**: `monitor_*.sh`
- **构建脚本**: `build_*.sh`
- **安装脚本**: `install_*.sh`

---

## 🔧 脚本开发规范

### 1. 脚本头部模板

```bash
#!/bin/bash
set -e  # 遇到错误立即退出

# 脚本说明
# 用途: [脚本用途]
# 作者: [作者]
# 日期: [日期]

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"
```

### 2. 错误处理

```bash
# 检查命令是否成功
if ! command_here; then
    echo "ERROR: Command failed"
    exit 1
fi

# 检查文件是否存在
if [ ! -f "required_file" ]; then
    echo "ERROR: File not found"
    exit 1
fi
```

### 3. 日志输出

```bash
echo "[INFO] Starting process..."
echo "[WARN] Warning message"
echo "[ERROR] Error occurred" >&2  # 输出到 stderr
```

### 4. 清理资源

```bash
# 捕获退出信号，确保清理
trap cleanup EXIT

cleanup() {
    echo "Cleaning up..."
    pkill -f process_name
    rm -f temp_file
}
```

---

## 📚 相关文档

- 构建指南: [docs/核心文档/BUILD_GUIDE.md](../docs/核心文档/BUILD_GUIDE.md)
- 使用说明: [docs/核心文档/USAGE.md](../docs/核心文档/USAGE.md)
- 测试报告: [docs/测试报告/](../docs/测试报告/)

---

## 🔗 快速链接

- **项目根目录**: `/Users/user/PWorks/RD/quantlink-trade-system/`
- **Gateway 源码**: `gateway/`
- **Golang 源码**: `golang/`
- **配置文件**: `config/`
- **日志目录**: `log/`

---

**整理日期**: 2026-01-30
**脚本总数**: 29 个
