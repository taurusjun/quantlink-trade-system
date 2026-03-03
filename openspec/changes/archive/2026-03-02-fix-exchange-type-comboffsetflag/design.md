## Context

Java 策略通过 SHM RequestMsg 向 counter_bridge 发送订单。counter_bridge 的 `SetCombOffsetFlag()` 根据 `req.Exchange_Type` 判断是否为 SHFE 交易所，决定使用 CLOSE_TODAY（今仓平仓）还是 CLOSE_YESTERDAY（昨仓平仓）。

C++ 原代码在 `CommonClient::FillReqInfo()` 中设置 `m_reqMsg.Exchange_Type = m_exchangeType`，其中 `m_exchangeType` 从 cfg 文件的 EXCHANGES 字段映射（如 "SFE" → CHINA_SHFE=57）。Java 迁移时遗漏了这一行，导致 Exchange_Type 默认为 0，SHFE 合约被误判为非 SHFE。

## Goals / Non-Goals

**Goals:**
- 修复 Java 发单 Exchange_Type 缺失问题，恢复 SHFE 合约正确的开平标志
- 提高 counter_bridge 启动时持仓初始化的可靠性
- 使 CTP 错误日志在 UTF-8 终端可读

**Non-Goals:**
- 不重构 SetCombOffsetFlag 整体逻辑
- 不修改 CTP 插件的 SetOpenClose 逻辑（已禁用，由 counter_bridge 统一管理）
- 不修改 RequestMsg 结构体布局

## Decisions

### Decision 1: Exchange 字符串→字节映射位置

**选择**: 在 `CfgConfig` 中添加静态方法 `parseExchangeType(String)`

**理由**: C++ 原代码在 `CommonClient.cpp:850-901` 中用 if-else 链做映射。Java 中 CfgConfig 已持有 `exchanges` 字段，在此添加映射方法最自然。CommonClient 通过 setter 接收 exchangeType 值。

**替代方案**: 在 Constants 中添加 Map — 但 Constants 是纯常量类，不适合放逻辑。

### Decision 2: exchangeType 传递路径

**选择**: `TraderMain.init()` 中从 `CfgConfig.exchanges` 解析后调用 `client.setExchangeType(byte)`

**理由**: 与 C++ 一致 — C++ 在 `CommonClient::Initialize()` 后通过 simConfig 循环设置 `m_exchangeType`。TraderMain 已有 cfgConfig 引用，是最合适的设置点。

### Decision 3: counter_bridge 持仓 fallback 策略

**选择**: `QueryPositions()` 返回空时，尝试 `GetCachedPositions()` 作为 fallback

**理由**: `GetCachedPositions()` 读取 `m_positions`（由 `UpdatePositionFromCTP()` 在登录成功后填充），不需要额外 CTP 查询请求，不受限频影响。两个数据源的 PositionInfo 结构完全一致。

### Decision 4: GBK→UTF-8 转码方案

**选择**: 使用 `iconv` 库在 `ctp_td_plugin.cpp` 中添加 `GbkToUtf8()` 工具函数

**理由**: macOS 和 Linux 均自带 iconv。在输出 `pRspInfo->ErrorMsg` 的每个位置调用转码。CTP API 的 ErrorMsg 字段固定为 GBK 编码。

## Risks / Trade-offs

- [Risk] `GetCachedPositions()` 在 CTP 登录回调完成前可能为空 → 已有 3 秒等待（counter_bridge L977-978）确保登录和持仓回调完成
- [Risk] iconv 在某些极简 Docker 镜像中可能缺失 → 当前部署环境为 macOS，无此风险
- [Risk] CfgConfig.exchanges 字段可能不匹配已知交易所名 → 添加日志 warning 并默认为 0（与当前行为一致，不会更差）
