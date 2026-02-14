# 移除 CTP 行情插件调试打印代码

## 为什么要做这个改动

CTP 行情插件的 `OnRtnDepthMarketData()` 中有一段硬编码的调试打印代码——这是行情数据管线中最热的路径。它在每个 tick 都会构造一个 `std::string`，当合约匹配 `"ag2603"` 时还会向 stdout 输出 6 行内容。这在生产环境中增加了不必要的延迟和 I/O 开销。

## 改动内容

- 删除 `ctp_md_plugin.cpp` 中第 315-323 行的调试 `cout` 代码块
- 不影响行情数据处理的任何业务逻辑——该代码块纯粹是调试输出

## 能力变更

### 修改的能力
- `ctp-market-data-processing`：移除 tick 处理热路径中的调试开销

## 影响范围

- `gateway/plugins/ctp/src/ctp_md_plugin.cpp`：从 `OnRtnDepthMarketData()` 中移除 9 行调试代码
