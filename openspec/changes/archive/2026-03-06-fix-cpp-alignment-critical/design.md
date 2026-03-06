## Context

Java 迁移代码审计发现多处逻辑源自 Go 翻译而非 C++ 原代码，涉及阈值解析、行情处理、交易时段控制等核心路径。已有三个 commit 完成修复（3eff909, 7976e1f, e3b0555），本文档记录设计决策。

**受影响文件**:
- `ConfigParser.java` — loadThresholds() 重写
- `CommonClient.java` — sendInfraMDUpdate() 补齐 + INVALID 修正
- `SimConfig.java` — DateConfig 字段和方法
- `TraderMain.java` — initDateConfigEpoch() 调用

## Goals / Non-Goals

**Goals:**
- ConfigParser.loadThresholds() 与 C++ AddThreshold() 逐分支对齐（97 个 case）
- CommonClient 补齐 C++ SendInfraMDUpdate 中的 endPkt、CheckLastUpdate、UpdateActive 三个处理块
- Tick INVALID 判断对齐 C++ FillTick() 的 OR 逻辑
- SimConfig DateConfig 支持交易时段控制（含夜盘跨日）

**Non-Goals:**
- 不处理 MEDIUM 级别问题（m_sendMail、fillOrderBook 缺失字段等）
- 不修改 Go 代码或 C++ gateway 代码
- 不修改现有测试用例

## Decisions

### 1. loadThresholds: switch-case vs 反射

**选择**: 显式 switch-case 链（280 行）

**理由**: C++ AddThreshold() 使用 if-else 链处理每个参数名，包含大量副作用（SIZE→BEGIN_SIZE/BID_SIZE/ASK_SIZE、时间单位转换 ×1e6/×1e9、字段重映射 DECAY→DECAY1）。Go 风格的反射无法表达这些副作用，且无法在编译期发现遗漏。switch-case 与 C++ 1:1 对应，便于对照审查。

### 2. INVALID 判断: OR vs AND

**选择**: `bidQty[0]==0 || askQty[0]==0`（OR）

**理由**: C++ Tick::FillTick() 原代码: `if (bidQty[0] == 0 || askQty[0] == 0) { tickStatus = Tick::Status::INVALID; }`。Java 之前误用 AND 逻辑，只在买卖量同时为零时标记无效，导致单边无量时仍处理行情。

### 3. simActive 默认值: false vs true

**选择**: `simActive = false`（对齐 C++ DateConfig::Reset()）

**理由**: C++ `DateConfig::Reset()` 将 `m_simActive` 设为 false，启动后由 `UpdateActive()` 根据实际时间判定。Java 之前默认 true，不影响功能（因 startTimeEpoch=0, endTimeEpoch=MAX 时 updateActive 始终返回 true），但语义不对齐。

### 4. CheckLastUpdate 实现位置

**选择**: 在 CommonClient.sendInfraMDUpdate() 中实现

**理由**: 对齐 C++ CommonClient::SendInfraMDUpdate() 调用链：endPkt 处理 → CheckLastUpdate → SendINDUpdate。C++ 在 CheckLastUpdate 中遍历 simConfigs/instruMap 检测僵尸行情，触发时设置 onExit/onCancel/onFlat。

## Risks / Trade-offs

- **[Risk] switch-case 维护成本** → 新增阈值参数时需手动添加 case。Mitigation: 有 C++ 对照，且阈值参数集稳定不常变化
- **[Risk] simActive=false 启动瞬间** → 在 initDateConfigEpoch() 前 simActive=false，策略不会处理行情。Mitigation: initDateConfigEpoch() 在策略启动前调用，且未配置时间时默认设为 true
- **[Risk] CheckLastUpdate 120s 超时** → 可能在行情中断时过早触发平仓。Mitigation: 与 C++ 行为一致，updateInterval=120s 是合理默认值
