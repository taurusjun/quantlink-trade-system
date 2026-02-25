## ADDED Requirements

### Requirement: Go trader 强制 --Live 作为第一个 CLI 参数

Go trader 启动时 SHALL 要求 `os.Args[1] == "--Live"`，否则打印用法并退出。对齐 C++ `main.cpp:386` 的 `argv[1]` 模式校验（`GetMode()`）。

#### Scenario: 正确传入 --Live 参数
- **WHEN** 用户执行 `./trader --Live --controlFile xxx --strategyID 92201 --configFile xxx.cfg`
- **THEN** trader 正常启动，打印 `*****TradeBot started in Live Mode*****`

#### Scenario: 未传入 --Live 参数
- **WHEN** 用户执行 `./trader --controlFile xxx --strategyID 92201`（缺少 --Live）
- **THEN** trader 打印用法示例并以 exit code 1 退出

#### Scenario: 传入 --Sim 参数
- **WHEN** 用户执行 `./trader --Sim --controlFile xxx --strategyID 92201`
- **THEN** trader 打印 "Go trader 仅支持 --Live 模式" 并以 exit code 1 退出

### Requirement: Live 模式策略启动时不激活

在 Live 模式下，策略 SHALL 以 `m_Active = false` 状态启动，等待 SIGUSR1 信号激活。对齐 C++ `ExecutionStrategy.cpp:377-380`。

#### Scenario: 策略启动后等待激活
- **WHEN** trader 启动并完成初始化
- **THEN** 策略处于未激活状态，日志输出 "策略未激活 (Live 模式，等待 SIGUSR1 激活)"

#### Scenario: 收到 SIGUSR1 后激活
- **WHEN** trader 收到 SIGUSR1 信号
- **THEN** 策略调用 `HandleSquareON()` 激活

### Requirement: build_deploy_new.sh 生成的启动脚本传入 --Live

`start_strategy.sh` 生成的 trader 启动命令 SHALL 在所有 flag 之前添加 `--Live` 作为第一个参数。

#### Scenario: 生成的启动命令格式正确
- **WHEN** `build_deploy_new.sh` 生成 `start_strategy.sh`
- **THEN** 生成的 trader 命令格式为 `./trader --Live --controlFile ... --strategyID ... --configFile ...`
