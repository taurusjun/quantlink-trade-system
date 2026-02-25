## Context

Go trader 的 CLI 已在前一个 change（cli-align-cpp-tradebot）中从 `-config YAML` 迁移到 `--controlFile + --configFile + --strategyID`。但 C++ TradeBot 的第一个参数 `argv[1]` 是模式标志（`--Live/--Sim/--Regress/--LeadLag`），此部分尚未对齐。

经调研，C++ Sim/Regress/LeadLag 模式依赖 `hftbase/ExchangeSimulator/ExchSim` 磁盘回放架构（MDDUMP 文件），与 Go 的 SysV SHM 直连架构完全不同，因此仅保留 `--Live`。

## Goals / Non-Goals

**Goals:**
- `argv[1]` 必须是 `--Live`，匹配 C++ `GetMode()` 校验
- 策略以 `m_Active = false` 启动，SIGUSR1 激活（C++ Live 分支行为）
- `build_deploy_new.sh` 生成的脚本自动添加 `--Live`

**Non-Goals:**
- 不实现 Sim/Regress/LeadLag 模式（架构不适用）
- 不改变现有信号处理逻辑（SIGUSR1/SIGUSR2/SIGTSTP 已实现）

## Decisions

### Decision 1: os.Args 操作方式

在 `flag.Parse()` 之前手动检查 `os.Args[1]`，确认后用 `os.Args = append(os.Args[:1], os.Args[2:]...)` 移除 `--Live`，使后续 flag 正常解析。

**备选方案**: 自定义 FlagSet 跳过 `argv[1]` — 更复杂且不如直接操作 os.Args 清晰。

### Decision 2: 仅支持 --Live，硬拒其他模式

传入 `--Sim`/`--Regress` 等直接报错退出，不做静默降级。原因：Sim/Regress 的行为期望（磁盘回放、无真实交易）与 Go trader 的 SHM 实时架构完全不同，静默降级会导致误操作。

## Risks / Trade-offs

- [风险] 操作员习惯性传入 `--Sim` → 会被明确的错误消息拦截，提示仅支持 `--Live`
- [取舍] 不支持 Sim 模式的自动激活 → Go trader 的模拟测试通过 `start_gateway.sh sim` 控制网关层模拟，策略层统一用 Live 行为
