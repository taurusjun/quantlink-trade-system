## Context

tbsrc-golang trader 不使用 control/model 文件体系，而是一个自包含的 YAML 配置文件，通过 `-config` 指定。每个策略 ID 对应一个独立配置文件（如 `trader.92201.yaml`）。

当前 `start_strategy.sh` 逻辑：
```
查找 controls/{session}/control.*.{strategy_id} → 传递 --controlFile --strategyID --Live
```

新逻辑：
```
查找 config/trader.{strategy_id}.yaml → 传递 -config -data
```

## Goals / Non-Goals

**Goals:**
- `start_strategy.sh` 适配 tbsrc-golang trader 的 `-config` / `-data` 接口
- 为策略 92201 (ag2603/ag2605) 和 92202 (au2604/au2606) 创建 tbsrc-golang 格式配置
- 配置放在 `data_new/config/` 中（通过 build 脚本合并到 deploy_new）

**Non-Goals:**
- 不保留 legacy golang trader 兼容
- 不修改 control/model 文件（保留但不再使用）

## Decisions

**决策 1: 配置文件命名**
- 格式: `config/trader.{strategy_id}.yaml`
- 示例: `config/trader.92201.yaml`, `config/trader.92202.yaml`
- start_strategy.sh 按 strategy_id 自动查找

**决策 2: data 目录**
- `-data` 指向 `./data/`（deploy_new 内部）
- daily_init 文件路径: `./data/daily_init.{strategy_id}.json`

## Risks / Trade-offs

- [Risk] 旧 controls/models 不再使用 → 保留在 deploy_new 但不影响运行
