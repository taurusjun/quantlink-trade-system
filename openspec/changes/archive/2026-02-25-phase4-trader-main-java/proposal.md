# Phase 4: Trader 主程序入口

## Why

Phase 1-3 完成了 SHM/Connector/Core/Strategy 四层库代码的 C++ → Java 迁移，但缺少可运行的主程序入口。需要一个 Trader main 类来：
- 解析 C++ 格式的配置文件（controlFile、.cfg INI、model .par.txt）
- 初始化 SysV SHM 连接（行情/订单/回报/ClientStore）
- 创建策略实例并注册回调
- 启动轮询循环处理行情和回报
- 处理 Unix 信号（激活/平仓/重载/关闭）

## What Changes

1. **配置解析层** — 解析 C++ 遗留格式的 controlFile、.cfg INI、model .par.txt 文件
2. **Trader 主程序** — main() 入口，完整的初始化和运行流程
3. **信号处理** — 使用 sun.misc.Signal 处理 SIGUSR1/SIGUSR2/SIGTSTP/SIGTERM

## Capabilities

- java-config-parser: C++ 遗留格式配置文件解析（control/cfg/model/daily_init）
- java-trader-main: Trader 主程序入口和运行循环

## Impact

- 新增 `config/` 包：ConfigParser、ControlConfig、CfgConfig、ModelConfig
- 新增 `TraderMain.java` 主程序入口
- 可以连接现有 C++ 网关（md_shm_feeder + counter_bridge）运行模拟测试
