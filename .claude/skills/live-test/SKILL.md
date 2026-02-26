---
name: live-test
description: 启动 CTP 实盘测试的完整流程。编译部署、启动网关、启动策略、检查日志。
---

执行 CTP 实盘测试的完整流程。

**输入**: 可选参数 `<strategy_id>` 和 `<session>`，默认 `92201 day`。

## 步骤

### 1. 检查残留进程

```bash
ps aux | grep -E "md_shm_feeder|counter_bridge|trader|webserver" | grep -v grep
```

如果有残留进程，先执行 `cd deploy_new && ./scripts/stop_all.sh`，确认全部停止。

### 2. 编译部署（live 模式）

```bash
./scripts/build_deploy_new.sh --mode live
```

确认编译成功，检查 `deploy_new/bin/` 下的二进制文件。

### 3. 检查 daily_init 数据

```bash
cat deploy_new/data/live/daily_init.<strategy_id>
```

展示当前 avgPx 和持仓状态，确认数据合理。

### 4. 检查 CTP 配置

确认 CTP 配置文件存在：
- `deploy_new/config/ctp/ctp_md.secret.yaml`
- `deploy_new/config/ctp/ctp_td.secret.yaml`

### 5. 启动网关组件

由于 `start_gateway.sh` 有交互式确认（`read -p`），需要手动逐个启动：

```bash
cd deploy_new
mkdir -p log ctp_flow

# 清理共享内存
ipcs -m 2>/dev/null | grep "$(whoami)" | awk '{print $2}' | xargs -I{} ipcrm -m {} 2>/dev/null || true

# 启动 MD SHM Feeder (CTP)
QUEUE_SIZE=2048
./bin/md_shm_feeder "ctp:config/ctp/ctp_md.secret.yaml" --queue-size "$QUEUE_SIZE" > "log/md_shm_feeder.$(date +%Y%m%d).log" 2>&1 &
```

等待 2 秒后检查日志，确认 CTP 登录成功和合约订阅。

```bash
# 启动 Counter Bridge (CTP)
./bin/counter_bridge ctp:"config/ctp/ctp_td.secret.yaml" > "log/counter_bridge.$(date +%Y%m%d).log" 2>&1 &
```

等待 3 秒后检查日志，确认 CTP 交易连接成功。

```bash
# 启动 WebServer
./bin/webserver -port 8080 > "log/webserver.$(date +%Y%m%d).log" 2>&1 &
```

### 6. 记录网关模式

```bash
echo "ctp" > .gateway_mode
```

### 7. 启动策略

```bash
./scripts/start_strategy.sh <strategy_id> <session>
```

### 8. 检查日志

等待 3 秒，检查策略日志：
- 确认配置加载成功（symbols、thresholds）
- 确认 daily_init 加载（avgPx、ytd 持仓）
- 确认行情接收（spread 日志或 AVG_SPREAD_AWAY 日志）
- 确认 API Server 启动（端口 9201）

```bash
tail -30 deploy_new/log/trader.<strategy_id>.$(date +%Y%m%d).log
```

### 9. 汇报状态

展示：
- 所有进程 PID 和状态
- 策略加载参数（avgPx、持仓、阈值）
- 行情状态（spread 是否 valid）
- 策略激活状态（Live 模式默认未激活，需手动激活）
- Web Dashboard 地址：http://localhost:9201/dashboard
- Overview 地址：http://localhost:8080

提示用户：
- 策略需要手动激活（Web UI 或 `kill -SIGUSR1 <pid>`）
- 停止命令：`cd deploy_new && ./scripts/stop_all.sh`

## 注意事项

- 如果 AVG_SPREAD_AWAY 频繁触发，可能需要调整 daily_init 的 avgPx 或模型的 AVG_SPREAD_AWAY 参数
- 阈值支持热加载：修改 model 文件后调用 `curl -X POST http://localhost:9201/api/v1/strategy/reload-thresholds`
- 非交易时间 CTP 行情可能无数据，但连接会正常建立
