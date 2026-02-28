## live-sim-directory-structure

### 要求

1. `data_new/` 模板目录结构:
   - `common/config/` — 共享配置
   - `live/{controls,models,data}/` — CTP 实盘环境
   - `sim/{controls,models,data}/` — 模拟测试环境
   - `common/controls/` 不再存在

2. `deploy_java/` 运行时目录结构:
   - `config/` — 合并后的配置
   - `live/{controls,models,data}/` — CTP 实盘
   - `sim/{controls,models,data}/` — 模拟
   - 顶层不再有 `controls/`、`models/`、`data/` 目录

3. control 文件 model 路径:
   - live: `./live/models/model.{symbols}.par.txt.{id}`
   - sim: `./sim/models/model.{symbols}.par.txt.{id}`

4. `build_deploy_java.sh` 部署逻辑:
   - 从 `data_new/{live,sim}/` 分别复制 controls/models/data 到 `deploy_java/{live,sim}/`
   - 不再有运行时 model 覆盖

5. `start_strategy.sh` 启动逻辑:
   - 根据 `.gateway_mode` (ctp→live, sim→sim) 选择环境目录
   - controlFile 路径: `${ENV_DIR}/controls/${SESSION}/control.*.par.txt.${ID}`
   - dataDir: `./${ENV_DIR}/data`

6. CTP 实盘风控参数匹配持仓规模:
   - MAX_SIZE=100, STOP_LOSS=80000, MAX_LOSS=80000, UPNL_LOSS=60000
   - BEGIN_PLACE=0.8, LONG_PLACE=2.0, SHORT_PLACE=-1.2, AVG_SPREAD_AWAY=110
