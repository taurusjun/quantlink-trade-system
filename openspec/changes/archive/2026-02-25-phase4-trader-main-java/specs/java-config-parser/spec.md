# Java Config Parser Spec

## 概述

解析 C++ 遗留格式配置文件，提供与 Go `config.BuildFromCppFiles()` 等价的功能。

## ControlConfig

解析 controlFile 单行格式:
```
baseName modelFile exchange id execStrat startTime endTime [secondName [thirdName]]
```

字段:
- baseName: 主合约基础名 (如 `ag_F_3_SFE`)
- modelFile: model 文件路径
- exchange: 交易所代码
- id: 策略数字 ID
- execStrat: 策略类型 (如 `TB_PAIR_STRAT`)
- startTime/endTime: 交易时段 (HHMM)
- secondName: 第二腿基础名 (可选)

## CfgConfig

解析 .cfg INI 格式:
```ini
EXCHANGES=CHINA_SHFE
PRODUCT=AG
[CHINA_SHFE]
MDSHMKEY=4097
ORSREQUESTSHMKEY=8193
ORSRESPONSESHMKEY=12289
CLIENTSTORESHMKEY=16385
MDSHMSIZE=2048
ORSREQUESTSHMSIZE=1024
ORSRESPONSESHMSIZE=1024
```

## ModelConfig

解析 model .par.txt 格式:
- 2-token 行: `KEY VALUE` → 阈值参数
- 3+ token 行: 指标定义 (忽略)
- `#` 开头且后接特殊关键字 (DEP_STD_DEV, LOOKAHEAD, TRGT_STD_DEV): 注释但需解析
- 纯 `#` 注释行: 忽略

## 符号名转换

`baseNameToSymbol(baseName, yearPrefix)`:
- 输入: `ag_F_3_SFE`, yearPrefix=`26`
- 解析: 产品=`ag`, 月份代码=`3`
- 输出: `ag2603`

月份代码映射: 1→01, 2→02, ..., 9→09, X→10, Y→11, Z→12

## 测试要求

- 解析真实格式的 controlFile
- 解析真实格式的 .cfg 文件
- 解析真实格式的 model 文件
- 符号名转换测试
