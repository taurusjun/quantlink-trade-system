## 设计方案

### 核心思路

保持内部 `config.Config` 结构体不变，新增 C++ 格式文件解析器，在 main.go 中用 C++ CLI 参数替换 YAML 路径。

### CLI 参数对齐

```
Go trader --Live \
  --controlFile ./controls/day/control.ag2603.ag2605.par.txt.92201 \
  --strategyID 92201 \
  --configFile ./config/config_CHINA.92201.cfg \
  --adjustLTP 1 --printMod 1 --updateInterval 300000 \
  --logFile ./log/log.xxx.20260224
```

### 文件解析流程

```
1. --controlFile → ParseControlFile() → ControlConfig
   (baseName, modelFile, exchange, id, execStrat, startTime, endTime, secondName)

2. --configFile → ParseCfgFile() → CfgConfig
   (MDSHMKEY, ORSREQUESTSHMKEY, ORSRESPONSESHMKEY, CLIENTSTORESHMKEY, PRODUCT, exchange sections)

3. controlConfig.ModelFile → ParseModelFile() → ModelConfig
   (thresholds: BEGIN_PLACE, ALPHA, SIZE, MAX_SIZE 等; indicators)

4. 组合 → BuildConfig(controlCfg, cfgCfg, modelCfg, strategyID) → config.Config
   (保持现有 Config 结构，只是构建方式不同)

5. daily_init 路径: ../data/daily_init.<strategyID> (硬编码)
```

### 新增文件

| 文件 | 职责 |
|------|------|
| `pkg/config/control_file.go` | 解析 controlFile（单行空格分隔 8 字段） |
| `pkg/config/cfg_file.go` | 解析 .cfg INI 格式（支持 [SECTION]） |
| `pkg/config/model_file.go` | 解析 model file（阈值 key-value + indicator lines） |
| `pkg/config/build_config.go` | 从三个解析结果组合构建 Config |

### 修改文件

| 文件 | 变更 |
|------|------|
| `cmd/trader/main.go` | CLI 参数替换，调用新解析流程 |
| `data_new/config/config_CHINA.92201.cfg` | 新建 C++ 格式 .cfg |
| `data_new/data/daily_init.92201` | 新建 daily_init |
| `scripts/build_deploy_new.sh` | 更新 start_strategy.sh 生成 |

### baseName 到合约名映射

C++ 使用 `ag_F_3_SFE` 格式（product_F_month_exchange），Go 当前使用 `ag2603`。解析 controlFile 时需要做映射：
- `ag_F_3_SFE` → `ag2603`（取 product + 年份前两位 + month 两位）
- 年份从当前日期推导，或在 model file indicator 行中匹配

### SHM Key 映射

C++ 原系统使用十进制 SHM key（872/3872/4872/727272），新系统使用 hex key（0x1001/0x2001/0x3001/0x4001）。.cfg 文件中写新系统的 key。

### 阈值热加载

SIGUSR2 热加载改为重新读取 modelFile 而非 YAML。

### 向后兼容

保留 YAML config.Load() 函数不删除，但 main.go 不再调用。
