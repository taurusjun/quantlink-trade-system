## Phase 1: data_new 重组（已完成）

- [x] 1.1 重组 data_new 为 common/sim/live 三层结构
- [x] 1.2 build_deploy_new.sh 新增 --mode sim|live 参数，config 按模式合并

## Phase 2: deploy_new data 按模式分目录

- [x] 2.1 修改 `build_deploy_new.sh` 数据合并段落：sim 和 live 的 data 同时部署到 `deploy_new/data/sim/` 和 `deploy_new/data/live/`（不覆盖已有文件）
- [x] 2.2 Go trader `main.go` 新增 `-dataDir` flag（默认 `./data`），替代 `DailyInitPath` 中硬编码的 `./data`
- [x] 2.3 修改 `start_gateway.sh` 模板：启动时写 `.gateway_mode` 文件（内容为 sim 或 ctp）
- [x] 2.4 修改 `start_strategy.sh` 模板：读 `.gateway_mode`，映射 sim→`./data/sim`、ctp→`./data/live`，传 `-dataDir` 给 trader
- [x] 2.5 修改 `stop_all.sh` 模板：清理 `.gateway_mode` 文件

## Phase 3: 验证

- [x] 3.1 `--mode sim` 部署后启动 sim 测试：trader 日志显示从 `./data/sim/daily_init.92201` 加载
- [x] 3.2 `--mode live` 部署后启动 ctp 测试：trader 日志显示从 `./data/live/daily_init.92201` 加载
- [x] 3.3 验证两种模式的 daily_init 互不污染
