## Why

Java 迁移审计发现多处代码从 Go 翻译而来，而非对齐 C++ 原代码，导致行为偏差。包含 CRITICAL 级别（loadThresholds 逻辑完全不同）和 HIGH 级别（UpdateActive/CheckLastUpdate/endPkt/INVALID 缺失）问题，在实盘环境下可能导致阈值错误、行情混淆、僵尸持仓等严重后果。

## What Changes

- **ConfigParser.loadThresholds() 重写**: 从 Go 风格的反射赋值改为 1:1 对齐 C++ `AddThreshold()` 的 switch-case 链（97 分支），包含时间单位转换、副作用赋值、字段重映射、特殊布尔处理
- **ConfigParser bu tickSize 修正**: 从 2.0 修正为 1.0（对齐 C++ 原代码）
- **CommonClient.sendInfraMDUpdate() 补齐**: 添加 endPkt==1 处理、checkLastUpdate() 僵尸行情检测、UpdateActive() 交易时段控制
- **CommonClient INVALID 判断修正**: 从 `bidPx==0 && askPx==0`（AND）修正为 `bidQty==0 || askQty==0`（OR），对齐 C++ `Tick::FillTick()`
- **SimConfig DateConfig 补齐**: 添加 startTimeEpoch/endTimeEpoch 字段、updateActive() 方法、initDateConfigEpoch() 时间初始化
- **TraderMain initDateConfigEpoch() 调用**: 策略初始化时计算交易时段 epoch 值

## Capabilities

### New Capabilities

（无新增能力，均为已有能力的修正）

### Modified Capabilities

- `strategy`: loadThresholds 阈值解析逻辑重写，INVALID 判断修正，DateConfig 交易时段控制补齐

## Impact

- **ConfigParser.java**: loadThresholds() 完全重写（~280行新代码），bu tickSize 修正
- **CommonClient.java**: sendInfraMDUpdate() 扩展 3 个处理块，新增 checkLastUpdate() 方法，INVALID 判断修正
- **SimConfig.java**: 新增 DateConfig 字段和方法（updateActive, initDateConfigEpoch）
- **TraderMain.java**: 新增 initDateConfigEpoch() 调用
- **已提交 commits**: 3eff909, 7976e1f, e3b0555
