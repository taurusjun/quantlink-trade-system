## cpp-config-parser

### 概述

解析 C++ TradeBot 原格式配置文件，构建 Go Config 结构体。

### 需求

1. **ParseControlFile**: 解析 controlFile 单行格式
   - 输入: 文件路径
   - 格式: `baseName modelFile exchange id execStrat startTime endTime [secondName] [thirdName]`
   - 输出: ControlConfig 结构体

2. **ParseCfgFile**: 解析 .cfg INI 格式
   - 输入: 文件路径 + exchange 名称
   - 格式: `KEY = VALUE` + `[SECTION]` 分段
   - 输出: CfgConfig 结构体（SHM keys、PRODUCT 等）

3. **ParseModelFile**: 解析 model file
   - 输入: 文件路径
   - 格式: 每行 `KEY VALUE`（2 token = 阈值）或 `baseName type indName args...`（3+ token = indicator）
   - `#` 开头为注释
   - 输出: ModelConfig 结构体（ThresholdSet 参数 map）

4. **BuildConfig**: 组合构建 Config
   - 输入: ControlConfig + CfgConfig + ModelConfig + strategyID + 其他 CLI 参数
   - 输出: config.Config（保持现有结构不变）

5. **baseName 映射**: `ag_F_3_SFE` → `ag2603`
   - product = baseName 中 `_` 前部分（小写）
   - month = baseName 中 F 后的数字
   - year prefix = 从当前年份推导（20xx 的后两位）
   - 合约名 = product + year[2:] + month（两位补零）

### 接口

```go
type ControlConfig struct {
    BaseName    string // ag_F_3_SFE
    ModelFile   string // ./models/model.xxx
    Exchange    string // SFE
    ID          string // 16
    ExecStrat   string // TB_PAIR_STRAT
    StartTime   string // 0900
    EndTime     string // 1500
    SecondName  string // ag_F_5_SFE
    ThirdName   string // (optional)
}

type CfgConfig struct {
    MDShmKey          int
    OrsRequestShmKey  int
    OrsResponseShmKey int
    ClientStoreShmKey int
    MDShmSize         int
    OrsRequestShmSize int
    OrsResponseShmSize int
    Product           string
    Exchanges         string
}

type ModelConfig struct {
    Thresholds map[string]string // KEY -> VALUE
    Indicators []IndicatorDef
}
```
