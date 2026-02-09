# Scripts 脚本目录

本目录包含 QuantLink Trade System 的核心测试脚本。

**最后更新**: 2026-02-09

---

## 📂 目录结构

```
scripts/
├── README.md                      # 本文件
├── test/                          # 测试脚本
│   └── e2e/                       # 端到端测试
│       ├── test_simulator_e2e.sh  # 模拟交易所端到端测试
│       ├── test_ctp_live_e2e.sh   # CTP实盘端到端测试
│       └── test_full_chain.sh     # 完整链路测试
├── live/                          # 实盘启动脚本
│   ├── start_simulator.sh         # 启动模拟交易系统
│   ├── start_ctp_live.sh          # 启动CTP实盘系统
│   └── stop_all.sh                # 停止所有服务
└── archive/                       # 已归档脚本（历史版本）
```

---

## 🚀 核心脚本说明

### 1. 模拟测试

**test/e2e/test_simulator_e2e.sh** - 模拟交易所端到端测试
- 启动完整模拟环境（md_simulator → md_gateway → trader → ors_gateway → counter_gateway）
- 验证订单全链路流转
- 适用于开发和调试阶段

```bash
./scripts/test/e2e/test_simulator_e2e.sh
```

**live/start_simulator.sh** - 启动模拟交易系统
- 长期运行的模拟环境
- 用于功能测试和策略调试

```bash
./scripts/live/start_simulator.sh
```

### 2. CTP实盘测试

**test/e2e/test_ctp_live_e2e.sh** - CTP实盘端到端测试
- 连接真实CTP行情和交易服务器（SimNow标准环境）
- 验证实盘订单流转
- 需要配置 `config/ctp/ctp_md.secret.yaml` 和 `config/ctp/ctp_td.secret.yaml`

```bash
./scripts/test/e2e/test_ctp_live_e2e.sh
```

**live/start_ctp_live.sh** - 启动CTP实盘系统
- 生产环境启动脚本
- 自动检查配置完整性
- 支持 Ctrl+C 安全停止

```bash
./scripts/live/start_ctp_live.sh
```

### 3. 停止服务

**live/stop_all.sh** - 停止所有交易服务
```bash
./scripts/live/stop_all.sh
```

---

## ⚙️ 配置要求

### 模拟测试
- 无额外配置，使用 `config/trader.test.yaml`

### CTP实盘测试
需要创建以下 secret 文件：

**config/ctp/ctp_md.secret.yaml**
```yaml
ctp:
  user_id: "你的用户ID"
  password: "你的密码"
```

**config/ctp/ctp_td.secret.yaml**
```yaml
ctp:
  user_id: "你的用户ID"
  password: "你的密码"
  investor_id: "你的投资者ID"
```

---

## ⚠️ 注意事项

1. 实盘测试前请确认 SimNow 服务器状态
2. 标准环境交易时段：周一至周五 9:00-15:00
3. 测试完成后务必运行 `stop_all.sh` 停止所有服务
4. 归档脚本在 `archive/` 目录，如需使用请查阅对应文档

---

## 📚 相关文档

- 架构说明: [docs/核心文档/CURRENT_ARCHITECTURE_FLOW.md](../docs/核心文档/CURRENT_ARCHITECTURE_FLOW.md)
- 使用指南: [docs/核心文档/USAGE.md](../docs/核心文档/USAGE.md)
- 构建指南: [docs/核心文档/BUILD_GUIDE.md](../docs/核心文档/BUILD_GUIDE.md)

---

**整理日期**: 2026-02-09
**核心脚本数**: 6 个
