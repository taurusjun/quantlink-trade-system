## ADDED Requirements

### Requirement: 系统架构全景文档

提供一份完整的系统架构理解文档，覆盖 QuantLink Trade System 的所有核心方面。

#### Scenario: 新开发者阅读文档后能理解系统全貌

- **WHEN** 新开发者阅读 `docs/系统分析/系统架构全景_2026-02-24.md`
- **THEN** 能够回答以下问题：
  - 系统有哪些进程？各自的职责是什么？
  - 进程之间如何通信？SHM key 分配是怎样的？
  - 行情数据从哪里来，经过哪些组件，最终到达策略？
  - 策略发出的订单经过哪些组件到达交易所？
  - 回报如何从交易所返回到策略？
  - 消息体（MarketUpdateNew、RequestMsg、ResponseMsg）的关键字段有哪些？
  - Go 代码的包结构是怎样的？各包负责什么？
  - 配置文件的结构是怎样的？
  - 系统的启动顺序是什么？
  - 为什么选择 SysV MWMR SHM 而不是 NATS/gRPC？

#### Scenario: 文档结构完整

- **WHEN** 检查文档目录结构
- **THEN** 包含以下章节：
  - 系统总览（进程列表、架构图）
  - 核心组件（C++ 和 Go 组件详细说明）
  - SysV MWMR SHM 队列设计（key 分配、内存布局、队列元素大小）
  - 消息结构体（MarketUpdateNew、RequestMsg、ResponseMsg 的字段表）
  - 数据流（行情路径、订单路径、回报路径、客户端注册路径）
  - Go 包职责（每个 pkg/ 下的包及其作用）
  - 配置体系（YAML 结构、daily_init、热重载）
  - 启动与停止流程
  - 关键设计决策

#### Scenario: 文档遵循项目规范

- **WHEN** 检查文档格式
- **THEN** 文档使用中文编写，技术术语保留英文
- **AND** 架构图使用 ASCII art
- **AND** 文件命名遵循 `模块_摘要_YYYY-MM-DD.md` 格式
- **AND** 放置在 `docs/系统分析/` 目录
