# Scripts 脚本目录

本目录包含 QuantLink Trade System 的核心测试脚本。

**最后更新**: 2026-02-09

---

## 目录结构

```
scripts/
├── README.md
├── test/e2e/                      # 端到端测试
│   ├── test_simulator_e2e.sh      # 模拟交易所测试
│   ├── test_ctp_live_e2e.sh       # CTP实盘测试
│   └── test_full_chain.sh         # 完整链路测试
├── live/
│   └── stop_all.sh                # 停止所有服务
└── archive/                       # 已归档脚本
```

---

## 核心脚本

### 1. 模拟交易测试

**test/e2e/test_simulator_e2e.sh**

```bash
# 运行测试（启动系统 → 验证 → 退出）
./scripts/test/e2e/test_simulator_e2e.sh

# 启动系统并保持运行（开发/调试用）
./scripts/test/e2e/test_simulator_e2e.sh --run
```

**架构**:
```
md_simulator → [SHM] → md_gateway → [NATS] → trader → [gRPC] → ors_gateway → [SHM] → counter_gateway
```

### 2. CTP实盘测试

**test/e2e/test_ctp_live_e2e.sh**

```bash
# 运行测试（启动系统 → 验证 → 退出）
./scripts/test/e2e/test_ctp_live_e2e.sh

# 启动系统并保持运行（实盘交易用）
./scripts/test/e2e/test_ctp_live_e2e.sh --run
```

**架构**:
```
CTP行情服务器 → ctp_md_gateway → [SHM] → md_gateway → [NATS] → trader → [gRPC] → ors_gateway → counter_bridge(CTP) → CTP交易服务器
```

**配置要求**:
- `config/ctp/ctp_md.secret.yaml` - 行情账号
- `config/ctp/ctp_td.secret.yaml` - 交易账号

### 3. 停止服务

```bash
./scripts/live/stop_all.sh
```

---

## 参数说明

| 参数 | 说明 |
|------|------|
| (无参数) | 运行测试后自动退出 |
| `--run` | 前台运行（Ctrl+C停止） |
| `--background` | 后台运行（用于长期运行） |

---

## 注意事项

1. 模拟测试无需额外配置
2. CTP实盘需要 SimNow 账号（交易时段：周一至周五 9:00-15:00）
3. 归档脚本在 `archive/` 目录
