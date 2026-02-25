# Java Trader Main Spec

## 概述

Trader 主程序入口，完整实现初始化 → 运行 → 关闭流程。对齐 C++ main.cpp 和 Go main.go。

## CLI 参数

必须参数:
- 位置参数 1: `--Live` (模式标识)
- `-controlFile`: controlFile 路径
- `-strategyID`: 策略 ID (整数)
- `-configFile`: .cfg 文件路径

可选参数:
- `-dataDir`: 数据目录 (默认 `./data`)
- `-yearPrefix`: 年份前缀 (默认 `26`)
- `-adjustLTP`: 调整 LTP 标志 (默认 0)
- `-printMod`: 打印模式 (默认 0)
- `-updateInterval`: 更新间隔 (默认 300000)
- `-logFile`: 日志文件路径

## 初始化流程

1. 解析 CLI → 验证必须参数
2. 加载 controlFile → ControlConfig
3. 加载 .cfg → CfgConfig (SHM keys)
4. 加载 model → ThresholdSet
5. baseName → symbol 转换 (使用 yearPrefix)
6. 创建 Instrument (从产品查表获取 tickSize/lotSize/priceMultiplier)
7. 创建 Connector (4 SHM queues)
8. 创建 CommonClient (绑定 Connector)
9. 创建 PairwiseArbStrategy (双腿)
10. 加载 daily_init (avgSpreadRatio + 昨仓)
11. 启动 Connector 轮询
12. 进入信号循环

## 信号处理

- SIGUSR1: 激活策略 (active=true)
- SIGUSR2: 热加载阈值 (重读 model 文件)
- SIGTSTP: 平仓 (handleSquareoff)
- SIGINT/SIGTERM: 优雅关闭

## 关闭流程

1. 策略平仓 + 保存 daily_init
2. 停止 Connector 轮询
3. 释放 SHM 资源

## 产品查表

| 产品 | tickSize | lotSize | priceMultiplier | priceFactor |
|------|----------|---------|-----------------|-------------|
| ag   | 1.0      | 15.0    | 15.0            | 1.0         |
| au   | 0.02     | 1000.0  | 1000.0          | 1.0         |

## 测试要求

- CLI 参数解析测试
- 完整初始化流程测试 (使用 mock)
- 信号处理测试
