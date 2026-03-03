---
name: live-java
description: 启动 Java + CTP 实盘交易的完整流程。编译部署、启动网关、启动策略、检查日志。
---

执行 Java CTP 实盘交易的完整流程。

**输入**: 可选参数 `<strategy_id>` 和 `<session>`，默认 `92201`，session 自动检测（20:00-04:00=night，其余=day）。

## 步骤

### 1. 检查残留进程

```bash
ps aux | grep -E "md_shm_feeder|counter_bridge|TraderMain|OverviewServer" | grep -v grep
```

如果有残留进程，先执行 `cd deploy_java && ./scripts/stop_all.sh`，确认全部停止。

检查端口占用（zombie 进程问题）：
```bash
lsof -i :8082 -i :9201 -i :8080 2>/dev/null | grep LISTEN
```

如果有 UNE 状态的 zombie 进程占用端口，提醒用户需要 reboot 才能清理。

### 2. 编译部署（live 模式）

```bash
cd /Users/user/PWorks/RD/quantlink-trade-system
./scripts/build_deploy_java.sh --mode live
```

确认编译成功，检查 `deploy_java/bin/` 和 `deploy_java/lib/trader-1.0-SNAPSHOT.jar`。

### 3. 检查 daily_init 数据

```bash
cat deploy_java/live/data/daily_init.<strategy_id>
```

展示当前 avgPx、avgSpreadRatio 和持仓状态，确认数据合理。

如果 daily_init 不存在，提醒用户首次运行需要确认初始参数。

### 4. 检查 model 参数

```bash
cat deploy_java/live/models/model.*.par.txt.<strategy_id>
```

展示关键阈值：BEGIN_PLACE, LONG_PLACE, SHORT_PLACE, MAX_SIZE, AVG_SPREAD_AWAY 等。

### 5. 检查 CTP 配置

确认 CTP 配置文件存在：
- `deploy_java/config/ctp/ctp_md.secret.yaml`
- `deploy_java/config/ctp/ctp_td.secret.yaml`

不要展示文件内容（含敏感信息），只确认文件存在。

### 6. 启动网关（CTP 模式）

```bash
cd deploy_java
./scripts/start_gateway.sh ctp
```

等待 3 秒后检查日志：
```bash
tail -20 deploy_java/log/md_shm_feeder.$(date +%Y%m%d).log
tail -20 deploy_java/log/counter_bridge.$(date +%Y%m%d).log
```

确认：
- MD SHM Feeder: CTP 登录成功 + 合约订阅
- Counter Bridge: CTP 交易连接成功 + 持仓加载 + HTTP :8082 启动

### 7. 启动策略

```bash
cd deploy_java
./scripts/start_strategy.sh <strategy_id> <session>
```

### 8. 检查策略日志

等待 5 秒，检查策略日志：
```bash
tail -50 deploy_java/nohup.out.<strategy_id>
```

确认：
- 配置加载成功（symbols、thresholds）
- daily_init 加载（avgPx、avgSpreadRatio、ytd 持仓）
- 行情接收（spread 日志）
- AVG_SPREAD_AWAY 检查通过或已记录 drift warning
- API Server 启动（端口 9201）
- Dashboard 可访问

### 9. 汇报状态

展示：
- 所有进程 PID 和状态
- 策略加载参数（avgPx、avgSpreadRatio、持仓、阈值）
- 行情状态（spread 是否 valid）
- 策略激活状态（默认未激活，需手动激活）
- 各服务端口状态：
  - Dashboard: http://localhost:9201/dashboard.html
  - Overview: http://localhost:8080/
  - Counter Bridge HTTP: http://localhost:8082/health

提示用户：
- 策略需要手动激活（Dashboard UI 或 `curl -X POST http://localhost:9201/api/v1/strategy/activate`）
- 停止命令：`cd deploy_java && ./scripts/stop_all.sh`
- 实盘重启：`cd deploy_java && ./scripts/restart_live.sh <strategy_id>`

## 注意事项

- 如果 AVG_SPREAD_AWAY 在 inactive 状态触发，只会打 warning 不会退出（已修复）
- handleSquareON 激活时会自动用实时价差重置 avgSpreadRatio（已修复）
- 阈值支持热加载：修改 model 文件后调用 `curl -X POST http://localhost:9201/api/v1/strategy/reload-thresholds`
- CTP 交易时段：日盘 9:00-15:00，夜盘 21:00-02:30
- 如果端口 8082 被 zombie 进程占用，counter_bridge 的 HTTP 功能不可用但不影响交易
