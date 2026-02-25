# 提案: 修复方法命名一致性 + symbolID 路由

## 为什么

Java 迁移代码中存在两个问题：
1. **方法命名违规**: 部分方法名未与 C++ 原代码保持一致（如 `loadDailyInit` 不存在于 C++，`start` 应为 `startAsync`）
2. **symbolID 路由不匹配**: Java 使用 `hashCode()` 生成 symbolID，而 C++ md_shm_feeder 从不设置 `m_symbolID`（始终为 0），Go 版本使用 symbol 字符串路由。导致 CTP 实盘行情无法路由到策略。

## 变更内容

### 方法命名修复（与 C++ 对齐）

| 类 | 当前 Java 方法 | C++ 原方法 | 修改为 |
|----|-----------|---------|-------|
| Connector | `start()` | `StartAsync()` | `startAsync()` |
| Connector | `pollMD()` | `HandleLiveMdUpdates()` | `handleLiveMdUpdates()` |
| Connector | `pollORS()` | `HandleOrderResponse()` | `handleOrderResponse()` |
| Connector | `nextOrderID()` | `GetUniqueOrderNumber()` | `getUniqueOrderNumber()` |
| Instrument | `getMidPrice()` | `calculate_MIDPrice()` | `calculateMIDPrice()` |
| Instrument | `getMswPrice()` | `calculate_MSWPrice()` | `calculateMSWPrice()` |
| Instrument | `getLtpPrice()` | `calculate_LTPPrice()` | `calculateLTPPrice()` |
| Instrument | `getMswMidPrice()` | `calculate_MSWMIDPrice()` | `calculateMSWMIDPrice()` |
| ControlConfig | `parse()` | `LoadControlFile()` | `loadControlFile()` |
| CfgConfig | `parse()` | `LoadCfg()` | `loadCfg()` |
| ModelConfig | `parse()` | `LoadModelFile()` | `loadModelFile()` |
| PairwiseArbStrategy | `loadDailyInit()` | (构造函数内联逻辑) | 删除，逻辑合并到构造函数 |

### symbolID 路由修复

改为按 symbol 字符串路由（与 Go 版本一致），因为：
- C++ md_shm_feeder 不设置 `m_symbolID`（memset 后为 0）
- Go 版本 Client.OnMDUpdate() 使用 `extractSymbol()` 按字符串路由
- `ConfigParams.simConfigMap` 改为 `Map<String, List<SimConfig>>`

## 能力

- method-naming: 方法命名修复
- symbol-routing: symbolID → symbol 字符串路由

## 影响

- 所有 Java 策略/核心代码文件
- 测试文件需同步更新方法调用
- 不影响 C++ 网关代码
