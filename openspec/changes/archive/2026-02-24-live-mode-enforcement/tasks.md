## 1. 代码变更（已完成）

- [x] 1.1 main.go: 强制 `os.Args[1] == "--Live"`，否则打印用法退出
- [x] 1.2 main.go: 移除 `--Live` 后调用 `flag.Parse()`
- [x] 1.3 main.go: 策略激活改为 Live-only（不激活，等待 SIGUSR1）
- [x] 1.4 build_deploy_new.sh: `start_strategy.sh` 的 trader 命令首位添加 `--Live`
- [x] 1.5 main.go: daily_init 路径从 `../data` 修正为 `./data`（Go CWD=deploy_new/）

## 2. 编译验证（已完成）

- [x] 2.1 `go build` 编译通过

## 3. 模拟测试（已完成）

- [x] 3.1 `build_deploy_new.sh` 重新编译部署到 `deploy_new/`
- [x] 3.2 启动 gateway（sim 模式）验证启动脚本格式正确
- [x] 3.3 启动 strategy 92201 night，确认 `--Live` 模式日志输出
- [x] 3.4 确认策略以未激活状态启动（`策略未激活 (Live 模式，等待 SIGUSR1 激活)`）
- [x] 3.5 发送 SIGUSR1 激活策略，确认激活成功（`HandleSquareON: strategy reactivated`）
- [x] 3.6 确认行情接收和价差计算正常
- [x] 3.7 优雅关闭，确认 daily_init 保存正常

## 4. CTP 实盘测试（已完成）

- [x] 4.1 启动 CTP gateway + strategy 92201 night
- [x] 4.2 确认 `--Live` 模式日志 + 策略未激活
- [x] 4.3 SIGUSR1 激活策略
- [x] 4.4 确认 CTP 实盘行情接收正常（ag 价差 371-389）
- [x] 4.5 SIGTERM 优雅关闭 + daily_init 保存正常
