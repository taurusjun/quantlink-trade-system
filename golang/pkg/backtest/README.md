# Backtest Package

QuantLink 回测系统核心包

## 功能特性

- ✅ CSV 历史数据加载和回放
- ✅ 三种回放模式（实时/快速/极速）
- ✅ 订单撮合模拟
- ✅ 完整的绩效统计（Sharpe, Sortino, Drawdown等）
- ✅ 多格式报告生成（Markdown, JSON, CSV）
- ✅ 批量回测支持

## 组件结构

```
backtest/
├── config.go          # 配置加载和验证
├── types.go           # 类型定义
├── datareader.go      # 历史数据读取和回放
├── order_router.go    # 订单路由和撮合引擎
├── statistics.go      # 绩效统计计算
├── report.go          # 报告生成
└── runner.go          # 回测主控制器
```

## 快速开始

```go
package main

import (
    "log"
    "github.com/yourusername/quantlink-trade-system/pkg/backtest"
)

func main() {
    // 加载配置
    config, err := backtest.LoadBacktestConfig("config/backtest.yaml")
    if err != nil {
        log.Fatal(err)
    }

    // 创建并运行回测
    runner, err := backtest.NewBacktestRunner(config)
    if err != nil {
        log.Fatal(err)
    }

    result, err := runner.Run()
    if err != nil {
        log.Fatal(err)
    }

    // 打印结果
    log.Printf("Total PNL: %.2f", result.TotalPNL)
    log.Printf("Sharpe Ratio: %.2f", result.SharpeRatio)
}
```

## 配置示例

```yaml
backtest:
  start_date: "2026-01-01"
  end_date: "2026-01-31"
  data:
    data_path: "./data/market_data"
    symbols: ["ag2502", "ag2504"]
  replay:
    mode: "fast"
    speed: 10.0
  initial:
    capital: 1000000.0
```

## API 文档

### BacktestRunner

主控制器，协调所有组件。

```go
runner, err := backtest.NewBacktestRunner(config)
result, err := runner.Run()
```

### HistoricalDataReader

历史数据读取和回放。

```go
reader := backtest.NewHistoricalDataReader(config, natsConn)
err := reader.LoadData()
err := reader.Replay()
```

### BacktestOrderRouter

订单路由和撮合。

```go
router := backtest.NewBacktestOrderRouter(config, port)
err := router.SubmitOrder(orderRequest)
```

### BacktestStatistics

绩效统计。

```go
stats := backtest.NewBacktestStatistics(config)
result := stats.GenerateReport()
stats.PrintSummary()
```

### ReportGenerator

报告生成。

```go
generator := backtest.NewReportGenerator(config, result)
err := generator.GenerateMarkdown()
err := generator.GenerateJSON()
```

## 性能指标

| 指标 | 数值 |
|------|------|
| 数据加载 | ~1000 ticks/ms |
| 回放速度（fast 10x） | ~500,000 ticks/s |
| 回放速度（instant） | ~2,000,000 ticks/s |
| 内存占用 | ~100MB (100万ticks) |

## 测试

```bash
# 运行单元测试
go test ./...

# 运行集成测试
go test -tags=integration ./...

# 性能测试
go test -bench=. -benchmem
```

## 最佳实践

1. **数据质量**: 使用高质量的 tick 数据
2. **参数保守**: 滑点和手续费宁愿高估
3. **样本外验证**: 训练集和测试集分离
4. **小批量测试**: 先小规模验证再大规模运行

## 已知限制

1. 当前仅支持简单撮合（价格匹配）
2. 不支持订单簿深度模拟
3. 不支持部分成交
4. 回测模式下不支持实时风控

## Phase 3: 参数优化与生产部署 ✅

### 新增功能

- ✅ **参数优化器** (`optimizer.go`) - Grid Search 网格搜索
- ✅ **参数导出器** (`param_exporter.go`) - 导出最优参数
- ✅ **生产配置生成器** (`production_config.go`) - 生成生产配置
- ✅ **优化工具** (`cmd/backtest_optimize/`) - 命令行工具

### 使用示例

```bash
# 1. 参数优化
./bin/backtest_optimize \
  -action optimize \
  -params "entry_zscore:1.5:3.0:0.1,exit_zscore:0.5:1.5:0.1" \
  -goal sharpe \
  -workers 8

# 2. 导出生产配置
./bin/backtest_optimize \
  -action export \
  -current optimal_params_ag2502_ag2504_20260124.yaml \
  -strategy-id 92201

# 3. 参数对比
./bin/backtest_optimize \
  -action compare \
  -baseline old.yaml \
  -current new.yaml
```

详见：[参数优化使用指南](../../../docs/回测_参数优化使用指南_2026-01-24-20_30.md)

---

## 未来计划

- [ ] 高级撮合引擎（订单簿深度）
- [ ] 遗传算法优化（GA）
- [ ] 分布式回测
- [ ] 可视化图表
- [ ] 更多数据源（Parquet, Database）

## 许可

内部使用
