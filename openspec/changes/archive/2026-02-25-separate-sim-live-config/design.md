## Context

data_new 已重组为 common/sim/live 三层结构，build_deploy_new.sh 已支持 `--mode sim|live`。但 deploy_new/data/ 仍是扁平结构 — 所有模式共享同一份 `daily_init.*`。运行 sim 后再切换 live（或反过来），`daily_init` 中的持仓和均价状态会交叉污染。

当前数据流:
```
data_new/sim/data/daily_init.92201  ──→  deploy_new/data/daily_init.92201  ←── 运行时 sim 更新
data_new/live/data/daily_init.92201 ──→  deploy_new/data/daily_init.92201  ←── 运行时 live 更新
                                              ^^ 同一个文件！
```

目标数据流:
```
data_new/sim/data/daily_init.92201  ──→  deploy_new/data/sim/daily_init.92201  ←── 运行时 sim 更新
data_new/live/data/daily_init.92201 ──→  deploy_new/data/live/daily_init.92201 ←── 运行时 live 更新
                                              ^^ 彻底隔离
```

## Goals / Non-Goals

**Goals:**
- deploy_new/data/ 按模式分目录（data/sim/、data/live/），运行时状态彻底隔离
- start_gateway.sh 记录当前模式，start_strategy.sh 自动选择对应数据目录
- trader 通过 `-dataDir` flag 支持可配置数据目录

**Non-Goals:**
- 不改变 daily_init 文件格式
- 不改变策略逻辑

## Decisions

### 1. deploy_new/data/ 分为 data/sim/ 和 data/live/

build_deploy_new.sh 将模式数据部署到 `deploy_new/data/${DEPLOY_MODE}/` 而非 `deploy_new/data/`。

### 2. .gateway_mode 文件传递模式

start_gateway.sh 启动时写 `.gateway_mode` 文件（内容为 `sim` 或 `ctp`），start_strategy.sh 读取该文件确定数据目录。映射关系：`sim` → `data/sim`，`ctp` → `data/live`。

### 3. trader -dataDir flag

main.go 新增 `-dataDir` flag（默认 `./data`），用于 `DailyInitPath` 和 `SaveMatrix2` 的根路径。start_strategy.sh 根据 .gateway_mode 传入 `-dataDir ./data/sim` 或 `-dataDir ./data/live`。

### 4. build_deploy_new.sh 同时部署两个模式的数据

无论 `--mode` 是什么，data/ 下的 sim 和 live 子目录都从 data_new 部署（只影响 config 的选择）。这样 deploy_new 可以随时切换模式而不需要重新部署。

## Risks / Trade-offs

- **[兼容性]** trader 新增 -dataDir flag，旧启动脚本不传该参数时 fallback 到 `./data`（向后兼容）
- **[目录不存在]** start_strategy.sh 需确保 `data/${mode}/` 目录存在，不存在时报错提示
