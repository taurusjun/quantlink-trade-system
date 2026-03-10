停止 Java CTP 实盘交易的所有进程。

## 步骤

### 1. 检查运行中的进程

```bash
ps aux | grep -E "md_shm_feeder|counter_bridge|TraderMain|OverviewServer" | grep -v grep
```

如果没有进程在运行，直接告知用户"没有运行中的交易进程"并结束。

### 2. 停止所有进程

```bash
cd deploy_java && ./scripts/stop_all.sh
```

### 3. 确认停止

```bash
ps aux | grep -E "md_shm_feeder|counter_bridge|TraderMain|OverviewServer" | grep -v grep
```

确认所有进程已停止。如果仍有残留进程，提醒用户可能需要手动 kill。

### 4. 检查端口释放

```bash
lsof -i :8082 -i :9201 -i :8080 2>/dev/null | grep LISTEN
```

如果端口仍被占用（zombie 进程），提醒用户可能需要 reboot。
