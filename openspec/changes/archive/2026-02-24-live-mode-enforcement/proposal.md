## Why

Go trader 的 CLI 启动方式需要与 C++ TradeBot 原代码完全对齐。C++ `main.cpp:386` 要求 `argv[1]` 必须是 `--Regress/--Sim/--Live/--LeadLag` 之一，其中 Sim/Regress/LeadLag 依赖 hftbase `ExchSim` 磁盘回放架构，不适用于 Go 的 SysV SHM 直连架构。因此 Go trader 仅保留 `--Live` 模式，强制要求启动时传入 `--Live` 作为第一个参数。

## What Changes

- Go trader `main.go` 强制 `os.Args[1] == "--Live"`，否则报错退出（对齐 C++ `TradeBotUtils.cpp:2590-2608 GetMode()`）
- 移除 `--Live` 参数后再调用 `flag.Parse()`，使后续 `--controlFile` 等 flag 正常解析
- 策略启动时不激活（`m_Active = false`），等待 SIGUSR1 信号激活（对齐 C++ `ExecutionStrategy.cpp:377-380` Live 分支）
- `build_deploy_new.sh` 生成的 `start_strategy.sh` 在 trader 命令行首位添加 `--Live`

## Capabilities

### New Capabilities
- `live-mode-cli`: 强制 `--Live` 模式参数校验 + 策略启动不激活行为

### Modified Capabilities

## Impact

- `tbsrc-golang/cmd/trader/main.go` — CLI 入口，模式校验 + 激活逻辑
- `scripts/build_deploy_new.sh` — 生成的启动脚本添加 `--Live` 参数
