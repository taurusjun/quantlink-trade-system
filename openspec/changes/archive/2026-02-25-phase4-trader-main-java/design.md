# Phase 4 设计：Trader 主程序入口

## 架构决策

### 1. 配置解析：复用 C++ 格式文件

Java Trader 必须与现有 C++ 网关基础设施兼容，因此直接解析 C++ 格式的配置文件：
- controlFile（空格分隔单行）
- .cfg INI 文件（SHM key 配置）
- model .par.txt 文件（阈值 + 指标定义）
- daily_init 文件（已在 PairwiseArbStrategy 中实现）

不引入新的 YAML 配置格式，保持与 Go 版 `BuildFromCppFiles()` 相同的解析逻辑。

### 2. 主程序流程（对齐 C++ main.cpp）

```
1. 解析 CLI 参数 (--Live, --controlFile, --strategyID, --configFile, etc.)
2. 解析 controlFile → ControlConfig
3. 解析 .cfg 文件 → CfgConfig (SHM keys/sizes)
4. 解析 model 文件 → ThresholdSet + 符号信息
5. 创建 Instrument (从 baseName 转换 symbol)
6. 创建 Connector (4个 SysV SHM 队列)
7. 创建 CommonClient (注册回调)
8. 创建 PairwiseArbStrategy
9. 加载 daily_init
10. 启动 Connector 轮询线程
11. 信号循环 (SIGUSR1=激活, SIGUSR2=重载, SIGTSTP=平仓, SIGTERM=关闭)
```

### 3. 信号处理

使用 `sun.misc.Signal` API（JDK 内置，无需外部依赖）处理 Unix 信号。

### 4. 包结构

```
com.quantlink.trader.config/
  ConfigParser.java      — 统一配置加载入口
  ControlConfig.java     — controlFile 解析结果
  CfgConfig.java         — .cfg INI 解析结果
  ModelConfig.java        — model 文件解析结果

com.quantlink.trader/
  TraderMain.java        — main() 入口
```

### 5. 符号名转换

C++ baseName 格式: `ag_F_3_SFE` → Java symbol: `ag2603`
转换规则: 提取产品名 + 月份代码 + yearPrefix

## 依赖关系

Phase 4 依赖 Phase 1-3 已完成的：
- `shm/` — SysVShm, MWMRQueue, ClientStore, Types, Constants
- `connector/` — Connector
- `core/` — Instrument, ThresholdSet, SimConfig, ConfigParams, CommonClient, OrderStats
- `strategy/` — ExecutionStrategy, ExtraStrategy, PairwiseArbStrategy
